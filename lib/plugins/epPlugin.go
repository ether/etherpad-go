package plugins

import (
	"embed"

	"github.com/ether/etherpad-go/lib/hooks"
	"go.uber.org/zap"
)

type EpPlugin interface {
	Name() string
	Description() string
	Init(hookSystem *hooks.Hook, uiAssets embed.FS, zap *zap.SugaredLogger)
	SetEnabled(enabled bool)
	IsEnabled() bool
}
