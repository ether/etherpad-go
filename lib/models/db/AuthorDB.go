package db

type AuthorDB struct {
	ID        string
	Name      *string
	ColorId   int
	PadIDs    map[string]struct{}
	Timestamp int64
}
