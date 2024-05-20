package db

import "github.com/ether/etherpad-go/lib/apool"

type PadMetaData struct {
	Author    string
	Timestamp int
	atext     apool.AText
}
