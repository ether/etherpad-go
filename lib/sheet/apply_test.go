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
	// No-op, not an error: after a deleteSheet, late ops targeting the gone
	// sheet must not poison the ordered-log replay (M4 convergence rule).
	w := mkWB(t)
	if err := w.Apply(Op{Type: OpSetCell, Sheet: "nope", Row: 0, Col: 0, Raw: ptr("x")}); err != nil {
		t.Fatalf("apply to unknown sheet must be a silent no-op, got %v", err)
	}
}

func TestApplySetStyleInternsProps(t *testing.T) {
	w := NewWorkbook()
	w.AddSheet("s1", "Sheet1")

	props := map[string]string{"bold": "1", "color": "#cc0000"}
	if err := w.Apply(Op{Type: OpSetStyle, Sheet: "s1", Row: 1, Col: 2, Props: props}); err != nil {
		t.Fatal(err)
	}
	cell := w.SheetByID("s1").GetCell(CellRef{1, 2})
	if cell.StyleId == 0 {
		t.Fatalf("expected non-zero styleId after interning props")
	}
	got, ok := w.Styles.Get(cell.StyleId)
	if !ok || got.Props["bold"] != "1" || got.Props["color"] != "#cc0000" {
		t.Fatalf("pool did not store props: %+v ok=%v", got, ok)
	}

	// Dedup: identical props reused on another cell -> same id.
	if err := w.Apply(Op{Type: OpSetStyle, Sheet: "s1", Row: 3, Col: 4, Props: props}); err != nil {
		t.Fatal(err)
	}
	if w.SheetByID("s1").GetCell(CellRef{3, 4}).StyleId != cell.StyleId {
		t.Fatalf("identical props should dedup to the same id")
	}

	// Different props -> different id.
	if err := w.Apply(Op{Type: OpSetStyle, Sheet: "s1", Row: 5, Col: 6, Props: map[string]string{"italic": "1"}}); err != nil {
		t.Fatal(err)
	}
	if w.SheetByID("s1").GetCell(CellRef{5, 6}).StyleId == cell.StyleId {
		t.Fatalf("different props must not share an id")
	}
}

func TestApplySetCellWithPropsInterns(t *testing.T) {
	w := NewWorkbook()
	w.AddSheet("s1", "Sheet1")
	raw := "42"
	if err := w.Apply(Op{Type: OpSetCell, Sheet: "s1", Row: 0, Col: 0, Raw: &raw, Props: map[string]string{"align": "right"}}); err != nil {
		t.Fatal(err)
	}
	cell := w.SheetByID("s1").GetCell(CellRef{0, 0})
	if cell.Raw != "42" || cell.StyleId == 0 {
		t.Fatalf("setCell should set raw AND intern props: %+v", cell)
	}
}
