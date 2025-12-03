package ws

import (
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Mock WebSocket connection implementing WebSocketConn interface
type mockWebSocketConn struct {
	closed bool
	mu     sync.Mutex
}

func (m *mockWebSocketConn) SetReadLimit(size int64) {
	println("SetReadLimit", size)
}

func (m *mockWebSocketConn) ReadMessage() (messageType int, p []byte, err error) {
	time.Sleep(1 * time.Second)
	return websocket.TextMessage, []byte("test"), nil
}

func (m *mockWebSocketConn) WriteMessage(messageType int, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return websocket.ErrCloseSent
	}
	return nil
}

func (m *mockWebSocketConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	return nil
}

func (m *mockWebSocketConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (m *mockWebSocketConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockWebSocketConn) SetPongHandler(h func(appData string) error) {
	// Mock implementation
}

func (m *mockWebSocketConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return nil
}

func (m *mockWebSocketConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
}

func NewMockWebSocketConn() WebSocketConn {
	return &mockWebSocketConn{
		closed: false,
		mu:     sync.Mutex{},
	}
}
