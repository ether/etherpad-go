package ep_font_color

import (
	"regexp"
	"strings"

	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
)

var dataColorRegex = regexp.MustCompile(`data-color=["']([0-9a-zA-Z]+)["']`)

type EpFontColorPlugin struct {
	enabled bool
}

func (p *EpFontColorPlugin) Name() string        { return "ep_font_color" }
func (p *EpFontColorPlugin) Description() string { return "Adds font color support to the editor" }
func (p *EpFontColorPlugin) SetEnabled(e bool)   { p.enabled = e }
func (p *EpFontColorPlugin) IsEnabled() bool     { return p.enabled }

func (p *EpFontColorPlugin) Init(store *interfaces.EpPluginStore) {
	store.Logger.Info("Initializing ep_font_color plugin")

	// HTML export: convert data-color="x" to class="color:x"
	store.HookSystem.EnqueueHook("getLineHTMLForExport", func(ctx any) {
		event := ctx.(*events.LineHtmlForExportContext)
		if event.LineContent == nil {
			return
		}
		*event.LineContent = dataColorRegex.ReplaceAllStringFunc(*event.LineContent, func(match string) string {
			sub := dataColorRegex.FindStringSubmatch(match)
			if len(sub) < 2 {
				return match
			}
			return `class="color:` + strings.ToLower(sub[1]) + `"`
		})
	})

	// Translation hook
	store.HookSystem.EnqueueGetPluginTranslationHooks(func(ctx *events.LocaleLoadContext) {
		loaded, err := utils.LoadPluginTranslations(ctx.RequestedLocale, store.UIAssets, "ep_font_color")
		if err != nil {
			return
		}
		for k, v := range loaded {
			ctx.LoadedTranslations[k] = v
		}
	})
}

var _ interfaces.EpPlugin = (*EpFontColorPlugin)(nil)
