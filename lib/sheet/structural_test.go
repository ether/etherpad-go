package sheet

import "testing"

func wb2() *Workbook {
	w := NewWorkbook()
	w.AddSheet("s1", "Sheet1")
	w.AddSheet("s2", "Sheet2")
	return w
}

func sheetIds(w *Workbook) []string {
	ids := make([]string, len(w.Sheets))
	for i, s := range w.Sheets {
		ids[i] = s.Id
	}
	return ids
}

func TestSheetListOps(t *testing.T) {
	w := wb2()
	if err := w.Apply(Op{Type: OpAddSheet, Sheet: "s3", Name: "Drei", Index: 1}); err != nil {
		t.Fatal(err)
	}
	if got := sheetIds(w); got[0] != "s1" || got[1] != "s3" || got[2] != "s2" {
		t.Fatalf("addSheet at index 1: got %v", got)
	}
	// duplicate add converges as no-op
	if err := w.Apply(Op{Type: OpAddSheet, Sheet: "s3", Name: "Nochmal", Index: 0}); err != nil {
		t.Fatal(err)
	}
	if len(w.Sheets) != 3 || w.SheetByID("s3").Name != "Drei" {
		t.Fatalf("duplicate addSheet must be a no-op: %v", sheetIds(w))
	}
	if err := w.Apply(Op{Type: OpRenameSheet, Sheet: "s3", Name: "Umbenannt"}); err != nil {
		t.Fatal(err)
	}
	if w.SheetByID("s3").Name != "Umbenannt" {
		t.Fatal("renameSheet did not rename")
	}
	if err := w.Apply(Op{Type: OpMoveSheet, Sheet: "s3", ToIndex: 2}); err != nil {
		t.Fatal(err)
	}
	if got := sheetIds(w); got[2] != "s3" {
		t.Fatalf("moveSheet to end: got %v", got)
	}
	if err := w.Apply(Op{Type: OpDeleteSheet, Sheet: "s3"}); err != nil {
		t.Fatal(err)
	}
	if len(w.Sheets) != 2 || w.SheetByID("s3") != nil {
		t.Fatal("deleteSheet did not delete")
	}
}

func TestDeleteLastSheetIsNoop(t *testing.T) {
	w := NewWorkbook()
	w.AddSheet("s1", "Sheet1")
	if err := w.Apply(Op{Type: OpDeleteSheet, Sheet: "s1"}); err != nil {
		t.Fatal(err)
	}
	if len(w.Sheets) != 1 {
		t.Fatal("the last sheet must never be deleted")
	}
}

func TestOpsOnDeletedSheetAreNoops(t *testing.T) {
	w := wb2()
	raw := "x"
	if err := w.Apply(Op{Type: OpDeleteSheet, Sheet: "s2"}); err != nil {
		t.Fatal(err)
	}
	// A late op composed against the pre-delete state must not error.
	if err := w.Apply(Op{Type: OpSetCell, Sheet: "s2", Row: 0, Col: 0, Raw: &raw}); err != nil {
		t.Fatalf("op on deleted sheet must be a no-op, got %v", err)
	}
}

func TestSetDimensionAndFreeze(t *testing.T) {
	w := wb2()
	if err := w.Apply(Op{Type: OpSetDimension, Sheet: "s1", Axis: "col", Index: 2, Size: 140}); err != nil {
		t.Fatal(err)
	}
	if err := w.Apply(Op{Type: OpSetDimension, Sheet: "s1", Axis: "row", Index: 5, Size: 40}); err != nil {
		t.Fatal(err)
	}
	s := w.SheetByID("s1")
	if s.ColWidths[2] != 140 || s.RowHeights[5] != 40 {
		t.Fatalf("dims not stored: %v %v", s.ColWidths, s.RowHeights)
	}
	if err := w.Apply(Op{Type: OpSetFreeze, Sheet: "s1", FrozenRows: 1, FrozenCols: 1}); err != nil {
		t.Fatal(err)
	}
	if s.FrozenRows != 1 || s.FrozenCols != 1 {
		t.Fatal("freeze not stored")
	}
}

