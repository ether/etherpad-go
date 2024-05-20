package db

import (
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/db"
)

type PadMethods interface {
	DoesPadExist(padID string) bool
	CreatePad(padID string) bool
	GetPadIds() []string
	SaveRevision(padId string, rev int, changeset string, text apool.APool)
	GetPad(padID string) (db.PadDB, error)
	GetReadonlyPad(padId string) (string, error)
	CreatePad2ReadOnly(padId string, readonlyId string)
	CreateReadOnly2Pad(padId string, readonlyId string)
	GetReadOnly2Pad(id string) string
}

type PadMetaData interface {
	GetPadMetaData(padId string, revNum int) (db.PadMetaData, error)
}

type AuthorMethods interface {
	GetAuthor(author string) (db.AuthorDB, error)
	GetAuthorByMapperKeyAndMapperValue(key string, value string) (db.AuthorDB, error)
	SaveAuthor(author db.AuthorDB)
	SaveAuthorName(authorId string, authorName string)
	SaveAuthorColor(authorId string, authorColor string)
}

type DataStore interface {
	PadMethods
	AuthorMethods
	PadMetaData
}
