package pad

import (
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/utils"
	"strings"
)

type ReadOnlyManager struct {
	Store db.DataStore
}

type IdRequest struct {
	ReadOnlyPadId string
	PadId         string
	ReadOnly      bool
}

func (r *ReadOnlyManager) isReadOnlyID(id *string) bool {
	return strings.HasPrefix(*id, "r.")
}

func (r *ReadOnlyManager) getReadOnlyId(pad string) string {
	var readonlyId, err = r.Store.GetReadonlyPad(pad)
	if err != nil {
		readonlyId = "r." + utils.RandomString(16)
		r.Store.CreateReadOnly2Pad(pad, readonlyId)
		r.Store.CreatePad2ReadOnly(pad, readonlyId)
	}

	return readonlyId
}

func (r *ReadOnlyManager) getPadId(readonlyId string) string {
	return r.Store.GetReadOnly2Pad(readonlyId)
}

func (r *ReadOnlyManager) getIds(id *string) IdRequest {
	readonly := r.isReadOnlyID(id)
	var readOnlyPadId string
	if readonly {
		readOnlyPadId = *id
	} else {
		readOnlyPadId = r.getReadOnlyId(*id)
	}

	var padId string

	if readonly {
		padId = r.getPadId(readOnlyPadId)
	} else {
		padId = *id
	}

	return IdRequest{
		ReadOnlyPadId: readOnlyPadId,
		PadId:         padId,
		ReadOnly:      readonly,
	}

}
