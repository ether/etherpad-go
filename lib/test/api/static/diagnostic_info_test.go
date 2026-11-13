package static

import (
	"net/http/httptest"
	"strings"
	"testing"

	staticapi "github.com/ether/etherpad-go/lib/api/static"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's apicalls spec. The pad posts here when its
// connection is lost; etherpad-go used to answer 404.
func TestConnectionDiagnosticInfo(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "connection diagnostic info", Test: testConnectionDiagnosticInfo})
	defer testDb.StartTestDBHandler()
}

func testConnectionDiagnosticInfo(t *testing.T, ds testutils.TestDataStore) {
	store := ds.ToInitStore()
	staticapi.Init(store)
	post := func(body string) int {
		req := httptest.NewRequest("POST", "/ep/pad/connection-diagnostic-info", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := store.C.Test(req)
		require.NoError(t, err)
		return resp.StatusCode
	}
	assert.Equal(t, 200, post(`{"diagnosticInfo":{"disconnectedMessage":"socket.io timeout"}}`))
	assert.Equal(t, 400, post(`{}`))
	assert.Equal(t, 400, post(`{"diagnosticInfo":"text"}`))
	assert.Equal(t, 400, post(`not json`))
}
