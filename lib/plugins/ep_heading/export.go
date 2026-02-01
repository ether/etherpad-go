package ep_heading

import (
	"fmt"
	"strconv"

	"github.com/ether/etherpad-go/lib/hooks/events"
)

// PDF Export
func (e *EpHeadingsPlugin) getLinePDFForExport(ctx *events.LinePDFForExportContext) {
	header := e.analyzeLine(ctx.AttribLine, ctx.Apool)
	if header == nil {
		return
	}

	// Heading-Level extrahieren
	level := 1
	if len(*header) >= 2 {
		if l, err := strconv.Atoi(string((*header)[1])); err == nil {
			level = l
		}
	}

	// DOCX Heading Style (Heading1, Heading2, etc.)
	style := fmt.Sprintf("Heading%d", level)
	ctx.Heading = &style
}

// DOCX Export
func (e *EpHeadingsPlugin) getLineDocxForExport(ctx *events.LineDocxForExportContext) {
	header := e.analyzeLine(ctx.AttribLine, ctx.Apool)
	if header == nil {
		return
	}

	// Heading-Level extrahieren
	level := 1
	if len(*header) >= 2 {
		if l, err := strconv.Atoi(string((*header)[1])); err == nil {
			level = l
		}
	}

	// DOCX Heading Style (Heading1, Heading2, etc.)
	style := fmt.Sprintf("Heading%d", level)
	ctx.Heading = &style
}

// ODT Export
func (e *EpHeadingsPlugin) getLineOdtForExport(ctx *events.LineOdtForExportContext) {
	header := e.analyzeLine(ctx.AttribLine, ctx.Apool)
	if header == nil {
		return
	}

	// Heading-Level extrahieren
	level := 1
	if len(*header) >= 2 {
		if l, err := strconv.Atoi(string((*header)[1])); err == nil {
			level = l
		}
	}

	// DOCX Heading Style (Heading1, Heading2, etc.)
	style := fmt.Sprintf("Heading%d", level)
	ctx.Heading = &style
}
