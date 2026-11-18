package sheetdoc

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/sheet"
)

// DefaultSheetID is the id of the single sheet created for a brand-new workbook.
const DefaultSheetID = "s1"

type entry struct {
	mu  sync.Mutex
	doc *sheet.Document
	// epoch is the stored epoch doc belongs to.
	epoch int
}

// Manager owns the in-memory sheet documents and serializes operations per
// document (total order), persisting each op and a workbook snapshot.
//
// Several Etherpad instances may share the database. The stored ops are the
// authoritative history; a cached document is only a copy and is brought up
// to the stored state before every use. An op counts once it is stored: if
// its revision number is taken, another instance was first, and the op is
// rebased onto the newer state and stored again.
type Manager struct {
	store db.DataStore
	mu    sync.Mutex
	docs  map[string]*entry
	// history keeps workbooks of earlier revisions (history.go).
	history checkpoints
}

func NewManager(store db.DataStore) *Manager {
	return &Manager{store: store, docs: map[string]*entry{}}
}

// StoredOp is a stored op with its revision and author.
type StoredOp struct {
	Rev    int
	Op     sheet.Op
	Author string
}

// maxSubmitRetries bounds how often Submit rebases onto ops other instances
// stored first.
const maxSubmitRetries = 5

// withDoc runs fn on padId's document at the stored state, holding the
// document's lock. The document is loaded, or created as a fresh single-sheet
// workbook, on first access. Only this document waits for the database.
func (m *Manager) withDoc(padId string, fn func(e *entry) error) error {
	m.mu.Lock()
	e, ok := m.docs[padId]
	if !ok {
		e = &entry{}
		m.docs[padId] = e
	}
	m.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	if err := m.sync(padId, e); err != nil {
		e.doc = nil
		return err
	}
	return fn(e)
}

// sync brings e up to the stored state. The caller holds e.mu.
func (m *Manager) sync(padId string, e *entry) error {
	v, err := m.store.GetSheetVersion(padId)
	if err != nil {
		return err
	}
	if e.doc != nil && v.Exists && v.Epoch == e.epoch {
		head := e.doc.Head()
		switch latest := v.Latest(); {
		case latest == head:
			return nil
		case latest > head:
			// Another instance appended; apply its ops.
			if err := m.applyStored(padId, e.doc, head+1, latest); err == nil {
				return nil
			}
		}
		// Anything else (older state, failed catch-up): load afresh.
	}
	e.doc = nil

	if !v.Exists {
		wb := sheet.NewWorkbook()
		wb.AddSheet(DefaultSheetID, "Sheet1")
		snapBytes, err := json.Marshal(sheet.NewDocument(wb).Workbook().Snapshot())
		if err != nil {
			return err
		}
		if err := m.store.SaveSheet(padId, 0, string(snapBytes)); err != nil {
			return err
		}
		// The base the history starts from. Another instance creating the
		// sheet at the same moment stores the same one.
		_ = m.store.SaveSheetOp(padId, 0, string(snapBytes), nil, time.Now().UnixMilli())
		// Another instance may have created it at the same moment; take
		// what is stored now.
		if v, err = m.store.GetSheetVersion(padId); err != nil {
			return err
		}
	}
	doc, err := m.loadStored(padId, v)
	if err != nil {
		return err
	}
	e.doc, e.epoch = doc, v.Epoch
	return nil
}

