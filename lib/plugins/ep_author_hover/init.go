package ep_author_hover

import (
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
)

type EpAuthorHoverPlugin struct {
	enabled bool
}

func (p *EpAuthorHoverPlugin) Name() string {
	return "ep_author_hover"
}

func (p *EpAuthorHoverPlugin) SetEnabled(enabled bool) {
	p.enabled = enabled
}

func (p *EpAuthorHoverPlugin) IsEnabled() bool {
	return p.enabled
}

func (p *EpAuthorHoverPlugin) Description() string {
	return "Shows author information when hovering over text in the editor"
}

func (p *EpAuthorHoverPlugin) Init(store *interfaces.EpPluginStore) {
	store.Logger.Info("Initializing ep_author_hover plugin")

	store.HookSystem.EnqueueGetPluginTranslationHooks(
		func(ctx *events.LocaleLoadContext) {
			store.Logger.Debugf(
				"Loading ep_author_hover translations for locale: %s",
				ctx.RequestedLocale,
			)

			loadedTranslations, err := utils.LoadPluginTranslations(
				ctx.RequestedLocale,
				store.UIAssets,
				"ep_author_hover",
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

var _ interfaces.EpPlugin = (*EpAuthorHoverPlugin)(nil)
