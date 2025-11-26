package utils

import (
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/revision"
)

func CreateRevision(changeset string, timestamp int64, isKeyRev bool, authorId *string, atext apool.AText, attribPool apool.APool) revision.Revision {
	if authorId != nil {
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
