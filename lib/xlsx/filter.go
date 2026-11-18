package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

// Sheet filters <-> xlsx AutoFilter. excelize writes an AutoFilter with at
// most two values per column, so Export writes the range with excelize and
// the value lists into the worksheet XML afterwards.

// filterColumns of one exported sheet: the <filterColumn> elements to put
// into its <autoFilter>.
type filterColumns struct {
	sheet string
	xml   string
}

// exportFilter writes the AutoFilter range of a sheet with filters and hides
// the rows it filters out (Excel shows the hidden rows of a file, it does
// not filter again on open). Rows are judged by their stored text; cells the
// server cannot show as the browser does (formulas, number formats) never
// hide a row.
func exportFilter(f *excelize.File, wb *sheet.Workbook, s *sheet.Sheet, name string) (*filterColumns, error) {
	if len(s.Filters) == 0 {
		return nil, nil
	}
	lastRow, lastCol := s.FilterRow, 0
	for ref := range s.Cells {
		lastRow = max(lastRow, ref.Row)
		lastCol = max(lastCol, ref.Col)
	}
	cols := make([]int, 0, len(s.Filters))
	for c := range s.Filters {
		cols = append(cols, c)
		lastCol = max(lastCol, c)
	}
	sort.Ints(cols)
	from, _ := excelize.CoordinatesToCellName(1, s.FilterRow+1)
	to, _ := excelize.CoordinatesToCellName(lastCol+1, max(lastRow, s.FilterRow+1)+1)
	if err := f.AutoFilter(name, from+":"+to, nil); err != nil {
		return nil, err
	}
	judged := func(cell sheet.Cell) bool {
		if cell.Raw == "" || cell.Kind() == sheet.KindFormula {
			return false
		}
		st, _ := wb.Styles.Get(cell.StyleId)
		f := st.Props["numFmt"]
		return f == "" || f == "general" || f == "text"
	}
	for r := s.FilterRow + 1; r <= lastRow; r++ {
		for _, c := range cols {
			cell := s.GetCell(sheet.CellRef{Row: r, Col: c})
			if judged(cell) && !s.FilterKeeps(c, cell.Raw) {
				if err := f.SetRowVisible(name, r+1, false); err != nil {
					return nil, err
				}
				break
			}
		}
	}
	var b strings.Builder
	for _, c := range cols {
		// xlsx lists the kept values; a filter that lists the hidden ones
		// keeps the column's other values.
		kept := s.Filters[c]
		if s.FilterHide[c] {
			kept = nil
			seen := map[string]bool{}
			for r := s.FilterRow + 1; r <= lastRow; r++ {
				v := s.GetCell(sheet.CellRef{Row: r, Col: c}).Raw
				if strings.TrimSpace(v) == "" || seen[v] || !s.FilterKeeps(c, v) {
					continue
				}
				seen[v] = true
				kept = append(kept, v)
			}
		}
		b.WriteString(`<filterColumn colId="` + strconv.Itoa(c) + `"><filters>`)
		for _, v := range kept {
			b.WriteString(`<filter val="`)
			_ = xml.EscapeText(&b, []byte(v))
			b.WriteString(`"/>`)
		}
		b.WriteString(`</filters></filterColumn>`)
	}
	return &filterColumns{sheet: name, xml: b.String()}, nil
}

var autoFilterRe = regexp.MustCompile(`(?s)<autoFilter ref="([^"]*)"\s*(?:/>|>.*?</autoFilter>)`)

// injectFilterColumns puts the value lists into the written workbook.
func injectFilterColumns(data []byte, fcs []*filterColumns) ([]byte, error) {
	if len(fcs) == 0 {
		return data, nil
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	paths := sheetPaths(zipRead(zr))
	edits := map[string]string{}
	for _, fc := range fcs {
		if p := paths[fc.sheet]; p != "" {
			edits[p] = fc.xml
		}
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, zf := range zr.File {
		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		if cols, ok := edits[zf.Name]; ok {
			body = autoFilterRe.ReplaceAllFunc(body, func(m []byte) []byte {
				ref := autoFilterRe.FindSubmatch(m)[1]
				return []byte(`<autoFilter ref="` + string(ref) + `">` + cols + `</autoFilter>`)
			})
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: zf.Name, Method: zip.Deflate, Modified: zf.Modified})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(body); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// importFilter reads an AutoFilter's value lists (custom filters such as
// "greater than" are not kept).
func importFilter(sh *sheet.Sheet, af *xmlAutoFilter) {
	if af == nil {
		return
	}
	start := strings.SplitN(af.Ref, ":", 2)[0]
	c0, r0, err := excelize.CellNameToCoordinates(start)
	if err != nil {
		return
	}
	for _, fc := range af.Columns {
		// Date groups ("all of 2024") are not read; with them missing, the
		// remaining list would hide those rows.
		if fc.Filters == nil || len(fc.Filters.Values) == 0 || len(fc.Filters.DateGroups) > 0 {
			continue
		}
		col := c0 - 1 + fc.ColID
		if col < 0 || col >= maxCols {
			continue
		}
		var values []string
		for _, v := range fc.Filters.Values {
			if len(v.Val) <= sheet.MaxFilterValueLength && len(values) < sheet.MaxFilterValues {
				values = append(values, v.Val)
			}
		}
		if len(values) == 0 {
			continue
		}
		if sh.Filters == nil {
			sh.Filters = map[int][]string{}
		}
		sh.Filters[col] = values
		sh.FilterRow = r0 - 1
	}
}