// loadStored builds the document from the stored snapshot and ops.
func (m *Manager) loadStored(padId string, v *db.SheetVersion) (*sheet.Document, error) {
	sd, err := m.store.GetSheet(padId)
	if err != nil {
		return nil, err
	}
	var snap sheet.WorkbookSnapshot
	if err := json.Unmarshal([]byte(sd.Snapshot), &snap); err != nil {
		return nil, err
	}
	log, err := m.storedOps(padId, 1, sd.Head)
	if err != nil {
		return nil, err
	}
	if len(log) != sd.Head {
		return nil, fmt.Errorf("sheet %s: snapshot at %d but %d ops stored", padId, sd.Head, len(log))
	}
	ops := make([]sheet.Op, len(log))
	for i, o := range log {
		ops[i] = o.Op
	}
	doc := sheet.NewDocumentAt(sheet.WorkbookFromSnapshot(snap), ops)
	// Ops stored after the snapshot (the instance that stored them had not
	// written the snapshot yet).
	if latest := max(v.Latest(), doc.Head()); latest > doc.Head() {
		if err := m.applyStored(padId, doc, doc.Head()+1, latest); err != nil {
			return nil, err
		}
	}
	return doc, nil
}

func (m *Manager) storedOps(padId string, from, to int) ([]StoredOp, error) {
	from = max(from, 1) // rev 0 is the base snapshot, not an op
	if to < from {
		return nil, nil
	}
	opsDB, err := m.store.GetSheetOps(padId, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]StoredOp, 0, len(*opsDB))
	for _, o := range *opsDB {
		var op sheet.Op
		if err := json.Unmarshal([]byte(o.Op), &op); err != nil {
			return nil, err
		}
		so := StoredOp{Rev: o.Rev, Op: op}
		if o.AuthorId != nil {
			so.Author = *o.AuthorId
		}
		out = append(out, so)
	}
	return out, nil
}

// applyStored applies the stored ops from..to to doc. Stored ops are already
// rebased, so each applies at the head it was stored for.
func (m *Manager) applyStored(padId string, doc *sheet.Document, from, to int) error {
	ops, err := m.storedOps(padId, from, to)
	if err != nil {
		return err
	}
	if len(ops) != to-from+1 {
		return fmt.Errorf("sheet %s: expected ops %d..%d, got %d", padId, from, to, len(ops))
	}
	for _, o := range ops {
		if o.Rev != doc.Head()+1 {
			return fmt.Errorf("sheet %s: op %d missing", padId, doc.Head()+1)
		}
		op := o.Op
		op.BaseRev = doc.Head()
		if _, err := doc.Submit(op); err != nil {
			return fmt.Errorf("sheet %s: applying stored op %d: %w", padId, o.Rev, err)
		}
	}
	return nil
}

// Submit rebases, applies, and persists one op, returning the rebased op (for
// broadcast) and the new head revision.
func (m *Manager) Submit(padId string, op sheet.Op, authorId *string, tsMillis int64) (sheet.Op, int, error) {
	rebased, rev, _, err := m.SubmitOnce(padId, op, authorId, tsMillis)
	return rebased, rev, err
}

// SubmitOnce is Submit for ops a client may send twice: an op whose cid is
// already in the log after its base revision (the client resent it after a
// reconnect) is not applied again. duplicate reports that case; rebased and
// rev are then the stored op and its revision.
func (m *Manager) SubmitOnce(padId string, op sheet.Op, authorId *string, tsMillis int64) (rebased sheet.Op, rev int, duplicate bool, err error) {
	for attempt := 0; ; attempt++ {
		rebased, rev, duplicate, err = m.submitOnce(padId, op, authorId, tsMillis)
		if errors.Is(err, db.ErrConflict) && attempt < maxSubmitRetries {
			// Another instance stored this revision first; rebase onto it.
			continue
		}
		return rebased, rev, duplicate, err
	}
}

func (m *Manager) submitOnce(padId string, op sheet.Op, authorId *string, tsMillis int64) (rebased sheet.Op, rev int, duplicate bool, err error) {
	err = m.withDoc(padId, func(e *entry) error {
		if op.Cid != "" {
			log := e.doc.Log()
			for i := min(max(op.BaseRev, 0), len(log)); i < len(log); i++ {
				if log[i].Cid == op.Cid {
					rebased, rev, duplicate = log[i], i+1, true
					return nil
				}
			}
		}
		applied, newRev, err := m.commit(padId, e, op, authorId, tsMillis)
		if err != nil {
			return err
		}
		m.saveSnapshot(padId, e)
		rebased, rev = applied, newRev
		return nil
	})
	return rebased, rev, duplicate, err
}

