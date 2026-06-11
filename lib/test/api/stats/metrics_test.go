package stats

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/api/stats"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/require"
)

func TestAdminMessageHandlerAllMethods(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)

	defer testDb.StartTestDBHandler()

	testDb.AddTests(
		testutils.TestRunConfig{
			Name: "Metrics Endpoint Exists",
			Test: testMetricsEndpointExists,
		},
		testutils.TestRunConfig{
			Name: "Health Endpoint Exists",
			Test: testHealthendpointExists,
		},
		testutils.TestRunConfig{
			Name: "GetStats endpoint returns instance stats",
			Test: testGetStatsEndpoint,
		},
	)

}

func testMetricsEndpointExists(t *testing.T, testDb testutils.TestDataStore) {
	stats.Init(testDb.ToInitStore())

	req := httptest.NewRequest("GET", "/metrics", nil)

	resp, err := testDb.App.Test(req)
	require.NoError(t, err)

	body := make([]byte, resp.ContentLength)
	_, err = resp.Body.Read(body)
	require.NoError(t, err)

	output := string(body)

	require.Equal(t, 200, resp.StatusCode)
	require.Contains(t, output, "etherpad_active_pads 0")
	require.Contains(t, output, "etherpad_total_users 0")
}

func testHealthendpointExists(t *testing.T, testDb testutils.TestDataStore) {
	stats.Init(testDb.ToInitStore())

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := testDb.App.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}

func testGetStatsEndpoint(t *testing.T, testDb testutils.TestDataStore) {
	initStore := testDb.ToInitStore()
	stats.Init(initStore)

	// Create a pad so totalPads is at least 1
	_, err := testDb.PadManager.GetPad("statspad", nil, nil)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/admin/api/stats", nil)
	resp, err := initStore.C.Test(req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	var response stats.StatsResponse
	body, _ := io.ReadAll(resp.Body)
	require.NoError(t, json.Unmarshal(body, &response))
	require.GreaterOrEqual(t, response.TotalPads, 1)
	require.GreaterOrEqual(t, response.TotalSessions, 0)
	require.GreaterOrEqual(t, response.TotalActivePads, 0)
}
