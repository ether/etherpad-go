package api

import (
	"embed"

	"github.com/ether/etherpad-go/lib/api/author"
	"github.com/ether/etherpad-go/lib/api/groups"
	"github.com/ether/etherpad-go/lib/api/oidc"
	"github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/api/static"
	"github.com/ether/etherpad-go/lib/db"
	pad2 "github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"go.uber.org/zap"
)

func InitAPI(c *fiber.App, uiAssets embed.FS, retrievedSettings settings.Settings, cookieStore *session.Store, store db.DataStore, handler *ws.PadMessageHandler, manager *pad2.Manager, validator *validator.Validate, setupLogger *zap.SugaredLogger) {
	author.Init(c, store, validator)
	pad.Init(c, handler, manager)
	groups.Init(c)
	static.Init(c, uiAssets, retrievedSettings, cookieStore)
	oidc.Init(c, retrievedSettings, setupLogger)
}
