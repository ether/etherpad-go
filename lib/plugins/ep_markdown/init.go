package ep_markdown

import (
	"embed"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/utils"
	"go.uber.org/zap"
)

func InitPlugin(hookSystem *hooks.Hook, uiAssets embed.FS, zap *zap.SugaredLogger) {
	hookSystem.EnqueueGetPluginTranslationHooks(func(ctx *events.LocaleLoadContext) {
		zap.Infof("Loading ep_markdown translations for locale: %s", ctx.RequestedLocale)
		var loadedTranslations, err = utils.LoadPluginTranslations(ctx.RequestedLocale, uiAssets, "ep_markdown")
		if err != nil {
			return
		}
		for k, v := range loadedTranslations {
			ctx.LoadedTranslations[k] = v
		}
	})
}
