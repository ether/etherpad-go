package locales

import (
	"encoding/json"
	"os"
	"strings"
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
