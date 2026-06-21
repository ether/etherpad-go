package db

import (
	"errors"
	"time"

	"github.com/ether/etherpad-go/lib/models/db"
)

func (m *MemoryDataStore) SaveSheet(padId string, head int, snapshot string) error {
	now := time.Now()
	created := now
	if existing, ok := m.sheetStore[padId]; ok {
		created = existing.CreatedAt
	}
	m.sheetStore[padId] = db.SheetDB{ID: padId, Head: head, Snapshot: snapshot, CreatedAt: created, UpdatedAt: &now}
	return nil
}

func (m *MemoryDataStore) GetSheet(padId string) (*db.SheetDB, error) {
	s, ok := m.sheetStore[padId]
	if !ok {
		return nil, errors.New(SheetDoesNotExistError)
	}
	return &s, nil
}

func (m *MemoryDataStore) DoesSheetExist(padId string) (*bool, error) {
	_, ok := m.sheetStore[padId]
	return &ok, nil
}

func (m *MemoryDataStore) RemoveSheet(padId string) error {
	delete(m.sheetStore, padId)
	delete(m.sheetOps, padId)
	return nil
}

func (m *MemoryDataStore) SaveSheetOp(padId string, rev int, op string, authorId *string, timestamp int64) error {
	if m.sheetOps[padId] == nil {
		m.sheetOps[padId] = make(map[int]db.SheetOpDB)
	}
	if _, exists := m.sheetOps[padId][rev]; exists {
		return nil // write-once
	}
	m.sheetOps[padId][rev] = db.SheetOpDB{PadId: padId, Rev: rev, Op: op, AuthorId: authorId, Timestamp: timestamp}
	return nil
}

func (m *MemoryDataStore) GetSheetOps(padId string, startRev int, endRev int) (*[]db.SheetOpDB, error) {
	out := make([]db.SheetOpDB, 0)
	for r := startRev; r <= endRev; r++ {
		if op, ok := m.sheetOps[padId][r]; ok {
			out = append(out, op)
		}
	}
	return &out, nil
}
