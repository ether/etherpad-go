package ws

import "sync"

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

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			if client == nil {
				continue
			}
			h.ClientsRWMutex.Lock()
			h.Clients[client] = true
			h.ClientsRWMutex.Unlock()
		case client := <-h.Unregister:
			if client == nil {
				continue
			}
			h.ClientsRWMutex.Lock()
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}
			h.ClientsRWMutex.Unlock()
		case message := <-h.Broadcast:
			h.ClientsRWMutex.Lock()
			for client := range h.Clients {
				if client == nil {
					continue
				}
				select {
				case client.Send <- message:
				default:
					// Channel ist voll, Client entfernen
					println("Removing client due to full channel")
					delete(h.Clients, client)
					close(client.Send)
				}
			}
			h.ClientsRWMutex.Unlock()
		}
	}
}