// commit applies op to e's document and stores it. The caller holds e.mu.
func (m *Manager) commit(padId string, e *entry, op sheet.Op, authorId *string, tsMillis int64) (sheet.Op, int, error) {
	newRev, err := e.doc.Submit(op)
	if err != nil {
		// A failed apply may have changed the workbook partly.
		e.doc = nil
		return sheet.Op{}, 0, err
	}
	applied := e.doc.Log()[newRev-1]
	opBytes, err := json.Marshal(applied)
	if err == nil {
		// Storing the op is the commit point.
		err = m.store.SaveSheetOp(padId, newRev, string(opBytes), authorId, tsMillis)
	}
	if err != nil {
		// The cached document holds an op that is not stored: forget
		// it, the next access loads the stored state.
		e.doc = nil
		return sheet.Op{}, 0, err
	}
	return applied, newRev, nil
}

// saveSnapshot stores e's workbook. The stored snapshot is a cache of the
// ops; if writing it fails, the next load applies the ops after it.
func (m *Manager) saveSnapshot(padId string, e *entry) {
	if snapBytes, err := json.Marshal(e.doc.Workbook().Snapshot()); err == nil {
		_ = m.store.SaveSheet(padId, e.doc.Head(), string(snapBytes))
	}
}

// SetWorkbook replaces the document's workbook (e.g. from an xlsx import),
// resetting it to revision 0 with an empty op-log and starting a new epoch,
// by which other instances notice that their copy is outdated.
func (m *Manager) SetWorkbook(padId string, wb *sheet.Workbook) error {
	snapBytes, err := json.Marshal(sheet.NewDocument(wb).Workbook().Snapshot())
	if err != nil {
		return err
	}
	m.mu.Lock()
	e, ok := m.docs[padId]
	if !ok {
		e = &entry{}
		m.docs[padId] = e
	}
	m.mu.Unlock()
	e.mu.Lock()
	defer e.mu.Unlock()
	e.doc = nil
	return m.store.ResetSheet(padId, string(snapBytes))
}

// Snapshot returns the current workbook snapshot and head (for the initial
// client state on connect).
func (m *Manager) Snapshot(padId string) (snap sheet.WorkbookSnapshot, head int, err error) {
	err = m.withDoc(padId, func(e *entry) error {
		snap, head = e.doc.Workbook().Snapshot(), e.doc.Head()
		return nil
	})
	return snap, head, err
}

// State returns the stored epoch and head of the document.
func (m *Manager) State(padId string) (epoch, head int, err error) {
	err = m.withDoc(padId, func(e *entry) error {
		epoch, head = e.epoch, e.doc.Head()
		return nil
	})
	return epoch, head, err
}

// StoredOps returns the stored ops from..to with their authors.
func (m *Manager) StoredOps(padId string, from, to int) ([]StoredOp, error) {
	ops, err := m.storedOps(padId, from, to)
	if err != nil {
		return nil, err
	}
	if len(ops) != max(0, to-from+1) {
		return nil, fmt.Errorf("sheet %s: expected ops %d..%d, got %d", padId, from, to, len(ops))
	}
	return ops, nil
}

// OpsSince returns the rebased ops applied after sinceRev (for reconnect).
func (m *Manager) OpsSince(padId string, sinceRev int) (out []sheet.Op, err error) {
	err = m.withDoc(padId, func(e *entry) error {
		log := e.doc.Log()
		sinceRev = min(max(sinceRev, 0), len(log))
		out = make([]sheet.Op, len(log)-sinceRev)
		copy(out, log[sinceRev:])
		return nil
	})
	return out, err
}
