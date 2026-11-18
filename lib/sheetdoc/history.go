package sheetdoc

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/sheet"
)

// ErrNoHistory means the versions before the current one are not known:
// the workbook was imported before the import's base was kept.
var ErrNoHistory = errors.New("sheet history not available")

// ErrHistoryChanged means the workbook was replaced while its history was
// read.
var ErrHistoryChanged = errors.New("sheet was replaced meanwhile")

// ErrNoRevision means the revision is not in the history.
var ErrNoRevision = errors.New("no such revision")

// Revision is one stored change.
type Revision struct {
	Rev       int    `json:"rev"`
	Timestamp int64  `json:"timestamp"`
	Author    string `json:"author,omitempty"`
}

// History lists the stored changes up to the head. available is false when
// the base the changes start from is unknown; earlier versions can then not
// be shown.
func (m *Manager) History(padId string) (revs []Revision, head int, available bool, err error) {
	epoch, head, err := m.State(padId)
	if err != nil {
		return nil, 0, false, err
	}
	rows, err := m.store.GetSheetOpInfo(padId, 1, head)
	if err != nil {
		return nil, 0, false, err
	}
	revs = make([]Revision, 0, len(*rows))
	for _, r := range *rows {
		rv := Revision{Rev: r.Rev, Timestamp: r.Timestamp}
		if r.AuthorId != nil {
			rv.Author = *r.AuthorId
		}
		revs = append(revs, rv)
	}
	switch _, err := m.base(padId, epoch); {
	case errors.Is(err, ErrNoHistory):
	case err != nil:
		return nil, 0, false, err
	default:
		available = true
	}
	if err := m.sameEpoch(padId, epoch); err != nil {
		return nil, 0, false, err
	}
	return revs, head, available, nil
}

// WorkbookAt rebuilds the workbook as it was at rev.
func (m *Manager) WorkbookAt(padId string, rev int) (*sheet.Workbook, error) {
	epoch, _, err := m.State(padId)
	if err != nil {
		return nil, err
	}
	return m.workbookAt(padId, epoch, rev)
}

// workbookAt is WorkbookAt in epoch: ErrHistoryChanged if the workbook was
// replaced meanwhile. The result belongs to the caller.
func (m *Manager) workbookAt(padId string, epoch, rev int) (*sheet.Workbook, error) {
	v, err := m.store.GetSheetVersion(padId)
	if err != nil {
		return nil, err
	}
	if v.Epoch != epoch {
		return nil, ErrHistoryChanged
	}
	if rev < 0 || rev > v.Latest() {
		return nil, ErrNoRevision
	}
	if _, err := m.base(padId, epoch); err != nil {
		return nil, err
	}
	from, start := m.history.nearest(padId, epoch, rev)
	if start == nil {
		return nil, ErrHistoryChanged
	}
	wb, err := m.replay(padId, epoch, start, from, rev)
	if err != nil {
		return nil, err
	}
	if err := m.sameEpoch(padId, epoch); err != nil {
		return nil, err
	}
	return wb, nil
}

// replay applies the stored ops from+1..to to wb (the workbook at from),
// keeping checkpoints on the way.
func (m *Manager) replay(padId string, epoch int, wb *sheet.Workbook, from, to int) (*sheet.Workbook, error) {
	for at := from; at < to; {
		end := min(at+replayBatch, to)
		ops, err := m.storedOps(padId, at+1, end)
		if err != nil {
			return nil, err
		}
		if len(ops) != end-at {
			return nil, fmt.Errorf("sheet %s: expected ops %d..%d, got %d", padId, at+1, end, len(ops))
		}
		for _, o := range ops {
			if err := wb.Apply(o.Op); err != nil {
				return nil, fmt.Errorf("sheet %s: replaying op %d: %w", padId, o.Rev, err)
			}
			if o.Rev%checkpointEvery == 0 {
				m.history.keep(padId, epoch, o.Rev, wb)
			}
		}
		at = end
	}
	return wb, nil
}

// base makes sure the workbook the ops of epoch start from is known and
// cached as checkpoint 0; ErrNoHistory if it is not known.
func (m *Manager) base(padId string, epoch int) (*sheet.Workbook, error) {
	if wb, known, ok := m.history.base(padId, epoch); ok {
		if !known {
			return nil, ErrNoHistory
		}
		return wb, nil
	}
	rows, err := m.store.GetSheetOps(padId, 0, 0)
	if err != nil {
		return nil, err
	}
	if len(*rows) > 0 {
		var snap sheet.WorkbookSnapshot
		if err := json.Unmarshal([]byte((*rows)[0].Op), &snap); err != nil {
			return nil, err
		}
		wb := sheet.WorkbookFromSnapshot(snap)
		m.history.setBase(padId, epoch, wb)
		return wb, nil
	}
	if epoch != 0 {
		m.history.setBase(padId, epoch, nil)
		return nil, ErrNoHistory
	}
	// Never replaced, but stored before the base was kept. That is the
	// fresh workbook only if all ops replayed on it give the current
	// state: an import from before the epochs also has epoch 0.
	fresh := sheet.NewWorkbook()
	fresh.AddSheet(DefaultSheetID, "Sheet1")
	snap, head, err := m.Snapshot(padId)
	if err != nil {
		return nil, err
	}
	m.history.reset(padId, epoch)
	replayed, err := m.replay(padId, epoch, fresh.Clone(), 0, head)
	if err != nil || len(sheet.RestoreOps(replayed, sheet.WorkbookFromSnapshot(snap))) > 0 {
		// Not ours to show: forget the checkpoints made on the way.
		m.history.reset(padId, epoch)
		m.history.setBase(padId, epoch, nil)
		return nil, ErrNoHistory
	}
	m.history.setBase(padId, epoch, fresh)
	return fresh, nil
}

func (m *Manager) sameEpoch(padId string, epoch int) error {
	v, err := m.store.GetSheetVersion(padId)
	if err != nil {
		return err
	}
	if v.Epoch != epoch {
		return ErrHistoryChanged
	}
	return nil
}

// Restore brings the workbook back to how it was at rev. It stores the
// changes that lead there as ordinary ops, so the history in between stays
// and the restore itself can be undone by restoring the revision before it.
// It returns the number of ops stored.
func (m *Manager) Restore(padId string, rev int, authorId *string, tsMillis int64) (int, error) {
	// The epoch comes first: a workbook imported while the old version is
	// rebuilt must not be overwritten with a version of the one before.
	epoch, _, err := m.State(padId)
	if err != nil {
		return 0, err
	}
	target, err := m.workbookAt(padId, epoch, rev)
	if err != nil {
		return 0, err
	}
	stored := 0
	for attempt := 0; ; attempt++ {
		err = m.withDoc(padId, func(e *entry) error {
			if e.epoch != epoch {
				return ErrHistoryChanged
			}
			// Diffing against the current state makes a retry after a
			// conflict continue where the last attempt stopped.
			ops := sheet.RestoreOps(e.doc.Workbook().Clone(), target)
			for _, op := range ops {
				op.BaseRev = e.doc.Head()
				if _, _, err := m.commit(padId, e, op, authorId, tsMillis); err != nil {
					return err
				}
				stored++
			}
			if len(ops) > 0 {
				m.saveSnapshot(padId, e)
			}
			return nil
		})
		if errors.Is(err, db.ErrConflict) && attempt < maxSubmitRetries {
			continue
		}
		return stored, err
	}
}
