package lib

import (
	"embed"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	pad2 "github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"go.uber.org/zap"
)

type InitStore struct {
	C                 *fiber.App
	UiAssets          embed.FS
	RetrievedSettings *settings.Settings
	CookieStore       *session.Store
	Store             db.DataStore
	Handler           *ws.PadMessageHandler
	PadManager        *pad2.Manager
	Validator         *validator.Validate
	Logger            *zap.SugaredLogger
	Hooks             *hooks.Hook
	ReadOnlyManager   *pad2.ReadOnlyManager
	SecurityManager   *pad2.SecurityManager
}
