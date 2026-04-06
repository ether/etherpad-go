package ep_print

import (
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
)

type EpPrintPlugin struct {
	enabled bool
}

func (p *EpPrintPlugin) Name() string {
	return "ep_print"
}

func (p *EpPrintPlugin) SetEnabled(enabled bool) {
	p.enabled = enabled
}

func (p *EpPrintPlugin) IsEnabled() bool {
	return p.enabled
}

func (p *EpPrintPlugin) Description() string {
	return "Adds print support with a toolbar button and print stylesheet"
}

func (p *EpPrintPlugin) Init(store *interfaces.EpPluginStore) {
	store.Logger.Info("Initializing ep_print plugin")

	store.HookSystem.EnqueueGetPluginTranslationHooks(
		func(ctx *events.LocaleLoadContext) {
			store.Logger.Debugf(
				"Loading ep_print translations for locale: %s",
				ctx.RequestedLocale,
			)

			loadedTranslations, err := utils.LoadPluginTranslations(
				ctx.RequestedLocale,
				store.UIAssets,
				"ep_print",
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

var _ interfaces.EpPlugin = (*EpPrintPlugin)(nil)
