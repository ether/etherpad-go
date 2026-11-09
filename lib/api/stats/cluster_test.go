package stats

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/cluster"
	"github.com/ether/etherpad-go/lib/lifecycle"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBus struct {
	cluster.Local
	members int
}

func (f fakeBus) Enabled() bool { return true }
func (f fakeBus) Members() int  { return f.members }

type fixedCheck struct{ status HealthStatus }

func (f fixedCheck) Name() string { return "database" }
func (f fixedCheck) Check() Check { return Check{Status: f.status, Output: "down"} }

func TestClusterChecker(t *testing.T) {
	single := ClusterChecker{Clustered: false, Bus: cluster.Local{}}.Check()
	assert.Equal(t, StatusPass, single.Status)
	assert.Equal(t, ClusterObservation{Members: 1}, single.Observed)

	running := ClusterChecker{Clustered: true, Bus: fakeBus{members: 3}}.Check()
	assert.Equal(t, StatusPass, running.Status)
	assert.Equal(t, ClusterObservation{Enabled: true, Members: 3}, running.Observed)

	// Clustered database but no bus: correct, only slower, so a warning.
	missing := ClusterChecker{Clustered: true, Bus: cluster.Local{}}.Check()
	assert.Equal(t, StatusWarn, missing.Status)
}

func probe(t *testing.T, h fiber.Handler) (int, map[string]any) {
	t.Helper()
	app := fiber.New()
	app.Get("/p", h)
	resp, err := app.Test(httptest.NewRequest("GET", "/p", nil))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	require.NoError(t, json.Unmarshal(body, &out))
	return resp.StatusCode, out
}

func TestReadyHandlerFollowsTheLifecycle(t *testing.T) {
	state := &lifecycle.State{}
	ready := ReadyHandler(state, fixedCheck{StatusPass})

	code, body := probe(t, ready)
	assert.Equal(t, 503, code, "not ready while starting")
	assert.Equal(t, "starting", body["output"])

	state.MarkStarted()
	code, _ = probe(t, ready)
	assert.Equal(t, 200, code)

	code, body = probe(t, ReadyHandler(state, fixedCheck{StatusFail}))
	assert.Equal(t, 503, code, "not ready without database")
	assert.Contains(t, body["output"], "database")

	state.BeginDrain()
	code, body = probe(t, ready)
	assert.Equal(t, 503, code, "not ready while shutting down")
	assert.Equal(t, "shutting down", body["output"])

	code, _ = probe(t, LiveHandler())
	assert.Equal(t, 200, code, "still alive while shutting down")
}
