package sheetio

import (
	"errors"
	"strconv"
	"time"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/sheetdoc"
	"github.com/gofiber/fiber/v3"
)

type historyAuthor struct {
	Name  string `json:"name,omitempty"`
	Color string `json:"color,omitempty"`
}

// SheetHistory handles GET /s/:pad/history: the stored changes with their
// time and author, oldest first.
func SheetHistory(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("pad")
		if _, err := checkGrant(c, store, padId, false); err != nil {
			return err
		}
		padId, err := existingSheet(store, padId)
		if err != nil {
			return err
		}
		revs, head, available, err := store.Handler.SheetManager().History(padId)
		if err != nil {
			return historyError(err)
		}
		authors := map[string]historyAuthor{}
		for _, r := range revs {
			if _, done := authors[r.Author]; done || r.Author == "" {
				continue
			}
			a := historyAuthor{}
			if got, err := store.AuthorManager.GetAuthor(r.Author); err == nil {
				if got.Name != nil {
					a.Name = *got.Name
				}
				a.Color = got.ColorId
			}
			authors[r.Author] = a
		}
		return c.JSON(fiber.Map{"head": head, "available": available, "revisions": revs, "authors": authors})
	}
}

// SheetAtRevision handles GET /s/:pad/history/:rev: the workbook snapshot
// as it was at that revision.
func SheetAtRevision(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("pad")
		if _, err := checkGrant(c, store, padId, false); err != nil {
			return err
		}
		padId, err := existingSheet(store, padId)
		if err != nil {
			return err
		}
		rev, err := strconv.Atoi(c.Params("rev"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid revision")
		}
		wb, err := store.Handler.SheetManager().WorkbookAt(padId, rev)
		if err != nil {
			return historyError(err)
		}
		return c.JSON(fiber.Map{"rev": rev, "snapshot": wb.Snapshot()})
	}
}

// RestoreSheet handles POST /s/:pad/history/:rev/restore: it brings the
// workbook back to that revision with ordinary ops, which reach the
// connected clients like anyone's edits.
func RestoreSheet(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("pad")
		rev, err := strconv.Atoi(c.Params("rev"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid revision")
		}
		// Same lock as SHEET_OPs: the ops are sent to the clients in order.
		unlock, err := store.PadManager.LockPad(padId)
		if err != nil {
			return c.Status(503).SendString("Pad temporarily unavailable")
		}
		defer unlock()
		authorId, err := checkGrant(c, store, padId, true)
		if err != nil {
			return err
		}
		if padId, err = existingSheet(store, padId); err != nil {
			return err
		}
		var author *string
		if authorId != "" {
			author = &authorId
		}
		n, err := store.Handler.SheetManager().Restore(padId, rev, author, time.Now().UnixMilli())
		if n > 0 {
			// Also after a partial restore: what was stored goes out.
			store.Handler.BroadcastSheetReload(padId)
		}
		if err != nil {
			return historyError(err)
		}
		_, head, _ := store.Handler.SheetManager().State(padId)
		return c.JSON(fiber.Map{"code": 0, "ops": n, "head": head})
	}
}

func historyError(err error) error {
	switch {
	case errors.Is(err, sheetdoc.ErrNoHistory):
		return fiber.NewError(fiber.StatusNotFound, "history not available")
	case errors.Is(err, sheetdoc.ErrHistoryChanged):
		return fiber.NewError(fiber.StatusConflict, "sheet was replaced")
	case errors.Is(err, sheetdoc.ErrNoRevision):
		return fiber.NewError(fiber.StatusNotFound, "no such revision")
	}
	return fiber.NewError(fiber.StatusInternalServerError, err.Error())
}
