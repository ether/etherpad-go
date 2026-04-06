package interfaces

import (
	"embed"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
	"go.uber.org/zap"
)

type EpPluginStore struct {
	Logger            *zap.SugaredLogger
	HookSystem        *hooks.Hook
	UIAssets          embed.FS
	PadManager        *pad.Manager
	App               *fiber.App
	RetrievedSettings *settings.Settings
}
