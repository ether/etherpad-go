package xlsx

import (
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

// Export renders a workbook to .xlsx bytes. Cells whose raw starts with '='
// become formulas; numeric-looking raw is written as a number, the rest as a
// string. Styles and merges are out of scope for v1 (the sheet model does not
// store them yet).
func Export(wb *sheet.Workbook) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	const defaultSheet = "Sheet1"

	for i, s := range wb.Sheets {
		name := s.Name
		if name == "" {
			name = s.Id
		}
		if i == 0 {
			if name != defaultSheet {
				if err := f.SetSheetName(defaultSheet, name); err != nil {
					return nil, err
				}
			}
		} else {
			if _, err := f.NewSheet(name); err != nil {
				return nil, err
			}
		}

		for ref, cell := range s.Cells {
			axis, err := excelize.CoordinatesToCellName(ref.Col+1, ref.Row+1)
			if err != nil {
				return nil, err
			}
			if strings.HasPrefix(cell.Raw, "=") {
				if err := f.SetCellFormula(name, axis, cell.Raw[1:]); err != nil {
					return nil, err
				}
				continue
			}
			if n, err := strconv.ParseFloat(cell.Raw, 64); err == nil {
				if err := f.SetCellValue(name, axis, n); err != nil {
					return nil, err
				}
			} else if err := f.SetCellValue(name, axis, cell.Raw); err != nil {
				return nil, err
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
