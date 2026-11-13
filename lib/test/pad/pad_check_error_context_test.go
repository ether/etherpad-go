package pad

import (
	"fmt"
	"testing"

	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's padCheckErrorContext: errors about a damaged pad
// name the pad and the revision or chat message.
func TestPadCheckErrorContext(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "a missing revision", Test: testCheckMissingRevision},
		testutils.TestRunConfig{Name: "a missing chat message", Test: testCheckMissingChat},
		testutils.TestRunConfig{Name: "an anonymized chat author is fine", Test: testCheckAnonymizedChat},
	)
	testDb.StartTestDBHandler()
}

func padWithMissingRevision(t *testing.T, ds testutils.TestDataStore, padId string, missing int) {
	t.Helper()
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	empty := ""
	p, err := ds.PadManager.GetPad(padId, &empty, &a.Id)
	require.NoError(t, err)
	for i := 0; i < 6; i++ {
		require.NoError(t, p.SetText(p.Text()+fmt.Sprintf("line %d\n", i), &a.Id))
	}
	row, err := ds.DS.GetPad(padId)
	require.NoError(t, err)
	revs, err := ds.DS.GetRevisions(padId, 0, row.Head)
	require.NoError(t, err)
	require.NoError(t, ds.DS.RemoveRevisionsOfPad(padId))
	for _, r := range *revs {
		if r.RevNum == missing {
			continue
		}
		pool := db2.RevPool{}
		if r.Pool != nil {
			pool = *r.Pool
		}
		require.NoError(t, ds.DS.SaveRevision(padId, r.RevNum, r.Changeset, r.AText, pool, r.AuthorId, r.Timestamp))
	}
	ds.PadManager.UnloadPad(padId)
}

func testCheckMissingRevision(t *testing.T, ds testutils.TestDataStore) {
	padId := "checkMissingRev"
	padWithMissingRevision(t, ds, padId, 3)
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	if err == nil {
		err = p.Check()
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("(pad %s revision 3)", padId))

	initStore := ds.ToInitStore()
	h := ws.NewAdminMessageHandler(ds.DS, ds.Hooks, ds.PadManager, initStore.Handler, ds.Logger, nil, nil, nil)
	err = h.DeleteRevisions(padId, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("pad %s is missing revision(s) [3]", padId))
}

func testCheckMissingChat(t *testing.T, ds testutils.TestDataStore) {
	padId := "checkMissingChat"
	text := "hello"
	p, err := ds.PadManager.GetPad(padId, &text, nil)
	require.NoError(t, err)
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = p.AppendChatMessage(&a.Id, 1, "hi")
		require.NoError(t, err)
	}
	// Chat message 0 disappears (the chat head says there are two).
	require.NoError(t, ds.DS.RemoveChat(padId))
	require.NoError(t, ds.DS.SaveChatMessage(padId, 1, &a.Id, 1, "hi"))
	require.NoError(t, ds.DS.SaveChatHeadOfPad(padId, 1))
	ds.PadManager.UnloadPad(padId)
	p, err = ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	err = p.Check()
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("(pad %s chat message 0)", padId))
}

func testCheckAnonymizedChat(t *testing.T, ds testutils.TestDataStore) {
	padId := "checkAnonChat"
	text := "hello"
	p, err := ds.PadManager.GetPad(padId, &text, nil)
	require.NoError(t, err)
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	_, err = p.AppendChatMessage(&a.Id, 1, "hi")
	require.NoError(t, err)
	require.NoError(t, ds.DS.ClearChatAuthorship(a.Id))
	ds.PadManager.UnloadPad(padId)
	p, err = ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	assert.NoError(t, p.Check(), "the pad can still be compacted after anonymizing")
}
