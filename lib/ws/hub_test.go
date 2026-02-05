package ws

import (
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func TestNewHub(t *testing.T) {
	hub := NewHub()
	require.NotNil(t, hub)
	assert.NotNil(t, hub.Clients)
	assert.NotNil(t, hub.Broadcast)
	assert.NotNil(t, hub.Register)
	assert.NotNil(t, hub.Unregister)
	assert.Empty(t, hub.Clients)
}

func TestHub_RegisterClient(t *testing.T) {
	hub := NewHub()
	client := &Client{
		Hub:       hub,
		Conn:      NewMockWebSocketConn(),
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
		Ctx:       nil,
		Handler:   nil,
	}

	go hub.Run()
	defer func() {
		close(hub.Register)
		close(hub.Unregister)
		close(hub.Broadcast)
	}()

	hub.Register <- client

	time.Sleep(10 * time.Millisecond)

	assert.Contains(t, hub.Clients, client)
}

func TestHub_UnregisterClient(t *testing.T) {
	hub := NewHub()
	client := &Client{
		Hub:       hub,
		Conn:      NewMockWebSocketConn(),
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session123",
	}

	go hub.Run()
	defer func() {
		close(hub.Register)
		close(hub.Unregister)
		close(hub.Broadcast)
	}()

	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	hub.Unregister <- client
	time.Sleep(10 * time.Millisecond)

	assert.NotContains(t, hub.Clients, client)

	select {
	case _, ok := <-client.Send:
		assert.False(t, ok, "Send channel should be closed")
	default:
	}
}

func TestHub_BroadcastMessage(t *testing.T) {
	hub := NewHub()

	client1 := &Client{
		Hub:       hub,
		Conn:      NewMockWebSocketConn(),
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session1",
	}

	client2 := &Client{
		Hub:       hub,
		Conn:      NewMockWebSocketConn(),
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session2",
	}

	go hub.Run()
	defer func() {
		close(hub.Register)
		close(hub.Unregister)
		close(hub.Broadcast)
	}()

	hub.Register <- client1
	hub.Register <- client2
	time.Sleep(10 * time.Millisecond)

	testMessage := []byte(`{"type":"test","data":"hello"}`)
	hub.Broadcast <- testMessage
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-client1.Send:
		assert.Equal(t, testMessage, msg)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Client1 did not receive message")
	}

	select {
	case msg := <-client2.Send:
		assert.Equal(t, testMessage, msg)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Client2 did not receive message")
	}
}

func TestHub_BroadcastToFullChannel(t *testing.T) {
	hub := NewHub()

	client := &Client{
		Hub:       hub,
		Conn:      NewMockWebSocketConn(),
		Send:      make(chan []byte, 1),
		Room:      "test-pad",
		SessionId: "session1",
	}

	go hub.Run()
	defer func() {
		close(hub.Register)
		close(hub.Unregister)
		close(hub.Broadcast)
	}()

	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	client.Send <- []byte("first message")

	hub.Broadcast <- []byte("second message that causes overflow")
	time.Sleep(50 * time.Millisecond)

	assert.NotContains(t, hub.Clients, client)
}

func TestHub_ConcurrentOperations(t *testing.T) {
	hub := NewHub()

	go hub.Run()
	defer func() {
		close(hub.Register)
		close(hub.Unregister)
		close(hub.Broadcast)
	}()

	const numClients = 10
	const numMessages = 5

	var wg sync.WaitGroup
	clients := make([]*Client, numClients)

	wg.Add(numClients)
	for i := 0; i < numClients; i++ {
		go func(index int) {
			defer wg.Done()
			clients[index] = &Client{
				Hub:       hub,
				Conn:      NewMockWebSocketConn(),
				Send:      make(chan []byte, 256),
				Room:      "test-pad",
				SessionId: "session" + string(rune('0'+index)),
			}
			hub.Register <- clients[index]
		}(i)
	}
	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	wg.Add(numMessages)
	for i := 0; i < numMessages; i++ {
		go func(msgIndex int) {
			defer wg.Done()
			message := []byte(`{"type":"test","msg":"` + string(rune('0'+msgIndex)) + `"}`)
			hub.Broadcast <- message
		}(i)
	}
	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	for i, client := range clients {
		if client == nil {
			continue
		}

		receivedCount := 0
		timeout := time.After(100 * time.Millisecond)

	messageLoop:
		for {
			select {
			case <-client.Send:
				receivedCount++
				if receivedCount >= numMessages {
					break messageLoop
				}
			case <-timeout:
				break messageLoop
			}
		}

		assert.Equal(t, numMessages, receivedCount, "Client %d should receive all messages", i)
	}
}

func TestHub_MultipleRooms(t *testing.T) {
	hub := NewHub()

	client1 := &Client{
		Hub:       hub,
		Conn:      NewMockWebSocketConn(),
		Send:      make(chan []byte, 256),
		Room:      "pad1",
		SessionId: "session1",
	}

	client2 := &Client{
		Hub:       hub,
		Conn:      NewMockWebSocketConn(),
		Send:      make(chan []byte, 256),
		Room:      "pad2",
		SessionId: "session2",
	}

	go hub.Run()
	defer func() {
		close(hub.Register)
		close(hub.Unregister)
		close(hub.Broadcast)
	}()

	hub.Register <- client1
	hub.Register <- client2
	time.Sleep(10 * time.Millisecond)

	testMessage := []byte(`{"type":"test","data":"cross-room"}`)
	hub.Broadcast <- testMessage
	time.Sleep(10 * time.Millisecond)

	select {
	case msg := <-client1.Send:
		assert.Equal(t, testMessage, msg)
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Client1 (pad1) did not receive message")
	}

	select {
	case msg := <-client2.Send:
		assert.Equal(t, testMessage, msg)
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Client2 (pad2) did not receive message")
	}
}

func TestHub_ClientWithHandlers(t *testing.T) {
	hub := NewHub()

	mockPadHandler := &PadMessageHandler{}
	mockAdminHandler := &AdminMessageHandler{}

	client := &Client{
		Hub:          hub,
		Conn:         NewMockWebSocketConn(),
		Send:         make(chan []byte, 256),
		Room:         "test-pad",
		SessionId:    "session1",
		Handler:      mockPadHandler,
		adminHandler: mockAdminHandler,
	}

	go hub.Run()
	defer func() {
		close(hub.Register)
		close(hub.Unregister)
		close(hub.Broadcast)
	}()

	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	assert.Contains(t, hub.Clients, client)
	assert.Equal(t, mockPadHandler, client.Handler)
	assert.Equal(t, mockAdminHandler, client.adminHandler)
}

func TestHub_ClientWithFiberContext(t *testing.T) {
	hub := NewHub()
	app := fiber.New()

	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})
	defer app.ReleaseCtx(ctx)

	client := &Client{
		Hub:       hub,
		Conn:      NewMockWebSocketConn(),
		Send:      make(chan []byte, 256),
		Room:      "test-pad",
		SessionId: "session1",
		Ctx:       ctx,
	}

	go hub.Run()
	defer func() {
		close(hub.Register)
		close(hub.Unregister)
		close(hub.Broadcast)
	}()

	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	assert.Contains(t, hub.Clients, client)
	assert.Equal(t, ctx, client.Ctx)
}
