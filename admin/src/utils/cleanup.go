package utils

import (
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/pad"
)

func CreateRevision(changeset string, timestamp int64, isKeyRev bool, authorId *string, atext apool.AText, attribPool apool.APool) pad.Revision {
	if authorId != nil {
		attribPool.PutAttrib(apool.Attribute{
			Key:   "author",
			Value: *authorId,
		}, nil)
	}

	rev := pad.Revision{
		Changeset: changeset,
		Meta: pad.RevisionMeta{
			Author:    authorId,
			Timestamp: timestamp,
		},
	}

	if isKeyRev {
		rev.Meta.Atext = &atext
		rev.Meta.APool = &attribPool
	}

	return rev
}
