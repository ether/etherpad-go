package pad

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/ether/etherpad-go/lib/sheetdoc"
	"github.com/stretchr/testify/require"
)

// Sheets on several instances: each instance has its own sheetdoc.Manager
// (its own cached documents) on the shared database.

func newSheetPad(t *testing.T, m *Manager, padID string) {
	t.Helper()
	unlock := mustLock(m.LockPad(padID))
	defer unlock()
	_, err := m.GetTypedPad(padID, "sheet", nil)
	require.NoError(t, err)
}

func setCell(row, col int, raw string, baseRev int) sheet.Op {
	return sheet.Op{Type: sheet.OpSetCell, Sheet: sheetdoc.DefaultSheetID, Row: row, Col: col, Raw: &raw, BaseRev: baseRev}
}

func cellOf(t *testing.T, m *sheetdoc.Manager, padID string, row, col int) string {
	t.Helper()
	snap, _, err := m.Snapshot(padID)
	require.NoError(t, err)
	return sheet.WorkbookFromSnapshot(snap).SheetByID(sheetdoc.DefaultSheetID).GetCell(sheet.CellRef{Row: row, Col: col}).Raw
}

func TestSheetOpsFromTwoInstancesAreAllKept(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		storeA, storeB := open(t), open(t)
		const padID = "sheetTwoInstances"
		padsA, padsB := newInstance(t, storeA), newInstanceB(storeB)
		newSheetPad(t, padsA, padID)
		a, b := sheetdoc.NewManager(storeA), sheetdoc.NewManager(storeB)
		locks := map[*sheetdoc.Manager]*Manager{a: padsA, b: padsB}

		// Both instances have the document cached at head 0.
		_, _, err := a.Snapshot(padID)
		require.NoError(t, err)
		_, _, err = b.Snapshot(padID)
		require.NoError(t, err)

		// Their users type at the same time, each op based on what its
		// user saw last.
		const perInstance = 30
		var wg sync.WaitGroup
		for i, m := range []*sheetdoc.Manager{a, b} {
			wg.Add(1)
			go func(col int, m *sheetdoc.Manager) {
				defer wg.Done()
				base := 0
				for row := 0; row < perInstance; row++ {
					// As the websocket handler does: under the pad's lock.
					unlock, err := locks[m].LockPad(padID)
					if err != nil {
						t.Errorf("lock: %v", err)
						return
					}
					_, rev, err := m.Submit(padID, setCell(row, col, fmt.Sprint("i", col, "r", row), base), nil, 1)
					unlock()
					if err != nil {
						t.Errorf("submit: %v", err)
						return
					}
					base = rev
				}
			}(i, m)
		}
		wg.Wait()

		for _, m := range []*sheetdoc.Manager{a, b, sheetdoc.NewManager(open(t))} {
			_, head, err := m.Snapshot(padID)
			require.NoError(t, err)
			require.Equal(t, 2*perInstance, head, "every op got its own revision")
			for row := 0; row < perInstance; row++ {
				require.Equal(t, fmt.Sprint("i0r", row), cellOf(t, m, padID, row, 0))
				require.Equal(t, fmt.Sprint("i1r", row), cellOf(t, m, padID, row, 1))
			}
		}
	})
}

// A workbook replaced through one instance (xlsx import) restarts the
// numbering; the other instance must not keep working on its old copy.
func TestSheetImportOnOneInstanceReachesTheOther(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		storeA, storeB := open(t), open(t)
		const padID = "sheetImport"
		newSheetPad(t, newInstance(t, storeA), padID)
		a, b := sheetdoc.NewManager(storeA), sheetdoc.NewManager(storeB)
		for i := 0; i < 3; i++ {
			_, _, err := a.Submit(padID, setCell(i, 0, "old", i), nil, 1)
			require.NoError(t, err)
		}
		require.Equal(t, "old", cellOf(t, b, padID, 0, 0)) // B caches head 3
		epochBefore, _, err := b.State(padID)
		require.NoError(t, err)

		wb := sheet.NewWorkbook()
		wb.AddSheet(sheetdoc.DefaultSheetID, "Imported")
		require.NoError(t, a.SetWorkbook(padID, wb))
		// Three new ops bring the head back to 3: the head alone would
		// not tell B that the workbook changed.
		for i := 0; i < 3; i++ {
			_, _, err := a.Submit(padID, setCell(i, 1, "new", i), nil, 1)
			require.NoError(t, err)
		}

		epoch, head, err := b.State(padID)
		require.NoError(t, err)
		require.NotEqual(t, epochBefore, epoch, "B notices the replaced workbook")
		require.Equal(t, 3, head)
		require.Equal(t, "", cellOf(t, b, padID, 0, 0), "the old content is gone on B too")
		require.Equal(t, "new", cellOf(t, b, padID, 0, 1))
	})
}

