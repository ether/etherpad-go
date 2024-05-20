package db

type ChangesetMeta struct {
	AuthorId  string
	Timestamp int
}

type ChangesetDB struct {
	OldLen   int
	NewLen   int
	Ops      string
	CharBank string
}

type RevisionDB struct {
	RevNum int
	cs     ChangesetDB
	meta   ChangesetMeta
}
