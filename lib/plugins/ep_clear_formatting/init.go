package ep_clear_formatting

import (
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
)

type EpClearFormattingPlugin struct {
	enabled bool
}

func (p *EpClearFormattingPlugin) Name() string {
	return "ep_clear_formatting"
}

func (p *EpClearFormattingPlugin) SetEnabled(enabled bool) {
	p.enabled = enabled
}

func (p *EpClearFormattingPlugin) IsEnabled() bool {
	return p.enabled
}

func (p *EpClearFormattingPlugin) Description() string {
	return "Clears all text formatting from the selected text"
}

func (p *EpClearFormattingPlugin) Init(store *interfaces.EpPluginStore) {
	store.Logger.Info("Initializing ep_clear_formatting plugin")

	store.HookSystem.EnqueueGetPluginTranslationHooks(
		func(ctx *events.LocaleLoadContext) {
			store.Logger.Debugf(
				"Loading ep_clear_formatting translations for locale: %s",
				ctx.RequestedLocale,
			)

			loadedTranslations, err := utils.LoadPluginTranslations(
				ctx.RequestedLocale,
				store.UIAssets,
				"ep_clear_formatting",
			)
			if err != nil {
				return
			}

			for k, v := range loadedTranslations {
				ctx.LoadedTranslations[k] = v
			}
		},
	)
}

var _ interfaces.EpPlugin = (*EpClearFormattingPlugin)(nil)
