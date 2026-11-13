package pad

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	padManager "github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's compactPad, cleanupNoDestructiveWindow,
// cleanupHoleSafety and cleanupMissingRevisions specs.
func TestCompactParity(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "keeps the last N revisions and checks out", Test: testCompactKeepsLastN},
		testutils.TestRunConfig{Name: "collapses all history when keepRevisions is omitted", Test: testCompactAll},
		testutils.TestRunConfig{Name: "refuses to run when cleanup is disabled", Test: testCompactDisabled},
		testutils.TestRunConfig{Name: "a failed rewrite leaves the pad unchanged", Test: testCompactFailedRewrite},
		testutils.TestRunConfig{Name: "the history swap is all or nothing", Test: testReplaceRevisionsAtomic},
		testutils.TestRunConfig{Name: "a damaged pad is not compacted by keep-count", Test: testCompactDamagedPad},
	)
	testDb.StartTestDBHandler()
}

func compactFixture(t *testing.T, ds testutils.TestDataStore, padId string, n int) string {
	t.Helper()
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	text := "line 0"
	p, err := ds.PadManager.GetPad(padId, &text, &a.Id)
	require.NoError(t, err)
	for i := 1; i <= n; i++ {
		require.NoError(t, p.SetText(p.Text()+fmt.Sprintf("line %d\n", i), &a.Id))
	}
	require.NoError(t, p.AddSavedRevision(a.Id))
	return a.Id
}

// freshPad loads the pad through a new manager: nothing cached.
func freshPad(t *testing.T, ds testutils.TestDataStore, padId string) (*padManager.Manager, int, string) {
	t.Helper()
	h := hooks.NewHook()
	m := padManager.NewManager(ds.DS, &h)
	p, err := m.GetPad(padId, nil, nil)
	require.NoError(t, err)
	require.NoError(t, p.Check())
	return m, p.Head, p.Text()
}

func compact(t *testing.T, ds testutils.TestDataStore, padId, body string) (int, map[string]any) {
	t.Helper()
	initStore := ds.ToInitStore()
	pad.Init(initStore)
	req := httptest.NewRequest("POST", "/admin/api/pads/"+padId+"/compact", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req)
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return resp.StatusCode, out
}

func testCompactKeepsLastN(t *testing.T, ds testutils.TestDataStore) {
	padId := "compactLastN"
	authorId := compactFixture(t, ds, padId, 10)
	_, head, before := freshPad(t, ds, padId)
	require.Equal(t, 10, head)

	ds.Settings.Cleanup.Enabled = true
	status, out := compact(t, ds, padId, `{"keepRevisions":3}`)
	require.Equal(t, 200, status, out)
	assert.Equal(t, "keepLast", out["mode"])

	m, head, text := freshPad(t, ds, padId)
	assert.Equal(t, 3, head)
	assert.Equal(t, before, text)
	for r := 4; r <= 10; r++ {
		_, err := ds.DS.GetRevision(padId, r)
		assert.Error(t, err, "revision %d must be gone", r)
	}
	p, err := m.GetPad(padId, nil, nil)
	require.NoError(t, err)
	if assert.Len(t, p.SavedRevisions, 1) {
		assert.Equal(t, 3, p.SavedRevisions[0].RevNum, "saved revision renumbered")
	}
	// Still editable, and can be compacted again.
	require.NoError(t, p.SetText(p.Text()+"after\n", &authorId))
	_, head, _ = freshPad(t, ds, padId)
	assert.Equal(t, 4, head)
}

func testCompactAll(t *testing.T, ds testutils.TestDataStore) {
	padId := "compactAll"
	compactFixture(t, ds, padId, 3)
	_, _, before := freshPad(t, ds, padId)
	ds.Settings.Cleanup.Enabled = true
	status, out := compact(t, ds, padId, `{}`)
	require.Equal(t, 200, status, out)
	assert.Equal(t, "all", out["mode"])
	_, head, text := freshPad(t, ds, padId)
	assert.LessOrEqual(t, head, 1)
	assert.Equal(t, before, text, "byte-exact")
}

func testCompactDisabled(t *testing.T, ds testutils.TestDataStore) {
	padId := "compactDisabled"
	compactFixture(t, ds, padId, 3)
	ds.Settings.Cleanup.Enabled = false
	status, _ := compact(t, ds, padId, `{"keepRevisions":1}`)
	assert.Equal(t, 403, status)
	_, head, _ := freshPad(t, ds, padId)
	assert.Equal(t, 3, head)
}

type failingReplace struct {
	db.DataStore
}

func (f *failingReplace) ReplaceRevisions(string, db2.PadDB, []db2.PadSingleRevision) error {
	return errors.New("disk full")
}

