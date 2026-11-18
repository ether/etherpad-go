package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
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

func TestCondRulesStrictJSON(t *testing.T) {
	for _, v := range []string{
		`[{"op":"gt","v1":"1","bg":"#fff"}],`,
		`[{"op":"gt","v1":"1","bg":"#fff"}] x`,
		`[{"op":"gt","v1":"1","bg":"#fff"}][]`,
	} {
		if sheet.ParseCondRules(v) != nil {
			t.Errorf("accepted %q", v)
		}
	}
	// Eight rules with long, escaped values still fit.
	long := strings.Repeat(`"ä`, 85) // 255 bytes, every quote escaped in JSON
	var rules []sheet.CondRule
	for range sheet.MaxCondRules {
		rules = append(rules, sheet.CondRule{Op: "between", V1: long, V2: long, Bg: "#ffc7ce", Color: "#9c0006", Bold: "1"})
	}
	b, _ := json.Marshal(rules)
	if sheet.ParseCondRules(string(b)) == nil {
		t.Errorf("eight maximal rules (%d bytes) refused", len(b))
	}
}

func TestImportCondFormatPriority(t *testing.T) {
	f := excelize.NewFile()
	red, _ := f.NewConditionalStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFC7CE"}}})
	green, _ := f.NewConditionalStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"C6EFCE"}}})
	// Written first, so priority 1; its sqref sorts after "A1".
	if err := f.SetConditionalFormat("Sheet1", "A1:A2", []excelize.ConditionalFormatOptions{{Type: "cell", Criteria: ">", Value: "1", Format: &red}}); err != nil {
		t.Fatal(err)
	}
	if err := f.SetConditionalFormat("Sheet1", "A1", []excelize.ConditionalFormatOptions{{Type: "cell", Criteria: ">", Value: "0", Format: &green}}); err != nil {
		t.Fatal(err)
	}
	_ = f.SetCellValue("Sheet1", "A1", 5)
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Import(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	wb := sheet.WorkbookFromSnapshot(snap)
	st, _ := wb.Styles.Get(wb.Sheets[0].GetCell(sheet.CellRef{Row: 0, Col: 0}).StyleId)
	rules := sheet.ParseCondRules(st.Props["cf"])
	if len(rules) != 2 || rules[0].Bg != "#ffc7ce" || rules[1].Bg != "#c6efce" {
		t.Fatalf("rules = %+v", rules)
	}
}

func TestCellsOfSqrefDedupesAndStops(t *testing.T) {
	b := &refBudget{left: 1000}
	got := cellsOfSqref(strings.Repeat("A1:B10 ", 5000), b)
	if len(got) != 20 || b.left != 980 {
		t.Fatalf("got %d cells, budget left %d", len(got), b.left)
	}
	got = cellsOfSqref("A1:CZ200", b)
	if len(got) != 980 || b.left != 0 {
		t.Fatalf("got %d cells over the budget", len(got))
	}
	if got = cellsOfSqref("A1", b); len(got) != 0 {
		t.Fatal("spent budget still expands")
	}
}

