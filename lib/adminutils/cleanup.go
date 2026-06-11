package utils

import (
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/revision"
)

func CreateRevision(changeset string, timestamp int64, isKeyRev bool, authorId *string, atext apool.AText, attribPool apool.APool) revision.Revision {
	if authorId != nil {
		// Work on a deep copy: the struct parameter shares the caller's maps,
		// so PutAttrib on it would add entries to the caller's pool without
		// bumping the caller's NextNum, corrupting the pool (Pad.Check then
		// fails with "numToAttrib length does not match nextNum").
		attribPool = attribPool.Clone()
		attribPool.PutAttrib(apool.Attribute{
			Key:   "author",
			Value: *authorId,
		}, nil)
	}

	rev := revision.Revision{
		Changeset: changeset,
		Meta: revision.RevisionMeta{
			Author:    authorId,
			Timestamp: timestamp,
		},
	}

	if isKeyRev {
		rev.Meta.Atext = &atext
		rev.Meta.APool = &attribPool
	}
	rev.Meta.IsKeyRev = isKeyRev

	return rev
}
