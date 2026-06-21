package sheet

import (
	"encoding/json"
	"testing"
)

func TestWorkbookSnapshotRoundTrip(t *testing.T) {
	w := NewWorkbook()
	s := w.AddSheet("s1", "Sheet1")
	sid := w.Styles.Put(Style{Props: map[string]string{"bold": "1"}})
	s.SetCell(CellRef{1, 2}, Cell{Raw: "=A1+1", StyleId: sid})
	s.SetCell(CellRef{0, 0}, Cell{Raw: "hi"})

	b, err := json.Marshal(w.Snapshot())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var snap WorkbookSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := WorkbookFromSnapshot(snap)

	if got.SheetByID("s1").GetCell(CellRef{1, 2}).Raw != "=A1+1" {
		t.Fatal("cell raw lost in round-trip")
	}
	if got.SheetByID("s1").GetCell(CellRef{0, 0}).Raw != "hi" {
		t.Fatal("cell lost in round-trip")
	}
	// style pool index must be rebuilt so dedup still works after load
	if got.Styles.Put(Style{Props: map[string]string{"bold": "1"}}) != sid {
		t.Fatal("style pool dedup index not rebuilt after load")
	}
}

func TestWorkbookFromEmptySnapshot(t *testing.T) {
	got := WorkbookFromSnapshot(WorkbookSnapshot{})
	if got.Styles == nil {
		t.Fatal("nil styles must default to a fresh pool")
	}
	if got.Styles.Put(Style{Props: map[string]string{"x": "1"}}) == 0 {
		t.Fatal("fresh pool should assign non-zero ids")
	}
}
