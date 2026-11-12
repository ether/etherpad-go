package db

import (
	"fmt"

	"github.com/ether/etherpad-go/lib/models/db"
)

func (m *MemoryDataStore) GetPadVersion(padID string) (*PadVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := &PadVersion{Rev: -1, ChatHead: -1}
	row, exists := m.padStore[padID]
	v.Exists = exists
	if exists {
		v.Meta = &PadMeta{
			SavedRevisions: append([]db.SavedRevision{}, row.SavedRevisions...),
			PublicStatus:   row.PublicStatus,
		}
		if row.Settings != nil {
			v.Meta.Settings = *row.Settings
		}
		if row.ReadOnlyId != nil {
			id := *row.ReadOnlyId
			v.Meta.ReadOnlyId = &id
		}
	}
	if rev, ok := m.maxRev[padID]; ok {
		v.Rev = rev
	}
	if head, ok := m.maxChat[padID]; ok {
		v.ChatHead = head
	}
	return v, nil
}

func (m *MemoryDataStore) GetRevisionDeltas(padID string, fromRev, toRev int) ([]RevisionDelta, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	revs := m.padRevisions[padID]
	var deltas []RevisionDelta
	for r := fromRev; r <= toRev; r++ {
		rev, ok := revs[r]
		if !ok {
			continue
		}
		deltas = append(deltas, RevisionDelta{Rev: r, Changeset: rev.Changeset, AuthorId: rev.AuthorId, Timestamp: rev.Timestamp})
	}
	return deltas, nil
}

func (m *MemoryDataStore) GetRevisionPool(padID string, rev int) (*db.RevPool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.padRevisions[padID][rev]
	if !ok {
		return nil, fmt.Errorf("revision %d not found", rev)
	}
	var pool db.RevPool
	if stored.Pool != nil {
		pool = *stored.Pool
	}
	return &pool, nil
}

// LockPadsExclusive locks within this process: an in-memory store belongs
// to one process.
func (m *MemoryDataStore) LockPadsExclusive(padIDs []string) (func(), error) {
	return m.padLocks.lockAll(padIDs), nil
}

var _ ConsistencyMethods = (*MemoryDataStore)(nil)
