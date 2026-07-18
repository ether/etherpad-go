package sheet

import "testing"

func mergeOp(sheet string, r0, c0, r1, c1 int) Op {
	return Op{Type: OpMergeCells, Sheet: sheet, Row: r0, Col: c0, EndRow: r1, EndCol: c1}
}

func TestMergeUnmergeApply(t *testing.T) {
	wb := NewWorkbook()
	wb.AddSheet("s1", "Sheet1")

	if err := wb.Apply(mergeOp("s1", 1, 1, 3, 2)); err != nil {
		t.Fatalf("merge: %v", err)
	}
	s := wb.SheetByID("s1")
	if sp := s.Merges[CellRef{1, 1}]; sp != (Span{Rows: 3, Cols: 2}) {
		t.Fatalf("merge span = %+v", sp)
	}

	// Overlapping merge absorbs the existing one (Excel semantics).
	if err := wb.Apply(mergeOp("s1", 2, 2, 5, 5)); err != nil {
		t.Fatalf("merge 2: %v", err)
	}
	if len(s.Merges) != 1 {
		t.Fatalf("expected absorb, merges = %v", s.Merges)
	}
	if sp := s.Merges[CellRef{2, 2}]; sp != (Span{Rows: 4, Cols: 4}) {
		t.Fatalf("absorbed span = %+v", sp)
	}

	// Unmerge by any intersecting rectangle.
	if err := wb.Apply(Op{Type: OpUnmergeCells, Sheet: "s1", Row: 3, Col: 3, EndRow: 3, EndCol: 3}); err != nil {
		t.Fatalf("unmerge: %v", err)
	}
	if len(s.Merges) != 0 {
		t.Fatalf("merges left after unmerge: %v", s.Merges)
	}

	// Degenerate 1x1 merge (possible after rebase) is a no-op, not an error.
	if err := wb.Apply(mergeOp("s1", 0, 0, 0, 0)); err != nil {
		t.Fatalf("1x1 merge: %v", err)
	}
	if len(s.Merges) != 0 {
		t.Fatalf("1x1 merge stored: %v", s.Merges)
	}
}

func TestMergeStructuralShifts(t *testing.T) {
	newWb := func() *Workbook {
		wb := NewWorkbook()
		wb.AddSheet("s1", "Sheet1")
		// rows 2-4, cols 1-2
		if err := wb.Apply(mergeOp("s1", 2, 1, 4, 2)); err != nil {
			t.Fatalf("seed merge: %v", err)
		}
		return wb
	}

	// Insert above: merge moves down.
	wb := newWb()
	_ = wb.Apply(Op{Type: OpInsertRows, Sheet: "s1", Index: 0, Count: 2})
	if sp, ok := wb.SheetByID("s1").Merges[CellRef{4, 1}]; !ok || sp != (Span{3, 2}) {
		t.Fatalf("insert above: %v", wb.SheetByID("s1").Merges)
	}

	// Insert inside: merge grows.
	wb = newWb()
	_ = wb.Apply(Op{Type: OpInsertRows, Sheet: "s1", Index: 3, Count: 1})
	if sp, ok := wb.SheetByID("s1").Merges[CellRef{2, 1}]; !ok || sp != (Span{4, 2}) {
		t.Fatalf("insert inside: %v", wb.SheetByID("s1").Merges)
	}

	// Delete a band overlapping the bottom: merge shrinks.
	wb = newWb()
	_ = wb.Apply(Op{Type: OpDeleteRows, Sheet: "s1", Index: 4, Count: 3})
	if sp, ok := wb.SheetByID("s1").Merges[CellRef{2, 1}]; !ok || sp != (Span{2, 2}) {
		t.Fatalf("delete bottom: %v", wb.SheetByID("s1").Merges)
	}

	// Delete the whole row range: merge is dropped.
	wb = newWb()
	_ = wb.Apply(Op{Type: OpDeleteRows, Sheet: "s1", Index: 2, Count: 3})
	if len(wb.SheetByID("s1").Merges) != 0 {
		t.Fatalf("delete all rows: %v", wb.SheetByID("s1").Merges)
	}

	// Delete cols so the merge collapses to a single column AND single... no:
	// rows stay 3, cols 2->1: still a 3x1 merge (kept). Deleting both cols drops it.
	wb = newWb()
	_ = wb.Apply(Op{Type: OpDeleteCols, Sheet: "s1", Index: 1, Count: 1})
	if sp, ok := wb.SheetByID("s1").Merges[CellRef{2, 1}]; !ok || sp != (Span{3, 1}) {
		t.Fatalf("delete one col: %v", wb.SheetByID("s1").Merges)
	}
	_ = wb.Apply(Op{Type: OpDeleteCols, Sheet: "s1", Index: 1, Count: 1})
	if len(wb.SheetByID("s1").Merges) != 0 {
		t.Fatalf("delete both cols: %v", wb.SheetByID("s1").Merges)
	}
}

func TestMergeTransform(t *testing.T) {
	in := mergeOp("s1", 2, 1, 4, 2)
	out := Transform(in, Op{Type: OpInsertRows, Sheet: "s1", Index: 3, Count: 2})
	if out.Row != 2 || out.EndRow != 6 {
		t.Fatalf("transform insert: %+v", out)
	}
	out = Transform(in, Op{Type: OpDeleteRows, Sheet: "s1", Index: 0, Count: 2})
	if out.Row != 0 || out.EndRow != 2 {
		t.Fatalf("transform delete: %+v", out)
	}
	// Concurrent delete of the entire range collapses the merge to 1x1 in one
	// dimension; Submit must not error (Apply no-ops when fully degenerate).
	out = Transform(in, Op{Type: OpDeleteRows, Sheet: "s1", Index: 2, Count: 3})
	if out.Row != 2 || out.EndRow != 2 {
		t.Fatalf("transform collapse: %+v", out)
	}
	if err := out.Validate(); err != nil {
		t.Fatalf("collapsed merge must stay valid: %v", err)
	}
}

func TestMergeSnapshotRoundTrip(t *testing.T) {
	wb := NewWorkbook()
	wb.AddSheet("s1", "Sheet1")
	_ = wb.Apply(mergeOp("s1", 0, 0, 1, 1))
	_ = wb.Apply(mergeOp("s1", 5, 5, 5, 7))

	got := WorkbookFromSnapshot(wb.Snapshot())
	s := got.SheetByID("s1")
	if len(s.Merges) != 2 || s.Merges[CellRef{0, 0}] != (Span{2, 2}) || s.Merges[CellRef{5, 5}] != (Span{1, 3}) {
		t.Fatalf("snapshot roundtrip merges = %v", s.Merges)
	}
}
