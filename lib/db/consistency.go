package db

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/ether/etherpad-go/lib/models/db"
)

// ErrConflict is returned by SaveRevision and SaveChatMessage when the
// revision or chat message number is already taken. Another writer (for
// instance another Etherpad instance on the same database) got there first;
// the caller must catch up and retry. It is never silently ignored, because
// that would drop a change the client was told was saved.
var ErrConflict = errors.New("already exists")

// PadVersion is what a pad's authoritative state in the database is at.
type PadVersion struct {
	Exists bool
	// Rev is the highest stored revision, -1 if there is none.
	Rev int
	// ChatHead is the highest stored chat message number, -1 if none.
	ChatHead int
}

// RevisionDelta is a stored revision without its full text.
type RevisionDelta struct {
	Rev       int
	Changeset string
	AuthorId  *string
	Timestamp int64
}

// ConsistencyMethods let several Etherpad instances share one database.
// The revisions in padRev are the authoritative pad state; the pad row and
// each instance's cached pad are derived from them.
type ConsistencyMethods interface {
	// GetPadVersion returns the highest stored revision and chat message.
	GetPadVersion(padID string) (*PadVersion, error)
	// GetRevisionDeltas returns the changesets of revisions fromRev..toRev
	// (inclusive, ascending) and the attribute pool stored with toRev.
	GetRevisionDeltas(padID string, fromRev, toRev int) ([]RevisionDelta, *db.RevPool, error)
	// LockPadExclusive takes a lock on padID that excludes every other
	// holder on the same database, also in other processes, and returns
	// the function that releases it. Datastores that only one process can
	// use return a no-op.
	LockPadExclusive(padID string) (release func(), err error)
}

// advisoryLockKey maps a lock name to a 64-bit key for pg_advisory_lock.
// Collisions only make two pads share a lock.
func advisoryLockKey(name string) int64 {
	sum := sha256.Sum256([]byte(name))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}

// mysqlLockName maps a lock name to a GET_LOCK name (at most 64 chars).
func mysqlLockName(name string) string {
	sum := sha256.Sum256([]byte(name))
	return "ep:" + hex.EncodeToString(sum[:24])
}

func padLockName(padID string) string { return "etherpad:pad:" + padID }

// lockSlots bounds how many database locks a process holds at once. Each
// held lock pins a pooled connection, and the work done under the lock needs
// further connections; without a bound, many pads locked at once could take
// every connection and wait forever for one more.
func newLockSlots(maxConns int) chan struct{} {
	return make(chan struct{}, max(1, maxConns/2))
}

// processLocks is the pad lock of datastores that only one process uses
// (Memory, SQLite): a mutex per pad, shared by everything using the store.
type processLocks struct {
	mu    sync.Mutex
	locks map[string]*processLock
}

type processLock struct {
	mu   sync.Mutex
	refs int
}

func newProcessLocks() *processLocks {
	return &processLocks{locks: map[string]*processLock{}}
}

func (l *processLocks) lock(name string) func() {
	l.mu.Lock()
	pl, ok := l.locks[name]
	if !ok {
		pl = &processLock{}
		l.locks[name] = pl
	}
	pl.refs++
	l.mu.Unlock()

	pl.mu.Lock()
	return func() {
		pl.mu.Unlock()
		l.mu.Lock()
		pl.refs--
		if pl.refs == 0 {
			delete(l.locks, name)
		}
		l.mu.Unlock()
	}
}
