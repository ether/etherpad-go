package ws

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WebSocketConnData struct {
	messageType int
	Data        []byte
}
type MockWebSocketConn struct {
	closed bool
	mu     sync.Mutex
	Data   []WebSocketConnData
}

func (m *MockWebSocketConn) NextWriter(messageType int) (io.WriteCloser, error) {
	println("NextWriter", messageType)
	return nil, nil
}

func (m *MockWebSocketConn) SetReadLimit(size int64) {
	println("SetReadLimit", size)
}

func (m *MockWebSocketConn) ReadMessage() (messageType int, p []byte, err error) {
	time.Sleep(1 * time.Second)
	return websocket.TextMessage, []byte("test"), nil
}

func (m *MockWebSocketConn) WriteMessage(messageType int, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return websocket.ErrCloseSent
	}
	m.Data = append(m.Data, WebSocketConnData{
		messageType: messageType,
		Data:        data,
	})

	return nil
}

func (m *MockWebSocketConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	return nil
}

func (m *MockWebSocketConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (m *MockWebSocketConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *MockWebSocketConn) SetPongHandler(h func(appData string) error) {
	// Mock implementation
}

func (m *MockWebSocketConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return nil
}

func (m *MockWebSocketConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
}

func NewMockWebSocketConn() WebSocketConn {
	return &MockWebSocketConn{
		closed: false,
		mu:     sync.Mutex{},
		Data:   make([]WebSocketConnData, 0),
	}
}

func NewActualMockWebSocketconn() *MockWebSocketConn {
	return &MockWebSocketConn{
		closed: false,
		mu:     sync.Mutex{},
		Data:   make([]WebSocketConnData, 0),
	}
}
