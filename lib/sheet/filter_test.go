package sheet

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFilterApplyShiftAndSnapshot(t *testing.T) {
	wb := NewWorkbook()
	s := wb.AddSheet("s1", "Sheet1")
	must := func(op Op) {
		t.Helper()
		op.Sheet = "s1"
		if err := wb.Apply(op); err != nil {
			t.Fatal(err)
		}
	}
	must(Op{Type: OpSetFilter, Row: 2, Col: 1, Values: []string{"x"}})
	must(Op{Type: OpSetFilter, Row: 2, Col: 3, Values: []string{"a", "b"}})
	if s.FilterRow != 2 || !reflect.DeepEqual(s.Filters, map[int][]string{1: {"x"}, 3: {"a", "b"}}) {
		t.Fatalf("filters = %v row %d", s.Filters, s.FilterRow)
	}
	must(Op{Type: OpInsertCols, Index: 0, Count: 1})
	must(Op{Type: OpInsertRows, Index: 0, Count: 2})
	if s.FilterRow != 4 || !reflect.DeepEqual(s.Filters, map[int][]string{2: {"x"}, 4: {"a", "b"}}) {
		t.Fatalf("after inserts: %v row %d", s.Filters, s.FilterRow)
	}
	must(Op{Type: OpDeleteCols, Index: 2, Count: 1})
	if !reflect.DeepEqual(s.Filters, map[int][]string{3: {"a", "b"}}) {
		t.Fatalf("after delete: %v", s.Filters)
	}

	b, err := json.Marshal(wb.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	var snap WorkbookSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		t.Fatal(err)
	}
	got := WorkbookFromSnapshot(snap).Sheets[0]
	if got.FilterRow != 4 || !reflect.DeepEqual(got.Filters, s.Filters) {
		t.Fatalf("snapshot: %v row %d", got.Filters, got.FilterRow)
	}

	must(Op{Type: OpSetFilter, Col: 3})
	if s.Filters != nil {
		t.Fatalf("removing the last filter left %v", s.Filters)
	}
	must(Op{Type: OpSetFilter, Row: 0, Col: 0, Values: []string{"q"}})
	must(Op{Type: OpSetFilter, Clear: true})
	if s.Filters != nil {
		t.Fatal("clear left filters")
	}
	if err := (Op{Type: OpSetFilter, Sheet: "s1", Col: -1}).Validate(); err == nil {
		t.Fatal("negative column accepted")
	}
}

func TestFilterOnConcurrentlyDeletedColumnIsDropped(t *testing.T) {
	got := Transform(Op{Type: OpSetFilter, Sheet: "s1", Col: 2, Values: []string{"x"}},
		Op{Type: OpDeleteCols, Sheet: "s1", Index: 2, Count: 1})
	if !got.Noop {
		t.Fatalf("got %+v", got)
	}
	wb := NewWorkbook()
	s := wb.AddSheet("s1", "Sheet1")
	s.Filters = map[int][]string{2: {"keep"}}
	if err := wb.Apply(got); err != nil || len(s.Filters) != 1 || s.Filters[2][0] != "keep" {
		t.Fatalf("noop applied: %v %v", err, s.Filters)
	}
	// A filter next to the deleted column just moves.
	if got := Transform(Op{Type: OpSetFilter, Sheet: "s1", Col: 3, Values: []string{"x"}},
		Op{Type: OpDeleteCols, Sheet: "s1", Index: 2, Count: 1}); got.Noop || got.Col != 2 {
		t.Fatalf("neighbour: %+v", got)
	}
}

func TestFilterKeysAndHideMode(t *testing.T) {
	for _, c := range [][2]string{{"Offen", " offen "}, {"1,234.5", "1234.50"}, {"10", "1e1"}} {
		if FilterKey(c[0]) != FilterKey(c[1]) {
			t.Errorf("%q and %q differ", c[0], c[1])
		}
	}
	if FilterKey("1,5") == FilterKey("15") {
		t.Error("a German decimal comma read as a thousands separator")
	}
	wb := NewWorkbook()
	s := wb.AddSheet("s1", "Sheet1")
	if err := wb.Apply(Op{Type: OpSetFilter, Sheet: "s1", Col: 1, Values: []string{"y"}, Exclude: true}); err != nil {
		t.Fatal(err)
	}
	if s.FilterKeeps(1, "Y") || !s.FilterKeeps(1, "x") || !s.FilterKeeps(1, "") {
		t.Error("hide mode keeps the wrong rows")
	}
	b, _ := json.Marshal(wb.Snapshot())
	var snap WorkbookSnapshot
	_ = json.Unmarshal(b, &snap)
	if got := WorkbookFromSnapshot(snap).Sheets[0]; !got.FilterHide[1] {
		t.Error("hide mode lost in the snapshot")
	}
	if err := wb.Apply(Op{Type: OpSetFilter, Sheet: "s1", Col: 1, Values: []string{"x"}}); err != nil || s.FilterHide[1] {
		t.Error("a keep list did not end hide mode")
	}
}
