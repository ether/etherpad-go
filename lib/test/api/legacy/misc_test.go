package legacy

import (
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ports of etherpad-lite's api/api, api/chat, api/characterEncoding,
// api/appendTextAuthor, api/createDiffHTML, api/restoreRevision and
// api/instance specs.
func TestLegacyAPIMisc(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "versions, functions and keys", Test: testLegacyVersionsAndAuth},
		testutils.TestRunConfig{Name: "parameters from query, form, url-encoded and JSON", Test: testLegacyParameterSources},
		testutils.TestRunConfig{Name: "chat", Test: testLegacyChat},
		testutils.TestRunConfig{Name: "emojis survive setHTML and getHTML", Test: testLegacyCharacterEncoding},
		testutils.TestRunConfig{Name: "appendText attribution", Test: testLegacyAppendTextAuthor},
		testutils.TestRunConfig{Name: "createDiffHTML", Test: testLegacyCreateDiffHTML},
		testutils.TestRunConfig{Name: "restoreRevision and API versions", Test: testLegacyRestoreRevisionVersions},
		testutils.TestRunConfig{Name: "getStats", Test: testLegacyGetStats},
		testutils.TestRunConfig{Name: "revision and read-only functions", Test: testLegacyRevisionFunctions},
	)
	testDb.StartTestDBHandler()
}

func testLegacyVersionsAndAuth(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)

	// /api names the current version
	for _, path := range []string{"/api", "/api/"} {
		resp, err := c.app.Test(httptest.NewRequest("GET", path, nil))
		require.NoError(t, err)
		require.Equal(t, 200, resp.StatusCode)
		var body map[string]string
		require.NoError(t, jsonDecode(resp.Body, &body))
		assert.Equal(t, "1.3.1", body["currentVersion"])
	}

	// every older version works
	for _, v := range []string{"1", "1.1", "1.2", "1.2.1", "1.2.7", "1.2.8", "1.2.9", "1.2.10", "1.2.11", "1.2.12", "1.2.13", "1.2.14", "1.2.15", "1.3.0"} {
		c.version = v
		ok(t, c.get("createGroup", nil))
	}

	c.version = "1"
	status, r, _ := c.getStatus("listAllPads", nil)
	assert.Equal(t, 404, status, "listAllPads is from 1.2.1")
	assert.Equal(t, 3, r.Code)
	assert.Equal(t, "no such function", r.Message)

	c.version = "9.9"
	status, r, _ = c.getStatus("listAllPads", nil)
	assert.Equal(t, 404, status)
	assert.Equal(t, 3, r.Code)
	assert.Equal(t, "no such api version", r.Message)

	c.version = "1.3.1"
	status, r, _ = c.getStatus("noSuchFunction", nil)
	assert.Equal(t, 404, status)
	assert.Equal(t, 3, r.Code)

	c.key = "wrong"
	status, r, _ = c.getStatus("createPad", map[string]string{"padID": "test"})
	assert.Equal(t, 401, status)
	assert.Equal(t, 4, r.Code)
	assert.Equal(t, "no or wrong API Key", r.Message)
	c.key = ""
	status, r, _ = c.getStatus("createPad", map[string]string{"padID": "test"})
	assert.Equal(t, 401, status)
	assert.Equal(t, 4, r.Code)

	// api_key and the Authorization header are accepted too
	c.key = ""
	status, r, _ = c.getStatus("listAllPads", map[string]string{"api_key": testAPIKey})
	assert.Equal(t, 200, status)
	assert.Equal(t, 0, r.Code)
	req := httptest.NewRequest("GET", c.path("listAllPads"), nil)
	req.Header.Set("Authorization", testAPIKey)
	status, r, _ = c.do(req)
	assert.Equal(t, 200, status)
	assert.Equal(t, 0, r.Code)

	// checkToken answers null
	c.key = testAPIKey
	r = c.get("checkToken", nil)
	assert.Equal(t, 0, r.Code)
	assert.Nil(t, r.Data)

	// a wrong parameter is code 1 with HTTP 200
	status, r, _ = c.getStatus("getText", map[string]string{"padID": "doesNotExist"})
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, r.Code)
	assert.Equal(t, "padID does not exist", r.Message)
	failed(t, c.get("getText", nil), "padID is not a string")
}

