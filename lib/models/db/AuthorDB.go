package db

type AuthorDB struct {
	ID        string
	Name      *string
	ColorId   string
	PadIDs    map[string]struct{}
	Timestamp int64
}
