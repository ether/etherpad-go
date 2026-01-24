package ep_spellcheck

import (
	"embed"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/utils"
	"go.uber.org/zap"
)

func InitPlugin(hookSystem *hooks.Hook, uiAssets embed.FS, zap *zap.SugaredLogger) {
	zap.Info("Initializing ep_spellcheck plugin")
	hookSystem.EnqueueGetPluginTranslationHooks(func(ctx *events.LocaleLoadContext) {
		zap.Debugf("Loading ep_spellcheck translations for locale: %s", ctx.RequestedLocale)
		var loadedTranslations, err = utils.LoadPluginTranslations(ctx.RequestedLocale, uiAssets, "ep_spellcheck")
		if err != nil {
			return
		}
		for k, v := range loadedTranslations {
			ctx.LoadedTranslations[k] = v
		}
	})
}
