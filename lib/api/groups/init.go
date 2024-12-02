package groups

import (
	error2 "github.com/ether/etherpad-go/lib/api/error"
	"github.com/gofiber/fiber/v2"
)

func Init(app *fiber.App) {
	app.Get("/groups/pads", func(c *fiber.Ctx) error {
		var groupId = c.Query("groupID")
		if groupId == "" {
			return c.Status(400).JSON(error2.Error{
				Message: "groupID is required",
				Error:   400,
			})
		}
		return c.SendStatus(200)
	})
}
