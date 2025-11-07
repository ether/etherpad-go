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
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub
	// The websocket connection.
	conn *websocket.Conn
	// Buffered channel of outbound messages.
	Send      chan []byte
	Room      string
	SessionId string
	ctx       *fiber.Ctx
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			HandleDisconnectOfPadClient(c)
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		decodedMessage := string(message[:])

		if strings.Contains(decodedMessage, "CLIENT_READY") {
			var clientReady ws.ClientReady
			err := json.Unmarshal(message, &clientReady)
			if err != nil {
				println("Error unmarshalling", err)
			}

			handleMessage(clientReady, c, c.ctx)
		} else if strings.Contains(decodedMessage, "USER_CHANGES") {
			var userchange ws.UserChange
			err := json.Unmarshal(message, &userchange)

			if err != nil {
				println("Error unmarshalling")
				return
			}

			handleMessage(userchange, c, c.ctx)
		} else if strings.Contains(decodedMessage, "USERINFO_UPDATE") {
			var userInfoChange UserInfoUpdateWrapper
			errorUserInfoChange := json.Unmarshal(message, &userInfoChange)

			if errorUserInfoChange != nil {
				println("Error unmarshalling")
				return
			}

			handleMessage(userInfoChange.Data, c, c.ctx)
		} else if strings.Contains(decodedMessage, "GET_CHAT_MESSAGES") {
			var getChatMessages ws.GetChatMessages
			err := json.Unmarshal(message, &getChatMessages)

			if err != nil {
				println("Error unmarshalling", err)
				return
			}

			handleMessage(getChatMessages, c, c.ctx)

		} else if strings.Contains(decodedMessage, "CHAT_MESSAGE") {
			var chatMessage ws.ChatMessage
			err := json.Unmarshal(message, &chatMessage)

			if err != nil {
				println("Error unmarshalling", err)
			}
			handleMessage(chatMessage, c, c.ctx)
		}

		c.hub.Broadcast <- message
	}
}

func (c *Client) Leave() {
	HubGlob.Unregister <- c
}

func (c *Client) SendUserDupMessage() {
	c.Send <- []byte(`{"disconnect":"userdup"}`)
}

func (c *Client) SendPadDelete() {
	c.Send <- []byte(`{"disconnect":"deleted"}`)
}

// ServeWs serveWs handles websocket requests from the peer.
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, sessionStore *session.Store, fiber *fiber.Ctx) {
	store, err := sessionStore.Get(fiber)

	if err != nil {
		fiber.SendString("Error estabilishing socket conn")
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, Send: make(chan []byte, 256), SessionId: store.ID(), ctx: fiber}
	SessionStoreInstance.initSession(store.ID())
	client.hub.Register <- client
	client.readPump()
}
