package api

import (
	"embed"

	"github.com/ether/etherpad-go/lib/api/author"
	"github.com/ether/etherpad-go/lib/api/groups"
	"github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/api/static"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

func InitAPI(c *fiber.App, uiAssets embed.FS, settings settings.Settings, cookieStore *session.Store, store db.DataStore, handler *ws.PadMessageHandler) {
	author.Init(c, store)
	pad.Init(c, store, handler)
	groups.Init(c)
	static.Init(c, uiAssets, settings, cookieStore)
}
