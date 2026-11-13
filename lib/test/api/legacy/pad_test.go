package legacy

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's api/pad spec, run against /api/{version}.
func TestLegacyPadAPI(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "pad lifecycle", Test: testLegacyPadLifecycle},
		testutils.TestRunConfig{Name: "copyPadWithoutHistory", Test: testLegacyCopyPadWithoutHistory},
		testutils.TestRunConfig{Name: "copied pools are independent", Test: testLegacyCopiedPoolsIndependent},
	)
	testDb.StartTestDBHandler()
}

const ulHTML = `<!doctype html><html><body><ul class="bullet"><li>one</li><li>two</li><li>0</li><li>1</li><li>2<ul class="bullet"><li>3</li><li>4</li></ul></li></ul><ol class="number"><li>item<ol class="number"><li>item1</li><li>item2</li></ol></li></ol></body></html>`

func makeID() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func longText() string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 80000)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func testLegacyPadLifecycle(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	padID, newPadID, copiedPadID, anotherPadID := makeID(), makeID(), makeID(), makeID()
	text := longText()
	P := func(kv ...string) map[string]string {
		m := map[string]string{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return m
	}

	// deletes a Pad that does not exist: answered, as an error
	failed(t, c.get("deletePad", P("padID", padID)), "padID does not exist")

	ok(t, c.get("createPad", P("padID", padID)))
	assert.EqualValues(t, 0, ok(t, c.get("getRevisionsCount", P("padID", padID)))["revisions"])
	assert.EqualValues(t, 0, ok(t, c.get("getSavedRevisionsCount", P("padID", padID)))["savedRevisions"])
	assert.Equal(t, []any{}, ok(t, c.get("listSavedRevisions", P("padID", padID)))["savedRevisions"])
	assert.Greater(t, len(ok(t, c.get("getHTML", P("padID", padID)))["html"].(string)), 1)
	assert.Contains(t, ok(t, c.get("listAllPads", nil))["padIDs"], padID)
	ok(t, c.get("deletePad", P("padID", padID)))
	assert.NotContains(t, ok(t, c.get("listAllPads", nil))["padIDs"], padID)
	failed(t, c.get("getHTML", P("padID", padID)), "padID does not exist")

	ok(t, c.get("createPad", P("padID", padID, "text", "testText")))
	assert.Equal(t, "testText\n", ok(t, c.get("getText", P("padID", padID)))["text"])
	ok(t, c.postJSON("setText", map[string]any{"padID": padID, "text": "testTextTwo"}))
	assert.Equal(t, "testTextTwo\n", ok(t, c.get("getText", P("padID", padID)))["text"])
	assert.EqualValues(t, 1, ok(t, c.get("getRevisionsCount", P("padID", padID)))["revisions"])
	ok(t, c.get("saveRevision", P("padID", padID)))
	assert.EqualValues(t, 1, ok(t, c.get("getSavedRevisionsCount", P("padID", padID)))["savedRevisions"])
	assert.Equal(t, []any{float64(1)}, ok(t, c.get("listSavedRevisions", P("padID", padID)))["savedRevisions"])
	assert.EqualValues(t, 0, ok(t, c.get("padUsersCount", P("padID", padID)))["padUsersCount"])
	assert.NotEmpty(t, ok(t, c.get("getReadOnlyID", P("padID", padID)))["readOnlyID"])
	assert.Len(t, ok(t, c.get("listAuthorsOfPad", P("padID", padID)))["authorIDs"], 0)
	lastEdited := ok(t, c.get("getLastEdited", P("padID", padID)))["lastEdited"].(float64)
	assert.NotZero(t, lastEdited)
	ok(t, c.postJSON("setText", map[string]any{"padID": padID, "text": "testTextThree"}))
	assert.GreaterOrEqual(t, ok(t, c.get("getLastEdited", P("padID", padID)))["lastEdited"].(float64), lastEdited)
	assert.Len(t, ok(t, c.get("padUsers", P("padID", padID)))["padUsers"], 0)
	ok(t, c.get("deletePad", P("padID", padID)))
	ok(t, c.get("createPad", P("padID", padID)))

	// setText as a form field, padID in the query
	ok(t, c.postForm("setText", P("padID", padID), P("text", text)))
	assert.Equal(t, text+"\n", ok(t, c.get("getText", P("padID", padID)))["text"])
	ok(t, c.postForm("setText", P("padID", padID), P("text", text+"\n")))
	assert.Equal(t, text+"\n", ok(t, c.get("getText", P("padID", padID)))["text"], "no excess newline")
	assert.NotZero(t, ok(t, c.get("getLastEdited", P("padID", padID)))["lastEdited"])

	ok(t, c.get("movePad", P("sourceID", padID, "destinationID", newPadID, "force", "true")))
	assert.Equal(t, text+"\n", ok(t, c.get("getText", P("padID", newPadID)))["text"])
	ok(t, c.get("movePad", P("sourceID", newPadID, "destinationID", padID, "force", "false")))
	assert.Equal(t, text+"\n", ok(t, c.get("getText", P("padID", padID)))["text"])
	assert.NotZero(t, ok(t, c.get("getLastEdited", P("padID", padID)))["lastEdited"])

	// appendText in 1.2.13
	c.version = "1.2.13"
	ok(t, c.get("appendText", P("padID", padID, "text", "hello")))
	c.version = "1.3.1"
	assert.Equal(t, text+"hello\n", ok(t, c.get("getText", P("padID", padID)))["text"])

	// getText of an old revision
	rev := int(ok(t, c.get("getRevisionsCount", P("padID", padID)))["revisions"].(float64))
	require.Greater(t, rev, 0)
	assert.Equal(t, text+"\n", ok(t, c.get("getText", P("padID", padID, "rev", strconv.Itoa(rev-1))))["text"])
	failed(t, c.get("getText", P("padID", padID, "rev", strconv.Itoa(rev+1))), "rev is higher than the head revision of the pad")
	failed(t, c.get("getText", P("padID", padID, "rev", "abc")), "rev is not a number")

	// setHTML with ugly HTML
	ok(t, c.postJSON("setHTML", map[string]any{"padID": padID, "html": "<div><b>Hello HTML</title></head></div>"}))

	// nested lists of different types round-trip
	ok(t, c.postJSON("setHTML", map[string]any{"padID": padID, "html": ulHTML}))
	// etherpad-lite also writes start="1" on the outer and start="2" on the
	// nested list (a counter its importer keeps across lists); browsers then
	// number the nested list from 2. etherpad-go leaves both out.
	html := ok(t, c.get("getHTML", P("padID", padID)))["html"].(string)
	assert.Equal(t, `<!doctype html><html><body><ul class="bullet"><li>one</li><li>two</li><li>0</li><li>1</li><li>2<ul class="bullet"><li>3</li><li>4</ul></li></ul><ol class="number"><li>item<ol class="number"><li>item1</li><li>item2</ol></li></ol></body></html>`,
		strings.ToLower(strings.Replace(html, "<br></body>", "</body>", 1)))

	// white space between list items
	ok(t, c.get("setHTML", P("padID", padID, "html", `<!doctype html><html><body><ul class="bullet"> <li>one</li></ul></body></html>`)))
	html = ok(t, c.get("getHTML", P("padID", padID)))["html"].(string)
	assert.Equal(t, `<!doctype html><html><body><ul class="bullet"><li>one</ul></body></html>`,
		strings.ToLower(strings.Replace(html, "<br></body>", "</body>", 1)))

	// bad characters in a new pad id
	for _, bad := range []string{"/", "#", "?", "&"} {
		failed(t, c.get("createPad", P("padID", bad)), "")
	}

	ok(t, c.get("copyPad", P("sourceID", padID, "destinationID", copiedPadID, "force", "true")))

	// setting identical text adds no revision
	ok(t, c.postForm("setText", P("padID", padID), P("text", "identical text\n")))
	assert.Equal(t, "identical text\n", ok(t, c.get("getText", P("padID", padID)))["text"])
	revs := ok(t, c.get("getRevisionsCount", P("padID", padID)))["revisions"]
	ok(t, c.postForm("setText", P("padID", padID), P("text", "identical text\n")))
	assert.Equal(t, revs, ok(t, c.get("getRevisionsCount", P("padID", padID)))["revisions"])

	// a pad created with empty text is empty
	ok(t, c.get("createPad", P("padID", anotherPadID, "text", "")))
	assert.Equal(t, "\n", ok(t, c.get("getText", P("padID", anotherPadID)))["text"])
	ok(t, c.get("deletePad", P("padID", anotherPadID)))

	failed(t, c.get("createPad", P("padID", padID)), "padID does already exist")
	failed(t, c.get("createPad", P("padID", "g.abc$x")), "createPad can't create group pads")
}

func createPadWithHTML(t *testing.T, c *client, padID, html string) {
	t.Helper()
	ok(t, c.get("createPad", map[string]string{"padID": padID}))
	ok(t, c.postJSON("setHTML", map[string]any{"padID": padID, "html": html}))
}

func testLegacyCopyPadWithoutHistory(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	src := makeID()
	createPadWithHTML(t, c, src, ulHTML)
	srcHTML := ok(t, c.get("getHTML", map[string]string{"padID": src}))["html"]

	dst := makeID()
	ok(t, c.get("copyPadWithoutHistory", map[string]string{"sourceID": src, "destinationID": dst, "force": "false"}))
	assert.Equal(t, srcHTML, ok(t, c.get("getHTML", map[string]string{"padID": dst}))["html"], "same content as the source")

	failed(t, c.get("copyPadWithoutHistory", map[string]string{
		"sourceID": src, "destinationID": "notExistentGroup$" + makeID(), "force": "true",
	}), "")

	existing := makeID()
	createPadWithHTML(t, c, existing, ulHTML)
	failed(t, c.get("copyPadWithoutHistory", map[string]string{"sourceID": src, "destinationID": existing, "force": "false"}),
		"destinationID does already exist")
	ok(t, c.get("copyPadWithoutHistory", map[string]string{"sourceID": src, "destinationID": existing, "force": "true"}))
}

// Regression test of etherpad-lite #5296.
func testLegacyCopiedPoolsIndependent(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	src, dst := makeID(), makeID()
	createPadWithHTML(t, c, src, ulHTML)
	getHTML := func(id string) string {
		return ok(t, c.get("getHTML", map[string]string{"padID": id}))["html"].(string)
	}
	setBody := func(id, body string) {
		ok(t, c.postJSON("setHTML", map[string]any{"padID": id, "html": "<!DOCTYPE HTML><html><body>" + body + "</body></html>"}))
	}
	orig := getHTML(src)
	assert.NotContains(t, orig, "<strong>")
	assert.NotContains(t, orig, "<em>")
	ok(t, c.get("copyPadWithoutHistory", map[string]string{"sourceID": src, "destinationID": dst, "force": "false"}))
	setBody(src, "<strong>bold</strong>")
	setBody(dst, "<em>italic</em>")
	setBody(src, "<strong>bold</strong> foo")

	check := func() {
		s, d := getHTML(src), getHTML(dst)
		assert.Contains(t, s, "<strong>bold</strong>")
		assert.NotContains(t, s, "<em>")
		assert.Contains(t, d, "<em>italic</em>")
		assert.NotContains(t, d, "<strong>")
	}
	check()
	ds.PadManager.UnloadPad(src)
	ds.PadManager.UnloadPad(dst)
	check()
}
