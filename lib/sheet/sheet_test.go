package sheet

import "testing"

func TestSheetSetGetClear(t *testing.T) {
	s := NewSheet("s1", "Sheet1")
	s.SetCell(CellRef{2, 3}, Cell{Raw: "hi"})
	if got := s.GetCell(CellRef{2, 3}); got.Raw != "hi" {
		t.Fatalf("expected hi, got %q", got.Raw)
	}
	// empty cell must not be stored
	s.SetCell(CellRef{2, 3}, Cell{})
	if _, ok := s.Cells[CellRef{2, 3}]; ok {
		t.Fatal("empty cell should be removed from sparse storage")
	}
}

func TestWorkbookCloneIsDeep(t *testing.T) {
	w := NewWorkbook()
	sh := w.AddSheet("s1", "Sheet1")
	sh.SetCell(CellRef{0, 0}, Cell{Raw: "x"})
	clone := w.Clone()
	clone.Sheets[0].SetCell(CellRef{0, 0}, Cell{Raw: "y"})
	if w.Sheets[0].GetCell(CellRef{0, 0}).Raw != "x" {
		t.Fatal("clone must not share cell storage with original")
	}
}

func TestWorkbookSheetByID(t *testing.T) {
	w := NewWorkbook()
	w.AddSheet("s1", "Sheet1")
	if w.SheetByID("s1") == nil {
		t.Fatal("expected to find sheet s1")
	}
	if w.SheetByID("nope") != nil {
		t.Fatal("expected nil for unknown sheet")
	}
}
