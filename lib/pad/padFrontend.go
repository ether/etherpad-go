package pad

import (
	"embed"
	"strconv"

	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/lib/models"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
)
import padAsset "github.com/ether/etherpad-go/assets/pad"

func HandlePadOpen(c *fiber.Ctx, uiAssets embed.FS, retrievedSettings settings.Settings) error {
	pad := models.Model{
		Name: "test",
	}

	var language = c.Cookies("language", "en")
	var keyValues, err = utils.LoadTranslations(language, uiAssets)
	if err != nil {
		return err
	}

	jsFilePath := "/js/pad/assets/pad.js?v=" + strconv.Itoa(utils.RandomVersionString)

	padComp := padAsset.Greeting(pad, jsFilePath, keyValues, retrievedSettings)

	return adaptor.HTTPHandler(templ.Handler(padComp))(c)
}
