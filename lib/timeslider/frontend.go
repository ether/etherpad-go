package timeslider

import (
	"embed"
	"strconv"

	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/assets/timeslider"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
)

func HandleTimesliderOpen(c *fiber.Ctx, uiAssets embed.FS, retrievedSettings *settings.Settings) error {

	var language = c.Cookies("language", "en")
	var keyValues, err = utils.LoadTranslations(language, uiAssets)
	if err != nil {
		return err
	}

	jsFilePath := "/js/timeslider/assets/timeslider.js?v=" + strconv.Itoa(utils.RandomVersionString)

	timesliderComp := timeslider.Timeslider(jsFilePath, keyValues, retrievedSettings)

	return adaptor.HTTPHandler(templ.Handler(timesliderComp))(c)
}
