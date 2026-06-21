package db

import "testing"

func TestMemorySheetRoundTrip(t *testing.T) {
	m := NewMemoryDataStore()
	if err := m.SaveSheet("p1", 3, `{"sheets":[]}`); err != nil {
		t.Fatalf("SaveSheet: %v", err)
	}
	got, err := m.GetSheet("p1")
	if err != nil {
		t.Fatalf("GetSheet: %v", err)
	}
	if got.Head != 3 || got.Snapshot != `{"sheets":[]}` {
		t.Fatalf("unexpected sheet: %+v", got)
	}
	ex, _ := m.DoesSheetExist("p1")
	if ex == nil || !*ex {
		t.Fatal("DoesSheetExist should be true")
	}
}

func TestMemorySheetOps(t *testing.T) {
	m := NewMemoryDataStore()
	_ = m.SaveSheet("p1", 0, "{}")
	for r := 1; r <= 3; r++ {
		if err := m.SaveSheetOp("p1", r, `{"type":"setCell"}`, nil, int64(r)); err != nil {
			t.Fatalf("SaveSheetOp: %v", err)
		}
	}
	ops, err := m.GetSheetOps("p1", 2, 3)
	if err != nil {
		t.Fatalf("GetSheetOps: %v", err)
	}
	if len(*ops) != 2 || (*ops)[0].Rev != 2 || (*ops)[1].Rev != 3 {
		t.Fatalf("expected revs 2,3 got %+v", *ops)
	}
}
