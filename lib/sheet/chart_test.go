package sheet

import (
	"encoding/json"
	"testing"
)

func TestChartsApplyMoveSnapshot(t *testing.T) {
	wb := NewWorkbook()
	s := wb.AddSheet("s1", "Sheet1")
	ch := &Chart{Id: "c1", Type: "column", Range: "A1:C10", Title: "Umsatz", Width: 400, Height: 300}
	if err := wb.Apply(Op{Type: OpSetChart, Sheet: "s1", Row: 2, Col: 4, Chart: ch}); err != nil {
		t.Fatal(err)
	}
	if err := wb.Apply(Op{Type: OpInsertRows, Sheet: "s1", Index: 0, Count: 2}); err != nil {
		t.Fatal(err)
	}
	if err := wb.Apply(Op{Type: OpDeleteCols, Sheet: "s1", Index: 1, Count: 1}); err != nil {
		t.Fatal(err)
	}
	got := s.Charts[0]
	if got.Row != 4 || got.Col != 3 || got.Range != "A3:B12" {
		t.Fatalf("chart = %+v", got)
	}
	b, _ := json.Marshal(wb.Snapshot())
	var snap WorkbookSnapshot
	_ = json.Unmarshal(b, &snap)
	if c := WorkbookFromSnapshot(snap).Sheets[0].Charts; len(c) != 1 || c[0] != got {
		t.Fatalf("snapshot charts = %+v", c)
	}
	if err := wb.Apply(Op{Type: OpDeleteRows, Sheet: "s1", Index: 0, Count: 20}); err != nil {
		t.Fatal(err)
	}
	if s.Charts[0].Range != "#REF!" || s.Charts[0].Row != 0 {
		t.Fatalf("after deleting its rows: %+v", s.Charts[0])
	}
	if err := wb.Apply(Op{Type: OpDeleteChart, Sheet: "s1", Chart: &Chart{Id: "c1"}}); err != nil || len(s.Charts) != 0 {
		t.Fatalf("delete: %v %v", err, s.Charts)
	}
}

func TestChartValidateAndTransform(t *testing.T) {
	bad := []Chart{
		{Id: "c 1", Type: "column", Range: "A1:B2", Width: 400, Height: 300},
		{Id: "c1", Type: "radar", Range: "A1:B2", Width: 400, Height: 300},
		{Id: "c1", Type: "pie", Range: "Sheet2!A1:B2", Width: 400, Height: 300},
		{Id: "c1", Type: "pie", Range: "A1:B2", Width: 10, Height: 300},
	}
	for _, c := range bad {
		c := c
		if (Op{Type: OpSetChart, Sheet: "s1", Chart: &c}).Validate() == nil {
			t.Errorf("accepted %+v", c)
		}
	}
	in := Op{Type: OpSetChart, Sheet: "s1", Row: 5, Col: 1, Chart: &Chart{Id: "c1", Type: "line", Range: "A2:B9", Width: 400, Height: 300}}
	got := Transform(in, Op{Type: OpInsertRows, Sheet: "s1", Name: "Sheet1", Index: 3, Count: 2})
	if got.Row != 7 || got.Chart.Range != "A2:B11" || in.Chart.Range != "A2:B9" {
		t.Fatalf("transform: row %d range %q (input %q)", got.Row, got.Chart.Range, in.Chart.Range)
	}
}

func TestChartReviewFixes(t *testing.T) {
	base := Chart{Id: "c1", Type: "column", Width: 400, Height: 300}
	for _, r := range []string{"A0", "A0:B5", "A1:ZZZ9999999", "A1:AZ5"} {
		c := base
		c.Range = r
		if (Op{Type: OpSetChart, Sheet: "s1", Chart: &c}).Validate() == nil {
			t.Errorf("accepted range %q", r)
		}
	}
	c := base
	c.Range = "A1:AY10000" // 51 columns: a category column and 50 series
	if err := (Op{Type: OpSetChart, Sheet: "s1", Chart: &c}).Validate(); err != nil {
		t.Errorf("refused the largest range: %v", err)
	}

	// A concurrent edit of a deleted chart does not bring it back.
	in := Op{Type: OpSetChart, Sheet: "s1", Row: 1, Chart: &c}
	if got := Transform(in, Op{Type: OpDeleteChart, Sheet: "s1", Chart: &Chart{Id: "c1"}}); !got.Noop {
		t.Error("edit of a deleted chart not dropped")
	}

	// Undo of a delete puts the chart back at its drawing position.
	wb := NewWorkbook()
	s := wb.AddSheet("s1", "Sheet1")
	for _, id := range []string{"a", "b", "c"} {
		ch := base
		ch.Id, ch.Range = id, "A1:B2"
		if err := wb.Apply(Op{Type: OpSetChart, Sheet: "s1", Chart: &ch}); err != nil {
			t.Fatal(err)
		}
	}
	b := s.Charts[1]
	_ = wb.Apply(Op{Type: OpDeleteChart, Sheet: "s1", Chart: &Chart{Id: "b"}})
	if err := wb.Apply(Op{Type: OpSetChart, Sheet: "s1", Order: 2, Chart: &b}); err != nil {
		t.Fatal(err)
	}
	if s.Charts[1].Id != "b" {
		t.Errorf("order = %v", s.Charts)
	}
}
