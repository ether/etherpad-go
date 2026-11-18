package sheet

import (
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

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
	// Number of frozen top rows / left columns (position: sticky in the view).
	FrozenRows int `json:"-"`
	FrozenCols int `json:"-"`
	// Merged-cell ranges keyed by top-left anchor. Non-anchor cells keep their
	// content (hidden by the view); unmerge reveals it again.
	Merges map[CellRef]Span `json:"-"`
	// Filters: column -> the values whose rows are shown. Rows up to
	// FilterRow (the header) always show.
	Filters   map[int][]string `json:"-"`
	FilterRow int              `json:"-"`
	// FilterHide marks the columns whose Filters list the hidden values
	// instead of the shown ones.
	FilterHide map[int]bool `json:"-"`
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
		Merges: maps.Clone(s.Merges), FilterRow: s.FilterRow, FilterHide: maps.Clone(s.FilterHide),
	}
	if s.Filters != nil {
		cp.Filters = make(map[int][]string, len(s.Filters))
		for c, v := range s.Filters {
			cp.Filters[c] = slices.Clone(v)
		}
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
		// Shift the anchor and the EXCLUSIVE end. The end needs shiftEnd: on
		// insert an exclusive bound only moves for index STRICTLY inside
		// (coord > index), or an insert at the merge's trailing edge would
		// grow it; on delete shiftCoord's in-band clamp is exactly right.
		nlo := shiftCoord(lo, index, delta)
		nspan := shiftEnd(lo+span, index, delta) - nlo
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

// shiftEnd shifts an EXCLUSIVE upper bound: like shiftCoord, except an insert
// exactly at the bound (coord == index) does not move it.
func shiftEnd(coord, index, delta int) int {
	if delta >= 0 && coord == index {
		return coord
	}
	return shiftCoord(coord, index, delta)
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

// shiftFilters moves column filters after a column insert (delta > 0) or the
// delete of -delta columns at index; filters of deleted columns go.
func shiftFilters[V any](m map[int]V, index, delta int) map[int]V {
	if len(m) == 0 {
		return m
	}
	next := make(map[int]V, len(m))
	for c, v := range m {
		if delta < 0 && c >= index && c < index-delta {
			continue
		}
		next[shiftCoord(c, index, delta)] = v
	}
	return next
}

// FilterKey is how filter values compare, in the server and the browser
// (cellFilter.ts): without case and surrounding space, and numbers by value,
// so Excel's "1,234.5" in an imported filter matches a cell showing 1234.5.
func FilterKey(v string) string {
	t := strings.ToLower(strings.TrimSpace(v))
	n := t
	if thousandsRe.MatchString(n) {
		n = strings.ReplaceAll(n, ",", "")
	}
	if f, err := strconv.ParseFloat(n, 64); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) && numberRe.MatchString(n) {
		return "#" + strconv.FormatFloat(f, 'g', -1, 64)
	}
	return t
}

var (
	thousandsRe = regexp.MustCompile(`^-?\d{1,3}(,\d{3})+(\.\d+)?$`)
	numberRe    = regexp.MustCompile(`^-?(\d+\.?\d*|\.\d+)(e[-+]?\d+)?$`)
)

// FilterKeeps reports whether a column filter keeps a row with this value.
// Empty cells never hide a row.
func (s *Sheet) FilterKeeps(col int, value string) bool {
	values, ok := s.Filters[col]
	if !ok || strings.TrimSpace(value) == "" {
		return true
	}
	k := FilterKey(value)
	listed := slices.ContainsFunc(values, func(v string) bool { return FilterKey(v) == k })
	return listed != s.FilterHide[col]
}
