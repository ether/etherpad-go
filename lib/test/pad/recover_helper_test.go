package pad

import (
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

// recoveringApp is parityApp behind the recover middleware the server
// installs (lib/server/recover.go).
func recoveringApp(hookSystem *hooks.Hook, cfg *settings.Settings) *fiber.App {
	app := fiber.New()
	app.Use(recover.New())
	inner := parityApp(hookSystem, cfg)
	app.Use(func(c fiber.Ctx) error {
		inner.Handler()(c.RequestCtx())
		return nil
	})
	return app
}
