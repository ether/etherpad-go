package io

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The remaining cases of etherpad-lite's ExportEtherpad spec (revNum
// bounding, read-only export).
func TestExportEtherpadParity(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "chat posted at the revision's time is kept", Test: testExportChatAtRevisionTime},
		testutils.TestRunConfig{Name: "revNum = head drops chat posted after the last edit", Test: testExportHeadDropsLaterChat},
		testutils.TestRunConfig{Name: "revNum above head is the full history", Test: testExportRevAboveHead},
		testutils.TestRunConfig{Name: "export of the read-only pad uses the read-only id", Test: testExportReadOnlyId},
	)
	testDb.StartTestDBHandler()
}

func padWithRevs(t *testing.T, ds testutils.TestDataStore, padId string, n int) (string, int) {
	t.Helper()
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	p, err := ds.PadManager.GetPad(padId, nil, &a.Id)
	require.NoError(t, err)
	for i := 0; i < n; i++ {
		require.NoError(t, p.SetText(p.Text()+fmt.Sprintf("line %d\n", i), &a.Id))
	}
	return a.Id, p.Head
}

func testExportChatAtRevisionTime(t *testing.T, ds testutils.TestDataStore) {
	padId := "exportChatAtRev"
	authorId, _ := padWithRevs(t, ds, padId, 3)
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	bound := p.Head
	boundTime, err := p.GetRevisionDate(bound)
	require.NoError(t, err)
	for _, m := range []struct {
		text string
		ts   int64
	}{{"before", *boundTime - 1}, {"at", *boundTime}, {"after", *boundTime + 1}} {
		_, err = p.AppendChatMessage(&authorId, m.ts, m.text)
		require.NoError(t, err)
	}
	require.NoError(t, p.SetText(p.Text()+"more\n", &authorId))

	export, err := newExporter(ds).GetPadRaw(padId, nil, &bound)
	require.NoError(t, err)
	var keys []string
	for k := range export.Chats {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{"pad:" + padId + ":chat:0", "pad:" + padId + ":chat:1"}, keys)
	assert.Equal(t, "at", export.Chats["pad:"+padId+":chat:1"].Text)
	assert.Equal(t, 1, export.Pad["pad:"+padId].ChatHead)

	full, err := newExporter(ds).GetPadRaw(padId, nil, nil)
	require.NoError(t, err)
	assert.Contains(t, full.Chats, "pad:"+padId+":chat:2")
}

func testExportHeadDropsLaterChat(t *testing.T, ds testutils.TestDataStore) {
	padId := "exportHeadChat"
	authorId, head := padWithRevs(t, ds, padId, 2)
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	headTime, err := p.GetRevisionDate(head)
	require.NoError(t, err)
	_, err = p.AppendChatMessage(&authorId, *headTime, "at")
	require.NoError(t, err)
	_, err = p.AppendChatMessage(&authorId, *headTime+1, "later")
	require.NoError(t, err)

	export, err := newExporter(ds).GetPadRaw(padId, nil, &head)
	require.NoError(t, err)
	assert.Len(t, export.Chats, 1)
	assert.Equal(t, "at", export.Chats["pad:"+padId+":chat:0"].Text)
	assert.Equal(t, 0, export.Pad["pad:"+padId].ChatHead)
}

func testExportRevAboveHead(t *testing.T, ds testutils.TestDataStore) {
	padId := "exportAboveHead"
	_, head := padWithRevs(t, ds, padId, 3)
	above := head + 100
	export, err := newExporter(ds).GetPadRaw(padId, nil, &above)
	require.NoError(t, err)
	assert.Equal(t, head, export.Pad["pad:"+padId].Head)
	assert.Len(t, export.Revisions, head+1)
}

func testExportReadOnlyId(t *testing.T, ds testutils.TestDataStore) {
	padId := "exportReadOnly"
	authorId, _ := padWithRevs(t, ds, padId, 1)
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	_, err = p.AppendChatMessage(&authorId, 1, "hi")
	require.NoError(t, err)
	readOnlyId := ds.ReadOnlyManager.GetReadOnlyId(padId)

	export, err := newExporter(ds).GetPadRaw(padId, &readOnlyId, nil)
	require.NoError(t, err)
	b := marshal(t, export)
	assert.NotContains(t, string(b), padId, "the read/write id must not leak")
	assert.Contains(t, export.Pad, "pad:"+readOnlyId)
	assert.Contains(t, export.Chats, "pad:"+readOnlyId+":chat:0")
	for k := range export.Revisions {
		assert.True(t, strings.HasPrefix(k, "pad:"+readOnlyId+":revs:"), k)
	}
	for _, a := range export.Authors {
		assert.Equal(t, readOnlyId, a.PadIDs)
	}
}
