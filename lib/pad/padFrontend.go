package pad

import (
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/lib/models"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
)
import padAsset "github.com/ether/etherpad-go/assets/pad"

func LoadTranslations(language string, uiAssets embed.FS) (map[string]string, error) {
	if language == "" || strings.Contains(language, "/") || strings.Contains(language, "\\") {
		language = "en"
	}

	content, err := uiAssets.ReadFile("assets/locales/" + language + ".json")
	if err != nil {
		content, _ = uiAssets.ReadFile("assets/locales/en.json")
	}

	var keyValues map[string]interface{}

	if err := json.Unmarshal(content, &keyValues); err != nil {
		println(err.Error())
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	delete(keyValues, "@metadata")

	out := make(map[string]string, len(keyValues))
	for k, v := range keyValues {
		switch val := v.(type) {
		case string:
			out[k] = val
		default:
			b, err := json.Marshal(val)
			if err != nil {
				out[k] = ""
			} else {
				out[k] = string(b)
			}
		}
	}

	return out, nil
}

func HandlePadOpen(c *fiber.Ctx, uiAssets embed.FS) error {
	pad := models.Model{
		Name: "test",
	}

	var language = c.Cookies("language", "en")
	var keyValues, err = LoadTranslations(language, uiAssets)
	if err != nil {
		return err
	}

	jsFilePath := "/js/pad/assets/pad.js?v=" + strconv.Itoa(utils.RandomVersionString)

	padComp := padAsset.Greeting(pad, jsFilePath, keyValues)

	return adaptor.HTTPHandler(templ.Handler(padComp))(c)
}
