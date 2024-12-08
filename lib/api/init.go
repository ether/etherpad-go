package api

import (
	"github.com/ether/etherpad-go/lib/api/author"
	"github.com/ether/etherpad-go/lib/api/groups"
	"github.com/ether/etherpad-go/lib/api/pad"
	"github.com/gofiber/fiber/v2"
)

func InitAPI(c *fiber.App) {
	author.Init(c)
	pad.Init(c)
	groups.Init(c)
}
