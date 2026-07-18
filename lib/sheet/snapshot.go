package sheet

import (
	"maps"
	"sort"
)

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

// MergeSnapshot is one merged range: top-left anchor plus its span.
type MergeSnapshot struct {
	Row  int `json:"row"`
	Col  int `json:"col"`
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

type SheetSnapshot struct {
	Id    string         `json:"id"`
	Name  string         `json:"name"`
	Cells []CellSnapshot `json:"cells"`
	// Sparse dimension overrides; JSON object keys are stringified indices.
	ColWidths  map[int]int     `json:"colWidths,omitempty"`
	RowHeights map[int]int     `json:"rowHeights,omitempty"`
	FrozenRows int             `json:"frozenRows,omitempty"`
	FrozenCols int             `json:"frozenCols,omitempty"`
	Merges     []MergeSnapshot `json:"merges,omitempty"`
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
		ss := SheetSnapshot{Id: s.Id, Name: s.Name, Cells: cells, FrozenRows: s.FrozenRows, FrozenCols: s.FrozenCols}
		// Clone: snapshots are consumed after the document lock is released
		// (export), so aliasing the live maps would race with Apply().
		if len(s.ColWidths) > 0 {
			ss.ColWidths = maps.Clone(s.ColWidths)
		}
		if len(s.RowHeights) > 0 {
			ss.RowHeights = maps.Clone(s.RowHeights)
		}
		for a, sp := range s.Merges {
			ss.Merges = append(ss.Merges, MergeSnapshot{a.Row, a.Col, sp.Rows, sp.Cols})
		}
		sort.Slice(ss.Merges, func(a, b int) bool {
			if ss.Merges[a].Row != ss.Merges[b].Row {
				return ss.Merges[a].Row < ss.Merges[b].Row
			}
			return ss.Merges[a].Col < ss.Merges[b].Col
		})
		out.Sheets[i] = ss
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
		maps.Copy(sh.ColWidths, ss.ColWidths)
		maps.Copy(sh.RowHeights, ss.RowHeights)
		sh.FrozenRows, sh.FrozenCols = ss.FrozenRows, ss.FrozenCols
		for _, m := range ss.Merges {
			sh.Merges[CellRef{m.Row, m.Col}] = Span{Rows: m.Rows, Cols: m.Cols}
		}
		w.Sheets[i] = sh
	}
	return w
}
