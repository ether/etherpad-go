package xlsx

import (
	"bytes"
	"testing"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

func TestExportWritesValuesAndFormulas(t *testing.T) {
	wb := sheet.NewWorkbook()
	s := wb.AddSheet("Sheet1", "Sheet1")
	s.SetCell(sheet.CellRef{Row: 0, Col: 0}, sheet.Cell{Raw: "2"})
	s.SetCell(sheet.CellRef{Row: 1, Col: 0}, sheet.Cell{Raw: "3"})
	s.SetCell(sheet.CellRef{Row: 0, Col: 1}, sheet.Cell{Raw: "=SUM(A1:A2)"})

	data, err := Export(wb)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	if v, _ := f.GetCellValue("Sheet1", "A1"); v != "2" {
		t.Fatalf("A1 = %q", v)
	}
	formula, _ := f.GetCellFormula("Sheet1", "B1")
	if formula != "SUM(A1:A2)" {
		t.Fatalf("B1 formula = %q", formula)
	}
}

func TestExportMultipleSheets(t *testing.T) {
	wb := sheet.NewWorkbook()
	wb.AddSheet("First", "First").SetCell(sheet.CellRef{Row: 0, Col: 0}, sheet.Cell{Raw: "a"})
	wb.AddSheet("Second", "Second").SetCell(sheet.CellRef{Row: 0, Col: 0}, sheet.Cell{Raw: "b"})

	data, err := Export(wb)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	names := f.GetSheetList()
	if len(names) != 2 || names[0] != "First" || names[1] != "Second" {
		t.Fatalf("sheet list = %v", names)
	}
}
