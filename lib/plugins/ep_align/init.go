package ep_align

import (
	"embed"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/utils"
)

func InitPlugin(hookSystem *hooks.Hook, uiAssets embed.FS) {
	// HTML Export hook
	hookSystem.EnqueueHook("getLineHTMLForExport", func(ctx any) {
		var event = ctx.(*events.LineHtmlForExportContext)
		GetLineHTMLForExport(event)
	})

	// PDF Export hook
	hookSystem.EnqueueHook("getLinePDFForExport", func(ctx any) {
		var event = ctx.(*events.LinePDFForExportContext)
		GetLinePDFForExport(event)
	})

	// DOCX Export hook
	hookSystem.EnqueueHook("getLineDocxForExport", func(ctx any) {
		var event = ctx.(*events.LineDocxForExportContext)
		GetLineDocxForExport(event)
	})

	// ODT Export hook
	hookSystem.EnqueueHook("getLineOdtForExport", func(ctx any) {
		var event = ctx.(*events.LineOdtForExportContext)
		GetLineOdtForExport(event)
	})

	hookSystem.EnqueueGetPluginTranslationHooks(func(ctx *events.LocaleLoadContext) {
		var loadedTranslations, err = utils.LoadPluginTranslations(ctx.RequestedLocale, uiAssets, "ep_align")
		if err != nil {
			return
		}
		for k, v := range loadedTranslations {
			ctx.LoadedTranslations[k] = v
		}
	})
}
