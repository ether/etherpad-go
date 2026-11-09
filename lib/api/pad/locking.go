package pad

import (
	"encoding/json"

	"github.com/ether/etherpad-go/lib"
	padlocks "github.com/ether/etherpad-go/lib/pad"
	"github.com/gofiber/fiber/v3"
)

// The HTTP API changes pads outside the websocket's USER_CHANGES path. These
// wrappers make every :padId route take the same per-pad lock as the
// websocket handlers (pad.Manager.LockPad), so API writes can't interleave
// with a commit being rebased and appended, and reads see a consistent pad.

// writeLocked runs h while holding the write lock of the :padId pad.
func writeLocked(initStore *lib.InitStore, h fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		unlock, err := initStore.PadManager.LockPad(c.Params("padId"))
		if err != nil {
			return lockFailed(c, err)
		}
		defer unlock()
		return h(c)
	}
}

// readLocked runs h while holding the read lock of the :padId pad.
func readLocked(initStore *lib.InitStore, h fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		unlock, err := initStore.PadManager.RLockPad(c.Params("padId"))
		if err != nil {
			return lockFailed(c, err)
		}
		defer unlock()
		return h(c)
	}
}

// writeLockedWithDestination locks both the :padId pad and the destination
// pad named in the request body (copy/move). LockPad orders the IDs, so
// concurrent A->B and B->A requests can't deadlock.
func writeLockedWithDestination(initStore *lib.InitStore, h fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		var body struct {
			DestinationID string `json:"destinationID"`
		}
		// A malformed body is reported by the handler's own Bind().
		_ = json.Unmarshal(c.Body(), &body)
		unlock, err := initStore.PadManager.LockPad(c.Params("padId"), body.DestinationID)
		if err != nil {
			return lockFailed(c, err)
		}
		defer unlock()
		return h(c)
	}
}

// lockFailed answers a request whose pad could not be locked: the database
// is unreachable, so the client should retry.
func lockFailed(c fiber.Ctx, err error) error {
	return c.Status(padlocks.UnavailableStatus).JSON(fiber.Map{"code": 2, "message": err.Error(), "data": nil})
}
