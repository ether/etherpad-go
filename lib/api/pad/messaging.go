package pad

import (
	"github.com/ether/etherpad-go/lib"
	errors2 "github.com/ether/etherpad-go/lib/api/errors"
	utils2 "github.com/ether/etherpad-go/lib/api/utils"
	"github.com/gofiber/fiber/v3"
)

// SendClientsMessageRequest carries the custom message type to broadcast.
type SendClientsMessageRequest struct {
	Msg string `json:"msg"`
}

// SendClientsMessage godoc
// @Summary Send a custom message to all clients of a pad
// @Description Broadcasts a custom COLLABROOM message type to every client connected to the pad (original API sendClientsMessage)
// @Tags Pads
// @Accept json
// @Produce json
// @Param padId path string true "Pad ID"
// @Param request body SendClientsMessageRequest true "Message type"
// @Success 200 {string} string "OK"
// @Failure 400 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/pads/{padId}/sendClientsMessage [post]
func SendClientsMessage(initStore *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padId := c.Params("padId")
		var request SendClientsMessageRequest
		if err := c.Bind().Body(&request); err != nil {
			return c.Status(400).JSON(errors2.InvalidRequestError)
		}
		if request.Msg == "" {
			return c.Status(400).JSON(errors2.NewMissingParamError("msg"))
		}

		if _, err := utils2.GetPadSafe(padId, true, nil, nil, initStore.PadManager); err != nil {
			return c.Status(404).JSON(errors2.PadNotFoundError)
		}

		initStore.Handler.SendCustomMessageToPad(padId, request.Msg)
		return c.SendStatus(200)
	}
}
