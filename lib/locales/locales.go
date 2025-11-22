package locales

import (
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/gofiber/fiber/v2"
)

var Locales map[string]interface{}

func init() {
	Locales = make(map[string]interface{})
	files, _ := os.ReadDir("./assets/locales")
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileName := file.Name()
		Locales[strings.Replace(fileName, ".json", "", -1)] = `locales/` + fileName
		content, _ := os.ReadFile("./assets/locales/en.json")

		var enMap = make(map[string]string)
		json.Unmarshal(content, &enMap)
		Locales["en"] = enMap
	}
}

var locationsToSearchFor = []string{
	".", "ep_admin_pads",
}

type LocaleMap struct {
	Locale  map[string]string
	DiffKey string
}

func HandleLocale(c *fiber.Ctx, uiAssets embed.FS) error {
	locale := c.Params("locale")
	localeToSearchFor := strings.Replace(locale, ".json", "", -1)
	if _, ok := Locales[localeToSearchFor]; !ok {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Locale not found",
		})
	}

	prefixPath := "./assets/locales/"

	var possibleMaps = make([]LocaleMap, 0)

	for _, location := range locationsToSearchFor {
		absLocation := path.Join(prefixPath, location, locale)
		fileContent, err := fs.ReadFile(uiAssets, absLocation)
		if err != nil {
			continue
		}
		var raw map[string]interface{}

		var diffKey string
		if location != "." {
			diffKey = location
		}

		var languageMap = LocaleMap{
			Locale:  make(map[string]string),
			DiffKey: diffKey,
		}
		if err := json.Unmarshal(fileContent, &raw); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Could not parse locale file",
			})
		}
		for k, v := range raw {
			if s, ok := v.(string); ok {
				languageMap.Locale[k] = s
			}
		}

		possibleMaps = append(possibleMaps, languageMap)
	}

	finalMap := make(map[string]interface{})
	for _, m := range possibleMaps {
		for k, v := range m.Locale {
			if m.DiffKey != "" {
				if _, ok := finalMap[m.DiffKey]; !ok {
					finalMap[m.DiffKey] = make(map[string]string)
				}
				diffMap := finalMap[m.DiffKey].(map[string]string)
				diffMap[k] = v
				continue
			}
			finalMap[k] = v
		}
	}

	return c.Status(fiber.StatusOK).JSON(finalMap)
}
