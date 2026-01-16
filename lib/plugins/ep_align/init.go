package ep_align

import (
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
)

func InitPlugin(hookSystem *hooks.Hook) {
	// HTML Export hook
	hookSystem.EnqueueHook("getLineHTMLForExport", func(hookName string, ctx any) {
		var event = ctx.(*events.LineHtmlForExportContext)
		GetLineHTMLForExport(event)
	})

	// PDF Export hook
	hookSystem.EnqueueHook("getLinePDFForExport", func(hookName string, ctx any) {
		var event = ctx.(*events.LinePDFForExportContext)
		GetLinePDFForExport(event)
	})

	// DOCX Export hook
	hookSystem.EnqueueHook("getLineDocxForExport", func(hookName string, ctx any) {
		var event = ctx.(*events.LineDocxForExportContext)
		GetLineDocxForExport(event)
	})

	// ODT Export hook
	hookSystem.EnqueueHook("getLineOdtForExport", func(hookName string, ctx any) {
		var event = ctx.(*events.LineOdtForExportContext)
		GetLineOdtForExport(event)
	})
}
