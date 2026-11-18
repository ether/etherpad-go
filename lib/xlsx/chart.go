package xlsx

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

// Charts are exported as native Excel charts of the same range, cut to what
// the browser shows (MaxChartSeries series, MaxChartPoints points). The
// browser decides categories and series names from the shown values; the
// server does not compute formulas, so it takes a formula for text when it
// builds text (a string literal, "&" or a text function), else for a number.

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
	r0, c0, r1, c1, ok := sheet.ParseChartRange(c.Range)
	if !ok {
		return nil, false
	}
	isText := func(r, col int) bool {
		cell := s.GetCell(sheet.CellRef{Row: r, Col: col})
		raw := strings.TrimSpace(cell.Raw)
		if raw == "" {
			return false
		}
		if cell.Kind() == sheet.KindFormula {
			return textFormulaRe.MatchString(raw)
		}
		_, err := strconv.ParseFloat(raw, 64)
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
	r1 = min(r1, dataRow+sheet.MaxChartPoints-1)
	c1 = min(c1, first+sheet.MaxChartSeries-1)
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

// textFormulaRe: formulas that build text.
var textFormulaRe = regexp.MustCompile(`(?i)"|&|\b(TEXT|CONCAT|CONCATENATE|TEXTJOIN|LEFT|RIGHT|MID|UPPER|LOWER|PROPER|TRIM|SUBSTITUTE|REPLACE|REPT|CHAR|T)\s*\(`)
