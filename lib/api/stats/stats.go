package stats

import (
	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/api/errors"
	"github.com/gofiber/fiber/v3"
)

// StatsResponse mirrors the original getStats API payload.
type StatsResponse struct {
	TotalPads       int `json:"totalPads"`
	TotalSessions   int `json:"totalSessions"`
	TotalActivePads int `json:"totalActivePads"`
}

// GetStats godoc
// @Summary Instance statistics
// @Description Returns the total number of pads, connected sessions and pads with connected users
// @Tags Stats
// @Produce json
// @Success 200 {object} StatsResponse
// @Failure 500 {object} errors.Error
// @Security BearerAuth
// @Router /admin/api/stats [get]
func GetStats(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		padIds, err := store.Store.GetPadIds()
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		sessionStats, err := store.Handler.SessionStore.GetStats()
		if err != nil {
			return c.Status(500).JSON(errors.InternalServerError)
		}
		totalPads := 0
		if padIds != nil {
			totalPads = len(*padIds)
		}
		return c.JSON(StatsResponse{
			TotalPads:       totalPads,
			TotalSessions:   sessionStats.ActiveUsers,
			TotalActivePads: sessionStats.ActivePads,
		})
	}
}
