package xlsx

import (
	"bytes"
	"io"
	"math"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

// The client grid is 10000 rows by 104 columns (A..CZ); nothing beyond is read.
const (
	maxRows = 10000
	maxCols = 104
)

// Import parses an .xlsx into a WorkbookSnapshot. Sheet id == sheet name. Cells
// carry their raw value, or "=<formula>" when a formula is present. Cell styles
// (allowlisted props only), column widths / row heights, merged ranges and
// freeze panes are imported.
func Import(r io.Reader) (sheet.WorkbookSnapshot, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return sheet.WorkbookSnapshot{}, err
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return sheet.WorkbookSnapshot{}, err
	}
	defer f.Close()
	sheets := sheetXMLByName(data)
	budget := &refBudget{left: maxMetaCells}

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
		if len(rows) > maxRows {
			rows = rows[:maxRows]
		}
		for rIdx, row := range rows {
			if len(row) > maxCols {
				row = row[:maxCols]
			}
			for cIdx, val := range row {
				axis, err := excelize.CoordinatesToCellName(cIdx+1, rIdx+1)
				if err != nil {
					return sheet.WorkbookSnapshot{}, err
				}
				raw := val
				if formula, ferr := f.GetCellFormula(name, axis); ferr == nil && formula != "" {
					raw = "=" + stripFunctionPrefixes(formula)
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

		// Styled-but-empty cells never show up in GetRows: take them from
		// the sheet XML.
		sx := sheets[name]
		if sx == nil {
			sx = &sheetXML{}
		}
		for _, sc := range sx.styled {
			ref := sheet.CellRef{Row: sc.ref[0], Col: sc.ref[1]}
			if _, seen := sh.Cells[ref]; seen {
				continue
			}
			if id := poolIdFor(sc.style); id != 0 {
				sh.SetCell(ref, sheet.Cell{StyleId: id})
			}
		}

		// Dimensions: excelize returns the sheet default for untouched
		// indices, so probe a far-away column/row as the baseline and store
		// only deviations.
		baseW, _ := f.GetColWidth(name, "XFD")
		for c := range maxCols {
			cn, _ := excelize.ColumnNumberToName(c + 1)
			if w, err := f.GetColWidth(name, cn); err == nil && math.Abs(w-baseW) > 0.01 {
				sh.ColWidths[c] = colWidthToPx(w)
			}
		}
		baseH, _ := f.GetRowHeight(name, excelize.TotalRows)
		for _, r := range sx.tallRows {
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

		importCellMeta(f, wb, sh, name, budget, sx.condFormats)
		importFilter(sh, sx.autoFilter)

		if panes, err := f.GetPanes(name); err == nil && panes.Freeze {
			sh.FrozenRows = min(max(panes.YSplit, 0), sheet.MaxFrozenRows)
			sh.FrozenCols = min(max(panes.XSplit, 0), sheet.MaxFrozenCols)
		}
	}
	return wb.Snapshot(), nil
}
