package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/models/ws/admin"
)

// Hub maintains the set of active Clients and broadcasts messages to the
// Clients.
type Hub struct {
	// Registered Clients.
	Clients        map[*Client]bool
	ClientsRWMutex sync.RWMutex

	// Inbound messages from the Clients.
	Broadcast chan []byte

	// Register requests from the Clients.
	Register chan *Client

	// Unregister requests from Clients.
	Unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Clients:    make(map[*Client]bool),
	}
}

// BroadcastShout sends a sticky/non-sticky notice to every connected client
// using the same COLLABROOM/result:shout envelope the admin "shout" command
// uses. Used by the updater to announce an imminent restart during draining.
func (h *Hub) BroadcastShout(message string, sticky bool) {
	resp := admin.ShoutMessageResponse{Type: "COLLABROOM"}
	resp.Data.Type = "result:shout"
	resp.Data.Payload = admin.ShoutMessage{
		Timestamp: time.Now().UnixMilli(),
		Message:   admin.ShoutMessageRequest{Message: message, Sticky: sticky},
	}
	payload, err := json.Marshal([]any{"result:shout", resp})
	if err != nil {
		return
	}
	h.ClientsRWMutex.RLock()
	defer h.ClientsRWMutex.RUnlock()
	for client := range h.Clients {
		if client != nil {
			client.SafeSend(payload)
		}
	}
}

// Run registers and unregisters clients and delivers Broadcast messages
// until one of the hub's channels is closed.
func (h *Hub) Run() {
	for {
		select {
		case client, ok := <-h.Register:
			if !ok {
				return
			}
			if client == nil {
				continue
			}
			h.ClientsRWMutex.Lock()
			h.Clients[client] = true
			h.ClientsRWMutex.Unlock()
		case client, ok := <-h.Unregister:
			if !ok {
				return
			}
			if client == nil {
				continue
			}
			h.ClientsRWMutex.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}
			h.ClientsRWMutex.Unlock()
		case message, ok := <-h.Broadcast:
			if !ok {
				return
			}
			h.broadcast(message)
		}
	}
}

// broadcast sends message to every client and drops the clients whose send
// buffer is full. It changes the client set, so it needs the write lock.
func (h *Hub) broadcast(message []byte) {
	h.ClientsRWMutex.Lock()
	defer h.ClientsRWMutex.Unlock()
	for client := range h.Clients {
		if client == nil {
			continue
		}
		select {
		case client.Send <- message:
		default:
			close(client.Send)
			delete(h.Clients, client)
		}
	}
}

// HasClient reports whether client is registered.
func (h *Hub) HasClient(client *Client) bool {
	h.ClientsRWMutex.RLock()
	defer h.ClientsRWMutex.RUnlock()
	return h.Clients[client]
}

// NumClients is the number of registered clients.
func (h *Hub) NumClients() int {
	h.ClientsRWMutex.RLock()
	defer h.ClientsRWMutex.RUnlock()
	return len(h.Clients)
}
