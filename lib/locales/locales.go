package locales

import (
	"embed"
	"encoding/json"
	"io/fs"
	"path"
	"strings"

	"github.com/ether/etherpad-go/lib"
	"github.com/gofiber/fiber/v3"
)

var Locales map[string]interface{}

func Init(initStore *lib.InitStore) {
	Locales = make(map[string]interface{})
	files, _ := fs.ReadDir(initStore.UiAssets, "assets/locales")
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileName := file.Name()
		Locales[strings.Replace(fileName, ".json", "", -1)] = `locales/` + fileName
		content, err := fs.ReadFile(initStore.UiAssets, "assets/locales/en.json")
		if err != nil {
			initStore.Logger.Warnf("Could not read en.json: %v", err)
			continue
		}

		var enMap = make(map[string]string)
		if err := json.Unmarshal(content, &enMap); err != nil {
			initStore.Logger.Warnf("Could not unmarshal en.json: %v", err)
			continue
		}
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

func buildLocaleFallbacks(locale string) []string {
	locale = strings.TrimSuffix(locale, ".json")
	locale = strings.ReplaceAll(locale, "_", "-")
	locale = strings.ToLower(locale)

	seen := make(map[string]struct{})
	var fallbacks []string

	add := func(l string) {
		if _, ok := seen[l]; ok {
			return
		}
		seen[l] = struct{}{}
		fallbacks = append(fallbacks, l)
	}

	parts := strings.Split(locale, "-")
	if len(parts) > 1 {
		add(locale)
		add(parts[0])
	} else if locale != "" {
		add(locale)
	}

	if locale != "en" {
		add("en")
	}

	return fallbacks
}

func loadLocaleFile(
	fs embed.FS,
	path string,
) (map[string]string, error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		return nil, err
	}

	raw := map[string]interface{}{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	out := make(map[string]string)
	for k, v := range raw {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}

	return out, nil
}

func HandleLocale(c fiber.Ctx, uiAssets embed.FS, prefix string) error {
	requestedLocale := c.Params("locale")

	fallbacks := buildLocaleFallbacks(requestedLocale)
	prefixPath := "assets/locales" + prefix

	finalMap := make(map[string]interface{})
	foundAny := false

	for _, locale := range fallbacks {
		for _, location := range locationsToSearchFor {
			localeFile := locale + ".json"
			filePath := path.Join(prefixPath, location, localeFile)

			localeMap, err := loadLocaleFile(uiAssets, filePath)
			if err != nil {
				continue
			}

			foundAny = true

			// Root locale
			if location == "." {
				for k, v := range localeMap {
					finalMap[k] = v
				}
				continue
			}

			// Namespaced locale
			if _, ok := finalMap[location]; !ok {
				finalMap[location] = make(map[string]string)
			}

			target := finalMap[location].(map[string]string)
			for k, v := range localeMap {
				target[k] = v
			}
			break
		}
	}

	if !foundAny {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "No locale files found",
		})
	}

	return c.JSON(finalMap)
}
