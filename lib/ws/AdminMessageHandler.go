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
	case "padLoad":
		{
			var padLoadData admin.PadLoadData
			if err := json.Unmarshal([]byte(message.Data), &padLoadData); err != nil {
				println("Error unmarshalling padLoad data:", err.Error())
				return
			}
			dbPads, err := h.store.QueryPad(padLoadData.Offset, padLoadData.Limit, padLoadData.SortBy, padLoadData.Ascending, padLoadData.Pattern)
			if err != nil {
				println("Error querying pads:", err.Error())
				return
			}

			resp := make([]interface{}, 2)
			resp[0] = "results:padLoad"
			resp[1] = map[string]interface{}{
				"pads": dbPads,
			}

			responseBytes, err := json.Marshal(resp)
			if err != nil {
				println("Error marshalling response:", err.Error())
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, responseBytes)
		}
	case "getInstalled":
		{
			var epPlugin = []admin.InstalledPluginDefinition{
				{
					Name:     "etherpad",
					Version:  retrievedSettings.GitVersion,
					Path:     "/etherpad",
					RealPath: "/etherpad",
				},
			}

			resp := make([]interface{}, 2)
			resp[0] = "results:installed"
			resp[1] = map[string]interface{}{
				"installed": epPlugin,
			}

			responseBytes, err := json.Marshal(resp)
			if err != nil {
				println("Error marshalling response:", err.Error())
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, responseBytes)
		}
	case "search":
		{
			pluginDef := admin.SeachchPluginDefinition{
				Results: make([]admin.PluginSearchDefinition, 0),
				Query: struct {
					Offset     int    `json:"offset"`
					Limit      int    `json:"limit"`
					SortBy     string `json:"sortBy"`
					SortDir    string `json:"sortDir"`
					SearchTerm string `json:"searchTerm"`
				}{Offset: 0, Limit: 99999, SortBy: "name", SortDir: "asc", SearchTerm: ""},
			}

			resp := make([]interface{}, 2)
			resp[0] = "results:search"
			resp[1] = pluginDef

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
