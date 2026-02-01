package events

import (
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models"
)

// LinePDFForExportContext is the context for the getLinePDFForExport hook
type LinePDFForExportContext struct {
	Line        *models.LineModel
	LineContent *string
	Apool       *apool.APool
	AttribLine  *string
	Text        *string
	PadId       *string
	Alignment   *string // "left", "center", "right", "justify"
	FontSize    *float64
	Bold        *bool
	Heading     *string // e.g., "h1", "h2", etc.
}

// LineDocxForExportContext is the context for the getLineDocxForExport hook
type LineDocxForExportContext struct {
	Line        *models.LineModel
	LineContent *string
	Apool       *apool.APool
	AttribLine  *string
	Text        *string
	PadId       *string
	Alignment   *string // "left", "center", "right", "justify"
	Heading     *string // e.g., "Heading1", "Normal", etc. -> word styles
}

// LineOdtForExportContext is the context for the getLineOdtForExport hook
type LineOdtForExportContext struct {
	Line         *models.LineModel
	LineContent  *string
	Apool        *apool.APool
	AttribLine   *string
	Text         *string
	PadId        *string
	Alignment    *string // "left", "center", "right", "justify"
	IsHeading    *bool
	OutlineLevel *int
	Heading      *string // e.g., "Heading 1", "Text Body", etc. -> odt styles
}

type LineMarkdownForExportContext struct {
	Apool      *apool.APool
	AttribLine *string
	Text       *string
	PadId      *string
	Heading    *string // "h1", "h2", etc.
}

// LineTxtForExportContext is the context for the getLineTxtForExport hook
type LineTxtForExportContext struct {
	Line        *models.LineModel
	LineContent *string
	Apool       *apool.APool
	AttribLine  *string
	Text        *string
	PadId       *string
}
