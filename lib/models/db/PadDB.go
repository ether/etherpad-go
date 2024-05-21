package db

import (
	"github.com/ether/etherpad-go/lib/apool"
)

type PadDB struct {
	ID             string
	RevNum         int
	Pool           *apool.APool
	AText          *apool.AText
	SavedRevisions []string
	ReadOnlyId     string
}
