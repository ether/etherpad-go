package timeslider

import (
	"embed"
	"strconv"

	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/assets/timeslider"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/socialmeta"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
)

func HandleTimesliderOpen(c fiber.Ctx, uiAssets embed.FS, retrievedSettings *settings.Settings, hook *hooks.Hook) error {

	var language = c.Cookies("language", "en")
	var keyValues, err = utils.LoadTranslations(language, uiAssets, hook)
	if err != nil {
		return err
	}

	jsFilePath := "/js/timeslider/assets/timeslider.js?v=" + strconv.Itoa(utils.RandomVersionString)

	padName := c.Params("pad")
	scheme := "http"
	if c.Protocol() == "https" {
		scheme = "https"
	}
	favicon := ""
	if retrievedSettings.Favicon != nil {
		favicon = *retrievedSettings.Favicon
	}
	availableLangsSet := make(map[string]struct{}, len(hooks.AvailableLangs))
	for k := range hooks.AvailableLangs {
		availableLangsSet[k] = struct{}{}
	}
	socialMetaHTML := socialmeta.Render(socialmeta.Opts{
		Req: socialmeta.RequestInfo{
			Scheme:         scheme,
			Host:           string(c.Request().Host()),
			Path:           c.Path(),
			AcceptLanguage: c.Get("Accept-Language"),
		},
		Settings: socialmeta.Settings{
			Title:               retrievedSettings.Title,
			Favicon:             favicon,
			PublicURL:           retrievedSettings.PublicURL,
			DescriptionOverride: retrievedSettings.SocialMeta.Description,
		},
		AvailableLangs: availableLangsSet,
		Locales:        hooks.AllLocales,
		Kind:           socialmeta.KindTimeslider,
		PadName:        padName,
	})

	timesliderComp := timeslider.Timeslider(jsFilePath, keyValues, retrievedSettings, socialMetaHTML)

	return adaptor.HTTPHandler(templ.Handler(timesliderComp))(c)
}
