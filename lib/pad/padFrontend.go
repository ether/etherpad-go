package pad

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/lib/models"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
)
import padAsset "github.com/ether/etherpad-go/assets/pad"

func HandlePadOpen(c *fiber.Ctx) error {
	pad := models.Model{
		Name: "test",
	}

	jsFilePath := "/js/pad/assets/pad.js?v=" + strconv.Itoa(utils.RandomVersionString)

	padComp := padAsset.Greeting(pad, jsFilePath)

	return adaptor.HTTPHandler(templ.Handler(padComp))(c)
}
