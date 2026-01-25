package db

import "time"

type AuthorDB struct {
	ID        string
	Name      *string
	ColorId   string
	Timestamp int64
	Token     *string
	CreatedAt time.Time
}
