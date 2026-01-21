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

func LoadTranslations(language string, uiAssets embed.FS, hooks *hooks.Hook) (map[string]string, error) {
	if language == "" || strings.Contains(language, "/") || strings.Contains(language, "\\") {
		language = "en"
	}

	content, err := uiAssets.ReadFile(baseUiAssetsDir + language + ".json")
	if err != nil {
		content, _ = uiAssets.ReadFile("assets/locales/en.json")
	}

	pluginTranslations := make(map[string]string)
	localeLoadContext := events.LocaleLoadContext{
		LoadedTranslations: pluginTranslations,
		RequestedLocale:    language,
	}

	hooks.ExecuteGetPluginTranslationHooks(&localeLoadContext)

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

	// Merge plugin translations
	for k, v := range pluginTranslations {
		out[k] = v
	}

	return out, nil
}
