package pad

import (
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/db"
)

func mapDBPadToModel(dbPad *db.PadDB, padToAssignTo *Pad) {
	padToAssignTo.ChatHead = dbPad.ChatHead
	padToAssignTo.Head = dbPad.RevNum
	padToAssignTo.PublicStatus = dbPad.PublicStatus

	var newPool = apool.NewAPool()
	newPool.FromJsonable(dbPad.Pool)

	padToAssignTo.Pool = newPool
	padToAssignTo.AText = dbPad.AText
}
