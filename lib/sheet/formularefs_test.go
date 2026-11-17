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
