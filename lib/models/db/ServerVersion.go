package db

import "time"

type ServerVersion struct {
	Version   string
	UpdatedAt time.Time
}
