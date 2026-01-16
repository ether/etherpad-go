package events

import (
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models"
)

type LineHtmlForExportContext struct {
	Line        *models.LineModel
	LineContent *string
	Apool       *apool.APool
	AttribLine  *string
	Text        *string
	PadId       *string
}
