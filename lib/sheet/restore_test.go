package sheet

import (
	"encoding/json"
	"fmt"
	"testing"
)

// view is what a workbook shows, independent of style ids.
func view(w *Workbook) string {
	type cell struct {
		Raw   string
		Props map[string]string
	}
	type sh struct {
		Id, Name   string
		Cells      map[string]cell
		Merges     map[string]Span
		Cols, Rows map[int]int
		Freeze     [2]int
		Filters    map[int][]string
		FilterRow  int
		Hide       map[int]bool
		Charts     []Chart
	}
	var out []sh
	for _, s := range w.Sheets {
		v := sh{Id: s.Id, Name: s.Name, Cells: map[string]cell{}, Merges: map[string]Span{}, Cols: s.ColWidths, Rows: s.RowHeights,
			Freeze: [2]int{s.FrozenRows, s.FrozenCols}, Filters: s.Filters, FilterRow: s.FilterRow, Hide: s.FilterHide, Charts: s.Charts}
		for r, c := range s.Cells {
			v.Cells[fmt.Sprint(r.Row, ",", r.Col)] = cell{c.Raw, propsOf(w, c.StyleId)}
		}
		for a, sp := range s.Merges {
			v.Merges[fmt.Sprint(a.Row, ",", a.Col)] = sp
		}
		out = append(out, v)
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func TestRestoreOpsReachTheOldVersion(t *testing.T) {
	str := func(s string) *string { return &s }
	doc := NewDocument(func() *Workbook { w := NewWorkbook(); w.AddSheet("s1", "Sheet1"); return w }())
	apply := func(ops ...Op) {
		t.Helper()
		for _, op := range ops {
			op.BaseRev = doc.Head()
			if _, err := doc.Submit(op); err != nil {
				t.Fatalf("%+v: %v", op, err)
			}
		}
	}
	apply(
		Op{Type: OpSetCell, Sheet: "s1", Row: 0, Col: 0, Raw: str("Monat"), Props: map[string]string{"bold": "1"}},
		Op{Type: OpSetCell, Sheet: "s1", Row: 1, Col: 0, Raw: str("Jan")},
		Op{Type: OpSetCell, Sheet: "s1", Row: 1, Col: 1, Raw: str("=A1")},
		Op{Type: OpAddSheet, Sheet: "s2", Name: "Zwei", Index: 1},
		Op{Type: OpSetCell, Sheet: "s2", Row: 3, Col: 3, Raw: str("x")},
		Op{Type: OpMergeCells, Sheet: "s1", Row: 5, Col: 0, EndRow: 5, EndCol: 2},
		Op{Type: OpSetDimension, Sheet: "s1", Axis: "col", Index: 0, Size: 140},
		Op{Type: OpSetFilter, Sheet: "s1", Row: 0, Col: 0, Values: []string{"Jan"}},
		Op{Type: OpSetChart, Sheet: "s1", Row: 2, Col: 4, Chart: &Chart{Id: "c1", Type: "pie", Range: "A1:B2", Width: 300, Height: 200}},
	)
	old := doc.Workbook().Clone()

	apply(
		Op{Type: OpSetCell, Sheet: "s1", Row: 1, Col: 0, Raw: str("Feb"), Props: map[string]string{"italic": "1"}},
		Op{Type: OpSetStyle, Sheet: "s1", Row: 0, Col: 0, Props: map[string]string{}},
		Op{Type: OpSetCell, Sheet: "s1", Row: 9, Col: 9, Raw: str("neu")},
		Op{Type: OpInsertRows, Sheet: "s1", Index: 0, Count: 2},
		Op{Type: OpDeleteSheet, Sheet: "s2"},
		Op{Type: OpAddSheet, Sheet: "s3", Name: "Drei", Index: 0},
		Op{Type: OpRenameSheet, Sheet: "s1", Name: "Daten"},
		Op{Type: OpSetDimension, Sheet: "s1", Axis: "row", Index: 4, Size: 60},
		Op{Type: OpSetFreeze, Sheet: "s1", FrozenRows: 2},
		Op{Type: OpSetFilter, Sheet: "s1", Row: 2, Col: 1, Values: []string{"x"}, Exclude: true},
		Op{Type: OpDeleteChart, Sheet: "s1", Chart: &Chart{Id: "c1"}},
		Op{Type: OpSetChart, Sheet: "s1", Row: 0, Col: 0, Chart: &Chart{Id: "c2", Type: "line", Range: "A1:A3", Width: 300, Height: 200}},
	)

	if view(doc.Workbook()) == view(old) {
		t.Fatal("edits did not change the view")
	}
	apply(RestoreOps(doc.Workbook(), old)...)
	got := doc.Workbook()
	// Row 4 had no own height in the old version: it gets the plain one.
	if got.SheetByID("s1").RowHeights[4] != defaultRowHeight {
		t.Errorf("row 4 height = %d", got.SheetByID("s1").RowHeights[4])
	}
	delete(got.SheetByID("s1").RowHeights, 4)
	if view(got) != view(old) {
		t.Fatalf("restored:\n%s\nwant:\n%s", view(got), view(old))
	}
	if again := RestoreOps(got, old); len(again) != 0 {
		t.Errorf("restoring the same version again yields %d ops", len(again))
	}
}
