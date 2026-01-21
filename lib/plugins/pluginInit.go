package plugins

import (
	"embed"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/plugins/ep_align"
	"github.com/ether/etherpad-go/lib/plugins/ep_spellcheck"
	"github.com/ether/etherpad-go/lib/settings"
	"go.uber.org/zap"
)

func InitPlugins(s *settings.Settings, hookSystem *hooks.Hook, zap *zap.SugaredLogger, uiAssets embed.FS) {
	if _, ok := s.Plugins["ep_align"]; ok {
		if s.Plugins["ep_align"].Enabled {
			ep_align.InitPlugin(hookSystem, uiAssets)
		}
	}

	if _, ok := s.Plugins["ep_spellcheck"]; ok {
		if s.Plugins["ep_spellcheck"].Enabled {
			ep_spellcheck.InitPlugin(hookSystem, uiAssets)
		}
	}
}
