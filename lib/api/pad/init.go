package pad

import (
	"errors"
	utils2 "github.com/ether/etherpad-go/lib/api/utils"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v2"
)

func getText(padId string, rev string) (*string, error) {
	if rev != "" {
		rev = utils.CheckValidRev(rev)
	}
	revNum, err := utils.CheckValidRev(rev)

	if err != nil {
		return nil, err
	}

	pad, err := utils2.GetPadSafe(padId, true, nil, nil)
	var head = pad.Head

	if rev != "" {
		if *revNum > head {
			return nil, errors.New("revision number is higher than head")
		}

		var atext = pad.get
	}

}

func Init(c *fiber.App) {
	c.Get("/pad/:padId/text", func(c *fiber.Ctx) error {

	})
}
