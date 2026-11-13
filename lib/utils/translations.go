package utils

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
)

const baseUiAssetsDir = "assets/locales/"

func LoadPluginTranslations(language string, uiAssets embed.FS, pluginName string) (map[string]string, error) {
	if language == "" || strings.Contains(language, "/") || strings.Contains(language, "\\") {
		language = "en"
	}

	content, err := uiAssets.ReadFile(baseUiAssetsDir + pluginName + "/" + language + ".json")
	if err != nil {
		content, _ = uiAssets.ReadFile(baseUiAssetsDir + pluginName + "/en.json")
	}
	var keyValues map[string]interface{}
	if err := json.Unmarshal(content, &keyValues); err != nil {
		return nil, fmt.Errorf("parse translations: %w", err)
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

func LoadTranslations(language string, uiAssets embed.FS, hooks *hooks.Hook) (map[string]string, error) {
	if language == "" || strings.Contains(language, "/") || strings.Contains(language, "\\") {
		language = "en"
	}

	// A language without a file (e.g. from a stale cookie) gets English,
	// not an error page.
	content, err := uiAssets.ReadFile(baseUiAssetsDir + language + ".json")
	if err != nil {
		content, err = uiAssets.ReadFile(baseUiAssetsDir + "en.json")
		language = "en"
	}
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	pluginTranslations := make(map[string]string)
	localeLoadContext := events.LocaleLoadContext{
		LoadedTranslations: pluginTranslations,
		RequestedLocale:    language,
	}

	hooks.ExecuteGetPluginTranslationHooks(&localeLoadContext)

	var keyValues map[string]interface{}

	if err := json.Unmarshal(content, &keyValues); err != nil {
		return nil, fmt.Errorf("parse translations: %w", err)
	}

	delete(keyValues, "@metadata")

	// English fills in what the language has no translation for, so a new
	// label shows in English instead of as an empty string.
	if language != "en" {
		if enContent, err := uiAssets.ReadFile(baseUiAssetsDir + "en.json"); err == nil {
			var en map[string]interface{}
			if json.Unmarshal(enContent, &en) == nil {
				for k, v := range en {
					if _, ok := keyValues[k]; !ok && k != "@metadata" {
						keyValues[k] = v
					}
				}
			}
		}
	}

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

	// Merge plugin translations
	for k, v := range pluginTranslations {
		out[k] = v
	}

	return out, nil
}
