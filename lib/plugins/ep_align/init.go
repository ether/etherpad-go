package ep_align

import (
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
)

type EpAlignPlugin struct {
	enabled bool
}

func (p *EpAlignPlugin) Name() string {
	return "ep_align"
}

func (p *EpAlignPlugin) SetEnabled(enabled bool) {
	p.enabled = enabled
}

func (p *EpAlignPlugin) IsEnabled() bool {
	return p.enabled
}

func (p *EpAlignPlugin) Enabled() bool {
	return true
}

func (p *EpAlignPlugin) Description() string {
	return "Adds text alignment support and export handling"
}

func (p *EpAlignPlugin) Init(epPluginStore *interfaces.EpPluginStore) {
	epPluginStore.Logger.Info("Initializing ep_align plugin")

	// HTML Export hook
	epPluginStore.HookSystem.EnqueueHook(
		"getLineHTMLForExport",
		func(ctx any) {
			event := ctx.(*events.LineHtmlForExportContext)
			GetLineHTMLForExport(event)
		},
	)

	// PDF Export hook
	epPluginStore.HookSystem.EnqueueHook(
		"getLinePDFForExport",
		func(ctx any) {
			event := ctx.(*events.LinePDFForExportContext)
			GetLinePDFForExport(event)
		},
	)

	// DOCX Export hook
	epPluginStore.HookSystem.EnqueueHook(
		"getLineDocxForExport",
		func(ctx any) {
			event := ctx.(*events.LineDocxForExportContext)
			GetLineDocxForExport(event)
		},
	)

	// ODT Export hook
	epPluginStore.HookSystem.EnqueueHook(
		"getLineOdtForExport",
		func(ctx any) {
			event := ctx.(*events.LineOdtForExportContext)
			GetLineOdtForExport(event)
		},
	)

	// Translation hook
	epPluginStore.HookSystem.EnqueueGetPluginTranslationHooks(
		func(ctx *events.LocaleLoadContext) {
			loadedTranslations, err := utils.LoadPluginTranslations(
				ctx.RequestedLocale,
				epPluginStore.UIAssets,
				"ep_align",
			)
			if err != nil {
				return
			}

			epPluginStore.Logger.Debugf(
				"Loading ep_align translations for locale: %s",
				ctx.RequestedLocale,
			)

			for k, v := range loadedTranslations {
				ctx.LoadedTranslations[k] = v
			}
		},
	)
}

var _ interfaces.EpPlugin = (*EpAlignPlugin)(nil)
