package ws

import (
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/contrib/v3/websocket"
	"go.uber.org/zap"
)

// ServeAdminWs handles admin websocket requests using Fiber's websocket middleware.
// accountID is the account the admin token was issued to.
func ServeAdminWs(conn *websocket.Conn, configSettings *settings.Settings, logger *zap.SugaredLogger, handler AdminMessageHandler, accountID string) {
	client := &Client{Hub: handler.hub, Conn: conn, Send: make(chan []byte, 256), adminHandler: &handler, AdminAccount: accountID}
	client.Hub.Register <- client
	go client.writePump()
	client.readPumpAdmin(configSettings, logger)
}
