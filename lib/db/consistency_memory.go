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
		if row.ReadOnlyId != nil {
			id := *row.ReadOnlyId
			v.Meta.ReadOnlyId = &id
		}
	}
	for rev := range m.padRevisions[padID] {
		v.Rev = max(v.Rev, rev)
	}
	for _, msg := range m.chatPads {
		if msg.PadId == padID {
			v.ChatHead = max(v.ChatHead, msg.Head)
		}
	}
	return v, nil
}

func (m *MemoryDataStore) GetRevisionDeltas(padID string, fromRev, toRev int) ([]RevisionDelta, *db.RevPool, error) {
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
	last, ok := revs[toRev]
	if !ok {
		return nil, nil, fmt.Errorf("revision %d not found", toRev)
	}
	var pool db.RevPool
	if last.Pool != nil {
		pool = *last.Pool
	}
	return deltas, &pool, nil
}

// LockPadExclusive locks within this process: an in-memory store belongs to
// one process.
func (m *MemoryDataStore) LockPadExclusive(padID string) (func(), error) {
	return m.padLocks.lock(padID), nil
}

var _ ConsistencyMethods = (*MemoryDataStore)(nil)
