package db

import (
	"errors"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/db"
)

type MemoryDataStore struct {
	padStore     map[string]db.PadDB
	authorStore  map[string]db.AuthorDB
	readonly2Pad map[string]string
	pad2Readonly map[string]string
	authorMapper map[string]string
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
	text apool.AText, pool apool.APool, authorId *string, timestamp int) {
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
}

func (m *MemoryDataStore) GetPad(padID string) (*db.PadDB, error) {
	pad, ok := m.padStore[padID]

	if !ok {
		return nil, errors.New("Pad not found")
	}

	return &pad, nil
}

func (m *MemoryDataStore) GetReadonlyPad(padId string) (*string, error) {
	pad, ok := m.padStore[padId]

	if !ok {
		return nil, nil
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

func (m *MemoryDataStore) GetAuthorByMapperKeyAndMapperValue(key string, value string) (*db.AuthorDB, error) {
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
