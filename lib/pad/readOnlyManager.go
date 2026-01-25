package pad

import (
	"errors"
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

func NewReadOnlyManager(db db.DataStore) *ReadOnlyManager {
	return &ReadOnlyManager{
		Store: db,
	}
}

func (r *ReadOnlyManager) IsReadOnlyID(id *string) bool {
	return strings.HasPrefix(*id, "r.")
}

func (r *ReadOnlyManager) GetReadOnlyId(pad string) string {
	var readonlyId, err = r.Store.GetReadonlyPad(pad)
	if err != nil {
		var randomId = "r." + utils.RandomString(16)
		r.Store.SetReadOnlyId(pad, randomId)
		return randomId
	}

	return *readonlyId
}

func (r *ReadOnlyManager) GetPadId(readonlyId string) (*string, error) {
	return r.Store.GetPadByReadOnlyId(readonlyId)
}

func (r *ReadOnlyManager) GetIds(id *string) (*IdRequest, error) {
	readonly := r.IsReadOnlyID(id)
	var readOnlyPadId string
	if readonly {
		readOnlyPadId = *id
	} else {
		readOnlyPadId = r.GetReadOnlyId(*id)
	}

	var padId string

	if readonly {
		padIdPtr, err := r.GetPadId(readOnlyPadId)
		if err != nil {
			return nil, err
		}

		if padIdPtr == nil {
			return nil, errors.New("pad not found for readonly ID: " + readOnlyPadId)
		}

		padId = *padIdPtr
	} else {
		padId = *id
	}

	return &IdRequest{
		ReadOnlyPadId: readOnlyPadId,
		PadId:         padId,
		ReadOnly:      readonly,
	}, nil

}
