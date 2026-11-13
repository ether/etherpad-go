package pad

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestoreRevision(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "restores the text of an earlier revision", Test: testRestoreRevisionText},
		testutils.TestRunConfig{Name: "restores multi-byte text", Test: testRestoreRevisionMultiByte},
		testutils.TestRunConfig{Name: "restores formatting and authorship", Test: testRestoreRevisionAttributes},
		testutils.TestRunConfig{Name: "without an author the system author is used", Test: testRestoreRevisionNoAuthor},
		testutils.TestRunConfig{Name: "a revision above head is refused", Test: testRestoreRevisionAboveHead},
	)
	testDb.StartTestDBHandler()
}

func restoreRevision(t *testing.T, ds testutils.TestDataStore, padId string, rev int, authorId string) (int, string) {
	t.Helper()
	initStore := ds.ToInitStore()
	pad.Init(initStore)
	body, _ := json.Marshal(pad.RestoreRevisionRequest{Rev: rev, AuthorId: authorId})
	req := httptest.NewRequest("POST", "/admin/api/pads/"+padId+"/restoreRevision", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req)
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func restoreFixture(t *testing.T, ds testutils.TestDataStore, padId string, texts ...string) string {
	t.Helper()
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	p, err := ds.PadManager.GetPad(padId, &texts[0], &a.Id)
	require.NoError(t, err)
	for _, text := range texts[1:] {
		require.NoError(t, p.SetText(text, &a.Id))
	}
	return a.Id
}

func checkedPad(t *testing.T, ds testutils.TestDataStore, padId string) string {
	t.Helper()
	ds.PadManager.UnloadPad(padId)
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	require.NoError(t, p.Check())
	return p.Text()
}

func testRestoreRevisionText(t *testing.T, ds testutils.TestDataStore) {
	authorId := restoreFixture(t, ds, "restoreText", "first line\nsecond line", "something else\nentirely\n")
	status, body := restoreRevision(t, ds, "restoreText", 0, authorId)
	require.Equal(t, 200, status, body)
	assert.Equal(t, "first line\nsecond line\n", checkedPad(t, ds, "restoreText"))
}

func testRestoreRevisionMultiByte(t *testing.T, ds testutils.TestDataStore) {
	authorId := restoreFixture(t, ds, "restoreMultiByte", "Grüße 😀 aus Köln", "ÄÖÜ\n€€€\n")
	status, body := restoreRevision(t, ds, "restoreMultiByte", 0, authorId)
	require.Equal(t, 200, status, body)
	assert.Equal(t, "Grüße 😀 aus Köln\n", checkedPad(t, ds, "restoreMultiByte"))
}

func testRestoreRevisionAttributes(t *testing.T, ds testutils.TestDataStore) {
	padId := "restoreAttribs"
	authorId := restoreFixture(t, ds, padId, "plain bold")
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	// Make "bold" bold: keep 6 chars, then reformat 4.
	bold := p.Pool.PutAttrib(boldAttrib(), nil)
	author := p.Pool.PutAttrib(authorAttrib(authorId), nil)
	cs := "Z:b>0=6*" + itoa36(author) + "*" + itoa36(bold) + "=4$"
	_, err = p.AppendRevision(cs, &authorId)
	require.NoError(t, err)
	boldRev := p.Head
	// The attributes of the text before the final newline (the pad keeps
	// its current final newline, as in etherpad-lite).
	textAttribs := func(attribs string) string { return attribs[:strings.LastIndex(attribs, "|")] }
	before := textAttribs(p.AText.Attribs)
	require.NoError(t, p.SetText("gone\n", &authorId))

	status, body := restoreRevision(t, ds, padId, boldRev, authorId)
	require.Equal(t, 200, status, body)
	assert.Equal(t, "plain bold\n", checkedPad(t, ds, padId))
	p, err = ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(p.AText.Attribs, before), "%s vs %s: %s", before, p.AText.Attribs, "formatting and authors are restored")
}

func testRestoreRevisionNoAuthor(t *testing.T, ds testutils.TestDataStore) {
	restoreFixture(t, ds, "restoreNoAuthor", "old", "new\n")
	status, body := restoreRevision(t, ds, "restoreNoAuthor", 0, "")
	require.Equal(t, 200, status, body)
	assert.Equal(t, "old\n", checkedPad(t, ds, "restoreNoAuthor"))
}

func testRestoreRevisionAboveHead(t *testing.T, ds testutils.TestDataStore) {
	restoreFixture(t, ds, "restoreAbove", "old")
	status, _ := restoreRevision(t, ds, "restoreAbove", 5, "")
	assert.Equal(t, 400, status)
}

func boldAttrib() apool.Attribute { return apool.Attribute{Key: "bold", Value: "true"} }
func authorAttrib(id string) apool.Attribute {
	return apool.Attribute{Key: "author", Value: id}
}
func itoa36(n int) string { return utils.NumToString(n) }
