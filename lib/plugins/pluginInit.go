package plugins

import (
	"embed"
	"slices"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/plugins/ep_align"
	"github.com/ether/etherpad-go/lib/plugins/ep_markdown"
	"github.com/ether/etherpad-go/lib/plugins/ep_spellcheck"
	"github.com/ether/etherpad-go/lib/settings"
	"go.uber.org/zap"
)

var RegisteredPlugins = []EpPlugin{
	&ep_align.EpAlignPlugin{},
	&ep_spellcheck.EpSpellcheckPlugin{},
	&ep_markdown.EpMarkdownPlugin{},
}

func InitPlugins(app *fiber.App, s *settings.Settings, hookSystem *hooks.Hook, zap *zap.SugaredLogger, uiAssets embed.FS) {
	if _, ok := s.Plugins["ep_align"]; ok {
		if s.Plugins["ep_align"].Enabled {
			zap.Info("Loading ep_align")
			ep_align.InitPlugin(hookSystem, uiAssets, zap)
		}
	}
	for _, plugin := range RegisteredPlugins {
		if slices.Contains(enabledPlugins, plugin.Name()) {
			zap.Infof("Loading plugin: %s", plugin.Name())
			plugin.Init(hookSystem, uiAssets, zap)
			plugin.SetEnabled(true)
		}
	}

	if _, ok := s.Plugins["ep_markdown"]; ok {
		if s.Plugins["ep_markdown"].Enabled {
			zap.Info("Loading ep_markdown")
			ep_markdown.InitPlugin(hookSystem, uiAssets, zap)
		}
	}
}
