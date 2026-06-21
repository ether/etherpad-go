package pad

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"

	sheetAsset "github.com/ether/etherpad-go/assets/sheet"
)

// HandleSheetOpen serves the (stub) spreadsheet editor shell for /s/:pad.
// Pad creation with document type "sheet" happens on the websocket connect
// (added in plan 2); here we only deliver the shell.
func HandleSheetOpen(c fiber.Ctx) error {
	padName := c.Params("pad")
	jsFilePath := "/js/sheet/assets/sheet.js?v=" + strconv.Itoa(utils.RandomVersionString)
	comp := sheetAsset.SheetIndex(padName, jsFilePath)
	return adaptor.HTTPHandler(templ.Handler(comp))(c)
}
