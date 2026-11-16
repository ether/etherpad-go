package io

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	libio "github.com/ether/etherpad-go/lib/io"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite #8420 (src/tests/backend/specs/ImportEtherpad.ts):
// a .etherpad import brings text, revisions and chat, but the pad's security
// metadata stays with the target pad.
func TestImportEtherpadSecurity(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "publicStatus is taken from the target pad", Test: testImportKeepsTargetPublicStatus},
		testutils.TestRunConfig{Name: "read-only link survives an import", Test: testImportKeepsReadOnlyId},
		testutils.TestRunConfig{Name: "a non-creator's import keeps the pad's creator", Test: testImportNonCreatorKeepsCreator},
		testutils.TestRunConfig{Name: "a browser pad's export imports into a pad created through the API", Test: testImportIntoAPICreatedPad},
		testutils.TestRunConfig{Name: "creator may import a file naming another creator", Test: testImportCreatorMayChangeCreator},
		testutils.TestRunConfig{Name: "imports etherpad-lite and etherpad-go pad keys", Test: testImportAcceptsBothPadKeyFormats},
	)
	defer testDb.StartTestDBHandler()
}

// etherpadFile builds a minimal one-revision .etherpad export. padKey is the
// key of the pad record ("pad:src" as written by etherpad-lite).
func etherpadFile(t *testing.T, padKey, text, rev0Author string, publicStatus bool) []byte {
	t.Helper()
	ts := int64(1)
	// Like a real export: rev 0 inserts the text before the initial "\n",
	// attributed to its author, who is in the pool.
	body := len(text) - 1
	n36 := strconv.FormatInt(int64(body), 36) // changeset numbers are base 36
	attribs := fmt.Sprintf("*0+%s|1+1", n36)
	data := map[string]any{
		padKey: map[string]any{
			"atext":          map[string]string{"text": text, "attribs": attribs},
			"pool":           map[string]any{"numToAttrib": map[string][]string{"0": {"author", rev0Author}}, "nextNum": 1},
			"head":           0,
			"chatHead":       -1,
			"publicStatus":   publicStatus,
			"savedRevisions": []any{},
		},
		"pad:src:revs:0": libio.Revision{
			Changeset: fmt.Sprintf("Z:1>%s*0+%s$%s", n36, n36, text[:body]),
			Meta: libio.RevisionMeta{
				Author:    &rev0Author,
				Timestamp: &ts,
				AText:     &libio.AText{Text: text, Attribs: attribs},
			},
		},
	}
	// Real exports carry the authors of their revisions.
	data["globalAuthor:"+rev0Author] = map[string]any{"colorId": "#ff0000", "name": "file author", "timestamp": 1}
	b, err := json.Marshal(data)
	require.NoError(t, err)
	return b
}

func createPadBy(t *testing.T, ds testutils.TestDataStore, padId string) string {
	t.Helper()
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	text := "original\n"
	_, err = ds.PadManager.GetPad(padId, &text, &a.Id)
	require.NoError(t, err)
	return a.Id
}

func reload(t *testing.T, ds testutils.TestDataStore, padId string) (string, bool) {
	t.Helper()
	ds.PadManager.UnloadPad(padId)
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	return p.Text(), p.PublicStatus
}

func testImportKeepsTargetPublicStatus(t *testing.T, ds testutils.TestDataStore) {
	padId := "importPublicStatus"
	creator := createPadBy(t, ds, padId)
	require.NoError(t, ds.Importer.SetPadRaw(padId, etherpadFile(t, "pad:src", "imported\n", creator, true), creator))
	text, public := reload(t, ds, padId)
	assert.Equal(t, "imported\n", text)
	assert.False(t, public, "a private pad must not become public through an import")
}

func testImportKeepsReadOnlyId(t *testing.T, ds testutils.TestDataStore) {
	padId := "importReadOnlyId"
	creator := createPadBy(t, ds, padId)
	before := ds.ReadOnlyManager.GetReadOnlyId(padId)
	require.NotEmpty(t, before)
	require.NoError(t, ds.Importer.SetPadRaw(padId, etherpadFile(t, "pad:src", "imported\n", creator, false), creator))
	ds.PadManager.UnloadPad(padId)
	assert.Equal(t, before, ds.ReadOnlyManager.GetReadOnlyId(padId), "existing read-only links must keep working")
}

func rev0Author(t *testing.T, ds testutils.TestDataStore, padId string) string {
	t.Helper()
	rev, err := ds.DS.GetRevision(padId, 0)
	require.NoError(t, err)
	if rev.AuthorId == nil {
		return ""
	}
	return *rev.AuthorId
}

// etherpad-lite #8470: someone other than the creator imports a file naming
// another creator. The import goes through; the pad keeps its creator (the
// rev 0 author, who may delete the pad), the importer does not become it.
func testImportNonCreatorKeepsCreator(t *testing.T, ds testutils.TestDataStore) {
	padId := "importNonCreator"
	creator := createPadBy(t, ds, padId)
	intruder, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)

	require.NoError(t, ds.Importer.SetPadRaw(padId, etherpadFile(t, "pad:src", "imported\n", intruder.Id, false), intruder.Id))
	text, _ := reload(t, ds, padId)
	assert.Equal(t, "imported\n", text)
	assert.Equal(t, creator, rev0Author(t, ds, padId), "the creator stays the creator")

	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, p.GetAllAuthors(), creator, "the pool names the creator, as every revision author")
}

// A pad created through the HTTP API has the system author as its creator;
// the export of a pad a browser made names its browser author. Importing
// it used to fail with a permission error.
func testImportIntoAPICreatedPad(t *testing.T, ds testutils.TestDataStore) {
	padId := "importIntoAPIPad"
	text := "from the API\n"
	_, err := ds.PadManager.GetPad(padId, &text, nil)
	require.NoError(t, err)
	apiCreator := rev0Author(t, ds, padId)
	require.NotEmpty(t, apiCreator)

	browser, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	require.NoError(t, ds.Importer.SetPadRaw(padId, etherpadFile(t, "pad:src", "from the browser\n", browser.Id, false), browser.Id))
	got, _ := reload(t, ds, padId)
	assert.Equal(t, "from the browser\n", got)
	assert.Equal(t, apiCreator, rev0Author(t, ds, padId), "the importer does not become the creator")
}

func testImportCreatorMayChangeCreator(t *testing.T, ds testutils.TestDataStore) {
	padId := "importCreator"
	creator := createPadBy(t, ds, padId)
	require.NoError(t, ds.Importer.SetPadRaw(padId, etherpadFile(t, "pad:src", "restored\n", "a.someoneElse", false), creator))
	text, _ := reload(t, ds, padId)
	assert.Equal(t, "restored\n", text)
}

func testImportAcceptsBothPadKeyFormats(t *testing.T, ds testutils.TestDataStore) {
	for i, key := range []string{"pad:src", "pad:src:"} {
		padId := fmt.Sprintf("importKeyFormat%d", i)
		creator := createPadBy(t, ds, padId)
		require.NoError(t, ds.Importer.SetPadRaw(padId, etherpadFile(t, key, "keyed\n", creator, false), creator), key)
		text, _ := reload(t, ds, padId)
		assert.Equal(t, "keyed\n", text, key)
	}
}
