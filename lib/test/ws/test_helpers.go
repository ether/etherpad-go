package ws

import (
	"sync"
	"time"

	libws "github.com/ether/etherpad-go/lib/ws"
)

// mockWritePump simulates the writePump goroutine for tests.
// It reads from the client's Send channel and writes to the MockWebSocket.
func mockWritePump(client *libws.Client, mockConn *libws.MockWebSocketConn, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case message, ok := <-client.Send:
			if !ok {
				return
			}
			mockConn.WriteMessage(1, message)
		case <-time.After(500 * time.Millisecond):
			return
		}
	}
}

// startMockWritePump starts a mock write pump and returns a WaitGroup to wait for completion
func startMockWritePump(client *libws.Client, mockConn *libws.MockWebSocketConn) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	go mockWritePump(client, mockConn, &wg)
	return &wg
}
