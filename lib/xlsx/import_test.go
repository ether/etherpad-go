package xlsx

import (
	"bytes"
	"testing"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

func TestImportExportRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook()
	s := wb.AddSheet("Data", "Data")
	s.SetCell(sheet.CellRef{Row: 0, Col: 0}, sheet.Cell{Raw: "2"})
	s.SetCell(sheet.CellRef{Row: 1, Col: 0}, sheet.Cell{Raw: "3"})
	s.SetCell(sheet.CellRef{Row: 0, Col: 1}, sheet.Cell{Raw: "=SUM(A1:A2)"})
	s.SetCell(sheet.CellRef{Row: 2, Col: 0}, sheet.Cell{Raw: "hello"})

	data, err := Export(wb)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	snap, err := Import(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	got := sheet.WorkbookFromSnapshot(snap)
	sh := got.SheetByID("Data")
	if sh == nil {
		t.Fatal("sheet Data missing after roundtrip")
	}
	if sh.GetCell(sheet.CellRef{Row: 0, Col: 0}).Raw != "2" {
		t.Fatalf("A1 = %q", sh.GetCell(sheet.CellRef{Row: 0, Col: 0}).Raw)
	}
	if sh.GetCell(sheet.CellRef{Row: 2, Col: 0}).Raw != "hello" {
		t.Fatalf("A3 = %q", sh.GetCell(sheet.CellRef{Row: 2, Col: 0}).Raw)
	}
	if sh.GetCell(sheet.CellRef{Row: 0, Col: 1}).Raw != "=SUM(A1:A2)" {
		t.Fatalf("B1 = %q", sh.GetCell(sheet.CellRef{Row: 0, Col: 1}).Raw)
	}
}

func TestRoundTripStylesDimsAndFreeze(t *testing.T) {
	wb := sheet.NewWorkbook()
	s := wb.AddSheet("Data", "Data")
	props := map[string]string{
		"bold": "1", "italic": "1", "underline": "1",
		"color": "#ff0000", "bg": "#00ff00", "align": "center",
		"border": "all", "wrap": "1",
		"fontFamily": "Arial", "fontSize": "14",
		"numFmt": "currency:2",
	}
	styleId := wb.Styles.Put(sheet.Style{Props: props})
	s.SetCell(sheet.CellRef{Row: 0, Col: 0}, sheet.Cell{Raw: "42", StyleId: styleId})
	// styled but empty cell must survive too
	s.SetCell(sheet.CellRef{Row: 1, Col: 1}, sheet.Cell{StyleId: styleId})
	s.ColWidths[2] = 150
	s.RowHeights[3] = 40
	s.FrozenRows, s.FrozenCols = 1, 1

	data, err := Export(wb)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	snap, err := Import(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	got := sheet.WorkbookFromSnapshot(snap)
	sh := got.SheetByID("Data")
	if sh == nil {
		t.Fatal("sheet Data missing")
	}

	for _, ref := range []sheet.CellRef{{Row: 0, Col: 0}, {Row: 1, Col: 1}} {
		c := sh.GetCell(ref)
		if c.StyleId == 0 {
			t.Fatalf("cell %v lost its style", ref)
		}
		st, _ := got.Styles.Get(c.StyleId)
		for k, want := range props {
			if st.Props[k] != want {
				t.Errorf("cell %v prop %s = %q, want %q", ref, k, st.Props[k], want)
			}
		}
	}
	if px := sh.ColWidths[2]; px < 148 || px > 152 { // unit conversion rounds
		t.Errorf("col 2 width = %d, want ~150", px)
	}
	if px := sh.RowHeights[3]; px < 38 || px > 42 {
		t.Errorf("row 3 height = %d, want ~40", px)
	}
	if sh.FrozenRows != 1 || sh.FrozenCols != 1 {
		t.Errorf("freeze = %d/%d, want 1/1", sh.FrozenRows, sh.FrozenCols)
	}
	if len(sh.ColWidths) != 1 || len(sh.RowHeights) != 1 {
		t.Errorf("dims not sparse: cols=%v rows=%v", sh.ColWidths, sh.RowHeights)
	}
}

// TestImportForeignFile builds a file the way Excel would (builtin numFmt ids,
// style on a value cell) rather than via our own Export, so mapping gaps can't
// hide behind a symmetric round-trip.
func TestImportForeignFile(t *testing.T) {
	f := excelize.NewFile()
	styleId, err := f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true, Color: "FF0000"},
		NumFmt: 10, // builtin "0.00%"
	})
	if err != nil {
		t.Fatalf("NewStyle: %v", err)
	}
	if err := f.SetCellValue("Sheet1", "A1", 0.5); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	if err := f.SetCellStyle("Sheet1", "A1", "A1", styleId); err != nil {
		t.Fatalf("SetCellStyle: %v", err)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}

	snap, err := Import(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	wb := sheet.WorkbookFromSnapshot(snap)
	c := wb.SheetByID("Sheet1").GetCell(sheet.CellRef{Row: 0, Col: 0})
	if c.StyleId == 0 {
		t.Fatal("A1 has no style")
	}
	st, _ := wb.Styles.Get(c.StyleId)
	if st.Props["bold"] != "1" || st.Props["color"] != "#ff0000" || st.Props["numFmt"] != "percent:2" {
		t.Fatalf("props = %v", st.Props)
	}
}

func TestNumFmtCodeMapping(t *testing.T) {
	cases := map[string]string{
		"@": "text", "m/d/yyyy": "date", "0.00%": "percent:2",
		"$#,##0.00": "currency:2", "#,##0": "number:0", "General": "",
	}
	for code, want := range cases {
		if got := codeToNumFmt(code); got != want {
			t.Errorf("codeToNumFmt(%q) = %q, want %q", code, got, want)
		}
	}
	// symbolic -> code -> symbolic is stable
	for _, nf := range []string{"text", "date", "number:2", "currency:1", "percent:0"} {
		if got := codeToNumFmt(numFmtToCode(nf)); got != nf {
			t.Errorf("roundtrip %q -> %q", nf, got)
		}
	}
}
