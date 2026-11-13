package pad

import (
	"github.com/ether/etherpad-go/lib"
	errors2 "github.com/ether/etherpad-go/lib/api/errors"
	utils2 "github.com/ether/etherpad-go/lib/api/utils"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	io2 "github.com/ether/etherpad-go/lib/io"
	"github.com/ether/etherpad-go/lib/paddiff"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/gofiber/fiber/v3"
)

// CompactPadRequest represents the request to compact a pad's revision history
type CompactPadRequest struct {
	// KeepRevisions is how many of the latest revisions stay; omitted, the
	// whole history is collapsed into one revision.
	KeepRevisions *int `json:"keepRevisions"`
}

// CompactPadResponse represents the response after compacting a pad
type CompactPadResponse struct {
	Ok bool `json:"ok"`
	// Mode is "all" (history collapsed) or "keepLast".
	Mode          string `json:"mode"`
	KeepRevisions int    `json:"keepRevisions"`
}

// CompactPad godoc
// @Summary Compact a pad's revision history
// @Description Collapses the pad's revision history so that only the last keepRevisions revisions are kept, or all of it into one revision when keepRevisions is omitted (original API: compactPad). The history is swapped in one transaction; pad text is preserved. Needs cleanup.enabled. Destructive — consider exporting the pad first.
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param request body CompactPadRequest true "Number of recent revisions to keep (must be >= 1 and lower than the pad's head revision)"
// @Success 200 {object} CompactPadResponse
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/compact [post]
func CompactPad(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")
		var request CompactPadRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors2.InvalidRequestError)
		}

		foundPad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		// Destructive, so it is opt-in like the admin UI's cleanup.
		if !initStore.RetrievedSettings.Cleanup.Enabled {
			return c.Status(403).JSON(errors2.NewInvalidParamError("compacting needs cleanup.enabled = true in the settings"))
		}
		keep, mode := 0, "all"
		if request.KeepRevisions != nil {
			if *request.KeepRevisions < 0 {
				return c.Status(400).JSON(errors2.NewInvalidParamError("keepRevisions must be a non-negative integer"))
			}
			keep, mode = *request.KeepRevisions, "keepLast"
		}
		if keep >= foundPad.Head {
			// Nothing to drop.
			return c.JSON(CompactPadResponse{Ok: true, Mode: mode, KeepRevisions: keep})
		}
		// Reuse the revision compaction the admin UI uses
		// (AdminMessageHandler.DeleteRevisions). DeleteRevisions only needs the
		// store, pad manager, pad message handler and logger, all of which are
		// available from the InitStore, so a handler is wired up on the fly
		// (hub is not used by DeleteRevisions).
		adminHandler := ws.NewAdminMessageHandler(initStore.Store, initStore.Hooks, initStore.PadManager, initStore.Handler, initStore.Logger, nil, initStore.C, nil)
		if err := adminHandler.DeleteRevisions(padId, keep); err != nil {
			initStore.Logger.Errorf("Error compacting pad %s: %v", padId, err)
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		return c.JSON(CompactPadResponse{Ok: true, Mode: mode, KeepRevisions: keep})
	}
}

// CreateDiffHTML godoc
// @Summary Create an HTML diff between two revisions
// @Description Returns the changes between startRev and endRev as HTML (original API: createDiffHTML). Insertions keep their author attribution (rendered with the author's color), deletions are re-inserted with a 'removed' attribute (rendered struck through). Also returns the list of authors involved in the changes.
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param startRev query int true "Start revision number"
// @Param endRev query int false "End revision number (defaults to the head revision)"
// @Success 200 {object} DiffHTMLResponse
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/diffHTML [get]
func CreateDiffHTML(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		foundPad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		startRevStr := c.Query("startRev")
		if startRevStr == "" {
			return c.Status(400).JSON(errors2.NewMissingParamError("startRev"))
		}
		startRev, err := utils.CheckValidRev(startRevStr)
		if err != nil {
			return c.Status(400).JSON(errors2.InvalidRevisionError)
		}

		var endRev *int
		if endRevStr := c.Query("endRev"); endRevStr != "" {
			endRevNum, err := utils.CheckValidRev(endRevStr)
			if err != nil {
				return c.Status(400).JSON(errors2.InvalidRevisionError)
			}
			endRev = endRevNum
		}

		// The original API clamps startRev to the head revision before
		// validating the range; endRev is clamped inside GetValidRevisionRange.
		head := foundPad.Head
		start := *startRev
		if start > head {
			start = head
		}

		from, to, ok := paddiff.GetValidRevisionRange(start, endRev, head)
		if !ok {
			return c.Status(400).JSON(errors2.NewInvalidParamError("invalid revision range"))
		}

		diffAText, authors, err := paddiff.CreateDiffAText(foundPad, &foundPad.Pool, from, to)
		if err != nil {
			initStore.Logger.Errorf("Error creating diff atext for pad %s: %v", padId, err)
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		// Render the diff atext with the regular export-HTML pipeline (it
		// understands the 'removed' attribute). GetPadHTML reads pad.AText when
		// no revision is requested, so a shallow copy of the pad carrying the
		// diff atext is passed.
		padWithDiff := *foundPad
		padWithDiff.AText = *diffAText

		authorColors := buildAuthorColors(&foundPad.Pool, initStore.AuthorManager)
		exporter := io2.NewExportHtml(initStore.PadManager, initStore.AuthorManager, initStore.Hooks)
		html, err := exporter.GetPadHTML(&padWithDiff, nil, authorColors)
		if err != nil {
			initStore.Logger.Errorf("Error rendering diff HTML for pad %s: %v", padId, err)
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		return c.JSON(DiffHTMLResponse{
			HTML:    html,
			Authors: authors,
		})
	}
}

// buildAuthorColors maps the author IDs found in the pad's attribute pool to
// their colors (equivalent of the original pad.getAllAuthorColors; mirrors the
// unexported buildAuthorColorCache in lib/io/exportHtml.go).
func buildAuthorColors(padPool *apool.APool, authorManager *author.Manager) map[string]string {
	authorColors := make(map[string]string)
	for _, attr := range padPool.NumToAttrib {
		if attr.Key == "author" && attr.Value != "" {
			if _, exists := authorColors[attr.Value]; exists {
				continue
			}
			if authorData, err := authorManager.GetAuthor(attr.Value); err == nil {
				authorColors[attr.Value] = authorData.ColorId
			}
		}
	}
	return authorColors
}
