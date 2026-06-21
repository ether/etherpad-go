package xlsx

import (
	"bytes"
	"testing"

	"github.com/ether/etherpad-go/lib/sheet"
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
