package pad

import (
	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/lib/models"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"os"
	"strings"
)
import padAsset "github.com/ether/etherpad-go/assets/pad"

func HandlePadOpen(c *fiber.Ctx) error {
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

	return adaptor.HTTPHandler(templ.Handler(padComp))(c)
}
