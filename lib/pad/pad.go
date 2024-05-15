package pad

import (
	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/lib/models"
	"net/http"
)
import padAsset "github.com/ether/etherpad-go/assets/pad"

func HandlePadOpen(w http.ResponseWriter, r *http.Request) {
	pad := models.Model{
		Name: "test",
	}

	padComp := padAsset.Greeting(pad)

	templ.Handler(padComp).ServeHTTP(w, r)

}
