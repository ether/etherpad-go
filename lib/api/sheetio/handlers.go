package sheetio

import (
	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/ether/etherpad-go/lib/xlsx"
	"github.com/gofiber/fiber/v3"
)

// checkGrant authorizes the request for the pad, returning the author id:
// with the webaccess user and the author token cookie, as pad import and
// export do. write also needs write access (not through the read-only id,
// not with a readOnly authorization).
func checkGrant(c fiber.Ctx, store *lib.InitStore, padId string, write bool) (string, error) {
	granted, err := store.SecurityManager.CheckRequestAccess(c, padId)
	if err != nil || granted == nil || granted.AccessStatus != "grant" {
		return "", fiber.NewError(fiber.StatusForbidden, "accessDenied")
	}
	if write && !store.SecurityManager.RequestCanModify(c, padId) {
		return "", fiber.NewError(fiber.StatusForbidden, "accessDenied")
	}
	return granted.AuthorId, nil
}

// sheetId is the pad a request's id names: a read-only id stands for its
// pad (read access only, which checkGrant enforces).
func sheetId(store *lib.InitStore, id string) (string, error) {
	rom := store.SecurityManager.ReadOnlyManager
	if !rom.IsReadOnlyID(&id) {
		return id, nil
	}
	padId, err := rom.GetPadId(id)
	if err != nil || padId == nil {
		return "", fiber.NewError(fiber.StatusNotFound, "sheet not found")
	}
	return *padId, nil
}

// existingSheet is sheetId for routes that only read: the pad must exist
// and be a spreadsheet, so reading never creates a sheet.
func existingSheet(store *lib.InitStore, id string) (string, error) {
	padId, err := sheetId(store, id)
	if err != nil {
		return "", err
	}
	if ok, err := store.PadManager.DoesPadExist(padId); err != nil || ok == nil || !*ok {
		return "", fiber.NewError(fiber.StatusNotFound, "sheet not found")
	}
	p, err := store.PadManager.GetTypedPad(padId, "sheet", nil)
	if err != nil || p.DocumentType != "sheet" {
		return "", fiber.NewError(fiber.StatusNotFound, "sheet not found")
	}
	return padId, nil
}

// ImportSheet handles POST /s/:pad/import (multipart "file"), replacing the
// sheet's workbook and notifying connected clients to reload.
func ImportSheet(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("pad")
		// Same lock as SHEET_OPs, so an import can't interleave with an op.
		unlock, err := store.PadManager.LockPad(padId)
		if err != nil {
			return c.Status(503).SendString("Pad temporarily unavailable")
		}
		defer unlock()
		if _, err := checkGrant(c, store, padId, true); err != nil {
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
		if _, err := checkGrant(c, store, padId, false); err != nil {
			return err
		}
		padId, err := existingSheet(store, padId)
		if err != nil {
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
