package sheetdoc

import (
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/sheet"
)

func strptr(s string) *string { return &s }

func TestManagerSubmitAndPersist(t *testing.T) {
	store := db.NewMemoryDataStore()
	m := NewManager(store)

	rebased, rev, err := m.Submit("p1", sheet.Op{Type: sheet.OpSetCell, Sheet: DefaultSheetID, Row: 0, Col: 0, Raw: strptr("hi"), BaseRev: 0}, nil, 1)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if rev != 1 || rebased.Type != sheet.OpSetCell {
		t.Fatalf("unexpected rev/op: %d %+v", rev, rebased)
	}

	// A fresh manager backed by the same store must reload the persisted state.
	m2 := NewManager(store)
	snap, head, err := m2.Snapshot("p1")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if head != 1 {
		t.Fatalf("reloaded head: got %d", head)
	}
	wb := sheet.WorkbookFromSnapshot(snap)
	if wb.SheetByID(DefaultSheetID).GetCell(sheet.CellRef{Row: 0, Col: 0}).Raw != "hi" {
		t.Fatal("persisted cell not reloaded")
	}
}

func TestManagerOpsSinceForReconnect(t *testing.T) {
	store := db.NewMemoryDataStore()
	m := NewManager(store)
	for i := 0; i < 3; i++ {
		if _, _, err := m.Submit("p1", sheet.Op{Type: sheet.OpInsertRows, Sheet: DefaultSheetID, Index: 0, Count: 1, BaseRev: i}, nil, int64(i)); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
	}
	ops, err := m.OpsSince("p1", 1)
	if err != nil {
		t.Fatalf("OpsSince: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops since rev 1, got %d", len(ops))
	}
}

func TestManagerReloadRebasesStaleOp(t *testing.T) {
	store := db.NewMemoryDataStore()
	m := NewManager(store)
	if _, _, err := m.Submit("p1", sheet.Op{Type: sheet.OpInsertRows, Sheet: DefaultSheetID, Index: 0, Count: 2, BaseRev: 0}, nil, 1); err != nil {
		t.Fatal(err)
	}
	// New manager (simulated restart) must still rebase a stale baseRev-0 op.
	m2 := NewManager(store)
	if _, _, err := m2.Submit("p1", sheet.Op{Type: sheet.OpSetCell, Sheet: DefaultSheetID, Row: 1, Col: 0, Raw: strptr("b"), BaseRev: 0}, nil, 2); err != nil {
		t.Fatal(err)
	}
	snap, _, _ := m2.Snapshot("p1")
	wb := sheet.WorkbookFromSnapshot(snap)
	if wb.SheetByID(DefaultSheetID).GetCell(sheet.CellRef{Row: 3, Col: 0}).Raw != "b" {
		t.Fatalf("stale op not rebased after reload: %+v", wb.SheetByID(DefaultSheetID).Cells)
	}
}
