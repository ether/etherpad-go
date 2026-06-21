package sheetio

import (
	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/ether/etherpad-go/lib/xlsx"
	"github.com/gofiber/fiber/v3"
)

// checkGrant authorizes the request for the pad, returning the author id.
func checkGrant(c fiber.Ctx, store *lib.InitStore, padId string) (string, error) {
	token := c.Cookies("token")
	granted, err := store.SecurityManager.CheckAccess(&padId, nil, &token, nil)
	if err != nil {
		return "", fiber.NewError(fiber.StatusInternalServerError, "internalError")
	}
	if granted.AccessStatus != "grant" {
		return "", fiber.NewError(fiber.StatusForbidden, "accessDenied")
	}
	return granted.AuthorId, nil
}

// ImportSheet handles POST /s/:pad/import (multipart "file"), replacing the
// sheet's workbook and notifying connected clients to reload.
func ImportSheet(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("pad")
		if _, err := checkGrant(c, store, padId); err != nil {
			return err
		}

		fileHeader, err := c.FormFile("file")
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "missing file")
		}
		if max := store.RetrievedSettings.ImportMaxFileSize; max > 0 && fileHeader.Size > int64(max) {
			return fiber.NewError(fiber.StatusBadRequest, "maxFileSize")
		}
		file, err := fileHeader.Open()
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "uploadFailed")
		}
		defer file.Close()

		snap, err := xlsx.Import(file)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid xlsx: "+err.Error())
		}
		wb := sheet.WorkbookFromSnapshot(snap)
		if err := store.Handler.SheetManager().SetWorkbook(padId, wb); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		store.Handler.BroadcastSheetReload(padId)
		return c.JSON(fiber.Map{"code": 0, "message": "ok"})
	}
}

// ExportSheet handles GET /s/:pad/export.xlsx.
func ExportSheet(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("pad")
		if _, err := checkGrant(c, store, padId); err != nil {
			return err
		}

		snap, _, err := store.Handler.SheetManager().Snapshot(padId)
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, "sheet not found")
		}
		data, err := xlsx.Export(sheet.WorkbookFromSnapshot(snap))
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Set(fiber.HeaderContentDisposition, `attachment; filename="`+padId+`.xlsx"`)
		return c.Send(data)
	}
}
