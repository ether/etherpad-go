package sheetdoc

import (
	"encoding/json"
	"sync"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/sheet"
)

// DefaultSheetID is the id of the single sheet created for a brand-new workbook.
const DefaultSheetID = "s1"

type entry struct {
	mu  sync.Mutex
	doc *sheet.Document
}

// Manager owns the in-memory sheet documents and serializes operations per
// document (total order), persisting each op and a workbook snapshot.
type Manager struct {
	store db.DataStore
	mu    sync.Mutex
	docs  map[string]*entry
}

func NewManager(store db.DataStore) *Manager {
	return &Manager{store: store, docs: map[string]*entry{}}
}

// load returns the cached document entry for padId, loading it from the store
// or creating a fresh single-sheet workbook on first access.
func (m *Manager) load(padId string) (*entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.docs[padId]; ok {
		return e, nil
	}
	exists, err := m.store.DoesSheetExist(padId)
	if err != nil {
		return nil, err
	}
	var doc *sheet.Document
	if exists != nil && *exists {
		sd, err := m.store.GetSheet(padId)
		if err != nil {
			return nil, err
		}
		var snap sheet.WorkbookSnapshot
		if err := json.Unmarshal([]byte(sd.Snapshot), &snap); err != nil {
			return nil, err
		}
		wb := sheet.WorkbookFromSnapshot(snap)
		opsDB, err := m.store.GetSheetOps(padId, 1, sd.Head)
		if err != nil {
			return nil, err
		}
		log := make([]sheet.Op, 0, len(*opsDB))
		for _, o := range *opsDB {
			var op sheet.Op
			if err := json.Unmarshal([]byte(o.Op), &op); err != nil {
				return nil, err
			}
			log = append(log, op)
		}
		doc = sheet.NewDocumentAt(wb, log)
	} else {
		wb := sheet.NewWorkbook()
		wb.AddSheet(DefaultSheetID, "Sheet1")
		doc = sheet.NewDocument(wb)
		snapBytes, err := json.Marshal(doc.Workbook().Snapshot())
		if err != nil {
			return nil, err
		}
		if err := m.store.SaveSheet(padId, 0, string(snapBytes)); err != nil {
			return nil, err
		}
	}
	e := &entry{doc: doc}
	m.docs[padId] = e
	return e, nil
}

// Submit rebases, applies, and persists one op, returning the rebased op (for
// broadcast) and the new head revision.
func (m *Manager) Submit(padId string, op sheet.Op, authorId *string, tsMillis int64) (sheet.Op, int, error) {
	e, err := m.load(padId)
	if err != nil {
		return sheet.Op{}, 0, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	rev, err := e.doc.Submit(op)
	if err != nil {
		return sheet.Op{}, 0, err
	}
	rebased := e.doc.Log()[rev-1]

	opBytes, err := json.Marshal(rebased)
	if err != nil {
		return sheet.Op{}, 0, err
	}
	if err := m.store.SaveSheetOp(padId, rev, string(opBytes), authorId, tsMillis); err != nil {
		return sheet.Op{}, 0, err
	}
	snapBytes, err := json.Marshal(e.doc.Workbook().Snapshot())
	if err != nil {
		return sheet.Op{}, 0, err
	}
	if err := m.store.SaveSheet(padId, rev, string(snapBytes)); err != nil {
		return sheet.Op{}, 0, err
	}
	return rebased, rev, nil
}

// Snapshot returns the current workbook snapshot and head (for the initial
// client state on connect).
func (m *Manager) Snapshot(padId string) (sheet.WorkbookSnapshot, int, error) {
	e, err := m.load(padId)
	if err != nil {
		return sheet.WorkbookSnapshot{}, 0, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.doc.Workbook().Snapshot(), e.doc.Head(), nil
}

// OpsSince returns the rebased ops applied after sinceRev (for reconnect).
func (m *Manager) OpsSince(padId string, sinceRev int) ([]sheet.Op, error) {
	e, err := m.load(padId)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	log := e.doc.Log()
	if sinceRev < 0 {
		sinceRev = 0
	}
	if sinceRev > len(log) {
		sinceRev = len(log)
	}
	out := make([]sheet.Op, len(log)-sinceRev)
	copy(out, log[sinceRev:])
	return out, nil
}
