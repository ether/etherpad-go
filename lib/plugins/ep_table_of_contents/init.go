package ep_table_of_contents

import (
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
)

type EpTableOfContentsPlugin struct {
	enabled bool
}

func (p *EpTableOfContentsPlugin) Name() string {
	return "ep_table_of_contents"
}

func (p *EpTableOfContentsPlugin) Description() string {
	return "Adds a sidebar Table of Contents based on headings"
}

func (p *EpTableOfContentsPlugin) SetEnabled(enabled bool) {
	p.enabled = enabled
}

func (p *EpTableOfContentsPlugin) IsEnabled() bool {
	return p.enabled
}

func (p *EpTableOfContentsPlugin) Init(store *interfaces.EpPluginStore) {
	store.Logger.Info("Initializing ep_table_of_contents plugin")

	store.HookSystem.EnqueueGetPluginTranslationHooks(
		func(ctx *events.LocaleLoadContext) {
			store.Logger.Debugf(
				"Loading ep_table_of_contents translations for locale: %s",
				ctx.RequestedLocale,
			)

			loadedTranslations, err := utils.LoadPluginTranslations(
				ctx.RequestedLocale,
				store.UIAssets,
				"ep_table_of_contents",
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

var _ interfaces.EpPlugin = (*EpTableOfContentsPlugin)(nil)
