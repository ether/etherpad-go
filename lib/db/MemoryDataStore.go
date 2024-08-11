package db

import (
	"errors"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/db"
	session2 "github.com/ether/etherpad-go/lib/models/session"
)

type MemoryDataStore struct {
	padStore     map[string]db.PadDB
	authorStore  map[string]db.AuthorDB
	readonly2Pad map[string]string
	pad2Readonly map[string]string
	authorMapper map[string]string
	sessionStore map[string]session2.Session
}

func (m *MemoryDataStore) GetSessionById(sessionID string) *session2.Session {
	var retrievedSession, ok = m.sessionStore[sessionID]

	if !ok {
		return nil
	}

	return &retrievedSession
}

func (m *MemoryDataStore) SetSessionById(sessionID string, session session2.Session) {
	m.sessionStore[sessionID] = session
}

func (m *MemoryDataStore) RemoveSessionById(sessionID string) *session2.Session {
	var retrievedSession, ok = m.sessionStore[sessionID]

	if !ok {
		return nil
	}

	delete(m.sessionStore, sessionID)

	return &retrievedSession
}

func (m *MemoryDataStore) SetAuthorByToken(token string, author string) error {
	//TODO implement me
	panic("implement me")
}

func (m *MemoryDataStore) GetRevision(padId string, rev int) (*db.PadSingleRevision, error) {
	//TODO implement me
	panic("implement me")
}

func (m *MemoryDataStore) GetPadMetaData(padId string, revNum int) (*db.PadMetaData, error) {
	var retrievedPad, ok = m.padStore[padId]

	if !ok {
		panic("Pad not found")
	}
	var rev, found = retrievedPad.SavedRevisions[revNum]

	if !found {
		return nil, errors.New("revision not found")
	}

	return &db.PadMetaData{
		Author:    rev.PadDBMeta.Author,
		Timestamp: rev.PadDBMeta.Timestamp,
	}, nil
}

func (m *MemoryDataStore) GetPadIds() []string {
	var padIds []string
	for k := range m.padStore {
		padIds = append(padIds, k)
	}
	return padIds
}

func NewMemoryDataStore() *MemoryDataStore {
	return &MemoryDataStore{
		padStore:     make(map[string]db.PadDB),
		authorStore:  make(map[string]db.AuthorDB),
		pad2Readonly: make(map[string]string),
		readonly2Pad: make(map[string]string),
		authorMapper: make(map[string]string),
		sessionStore: make(map[string]session2.Session),
	}
}

func (m *MemoryDataStore) DoesPadExist(padID string) bool {
	_, ok := m.padStore[padID]
	return ok
}

func (m *MemoryDataStore) CreatePad(padID string, padDB db.PadDB) bool {
	m.padStore[padID] = padDB
	return true
}

func (m *MemoryDataStore) SaveRevision(padId string, rev int, changeset string,
	text apool.AText, pool apool.APool, authorId *string, timestamp int) error {
	var retrievedPad, ok = m.padStore[padId]
	if !ok {
		panic("Pad not found")
	}
	retrievedPad.RevNum = rev

	retrievedPad.SavedRevisions[rev] = db.PadRevision{
		Content: changeset,
		PadDBMeta: db.PadDBMeta{
			Pool:      &pool,
			AText:     &text,
			Author:    authorId,
			Timestamp: timestamp,
		},
	}
	return nil
}

func (m *MemoryDataStore) GetPad(padID string) (*db.PadDB, error) {
	pad, ok := m.padStore[padID]

	if !ok {
		return nil, errors.New("pad not found")
	}

	return &pad, nil
}

func (m *MemoryDataStore) GetReadonlyPad(padId string) (*string, error) {
	pad, ok := m.padStore[padId]

	if !ok {
		return nil, errors.New("read only id not found")
	}
	return &pad.ReadOnlyId, nil
}

func (m *MemoryDataStore) CreatePad2ReadOnly(padId string, readonlyId string) {
	m.pad2Readonly[padId] = readonlyId
}

func (m *MemoryDataStore) CreateReadOnly2Pad(padId string, readonlyId string) {
	m.readonly2Pad[readonlyId] = padId
}

func (m *MemoryDataStore) GetReadOnly2Pad(id string) *string {
	res, ok := m.readonly2Pad[id]

	if !ok {
		return nil
	}

	return &res
}

func (m *MemoryDataStore) GetAuthor(author string) (*db.AuthorDB, error) {
	retrievedAuthor, ok := m.authorStore[author]

	if !ok {
		return nil, errors.New("Author not found")
	}

	return &retrievedAuthor, nil
}

func (m *MemoryDataStore) GetAuthorByToken(token string) (*string, error) {
	return nil, nil
}

func (m *MemoryDataStore) SaveAuthor(author db.AuthorDB) {
	m.authorStore[author.ID] = author
}

func (m *MemoryDataStore) SaveAuthorName(authorId string, authorName string) {
	var retrievedAuthor, _ = m.authorStore[authorId]
	retrievedAuthor.Name = &authorName
}

func (m *MemoryDataStore) SaveAuthorColor(authorId string, authorColor string) {
	var retrievedAuthor, _ = m.authorStore[authorId]
	retrievedAuthor.ColorId = authorColor
}

var _ DataStore = (*MemoryDataStore)(nil)
