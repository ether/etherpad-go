package pad

import (
	"encoding/json"

	"github.com/ether/etherpad-go/lib"
	"github.com/gofiber/fiber/v3"
)

// The HTTP API changes pads outside the websocket's USER_CHANGES path. These
// wrappers make every :padId route take the same per-pad lock as the
// websocket handlers (pad.Manager.LockPad), so API writes can't interleave
// with a commit being rebased and appended, and reads see a consistent pad.

// writeLocked runs h while holding the write lock of the :padId pad.
func writeLocked(initStore *lib.InitStore, h fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		unlock := initStore.PadManager.LockPad(c.Params("padId"))
		defer unlock()
		return h(c)
	}
}

// readLocked runs h while holding the read lock of the :padId pad.
func readLocked(initStore *lib.InitStore, h fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		unlock := initStore.PadManager.RLockPad(c.Params("padId"))
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
		unlock := initStore.PadManager.LockPad(c.Params("padId"), body.DestinationID)
		defer unlock()
		return h(c)
	}
}
