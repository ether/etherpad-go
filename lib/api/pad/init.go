package pad

import (
	"errors"

	"github.com/ether/etherpad-go/lib"
	errors2 "github.com/ether/etherpad-go/lib/api/errors"
	utils2 "github.com/ether/etherpad-go/lib/api/utils"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/utils"
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

func Init(initStore *lib.InitStore) {
	initStore.C.Get("/pads/:padId/text", func(c *fiber.Ctx) error {
		foundPad, err := utils2.GetPadSafe(c.Params("padID", ""), true, nil, nil, initStore.PadManager)
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
			return c.JSON(fiber.Map{
				"text": foundText.Text,
			})
		}

		text, err := pad.GetTxtFromAText(foundPad, foundPad.AText)
		if err != nil {
			return c.Status(500).JSON(errors2.InternalApiError)
		}
		return c.JSON(fiber.Map{
			"text": *text,
		})
	})

	initStore.C.Get("/pads/:padId/attributePool", func(ctx *fiber.Ctx) error {
		var padIdToFind = ctx.Params("padId")
		var padFound, err = utils2.GetPadSafe(padIdToFind, true, nil, nil, initStore.PadManager)
		if err != nil {
			return ctx.Status(404).JSON(errors2.PadNotFoundError)
		}

		return ctx.JSON(AttributePoolResponse{
			Pool: padFound.Pool,
		})
	})
	initStore.C.Get("/pads/:padId/:rev/revisionChangeset", func(ctx *fiber.Ctx) error {
		var padId = ctx.Params("padId")
		var rev = ctx.Params("rev")

		var revNum, errorForPad = utils.CheckValidRev(rev)
		if errorForPad != nil {
			return ctx.Status(400).JSON(errors2.InvalidRevisionError)
		}

		var pad, errorForPad2 = utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager)
		if errorForPad2 != nil {
			return ctx.Status(404).JSON(errors2.PadNotFoundError)
		}
		var head = pad.Head

		if *revNum > head {
			return ctx.Status(400).JSON(errors2.RevisionHigherThanHeadError)
		}

		var revision, err = pad.GetRevision(*revNum)
		if err != nil {
			return ctx.Status(404).JSON(errors2.RevisionNotFoundError)
		}

		return ctx.JSON(revision.Changeset)
	})

	initStore.C.Post("/pads/:padId/text", func(ctx *fiber.Ctx) error {
		type Request struct {
			Text     string `json:"text"`
			AuthorId string `json:"authorId"`
		}

		var padId = ctx.Params("padId")
		var request Request
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
	})
}
