package pad

import (
	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/lib/models"
	"net/http"
	"os"
	"strings"
)
import padAsset "github.com/ether/etherpad-go/assets/pad"

func HandlePadOpen(w http.ResponseWriter, r *http.Request) {
	pad := models.Model{
		Name: "test",
	}

	// list files in dir
	entries, _ := os.ReadDir("./assets/js/pad/assets")

	var jsFilePath = "/js/pad/assets/"
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "js") {
			jsFilePath += e.Name()
		}
	}

	padComp := padAsset.Greeting(pad, jsFilePath)

	templ.Handler(padComp).ServeHTTP(w, r)

}
