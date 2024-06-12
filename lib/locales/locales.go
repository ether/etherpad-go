package locales

import (
	"os"
	"strings"
)

var Locales map[string]string

func init() {
	Locales = make(map[string]string)
	files, _ := os.ReadDir("./assets/locales")
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileName := file.Name()
		Locales[strings.Replace(fileName, ".json", "", -1)] = `locales/` + fileName
	}
}
