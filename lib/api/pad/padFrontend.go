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
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"

	padAsset "github.com/ether/etherpad-go/assets/pad"
)

var AvailableFonts = []string{
	"Quicksand",
	"Roboto",
	"Alegreya",
	"PlayfairDisplay",
	"Montserrat",
	"OpenDyslexic",
	"RobotoMono",
}

func HandlePadOpen(c fiber.Ctx, uiAssets embed.FS, retrievedSettings *settings.Settings, hooks *hooks.Hook) error {
	pad := models.Model{
		Name: "test",
	}

	var language = c.Cookies("language", "en")

	var keyValues, err = utils.LoadTranslations(language, uiAssets, hooks)
	if err != nil {
		return err
	}

	jsFilePath := "/js/pad/assets/pad.js?v=" + strconv.Itoa(utils.RandomVersionString)
	buttonGroups := plugins.GetToolbarButtonGroups()
	slices.SortFunc(buttonGroups, func(a, b plugins.ToolbarButtonGroup) int {
		return cmp.Compare(a.PluginName, b.PluginName)
	})
	settingsMenuGroups := plugins.GetSettingsMenuGroups()

	padComp := padAsset.PadIndex(pad, jsFilePath, keyValues, retrievedSettings, AvailableFonts, buttonGroups, settingsMenuGroups)

	return adaptor.HTTPHandler(templ.Handler(padComp))(c)
}
