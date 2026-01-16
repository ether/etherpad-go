package plugins

import (
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/plugins/ep_align"
	"github.com/ether/etherpad-go/lib/settings"
)

func InitPlugins(s *settings.Settings, hookSystem *hooks.Hook) {
	if _, ok := s.Plugins["ep_align"]; ok {
		if s.Plugins["ep_align"].Enabled {
			ep_align.InitPlugin(hookSystem)
		}
	}
}
