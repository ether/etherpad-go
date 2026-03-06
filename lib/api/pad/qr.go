package pad

import (
	"net/url"
	"strings"

	"github.com/ether/etherpad-go/lib"
	"github.com/gofiber/fiber/v2"
	qrcode "github.com/skip2/go-qrcode"
)

func HandlePadQr(c *fiber.Ctx, store *lib.InitStore) error {
	rawPadID := c.Params("pad")
	if rawPadID == "" {
		return c.SendStatus(fiber.StatusNotFound)
	}

	targetPadID := rawPadID
	readOnlyPadID := rawPadID
	isReadOnlyRoute := store.ReadOnlyManager.IsReadOnlyID(&rawPadID)

	if isReadOnlyRoute {
		padID, err := store.ReadOnlyManager.GetPadId(rawPadID)
		if err != nil || padID == nil {
			return c.SendStatus(fiber.StatusNotFound)
		}
		targetPadID = *padID
	} else {
		exists, err := store.PadManager.DoesPadExist(rawPadID)
		if err != nil || exists == nil || !*exists {
			return c.SendStatus(fiber.StatusNotFound)
		}
		readOnlyPadID = store.ReadOnlyManager.GetReadOnlyId(rawPadID)
	}

	useReadOnly := isReadOnlyRoute || strings.EqualFold(c.Query("readonly"), "true")
	linkPadID := targetPadID
	if useReadOnly {
		linkPadID = readOnlyPadID
	}

	targetURL := c.BaseURL() + "/p/" + url.PathEscape(linkPadID)
	png, err := qrcode.Encode(targetURL, qrcode.Highest, 1024)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not generate QR code")
	}

	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Type("png").Send(png)
}
