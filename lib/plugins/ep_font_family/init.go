package ep_font_family

import (
	"regexp"
	"strings"

	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
)

var dataFontFamilyRegex = regexp.MustCompile(`data-font-family=["']([0-9a-zA-Z-]+)["']`)

var fontFamilyMap = map[string]string{
	"arial":           "Arial, sans-serif",
	"avant-garde":     "\"Avant Garde\", sans-serif",
	"bookman":         "\"Bookman Old Style\", Bookman, serif",
	"calibri":         "Calibri, \"Gill Sans\", \"Gill Sans MT\", sans-serif",
	"courier":         "\"Courier New\", Courier, monospace",
	"garamond":        "Garamond, \"Hoefler Text\", serif",
	"helvetica":       "Helvetica, Arial, sans-serif",
	"monospace":       "\"Courier New\", Courier, monospace",
	"palatino":        "\"Palatino Linotype\", Palatino, \"Book Antiqua\", serif",
	"times-new-roman": "\"Times New Roman\", Times, serif",
}

type EpFontFamilyPlugin struct {
	enabled bool
}

func (p *EpFontFamilyPlugin) Name() string        { return "ep_font_family" }
func (p *EpFontFamilyPlugin) Description() string { return "Adds font family support to the editor" }
func (p *EpFontFamilyPlugin) SetEnabled(e bool)   { p.enabled = e }
func (p *EpFontFamilyPlugin) IsEnabled() bool     { return p.enabled }

func (p *EpFontFamilyPlugin) Init(store *interfaces.EpPluginStore) {
	store.Logger.Info("Initializing ep_font_family plugin")

	// HTML export: convert data-font-family="x" to style="font-family: ..."
	store.HookSystem.EnqueueHook("getLineHTMLForExport", func(ctx any) {
		event := ctx.(*events.LineHtmlForExportContext)
		if event.LineContent == nil {
			return
		}
		*event.LineContent = dataFontFamilyRegex.ReplaceAllStringFunc(*event.LineContent, func(match string) string {
			sub := dataFontFamilyRegex.FindStringSubmatch(match)
			if len(sub) < 2 {
				return match
			}
			fontName := strings.ToLower(sub[1])
			if family, ok := fontFamilyMap[fontName]; ok {
				return `style="font-family: ` + family + `"`
			}
			return match
		})
	})

	// Translation hook
	store.HookSystem.EnqueueGetPluginTranslationHooks(func(ctx *events.LocaleLoadContext) {
		loaded, err := utils.LoadPluginTranslations(ctx.RequestedLocale, store.UIAssets, "ep_font_family")
		if err != nil {
			return
		}
		for k, v := range loaded {
			ctx.LoadedTranslations[k] = v
		}
	})
}

var _ interfaces.EpPlugin = (*EpFontFamilyPlugin)(nil)
