package db

import (
	"github.com/ether/etherpad-go/lib/apool"
)

type PadDB struct {
	ID             string
	RevNum         int
	SavedRevisions map[int]PadRevision
	ReadOnlyId     string
}

type PadRevision struct {
	Content   string
	PadDBMeta PadDBMeta
}

type PadDBMeta struct {
	Author    *string
	Timestamp int
	Pool      *apool.APool
	AText     *apool.AText
}
