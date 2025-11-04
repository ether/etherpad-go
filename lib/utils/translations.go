package utils

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

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
