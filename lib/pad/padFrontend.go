package pad

import (
	"embed"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/lib/models"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
)
import padAsset "github.com/ether/etherpad-go/assets/pad"

func HandlePadOpen(c *fiber.Ctx, uiAssets embed.FS) error {
	pad := models.Model{
		Name: "test",
	}

	var language = c.Cookies("language")
	if language == "" || strings.Contains(language, "/") || strings.Contains(language, "\\") {
		language = "en"
	}

	content, err := uiAssets.ReadFile("assets/locales/" + language + ".json")
	if err != nil {
		content, _ = uiAssets.ReadFile("assets/locales/en.json")
	}

	var keyValues map[string]string

	if err := json.Unmarshal(content, &keyValues); err != nil {
		println(err.Error())
		return err
	}

	jsFilePath := "/js/pad/assets/pad.js?v=" + strconv.Itoa(utils.RandomVersionString)

	padComp := padAsset.Greeting(pad, jsFilePath, keyValues)

	return adaptor.HTTPHandler(templ.Handler(padComp))(c)
}
