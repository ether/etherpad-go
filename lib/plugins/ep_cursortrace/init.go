package ep_cursortrace

import (
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
)

type EpCursortracePlugin struct {
	enabled bool
}

func (p *EpCursortracePlugin) Name() string { return "ep_cursortrace" }
func (p *EpCursortracePlugin) Description() string {
	return "Show cursor/caret movements of other users in real time"
}
func (p *EpCursortracePlugin) SetEnabled(e bool) { p.enabled = e }
func (p *EpCursortracePlugin) IsEnabled() bool   { return p.enabled }

func (p *EpCursortracePlugin) Init(store *interfaces.EpPluginStore) {
	store.Logger.Info("Initializing ep_cursortrace plugin")

	// Translation hook
	store.HookSystem.EnqueueGetPluginTranslationHooks(func(ctx *events.LocaleLoadContext) {
		store.Logger.Debugf(
			"Loading ep_cursortrace translations for locale: %s",
			ctx.RequestedLocale,
		)

		loadedTranslations, err := utils.LoadPluginTranslations(
			ctx.RequestedLocale,
			store.UIAssets,
			"ep_cursortrace",
		)
		if err != nil {
			return
		}

		for k, v := range loadedTranslations {
			ctx.LoadedTranslations[k] = v
		}
	})
}

var _ interfaces.EpPlugin = (*EpCursortracePlugin)(nil)
