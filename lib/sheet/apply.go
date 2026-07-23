package sheet

import (
	"fmt"
	"slices"
)

// Apply mutates the workbook by op. The op is assumed already rebased to the
// current revision (see reconcile.go). Cell ops are last-writer-wins; the
// caller's total ordering decides the winner.
func (w *Workbook) Apply(op Op) error {
	if err := op.Validate(); err != nil {
		return err
	}

	// Sheet-list ops manage w.Sheets itself and never need an existing sheet.
	switch op.Type {
	case OpAddSheet:
		if w.SheetByID(op.Sheet) != nil {
			return nil // concurrent duplicate add: first wins
		}
		w.Sheets = slices.Insert(w.Sheets, min(op.Index, len(w.Sheets)), NewSheet(op.Sheet, op.Name))
		return nil
	case OpDeleteSheet:
		if len(w.Sheets) <= 1 {
			return nil // never delete the last sheet
		}
		for i, s := range w.Sheets {
			if s.Id == op.Sheet {
				w.Sheets = slices.Delete(w.Sheets, i, i+1)
				return nil
			}
		}
		return nil
	case OpRenameSheet:
		if s := w.SheetByID(op.Sheet); s != nil {
			s.Name = op.Name
		}
		return nil
	case OpMoveSheet:
		for i, s := range w.Sheets {
			if s.Id == op.Sheet {
				rest := slices.Delete(slices.Clone(w.Sheets), i, i+1)
				w.Sheets = slices.Insert(rest, min(op.ToIndex, len(rest)), s)
				return nil
			}
		}
		return nil
	}

	s := w.SheetByID(op.Sheet)
	if s == nil {
		// The sheet was deleted by an op ordered earlier; late ops targeting it
		// converge as no-ops instead of poisoning the ordered-log replay.
		return nil
	}
	switch op.Type {
	case OpSetCell:
		cur := s.GetCell(CellRef{op.Row, op.Col})
		if op.Raw != nil {
			cur.Raw = *op.Raw
			cur.Value = ""
			cur.ValueType = ""
		}
		if op.Value != nil {
			cur.Value = *op.Value
		}
		if op.ValueType != nil {
			cur.ValueType = *op.ValueType
		}
		if op.Props != nil {
			cur.StyleId = w.Styles.Put(Style{Props: op.Props})
		} else if op.StyleId != nil {
			cur.StyleId = *op.StyleId
		}
		s.SetCell(CellRef{op.Row, op.Col}, cur)
	case OpSetStyle:
		cur := s.GetCell(CellRef{op.Row, op.Col})
		if op.Props != nil {
			cur.StyleId = w.Styles.Put(Style{Props: op.Props})
		} else {
			cur.StyleId = *op.StyleId
		}
		s.SetCell(CellRef{op.Row, op.Col}, cur)
	case OpClearRange:
		for ref := range s.Cells {
			if ref.Row >= op.Row && ref.Row <= op.EndRow && ref.Col >= op.Col && ref.Col <= op.EndCol {
				delete(s.Cells, ref)
			}
		}
	case OpSetDimension:
		if op.Axis == "col" {
			s.ColWidths[op.Index] = op.Size
		} else {
			s.RowHeights[op.Index] = op.Size
		}
	case OpSetFreeze:
		s.FrozenRows = op.FrozenRows
		s.FrozenCols = op.FrozenCols
	case OpMergeCells:
		if op.EndRow == op.Row && op.EndCol == op.Col {
			return nil // degenerate 1x1 (e.g. collapsed by a rebase): no-op
		}
		// Excel semantics: merging over existing merges absorbs them. Cell
		// content is kept (the view hides non-anchor cells; unmerge reveals it).
		for a, sp := range s.Merges {
			if intersects(a, sp, op.Row, op.Col, op.EndRow, op.EndCol) {
				delete(s.Merges, a)
			}
		}
		s.Merges[CellRef{op.Row, op.Col}] = Span{Rows: op.EndRow - op.Row + 1, Cols: op.EndCol - op.Col + 1}
	case OpUnmergeCells:
		for a, sp := range s.Merges {
			if intersects(a, sp, op.Row, op.Col, op.EndRow, op.EndCol) {
				delete(s.Merges, a)
			}
		}
	case OpInsertRows:
		s.remap(func(r CellRef) (CellRef, bool) {
			if r.Row >= op.Index {
				return CellRef{r.Row + op.Count, r.Col}, true
			}
			return r, true
		})
		s.RowHeights = shiftDims(s.RowHeights, op.Index, op.Count)
		s.Merges = shiftMerges(s.Merges, "row", op.Index, op.Count)
	case OpDeleteRows:
		s.remap(func(r CellRef) (CellRef, bool) {
			if r.Row >= op.Index && r.Row < op.Index+op.Count {
				return r, false // drop
			}
			if r.Row >= op.Index+op.Count {
				return CellRef{r.Row - op.Count, r.Col}, true
			}
			return r, true
		})
		s.RowHeights = shiftDims(s.RowHeights, op.Index, -op.Count)
		s.Merges = shiftMerges(s.Merges, "row", op.Index, -op.Count)
	case OpInsertCols:
		s.remap(func(r CellRef) (CellRef, bool) {
			if r.Col >= op.Index {
				return CellRef{r.Row, r.Col + op.Count}, true
			}
			return r, true
		})
		s.ColWidths = shiftDims(s.ColWidths, op.Index, op.Count)
		s.Merges = shiftMerges(s.Merges, "col", op.Index, op.Count)
	case OpDeleteCols:
		s.remap(func(r CellRef) (CellRef, bool) {
			if r.Col >= op.Index && r.Col < op.Index+op.Count {
				return r, false
			}
			if r.Col >= op.Index+op.Count {
				return CellRef{r.Row, r.Col - op.Count}, true
			}
			return r, true
		})
		s.ColWidths = shiftDims(s.ColWidths, op.Index, -op.Count)
		s.Merges = shiftMerges(s.Merges, "col", op.Index, -op.Count)
	default:
		return fmt.Errorf("apply: unhandled op type %q", op.Type)
	}
	return nil
}
