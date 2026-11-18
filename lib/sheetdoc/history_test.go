package sheetdoc

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/sheet"
)

func set(t *testing.T, m *Manager, pad, sh string, row, col int, raw string, author string, ts int64) {
	t.Helper()
	_, head, err := m.State(pad)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Submit(pad, sheet.Op{Type: sheet.OpSetCell, Sheet: sh, Row: row, Col: col, Raw: strptr(raw), BaseRev: head}, &author, ts); err != nil {
		t.Fatal(err)
	}
}

func raw(t *testing.T, wb *sheet.Workbook, sh string, row, col int) string {
	t.Helper()
	return wb.SheetByID(sh).GetCell(sheet.CellRef{Row: row, Col: col}).Raw
}

func TestHistoryOfAFreshSheet(t *testing.T) {
	m := NewManager(db.NewMemoryDataStore())
	set(t, m, "p", DefaultSheetID, 0, 0, "a", "a.1", 100)
	set(t, m, "p", DefaultSheetID, 0, 0, "b", "a.2", 200)
	set(t, m, "p", DefaultSheetID, 1, 0, "c", "a.1", 300)

	revs, head, ok, err := m.History("p")
	if err != nil || !ok || head != 3 || len(revs) != 3 {
		t.Fatalf("History = %v %d %v %v", revs, head, ok, err)
	}
	if revs[1] != (Revision{Rev: 2, Timestamp: 200, Author: "a.2"}) {
		t.Errorf("rev 2 = %+v", revs[1])
	}
	for rev, want := range []string{"", "a", "b", "b"} {
		wb, err := m.WorkbookAt("p", rev)
		if err != nil {
			t.Fatal(err)
		}
		if got := raw(t, wb, DefaultSheetID, 0, 0); got != want {
			t.Errorf("A1 at %d = %q, want %q", rev, got, want)
		}
	}
	if _, err := m.WorkbookAt("p", 4); err == nil {
		t.Error("revision after the head")
	}
}

func TestHistoryWithoutStoredBase(t *testing.T) {
	// A sheet created before the base was kept: epoch 0 starts from the
	// fresh workbook.
	store := db.NewMemoryDataStore()
	m := NewManager(store)
	set(t, m, "p", DefaultSheetID, 0, 0, "a", "a.1", 1)
	store.RemoveSheetOps("p")
	_ = store.SaveSheetOp("p", 1, `{"type":"setCell","sheet":"s1","row":0,"col":0,"raw":"a"}`, nil, 1)
	wb, err := NewManager(store).WorkbookAt("p", 0)
	if err != nil || raw(t, wb, DefaultSheetID, 0, 0) != "" {
		t.Fatalf("fresh base: %v", err)
	}

	// After an import without a kept base the history is unknown.
	if err := m.SetWorkbook("p", sheet.NewWorkbook()); err != nil {
		t.Fatal(err)
	}
	_ = store.RemoveSheetOps("p")
	if _, _, ok, _ := m.History("p"); ok {
		t.Error("history available without base")
	}
	if _, err := m.WorkbookAt("p", 0); !errors.Is(err, ErrNoHistory) {
		t.Errorf("WorkbookAt = %v", err)
	}
}

func TestHistoryStartsAtTheImport(t *testing.T) {
	m := NewManager(db.NewMemoryDataStore())
	set(t, m, "p", DefaultSheetID, 0, 0, "before", "a.1", 1)
	wb := sheet.NewWorkbook()
	wb.AddSheet("imp", "Import").SetCell(sheet.CellRef{Row: 2, Col: 2}, sheet.Cell{Raw: "imported"})
	if err := m.SetWorkbook("p", wb); err != nil {
		t.Fatal(err)
	}
	set(t, m, "p", "imp", 2, 2, "edited", "a.1", 2)

	revs, head, ok, err := m.History("p")
	if err != nil || !ok || head != 1 || len(revs) != 1 {
		t.Fatalf("History = %v %d %v %v", revs, head, ok, err)
	}
	at0, err := m.WorkbookAt("p", 0)
	if err != nil || raw(t, at0, "imp", 2, 2) != "imported" {
		t.Fatalf("base = %v", err)
	}
}

