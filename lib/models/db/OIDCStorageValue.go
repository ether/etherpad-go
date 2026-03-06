package db

import "time"

type OIDCStorageValue struct {
	Key       string
	Payload   string
	UpdatedAt time.Time
}
