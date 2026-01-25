package db

import (
	"github.com/ether/etherpad-go/lib/models/db"
	session2 "github.com/ether/etherpad-go/lib/models/session"
)

type PadMethods interface {
	DoesPadExist(padID string) (*bool, error)
	RemovePad(padID string) error
	CreatePad(padID string, padDB db.PadDB) error
	GetPadIds() (*[]string, error)
	SaveRevision(padId string, rev int, changeset string, text db.AText, pool db.RevPool, authorId *string, timestamp int64) error
	GetRevision(padId string, rev int) (*db.PadSingleRevision, error)
	RemoveRevisionsOfPad(padId string) error
	GetRevisions(padId string, startRev int, endRev int) (*[]db.PadSingleRevision, error)
	GetPad(padID string) (*db.PadDB, error)
	GetReadonlyPad(padId string) (*string, error)
	SetReadOnlyId(padId string, readOnlyId string) error
	GetPadByReadOnlyId(id string) (*string, error)
	SaveChatHeadOfPad(padId string, head int) error
	QueryPad(offset int, limit int, sortBy string, ascending bool, pattern string) (*db.PadDBSearchResult, error)
}

type PadMetaData interface {
	GetPadMetaData(padId string, revNum int) (*db.PadMetaData, error)
}

type AuthorMethods interface {
	GetAuthor(author string) (*db.AuthorDB, error)
	GetAuthorByToken(token string) (*string, error)
	SetAuthorByToken(token string, author string) error
	SaveAuthor(author db.AuthorDB) error
	SaveAuthorName(authorId string, authorName string) error
	SaveAuthorColor(authorId string, authorColor string) error
}

type SessionMethods interface {
	GetSessionById(sessionID string) (*session2.Session, error)
	SetSessionById(sessionID string, session session2.Session) error
	RemoveSessionById(sessionID string) error
}

type GroupMethods interface {
	GetGroup(groupId string) (*string, error)
	SaveGroup(groupId string) error
	RemoveGroup(groupId string) error
}

type ChatMethods interface {
	RemoveChat(padId string) error
	SaveChatMessage(padId string, head int, authorId *string, timestamp int64, text string) error
	GetChatsOfPad(padId string, start int, end int) (*[]db.ChatMessageDBWithDisplayName, error)
	GetAuthorIdsOfPadChats(id string) (*[]string, error)
}

type DataStore interface {
	PadMethods
	AuthorMethods
	PadMetaData
	SessionMethods
	GroupMethods
	ChatMethods
	Close() error
}
