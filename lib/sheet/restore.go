package sheet

import (
	"maps"
	"slices"
	"sort"
)

// Default sizes a restored sheet uses where the old version had no own
// width/height but the current one has: ops cannot remove a dimension, so
// it is set to the grid's plain size.
const (
	defaultColWidth  = 80
	defaultRowHeight = 22
)

// RestoreOps returns the ops that turn cur into target: restoring an old
// version is ordinary ops on top of the history (which stays), live for
// everyone and undone like any edit.
func RestoreOps(cur, target *Workbook) []Op {
	var ops []Op
	order := make([]string, 0, len(cur.Sheets))
	for _, s := range cur.Sheets {
		order = append(order, s.Id)
	}

	// Sheets: add the missing ones first, so a sheet always remains.
	for i, t := range target.Sheets {
		if cur.SheetByID(t.Id) == nil {
			at := min(i, len(order))
			ops = append(ops, Op{Type: OpAddSheet, Sheet: t.Id, Name: t.Name, Index: at})
			order = slices.Insert(order, at, t.Id)
		}
	}
	for _, s := range cur.Sheets {
		if target.SheetByID(s.Id) == nil {
			ops = append(ops, Op{Type: OpDeleteSheet, Sheet: s.Id})
			order = slices.DeleteFunc(order, func(id string) bool { return id == s.Id })
		}
	}
	for i, t := range target.Sheets {
		if c := cur.SheetByID(t.Id); c != nil && c.Name != t.Name {
			ops = append(ops, Op{Type: OpRenameSheet, Sheet: t.Id, Name: t.Name})
		}
		if j := slices.Index(order, t.Id); j != i {
			ops = append(ops, Op{Type: OpMoveSheet, Sheet: t.Id, ToIndex: i})
			order = slices.Delete(order, j, j+1)
			order = slices.Insert(order, min(i, len(order)), t.Id)
		}
	}

	for _, t := range target.Sheets {
		c := cur.SheetByID(t.Id)
		if c == nil {
			c = NewSheet(t.Id, t.Name)
		}
		ops = append(ops, restoreSheetOps(cur, c, target, t)...)
	}
	return ops
}

func propsOf(w *Workbook, id int) map[string]string {
	if id == 0 {
		return nil
	}
	st, _ := w.Styles.Get(id)
	if len(st.Props) == 0 {
		return nil
	}
	return st.Props
}

func sortedRefs(m map[CellRef]Cell) []CellRef {
	refs := make([]CellRef, 0, len(m))
	for r := range m {
		refs = append(refs, r)
	}
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].Row < refs[j].Row || refs[i].Row == refs[j].Row && refs[i].Col < refs[j].Col
	})
	return refs
}

func restoreSheetOps(cw *Workbook, c *Sheet, tw *Workbook, t *Sheet) []Op {
	var ops []Op
	id := t.Id

	// Merges that are not in the old version go first (their cells may be
	// written below).
	for a, sp := range c.Merges {
		if t.Merges[a] != sp {
			ops = append(ops, Op{Type: OpUnmergeCells, Sheet: id, Row: a.Row, Col: a.Col, EndRow: a.Row + sp.Rows - 1, EndCol: a.Col + sp.Cols - 1})
		}
	}

	// Cells. An old version without any cells is one clearRange.
	if len(t.Cells) == 0 && len(c.Cells) > 0 {
		r1, c1 := 0, 0
		for r := range c.Cells {
			r1, c1 = max(r1, r.Row), max(c1, r.Col)
		}
		ops = append(ops, Op{Type: OpClearRange, Sheet: id, EndRow: r1, EndCol: c1})
	} else {
		for _, ref := range sortedRefs(c.Cells) {
			if _, ok := t.Cells[ref]; !ok {
				ops = append(ops, Op{Type: OpClearRange, Sheet: id, Row: ref.Row, Col: ref.Col, EndRow: ref.Row, EndCol: ref.Col})
			}
		}
		for _, ref := range sortedRefs(t.Cells) {
			tc := t.Cells[ref]
			tp := propsOf(tw, tc.StyleId)
			if cc, ok := c.Cells[ref]; ok && cc.Raw == tc.Raw && maps.Equal(propsOf(cw, cc.StyleId), tp) {
				continue
			}
			raw := tc.Raw
			op := Op{Type: OpSetCell, Sheet: id, Row: ref.Row, Col: ref.Col, Raw: &raw}
			if tp != nil {
				op.Props = maps.Clone(tp)
			} else {
				zero := 0
				op.StyleId = &zero
			}
			ops = append(ops, op)
		}
	}

	for a, sp := range t.Merges {
		if c.Merges[a] != sp {
			ops = append(ops, Op{Type: OpMergeCells, Sheet: id, Row: a.Row, Col: a.Col, EndRow: a.Row + sp.Rows - 1, EndCol: a.Col + sp.Cols - 1})
		}
	}

	dims := func(axis string, cur, old map[int]int, plain int) {
		keys := make([]int, 0, len(cur)+len(old))
		for k := range cur {
			keys = append(keys, k)
		}
		for k := range old {
			if _, ok := cur[k]; !ok {
				keys = append(keys, k)
			}
		}
		sort.Ints(keys)
		for _, k := range keys {
			want, ok := old[k]
			if !ok {
				want = plain
			}
			if cur[k] != want {
				ops = append(ops, Op{Type: OpSetDimension, Sheet: id, Axis: axis, Index: k, Size: want})
			}
		}
	}
	dims("col", c.ColWidths, t.ColWidths, defaultColWidth)
	dims("row", c.RowHeights, t.RowHeights, defaultRowHeight)

	if c.FrozenRows != t.FrozenRows || c.FrozenCols != t.FrozenCols {
		ops = append(ops, Op{Type: OpSetFreeze, Sheet: id, FrozenRows: t.FrozenRows, FrozenCols: t.FrozenCols})
	}

	sameFilters := c.FilterRow == t.FilterRow && len(c.Filters) == len(t.Filters) && maps.Equal(c.FilterHide, t.FilterHide)
	for col, v := range t.Filters {
		sameFilters = sameFilters && slices.Equal(c.Filters[col], v)
	}
	if !sameFilters {
		ops = append(ops, Op{Type: OpSetFilter, Sheet: id, Clear: true})
		cols := make([]int, 0, len(t.Filters))
		for col := range t.Filters {
			cols = append(cols, col)
		}
		sort.Ints(cols)
		for _, col := range cols {
			ops = append(ops, Op{Type: OpSetFilter, Sheet: id, Row: t.FilterRow, Col: col, Values: slices.Clone(t.Filters[col]), Exclude: t.FilterHide[col]})
		}
	}

	for _, ch := range c.Charts {
		if t.chartIndex(ch.Id) < 0 {
			ops = append(ops, Op{Type: OpDeleteChart, Sheet: id, Chart: &Chart{Id: ch.Id}})
		}
	}
	for i, ch := range t.Charts {
		if j := c.chartIndex(ch.Id); j >= 0 && c.Charts[j] == ch {
			continue
		}
		cp := ch
		ops = append(ops, Op{Type: OpSetChart, Sheet: id, Row: ch.Row, Col: ch.Col, Order: i + 1, Chart: &cp})
	}
	return ops
}
