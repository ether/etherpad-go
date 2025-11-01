package pad

import (
	"strings"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/utils"
)

type ReadOnlyManager struct {
	Store db.DataStore
}

type IdRequest struct {
	ReadOnlyPadId string
	PadId         string
	ReadOnly      bool
}

func NewReadOnlyManager() *ReadOnlyManager {
	return &ReadOnlyManager{
		Store: utils.GetDB(),
	}
}

func (r *ReadOnlyManager) isReadOnlyID(id *string) bool {
	return strings.HasPrefix(*id, "r.")
}

func (r *ReadOnlyManager) GetReadOnlyId(pad string) string {
	var readonlyId, err = r.Store.GetReadonlyPad(pad)
	if err != nil {
		var randomId = "r." + utils.RandomString(16)
		r.Store.CreateReadOnly2Pad(pad, randomId)
		r.Store.CreatePad2ReadOnly(pad, randomId)
		return randomId
	}

	return *readonlyId
}

func (r *ReadOnlyManager) RemoveReadOnlyPad(readonlyId, padId string) error {
	err := r.Store.RemoveReadOnly2Pad(readonlyId)
	if err != nil {
		return err
	}

	err = r.Store.RemovePad2ReadOnly(padId)
	return err
}

func (r *ReadOnlyManager) getPadId(readonlyId string) *string {
	return r.Store.GetReadOnly2Pad(readonlyId)
}

func (r *ReadOnlyManager) GetIds(id *string) IdRequest {
	readonly := r.isReadOnlyID(id)
	var readOnlyPadId string
	if readonly {
		readOnlyPadId = *id
	} else {
		readOnlyPadId = r.GetReadOnlyId(*id)
	}

	var padId string

	if readonly {
		padId = *r.getPadId(readOnlyPadId)
	} else {
		padId = *id
	}

	return IdRequest{
		ReadOnlyPadId: readOnlyPadId,
		PadId:         padId,
		ReadOnly:      readonly,
	}

}
