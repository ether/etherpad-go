package pad

type DefaultContent struct {
	Type     *string
	Content  *string
	Pad      *Pad
	AuthorId *string
	PadId    string
}

type Load struct {
	Pad   *Pad
	PadId string
}

// Update is the context passed to the padUpdate hook after a revision is appended.
type Update struct {
	Pad       *Pad
	PadId     string
	AuthorId  string
	Revs      int
	Changeset string
}

// Create is the context passed to the padCreate hook right after a pad's first
// revision is persisted.
type Create struct {
	Pad      *Pad
	PadId    string
	AuthorId string
}
