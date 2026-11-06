package io

import (
	"fmt"
	"testing"

	libio "github.com/ether/etherpad-go/lib/io"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A /:rev/export/etherpad request is a point-in-time snapshot: it must not
// carry later revisions, later text or chat posted after that revision
// (etherpad-lite #8389).
func TestExportEtherpadRevBound(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "rev-bound export stops at the revision, incl. chat", Test: testExportEtherpadRevBound},
		testutils.TestRunConfig{Name: "export without rev keeps everything", Test: testExportEtherpadUnbound},
	)
	defer testDb.StartTestDBHandler()
}

type revBoundFixture struct {
	padId     string
	rev1Text  string
	rev1Time  int64
	finalText string
}

func setupRevBoundPad(t *testing.T, ds testutils.TestDataStore, padId string) revBoundFixture {
	t.Helper()
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	text := "first"
	p, err := ds.PadManager.GetPad(padId, &text, &a.Id)
	require.NoError(t, err)
	require.NoError(t, p.SetText("second\n", &a.Id))
	rev1 := p.Head
	rev1Time, err := p.GetRevisionDate(rev1)
	require.NoError(t, err)
	rev1Text := p.Text()

	// Chat around rev 1, deliberately out of time order.
	_, err = p.AppendChatMessage(&a.Id, *rev1Time-1000, "before rev 1")
	require.NoError(t, err)
	_, err = p.AppendChatMessage(&a.Id, *rev1Time+60_000, "after rev 1")
	require.NoError(t, err)
	_, err = p.AppendChatMessage(&a.Id, *rev1Time-500, "also before rev 1")
	require.NoError(t, err)

	require.NoError(t, p.SetText("third and final\n", &a.Id))
	require.Greater(t, p.Head, rev1)
	return revBoundFixture{padId: padId, rev1Text: rev1Text, rev1Time: *rev1Time, finalText: p.Text()}
}

func newExporter(ds testutils.TestDataStore) *libio.ExportEtherpad {
	return libio.NewExportEtherpad(ds.Hooks, ds.PadManager, ds.DS, ds.Logger, testutils.GetTestAssets())
}

func testExportEtherpadRevBound(t *testing.T, ds testutils.TestDataStore) {
	f := setupRevBoundPad(t, ds, "exportRevBound")
	rev := 1
	export, err := newExporter(ds).GetPadRaw(f.padId, nil, &rev)
	require.NoError(t, err)

	padRecord, ok := export.Pad["pad:"+f.padId]
	require.True(t, ok, "pad record must use the etherpad-lite key")
	assert.Equal(t, rev, padRecord.Head)
	assert.Equal(t, f.rev1Text, padRecord.AText.Text)

	assert.Len(t, export.Revisions, rev+1)
	_, hasLater := export.Revisions[fmt.Sprintf("pad:%s:revs:%d", f.padId, rev+1)]
	assert.False(t, hasLater, "later revisions must not be exported")

	var texts []string
	for i := 0; i < len(export.Chats); i++ {
		m, ok := export.Chats[fmt.Sprintf("pad:%s:chat:%d", f.padId, i)]
		require.True(t, ok, "chat must be renumbered contiguously from 0")
		texts = append(texts, m.Text)
	}
	assert.ElementsMatch(t, []string{"before rev 1", "also before rev 1"}, texts)
	assert.Equal(t, len(texts)-1, padRecord.ChatHead)
}

func testExportEtherpadUnbound(t *testing.T, ds testutils.TestDataStore) {
	f := setupRevBoundPad(t, ds, "exportUnbound")
	export, err := newExporter(ds).GetPadRaw(f.padId, nil, nil)
	require.NoError(t, err)
	padRecord := export.Pad["pad:"+f.padId]
	assert.Equal(t, f.finalText, padRecord.AText.Text)
	assert.Len(t, export.Chats, 3)
	assert.Equal(t, 2, padRecord.ChatHead)
	assert.Len(t, export.Revisions, padRecord.Head+1)
}
