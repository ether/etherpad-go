package db

import (
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/db"
	session2 "github.com/ether/etherpad-go/lib/models/session"
)

type PadMethods interface {
	DoesPadExist(padID string) bool
	RemovePad(padID string) error
	CreatePad(padID string, padDB db.PadDB) bool
	GetPadIds() []string
	SaveRevision(padId string, rev int, changeset string, text apool.AText, pool apool.APool, authorId *string, timestamp int) error
	GetRevision(padId string, rev int) (*db.PadSingleRevision, error)
	RemoveRevisionsOfPad(padId string) error
	GetPad(padID string) (*db.PadDB, error)
	GetReadonlyPad(padId string) (*string, error)
	CreatePad2ReadOnly(padId string, readonlyId string)
	CreateReadOnly2Pad(padId string, readonlyId string)
	GetReadOnly2Pad(id string) *string
	RemoveReadOnly2Pad(id string) error
	RemovePad2ReadOnly(id string) error
}

type PadMetaData interface {
	GetPadMetaData(padId string, revNum int) (*db.PadMetaData, error)
}

type AuthorMethods interface {
	GetAuthor(author string) (*db.AuthorDB, error)
	GetAuthorByToken(token string) (*string, error)
	SetAuthorByToken(token string, author string) error
	SaveAuthor(author db.AuthorDB)
	SaveAuthorName(authorId string, authorName string)
	SaveAuthorColor(authorId string, authorColor string)
}

type SessionMethods interface {
	GetSessionById(sessionID string) *session2.Session
	SetSessionById(sessionID string, session session2.Session)
	RemoveSessionById(sessionID string) *session2.Session
}

type GroupMethods interface {
	GetGroup(groupId string) (*string, error)
}

type ChatMethods interface {
	RemoveChat(padId string) error
}

type DataStore interface {
	PadMethods
	AuthorMethods
	PadMetaData
	SessionMethods
	GroupMethods
	ChatMethods
}
