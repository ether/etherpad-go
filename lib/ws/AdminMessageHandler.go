package ws

import (
	"encoding/json"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/models/ws/admin"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gorilla/websocket"
)

type AdminMessageHandler struct {
	store      db.DataStore
	hook       *hooks.Hook
	padManager *pad.Manager
}

func NewAdminMessageHandler(store db.DataStore, h *hooks.Hook, m *pad.Manager) AdminMessageHandler {
	return AdminMessageHandler{
		store:      store,
		hook:       h,
		padManager: m,
	}
}

func (h AdminMessageHandler) HandleMessage(message admin.EventMessage, retrievedSettings *settings.Settings, c *Client) {
	switch message.Event {
	case "load":
		{
			resp := make([]interface{}, 2)
			resp[0] = "settings"
			resp[1] = map[string]interface{}{
				"results": retrievedSettings,
			}
			responseBytes, err := json.Marshal(resp)
			if err != nil {
				println("Error marshalling response:", err.Error())
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, responseBytes)
		}
	default:
		// Unknown event
		println("Unknown admin event:", message.Event)
	}
}
