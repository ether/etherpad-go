package utils

import (
	"errors"

	pad2 "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/pad"
)

func GetPadSafe(padID string, shouldExist bool, text *string, authorId *string, padManagerToUse *pad.Manager) (*pad2.Pad, error) {

	// A pad whose id is no longer valid for new pads (":" was allowed
	// before) stays reachable through the API as long as it exists.
	if ok, err := padManagerToUse.CanOpenPadId(padID); err != nil {
		return nil, err
	} else if !ok {
		return nil, errors.New("padID is not valid")
	}

	var exists, err = padManagerToUse.DoesPadExist(padID)
	if err != nil {
		return nil, err
	}

	if !*exists && shouldExist {
		return nil, errors.New("padID does not exist")
	}

	if *exists && !shouldExist {
		return nil, errors.New("padID already exists")
	}

	return padManagerToUse.GetPad(padID, text, authorId)
}
