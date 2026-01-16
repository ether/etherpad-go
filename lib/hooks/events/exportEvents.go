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
}

// LineOdtForExportContext is the context for the getLineOdtForExport hook
type LineOdtForExportContext struct {
	Line        *models.LineModel
	LineContent *string
	Apool       *apool.APool
	AttribLine  *string
	Text        *string
	PadId       *string
	Alignment   *string // "left", "center", "right", "justify"
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