func TestDimsShiftUnderStructuralOps(t *testing.T) {
	w := wb2()
	must := func(op Op) {
		t.Helper()
		if err := w.Apply(op); err != nil {
			t.Fatal(err)
		}
	}
	must(Op{Type: OpSetDimension, Sheet: "s1", Axis: "col", Index: 3, Size: 120})
	must(Op{Type: OpSetDimension, Sheet: "s1", Axis: "row", Index: 4, Size: 44})

	must(Op{Type: OpInsertCols, Sheet: "s1", Index: 0, Count: 2})
	if w.SheetByID("s1").ColWidths[5] != 120 {
		t.Fatalf("col width must shift right on insert: %v", w.SheetByID("s1").ColWidths)
	}
	must(Op{Type: OpDeleteCols, Sheet: "s1", Index: 0, Count: 2})
	if w.SheetByID("s1").ColWidths[3] != 120 {
		t.Fatalf("col width must shift back on delete: %v", w.SheetByID("s1").ColWidths)
	}
	// deleting the band containing the override drops it
	must(Op{Type: OpDeleteRows, Sheet: "s1", Index: 4, Count: 1})
	if len(w.SheetByID("s1").RowHeights) != 0 {
		t.Fatalf("row height inside deleted band must drop: %v", w.SheetByID("s1").RowHeights)
	}
}

func TestSnapshotRoundTripDimsAndSheets(t *testing.T) {
	w := wb2()
	w.SheetByID("s1").ColWidths[1] = 99
	w.SheetByID("s1").RowHeights[2] = 33
	w.SheetByID("s1").FrozenRows = 1
	got := WorkbookFromSnapshot(w.Snapshot())
	s := got.SheetByID("s1")
	if s.ColWidths[1] != 99 || s.RowHeights[2] != 33 || s.FrozenRows != 1 {
		t.Fatalf("snapshot round-trip lost dims/freeze: %+v", s)
	}
	if len(got.Sheets) != 2 || got.Sheets[1].Id != "s2" {
		t.Fatal("snapshot round-trip lost sheet order")
	}
}

func TestTransformSetDimensionUnderInsert(t *testing.T) {
	in := Op{Type: OpSetDimension, Sheet: "s1", Axis: "col", Index: 3, Size: 100}
	out := Transform(in, Op{Type: OpInsertCols, Sheet: "s1", Index: 1, Count: 2})
	if out.Index != 5 {
		t.Fatalf("setDimension col index must shift: got %d", out.Index)
	}
	// row-axis dimension is untouched by column inserts
	in.Axis = "row"
	out = Transform(in, Op{Type: OpInsertCols, Sheet: "s1", Index: 1, Count: 2})
	if out.Index != 3 {
		t.Fatalf("row dimension must not shift under col insert: got %d", out.Index)
	}
}

func TestStructuralValidate(t *testing.T) {
	bad := []Op{
		{Type: OpAddSheet, Sheet: "sX"},                                       // missing name
		{Type: OpRenameSheet, Sheet: "s1"},                                    // missing name
		{Type: OpMoveSheet, Sheet: "s1", ToIndex: -1},                         // negative index
		{Type: OpSetDimension, Sheet: "s1", Axis: "diag", Index: 0, Size: 10}, // bad axis
		{Type: OpSetDimension, Sheet: "s1", Axis: "col", Index: 0, Size: 0},   // zero size
		{Type: OpSetDimension, Sheet: "s1", Axis: "col", Index: 0, Size: 1e6}, // huge size
		{Type: OpSetFreeze, Sheet: "s1", FrozenRows: 2},                       // only 0/1
	}
	for _, op := range bad {
		if op.Validate() == nil {
			t.Fatalf("op %+v must be invalid", op)
		}
	}
}

func TestConvergenceConcurrentSheetOps(t *testing.T) {
	// Two clients: A deletes s2 while B types into s2. Both orders converge.
	raw := "late"
	a := Op{Type: OpDeleteSheet, Sheet: "s2"}
	b := Op{Type: OpSetCell, Sheet: "s2", Row: 0, Col: 0, Raw: &raw}

	w1 := wb2()
	if err := w1.Apply(a); err != nil {
		t.Fatal(err)
	}
	if err := w1.Apply(Transform(b, a)); err != nil {
		t.Fatal(err)
	}

	w2 := wb2()
	if err := w2.Apply(b); err != nil {
		t.Fatal(err)
	}
	if err := w2.Apply(Transform(a, b)); err != nil {
		t.Fatal(err)
	}

	if w1.SheetByID("s2") != nil || w2.SheetByID("s2") != nil {
		t.Fatal("delete must win in both orders")
	}
}
