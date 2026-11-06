package io

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	libio "github.com/ether/etherpad-go/lib/io"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportEtherpadRoundTrip(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "export then import reproduces the pad", Test: testEtherpadRoundTrip},
		testutils.TestRunConfig{Name: "file with a missing revision is rejected, target untouched", Test: testImportRejectsMissingRevision},
		testutils.TestRunConfig{Name: "authors without a globalAuthor record are imported", Test: testImportCreatesMissingAuthors},
	)
	defer testDb.StartTestDBHandler()
}

// buildRichPad creates a pad with >100 revisions (so a key revision exists),
// non-ASCII text, two authors and chat.
func buildRichPad(t *testing.T, ds testutils.TestDataStore, padId string) {
	t.Helper()
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	b, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	text := "Grüße 😀"
	p, err := ds.PadManager.GetPad(padId, &text, &a.Id)
	require.NoError(t, err)
	for i := 0; i < 110; i++ {
		who := &a.Id
		if i%2 == 1 {
			who = &b.Id
		}
		require.NoError(t, p.SpliceText(len([]rune(p.Text()))-1, 0, fmt.Sprintf(" ä%d", i), who))
	}
	for i := 0; i < 5; i++ {
		_, err := p.AppendChatMessage(&b.Id, int64(1000+i), fmt.Sprintf("chat %d ✓", i))
		require.NoError(t, err)
	}
	require.NoError(t, p.Check(), "fixture pad must be consistent")
}

func exportFile(t *testing.T, ds testutils.TestDataStore, padId string) []byte {
	t.Helper()
	export, err := newExporter(ds).GetPadRaw(padId, nil, nil)
	require.NoError(t, err)
	b, err := export.MarshalJSON()
	require.NoError(t, err)
	return b
}

func testEtherpadRoundTrip(t *testing.T, ds testutils.TestDataStore) {
	buildRichPad(t, ds, "roundTripSrc")
	src, err := ds.PadManager.GetPad("roundTripSrc", nil, nil)
	require.NoError(t, err)
	file := exportFile(t, ds, "roundTripSrc")

	dstCreator := createPadBy(t, ds, "roundTripDst")
	require.NoError(t, ds.Importer.SetPadRaw("roundTripDst", file, dstCreator))

	ds.PadManager.UnloadPad("roundTripDst")
	dst, err := ds.PadManager.GetPad("roundTripDst", nil, nil)
	require.NoError(t, err)
	assert.Equal(t, src.Text(), dst.Text())
	assert.Equal(t, src.Head, dst.Head)
	assert.Equal(t, src.ChatHead, dst.ChatHead)
	require.NoError(t, dst.Check(), "imported pad must pass the consistency check")

	msgs, err := dst.GetChatMessages(0, dst.ChatHead)
	require.NoError(t, err)
	require.Len(t, *msgs, 5)
	for i, m := range *msgs {
		assert.Equal(t, fmt.Sprintf("chat %d ✓", i), m.Message, "chat order must be kept")
	}
}

func testImportRejectsMissingRevision(t *testing.T, ds testutils.TestDataStore) {
	buildRichPad(t, ds, "brokenSrc")
	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(exportFile(t, ds, "brokenSrc"), &data))
	delete(data, "pad:brokenSrc:revs:50")
	broken, err := json.Marshal(data)
	require.NoError(t, err)

	creator := createPadBy(t, ds, "brokenDst")
	before, _ := reload(t, ds, "brokenDst")
	err = ds.Importer.SetPadRaw("brokenDst", broken, creator)
	require.Error(t, err)
	assert.True(t, errors.Is(err, libio.ErrInvalidEtherpadFile), err.Error())

	after, _ := reload(t, ds, "brokenDst")
	assert.Equal(t, before, after, "a rejected import must leave the target pad intact")
}

func testImportCreatesMissingAuthors(t *testing.T, ds testutils.TestDataStore) {
	buildRichPad(t, ds, "noAuthorsSrc")
	var data map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(exportFile(t, ds, "noAuthorsSrc"), &data))
	for k := range data {
		if len(k) > len("globalAuthor:") && k[:len("globalAuthor:")] == "globalAuthor:" {
			delete(data, k)
		}
	}
	trimmed, err := json.Marshal(data)
	require.NoError(t, err)

	// The source authors exist in this datastore, so rename every author id
	// (JSON strings starting with "a.") to ones the target DB doesn't know.
	file := []byte(strings.ReplaceAll(string(trimmed), `"a.`, `"a.zz`))
	creator := createPadBy(t, ds, "noAuthorsDst")
	require.NoError(t, ds.Importer.SetPadRaw("noAuthorsDst", file, creator))

	ds.PadManager.UnloadPad("noAuthorsDst")
	dst, err := ds.PadManager.GetPad("noAuthorsDst", nil, nil)
	require.NoError(t, err)
	require.NoError(t, dst.Check())
	for _, id := range dst.GetAllAuthors() {
		a, err := ds.AuthorManager.GetAuthor(id)
		require.NoError(t, err, "author %s must exist after import", id)
		assert.NotNil(t, a)
	}
}
