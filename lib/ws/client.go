package ws

// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

import (
	"bytes"
	"encoding/json"
	"log"
	"runtime/debug"
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/models/ws/admin"
	"github.com/ether/etherpad-go/lib/settings"
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
	Send      chan []byte
	Room      string
	SessionId string
	// IntegratorSessionID carries the integrator-set sessionID cookie
	// value (from createSession() HTTP API), read from the socket.io
	// handshake so the cookie can be HttpOnly. Upstream #7045 / #7755.
	IntegratorSessionID string
	// legacySessionIdWarned tracks whether we've already emitted the
	// deprecation warning for clients that still forward sessionID via
	// the CLIENT_READY message payload.
	legacySessionIdWarned bool
	ClientIP              string
	WebAccessUser         any
	// CookieToken is the author token from the HttpOnly cookie of the
	// websocket handshake; it wins over the token in CLIENT_READY.
	CookieToken string
	// drain is shared by copies of the client (the handler copies Client
	// values); set by readPump.
	drain        *clientDrain
	Handler      *PadMessageHandler
	adminHandler *AdminMessageHandler
	// AdminAccount is the account of an admin connection, from its token.
	AdminAccount string
	// LoginSession is the key of the pad login session the connection was
	// opened with (LoginSessionKey), "" without one.
	LoginSession string
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
		c.dispatchAdminMessage(eventMessage, retrievedSettings, logger)
	}
}

func (c *Client) dispatchAdminMessage(eventMessage admin.EventMessage, retrievedSettings *settings.Settings, logger *zap.SugaredLogger) {
	defer recoverClientPanic(c, retrievedSettings, logger, "admin")
	c.adminHandler.HandleMessage(eventMessage, retrievedSettings, c)
}

// recoverClientPanic is deferred around the handling of one websocket
// message: it logs a panic and closes that client's connection instead of
// letting the panic end the process.
func recoverClientPanic(c *Client, retrievedSettings *settings.Settings, logger *zap.SugaredLogger, kind string) {
	r := recover()
	if r == nil {
		return
	}
	logger.Errorf("panic while handling a %s message from %s: %v\n%s", kind, retrievedSettings.LogIP(c.ClientIP), r, debug.Stack())
	if c.Conn != nil {
		_ = c.Conn.Close()
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
	if c.drain == nil {
		c.drain = &clientDrain{}
	}
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
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		if !c.beginHandling() {
			// Shutting down: a change not acknowledged is sent again by the
			// client after it reconnected elsewhere.
			continue
		}
		c.dispatchMessage(message, retrievedSettings, logger)
		c.drain.handling.Done()
	}
}

// clientDrain lets shutdown stop a connection between two messages: once
// closing, messages are no longer handled, and handling counts the one
// being handled, so CloseAll can let it finish (and its acknowledgement go
// out) before it closes the connection.
type clientDrain struct {
	mu       sync.Mutex
	closing  bool
	handling sync.WaitGroup
}

// beginHandling registers a message about to be handled; false once the
// connection is closing.
func (c *Client) beginHandling() bool {
	c.drain.mu.Lock()
	defer c.drain.mu.Unlock()
	if c.drain.closing {
		return false
	}
	c.drain.handling.Add(1)
	return true
}

// stopHandling makes the client take no further messages and waits (until
// deadline) for the one it is handling and for its queued replies to go out.
func (c *Client) stopHandling(deadline time.Time) {
	if c.drain == nil {
		return
	}
	c.drain.mu.Lock()
	c.drain.closing = true
	c.drain.mu.Unlock()
	done := make(chan struct{})
	go func() {
		c.drain.handling.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Until(deadline)):
		return
	}
	for len(c.Send) > 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
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
func ServeWs(conn *websocket.Conn, sessionID string, integratorSessionID string, clientIP string, webAccessUser any,
	configSettings *settings.Settings,
	logger *zap.SugaredLogger, handler *PadMessageHandler, cookieToken string, loginSession string) {
	client := &Client{Hub: handler.hub, Conn: conn, Send: make(chan []byte, 256), drain: &clientDrain{}, SessionId: sessionID, IntegratorSessionID: integratorSessionID, ClientIP: clientIP, WebAccessUser: webAccessUser, Handler: handler, CookieToken: cookieToken, LoginSession: loginSession}
	handler.SessionStore.initSession(sessionID)
	client.Hub.Register <- client
	go client.writePump()
	client.readPump(configSettings, logger)
}
