package ep_spellcheck

import (
	"embed"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/utils"
)

func InitPlugin(hookSystem *hooks.Hook, uiAssets embed.FS) {
	hookSystem.EnqueueGetPluginTranslationHooks(func(ctx *events.LocaleLoadContext) {
		println("Loading ep_spellcheck translations for locale:", ctx.RequestedLocale)
		var loadedTranslations, err = utils.LoadPluginTranslations(ctx.RequestedLocale, uiAssets, "ep_spellcheck")
		if err != nil {
			return
		}
		for k, v := range loadedTranslations {
			ctx.LoadedTranslations[k] = v
		}
	})
}
