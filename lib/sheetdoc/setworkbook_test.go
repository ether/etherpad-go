package sheetdoc

import (
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/sheet"
)

func TestSetWorkbookReplacesStateAndClearsOps(t *testing.T) {
	store := db.NewMemoryDataStore()
	m := NewManager(store)

	// Establish a document with one op (head 1).
	if _, _, err := m.Submit("p1", sheet.Op{Type: sheet.OpSetCell, Sheet: DefaultSheetID, Row: 0, Col: 0, Raw: strptr("old"), BaseRev: 0}, nil, 1); err != nil {
		t.Fatal(err)
	}

	// Import a fresh workbook.
	wb := sheet.NewWorkbook()
	wb.AddSheet("Imported", "Imported").SetCell(sheet.CellRef{Row: 0, Col: 0}, sheet.Cell{Raw: "new"})
	if err := m.SetWorkbook("p1", wb); err != nil {
		t.Fatalf("SetWorkbook: %v", err)
	}

	snap, head, err := m.Snapshot("p1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if head != 0 {
		t.Fatalf("expected head 0 after import, got %d", head)
	}
	got := sheet.WorkbookFromSnapshot(snap)
	if got.SheetByID("Imported") == nil || got.SheetByID("Imported").GetCell(sheet.CellRef{Row: 0, Col: 0}).Raw != "new" {
		t.Fatal("imported workbook not set")
	}

	// A subsequent op must succeed at rev 1 (proves prior ops were cleared so
	// the write-once sheet_op PK does not block it).
	_, rev, err := m.Submit("p1", sheet.Op{Type: sheet.OpSetCell, Sheet: "Imported", Row: 1, Col: 0, Raw: strptr("more"), BaseRev: 0}, nil, 2)
	if err != nil {
		t.Fatalf("Submit after import: %v", err)
	}
	if rev != 1 {
		t.Fatalf("expected rev 1 after import, got %d", rev)
	}

	// Reload from the store confirms persistence with exactly one op.
	m2 := NewManager(store)
	ops, err := m2.OpsSince("p1", 0)
	if err != nil {
		t.Fatalf("OpsSince: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("expected 1 persisted op after import+edit, got %d", len(ops))
	}
}