func testLegacyParameterSources(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	padID := makeID()
	ok(t, c.get("createPad", map[string]string{"padID": padID}))
	ok(t, c.postURLEncoded("setText", map[string]string{"padID": padID, "text": "urlencoded"}))
	assert.Equal(t, "urlencoded\n", ok(t, c.get("getText", map[string]string{"padID": padID}))["text"])
	ok(t, c.postForm("setText", nil, map[string]string{"padID": padID, "text": "multipart"}))
	assert.Equal(t, "multipart\n", ok(t, c.get("getText", map[string]string{"padID": padID}))["text"])
	ok(t, c.postJSON("setText", map[string]any{"padID": padID, "text": "json"}))
	assert.Equal(t, "json\n", ok(t, c.get("getText", map[string]string{"padID": padID}))["text"])
	// the body wins over the query
	ok(t, c.postForm("setText", map[string]string{"text": "query"}, map[string]string{"padID": padID, "text": "body"}))
	assert.Equal(t, "body\n", ok(t, c.get("getText", map[string]string{"padID": padID}))["text"])
	// JSON booleans and numbers
	ok(t, c.postJSON("copyPad", map[string]any{"sourceID": padID, "destinationID": padID + "c", "force": true}))
	ok(t, c.postJSON("saveRevision", map[string]any{"padID": padID, "rev": 1}))
	assert.Equal(t, []any{float64(1)}, ok(t, c.get("listSavedRevisions", map[string]string{"padID": padID}))["savedRevisions"])
}

func testLegacyChat(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	padID := makeID()
	ok(t, c.get("createPad", map[string]string{"padID": padID}))
	authorID := ok(t, c.get("createAuthor", nil))["authorID"].(string)
	assert.EqualValues(t, -1, ok(t, c.get("getChatHead", map[string]string{"padID": padID}))["chatHead"])
	timestamp := time.Now().UnixMilli()
	ok(t, c.get("appendChatMessage", map[string]string{
		"padID": padID, "text": "blalblalbha", "authorID": authorID, "time": strconv.FormatInt(timestamp, 10),
	}))
	assert.EqualValues(t, 0, ok(t, c.get("getChatHead", map[string]string{"padID": padID}))["chatHead"])
	messages := ok(t, c.get("getChatHistory", map[string]string{"padID": padID}))["messages"].([]any)
	require.Len(t, messages, 1)
	m := messages[0].(map[string]any)
	assert.Equal(t, "blalblalbha", m["text"])
	assert.Equal(t, authorID, m["userId"])
	assert.EqualValues(t, timestamp, m["time"])

	failed(t, c.get("getChatHistory", map[string]string{"padID": padID, "start": "-1", "end": "0"}), "start is below zero")
	failed(t, c.get("getChatHistory", map[string]string{"padID": padID, "start": "1", "end": "0"}), "start is higher than end")
	failed(t, c.get("appendChatMessage", map[string]string{"padID": padID, "authorID": authorID}), "text is not a string")
}

func testLegacyCharacterEncoding(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	padID := makeID()
	ok(t, c.get("createPad", map[string]string{"padID": padID}))
	html, err := os.ReadFile("testdata/emojis.html")
	require.NoError(t, err)
	ok(t, c.postJSON("setHTML", map[string]any{"padID": padID, "html": string(html)}))
	out := ok(t, c.get("getHTML", map[string]string{"padID": padID}))["html"].(string)
	assert.Regexp(t, `&#127484`, out, "characters outside the BMP are written as entities")
}

func testLegacyAppendTextAuthor(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	authorID := ok(t, c.get("createAuthor", map[string]string{"name": "TestAuthor"}))["authorID"].(string)
	padID := makeID()
	ok(t, c.get("createPad", map[string]string{"padID": padID}))
	ok(t, c.postJSON("appendText", map[string]any{"padID": padID, "text": "authored text", "authorId": authorID}))
	assert.Contains(t, ok(t, c.get("listAuthorsOfPad", map[string]string{"padID": padID}))["authorIDs"], authorID)

	other := "appendTextNoAuthor_" + makeID()
	ok(t, c.get("createPad", map[string]string{"padID": other}))
	ok(t, c.postJSON("appendText", map[string]any{"padID": other, "text": "anonymous text"}))
	assert.Len(t, ok(t, c.get("listAuthorsOfPad", map[string]string{"padID": other}))["authorIDs"], 0)

	// before 1.3.0 appendText takes no authorId
	c.version = "1.2.13"
	third := makeID()
	ok(t, c.get("createPad", map[string]string{"padID": third}))
	ok(t, c.postJSON("appendText", map[string]any{"padID": third, "text": "x", "authorId": authorID}))
	c.version = "1.3.1"
	assert.Len(t, ok(t, c.get("listAuthorsOfPad", map[string]string{"padID": third}))["authorIDs"], 0)
}

func testLegacyCreateDiffHTML(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	padID := makeID()
	ok(t, c.get("createPad", map[string]string{"padID": padID}))
	ok(t, c.postJSON("setText", map[string]any{"padID": padID, "text": "first revision"}))
	ok(t, c.postJSON("setText", map[string]any{"padID": padID, "text": "second revision"}))
	for _, r := range [][2]string{{"1", "2"}, {"1", "1"}, {"0", "2"}} {
		d := ok(t, c.get("createDiffHTML", map[string]string{"padID": padID, "startRev": r[0], "endRev": r[1]}))
		assert.NotEmpty(t, d["html"], "%v", r)
		assert.IsType(t, []any{}, d["authors"], "%v", r)
	}
}

