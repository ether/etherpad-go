package stats

import (
	"github.com/ether/etherpad-go/lib/cluster"
	"github.com/ether/etherpad-go/lib/lifecycle"
	"github.com/gofiber/fiber/v3"
)

// ClusterChecker reports how this instance sees the cluster. A missing bus
// is only a warning: edits stay correct through the database, other
// instances just see them later.
type ClusterChecker struct {
	Clustered       bool
	Bus             cluster.Bus
	RetentionLeader *cluster.Elector
}

func (c ClusterChecker) Name() string { return "cluster" }

// ClusterObservation is the observedValue of the cluster check.
type ClusterObservation struct {
	Enabled         bool `json:"enabled"`
	Members         int  `json:"members"`
	RetentionLeader bool `json:"retentionLeader"`
}

func (c ClusterChecker) Check() Check {
	obs := ClusterObservation{Members: 1}
	if c.Bus != nil && c.Bus.Enabled() {
		obs.Enabled = true
		obs.Members = c.Bus.Members()
	}
	if c.RetentionLeader != nil {
		obs.RetentionLeader = c.RetentionLeader.IsLeader()
	}
	if c.Clustered && !obs.Enabled {
		return Check{Status: StatusWarn, Observed: obs,
			Output: "cluster bus not running: other instances see changes from this one only after a delay"}
	}
	return Check{Status: StatusPass, Observed: obs}
}

// LiveHandler answers the liveness probe: the process serves requests.
// It checks nothing else, so a database outage does not get every pod
// restarted at once.
func LiveHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": StatusPass})
	}
}

// ReadyHandler answers the readiness probe: 200 while this instance should
// get traffic, 503 while it starts, shuts down or cannot reach the database.
func ReadyHandler(state *lifecycle.State, db Checker) fiber.Handler {
	return func(c fiber.Ctx) error {
		switch {
		case state.Draining():
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": StatusFail, "output": "shutting down"})
		case !state.Ready():
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": StatusFail, "output": "starting"})
		}
		if check := db.Check(); check.Status == StatusFail {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": StatusFail, "output": "database: " + check.Output})
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": StatusPass})
	}
}
