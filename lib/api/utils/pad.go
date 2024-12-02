package utils

import (
	"errors"
	pad2 "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/pad"
)

func GetPadSafe(padID string, shouldExist bool, text *string, authorId *string) (*pad2.Pad, error) {
	var padManager = pad.NewManager()

	if !padManager.IsValidPadId(padID) {
		return nil, errors.New("padID is not valid")
	}

	var exists = padManager.DoesPadExist(padID)

	if !exists && shouldExist {
		return nil, errors.New("padID does not exist")
	}

	if exists && !shouldExist {
		return nil, errors.New("padID already exists")
	}

	return padManager.GetPad(padID, text, authorId)
}
