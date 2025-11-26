package revision

import "github.com/ether/etherpad-go/lib/apool"

type SavedRevision struct {
	RevNum int
}

type Revision struct {
	Changeset string
	Meta      RevisionMeta
}

type RevisionMeta struct {
	Author    *string
	Timestamp int64
	APool     *apool.APool
	Atext     *apool.AText
	IsKeyRev  bool
}
