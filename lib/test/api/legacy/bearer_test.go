package legacy

import (
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/api/legacy"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
)

// Port of etherpad-lite's api/jwtAdminClaim for the API's side: without an
// API key the API takes a bearer token, checked by the authenticator.
func TestLegacyBearerAuth(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "bearer tokens", Test: testLegacyBearer})
	testDb.StartTestDBHandler()
}

func testLegacyBearer(t *testing.T, ds testutils.TestDataStore) {
	store := ds.ToInitStore()
	legacy.New(store, "", func(token string) bool { return token == "good" }).Register(store.C)
	c := &client{t: t, app: store.C, version: legacy.LatestVersion}

	call := func(header string) (int, legacy.Response) {
		req := httptest.NewRequest("GET", c.path("listAllPads"), nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		status, r, _ := c.do(req)
		return status, r
	}
	status, r := call("")
	assert.Equal(t, 401, status)
	assert.Equal(t, "no or wrong API Key", r.Message)
	status, r = call("Bearer bad")
	assert.Equal(t, 401, status)
	assert.Equal(t, 4, r.Code)
	assert.Equal(t, "no or wrong OAuth token", r.Message)
	status, r = call("Bearer good")
	assert.Equal(t, 200, status)
	assert.Equal(t, 0, r.Code)
	status, _ = call("good")
	assert.Equal(t, 200, status, "the Bearer prefix may be left out, as in etherpad-lite")

	// an apikey parameter is no way in when the API key is not used
	status, _, _ = c.getStatus("listAllPads", map[string]string{"apikey": "good"})
	assert.Equal(t, 401, status)
}
