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
}
