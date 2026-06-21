package sheet

import "testing"

func mkWB(t *testing.T) *Workbook {
	t.Helper()
	w := NewWorkbook()
	w.AddSheet("s1", "Sheet1")
	return w
}

func ptr(s string) *string { return &s }

func TestApplySetCell(t *testing.T) {
	w := mkWB(t)
	if err := w.Apply(Op{Type: OpSetCell, Sheet: "s1", Row: 1, Col: 1, Raw: ptr("42")}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if w.SheetByID("s1").GetCell(CellRef{1, 1}).Raw != "42" {
		t.Fatal("setCell did not store raw")
	}
}

func TestApplyClearRange(t *testing.T) {
	w := mkWB(t)
	s := w.SheetByID("s1")
	s.SetCell(CellRef{0, 0}, Cell{Raw: "a"})
	s.SetCell(CellRef{1, 1}, Cell{Raw: "b"})
	s.SetCell(CellRef{5, 5}, Cell{Raw: "keep"})
	if err := w.Apply(Op{Type: OpClearRange, Sheet: "s1", Row: 0, Col: 0, EndRow: 2, EndCol: 2}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !s.GetCell(CellRef{0, 0}).IsEmpty() || !s.GetCell(CellRef{1, 1}).IsEmpty() {
		t.Fatal("clearRange did not clear cells in range")
	}
	if s.GetCell(CellRef{5, 5}).Raw != "keep" {
		t.Fatal("clearRange cleared a cell outside the range")
	}
}

func TestApplyInsertRowsShiftsCells(t *testing.T) {
	w := mkWB(t)
	s := w.SheetByID("s1")
	s.SetCell(CellRef{3, 0}, Cell{Raw: "row3"})
	if err := w.Apply(Op{Type: OpInsertRows, Sheet: "s1", Index: 2, Count: 2}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !s.GetCell(CellRef{3, 0}).IsEmpty() {
		t.Fatal("cell at old position should have moved")
	}
	if s.GetCell(CellRef{5, 0}).Raw != "row3" {
		t.Fatalf("expected cell shifted to row 5, got %+v", s.GetCell(CellRef{5, 0}))
	}
}

func TestApplyDeleteRowsRemovesAndShifts(t *testing.T) {
	w := mkWB(t)
	s := w.SheetByID("s1")
	s.SetCell(CellRef{2, 0}, Cell{Raw: "del"})
	s.SetCell(CellRef{5, 0}, Cell{Raw: "shift"})
	if err := w.Apply(Op{Type: OpDeleteRows, Sheet: "s1", Index: 2, Count: 2}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !s.GetCell(CellRef{2, 0}).IsEmpty() {
		t.Fatal("deleted-row cell should be gone")
	}
	if s.GetCell(CellRef{3, 0}).Raw != "shift" {
		t.Fatalf("expected row5 to shift to row3, got %+v", s.GetCell(CellRef{3, 0}))
	}
}

func TestApplyInsertColsShiftsCells(t *testing.T) {
	w := mkWB(t)
	s := w.SheetByID("s1")
	s.SetCell(CellRef{0, 3}, Cell{Raw: "col3"})
	if err := w.Apply(Op{Type: OpInsertCols, Sheet: "s1", Index: 2, Count: 1}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if s.GetCell(CellRef{0, 4}).Raw != "col3" {
		t.Fatalf("expected col shifted to 4, got %+v", s.GetCell(CellRef{0, 4}))
	}
}

func TestApplyUnknownSheet(t *testing.T) {
	w := mkWB(t)
	if err := w.Apply(Op{Type: OpSetCell, Sheet: "nope", Row: 0, Col: 0, Raw: ptr("x")}); err == nil {
		t.Fatal("apply to unknown sheet must error")
	}
}
