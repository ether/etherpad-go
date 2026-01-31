package ep_heading

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
	"go.uber.org/zap"
)

type EpHeadingsPlugin struct {
	enabled bool
	logger  *zap.SugaredLogger
}

func (e *EpHeadingsPlugin) Name() string {
	return "ep_heading"
}

func (e *EpHeadingsPlugin) Description() string {
	return "Adds support for headings in pads"
}

func (e *EpHeadingsPlugin) analyzeLine(alineAttrs *string, apool *apool.APool) *string {
	var header *string
	if alineAttrs != nil {
		ops, err := changeset.DeserializeOps(*alineAttrs)
		if err != nil {
			e.logger.Warnw("Failed to deserialize ops", "ops", alineAttrs)
			return nil
		}
		for _, op := range *ops {
			attributeMap := changeset.FromString(op.Attribs, apool)
			header = attributeMap.Get("heading")
		}
	}
	return header
}

type EpHeadingsExportContext struct {
	AttribLine  *string
	APool       *apool.APool
	text        string
	LineContent string
}

func (e *EpHeadingsPlugin) getLineHTMLForExport(ctx *events.LineHtmlForExportContext) {
	header := e.analyzeLine(ctx.AttribLine, ctx.Apool)
	if header == nil {
		return
	}

	if strings.HasPrefix(*ctx.Text, "*") {
		lineContent := strings.Replace(*ctx.LineContent, "*", "", 1)
		ctx.LineContent = &lineContent
	}

	paragraphRegex := regexp.MustCompile(`<p([^>]+)?>`)
	paragraph := paragraphRegex.FindString(*ctx.LineContent)

	if paragraph != "" {
		lineContent := strings.Replace(*ctx.LineContent, "<p", fmt.Sprintf("<%s ", *header), 1)
		lineContent = strings.Replace(*ctx.LineContent, "</p>", fmt.Sprintf("</%s>", *header), 1)
		ctx.LineContent = &lineContent
	} else {
		lineContent := fmt.Sprintf("<%s>%s</%s>", *header, *ctx.LineContent, *header)
		ctx.LineContent = &lineContent
	}
}

func (e *EpHeadingsPlugin) Init(store *interfaces.EpPluginStore) {
	// Nothing to do on the server
	e.logger = store.Logger

	// HTML Export hook
	store.HookSystem.EnqueueHook(
		"getLineHTMLForExport",
		func(ctx any) {
			event := ctx.(*events.LineHtmlForExportContext)
			e.getLineHTMLForExport(event)
		},
	)

	// Translation hook
	store.HookSystem.EnqueueGetPluginTranslationHooks(
		func(ctx *events.LocaleLoadContext) {
			loadedTranslations, err := utils.LoadPluginTranslations(
				ctx.RequestedLocale,
				store.UIAssets,
				"ep_headings",
			)
			if err != nil {
				return
			}

			store.Logger.Debugf(
				"Loading ep_align translations for locale: %s",
				ctx.RequestedLocale,
			)

			for k, v := range loadedTranslations {
				ctx.LoadedTranslations[k] = v
			}
		},
	)

	return
}

func (e *EpHeadingsPlugin) SetEnabled(enabled bool) {
	e.enabled = enabled
}

func (e *EpHeadingsPlugin) IsEnabled() bool {
	return e.enabled
}

var _ interfaces.EpPlugin = (*EpHeadingsPlugin)(nil)