func TestRestoreStoresOpsAndKeepsHistory(t *testing.T) {
	store := db.NewMemoryDataStore()
	m := NewManager(store)
	set(t, m, "p", DefaultSheetID, 0, 0, "one", "a.1", 1)
	set(t, m, "p", DefaultSheetID, 0, 1, "two", "a.1", 2)
	set(t, m, "p", DefaultSheetID, 0, 0, "three", "a.2", 3)

	n, err := m.Restore("p", 1, strptr("a.3"), 4)
	if err != nil || n == 0 {
		t.Fatalf("Restore = %d, %v", n, err)
	}
	snap, head, _ := m.Snapshot("p")
	cur := sheet.WorkbookFromSnapshot(snap)
	if raw(t, cur, DefaultSheetID, 0, 0) != "one" || raw(t, cur, DefaultSheetID, 0, 1) != "" {
		t.Fatalf("restored A1=%q B1=%q", raw(t, cur, DefaultSheetID, 0, 0), raw(t, cur, DefaultSheetID, 0, 1))
	}
	if head != 3+n {
		t.Errorf("head %d after %d restore ops", head, n)
	}
	// Persisted: a second instance loads the same state, and the old
	// versions are still there.
	m2 := NewManager(store)
	snap2, _, _ := m2.Snapshot("p")
	if raw(t, sheet.WorkbookFromSnapshot(snap2), DefaultSheetID, 0, 0) != "one" {
		t.Error("restore not stored")
	}
	at3, err := m2.WorkbookAt("p", 3)
	if err != nil || raw(t, at3, DefaultSheetID, 0, 0) != "three" {
		t.Errorf("rev 3 after restore: %v", err)
	}
	revs, _, _, _ := m2.History("p")
	if last := revs[len(revs)-1]; last.Author != "a.3" || last.Timestamp != 4 {
		t.Errorf("restore op author = %+v", last)
	}
	// Restoring the current state stores nothing.
	if n, err := m.Restore("p", head, nil, 5); err != nil || n != 0 {
		t.Errorf("no-op restore = %d, %v", n, err)
	}
}

func TestHistoryOfAnOldImportIsUnavailable(t *testing.T) {
	// Imported before the epochs: epoch 0, no base row, and the ops do not
	// lead from the fresh workbook to the stored state.
	store := db.NewMemoryDataStore()
	wb := sheet.NewWorkbook()
	wb.AddSheet(DefaultSheetID, "Sheet1").SetCell(sheet.CellRef{Row: 5, Col: 5}, sheet.Cell{Raw: "imported"})
	snap, _ := json.Marshal(sheet.NewDocument(wb).Workbook().Snapshot())
	if err := store.SaveSheet("p", 0, string(snap)); err != nil {
		t.Fatal(err)
	}
	m := NewManager(store)
	set(t, m, "p", DefaultSheetID, 0, 0, "edit", "a.1", 1)
	if _, _, ok, err := m.History("p"); err != nil || ok {
		t.Fatalf("History available = %v, %v", ok, err)
	}
	if _, err := m.WorkbookAt("p", 0); !errors.Is(err, ErrNoHistory) {
		t.Errorf("WorkbookAt = %v", err)
	}
	if _, err := m.Restore("p", 0, nil, 2); !errors.Is(err, ErrNoHistory) {
		t.Errorf("Restore = %v", err)
	}
	snap2, _, _ := m.Snapshot("p")
	if raw(t, sheet.WorkbookFromSnapshot(snap2), DefaultSheetID, 5, 5) != "imported" {
		t.Error("the import was touched")
	}
}

func TestRestoreRefusesAVersionOfAnEarlierImport(t *testing.T) {
	m := NewManager(db.NewMemoryDataStore())
	set(t, m, "p", DefaultSheetID, 0, 0, "old", "a.1", 1)
	epoch, _, _ := m.State("p")
	wb := sheet.NewWorkbook()
	wb.AddSheet("imp", "Import")
	if err := m.SetWorkbook("p", wb); err != nil {
		t.Fatal(err)
	}
	// A restore that read the epoch before the import must not apply.
	if _, err := m.workbookAt("p", epoch, 1); !errors.Is(err, ErrHistoryChanged) {
		t.Errorf("workbookAt old epoch = %v", err)
	}
}

func TestHistoryCheckpoints(t *testing.T) {
	m := NewManager(db.NewMemoryDataStore())
	const n = checkpointEvery*2 + 37
	for i := 1; i <= n; i++ {
		set(t, m, "p", DefaultSheetID, i%7, 0, fmt.Sprint(i), "a.1", int64(i))
	}
	for _, rev := range []int{n, checkpointEvery - 1, checkpointEvery, checkpointEvery*2 + 1, 3, n} {
		wb, err := m.WorkbookAt("p", rev)
		if err != nil {
			t.Fatal(err)
		}
		if got := raw(t, wb, DefaultSheetID, rev%7, 0); got != fmt.Sprint(rev) {
			t.Errorf("at %d: %q", rev, got)
		}
		// The caller owns the result.
		wb.SheetByID(DefaultSheetID).SetCell(sheet.CellRef{Row: 0, Col: 0}, sheet.Cell{Raw: "scribble"})
	}
	m.history.mu.Lock()
	points := len(m.history.sheets["p"].points)
	m.history.mu.Unlock()
	if points != 3 {
		t.Errorf("%d checkpoints, want base + 2", points)
	}
}
