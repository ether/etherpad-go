package db

import "github.com/ether/etherpad-go/lib/apool"

type PadMetaData struct {
	Id           string
	RevNum       int
	ChangeSet    string
	Atext        apool.AText
	AtextAttribs string
	AuthorId     *string
	Timestamp    int64
}
