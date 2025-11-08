package api

import (
	"embed"

	"github.com/ether/etherpad-go/lib/api/author"
	"github.com/ether/etherpad-go/lib/api/groups"
	"github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/api/static"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

func InitAPI(c *fiber.App, uiAssets embed.FS, settings settings.Settings, cookieStore *session.Store) {
	author.Init(c)
	pad.Init(c)
	groups.Init(c)
	static.Init(c, uiAssets, settings, cookieStore)
}
