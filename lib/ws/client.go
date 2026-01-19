package ws

// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/models/ws/admin"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/ws/ratelimiter"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"go.uber.org/zap"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	Hub *Hub
	// The websocket connection.
	Conn WebSocketConn
	// Buffered channel of outbound messages.
	Send         chan []byte
	Room         string
	SessionId    string
	Ctx          *fiber.Ctx
	Handler      *PadMessageHandler
	adminHandler *AdminMessageHandler
}

func (c *Client) readPumpAdmin(retrievedSettings *settings.Settings, logger *zap.SugaredLogger) {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(retrievedSettings.SocketIo.MaxHttpBufferSize)
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		var eventMessage admin.EventMessage

		err = json.Unmarshal(message, &eventMessage)
		if err != nil {
			logger.Error("error unmarshalling", err)
			return
		}
		c.adminHandler.HandleMessage(eventMessage, retrievedSettings, c)
	}
}

const (
	writeWait = 10 * time.Second

	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub hat den Channel geschlossen
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump pumps messages from the websocket connection to the Hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump(retrievedSettings *settings.Settings, logger *zap.SugaredLogger) {
	c.Hub.Register <- c
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(retrievedSettings.SocketIo.MaxHttpBufferSize)
	for {
		_, message, err := c.Conn.ReadMessage()
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		c.Conn.SetPongHandler(func(string) error {
			c.Conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			c.Handler.HandleDisconnectOfPadClient(c, retrievedSettings, logger)
			break
		}
		retrievedSettings.CommitRateLimiting.LoadTest = retrievedSettings.LoadTest
		if err := ratelimiter.CheckRateLimit(ratelimiter.IPAddress(c.Ctx.IP()), retrievedSettings.CommitRateLimiting); err != nil {
			println("Rate limit exceeded:", err.Error())
			continue
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		decodedMessage := string(message[:])

		if strings.Contains(decodedMessage, "CLIENT_READY") {
			var clientReady ws.ClientReady
			err := json.Unmarshal(message, &clientReady)
			if err != nil {
				println("Error unmarshalling", err)
			}

			c.Handler.HandleMessage(clientReady, c, c.Ctx, retrievedSettings, logger)
		} else if strings.Contains(decodedMessage, "USER_CHANGES") {
			var userchange ws.UserChange
			err := json.Unmarshal(message, &userchange)

			if err != nil {
				logger.Error("Error unmarshalling USER_CHANGES: ", err)
				continue
			}

			c.Handler.HandleMessage(userchange, c, c.Ctx, retrievedSettings, logger)
		} else if strings.Contains(decodedMessage, "USERINFO_UPDATE") {
			var userInfoChange UserInfoUpdateWrapper
			errorUserInfoChange := json.Unmarshal(message, &userInfoChange)

			if errorUserInfoChange != nil {
				logger.Error("Error unmarshalling USERINFO_UPDATE: ", errorUserInfoChange)
				continue
			}

			c.Handler.HandleMessage(userInfoChange.Data, c, c.Ctx, retrievedSettings, logger)
		} else if strings.Contains(decodedMessage, "GET_CHAT_MESSAGES") {
			var getChatMessages ws.GetChatMessages
			err := json.Unmarshal(message, &getChatMessages)

			if err != nil {
				logger.Error("Error unmarshalling GET_CHAT_MESSAGES: ", err)
				continue
			}

			c.Handler.HandleMessage(getChatMessages, c, c.Ctx, retrievedSettings, logger)
		} else if strings.Contains(decodedMessage, "CHANGESET_REQ") {
			var changesetReq ws.ChangesetReq
			err := json.Unmarshal(message, &changesetReq)
			if err != nil {
				logger.Error("Error unmarshalling CHANGESET_REQ: ", err)
				continue
			}

			c.Handler.HandleMessage(changesetReq, c, c.Ctx, retrievedSettings, logger)
		} else if strings.Contains(decodedMessage, "CHAT_MESSAGE") {
			var chatMessage ws.ChatMessage
			err := json.Unmarshal(message, &chatMessage)

			if err != nil {
				logger.Error("Error unmarshalling CHAT_MESSAGE: ", err)
				continue
			}
			c.Handler.HandleMessage(chatMessage, c, c.Ctx, retrievedSettings, logger)
		}

		c.Hub.Broadcast <- message
	}
}

func (c *Client) Leave() {
	c.Hub.Unregister <- c
}

// SafeSend sends a message to the client, returning false if the channel is closed.
// This prevents panic on send to closed channel.
func (c *Client) SafeSend(message []byte) (sent bool) {
	defer func() {
		if recover() != nil {
			sent = false
		}
	}()
	select {
	case c.Send <- message:
		return true
	default:
		return false
	}
}

func (c *Client) SendUserDupMessage() {
	c.SafeSend([]byte(`{"disconnect":"userdup"}`))
}

func (c *Client) SendPadDelete() {
	c.SafeSend([]byte(`{"disconnect":"deleted"}`))
}

// ServeWs handles websocket requests from the peer.
func ServeWs(w http.ResponseWriter, r *http.Request, sessionStore *session.Store,
	fiber *fiber.Ctx, configSettings *settings.Settings,
	logger *zap.SugaredLogger, handler *PadMessageHandler) {
	store, err := sessionStore.Get(fiber)

	if err != nil {
		fiber.SendString("Error estabilishing socket conn")
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{Hub: handler.hub, Conn: conn, Send: make(chan []byte, 256), SessionId: store.ID(), Ctx: fiber, Handler: handler}
	handler.SessionStore.initSession(store.ID())
	client.Hub.Register <- client
	go client.writePump()
	client.readPump(configSettings, logger)
}
