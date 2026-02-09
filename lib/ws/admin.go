package ws

import (
	"log"
	"net/http"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

func ServeAdminWs(w http.ResponseWriter, r *http.Request, fiber fiber.Ctx, configSettings *settings.Settings, logger *zap.SugaredLogger, handler AdminMessageHandler, done chan struct{}) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		close(done)
		return
	}
	client := &Client{Hub: handler.hub, Conn: conn, Send: make(chan []byte, 256), Ctx: fiber, adminHandler: &handler}
	client.Hub.Register <- client
	go client.writePump()
	client.readPumpAdmin(configSettings, logger)
	close(done)
}
