package revision

import "github.com/ether/etherpad-go/lib/apool"

type SavedRevision struct {
	RevNum    int
	SavedBy   string
	Timestamp int64
	Label     *string
	Id        string
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
