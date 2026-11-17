package sheet

import (
	"encoding/json"
	"os"
	"testing"
)

type refCase struct {
	Name string `json:"name"`
	Raw  string `json:"raw"`
	Edit struct {
		Target   string `json:"target"`
		OnTarget bool   `json:"onTarget"`
		Axis     string `json:"axis"`
		Index    int    `json:"index"`
		Delta    int    `json:"delta"`
		Rename   string `json:"rename"`
		Deleted  bool   `json:"deleted"`
	} `json:"edit"`
	Want string `json:"want"`
}

// The cases are shared with ui/src/js/sheet/formulaRefs.test.ts, so server
// and browser rewrite formulas the same way.
func TestRewriteFormula(t *testing.T) {
	data, err := os.ReadFile("testdata/formularefs.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []refCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		e := RefEdit{Target: c.Edit.Target, OnTarget: c.Edit.OnTarget, Axis: c.Edit.Axis,
			Index: c.Edit.Index, Delta: c.Edit.Delta, Rename: c.Edit.Rename, Deleted: c.Edit.Deleted}
		if got := RewriteFormula(c.Raw, e); got != c.Want {
			t.Errorf("%s: RewriteFormula(%q) = %q, want %q", c.Name, c.Raw, got, c.Want)
		}
	}
}

func TestApplyMovesFormulaRefs(t *testing.T) {
	wb := NewWorkbook()
	s1 := wb.AddSheet("s1", "Sheet1")
	s2 := wb.AddSheet("s2", "Sheet2")
	s1.SetCell(CellRef{0, 0}, Cell{Raw: "5"})
	s1.SetCell(CellRef{1, 0}, Cell{Raw: "=A1*2", Value: "10", ValueType: "number"})
	s1.SetCell(CellRef{0, 1}, Cell{Raw: "=A2+1"})
	s2.SetCell(CellRef{0, 0}, Cell{Raw: "=Sheet1!A2+A2"})

	if err := wb.Apply(Op{Type: OpInsertRows, Sheet: "s1", Index: 0, Count: 1}); err != nil {
		t.Fatal(err)
	}
	if got := s1.GetCell(CellRef{2, 0}); got.Raw != "=A2*2" || got.Value != "" {
		t.Errorf("A3 = %+v, want =A2*2 without the stale value", got)
	}
	if got := s1.GetCell(CellRef{1, 1}).Raw; got != "=A3+1" {
		t.Errorf("B2 = %q, want =A3+1", got)
	}
	if got := s2.GetCell(CellRef{0, 0}).Raw; got != "=Sheet1!A3+A2" {
		t.Errorf("Sheet2!A1 = %q, want =Sheet1!A3+A2", got)
	}

	if err := wb.Apply(Op{Type: OpRenameSheet, Sheet: "s1", Name: "Daten 1"}); err != nil {
		t.Fatal(err)
	}
	if got := s2.GetCell(CellRef{0, 0}).Raw; got != "='Daten 1'!A3+A2" {
		t.Errorf("after rename = %q", got)
	}

	if err := wb.Apply(Op{Type: OpDeleteSheet, Sheet: "s1"}); err != nil {
		t.Fatal(err)
	}
	if got := s2.GetCell(CellRef{0, 0}).Raw; got != "=#REF!+A2" {
		t.Errorf("after delete = %q", got)
	}
}

func TestTransformMovesConcurrentFormula(t *testing.T) {
	raw := "=A5+Sheet1!A5"
	in := Op{Type: OpSetCell, Sheet: "s1", Row: 7, Col: 1, Raw: &raw}
	got := Transform(in, Op{Type: OpInsertRows, Sheet: "s1", Index: 2, Count: 1})
	if got.Row != 8 || *got.Raw != "=A6+Sheet1!A5" {
		t.Errorf("got row %d raw %q", got.Row, *got.Raw)
	}
	if raw != "=A5+Sheet1!A5" {
		t.Error("Transform changed the caller's string")
	}
}

func TestTransformMovesFormulasNamingTheSheet(t *testing.T) {
	str := func(s string) *string { return &s }
	// A formula on another sheet, typed before a row insert on Sheet1.
	in := Op{Type: OpSetCell, Sheet: "s2", Row: 0, Col: 0, Raw: str("=Sheet1!A5+A5")}
	got := Transform(in, Op{Type: OpInsertRows, Sheet: "s1", Name: "Sheet1", Index: 2, Count: 1})
	if *got.Raw != "=Sheet1!A6+A5" || got.Row != 0 {
		t.Errorf("other sheet: %+v %q", got, *got.Raw)
	}
	// On the same sheet, both kinds move.
	in = Op{Type: OpSetCell, Sheet: "s1", Row: 7, Col: 0, Raw: str("=Sheet1!A5+A5")}
	if got = Transform(in, Op{Type: OpInsertRows, Sheet: "s1", Name: "Sheet1", Index: 2, Count: 1}); *got.Raw != "=Sheet1!A6+A6" || got.Row != 8 {
		t.Errorf("same sheet: row %d %q", got.Row, *got.Raw)
	}
	in = Op{Type: OpSetCell, Sheet: "s2", Raw: str("=Sheet1!A1*2")}
	if got = Transform(in, Op{Type: OpRenameSheet, Sheet: "s1", OldName: "Sheet1", Name: "Daten 1"}); *got.Raw != "='Daten 1'!A1*2" {
		t.Errorf("rename: %q", *got.Raw)
	}
	if got = Transform(in, Op{Type: OpDeleteSheet, Sheet: "s1", Name: "Sheet1"}); *got.Raw != "=#REF!*2" {
		t.Errorf("delete: %q", *got.Raw)
	}
}

func TestSubmitFillsSheetNames(t *testing.T) {
	wb := NewWorkbook()
	wb.AddSheet("s1", "Sheet1")
	wb.AddSheet("s2", "Sheet2")
	d := NewDocument(wb)
	str := func(s string) *string { return &s }
	// Both composed at revision 0: the formula arrives after the insert.
	if _, err := d.Submit(Op{Type: OpInsertRows, Sheet: "s1", Index: 0, Count: 1, Name: "client lies"}); err != nil {
		t.Fatal(err)
	}
	if d.Log()[0].Name != "Sheet1" {
		t.Errorf("logged name = %q", d.Log()[0].Name)
	}
	if _, err := d.Submit(Op{Type: OpSetCell, Sheet: "s2", Raw: str("=Sheet1!A1")}); err != nil {
		t.Fatal(err)
	}
	if got := wb.SheetByID("s2").GetCell(CellRef{0, 0}).Raw; got != "=Sheet1!A2" {
		t.Errorf("stale formula = %q", got)
	}
	if _, err := d.Submit(Op{Type: OpRenameSheet, Sheet: "s1", Name: "Neu", BaseRev: 2}); err != nil {
		t.Fatal(err)
	}
	if d.Log()[2].OldName != "Sheet1" {
		t.Errorf("old name = %q", d.Log()[2].OldName)
	}
}
