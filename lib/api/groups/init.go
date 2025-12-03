package groups

import (
	"github.com/ether/etherpad-go/lib/api/errors"
	"github.com/gofiber/fiber/v2"
)

func Init(app *fiber.App) {
	app.Get("/groups/pads", func(c *fiber.Ctx) error {
		var groupId = c.Query("groupID")
		if groupId == "" {
			return c.Status(400).JSON(errors.NewMissingParamError("groupID"))
		}
		return c.SendStatus(200)
	})
}
