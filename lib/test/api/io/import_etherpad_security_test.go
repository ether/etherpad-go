package io

import (
	"encoding/json"
	"errors"
	"fmt"
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
		testutils.TestRunConfig{Name: "non-creator cannot import a file naming another creator", Test: testImportNonCreatorCannotChangeCreator},
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
	attribs := fmt.Sprintf("*0+%x|1+1", body)
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
			Changeset: fmt.Sprintf("Z:1>%x*0+%x$%s", body, body, text[:body]),
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

func testImportNonCreatorCannotChangeCreator(t *testing.T, ds testutils.TestDataStore) {
	padId := "importNonCreator"
	createPadBy(t, ds, padId)
	original, _ := reload(t, ds, padId)
	intruder, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)

	err = ds.Importer.SetPadRaw(padId, etherpadFile(t, "pad:src", "hijacked\n", intruder.Id, false), intruder.Id)
	require.Error(t, err)
	assert.True(t, errors.Is(err, libio.ErrImportChangesCreator))
	text, _ := reload(t, ds, padId)
	assert.Equal(t, original, text, "a rejected import must leave the pad untouched")
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
