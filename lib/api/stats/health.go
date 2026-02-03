package stats

import (
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/gofiber/fiber/v3"
)

type DBChecker struct {
	db db.DataStore
}

func (d DBChecker) Name() string {
	return "database"
}

func (d DBChecker) Check() Check {
	err := d.db.Ping()

	if err != nil {
		return Check{
			Status: StatusFail,
			Output: err.Error(),
		}
	}

	return Check{
		Status:     StatusPass,
		Observed:   "ok",
		ObservedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

type EtherpadChecker struct {
	sessionStore *ws.SessionStore
}

func (e EtherpadChecker) Name() string {
	return "etherpad"
}

func (e EtherpadChecker) Check() Check {
	stats, err := e.sessionStore.GetStats()
	if err != nil {
		return Check{
			Status: StatusFail,
			Output: err.Error(),
		}
	}
	if stats.ActivePads < 0 {
		return Check{
			Status: StatusFail,
			Output: "invalid pad count",
		}
	}

	return Check{
		Status:   StatusPass,
		Observed: stats.ActivePads,
	}
}

// Handler godoc
// @Summary Health check endpoint
// @Description Returns the health status of the service (RFC Health Check Draft)
// @Tags Health
// @Produce json
// @Success 200 {object} HealthResponse "Service is healthy"
// @Failure 503 {object} HealthResponse "Service is unhealthy"
// @Router /health [get]
func Handler(
	version string,
	releaseID string,
	serviceID string,
	checkers []Checker,
) fiber.Handler {
	return func(c fiber.Ctx) error {
		resp := HealthResponse{
			Status:    StatusPass,
			Version:   version,
			ReleaseID: releaseID,
			ServiceID: serviceID,
			Checks:    map[string][]Check{},
		}

		httpStatus := fiber.StatusOK

		for _, checker := range checkers {
			check := checker.Check()
			resp.Checks[checker.Name()] = []Check{check}

			switch check.Status {
			case StatusFail:
				resp.Status = StatusFail
				httpStatus = fiber.StatusServiceUnavailable
			case StatusWarn:
				if resp.Status != StatusFail {
					resp.Status = StatusWarn
					httpStatus = fiber.StatusOK
				}
			}
		}

		return c.Status(httpStatus).JSON(resp)
	}
}
