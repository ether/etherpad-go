package sheet

import "maps"

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
}

func NewSheet(id, name string) *Sheet {
	return &Sheet{Id: id, Name: name, Cells: map[CellRef]Cell{}, ColWidths: map[int]int{}, RowHeights: map[int]int{}}
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
