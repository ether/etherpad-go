package xlsx

import (
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

// Charts are exported as native Excel charts of the same range. The
// browser decides categories and series names from the shown values; the
// export judges by the stored text (formulas count as numbers), which is
// the same for the usual table of a text column and number columns.

var chartTypes = map[string]excelize.ChartType{
	"column": excelize.Col, "bar": excelize.Bar, "line": excelize.Line, "pie": excelize.Pie,
}

func exportCharts(f *excelize.File, s *sheet.Sheet, name string) error {
	for _, c := range s.Charts {
		ch, ok := excelChart(s, name, c)
		if !ok {
			continue
		}
		anchor, err := excelize.CoordinatesToCellName(c.Col+1, c.Row+1)
		if err != nil {
			return err
		}
		if err := f.AddChart(name, anchor, ch); err != nil {
			return err
		}
	}
	return nil
}

// excelChart builds the excelize chart; false when the range holds no data.
func excelChart(s *sheet.Sheet, name string, c sheet.Chart) (*excelize.Chart, bool) {
	r0, c0, r1, c1, ok := parseA1Range(c.Range)
	if !ok {
		return nil, false
	}
	isText := func(r, col int) bool {
		cell := s.GetCell(sheet.CellRef{Row: r, Col: col})
		if strings.TrimSpace(cell.Raw) == "" || cell.Kind() == sheet.KindFormula {
			return false
		}
		_, err := strconv.ParseFloat(strings.TrimSpace(cell.Raw), 64)
		return err != nil
	}
	empty := func(r, col int) bool { return strings.TrimSpace(s.GetCell(sheet.CellRef{Row: r, Col: col}).Raw) == "" }
	catCol := false
	if c1 > c0 {
		catCol = empty(r0, c0)
		for r := r0 + 1; r <= r1 && !catCol; r++ {
			catCol = isText(r, c0)
		}
	}
	first := c0
	if catCol {
		first = c0 + 1
	}
	header := false
	for col := first; col <= c1 && !header; col++ {
		header = isText(r0, col)
	}
	dataRow := r0
	if header {
		dataRow = r0 + 1
	}
	if dataRow > r1 || first > c1 {
		return nil, false
	}
	ref := func(col, a, b int) string {
		from, _ := excelize.CoordinatesToCellName(col+1, a+1, true)
		to, _ := excelize.CoordinatesToCellName(col+1, b+1, true)
		return sheet.QuoteSheetName(name) + "!" + from + ":" + to
	}
	ch := &excelize.Chart{
		Type:      chartTypes[c.Type],
		Dimension: excelize.ChartDimension{Width: uint(c.Width), Height: uint(c.Height)},
		Legend:    excelize.ChartLegend{Position: "bottom"},
	}
	// One colour per series; only a pie colours its slices apart.
	vary := c.Type == "pie"
	ch.VaryColors = &vary
	if c.Title != "" {
		ch.Title = excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: c.Title}}}
	}
	for col := first; col <= c1; col++ {
		series := excelize.ChartSeries{Values: ref(col, dataRow, r1)}
		if catCol {
			series.Categories = ref(c0, dataRow, r1)
		}
		if header {
			cell, _ := excelize.CoordinatesToCellName(col+1, r0+1, true)
			series.Name = sheet.QuoteSheetName(name) + "!" + cell
		}
		ch.Series = append(ch.Series, series)
		if c.Type == "pie" {
			break // a pie shows one series
		}
	}
	return ch, true
}

// parseA1Range reads "A1:C10" (or one cell) into zero-based bounds.
func parseA1Range(r string) (r0, c0, r1, c1 int, ok bool) {
	parts := strings.SplitN(strings.ReplaceAll(r, "$", ""), ":", 2)
	ca, ra, err := excelize.CellNameToCoordinates(parts[0])
	if err != nil {
		return 0, 0, 0, 0, false
	}
	cb, rb := ca, ra
	if len(parts) == 2 {
		if cb, rb, err = excelize.CellNameToCoordinates(parts[1]); err != nil {
			return 0, 0, 0, 0, false
		}
	}
	return min(ra, rb) - 1, min(ca, cb) - 1, max(ra, rb) - 1, max(ca, cb) - 1, true
}