func testLegacyRestoreRevisionVersions(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	authorID := ok(t, c.get("createAuthor", nil))["authorID"].(string)
	setup := func() string {
		padID := makeID()
		empty := ""
		p, err := ds.PadManager.GetPad(padID, &empty, nil)
		require.NoError(t, err)
		require.NoError(t, p.SetText(p.Text()+"\nfoo", nil))
		require.NoError(t, p.SetText(p.Text()+"\nbar", nil))
		require.Equal(t, 2, p.Head)
		return padID
	}
	revisionAuthor := func(padID string) string {
		p, err := ds.PadManager.GetPad(padID, nil, nil)
		require.NoError(t, err)
		rev, err := ds.DS.GetRevision(padID, p.Head)
		require.NoError(t, err)
		require.Equal(t, 3, p.Head)
		if rev.AuthorId == nil {
			return ""
		}
		return *rev.AuthorId
	}

	// v1.2.11: authorId is ignored
	c.version = "1.2.11"
	padID := setup()
	ok(t, c.get("restoreRevision", map[string]string{"padID": padID, "rev": "1", "authorId": authorID}))
	assert.Equal(t, "", revisionAuthor(padID))

	// v1.3.0: the restore is attributed to authorId, which may be omitted
	c.version = "1.3.0"
	padID = setup()
	ok(t, c.get("restoreRevision", map[string]string{"padID": padID, "rev": "1", "authorId": authorID}))
	assert.Equal(t, authorID, revisionAuthor(padID))
	padID = setup()
	ok(t, c.get("restoreRevision", map[string]string{"padID": padID, "rev": "1"}))
	assert.Equal(t, "", revisionAuthor(padID))

	failed(t, c.get("restoreRevision", map[string]string{"padID": padID}), "rev is not defined")
	failed(t, c.get("restoreRevision", map[string]string{"padID": padID, "rev": "99"}), "rev is higher than the head revision of the pad")
}

func testLegacyGetStats(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	ok(t, c.get("createPad", map[string]string{"padID": makeID()}))
	stats := ok(t, c.get("getStats", nil))
	for _, k := range []string{"totalPads", "totalSessions", "totalActivePads"} {
		assert.IsType(t, float64(0), stats[k], k)
	}
	assert.EqualValues(t, 1, stats["totalPads"])
}

func testLegacyRevisionFunctions(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	padID := makeID()
	ok(t, c.get("createPad", map[string]string{"padID": padID, "text": "hello"}))
	ok(t, c.postJSON("setText", map[string]any{"padID": padID, "text": "hello world"}))

	r := c.get("getRevisionChangeset", map[string]string{"padID": padID, "rev": "0"})
	require.Equal(t, 0, r.Code, r.Message)
	assert.IsType(t, "", r.Data, "the changeset itself")
	assert.Contains(t, r.Data, "hello")
	r = c.get("getRevisionChangeset", map[string]string{"padID": padID})
	require.Equal(t, 0, r.Code, r.Message)
	assert.Contains(t, r.Data, "world", "the head revision")

	pool := ok(t, c.get("getAttributePool", map[string]string{"padID": padID}))["pool"].(map[string]any)
	assert.Contains(t, pool, "numToAttrib")
	assert.Contains(t, pool, "nextNum")

	roID := ok(t, c.get("getReadOnlyID", map[string]string{"padID": padID}))["readOnlyID"].(string)
	assert.Equal(t, padID, ok(t, c.get("getPadID", map[string]string{"roID": roID}))["padID"])
	failed(t, c.get("getPadID", map[string]string{"roID": "r.nope"}), "padID does not exist")

	authorID := ok(t, c.get("createAuthor", map[string]string{"name": "x"}))["authorID"].(string)
	ok(t, c.postJSON("setText", map[string]any{"padID": padID, "text": "by author", "authorId": authorID}))
	assert.Contains(t, ok(t, c.get("listPadsOfAuthor", map[string]string{"authorID": authorID}))["padIDs"], padID)

	ok(t, c.get("sendClientsMessage", map[string]string{"padID": padID, "msg": "hi"}))
	failed(t, c.get("setPublicStatus", map[string]string{"padID": padID, "publicStatus": "true"}),
		"You can only get/set the publicStatus of pads that belong to a group")

	// deletionToken
	other := makeID()
	token := ok(t, c.get("createPad", map[string]string{"padID": other}))["deletionToken"].(string)
	require.NotEmpty(t, token)
	failed(t, c.get("deletePad", map[string]string{"padID": other, "deletionToken": "wrong"}), "invalid deletionToken")
	ok(t, c.get("deletePad", map[string]string{"padID": other, "deletionToken": token}))
}
