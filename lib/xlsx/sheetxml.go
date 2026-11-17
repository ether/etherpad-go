package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Some things excelize does not tell, so they are read from the sheet XML:
// the priority of conditional formats (it decides the winner where ranges
// overlap), the cells that carry a style but no value (GetRows skips them)
// and the rows with their own height. Reading the XML costs what the file
// holds, where probing every cell of the grid would cost a million lookups.

type xmlCfRule struct {
	Type     string   `xml:"type,attr"`
	Priority int      `xml:"priority,attr"`
	DxfID    *int     `xml:"dxfId,attr"`
	Operator string   `xml:"operator,attr"`
	Text     string   `xml:"text,attr"`
	Formula  []string `xml:"formula"`
}

type sheetCondFormat struct {
	Sqref string      `xml:"sqref,attr"`
	Rules []xmlCfRule `xml:"cfRule"`
}

// maxSheetXML bounds the sheet XML read.
const maxSheetXML = 100 << 20

// styledCell is a <c> with a style index inside the grid.
type styledCell struct {
	ref   [2]int // row, col (zero-based)
	style int
}

type sheetXML struct {
	condFormats []sheetCondFormat
	styled      []styledCell
	tallRows    []int // zero-based rows with their own height
}

// parseSheetXML streams one worksheet.
func parseSheetXML(data []byte) *sheetXML {
	out := &sheetXML{}
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return out
			}
			return out // keep what was read
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "row":
			var r int
			custom := false
			for _, a := range se.Attr {
				switch a.Name.Local {
				case "r":
					r, _ = strconv.Atoi(a.Value)
				case "customHeight":
					custom = a.Value == "1" || a.Value == "true"
				}
			}
			if custom && r >= 1 && r <= maxRows {
				out.tallRows = append(out.tallRows, r-1)
			}
		case "c":
			var ref string
			style := 0
			for _, a := range se.Attr {
				switch a.Name.Local {
				case "r":
					ref = a.Value
				case "s":
					style, _ = strconv.Atoi(a.Value)
				}
			}
			if style <= 0 || ref == "" {
				continue
			}
			c, r, err := excelize.CellNameToCoordinates(ref)
			if err != nil || r > maxRows || c > maxCols || len(out.styled) >= maxRows*maxCols {
				continue
			}
			out.styled = append(out.styled, styledCell{ref: [2]int{r - 1, c - 1}, style: style})
		case "conditionalFormatting":
			var cf sheetCondFormat
			if dec.DecodeElement(&cf, &se) == nil {
				out.condFormats = append(out.condFormats, cf)
			}
		}
	}
}

// sheetXMLByName reads every worksheet, by sheet name; nil when the file
// cannot be read this way.
func sheetXMLByName(data []byte) map[string]*sheetXML {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}
	read := func(name string) []byte {
		for _, zf := range zr.File {
			if zf.Name != name {
				continue
			}
			rc, err := zf.Open()
			if err != nil {
				return nil
			}
			defer rc.Close()
			b, err := io.ReadAll(io.LimitReader(rc, maxSheetXML+1))
			if err != nil || len(b) > maxSheetXML {
				return nil
			}
			return b
		}
		return nil
	}

	var wbx struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	if xml.Unmarshal(read("xl/workbook.xml"), &wbx) != nil {
		return nil
	}
	var rels struct {
		Rels []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if xml.Unmarshal(read("xl/_rels/workbook.xml.rels"), &rels) != nil {
		return nil
	}
	targets := map[string]string{}
	for _, r := range rels.Rels {
		t := r.Target
		if strings.HasPrefix(t, "/") {
			t = strings.TrimPrefix(t, "/")
		} else {
			t = path.Join("xl", t)
		}
		targets[r.ID] = t
	}
	out := map[string]*sheetXML{}
	for _, sh := range wbx.Sheets {
		if data := read(targets[sh.RID]); data != nil {
			out[sh.Name] = parseSheetXML(data)
		}
	}
	return out
}
