package stats

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/api/stats"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's health spec.
func TestHealth(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "/health", Test: testHealth})
	defer testDb.StartTestDBHandler()
}

func testHealth(t *testing.T, ds testutils.TestDataStore) {
	store := ds.ToInitStore()
	ds.Settings.LoadTest = false
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		return pad.CheckAccessWithHooks(c, ds.Logger, ds.Settings, ds.ReadOnlyManager, ds.Hooks)
	})
	store.C = app
	stats.Init(store)
	_, releaseID := settings.BuildInfo()

	get := func() (int, map[string]any, string, string) {
		resp, err := app.Test(httptest.NewRequest("GET", "/health", nil))
		require.NoError(t, err)
		b, _ := io.ReadAll(resp.Body)
		var body map[string]any
		_ = json.Unmarshal(b, &body)
		return resp.StatusCode, body, resp.Header.Get("Content-Type"), resp.Header.Get("Set-Cookie")
	}

	status, body, ctype, cookie := get()
	assert.Equal(t, 200, status)
	assert.Equal(t, "application/health+json", ctype)
	assert.Equal(t, "pass", body["status"])
	if releaseID != "" {
		assert.Equal(t, releaseID, body["releaseId"])
	} else {
		assert.Empty(t, body["releaseId"], "test binaries carry no VCS revision")
	}
	assert.Empty(t, cookie, "no cookie is set")

	// no sign-in needed, as for the Kubernetes probes
	ds.Settings.RequireAuthentication = true
	ds.Settings.RequireAuthorization = true
	status, body, _, cookie = get()
	assert.Equal(t, 200, status)
	assert.Equal(t, "pass", body["status"])
	assert.Empty(t, cookie)

	// Anonymous clients get no operational detail (active pads, cluster
	// members, database errors), as in etherpad-lite: status and releaseId.
	for _, k := range []string{"checks", "version", "serviceId"} {
		assert.NotContains(t, body, k)
	}

	// Admins get every check on the admin API (its auth is tested with
	// the admin routes; here the handler's answer).
	resp, err := ds.App.Test(httptest.NewRequest("GET", "/admin/api/health", nil))
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	var detailed map[string]any
	require.NoError(t, json.Unmarshal(b, &detailed), string(b))
	checks, ok := detailed["checks"].(map[string]any)
	require.True(t, ok, string(b))
	assert.Contains(t, checks, "database")
	assert.Contains(t, checks, "etherpad")
}

// A failing check fails the public answer too, with the same status, but
// without the error text.
func TestHealthPublicHidesErrors(t *testing.T) {
	app := fiber.New()
	app.Get("/health", stats.Handler("1.0", "rel", "svc", []stats.Checker{failing{}}, false))
	resp, err := app.Test(httptest.NewRequest("GET", "/health", nil))
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	assert.Equal(t, 503, resp.StatusCode)
	assert.JSONEq(t, `{"status":"fail","releaseId":"rel"}`, string(b))
}

type failing struct{}

func (failing) Name() string { return "database" }
func (failing) Check() stats.Check {
	return stats.Check{Status: stats.StatusFail, Output: "dial tcp 10.0.0.5:5432: connection refused"}
}