func TestImportStyledEmptyCellFarAway(t *testing.T) {
	f := excelize.NewFile()
	_ = f.SetCellValue("Sheet1", "A1", "x")
	bold, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err := f.SetCellStyle("Sheet1", "C900", "C900", bold); err != nil {
		t.Fatal(err)
	}
	if err := f.SetRowHeight("Sheet1", 700, 40); err != nil {
		t.Fatal(err)
	}
	buf, _ := f.WriteToBuffer()
	snap, err := Import(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	wb := sheet.WorkbookFromSnapshot(snap)
	sh := wb.Sheets[0]
	st, _ := wb.Styles.Get(sh.GetCell(sheet.CellRef{Row: 899, Col: 2}).StyleId)
	if st.Props["bold"] != "1" {
		t.Errorf("C900 style = %v", st.Props)
	}
	if sh.RowHeights[699] == 0 {
		t.Errorf("row 700 height lost: %v", sh.RowHeights)
	}
}

func TestFilterRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook()
	s := wb.AddSheet("s1", "Daten")
	rows := []string{"Status", "offen", "bezahlt", "storniert", "offen", `a"&<b`}
	for r, v := range rows {
		s.SetCell(sheet.CellRef{Row: r + 1, Col: 1}, sheet.Cell{Raw: v})
	}
	s.SetCell(sheet.CellRef{Row: 3, Col: 2}, sheet.Cell{Raw: "=1+1"})
	s.Filters = map[int][]string{1: {"offen", `a"&<b`}}
	s.FilterRow = 1

	data, err := Export(wb)
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for row, want := range map[int]bool{3: true, 4: false, 5: false, 6: true, 7: true} {
		if vis, _ := f.GetRowVisible("Daten", row); vis != want {
			t.Errorf("row %d visible = %v, want %v", row, vis, want)
		}
	}
	snap, err := Import(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	got := sheet.WorkbookFromSnapshot(snap).Sheets[0]
	if got.FilterRow != 1 || !reflect.DeepEqual(got.Filters, s.Filters) {
		t.Fatalf("filters = %v row %d", got.Filters, got.FilterRow)
	}
}

func TestFilterExportHideModeAndFormats(t *testing.T) {
	wb := sheet.NewWorkbook()
	s := wb.AddSheet("s1", "Daten")
	pct := wb.Styles.Put(sheet.Style{Props: map[string]string{"numFmt": "percent"}})
	for r, v := range []string{"Status", "offen", "bezahlt", "offen"} {
		s.SetCell(sheet.CellRef{Row: r, Col: 0}, sheet.Cell{Raw: v})
	}
	s.SetCell(sheet.CellRef{Row: 0, Col: 1}, sheet.Cell{Raw: "Anteil"})
	s.SetCell(sheet.CellRef{Row: 1, Col: 1}, sheet.Cell{Raw: "0.25", StyleId: pct}) // shows 25%
	s.Filters = map[int][]string{0: {"Bezahlt"}, 1: {"25%"}}
	s.FilterHide = map[int]bool{0: true}

	data, err := Export(wb)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(data))
	for row, want := range map[int]bool{2: true, 3: false, 4: true} {
		if vis, _ := f.GetRowVisible("Daten", row); vis != want {
			t.Errorf("row %d visible = %v, want %v", row, vis, want)
		}
	}
	snap, _ := Import(bytes.NewReader(data))
	got := sheet.WorkbookFromSnapshot(snap).Sheets[0]
	if !reflect.DeepEqual(got.Filters[0], []string{"offen"}) {
		t.Errorf("hide mode exported as %v", got.Filters[0])
	}
}

func TestChartExport(t *testing.T) {
	wb := sheet.NewWorkbook()
	s := wb.AddSheet("s1", "Daten 1")
	for r, row := range [][]string{{"Monat", "Umsatz", "Kosten"}, {"Jan", "120", "80"}, {"Feb", "=B2*2", "95"}} {
		for c, v := range row {
			s.SetCell(sheet.CellRef{Row: r, Col: c}, sheet.Cell{Raw: v})
		}
	}
	s.Charts = []sheet.Chart{
		{Id: "c1", Type: "column", Range: "A1:C3", Title: "Umsatz", Row: 0, Col: 4, Width: 480, Height: 288},
		{Id: "c2", Type: "pie", Range: "#REF!", Row: 10, Col: 4, Width: 300, Height: 300},
	}
	data, err := Export(wb)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var charts []string
	for _, zf := range zr.File {
		if strings.HasPrefix(zf.Name, "xl/charts/chart") {
			rc, _ := zf.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			charts = append(charts, string(b))
		}
	}
	if len(charts) != 1 {
		t.Fatalf("%d charts written, want 1 (the #REF! one is skipped)", len(charts))
	}
	for _, want := range []string{"<barChart>", `<varyColors val="0">`, "&#39;Daten 1&#39;!$B$2:$B$3", "&#39;Daten 1&#39;!$A$2:$A$3", "&#39;Daten 1&#39;!$C$1", "Umsatz"} {
		if !strings.Contains(charts[0], want) {
			t.Errorf("chart XML lacks %q", want)
		}
	}
}

func TestChartExportLimitsAndTextFormulas(t *testing.T) {
	wb := sheet.NewWorkbook()
	s := wb.AddSheet("s1", "S")
	s.SetCell(sheet.CellRef{Row: 0, Col: 1}, sheet.Cell{Raw: `="Umsatz "&2024`})
	for r := 1; r <= 700; r++ {
		s.SetCell(sheet.CellRef{Row: r, Col: 0}, sheet.Cell{Raw: `=TEXT(` + strconv.Itoa(r) + `,"0")`})
		s.SetCell(sheet.CellRef{Row: r, Col: 1}, sheet.Cell{Raw: strconv.Itoa(r)})
	}
	ch, ok := excelChart(s, "S", sheet.Chart{Id: "c", Type: "line", Range: "A1:B701", Width: 400, Height: 300})
	if !ok || len(ch.Series) != 1 {
		t.Fatalf("chart = %+v", ch)
	}
	sr := ch.Series[0]
	if sr.Name != "S!$B$1" || sr.Categories != "S!$A$2:$A$501" || sr.Values != "S!$B$2:$B$501" {
		t.Errorf("series = %+v", sr)
	}
}