// sheetFaultStore fails or interferes with sheet writes.
type sheetFaultStore struct {
	db.DataStore
	failOp       atomic.Bool
	failSnapshot atomic.Bool
	beforeOp     func() // runs once before the next SaveSheetOp
	once         sync.Once
}

func (f *sheetFaultStore) SaveSheetOp(padId string, rev int, op string, authorId *string, ts int64) error {
	if f.failOp.Load() {
		return errSimulated
	}
	if f.beforeOp != nil {
		f.once.Do(f.beforeOp)
	}
	return f.DataStore.SaveSheetOp(padId, rev, op, authorId, ts)
}

func (f *sheetFaultStore) SaveSheet(padId string, head int, snapshot string) error {
	if f.failSnapshot.Load() {
		return errSimulated
	}
	return f.DataStore.SaveSheet(padId, head, snapshot)
}

func TestSheetWriteFailures(t *testing.T) {
	forEachSharedStore(t, func(t *testing.T, open func(t *testing.T) db.DataStore) {
		inner := open(t)
		pads := newInstance(t, inner)

		t.Run("an op that is not stored is not kept in memory", func(t *testing.T) {
			const padID = "sheetOpFails"
			newSheetPad(t, pads, padID)
			store := &sheetFaultStore{DataStore: inner}
			m := sheetdoc.NewManager(store)
			_, _, err := m.Submit(padID, setCell(0, 0, "kept", 0), nil, 1)
			require.NoError(t, err)

			store.failOp.Store(true)
			_, _, err = m.Submit(padID, setCell(1, 0, "doomed", 1), nil, 1)
			require.ErrorIs(t, err, errSimulated)
			store.failOp.Store(false)

			_, head, err := m.Snapshot(padID)
			require.NoError(t, err)
			require.Equal(t, 1, head)
			require.Equal(t, "", cellOf(t, m, padID, 1, 0))
			_, rev, err := m.Submit(padID, setCell(2, 0, "after", 1), nil, 1)
			require.NoError(t, err)
			require.Equal(t, 2, rev, "the revision number is reused")
		})

		t.Run("a stored op survives a failed snapshot write", func(t *testing.T) {
			const padID = "sheetSnapshotFails"
			newSheetPad(t, pads, padID)
			store := &sheetFaultStore{DataStore: inner}
			m := sheetdoc.NewManager(store)
			_, _, err := m.Submit(padID, setCell(0, 0, "one", 0), nil, 1)
			require.NoError(t, err)
			store.failSnapshot.Store(true)
			_, rev, err := m.Submit(padID, setCell(1, 0, "two", 1), nil, 1)
			require.NoError(t, err, "the op is stored, so it counts")
			require.Equal(t, 2, rev)

			fresh := sheetdoc.NewManager(inner) // e.g. another instance
			_, head, err := fresh.Snapshot(padID)
			require.NoError(t, err)
			require.Equal(t, 2, head)
			require.Equal(t, "two", cellOf(t, fresh, padID, 1, 0))
			_, rev, err = fresh.Submit(padID, setCell(2, 0, "three", 2), nil, 1)
			require.NoError(t, err)
			require.Equal(t, 3, rev)
		})

		t.Run("a taken revision number is rebased, not dropped", func(t *testing.T) {
			const padID = "sheetConflict"
			newSheetPad(t, pads, padID)
			other := sheetdoc.NewManager(inner)
			store := &sheetFaultStore{DataStore: inner}
			// Another instance stores revision 1 just before this one does.
			store.beforeOp = func() {
				_, _, err := other.Submit(padID, setCell(0, 1, "other", 0), nil, 1)
				require.NoError(t, err)
			}
			m := sheetdoc.NewManager(store)
			_, rev, err := m.Submit(padID, setCell(0, 0, "mine", 0), nil, 1)
			require.NoError(t, err)
			require.Equal(t, 2, rev, "stored after the other instance's op")
			for _, mgr := range []*sheetdoc.Manager{m, other} {
				require.Equal(t, "mine", cellOf(t, mgr, padID, 0, 0))
				require.Equal(t, "other", cellOf(t, mgr, padID, 0, 1))
			}
		})
	})
}
