package sheet

import "maps"

// Span is the extent of a merged-cell range, keyed by its top-left anchor.
// Rows/Cols >= 1; a 1x1 span is never stored.
type Span struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

// Sheet is a single tab: sparse cells plus structural metadata.
type Sheet struct {
	Id    string           `json:"id"`
	Name  string           `json:"name"`
	Cells map[CellRef]Cell `json:"-"` // sparse; JSON handled by the snapshot/persistence layer
	// Sparse per-index pixel overrides; unset = view default.
	ColWidths  map[int]int `json:"-"`
	RowHeights map[int]int `json:"-"`
	// 0 or 1 each: freeze the first row / first col (position: sticky in the view).
	FrozenRows int `json:"-"`
	FrozenCols int `json:"-"`
	// Merged-cell ranges keyed by top-left anchor. Non-anchor cells keep their
	// content (hidden by the view); unmerge reveals it again.
	Merges map[CellRef]Span `json:"-"`
}

func NewSheet(id, name string) *Sheet {
	return &Sheet{Id: id, Name: name, Cells: map[CellRef]Cell{}, ColWidths: map[int]int{}, RowHeights: map[int]int{}, Merges: map[CellRef]Span{}}
}

// SetCell stores a cell, dropping it from storage if empty (keeps it sparse).
func (s *Sheet) SetCell(ref CellRef, c Cell) {
	if c.IsEmpty() {
		delete(s.Cells, ref)
		return
	}
	s.Cells[ref] = c
}

// GetCell returns the cell at ref, or the zero Cell if unset.
func (s *Sheet) GetCell(ref CellRef) Cell {
	return s.Cells[ref]
}

func (s *Sheet) clone() *Sheet {
	cp := &Sheet{
		Id: s.Id, Name: s.Name, Cells: make(map[CellRef]Cell, len(s.Cells)),
		ColWidths: maps.Clone(s.ColWidths), RowHeights: maps.Clone(s.RowHeights),
		FrozenRows: s.FrozenRows, FrozenCols: s.FrozenCols,
		Merges: maps.Clone(s.Merges),
	}
	maps.Copy(cp.Cells, s.Cells)
	return cp
}

// shiftDims rebuilds a sparse dimension map after an insert/delete at index.
// delta > 0 inserts (indices at/after move up); delta < 0 deletes a band of
// -delta indices (entries inside the band are dropped).
func shiftDims(m map[int]int, index, delta int) map[int]int {
	if len(m) == 0 {
		return m
	}
	next := make(map[int]int, len(m))
	for i, v := range m {
		if delta < 0 && i >= index && i < index-delta {
			continue // deleted band
		}
		next[shiftCoord(i, index, delta)] = v
	}
	return next
}

// intersects reports whether the merge at anchor overlaps the inclusive
// rectangle [r0..r1] x [c0..c1].
func intersects(anchor CellRef, sp Span, r0, c0, r1, c1 int) bool {
	return anchor.Row <= r1 && anchor.Row+sp.Rows-1 >= r0 &&
		anchor.Col <= c1 && anchor.Col+sp.Cols-1 >= c0
}

// shiftMerges rebuilds the merge map after a row (axis "row") or col insert
// (delta > 0) / delete of -delta indices at index. Endpoints shift like cell
// coordinates (an insert inside a merge grows it, a delete shrinks it);
// merges that collapse to a single cell are dropped.
func shiftMerges(m map[CellRef]Span, axis string, index, delta int) map[CellRef]Span {
	if len(m) == 0 {
		return m
	}
	next := make(map[CellRef]Span, len(m))
	for a, sp := range m {
		lo, span := a.Row, sp.Rows
		if axis == "col" {
			lo, span = a.Col, sp.Cols
		}
		// Shift the anchor and the EXCLUSIVE end: shiftCoord clamps in-band
		// coords to index, which is exactly right for an exclusive bound.
		nlo := shiftCoord(lo, index, delta)
		nspan := shiftCoord(lo+span, index, delta) - nlo
		if nspan <= 0 {
			continue // merge entirely inside a deleted band
		}
		na, nsp := a, sp
		if axis == "col" {
			na.Col, nsp.Cols = nlo, nspan
		} else {
			na.Row, nsp.Rows = nlo, nspan
		}
		if nsp.Rows <= 1 && nsp.Cols <= 1 {
			continue // collapsed to a single cell
		}
		next[na] = nsp
	}
	return next
}

// remap rebuilds the sparse cell map by transforming each ref. The fn returns
// the new ref and whether to keep the cell. Used by structural row/col ops.
func (s *Sheet) remap(fn func(CellRef) (CellRef, bool)) {
	next := make(map[CellRef]Cell, len(s.Cells))
	for ref, c := range s.Cells {
		nref, keep := fn(ref)
		if keep {
			next[nref] = c
		}
	}
	s.Cells = next
}

// Workbook is the full document: ordered sheets plus the shared StylePool.
type Workbook struct {
	Sheets []*Sheet   `json:"sheets"`
	Styles *StylePool `json:"styles"`
}

func NewWorkbook() *Workbook {
	return &Workbook{Sheets: []*Sheet{}, Styles: NewStylePool()}
}

func (w *Workbook) AddSheet(id, name string) *Sheet {
	s := NewSheet(id, name)
	w.Sheets = append(w.Sheets, s)
	return s
}

func (w *Workbook) SheetByID(id string) *Sheet {
	for _, s := range w.Sheets {
		if s.Id == id {
			return s
		}
	}
	return nil
}

// Clone returns a deep copy so callers can simulate clients independently.
func (w *Workbook) Clone() *Workbook {
	cp := &Workbook{
		Sheets: make([]*Sheet, len(w.Sheets)),
		Styles: w.Styles.clone(),
	}
	for i, s := range w.Sheets {
		cp.Sheets[i] = s.clone()
	}
	return cp
}
