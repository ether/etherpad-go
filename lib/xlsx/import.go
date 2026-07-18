package xlsx

import (
	"io"
	"math"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

// The client grid is fixed at 200x52; dimension scans stop there.
const (
	maxRows = 200
	maxCols = 52
)

// Import parses an .xlsx into a WorkbookSnapshot. Sheet id == sheet name. Cells
// carry their raw value, or "=<formula>" when a formula is present. Cell styles
// (allowlisted props only), column widths / row heights and freeze panes are
// imported; merges are skipped (the sheet model does not store them).
func Import(r io.Reader) (sheet.WorkbookSnapshot, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return sheet.WorkbookSnapshot{}, err
	}
	defer f.Close()

	wb := sheet.NewWorkbook()
	// excelize style idx -> pool id (0 = nothing representable), shared across
	// sheets since styles are file-global.
	poolIds := map[int]int{}
	poolIdFor := func(xid int) int {
		if id, ok := poolIds[xid]; ok {
			return id
		}
		id := 0
		if st, err := f.GetStyle(xid); err == nil && st != nil {
			if props := styleToProps(st); len(props) > 0 {
				id = wb.Styles.Put(sheet.Style{Props: props})
			}
		}
		poolIds[xid] = id
		return id
	}

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
				styleId := 0
				if xid, serr := f.GetCellStyle(name, axis); serr == nil && xid != 0 {
					styleId = poolIdFor(xid)
				}
				if raw == "" && styleId == 0 {
					continue
				}
				sh.SetCell(sheet.CellRef{Row: rIdx, Col: cIdx}, sheet.Cell{Raw: raw, StyleId: styleId})
			}
		}

		// Styled-but-empty cells never show up in GetRows, and excelize does
		// not maintain the sheet dimension attribute, so sweep the whole grid
		// for styles on cells not yet seen. ponytail: 200x52 lookups per
		// sheet, fine for an import endpoint.
		for r := range maxRows {
			for c := range maxCols {
				ref := sheet.CellRef{Row: r, Col: c}
				if _, seen := sh.Cells[ref]; seen {
					continue
				}
				axis, _ := excelize.CoordinatesToCellName(c+1, r+1)
				if xid, serr := f.GetCellStyle(name, axis); serr == nil && xid != 0 {
					if id := poolIdFor(xid); id != 0 {
						sh.SetCell(ref, sheet.Cell{StyleId: id})
					}
				}
			}
		}

		// Dimensions: excelize returns the sheet default for untouched
		// indices, so probe a far-away column/row as the baseline and store
		// only deviations. ponytail: scans the fixed grid extent (52/200).
		baseW, _ := f.GetColWidth(name, "XFD")
		for c := range maxCols {
			cn, _ := excelize.ColumnNumberToName(c + 1)
			if w, err := f.GetColWidth(name, cn); err == nil && math.Abs(w-baseW) > 0.01 {
				sh.ColWidths[c] = colWidthToPx(w)
			}
		}
		baseH, _ := f.GetRowHeight(name, excelize.TotalRows)
		for r := range maxRows {
			if h, err := f.GetRowHeight(name, r+1); err == nil && math.Abs(h-baseH) > 0.01 {
				sh.RowHeights[r] = rowHeightToPx(h)
			}
		}

		if mcs, merr := f.GetMergeCells(name); merr == nil {
			for _, mc := range mcs {
				c0, r0, e0 := excelize.CellNameToCoordinates(mc.GetStartAxis())
				c1, r1, e1 := excelize.CellNameToCoordinates(mc.GetEndAxis())
				if e0 != nil || e1 != nil || (r1 == r0 && c1 == c0) {
					continue
				}
				sh.Merges[sheet.CellRef{Row: r0 - 1, Col: c0 - 1}] = sheet.Span{Rows: r1 - r0 + 1, Cols: c1 - c0 + 1}
			}
		}

		// Freeze panes: the model only supports freezing the first row/col.
		if panes, err := f.GetPanes(name); err == nil && panes.Freeze {
			if panes.YSplit > 0 {
				sh.FrozenRows = 1
			}
			if panes.XSplit > 0 {
				sh.FrozenCols = 1
			}
		}
	}
	return wb.Snapshot(), nil
}
