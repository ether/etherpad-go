package db

import (
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/db"
)

type MemoryDataStore struct {
	padStore     map[string]db.PadDB
	authorStore  map[string]db.AuthorDB
	readonly2Pad map[string]string
	pad2Readonly map[string]string
}

func (m MemoryDataStore) GetPadIds() []string {
	//TODO implement me
	panic("implement me")
}

func NewMemoryDataStore() *MemoryDataStore {
	return &MemoryDataStore{
		padStore:     make(map[string]db.PadDB),
		authorStore:  make(map[string]db.AuthorDB),
		pad2Readonly: make(map[string]string),
		readonly2Pad: make(map[string]string),
	}
}

func (m MemoryDataStore) DoesPadExist(padID string) bool {
	_, ok := m.padStore[padID]
	return ok
}

func (m MemoryDataStore) CreatePad(padID string) bool {
	//TODO implement me
	panic("implement me")
}

func (m MemoryDataStore) SaveRevision(padId string, rev int, changeset string, text apool.APool) {
	//TODO implement me
	panic("implement me")
}

func (m MemoryDataStore) GetPad(padID string) (db.PadDB, error) {
	//TODO implement me
	panic("implement me")
}

func (m MemoryDataStore) GetReadonlyPad(padId string) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (m MemoryDataStore) CreatePad2ReadOnly(padId string, readonlyId string) {
	//TODO implement me
	panic("implement me")
}

func (m MemoryDataStore) CreateReadOnly2Pad(padId string, readonlyId string) {
	//TODO implement me
	panic("implement me")
}

func (m MemoryDataStore) GetReadOnly2Pad(id string) string {
	//TODO implement me
	panic("implement me")
}

func (m MemoryDataStore) GetAuthor(author string) (db.AuthorDB, error) {
	//TODO implement me
	panic("implement me")
}

func (m MemoryDataStore) GetAuthorByMapperKeyAndMapperValue(key string, value string) (db.AuthorDB, error) {
	//TODO implement me
	panic("implement me")
}

func (m MemoryDataStore) SaveAuthor(author db.AuthorDB) {
	//TODO implement me
	panic("implement me")
}

func (m MemoryDataStore) SaveAuthorName(authorId string, authorName string) {
	//TODO implement me
	panic("implement me")
}

func (m MemoryDataStore) SaveAuthorColor(authorId string, authorColor string) {
	//TODO implement me
	panic("implement me")
}

var _ DataStore = (*MemoryDataStore)(nil)
