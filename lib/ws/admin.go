package ws

import (
	"log"
	"net/http"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func ServeAdminWs(hub *Hub, w http.ResponseWriter, r *http.Request, fiber *fiber.Ctx, configSettings *settings.Settings, logger *zap.SugaredLogger, handler AdminMessageHandler) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{Hub: hub, Conn: conn, Send: make(chan []byte, 256), Ctx: fiber, adminHandler: &handler}
	client.Hub.Register <- client
	client.readPumpAdmin(configSettings, logger)
}
