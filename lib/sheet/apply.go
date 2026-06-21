package sheet

import "fmt"

// Apply mutates the workbook by op. The op is assumed already rebased to the
// current revision (see reconcile.go). Cell ops are last-writer-wins; the
// caller's total ordering decides the winner.
func (w *Workbook) Apply(op Op) error {
	if err := op.Validate(); err != nil {
		return err
	}
	s := w.SheetByID(op.Sheet)
	if s == nil {
		return fmt.Errorf("apply: unknown sheet %q", op.Sheet)
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
		if op.StyleId != nil {
			cur.StyleId = *op.StyleId
		}
		s.SetCell(CellRef{op.Row, op.Col}, cur)
	case OpSetStyle:
		cur := s.GetCell(CellRef{op.Row, op.Col})
		cur.StyleId = *op.StyleId
		s.SetCell(CellRef{op.Row, op.Col}, cur)
	case OpClearRange:
		for ref := range s.Cells {
			if ref.Row >= op.Row && ref.Row <= op.EndRow && ref.Col >= op.Col && ref.Col <= op.EndCol {
				delete(s.Cells, ref)
			}
		}
	case OpInsertRows:
		s.remap(func(r CellRef) (CellRef, bool) {
			if r.Row >= op.Index {
				return CellRef{r.Row + op.Count, r.Col}, true
			}
			return r, true
		})
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
	case OpInsertCols:
		s.remap(func(r CellRef) (CellRef, bool) {
			if r.Col >= op.Index {
				return CellRef{r.Row, r.Col + op.Count}, true
			}
			return r, true
		})
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
	default:
		return fmt.Errorf("apply: unhandled op type %q", op.Type)
	}
	return nil
}
