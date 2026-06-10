package pad

import (
	"strings"

	"github.com/ether/etherpad-go/lib"
	errors2 "github.com/ether/etherpad-go/lib/api/errors"
	utils2 "github.com/ether/etherpad-go/lib/api/utils"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/gofiber/fiber/v3"
)

// handlerError carries an HTTP status and the error body to send to the client.
type handlerError struct {
	status int
	body   errors2.Error
}

// prepareCopyDestination validates the destination pad ID and applies the
// force semantics of the original Etherpad API: reject when the destination
// already exists unless force is set, in which case the destination pad is
// removed first.
func prepareCopyDestination(initStore *lib.InitStore, destinationID string, force bool) *handlerError {
	if destinationID == "" {
		return &handlerError{400, errors2.NewMissingParamError("destinationID")}
	}
	if !initStore.PadManager.IsValidPadId(destinationID) {
		return &handlerError{400, errors2.NewInvalidParamError("destinationID is not a valid pad ID")}
	}

	exists, err := initStore.PadManager.DoesPadExist(destinationID)
	if err != nil {
		return &handlerError{500, errors2.InternalServerError}
	}
	if exists != nil && *exists {
		if !force {
			return &handlerError{409, errors2.PadAlreadyExistsError}
		}
		if err := initStore.PadManager.RemovePad(destinationID); err != nil {
			return &handlerError{500, errors2.InternalServerError}
		}
	}
	return nil
}

// copyPadRecords copies the pad record, all revisions and the chat history of
// sourceID to destinationID using the DataStore primitives. The destination
// must not exist (callers go through prepareCopyDestination first).
func copyPadRecords(initStore *lib.InitStore, sourceID string, destinationID string) error {
	sourceDB, err := initStore.Store.GetPad(sourceID)
	if err != nil {
		return err
	}

	destinationDB := *sourceDB
	destinationDB.ID = destinationID
	// Read-only IDs must stay unique per pad; the destination gets its own on demand.
	destinationDB.ReadOnlyId = nil

	if err := initStore.Store.CreatePad(destinationID, destinationDB); err != nil {
		return err
	}

	// Copy the full revision history
	if sourceDB.Head >= 0 {
		revisions, err := initStore.Store.GetRevisions(sourceID, 0, sourceDB.Head)
		if err != nil {
			return err
		}
		for _, rev := range *revisions {
			pool := db2.RevPool{}
			if rev.Pool != nil {
				pool = *rev.Pool
			}
			if err := initStore.Store.SaveRevision(
				destinationID, rev.RevNum, rev.Changeset, rev.AText, pool, rev.AuthorId, rev.Timestamp,
			); err != nil {
				return err
			}
		}
	}

	// Copy the chat history
	if sourceDB.ChatHead >= 0 {
		messages, err := initStore.Store.GetChatsOfPad(sourceID, 0, sourceDB.ChatHead)
		if err != nil {
			return err
		}
		if messages != nil {
			for _, msg := range *messages {
				var timestamp int64
				if msg.Time != nil {
					timestamp = *msg.Time
				}
				if err := initStore.Store.SaveChatMessage(
					destinationID, msg.Head, msg.AuthorId, timestamp, msg.Message,
				); err != nil {
					return err
				}
			}
		}
	}

	// Make sure the next access loads the freshly written records from the database
	initStore.PadManager.UnloadPad(destinationID)
	return nil
}

// CopyPad godoc
// @Summary Copy a pad
// @Description Copies a pad including its full revision and chat history to a new pad. Fails if the destination exists unless force is set.
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Source Pad ID"
// @Param request body CopyPadRequest true "Destination ID and force flag"
// @Success 200 {object} PadIDResponse
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 409 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/copy [post]
func CopyPad(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")
		var request CopyPadRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors2.InvalidRequestError)
		}

		// Verify source pad exists
		if _, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager); err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		if hErr := prepareCopyDestination(initStore, request.DestinationID, request.Force); hErr != nil {
			return c.Status(hErr.status).JSON(hErr.body)
		}

		if err := copyPadRecords(initStore, padId, request.DestinationID); err != nil {
			initStore.Logger.Errorf("Error copying pad %s to %s: %v", padId, request.DestinationID, err)
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		return c.JSON(PadIDResponse{
			PadID: request.DestinationID,
		})
	}
}

// CopyPadWithoutHistory godoc
// @Summary Copy a pad without history
// @Description Copies the current text of a pad to a new pad with a single initial revision. Fails if the destination exists unless force is set.
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Source Pad ID"
// @Param request body CopyPadWithoutHistoryRequest true "Destination ID, force flag and Author ID"
// @Success 200 {object} PadIDResponse
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 409 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/copyWithoutHistory [post]
func CopyPadWithoutHistory(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")
		var request CopyPadWithoutHistoryRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors2.InvalidRequestError)
		}

		// Get the source pad
		sourcePad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		if hErr := prepareCopyDestination(initStore, request.DestinationID, request.Force); hErr != nil {
			return c.Status(hErr.status).JSON(hErr.body)
		}

		// Creating the destination pad re-adds the trailing newline, so strip it
		// from the source text first (mirrors the original copyPadWithoutHistory).
		text := strings.TrimSuffix(sourcePad.Text(), "\n")

		var authorId *string
		if request.AuthorId != "" {
			authorId = &request.AuthorId
		}

		if _, err := initStore.PadManager.GetPad(request.DestinationID, &text, authorId); err != nil {
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		return c.JSON(PadIDResponse{
			PadID: request.DestinationID,
		})
	}
}

// MovePad godoc
// @Summary Move a pad
// @Description Moves a pad (copy including history, then remove the source). Fails if the destination exists unless force is set.
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Source Pad ID"
// @Param request body MovePadRequest true "Destination ID and force flag"
// @Success 200 {object} PadIDResponse
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 409 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/move [post]
func MovePad(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")
		var request MovePadRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors2.InvalidRequestError)
		}

		// Verify source pad exists
		if _, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager); err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		if hErr := prepareCopyDestination(initStore, request.DestinationID, request.Force); hErr != nil {
			return c.Status(hErr.status).JSON(hErr.body)
		}

		if err := copyPadRecords(initStore, padId, request.DestinationID); err != nil {
			initStore.Logger.Errorf("Error copying pad %s to %s: %v", padId, request.DestinationID, err)
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		// Remove the source pad after a successful copy
		if err := initStore.PadManager.RemovePad(padId); err != nil {
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		return c.JSON(PadIDResponse{
			PadID: request.DestinationID,
		})
	}
}

// GetPublicStatus godoc
// @Summary Get public status of a pad
// @Description Returns whether the pad is marked as public
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Success 200 {object} PublicStatusResponse
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/publicStatus [get]
func GetPublicStatus(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		return c.JSON(PublicStatusResponse{
			PublicStatus: pad.PublicStatus,
		})
	}
}

// SetPublicStatus godoc
// @Summary Set public status of a pad
// @Description Marks the pad as public or private and persists the change
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param request body PublicStatusRequest true "Public status"
// @Success 200 {string} string "OK"
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/publicStatus [post]
func SetPublicStatus(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")
		var request PublicStatusRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors2.InvalidRequestError)
		}

		// Get the pad
		pad, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		pad.PublicStatus = request.PublicStatus
		if err := pad.Save(); err != nil {
			return c.Status(500).JSON(errors2.InternalServerError)
		}

		return c.SendStatus(200)
	}
}
