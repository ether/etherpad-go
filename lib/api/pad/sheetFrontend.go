package pad

import (
	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"

	sheetAsset "github.com/ether/etherpad-go/assets/sheet"
)

// HandleSheetOpen serves the (stub) spreadsheet editor shell for /s/:pad.
// Pad creation with document type "sheet" happens on the websocket connect
// (added in plan 2); here we only deliver the shell.
// The page takes the pad theme (etherpad-go or colibris) and, with
// enableDarkMode, follows the system's dark mode.
func HandleSheetOpen(c fiber.Ctx, s *settings.Settings) error {
	padName := c.Params("pad")
	jsFilePath := "../js/sheet/assets/sheet.js?v=" + utils.RandomVersionString
	comp := sheetAsset.SheetIndex(padName, jsFilePath, SheetPage(s))
	return adaptor.HTTPHandler(templ.Handler(comp))(c)
}

// SheetPage describes the <html> element of the spreadsheet page.
func SheetPage(s *settings.Settings) sheetAsset.Page {
	if s == nil {
		return sheetAsset.Page{}
	}
	// The spreadsheet has no skins; colibris here just means "classic".
	theme := s.PadTheme.Resolved("colibris")
	p := sheetAsset.Page{Theme: theme, DarkMode: s.EnableDarkMode}
	if accent := s.PadTheme.Accent(); accent != "" {
		p.Style = "--primary-color: " + accent + ";"
	}
	return p
}
