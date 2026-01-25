package db

import "time"

type AuthorDB struct {
	ID        string
	Name      *string
	ColorId   string
	PadIDs    map[string]struct{}
	Timestamp int64
	Token     *string
	CreatedAt time.Time
}
