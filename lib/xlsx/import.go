package xlsx

import (
	"io"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

// Import parses an .xlsx into a WorkbookSnapshot. Sheet id == sheet name. Cells
// carry their raw value, or "=<formula>" when a formula is present. Styles and
// merges are skipped in v1 (no error; the sheet model does not store them yet).
func Import(r io.Reader) (sheet.WorkbookSnapshot, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return sheet.WorkbookSnapshot{}, err
	}
	defer f.Close()

	wb := sheet.NewWorkbook()
	for _, name := range f.GetSheetList() {
		sh := wb.AddSheet(name, name)
		rows, err := f.GetRows(name)
		if err != nil {
			return sheet.WorkbookSnapshot{}, err
		}
		for rIdx, row := range rows {
			for cIdx, val := range row {
				axis, err := excelize.CoordinatesToCellName(cIdx+1, rIdx+1)
				if err != nil {
					return sheet.WorkbookSnapshot{}, err
				}
				raw := val
				if formula, ferr := f.GetCellFormula(name, axis); ferr == nil && formula != "" {
					raw = "=" + formula
				}
				if raw == "" {
					continue
				}
				sh.SetCell(sheet.CellRef{Row: rIdx, Col: cIdx}, sheet.Cell{Raw: raw})
			}
		}
	}
	return wb.Snapshot(), nil
}
