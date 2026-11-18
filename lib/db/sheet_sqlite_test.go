package db

import "testing"

func TestSQLiteSheetRoundTrip(t *testing.T) {
	store := newTestSQLiteStore(t) // helper from document_type_sqlite_test.go
	// FK requires the pad row to exist first.
	if err := store.CreatePad("p1", dbmodelPadDB("p1", "sheet")); err != nil {
		t.Fatalf("CreatePad: %v", err)
	}
	if err := store.SaveSheet("p1", 5, `{"sheets":[{"id":"s1"}]}`); err != nil {
		t.Fatalf("SaveSheet: %v", err)
	}
	got, err := store.GetSheet("p1")
	if err != nil {
		t.Fatalf("GetSheet: %v", err)
	}
	if got.Head != 5 || got.Snapshot != `{"sheets":[{"id":"s1"}]}` {
		t.Fatalf("unexpected: %+v", got)
	}
	// upsert
	if err := store.SaveSheet("p1", 6, "{}"); err != nil {
		t.Fatalf("SaveSheet upsert: %v", err)
	}
	got2, _ := store.GetSheet("p1")
	if got2.Head != 6 {
		t.Fatalf("upsert head not updated: %+v", got2)
	}

	for r := 1; r <= 3; r++ {
		if err := store.SaveSheetOp("p1", r, `{"type":"setCell"}`, nil, int64(r*10)); err != nil {
			t.Fatalf("SaveSheetOp: %v", err)
		}
	}
	ops, err := store.GetSheetOps("p1", 1, 2)
	if err != nil {
		t.Fatalf("GetSheetOps: %v", err)
	}
	if len(*ops) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(*ops))
	}
}

func TestSQLiteSheetCascadeOnPadDelete(t *testing.T) {
	store := newTestSQLiteStore(t)
	if err := store.CreatePad("p2", dbmodelPadDB("p2", "sheet")); err != nil {
		t.Fatalf("CreatePad: %v", err)
	}
	_ = store.SaveSheet("p2", 1, "{}")
	if err := store.RemovePad("p2"); err != nil {
		t.Fatalf("RemovePad: %v", err)
	}
	ex, _ := store.DoesSheetExist("p2")
	if ex == nil || *ex {
		t.Fatal("sheet should be cascade-deleted with its pad")
	}
}

func TestSQLiteResetSheetKeepsTheBase(t *testing.T) {
	store := newTestSQLiteStore(t)
	if err := store.CreatePad("p1", dbmodelPadDB("p1", "sheet")); err != nil {
		t.Fatalf("CreatePad: %v", err)
	}
	if err := store.SaveSheetOp("p1", 1, `{"type":"setCell"}`, nil, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.ResetSheet("p1", `{"sheets":[{"id":"x"}]}`); err != nil {
		t.Fatalf("ResetSheet: %v", err)
	}
	ops, err := store.GetSheetOps("p1", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(*ops) != 1 || (*ops)[0].Rev != 0 || (*ops)[0].Op != `{"sheets":[{"id":"x"}]}` || (*ops)[0].Timestamp == 0 {
		t.Fatalf("base not kept at rev 0: %+v", *ops)
	}
	v, err := store.GetSheetVersion("p1")
	if err != nil {
		t.Fatal(err)
	}
	if v.Latest() != 0 || v.Epoch != 1 {
		t.Fatalf("version after reset: %+v", v)
	}
	// The next op is rev 1, and a second reset replaces the base.
	if err := store.SaveSheetOp("p1", 1, `{"type":"setCell"}`, nil, 2); err != nil {
		t.Fatal(err)
	}
	if err := store.ResetSheet("p1", `{"sheets":[]}`); err != nil {
		t.Fatal(err)
	}
	ops, _ = store.GetSheetOps("p1", 0, 10)
	if len(*ops) != 1 || (*ops)[0].Op != `{"sheets":[]}` {
		t.Fatalf("after second reset: %+v", *ops)
	}
}
