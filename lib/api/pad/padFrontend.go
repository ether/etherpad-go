package pad

import (
	"cmp"
	"embed"
	"slices"
	"strconv"

	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/models"
	"github.com/ether/etherpad-go/lib/plugins"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/socialmeta"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
)
import padAsset "github.com/ether/etherpad-go/assets/pad"

var AvailableFonts = []string{
	"Quicksand",
	"Roboto",
	"Alegreya",
	"PlayfairDisplay",
	"Montserrat",
	"OpenDyslexic",
	"RobotoMono",
}

func HandlePadOpen(c fiber.Ctx, uiAssets embed.FS, retrievedSettings *settings.Settings, hooks2 *hooks.Hook) error {
	padName := c.Params("pad")
	pad := models.Model{
		Name: padName,
	}

	var language = c.Cookies("language", "en")

	var keyValues, err = utils.LoadTranslations(language, uiAssets, hooks2)
	if err != nil {
		return err
	}

	jsFilePath := "/js/pad/assets/pad.js?v=" + strconv.Itoa(utils.RandomVersionString)
	buttonGroups := plugins.GetToolbarButtonGroups()
	slices.SortFunc(buttonGroups, func(a, b plugins.ToolbarButtonGroup) int {
		return cmp.Compare(a.PluginName, b.PluginName)
	})
	settingsMenuGroups := plugins.GetSettingsMenuGroups()

	socialMetaHTML := socialmeta.Render(socialmeta.Opts{
		Req:            buildRequestInfo(c),
		Settings:       socialMetaSettings(retrievedSettings),
		AvailableLangs: availableLangsSet(),
		Locales:        hooks.AllLocales,
		Kind:           socialmeta.KindPad,
		PadName:        padName,
	})

	padComp := padAsset.PadIndex(pad, jsFilePath, keyValues, retrievedSettings, AvailableFonts, buttonGroups, settingsMenuGroups, socialMetaHTML)

	return adaptor.HTTPHandler(templ.Handler(padComp))(c)
}

// buildRequestInfo translates a fiber.Ctx into the request facts socialmeta
// needs. Kept local so the socialmeta package stays router-agnostic.
func buildRequestInfo(c fiber.Ctx) socialmeta.RequestInfo {
	scheme := "http"
	if c.Protocol() == "https" {
		scheme = "https"
	}
	return socialmeta.RequestInfo{
		Scheme:         scheme,
		Host:           string(c.Request().Host()),
		Path:           c.Path(),
		AcceptLanguage: c.Get("Accept-Language"),
	}
}

func socialMetaSettings(s *settings.Settings) socialmeta.Settings {
	favicon := ""
	if s.Favicon != nil {
		favicon = *s.Favicon
	}
	return socialmeta.Settings{
		Title:               s.Title,
		Favicon:             favicon,
		PublicURL:           s.PublicURL,
		DescriptionOverride: s.SocialMeta.Description,
	}
}

func availableLangsSet() map[string]struct{} {
	out := make(map[string]struct{}, len(hooks.AvailableLangs))
	for k := range hooks.AvailableLangs {
		out[k] = struct{}{}
	}
	return out
}
