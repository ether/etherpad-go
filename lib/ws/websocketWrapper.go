package ws

import (
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketConn interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	Close() error
	SetWriteDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetPongHandler(h func(appData string) error)
	WriteControl(messageType int, data []byte, deadline time.Time) error
	RemoteAddr() net.Addr
	SetReadLimit(size int64)
	NextWriter(messageType int) (io.WriteCloser, error)
}

type WebSocketWrapper struct {
	*websocket.Conn
}

func NewWebSocketWrapper(conn *websocket.Conn) WebSocketConn {
	return &WebSocketWrapper{Conn: conn}
}
