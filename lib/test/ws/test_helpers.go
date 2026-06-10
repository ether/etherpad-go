package ws

import (
	"sync"

	libws "github.com/ether/etherpad-go/lib/ws"
)

// mockPump simulates the writePump goroutine for tests. Handlers send
// messages synchronously (SafeSend into the buffered Send channel), so by
// the time a test calls Wait() every message is already buffered. Wait()
// closes the channel, lets the pump drain it deterministically and then
// installs a fresh Send channel so the same client can be reused with a new
// pump. This replaces an earlier idle-timeout based pump that flaked when a
// handler (e.g. bulkDeletePads on a slow MySQL container) took longer than
// the timeout before responding.
type mockPump struct {
	wg     sync.WaitGroup
	client *libws.Client
	once   sync.Once
}

// Wait drains and stops the pump. Safe to call multiple times.
func (p *mockPump) Wait() {
	p.once.Do(func() {
		close(p.client.Send)
		p.wg.Wait()
		p.client.Send = make(chan []byte, 256)
	})
}

func mockWritePump(send <-chan []byte, mockConn *libws.MockWebSocketConn, wg *sync.WaitGroup) {
	defer wg.Done()
	for message := range send {
		mockConn.WriteMessage(1, message)
	}
}

// startMockWritePump starts a mock write pump. Call Wait() on the returned
// pump after the (synchronous) handler call to drain the client's Send
// channel into the MockWebSocketConn.
func startMockWritePump(client *libws.Client, mockConn *libws.MockWebSocketConn) *mockPump {
	p := &mockPump{client: client}
	p.wg.Add(1)
	go mockWritePump(client.Send, mockConn, &p.wg)
	return p
}
