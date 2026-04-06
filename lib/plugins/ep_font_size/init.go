package ep_font_size

import (
	"regexp"
	"strings"

	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
)

var dataFontSizeRegex = regexp.MustCompile(`data-font-size=["']([0-9a-zA-Z]+)["']`)

type EpFontSizePlugin struct {
	enabled bool
}

func (p *EpFontSizePlugin) Name() string        { return "ep_font_size" }
func (p *EpFontSizePlugin) Description() string { return "Adds font size support to the editor" }
func (p *EpFontSizePlugin) SetEnabled(e bool)   { p.enabled = e }
func (p *EpFontSizePlugin) IsEnabled() bool     { return p.enabled }

func (p *EpFontSizePlugin) Init(store *interfaces.EpPluginStore) {
	store.Logger.Info("Initializing ep_font_size plugin")

	// HTML export: convert data-font-size="x" to class="font-size:x"
	store.HookSystem.EnqueueHook("getLineHTMLForExport", func(ctx any) {
		event := ctx.(*events.LineHtmlForExportContext)
		if event.LineContent == nil {
			return
		}
		*event.LineContent = dataFontSizeRegex.ReplaceAllStringFunc(*event.LineContent, func(match string) string {
			sub := dataFontSizeRegex.FindStringSubmatch(match)
			if len(sub) < 2 {
				return match
			}
			return `class="font-size:` + strings.ToLower(sub[1]) + `"`
		})
	})

	// Translation hook
	store.HookSystem.EnqueueGetPluginTranslationHooks(func(ctx *events.LocaleLoadContext) {
		loaded, err := utils.LoadPluginTranslations(ctx.RequestedLocale, store.UIAssets, "ep_font_size")
		if err != nil {
			return
		}
		for k, v := range loaded {
			ctx.LoadedTranslations[k] = v
		}
	})
}

var _ interfaces.EpPlugin = (*EpFontSizePlugin)(nil)
