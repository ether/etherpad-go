package plugins

import (
	"embed"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/plugins/ep_align"
	"github.com/ether/etherpad-go/lib/plugins/ep_markdown"
	"github.com/ether/etherpad-go/lib/plugins/ep_rss"
	"github.com/ether/etherpad-go/lib/plugins/ep_spellcheck"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func InitPlugins(app *fiber.App, s *settings.Settings, padManager *pad.Manager, hookSystem *hooks.Hook, zap *zap.SugaredLogger, uiAssets embed.FS) {
	if _, ok := s.Plugins["ep_align"]; ok {
		if s.Plugins["ep_align"].Enabled {
			zap.Info("Loading ep_align")
			ep_align.InitPlugin(hookSystem, uiAssets, zap)
		}
	}

	if _, ok := s.Plugins["ep_spellcheck"]; ok {
		if s.Plugins["ep_spellcheck"].Enabled {
			zap.Info("Loading ep_spellcheck")
			ep_spellcheck.InitPlugin(hookSystem, uiAssets, zap)
		}
	}

	if _, ok := s.Plugins["ep_markdown"]; ok {
		if s.Plugins["ep_markdown"].Enabled {
			zap.Info("Loading ep_markdown")
			ep_markdown.InitPlugin(hookSystem, uiAssets, zap)
		}
	}

	if _, ok := s.Plugins["ep_rss"]; ok {
		if s.Plugins["ep_rss"].Enabled {
			zap.Info("Loading ep_rss")
			ep_rss.RegisterFeedRoutes(app, padManager, zap)
		}
	}
}
