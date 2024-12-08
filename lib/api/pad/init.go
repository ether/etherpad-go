package pad

import (
	"errors"
	utils2 "github.com/ether/etherpad-go/lib/api/utils"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v2"
)

func getText(padId string, rev *string) (*string, error) {
	var revNum *int = nil
	if rev != nil {
		revPoint, err := utils.CheckValidRev(*rev)
		revNum = revPoint
		if err != nil {
			return nil, err
		}
	}

	pad, err := utils2.GetPadSafe(padId, true, nil, nil)

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
	var text = 
}

func Init(c *fiber.App) {
	c.Get("/pad/:padId/text", func(c *fiber.Ctx) error {

	})
}
