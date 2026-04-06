package ws

// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/models/ws/admin"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/ws/ratelimiter"
	"github.com/gofiber/contrib/v3/websocket"
	"go.uber.org/zap"
)

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
	Send          chan []byte
	Room          string
	SessionId     string
	ClientIP      string
	WebAccessUser any
	Handler       *PadMessageHandler
	adminHandler  *AdminMessageHandler
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
		if err := ratelimiter.CheckRateLimit(ratelimiter.IPAddress(c.ClientIP), retrievedSettings.CommitRateLimiting); err != nil {
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

			c.Handler.HandleMessage(clientReady, c, retrievedSettings, logger)
		} else if strings.Contains(decodedMessage, "PAD_DELETE") {
			var padDelete PadDelete
			err := json.Unmarshal(message, &padDelete)
			if err != nil {
				logger.Error("Error unmarshalling PAD_DELETE: ", err)
				continue
			}
			c.Handler.HandleMessage(padDelete, c, retrievedSettings, logger)
		} else if strings.Contains(decodedMessage, "SAVE_REVISION") {
			var saveRevision SavedRevision
			err := json.Unmarshal(message, &saveRevision)
			if err != nil {
				logger.Error("Error unmarshalling SAVE_REVISION: ", err)
				continue
			}

			c.Handler.HandleMessage(saveRevision, c, retrievedSettings, logger)

		} else if strings.Contains(decodedMessage, "USER_CHANGES") {
			var userchange ws.UserChange
			err := json.Unmarshal(message, &userchange)

			if err != nil {
				logger.Error("Error unmarshalling USER_CHANGES: ", err)
				continue
			}

			c.Handler.HandleMessage(userchange, c, retrievedSettings, logger)
		} else if strings.Contains(decodedMessage, "USERINFO_UPDATE") {
			var userInfoChange UserInfoUpdateWrapper
			errorUserInfoChange := json.Unmarshal(message, &userInfoChange)

			if errorUserInfoChange != nil {
				logger.Error("Error unmarshalling USERINFO_UPDATE: ", errorUserInfoChange)
				continue
			}

			c.Handler.HandleMessage(userInfoChange.Data, c, retrievedSettings, logger)
		} else if strings.Contains(decodedMessage, "GET_CHAT_MESSAGES") {
			var getChatMessages ws.GetChatMessages
			err := json.Unmarshal(message, &getChatMessages)

			if err != nil {
				logger.Error("Error unmarshalling GET_CHAT_MESSAGES: ", err)
				continue
			}

			c.Handler.HandleMessage(getChatMessages, c, retrievedSettings, logger)
		} else if strings.Contains(decodedMessage, "CHANGESET_REQ") {
			var changesetReq ws.ChangesetReq
			err := json.Unmarshal(message, &changesetReq)
			if err != nil {
				logger.Error("Error unmarshalling CHANGESET_REQ: ", err)
				continue
			}

			c.Handler.HandleMessage(changesetReq, c, retrievedSettings, logger)
		} else if strings.Contains(decodedMessage, "CHAT_MESSAGE") {
			var chatMessage ws.ChatMessage
			err := json.Unmarshal(message, &chatMessage)

			if err != nil {
				logger.Error("Error unmarshalling CHAT_MESSAGE: ", err)
				continue
			}
			c.Handler.HandleMessage(chatMessage, c, retrievedSettings, logger)
		}

		c.Hub.Broadcast <- message
	}
}

func (c *Client) Leave() {
	c.Hub.Unregister <- c
}

// SafeSend sends a message to the client, returning false if the channel is closed.
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
	msg, _ := json.Marshal([]interface{}{"message", map[string]string{"disconnect": "userdup"}})
	c.SafeSend(msg)
}

func (c *Client) SendPadDelete() {
	msg, _ := json.Marshal([]interface{}{"message", map[string]string{"disconnect": "deleted"}})
	c.SafeSend(msg)
}

// ServeWs handles websocket requests from the peer using Fiber's websocket middleware.
func ServeWs(conn *websocket.Conn, sessionID string, clientIP string, webAccessUser any,
	configSettings *settings.Settings,
	logger *zap.SugaredLogger, handler *PadMessageHandler) {
	client := &Client{Hub: handler.hub, Conn: conn, Send: make(chan []byte, 256), SessionId: sessionID, ClientIP: clientIP, WebAccessUser: webAccessUser, Handler: handler}
	handler.SessionStore.initSession(sessionID)
	client.Hub.Register <- client
	go client.writePump()
	client.readPump(configSettings, logger)
}
