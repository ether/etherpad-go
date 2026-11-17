package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"path"
	"strings"
)

// excelize reports conditional formats without their priority, which
// decides the winner where ranges overlap; read them from the sheet XML.

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

// maxSheetXML bounds the sheet XML read for conditional formats.
const maxSheetXML = 100 << 20

// condFormatsBySheet returns each sheet's conditional formats by sheet
// name; nil when the file cannot be read this way (then none are imported).
func condFormatsBySheet(data []byte) map[string][]sheetCondFormat {
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
	out := map[string][]sheetCondFormat{}
	for _, sh := range wbx.Sheets {
		data := read(targets[sh.RID])
		if data == nil {
			continue
		}
		var ws struct {
			CF []sheetCondFormat `xml:"conditionalFormatting"`
		}
		if xml.Unmarshal(data, &ws) == nil {
			out[sh.Name] = ws.CF
		}
	}
	return out
}
