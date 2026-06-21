package sheet

import "sort"

// CellSnapshot is the serializable form of one populated cell (map keys can't
// be JSON-encoded, so cells become a flat slice).
type CellSnapshot struct {
	Row       int    `json:"row"`
	Col       int    `json:"col"`
	Raw       string `json:"raw"`
	Value     string `json:"value,omitempty"`
	ValueType string `json:"valueType,omitempty"`
	StyleId   int    `json:"styleId,omitempty"`
}

type SheetSnapshot struct {
	Id    string         `json:"id"`
	Name  string         `json:"name"`
	Cells []CellSnapshot `json:"cells"`
}

// WorkbookSnapshot is the JSON-serializable form of a Workbook for persistence.
type WorkbookSnapshot struct {
	Sheets []SheetSnapshot `json:"sheets"`
	Styles *StylePool      `json:"styles"`
}

// Snapshot converts the workbook to its serializable form. Cells are emitted in
// (row, col) order for deterministic output.
func (w *Workbook) Snapshot() WorkbookSnapshot {
	out := WorkbookSnapshot{Sheets: make([]SheetSnapshot, len(w.Sheets)), Styles: w.Styles}
	for i, s := range w.Sheets {
		cells := make([]CellSnapshot, 0, len(s.Cells))
		for ref, c := range s.Cells {
			cells = append(cells, CellSnapshot{ref.Row, ref.Col, c.Raw, c.Value, c.ValueType, c.StyleId})
		}
		sort.Slice(cells, func(a, b int) bool {
			if cells[a].Row != cells[b].Row {
				return cells[a].Row < cells[b].Row
			}
			return cells[a].Col < cells[b].Col
		})
		out.Sheets[i] = SheetSnapshot{Id: s.Id, Name: s.Name, Cells: cells}
	}
	return out
}

// WorkbookFromSnapshot rebuilds a Workbook (and its StylePool dedup index) from
// a deserialized snapshot.
func WorkbookFromSnapshot(snap WorkbookSnapshot) *Workbook {
	w := &Workbook{Sheets: make([]*Sheet, len(snap.Sheets))}
	if snap.Styles == nil {
		w.Styles = NewStylePool()
	} else {
		w.Styles = snap.Styles
		if w.Styles.IdToStyle == nil {
			w.Styles.IdToStyle = map[int]Style{}
		}
		if w.Styles.NextId == 0 {
			w.Styles.NextId = 1
		}
		w.Styles.rebuildIndex()
	}
	for i, ss := range snap.Sheets {
		sh := NewSheet(ss.Id, ss.Name)
		for _, c := range ss.Cells {
			sh.Cells[CellRef{c.Row, c.Col}] = Cell{Raw: c.Raw, Value: c.Value, ValueType: c.ValueType, StyleId: c.StyleId}
		}
		w.Sheets[i] = sh
	}
	return w
}
