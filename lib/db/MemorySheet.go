package db

import (
	"errors"
	"time"

	"github.com/ether/etherpad-go/lib/models/db"
)

func (m *MemoryDataStore) SaveSheet(padId string, head int, snapshot string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	created := now
	if existing, ok := m.sheetStore[padId]; ok {
		created = existing.CreatedAt
	}
	m.sheetStore[padId] = db.SheetDB{ID: padId, Head: head, Snapshot: snapshot, CreatedAt: created, UpdatedAt: &now}
	return nil
}

func (m *MemoryDataStore) GetSheet(padId string) (*db.SheetDB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sheetStore[padId]
	if !ok {
		return nil, errors.New(SheetDoesNotExistError)
	}
	return &s, nil
}

func (m *MemoryDataStore) DoesSheetExist(padId string) (*bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.sheetStore[padId]
	return &ok, nil
}

func (m *MemoryDataStore) RemoveSheet(padId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sheetStore, padId)
	delete(m.sheetOps, padId)
	delete(m.sheetEpoch, padId)
	return nil
}

func (m *MemoryDataStore) GetSheetVersion(padId string) (*SheetVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sheetStore[padId]
	if !ok {
		return &SheetVersion{}, nil
	}
	v := &SheetVersion{Exists: true, Head: s.Head, Epoch: m.sheetEpoch[padId]}
	for rev := range m.sheetOps[padId] {
		v.OpHead = max(v.OpHead, rev)
	}
	return v, nil
}

func (m *MemoryDataStore) ResetSheet(padId string, snapshot string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	created := now
	if existing, ok := m.sheetStore[padId]; ok {
		created = existing.CreatedAt
	}
	delete(m.sheetOps, padId)
	m.sheetStore[padId] = db.SheetDB{ID: padId, Head: 0, Snapshot: snapshot, CreatedAt: created, UpdatedAt: &now}
	if m.sheetEpoch == nil {
		m.sheetEpoch = map[string]int{}
	}
	m.sheetEpoch[padId]++
	return nil
}

func (m *MemoryDataStore) SaveSheetOp(padId string, rev int, op string, authorId *string, timestamp int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sheetOps[padId] == nil {
		m.sheetOps[padId] = make(map[int]db.SheetOpDB)
	}
	if _, exists := m.sheetOps[padId][rev]; exists {
		return ErrConflict // write-once
	}
	m.sheetOps[padId][rev] = db.SheetOpDB{PadId: padId, Rev: rev, Op: op, AuthorId: copyStringPtr(authorId), Timestamp: timestamp}
	return nil
}

func (m *MemoryDataStore) RemoveSheetOps(padId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sheetOps, padId)
	return nil
}

func (m *MemoryDataStore) GetSheetOps(padId string, startRev int, endRev int) (*[]db.SheetOpDB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := make([]db.SheetOpDB, 0)
	for r := startRev; r <= endRev; r++ {
		if op, ok := m.sheetOps[padId][r]; ok {
			out = append(out, op)
		}
	}
	return &out, nil
}
