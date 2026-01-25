package pad

import (
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/models/revision"
)

func mapDBPadToModel(dbPad *db.PadDB, padToAssignTo *Pad) {
	padToAssignTo.ChatHead = dbPad.ChatHead
	padToAssignTo.Head = dbPad.Head
	padToAssignTo.PublicStatus = dbPad.PublicStatus
	padToAssignTo.CreatedAt = dbPad.CreatedAt
	padToAssignTo.UpdatedAt = dbPad.UpdatedAt

	padToAssignTo.ReadonlyId = dbPad.ReadOnlyId

	savedRevisions := make([]revision.SavedRevision, len(dbPad.SavedRevisions))
	for i, rev := range dbPad.SavedRevisions {
		savedRevisions[i] = revision.SavedRevision{
			RevNum:    rev.RevNum,
			SavedBy:   rev.SavedBy,
			Timestamp: rev.Timestamp,
			Label:     rev.Label,
			Id:        rev.Id,
		}
	}
	padToAssignTo.SavedRevisions = savedRevisions

	var newPool = apool.NewAPool()
	newPool.FromDB(db.PadPool{
		NumToAttrib: dbPad.Pool.NumToAttrib,
		NextNum:     dbPad.Pool.NextNum,
	})

	padToAssignTo.Pool = newPool
	padToAssignTo.AText = apool.FromDBAText(db.AText{
		Text:    dbPad.ATextText,
		Attribs: dbPad.ATextAttribs,
	})
}
