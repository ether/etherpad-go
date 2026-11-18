package sheetdoc

import (
	"encoding/json"
	"errors"

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
	rows, err := m.store.GetSheetOps(padId, 0, head)
	if err != nil {
		return nil, 0, false, err
	}
	revs = make([]Revision, 0, len(*rows))
	available = epoch == 0
	for _, r := range *rows {
		if r.Rev == 0 {
			available = true
			continue
		}
		rv := Revision{Rev: r.Rev, Timestamp: r.Timestamp}
		if r.AuthorId != nil {
			rv.Author = *r.AuthorId
		}
		revs = append(revs, rv)
	}
	if err := m.sameEpoch(padId, epoch); err != nil {
		return nil, 0, false, err
	}
	return revs, head, available, nil
}

// WorkbookAt rebuilds the workbook as it was at rev.
func (m *Manager) WorkbookAt(padId string, rev int) (*sheet.Workbook, error) {
	epoch, head, err := m.State(padId)
	if err != nil {
		return nil, err
	}
	if rev < 0 || rev > head {
		return nil, ErrNoRevision
	}
	doc, err := m.base(padId, epoch)
	if err != nil {
		return nil, err
	}
	if err := m.applyStored(padId, doc, 1, rev); err != nil {
		return nil, err
	}
	if err := m.sameEpoch(padId, epoch); err != nil {
		return nil, err
	}
	return doc.Workbook(), nil
}

// base is the document the stored ops of epoch start from.
func (m *Manager) base(padId string, epoch int) (*sheet.Document, error) {
	rows, err := m.store.GetSheetOps(padId, 0, 0)
	if err != nil {
		return nil, err
	}
	if len(*rows) == 0 {
		if epoch != 0 {
			return nil, ErrNoHistory
		}
		// Never replaced and created before the base was kept: the fresh
		// workbook.
		wb := sheet.NewWorkbook()
		wb.AddSheet(DefaultSheetID, "Sheet1")
		return sheet.NewDocument(wb), nil
	}
	var snap sheet.WorkbookSnapshot
	if err := json.Unmarshal([]byte((*rows)[0].Op), &snap); err != nil {
		return nil, err
	}
	return sheet.NewDocument(sheet.WorkbookFromSnapshot(snap)), nil
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
	target, err := m.WorkbookAt(padId, rev)
	if err != nil {
		return 0, err
	}
	epoch, _, err := m.State(padId)
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
