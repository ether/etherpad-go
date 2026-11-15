package legacy

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ether/etherpad-go/lib/api/legacy"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The REST-style paths of etherpad-lite: /rest/{version}/{resource}/{action}
// (openapi.ts) and /api/2/... with /api-docs.json (RestAPI.ts).
func TestLegacyREST(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "/rest paths call the functions", Test: testRESTPaths},
		testutils.TestRunConfig{Name: "/rest/openapi.json uses the /rest paths", Test: testRESTDefinition},
		testutils.TestRunConfig{Name: "/api/2 routes call the functions", Test: testAPI2Routes},
		testutils.TestRunConfig{Name: "/api/2 needs an API key", Test: testAPI2NeedsKey},
		testutils.TestRunConfig{Name: "/api-docs.json documents /api/2", Test: testAPI2Definition},
	)
	testDb.StartTestDBHandler()
}

func testRESTPaths(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	path := "/rest/" + legacy.LatestVersion

	_, r, _ := c.do(httptest.NewRequest("GET", path+"/pad/create?padID=restPad&text=hello&apikey="+url.QueryEscape(c.key), nil))
	require.Equal(t, 0, r.Code, r.Message)

	req := httptest.NewRequest("POST", path+"/pad/getText?apikey="+url.QueryEscape(c.key), bytes.NewReader([]byte(`{"padID":"restPad"}`)))
	req.Header.Set("Content-Type", "application/json")
	_, r, _ = c.do(req)
	require.Equal(t, 0, r.Code, r.Message)
	assert.Equal(t, "hello\n", r.Data.(map[string]any)["text"])

	// lite's regression: checkToken stays under pad/.
	_, r, _ = c.do(httptest.NewRequest("GET", path+"/pad/checkToken?apikey="+url.QueryEscape(c.key), nil))
	assert.Equal(t, 0, r.Code, r.Message)

	status, r, _ := c.do(httptest.NewRequest("GET", path+"/pad/nothing?apikey="+url.QueryEscape(c.key), nil))
	assert.Equal(t, 404, status)
	assert.Equal(t, 3, r.Code)

	// A function the version does not have.
	status, r, _ = c.do(httptest.NewRequest("GET", "/rest/1/pad/listAll?apikey="+url.QueryEscape(c.key), nil))
	assert.Equal(t, 404, status)
	assert.Equal(t, 3, r.Code)

	status, r, _ = c.do(httptest.NewRequest("GET", path+"/pad/getText?padID=restPad", nil))
	assert.Equal(t, 401, status)
	assert.Equal(t, 4, r.Code)
}

func testRESTDefinition(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	for _, p := range []string{"/rest/openapi.json", "/rest/" + legacy.LatestVersion + "/openapi.json"} {
		doc := getDefinition(t, c, p)
		paths := mapAt(t, doc, "paths")
		assert.Contains(t, paths, "/group/create", p)
		assert.Contains(t, paths, "/pad/checkToken", p)
		assert.NotContains(t, paths, "/createGroup", p)
		assert.Len(t, paths, 50, p)
		server := doc["servers"].([]any)[0].(map[string]any)["url"].(string)
		assert.Contains(t, server, "/rest/"+legacy.LatestVersion, p)
	}
	paths := mapAt(t, getDefinition(t, c, "/rest/1/openapi.json"), "paths")
	assert.NotContains(t, paths, "/pad/listAll", "1.2.1 added listAllPads")
}

func api2(c *client, method, path string, body map[string]any) (int, legacy.Response) {
	c.t.Helper()
	target := "/api/2" + path
	sep := "?"
	if bytes.ContainsRune([]byte(path), '?') {
		sep = "&"
	}
	target += sep + "apikey=" + url.QueryEscape(c.key)
	var r *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	} else {
		r = bytes.NewReader(nil)
	}
	hr := httptest.NewRequest(method, target, r)
	if body != nil {
		hr.Header.Set("Content-Type", "application/json")
	}
	status, resp, _ := c.do(hr)
	return status, resp
}

func testAPI2Routes(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)

	_, r := api2(c, "POST", "/pads", map[string]any{"padID": "api2Pad", "text": "one"})
	require.Equal(t, 0, r.Code, r.Message)

	_, r = api2(c, "PATCH", "/pads/text", map[string]any{"padID": "api2Pad", "text": "two"})
	require.Equal(t, 0, r.Code, r.Message)

	_, r = api2(c, "GET", "/pads/text?padID=api2Pad", nil)
	require.Equal(t, 0, r.Code, r.Message)
	assert.Equal(t, "onetwo\n", r.Data.(map[string]any)["text"])

	_, r = api2(c, "POST", "/groups", nil)
	require.Equal(t, 0, r.Code, r.Message)
	groupID := r.Data.(map[string]any)["groupID"].(string)

	_, r = api2(c, "GET", "/groups", nil)
	require.Equal(t, 0, r.Code, r.Message)
	assert.Contains(t, r.Data.(map[string]any)["groupIDs"], groupID)

	_, r = api2(c, "DELETE", "/groups", map[string]any{"groupID": groupID})
	assert.Equal(t, 0, r.Code, r.Message)

	_, r = api2(c, "DELETE", "/pads", map[string]any{"padID": "api2Pad"})
	assert.Equal(t, 0, r.Code, r.Message)

	// lite answers unknown routes and methods with code 1, "not found".
	status, r := api2(c, "PUT", "/pads", nil)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, r.Code)
	assert.Equal(t, "not found", r.Message)
}

func testAPI2NeedsKey(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	status, r, _ := c.do(httptest.NewRequest("GET", "/api/2/groups", nil))
	assert.Equal(t, 401, status)
	assert.Equal(t, 4, r.Code)
}

func testAPI2Definition(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	doc := getDefinition(t, c, "/api-docs.json")
	server := doc["servers"].([]any)[0].(map[string]any)["url"].(string)
	assert.Contains(t, server, "/api/2")

	// lite's api/api spec: apikey and api_key in the query, apikey header.
	schemes := mapAt(t, doc, "components", "securitySchemes")
	var query, alias, header bool
	for _, s := range schemes {
		m := s.(map[string]any)
		if m["type"] != "apiKey" {
			continue
		}
		query = query || (m["in"] == "query" && m["name"] == "apikey")
		alias = alias || (m["in"] == "query" && m["name"] == "api_key")
		header = header || (m["in"] == "header" && m["name"] == "apikey")
	}
	assert.True(t, query && alias && header, "%v", schemes)

	paths := mapAt(t, doc, "paths")
	pads := mapAt(t, paths, "/pads")
	assert.Contains(t, pads, "get")
	assert.Contains(t, pads, "post")
	assert.Contains(t, pads, "delete")
	text := mapAt(t, paths, "/pads/text")
	assert.Contains(t, text, "patch")
	assert.NotEmpty(t, mapAt(t, text, "post", "requestBody", "content", "application/json", "schema", "properties")["authorId"])
	params := mapAt(t, text, "get")["parameters"].([]any)
	assert.NotEmpty(t, params)
}
