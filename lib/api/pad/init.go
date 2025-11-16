package pad

import (
	"errors"

	apiError "github.com/ether/etherpad-go/lib/api/error"
	utils2 "github.com/ether/etherpad-go/lib/api/utils"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/gofiber/fiber/v2"
)

func getText(padId string, rev *string, manager *pad.Manager) (*string, error) {
	var revNum *int = nil
	if rev != nil {
		revPoint, err := utils.CheckValidRev(*rev)
		revNum = revPoint
		if err != nil {
			return nil, err
		}
	}

	pad, err := utils2.GetPadSafe(padId, true, nil, nil, manager)

	if err != nil {
		return nil, err
	}

	var head = pad.Head

	if revNum != nil {
		if *revNum > head {
			return nil, errors.New("revision number is higher than head")
		}

		var atext = pad.GetInternalRevisionAText(*revNum)
		return &atext.Text, nil
	}
	var emptyForNow = ""
	return &emptyForNow, nil
}

type AttributePoolResponse struct {
	Pool apool.APool `json:"pool"`
}

func Init(c *fiber.App, handler *ws.PadMessageHandler, manager *pad.Manager) {
	c.Get("/pads/:padId/text", func(c *fiber.Ctx) error {
		foundPad, err := utils2.GetPadSafe(c.Params("padID", ""), true, nil, nil, manager)
		if err != nil {
			return c.Status(404).JSON(apiError.Error{
				Message: "Pad not found",
			})
		}

		head := foundPad.Head

		optRev := c.Query("rev")
		if optRev != "" {
			revNum, err := utils.CheckValidRev(optRev)
			if err != nil {
				return c.Status(400).JSON(apiError.Error{
					Message: "Invalid revision number",
				})
			}
			if *revNum > head {
				return c.Status(400).JSON(apiError.Error{
					Message: "Revision number is higher than head",
				})
			}

			foundText := foundPad.GetInternalRevisionAText(*revNum)
			if foundText == nil {
				return c.Status(500).JSON(apiError.Error{
					Message: "Internal server error",
				})
			}
			return c.JSON(fiber.Map{
				"text": foundText.Text,
			})
		}

		text, err := pad.GetTxtFromAText(foundPad, foundPad.AText)
		if err != nil {
			return c.Status(500).JSON(apiError.Error{
				Message: "Internal server error",
			})
		}
		return c.JSON(fiber.Map{
			"text": *text,
		})
	})

	c.Get("/pads/:padId/attributePool", func(ctx *fiber.Ctx) error {
		var padIdToFind = ctx.Params("padId")
		var padFound, err = utils2.GetPadSafe(padIdToFind, true, nil, nil, manager)
		if err != nil {
			return ctx.Status(404).JSON(apiError.Error{
				Message: "Pad not found",
				Error:   404,
			})
		}

		return ctx.JSON(AttributePoolResponse{
			Pool: padFound.Pool,
		})
	})
	c.Get("/pads/:padId/:rev/revisionChangeset", func(ctx *fiber.Ctx) error {
		var padId = ctx.Params("padId")
		var rev = ctx.Params("rev")

		var revNum, errorForPad = utils.CheckValidRev(rev)
		if errorForPad != nil {
			return ctx.Status(400).JSON(apiError.Error{
				Message: "Invalid revision number",
				Error:   400,
			})
		}

		var pad, errorForPad2 = utils2.GetPadSafe(padId, true, nil, nil, manager)
		if errorForPad2 != nil {
			return ctx.Status(404).JSON(apiError.Error{
				Message: "Pad not found",
				Error:   404,
			})
		}
		var head = pad.Head

		if *revNum > head {
			return ctx.Status(400).JSON(apiError.Error{
				Message: "Revision number is higher than head",
				Error:   400,
			})
		}

		var revision, err = pad.GetRevision(*revNum)
		if err != nil {
			return ctx.Status(404).JSON(apiError.Error{
				Message: "Revision not found",
				Error:   404,
			})
		}

		return ctx.JSON(revision.Changeset)
	})

	c.Post("/pads/:padId/text", func(ctx *fiber.Ctx) error {
		type Request struct {
			Text     string `json:"text"`
			AuthorId string `json:"authorId"`
		}

		var padId = ctx.Params("padId")
		var request Request
		err := ctx.BodyParser(&request)

		if err != nil {
			return ctx.Status(400).JSON(apiError.Error{
				Message: "Invalid request",
				Error:   400,
			})
		}

		var pad, errPadSafe = utils2.GetPadSafe(padId, true, nil, nil, manager)
		if errPadSafe != nil {
			return ctx.Status(404).JSON(apiError.Error{
				Message: "Pad not found",
				Error:   404,
			})
		}
		err = pad.SetText(request.Text, nil)
		if err != nil {
			return ctx.Status(500).JSON(apiError.Error{
				Message: "Internal server error",
				Error:   500,
			})
		}
		handler.UpdatePadClients(pad)
		return ctx.SendStatus(200)
	})
}
