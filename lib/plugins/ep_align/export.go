package ep_align

import (
	"regexp"
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/hooks/events"
)

// analyzeLine extracts the alignment attribute from a line's attribute string.
// It returns the alignment value (e.g., "left", "center", "right", "justify") or nil if not set.
func analyzeLine(alineAttrs *string, pool *apool.APool) *string {
	if alineAttrs == nil {
		return nil
	}

	ops, err := changeset.DeserializeOps(*alineAttrs)
	if err != nil {
		return nil
	}

	// Check the first op for the align attribute (like the original JS code)
	if ops != nil && len(*ops) > 0 {
		op := (*ops)[0]
		// Create an AttributeMap from the op's attribs string
		attributeMap := changeset.FromString(op.Attribs, pool)
		return attributeMap.Get("align")
	}

	return nil
}

// GetLineHTMLForExport wraps a line with the appropriate alignment tag for export.
func GetLineHTMLForExport(event *events.LineHtmlForExportContext) {
	align := analyzeLine(event.AttribLine, event.Apool)
	if align == nil {
		return
	}

	lineContent := *event.LineContent
	text := ""
	if event.Text != nil {
		text = *event.Text
	}

	// Remove leading '*' if present in text
	if len(text) > 0 && text[0] == '*' {
		lineContent = strings.Replace(lineContent, "*", "", 1)
	}

	// Check if there's a heading tag
	headingRegex := regexp.MustCompile(`<h([1-6])([^>]+)?>`)
	headingMatch := headingRegex.FindString(lineContent)

	if headingMatch != "" {
		// There's a heading, add style to it
		if !strings.Contains(headingMatch, "style=") {
			// No style attribute, add one
			lineContent = strings.Replace(lineContent, ">", " style='text-align:"+*align+"'>", 1)
		} else {
			// Style attribute exists, append to it
			lineContent = strings.Replace(lineContent, "style=", "style='text-align:"+*align+" ", 1)
		}
	} else {
		// No heading, wrap in a <p> tag
		lineContent = "<p style='text-align:" + *align + "'>" + lineContent + "</p>"
	}

	// Write the modified content back to the event
	*event.LineContent = lineContent
}

// GetLinePDFForExport sets alignment for PDF export
func GetLinePDFForExport(event *events.LinePDFForExportContext) {
	align := analyzeLine(event.AttribLine, event.Apool)
	if align == nil {
		return
	}
	event.Alignment = align
}

// GetLineDocxForExport sets alignment for DOCX export
func GetLineDocxForExport(event *events.LineDocxForExportContext) {
	align := analyzeLine(event.AttribLine, event.Apool)
	if align == nil {
		return
	}
	event.Alignment = align
}

// GetLineOdtForExport sets alignment for ODT export
func GetLineOdtForExport(event *events.LineOdtForExportContext) {
	align := analyzeLine(event.AttribLine, event.Apool)
	if align == nil {
		return
	}
	event.Alignment = align
}
