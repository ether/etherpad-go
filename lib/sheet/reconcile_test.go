package sheet

import "testing"

func newDoc(t *testing.T) *Document {
	t.Helper()
	w := NewWorkbook()
	w.AddSheet("s1", "Sheet1")
	return NewDocument(w)
}

// workbooksEqual compares sparse cell contents of two workbooks (used by
// reconcile and convergence tests).
func workbooksEqual(a, b *Workbook) bool {
	if len(a.Sheets) != len(b.Sheets) {
		return false
	}
	for i := range a.Sheets {
		if a.Sheets[i].Id != b.Sheets[i].Id {
			return false
		}
		if len(a.Sheets[i].Cells) != len(b.Sheets[i].Cells) {
			return false
		}
		for ref, c := range a.Sheets[i].Cells {
			if b.Sheets[i].Cells[ref] != c {
				return false
			}
		}
	}
	return true
}

func TestSubmitAdvancesHead(t *testing.T) {
	d := newDoc(t)
	rev, err := d.Submit(Op{Type: OpSetCell, Sheet: "s1", Row: 0, Col: 0, Raw: ptr("a"), BaseRev: 0})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if rev != 1 || d.Head() != 1 {
		t.Fatalf("expected head 1, got rev=%d head=%d", rev, d.Head())
	}
}

func TestSubmitRebasesStaleCellOp(t *testing.T) {
	d := newDoc(t)
	if _, err := d.Submit(Op{Type: OpInsertRows, Sheet: "s1", Index: 0, Count: 2, BaseRev: 0}); err != nil {
		t.Fatal(err)
	}
	// Client still on base rev 0 sets a cell at row 1; after rebasing past the
	// insert at index 0 it must land at row 3.
	if _, err := d.Submit(Op{Type: OpSetCell, Sheet: "s1", Row: 1, Col: 0, Raw: ptr("b"), BaseRev: 0}); err != nil {
		t.Fatal(err)
	}
	if d.Workbook().SheetByID("s1").GetCell(CellRef{3, 0}).Raw != "b" {
		t.Fatalf("stale cell op was not rebased past the insert: %+v", d.Workbook().SheetByID("s1").Cells)
	}
}

func TestSubmitRejectsBadBaseRev(t *testing.T) {
	d := newDoc(t)
	if _, err := d.Submit(Op{Type: OpSetCell, Sheet: "s1", Row: 0, Col: 0, Raw: ptr("a"), BaseRev: 5}); err == nil {
		t.Fatal("baseRev beyond head must error")
	}
}

func TestConvergenceTwoClients(t *testing.T) {
	d := newDoc(t)
	_, _ = d.Submit(Op{Type: OpSetCell, Sheet: "s1", Row: 0, Col: 0, Raw: ptr("x"), BaseRev: 0})
	_, _ = d.Submit(Op{Type: OpInsertRows, Sheet: "s1", Index: 0, Count: 1, BaseRev: 0})
	_, _ = d.Submit(Op{Type: OpSetCell, Sheet: "s1", Row: 2, Col: 0, Raw: ptr("y"), BaseRev: 1})

	replay := NewWorkbook()
	replay.AddSheet("s1", "Sheet1")
	for _, logged := range d.Log() {
		if err := replay.Apply(logged); err != nil {
			t.Fatalf("replay apply: %v", err)
		}
	}
	if !workbooksEqual(replay, d.Workbook()) {
		t.Fatal("replaying the server op-log diverged from server state")
	}
}
