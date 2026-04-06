package plugins

import (
	"slices"

	"github.com/ether/etherpad-go/lib/plugins/ep_align"
	"github.com/ether/etherpad-go/lib/plugins/ep_author_hover"
	"github.com/ether/etherpad-go/lib/plugins/ep_chat_log_join_leave"
	"github.com/ether/etherpad-go/lib/plugins/ep_clear_formatting"
	"github.com/ether/etherpad-go/lib/plugins/ep_cursortrace"
	"github.com/ether/etherpad-go/lib/plugins/ep_font_color"
	"github.com/ether/etherpad-go/lib/plugins/ep_font_family"
	"github.com/ether/etherpad-go/lib/plugins/ep_font_size"
	"github.com/ether/etherpad-go/lib/plugins/ep_heading"
	"github.com/ether/etherpad-go/lib/plugins/ep_markdown"
	"github.com/ether/etherpad-go/lib/plugins/ep_print"
	"github.com/ether/etherpad-go/lib/plugins/ep_rss"
	"github.com/ether/etherpad-go/lib/plugins/ep_spellcheck"
	"github.com/ether/etherpad-go/lib/plugins/ep_table_of_contents"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
)

var RegisteredPlugins = []interfaces.EpPlugin{
	&ep_align.EpAlignPlugin{},
	&ep_author_hover.EpAuthorHoverPlugin{},
	&ep_chat_log_join_leave.EpChatLogJoinLeavePlugin{},
	&ep_clear_formatting.EpClearFormattingPlugin{},
	&ep_cursortrace.EpCursortracePlugin{},
	&ep_font_color.EpFontColorPlugin{},
	&ep_font_family.EpFontFamilyPlugin{},
	&ep_font_size.EpFontSizePlugin{},
	&ep_spellcheck.EpSpellcheckPlugin{},
	&ep_markdown.EpMarkdownPlugin{},
	&ep_print.EpPrintPlugin{},
	&ep_rss.EPRssPlugin{},
	&ep_heading.EpHeadingsPlugin{},
	&ep_table_of_contents.EpTableOfContentsPlugin{},
}

func InitPlugins(store *interfaces.EpPluginStore) {
	var ts = store.RetrievedSettings.GetAllPlugins()
	enabledPlugins := make([]string, 0)
	for _, pluginSettings := range ts {
		if pluginSettings.Enabled {
			enabledPlugins = append(enabledPlugins, pluginSettings.Name)
		}
	}
	for _, plugin := range RegisteredPlugins {
		if slices.Contains(enabledPlugins, plugin.Name()) {
			store.Logger.Infof("Loading plugin: %s", plugin.Name())
			plugin.Init(store)
			plugin.SetEnabled(true)
		}
	}
}
