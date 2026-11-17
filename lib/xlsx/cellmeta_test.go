package xlsx

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ether/etherpad-go/lib/sheet"
)

func TestCellMetaRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook()
	s := wb.AddSheet("Sheet1", "Sheet1")
	put := func(r, c int, raw string, props map[string]string) {
		if err := sheet.ValidateProps(props); err != nil {
			t.Fatal(err)
		}
		s.SetCell(sheet.CellRef{Row: r, Col: c}, sheet.Cell{Raw: raw, StyleId: wb.Styles.Put(sheet.Style{Props: props})})
	}
	cf := `[{"op":"gt","v1":"10","bg":"#ffc7ce","color":"#9c0006"},{"op":"contains","v1":"x\"y","bold":"1"}]`
	between := `[{"op":"between","v1":"1","v2":"5","bg":"#c6efce"},{"op":"eq","v1":"ja","bg":"#ffeb9c"},{"op":"blank","bold":"1"}]`
	put(0, 0, "Etherpad", map[string]string{"link": "https://etherpad.org/", "bold": "1"})
	put(1, 0, "5", map[string]string{"note": "Zweite Zeile\nmit Umlaut ä"})
	put(2, 0, "rot", map[string]string{"validList": "rot,grün,blau"})
	put(2, 1, "", map[string]string{"validList": "rot,grün,blau"})
	put(3, 0, "20", map[string]string{"cf": cf})
	put(3, 1, "3", map[string]string{"cf": cf})
	put(4, 0, "2", map[string]string{"cf": between})

	data, err := Export(wb)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Import(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	got := sheet.WorkbookFromSnapshot(snap)
	gs := got.Sheets[0]
	propsAt := func(r, c int) map[string]string {
		st, _ := got.Styles.Get(gs.GetCell(sheet.CellRef{Row: r, Col: c}).StyleId)
		return st.Props
	}
	if p := propsAt(0, 0); p["link"] != "https://etherpad.org/" || p["bold"] != "1" {
		t.Errorf("A1 props = %v", p)
	}
	if p := propsAt(1, 0); p["note"] != "Zweite Zeile\nmit Umlaut ä" {
		t.Errorf("A2 note = %q", p["note"])
	}
	for _, c := range []int{0, 1} {
		if p := propsAt(2, c); p["validList"] != "rot,grün,blau" {
			t.Errorf("row 3 col %d list = %q", c, p["validList"])
		}
	}
	for _, at := range [][3]any{{3, 0, cf}, {3, 1, cf}, {4, 0, between}} {
		want := sheet.ParseCondRules(at[2].(string))
		gotRules := sheet.ParseCondRules(propsAt(at[0].(int), at[1].(int))["cf"])
		if !reflect.DeepEqual(gotRules, want) {
			t.Errorf("cf at %v = %+v, want %+v", at[:2], gotRules, want)
		}
	}
}

func TestValidatePropsCellMeta(t *testing.T) {
	bad := []map[string]string{
		{"link": "javascript:alert(1)"},
		{"link": "https://"},
		{"link": "https://a.b/ c"},
		{"note": ""},
		{"cf": `[{"op":"gt","v1":"1"}]`},
		{"cf": `[{"op":"gt","bg":"#fff"}]`},
		{"cf": `[{"op":"drop","v1":"1","bg":"#fff"}]`},
		{"cf": `[{"op":"gt","v1":"1","bg":"url(x)"}]`},
		{"cf": `[{"op":"gt","v1":"1","bg":"#fff","x":1}]`},
	}
	for _, p := range bad {
		if sheet.ValidateProps(p) == nil {
			t.Errorf("accepted %v", p)
		}
	}
	good := []map[string]string{
		{"link": "mailto:a@b.de"},
		{"link": "http://localhost:9001/p/x"},
		{"cf": `[{"op":"notBlank","color":"#00f"}]`},
	}
	for _, p := range good {
		if err := sheet.ValidateProps(p); err != nil {
			t.Errorf("rejected %v: %v", p, err)
		}
	}
}
