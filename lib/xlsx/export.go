package xlsx

import (
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

// Export renders a workbook to .xlsx bytes. Cells whose raw starts with '='
// become formulas; numeric-looking raw is written as a number, the rest as a
// string. Cell styles, column widths / row heights, merged ranges and freeze
// panes are carried over.
func Export(wb *sheet.Workbook) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	const defaultSheet = "Sheet1"

	// Lazily translate pool style ids to excelize style ids (shared per file).
	styleIds := map[int]int{}
	styleFor := func(id int) (int, error) {
		if xid, ok := styleIds[id]; ok {
			return xid, nil
		}
		st, ok := wb.Styles.Get(id)
		if !ok || len(st.Props) == 0 {
			styleIds[id] = 0
			return 0, nil
		}
		xid, err := f.NewStyle(propsToStyle(st.Props))
		if err != nil {
			return 0, err
		}
		styleIds[id] = xid
		return xid, nil
	}

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
			} else if n, err := strconv.ParseFloat(cell.Raw, 64); err == nil && cell.Raw != "" {
				if err := f.SetCellValue(name, axis, n); err != nil {
					return nil, err
				}
			} else if cell.Raw != "" {
				if err := f.SetCellValue(name, axis, cell.Raw); err != nil {
					return nil, err
				}
			}
			if cell.StyleId != 0 {
				xid, err := styleFor(cell.StyleId)
				if err != nil {
					return nil, err
				}
				if xid != 0 {
					if err := f.SetCellStyle(name, axis, axis, xid); err != nil {
						return nil, err
					}
				}
			}
		}

		for col, px := range s.ColWidths {
			cn, err := excelize.ColumnNumberToName(col + 1)
			if err != nil {
				return nil, err
			}
			if err := f.SetColWidth(name, cn, cn, pxToColWidth(px)); err != nil {
				return nil, err
			}
		}
		for row, px := range s.RowHeights {
			if err := f.SetRowHeight(name, row+1, pxToRowHeight(px)); err != nil {
				return nil, err
			}
		}

		for a, sp := range s.Merges {
			start, err := excelize.CoordinatesToCellName(a.Col+1, a.Row+1)
			if err != nil {
				return nil, err
			}
			end, err := excelize.CoordinatesToCellName(a.Col+sp.Cols, a.Row+sp.Rows)
			if err != nil {
				return nil, err
			}
			if err := f.MergeCell(name, start, end); err != nil {
				return nil, err
			}
		}

		if s.FrozenRows > 0 || s.FrozenCols > 0 {
			topLeft, err := excelize.CoordinatesToCellName(s.FrozenCols+1, s.FrozenRows+1)
			if err != nil {
				return nil, err
			}
			if err := f.SetPanes(name, &excelize.Panes{
				Freeze: true, XSplit: s.FrozenCols, YSplit: s.FrozenRows,
				TopLeftCell: topLeft, ActivePane: "bottomRight",
			}); err != nil {
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
