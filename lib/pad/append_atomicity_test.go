package pad

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	dbmodel "github.com/ether/etherpad-go/lib/models/db"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's appendRevisionAtomicity spec (#8134): a failed
// write must neither leave the pad claiming a revision that was never
// stored nor keep a change in memory that the database does not have.
//
// Here the revision row is the commit point and the pad row only a cache,
// so a failing pad-row write must not cost the revision either.

var errSimulated = errors.New("simulated backend write failure")

// failingStore wraps a DataStore and fails selected writes.
type failingStore struct {
	db.DataStore
	failRev      atomic.Int64 // revision number whose SaveRevision fails; -1 for none
	failPadWrite atomic.Bool  // CreatePad and UpdatePadHead fail
	failReads    atomic.Bool  // reading stored revisions fails
}

func (f *failingStore) GetRevisionDeltas(padID string, from, to int) ([]db.RevisionDelta, error) {
	if f.failReads.Load() {
		return nil, errSimulated
	}
	return f.DataStore.GetRevisionDeltas(padID, from, to)
}

func (f *failingStore) GetRevision(padID string, rev int) (*dbmodel.PadSingleRevision, error) {
	if f.failReads.Load() {
		return nil, errSimulated
	}
	return f.DataStore.GetRevision(padID, rev)
}

func newFailingStore(inner db.DataStore) *failingStore {
	f := &failingStore{DataStore: inner}
	f.failRev.Store(-1)
	return f
}

func (f *failingStore) SaveRevision(padID string, rev int, cs string, text dbmodel.AText, pool dbmodel.RevPool, authorID *string, ts int64) error {
	if int64(rev) == f.failRev.Load() {
		return errSimulated
	}
	return f.DataStore.SaveRevision(padID, rev, cs, text, pool, authorID, ts)
}

func (f *failingStore) CreatePad(padID string, p dbmodel.PadDB) error {
	if f.failPadWrite.Load() {
		return errSimulated
	}
	return f.DataStore.CreatePad(padID, p)
}

func (f *failingStore) UpdatePadHead(padID string, head int, pool dbmodel.RevPool) error {
	if f.failPadWrite.Load() {
		return errSimulated
	}
	return f.DataStore.UpdatePadHead(padID, head, pool)
}

// tryAppendLine is appendLine that returns the error instead of failing.
func tryAppendLine(t *testing.T, m *Manager, padID, line string) error {
	t.Helper()
	unlock := mustLock(m.LockPad(padID))
	defer unlock()
	p, err := m.GetPad(padID, nil, nil)
	require.NoError(t, err)
	return p.SpliceText(len([]rune(p.Text()))-1, 0, line+"\n", nil)
}

func padState(t *testing.T, m *Manager, padID string) (head int, text string) {
	t.Helper()
	unlock := mustLock(m.RLockPad(padID))
	defer unlock()
	p, err := m.GetPad(padID, nil, nil)
	require.NoError(t, err)
	return p.Head, p.Text()
}

func storedHead(t *testing.T, store db.DataStore, padID string) int {
	t.Helper()
	v, err := store.GetPadVersion(padID)
	require.NoError(t, err)
	return v.Rev
}

// checkReloaded drops the cached pad, loads it from the store again and
// runs the consistency check over all revisions.
func checkReloaded(t *testing.T, m *Manager, padID string) (head int, text string) {
	t.Helper()
	m.UnloadPad(padID)
	unlock := mustLock(m.RLockPad(padID))
	defer unlock()
	p, err := m.GetPad(padID, nil, nil)
	require.NoError(t, err)
	require.NoError(t, p.Check())
	return p.Head, p.Text()
}

func TestAppendRevisionAtomicity(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		// Open once: the shared database lives as long as the test that
		// opened it.
		inner := open(t)
		t.Run("failed revision write rejects and changes nothing", func(t *testing.T) {
			store := newFailingStore(inner)
			m := newInstance(t, store)
			const padID = "atomicRevFail"
			appendLine(t, m, padID, "one")
			goodHead, textBefore := padState(t, m, padID)

			store.failRev.Store(int64(goodHead + 1))
			err := tryAppendLine(t, m, padID, "two")
			require.ErrorIs(t, err, errSimulated)

			head, text := padState(t, m, padID)
			require.Equal(t, goodHead, head, "in-memory head must not move")
			require.Equal(t, textBefore, text, "memory must not keep a change that was never stored")
			require.Equal(t, goodHead, storedHead(t, store, padID), "no revision may be stored")
		})

		t.Run("next append reuses the revision number and the pad checks out", func(t *testing.T) {
			store := newFailingStore(inner)
			m := newInstance(t, store)
			const padID = "atomicReuse"
			for _, l := range []string{"line 0", "line 1", "line 2"} {
				appendLine(t, m, padID, l)
			}
			goodHead, _ := padState(t, m, padID)

			store.failRev.Store(int64(goodHead + 1))
			require.Error(t, tryAppendLine(t, m, padID, "doomed"))
			store.failRev.Store(-1)
			appendLine(t, m, padID, "after")

			head, text := padState(t, m, padID)
			require.Equal(t, goodHead+1, head, "the failed number is reused, not skipped")
			require.Equal(t, goodHead+1, storedHead(t, store, padID))
			require.NotContains(t, text, "doomed")

			reloadedHead, reloadedText := checkReloaded(t, m, padID)
			require.Equal(t, head, reloadedHead)
			require.Equal(t, text, reloadedText)
		})

		t.Run("failed pad row write keeps the stored revision", func(t *testing.T) {
			store := newFailingStore(inner)
			m := newInstance(t, store)
			const padID = "atomicRowFail"
			appendLine(t, m, padID, "one")
			goodHead, _ := padState(t, m, padID)

			// The revision row is the commit point: once it is stored the
			// change counts, even if updating the cached pad row fails.
			store.failPadWrite.Store(true)
			require.NoError(t, tryAppendLine(t, m, padID, "two"))
			store.failPadWrite.Store(false)

			head, text := padState(t, m, padID)
			require.Equal(t, goodHead+1, head)
			require.Contains(t, text, "two")
			require.Equal(t, goodHead+1, storedHead(t, store, padID))

			// A fresh instance must see it although the pad row is behind.
			other := newInstanceB(store)
			otherHead, otherText := checkReloaded(t, other, padID)
			require.Equal(t, head, otherHead)
			require.Equal(t, text, otherText)
		})

		t.Run("failures across a key revision lose nothing", func(t *testing.T) {
			store := newFailingStore(inner)
			m := newInstance(t, store)
			const padID = "atomicKeyRev"
			var want []string
			for i := 0; storedHead(t, store, padID) < 105; i++ { // past the key revision at 100
				line := "l" + strings.Repeat("x", i%7) + string(rune('a'+i%26))
				// Every fifth append first fails once on its revision.
				if i%5 == 0 {
					head, _ := padState(t, m, padID)
					store.failRev.Store(int64(head + 1))
					require.Error(t, tryAppendLine(t, m, padID, "doomed"))
					store.failRev.Store(-1)
				}
				appendLine(t, m, padID, line)
				want = append(want, line)
			}
			_, text := checkReloaded(t, newInstanceB(store), padID)
			require.NotContains(t, text, "doomed")
			require.Contains(t, text, strings.Join(want, "\n")+"\n", "all accepted lines, in order")
		})
	})
}
