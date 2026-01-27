package stats

import (
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
	)

}

func testMetricsEndpointExists(t *testing.T, testDb testutils.TestDataStore) {
	stats.Init(testDb.ToInitStore())

	req := httptest.NewRequest("GET", "/metrics", nil)

	resp, err := testDb.App.Test(req, 1000)
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
	resp, err := testDb.App.Test(req, 1000)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
}
