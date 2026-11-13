package legacy

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/api/legacy"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of the OpenAPI part of etherpad-lite's api/api spec.
func TestLegacyOpenAPI(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "the document is complete", Test: testLegacyOpenAPIDocument},
		testutils.TestRunConfig{Name: "bearer mode documents bearer auth", Test: testLegacyOpenAPIBearer},
	)
	testDb.StartTestDBHandler()
}

func getDefinition(t *testing.T, c *client, path string) map[string]any {
	t.Helper()
	resp, err := c.app.Test(httptest.NewRequest("GET", path, nil))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	var raw map[string]any
	require.NoError(t, jsonDecode(resp.Body, &raw))
	return raw
}

func mapAt(t *testing.T, m map[string]any, keys ...string) map[string]any {
	t.Helper()
	for _, k := range keys {
		next, ok := m[k].(map[string]any)
		require.True(t, ok, "%s is an object", k)
		m = next
	}
	return m
}

func testLegacyOpenAPIDocument(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	doc := getDefinition(t, c, "/api/openapi.json")
	assert.True(t, strings.HasPrefix(doc["openapi"].(string), "3."))
	assert.NotEmpty(t, mapAt(t, doc, "info")["title"])
	assert.Equal(t, "1.3.1", mapAt(t, doc, "info")["version"])

	var names []string
	for _, tag := range doc["tags"].([]any) {
		names = append(names, tag.(map[string]any)["name"].(string))
	}
	assert.Subset(t, names, []string{"pad", "author", "session", "group", "chat", "server"})

	paths := mapAt(t, doc, "paths")
	assert.Len(t, paths, 50, "every function of 1.3.1")
	refs := mapAt(t, doc, "components", "responses")
	for path, item := range paths {
		ops := item.(map[string]any)
		require.Len(t, ops, 1, "%s: POST only", path)
		op := mapAt(t, ops, "post")
		assert.NotEmpty(t, op["tags"], path)
		assert.GreaterOrEqual(t, len(op["summary"].(string)), 3, path)
		assert.Equal(t, strings.TrimPrefix(path, "/"), op["operationId"])
		for _, r := range mapAt(t, op, "responses") {
			ref := r.(map[string]any)["$ref"].(string)
			assert.Contains(t, refs, strings.TrimPrefix(ref, "#/components/responses/"), "%s: %s resolves", path, ref)
		}
	}
	assert.Contains(t, paths, "/compactPad", "1.3.1 functions")
	params := mapAt(t, paths, "/setText", "post")["parameters"].([]any)
	assert.Len(t, params, 3, "padID, text, authorId")

	schemes := mapAt(t, doc, "components", "securitySchemes")
	found := map[string]bool{}
	for _, s := range schemes {
		m := s.(map[string]any)
		found[m["in"].(string)+":"+m["name"].(string)] = true
	}
	assert.True(t, found["query:apikey"])
	assert.True(t, found["query:api_key"])
	assert.True(t, found["header:apikey"])

	old := getDefinition(t, c, "/api/1.2.1/openapi.json")
	oldPaths := mapAt(t, old, "paths")
	assert.Contains(t, oldPaths, "/listAllPads")
	assert.NotContains(t, oldPaths, "/getStats", "getStats is from 1.2.14")
	assert.Len(t, mapAt(t, oldPaths, "/setText", "post")["parameters"], 2, "no authorId before 1.3.0")
	assert.Equal(t, "http://example.com/api/1.2.1", old["servers"].([]any)[0].(map[string]any)["url"])

	// GET and POST both reach the functions
	assert.Equal(t, 0, c.get("checkToken", nil).Code)
	assert.Equal(t, 0, c.postJSON("checkToken", nil).Code)
}

func testLegacyOpenAPIBearer(t *testing.T, ds testutils.TestDataStore) {
	store := ds.ToInitStore()
	legacy.New(store, "", func(string) bool { return false }).Register(store.C)
	c := &client{t: t, app: store.C, version: legacy.LatestVersion}
	schemes := mapAt(t, getDefinition(t, c, "/api/openapi.json"), "components", "securitySchemes")
	assert.Contains(t, schemes, "Bearer")
	assert.NotContains(t, schemes, "ApiKey")
}
