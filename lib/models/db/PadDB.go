package db

import "github.com/ether/etherpad-go/lib/models/pad"

type PadDB struct {
	ID     string
	RevNum int
	Pool   *pad.APool
}
