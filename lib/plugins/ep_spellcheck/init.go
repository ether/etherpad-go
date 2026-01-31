package ep_spellcheck

import (
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
)

type EpSpellcheckPlugin struct {
	enabled bool
}

func (p *EpSpellcheckPlugin) Name() string {
	return "ep_spellcheck"
}

func (p *EpSpellcheckPlugin) SetEnabled(enabled bool) {
	p.enabled = enabled
}

func (p *EpSpellcheckPlugin) IsEnabled() bool {
	return p.enabled
}

func (p *EpSpellcheckPlugin) Description() string {
	return "Adds spellchecking support to Etherpad"
}

func (p *EpSpellcheckPlugin) Init(store *interfaces.EpPluginStore) {
	store.Logger.Info("Initializing ep_spellcheck plugin")

	store.HookSystem.EnqueueGetPluginTranslationHooks(
		func(ctx *events.LocaleLoadContext) {
			store.Logger.Debugf(
				"Loading ep_spellcheck translations for locale: %s",
				ctx.RequestedLocale,
			)

			loadedTranslations, err := utils.LoadPluginTranslations(
				ctx.RequestedLocale,
				store.UIAssets,
				"ep_spellcheck",
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

var _ interfaces.EpPlugin = (*EpSpellcheckPlugin)(nil)