func testCompactFailedRewrite(t *testing.T, ds testutils.TestDataStore) {
	padId := "compactFailed"
	compactFixture(t, ds, padId, 10)
	_, _, before := freshPad(t, ds, padId)
	initStore := ds.ToInitStore()
	h := ws.NewAdminMessageHandler(&failingReplace{ds.DS}, ds.Hooks, ds.PadManager, initStore.Handler, ds.Logger, nil, nil, nil)
	require.Error(t, h.DeleteRevisions(padId, 3))

	_, head, text := freshPad(t, ds, padId)
	assert.Equal(t, 10, head, "no revision lost")
	assert.Equal(t, before, text)
	// The cached pad was dropped: the next load reads the store.
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 10, p.Head)
	require.NoError(t, p.Check())
	// A later compaction still works.
	h = ws.NewAdminMessageHandler(ds.DS, ds.Hooks, ds.PadManager, initStore.Handler, ds.Logger, nil, nil, nil)
	require.NoError(t, h.DeleteRevisions(padId, 3))
	_, head, text = freshPad(t, ds, padId)
	assert.Equal(t, 3, head)
	assert.Equal(t, before, text)
}

func testReplaceRevisionsAtomic(t *testing.T, ds testutils.TestDataStore) {
	padId := "replaceAtomic"
	compactFixture(t, ds, padId, 3)
	row, err := ds.DS.GetPad(padId)
	require.NoError(t, err)
	revs, err := ds.DS.GetRevisions(padId, 0, row.Head)
	require.NoError(t, err)

	// Numbered with a gap: refused by every store before anything changes.
	bad := append([]db2.PadSingleRevision{}, (*revs)...)
	bad[1].RevNum = 7
	require.Error(t, ds.DS.ReplaceRevisions(padId, *row, bad))

	// A revision the database refuses in the middle (an author that does not
	// exist violates the SQL stores' foreign key): the transaction rolls
	// back. The memory store has no foreign keys and accepts it.
	missing := "a.doesNotExist000001"
	broken := append([]db2.PadSingleRevision{}, (*revs)...)
	broken[2].AuthorId = &missing
	if err := ds.DS.ReplaceRevisions(padId, *row, broken); err == nil {
		t.Log("store has no foreign keys; restoring")
		require.NoError(t, ds.DS.ReplaceRevisions(padId, *row, *revs))
	}
	_, head, _ := freshPad(t, ds, padId)
	assert.Equal(t, row.Head, head)
	after, err := ds.DS.GetRevisions(padId, 0, row.Head)
	require.NoError(t, err)
	require.Len(t, *after, len(*revs))
	for i := range *revs {
		assert.Equal(t, (*revs)[i].Changeset, (*after)[i].Changeset)
		assert.Equal(t, (*revs)[i].AuthorId, (*after)[i].AuthorId)
	}
}

// cleanupMissingRevisions: a pad with a hole in its history is refused by
// the keep-count compaction and left as it is; a full compaction rebuilds
// it when its latest text can be rebuilt (from the last key revision on).
func testCompactDamagedPad(t *testing.T, ds testutils.TestDataStore) {
	punch := func(t *testing.T, padId string, hole int) []db2.PadSingleRevision {
		t.Helper()
		row, err := ds.DS.GetPad(padId)
		require.NoError(t, err)
		revs, err := ds.DS.GetRevisions(padId, 0, row.Head)
		require.NoError(t, err)
		var kept []db2.PadSingleRevision
		for _, r := range *revs {
			if r.RevNum != hole {
				kept = append(kept, r)
			}
		}
		require.NoError(t, ds.DS.RemoveRevisionsOfPad(padId))
		for _, r := range kept {
			pool := db2.RevPool{}
			if r.Pool != nil {
				pool = *r.Pool
			}
			require.NoError(t, ds.DS.SaveRevision(padId, r.RevNum, r.Changeset, r.AText, pool, r.AuthorId, r.Timestamp))
		}
		ds.PadManager.UnloadPad(padId)
		return kept
	}
	initStore := ds.ToInitStore()
	h := ws.NewAdminMessageHandler(ds.DS, ds.Hooks, ds.PadManager, initStore.Handler, ds.Logger, nil, nil, nil)

	t.Run("hole before the last key revision: refused, then repaired by full compaction", func(t *testing.T) {
		padId := "compactDamagedRepairable"
		compactFixture(t, ds, padId, 103)
		_, _, latest := freshPad(t, ds, padId)
		kept := punch(t, padId, 50)

		err := h.DeleteRevisions(padId, 2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing revision(s) [50]")
		assert.Contains(t, err.Error(), "text is safe")
		for _, r := range kept {
			_, err := ds.DS.GetRevision(padId, r.RevNum)
			require.NoError(t, err, "revision %d left as it was", r.RevNum)
		}

		require.NoError(t, h.DeleteRevisions(padId, 0))
		_, head, text := freshPad(t, ds, padId)
		assert.Equal(t, 0, head)
		assert.Equal(t, latest, text)
	})

	t.Run("hole after the last stored text: nothing is changed", func(t *testing.T) {
		padId := "compactDamagedLost"
		compactFixture(t, ds, padId, 5)
		kept := punch(t, padId, 2)
		for _, keep := range []int{2, 0} {
			err := h.DeleteRevisions(padId, keep)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing revision(s) [2]")
			assert.Contains(t, err.Error(), "Nothing changed")
		}
		for _, r := range kept {
			_, err := ds.DS.GetRevision(padId, r.RevNum)
			require.NoError(t, err, "revision %d left as it was", r.RevNum)
		}
	})
}
