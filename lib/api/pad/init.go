package pad

import (
	"github.com/ether/etherpad-go/lib"
	errors2 "github.com/ether/etherpad-go/lib/api/errors"
	utils2 "github.com/ether/etherpad-go/lib/api/utils"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v2"
)

type AttributePoolResponse struct {
	Pool apool.APool `json:"pool"`
}

// TextResponse represents the text response
type TextResponse struct {
	Text string `json:"text"`
}

// SetTextRequest represents the request to set text
type SetTextRequest struct {
	Text     string `json:"text"`
	AuthorId string `json:"authorId"`
}

// GetPadText godoc
// @Summary Get pad text
// @Description Returns the current text of a pad, optionally for a specific revision
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param rev query string false "Revision number"
// @Success 200 {object} TextResponse
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/text [get]
func GetPadText(initStore *lib.InitStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		foundPad, err := utils2.GetPadSafe(c.Params("padId", ""), true, nil, nil, initStore.PadManager)
		if err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		head := foundPad.Head

		optRev := c.Query("rev")
		if optRev != "" {
			revNum, err := utils.CheckValidRev(optRev)
			if err != nil {
				return c.Status(400).JSON(errors2.InvalidRevisionError)
			}
			if *revNum > head {
				return c.Status(400).JSON(errors2.RevisionHigherThanHeadError)
			}

			foundText := foundPad.GetInternalRevisionAText(*revNum)
			if foundText == nil {
				return c.Status(500).JSON(errors2.InternalApiError)
			}
			return c.JSON(TextResponse{
				Text: foundText.Text,
			})
		}

		text, err := pad.GetTxtFromAText(foundPad, foundPad.AText)
		if err != nil {
			return c.Status(500).JSON(errors2.InternalApiError)
		}
		return c.JSON(TextResponse{
			Text: *text,
		})
	}
}

// GetAttributePool godoc
// @Summary Get attribute pool of a pad
// @Description Returns the attribute pool of a pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Success 200 {object} AttributePoolResponse
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/attributePool [get]
func GetAttributePool(initStore *lib.InitStore) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		var padIdToFind = ctx.Params("padId")
		var padFound, err = utils2.GetPadSafe(padIdToFind, true, nil, nil, initStore.PadManager)
		if err != nil {
			return ctx.Status(404).JSON(errors2.PadNotFoundError)
		}

		return ctx.JSON(AttributePoolResponse{
			Pool: padFound.Pool,
		})
	}
}

// GetRevisionChangeset godoc
// @Summary Get revision changeset
// @Description Returns the changeset of a specific revision of a pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param rev path string true "Revision number"
// @Success 200 {object} string
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/{rev}/revisionChangeset [get]
func GetRevisionChangeset(initStore *lib.InitStore) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		var padId = ctx.Params("padId")
		var rev = ctx.Params("rev")

		var revNum, errorForPad = utils.CheckValidRev(rev)
		if errorForPad != nil {
			return ctx.Status(400).JSON(errors2.InvalidRevisionError)
		}

		var foundPad, errorForPad2 = utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if errorForPad2 != nil {
			return ctx.Status(404).JSON(errors2.PadNotFoundError)
		}
		var head = foundPad.Head

		if *revNum > head {
			return ctx.Status(400).JSON(errors2.RevisionHigherThanHeadError)
		}

		var revision, err = foundPad.GetRevision(*revNum)
		if err != nil {
			return ctx.Status(404).JSON(errors2.RevisionNotFoundError)
		}

		return ctx.JSON(revision.Changeset)
	}
}

// SetPadText godoc
// @Summary Set pad text
// @Description Updates the text of a pad
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param request body SetTextRequest true "Text and Author ID"
// @Success 200 {string} string "OK"
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/text [post]
func SetPadText(initStore *lib.InitStore) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		var padId = ctx.Params("padId")
		var request SetTextRequest
		err := ctx.BodyParser(&request)

		if err != nil {
			return ctx.Status(400).JSON(errors2.InvalidRequestError)
		}

		var retrievedPad, errPadSafe = utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if errPadSafe != nil {
			return ctx.Status(404).JSON(errors2.PadNotFoundError)
		}
		err = retrievedPad.SetText(request.Text, nil)
		if err != nil {
			return ctx.Status(500).JSON(errors2.InternalServerError)
		}
		initStore.Handler.UpdatePadClients(retrievedPad)
		return ctx.SendStatus(200)
	}
}

func Init(initStore *lib.InitStore) {
	// Auth (no :padId parameter)
	initStore.PrivateAPI.Get("/checkToken", CheckToken())

	// Pad list (no :padId parameter)
	initStore.PrivateAPI.Get("/pads", ListAllPads(initStore))

	// Read-only routes (specific path before :padId)
	initStore.PrivateAPI.Get("/pads/readonly/:roId", GetPadID(initStore))

	// Text operations
	initStore.PrivateAPI.Get("/pads/:padId/text", GetPadText(initStore))
	initStore.PrivateAPI.Post("/pads/:padId/text", SetPadText(initStore))
	initStore.PrivateAPI.Post("/pads/:padId/appendText", AppendText(initStore))

	// Attribute pool and changesets
	initStore.PrivateAPI.Get("/pads/:padId/attributePool", GetAttributePool(initStore))
	initStore.PrivateAPI.Get("/pads/:padId/revisionChangeset", GetRevisionChangesetOptional(initStore))
	initStore.PrivateAPI.Get("/pads/:padId/:rev/revisionChangeset", GetRevisionChangeset(initStore))

	// Pad operations
	initStore.PrivateAPI.Post("/pads/:padId/restoreRevision", RestoreRevision(initStore))
	initStore.PrivateAPI.Get("/pads/:padId/readOnlyID", GetReadOnlyID(initStore))
	initStore.PrivateAPI.Get("/pads/:padId/authors", ListAuthorsOfPad(initStore))
	initStore.PrivateAPI.Get("/pads/:padId/chatHead", GetChatHead(initStore))
	initStore.PrivateAPI.Get("/pads/:padId/revisionsCount", GetRevisionsCount(initStore))
	initStore.PrivateAPI.Get("/pads/:padId/lastEdited", GetLastEdited(initStore))
	initStore.PrivateAPI.Get("/pads/:padId/html", GetHTML(initStore))
	initStore.PrivateAPI.Post("/pads/:padId/html", SetHTML(initStore))

	// Users in pad
	initStore.PrivateAPI.Get("/pads/:padId/users", GetPadUsers(initStore))
	initStore.PrivateAPI.Get("/pads/:padId/usersCount", GetPadUsersCount(initStore))

	// Saved revisions
	initStore.PrivateAPI.Get("/pads/:padId/savedRevisionsCount", GetSavedRevisionsCount(initStore))
	initStore.PrivateAPI.Get("/pads/:padId/savedRevisions", ListSavedRevisions(initStore))
	initStore.PrivateAPI.Post("/pads/:padId/saveRevision", SaveRevision(initStore))

	// Chat
	initStore.PrivateAPI.Get("/pads/:padId/chatHistory", GetChatHistory(initStore))
	initStore.PrivateAPI.Post("/pads/:padId/chat", AppendChatMessage(initStore))

	// CRUD operations on pad itself (last to avoid conflicts)
	initStore.PrivateAPI.Post("/pads/:padId", CreatePad(initStore))
	initStore.PrivateAPI.Delete("/pads/:padId", DeletePad(initStore))
}
