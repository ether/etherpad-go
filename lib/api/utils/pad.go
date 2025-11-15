package utils

import (
	"errors"

	pad2 "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/pad"
)

func GetPadSafe(padID string, shouldExist bool, text *string, authorId *string, padManagerToUse *pad.Manager) (*pad2.Pad, error) {

	if !padManagerToUse.IsValidPadId(padID) {
		return nil, errors.New("padID is not valid")
	}

	var exists = padManagerToUse.DoesPadExist(padID)

	if !exists && shouldExist {
		return nil, errors.New("padID does not exist")
	}

	if exists && !shouldExist {
		return nil, errors.New("padID already exists")
	}

	return padManagerToUse.GetPad(padID, text, authorId)
}
