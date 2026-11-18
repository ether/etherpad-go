package sheet

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Chart is a chart floating over a sheet, drawn by the browser from a range
// of the same sheet. Its top-left corner sits on cell (Row, Col); the op
// carries the anchor in Op.Row/Op.Col so Transform moves it like any cell.
type Chart struct {
	Id     string `json:"id"`
	Type   string `json:"type"`  // column, bar, line, pie
	Range  string `json:"range"` // e.g. A1:C10 on the same sheet, #REF! once deleted
	Title  string `json:"title,omitempty"`
	Row    int    `json:"row"`
	Col    int    `json:"col"`
	Width  int    `json:"width"`  // px
	Height int    `json:"height"` // px
}

const (
	MaxChartsPerSheet = 50
	MaxChartTitle     = 255
	// A chart shows at most MaxChartSeries series (plus a category column)
	// and MaxChartPoints points (plus a header row); its range may not be
	// taller than the grid.
	MaxChartSeries = 50
	MaxChartPoints = 500
	maxChartRows   = 10000
	minChartSize   = 80
	maxChartSize   = 3000
)

var (
	ChartTypes   = []string{"column", "bar", "line", "pie"}
	chartIdRe    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	chartRangeRe = regexp.MustCompile(`^\$?([A-Z]{1,3})\$?([1-9][0-9]{0,6})(?::\$?([A-Z]{1,3})\$?([1-9][0-9]{0,6}))?$`)
)

func validChartId(id string) bool { return chartIdRe.MatchString(id) }

// ParseChartRange reads "A1:C10" (or one cell) into zero-based bounds.
func ParseChartRange(r string) (r0, c0, r1, c1 int, ok bool) {
	m := chartRangeRe.FindStringSubmatch(r)
	if m == nil {
		return 0, 0, 0, 0, false
	}
	col := func(s string) int {
		n := 0
		for _, ch := range s {
			n = n*26 + int(ch-'A'+1)
		}
		return n - 1
	}
	row := func(s string) int { n, _ := strconv.Atoi(s); return n - 1 }
	ra, ca := row(m[2]), col(m[1])
	rb, cb := ra, ca
	if m[3] != "" {
		rb, cb = row(m[4]), col(m[3])
	}
	return min(ra, rb), min(ca, cb), max(ra, rb), max(ca, cb), true
}

func (c *Chart) validate() error {
	if !validChartId(c.Id) {
		return fmt.Errorf("chart: bad id")
	}
	if !slices.Contains(ChartTypes, c.Type) {
		return fmt.Errorf("chart: unknown type %q", c.Type)
	}
	if c.Range != "#REF!" {
		r0, c0, r1, c1, ok := ParseChartRange(c.Range)
		if !ok {
			return fmt.Errorf("chart: bad range %q", c.Range)
		}
		if c1-c0+1 > MaxChartSeries+1 || r1-r0+1 > maxChartRows {
			return fmt.Errorf("chart: range %q is too large", c.Range)
		}
	}
	if len(c.Title) > MaxChartTitle {
		return fmt.Errorf("chart: title too long")
	}
	if c.Width < minChartSize || c.Width > maxChartSize || c.Height < minChartSize || c.Height > maxChartSize {
		return fmt.Errorf("chart: size out of range")
	}
	return nil
}

func (s *Sheet) chartIndex(id string) int {
	return slices.IndexFunc(s.Charts, func(c Chart) bool { return c.Id == id })
}

// moveCharts follows a row/column insert or delete: anchors move like cells
// (one inside a deleted band lands on its first row/column) and the data
// range like a formula's reference.
func (s *Sheet) moveCharts(e RefEdit) {
	for i := range s.Charts {
		c := &s.Charts[i]
		if e.Axis == "row" {
			c.Row = shiftChartAnchor(c.Row, e.Index, e.Delta)
		} else {
			c.Col = shiftChartAnchor(c.Col, e.Index, e.Delta)
		}
		c.Range = moveChartRange(c.Range, e)
	}
}

func shiftChartAnchor(v, index, delta int) int {
	if delta < 0 && v >= index && v < index-delta {
		return index
	}
	return shiftCoord(v, index, delta)
}

// moveChartRange rewrites a chart's range as a formula "=range" on its own
// sheet.
func moveChartRange(r string, e RefEdit) string {
	if r == "#REF!" {
		return r
	}
	e.OnTarget = true
	return strings.TrimPrefix(RewriteFormula("="+r, e), "=")
}
