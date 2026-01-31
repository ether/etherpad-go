package ep_markdown

import (
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
)

type EpMarkdownPlugin struct {
	enabled bool
}

func (p *EpMarkdownPlugin) Name() string {
	return "ep_markdown"
}

func (p *EpMarkdownPlugin) SetEnabled(enabled bool) {
	p.enabled = enabled
}

func (p *EpMarkdownPlugin) IsEnabled() bool {
	return p.enabled
}

func (p *EpMarkdownPlugin) Description() string {
	return "Adds Markdown support to Etherpad"
}

func (p *EpMarkdownPlugin) Init(store *interfaces.EpPluginStore) {
	store.Logger.Info("Initializing ep_markdown plugin")

	store.HookSystem.EnqueueGetPluginTranslationHooks(
		func(ctx *events.LocaleLoadContext) {
			store.Logger.Infof(
				"Loading ep_markdown translations for locale: %s",
				ctx.RequestedLocale,
			)

			loadedTranslations, err := utils.LoadPluginTranslations(
				ctx.RequestedLocale,
				store.UIAssets,
				"ep_markdown",
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

var _ interfaces.EpPlugin = (*EpMarkdownPlugin)(nil)
