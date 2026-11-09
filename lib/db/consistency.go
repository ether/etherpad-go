package db

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/ether/etherpad-go/lib/models/db"
	"golang.org/x/sync/semaphore"
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
	// Meta is the metadata in the pad row; nil if the pad does not exist.
	// It is read with the version so a writer holding the pad lock always
	// works on the current metadata, even if another instance changed it.
	Meta *PadMeta
}

// PadMeta is the part of the pad row that is not derived from revisions.
type PadMeta struct {
	SavedRevisions []db.SavedRevision
	PublicStatus   bool
	ReadOnlyId     *string
}

// scanPadMeta builds PadMeta from nullable columns of a GetPadVersion query.
func scanPadMeta(exists bool, saved []byte, public *bool, readOnly *string) (*PadMeta, error) {
	if !exists {
		return nil, nil
	}
	meta := &PadMeta{ReadOnlyId: readOnly, SavedRevisions: []db.SavedRevision{}}
	if public != nil {
		meta.PublicStatus = *public
	}
	if len(saved) > 0 && string(saved) != "null" {
		if err := json.Unmarshal(saved, &meta.SavedRevisions); err != nil {
			return nil, fmt.Errorf("saved revisions: %w", err)
		}
	}
	return meta, nil
}

// SheetVersion is what a stored sheet document is at.
type SheetVersion struct {
	Exists bool
	// Head is the revision the stored snapshot is at.
	Head int
	// OpHead is the highest stored op, 0 if none. The ops are the
	// authoritative history: an op is stored before the snapshot, so OpHead
	// can be ahead of Head.
	OpHead int
	// Epoch changes when the workbook is replaced and numbering restarts.
	Epoch int
}

// Latest is the revision the sheet is at.
func (v *SheetVersion) Latest() int { return max(v.Head, v.OpHead) }

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
	// (inclusive, ascending).
	GetRevisionDeltas(padID string, fromRev, toRev int) ([]RevisionDelta, error)
	// GetRevisionPool returns the attribute pool stored with rev.
	GetRevisionPool(padID string, rev int) (*db.RevPool, error)
	// LockPadsExclusive takes the locks of all padIDs, which must be
	// sorted and unique, excluding every other holder on the same
	// database, also in other processes. It takes all or none: on error
	// nothing is held. Datastores that only one process uses lock within
	// the process.
	LockPadsExclusive(padIDs []string) (release func(), err error)
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

// Database locks are session locks: each held lock pins one connection.
// They come from their own small pool, so locks never take the connections
// the work under them needs; with a single shared pool, a pool of one
// connection would wait forever for itself.

// lockPoolSize is the size of the lock pool for a work pool of maxConns:
// the pad lock slots, plus one connection for the leader lock and one for
// the migration lock.
func lockPoolSize(maxConns int) int { return padLockSlots(maxConns) + 2 }

// padLockSlots is how many pad locks a process holds at once. A multi-pad
// operation (copy, move) needs up to two at once.
func padLockSlots(maxConns int) int { return max(4, maxConns/2) }

// lockSlots hands out pad lock slots. A caller locking several pads takes
// all their slots as one unit, so it can never wait for slots it holds
// itself, and two callers cannot each hold part of what the other needs.
type lockSlots struct {
	sem  *semaphore.Weighted
	size int
}

func newLockSlots(size int) *lockSlots {
	return &lockSlots{sem: semaphore.NewWeighted(int64(size)), size: size}
}

// lockAll takes slots for and the locks of names, in the given (sorted)
// order, all or none.
func (s *lockSlots) lockAll(names []string, lockOne func(ctx context.Context, name string) (func(), error)) (func(), error) {
	if len(names) == 0 {
		return func() {}, nil
	}
	if len(names) > s.size {
		return nil, fmt.Errorf("cannot lock %d pads at once (at most %d)", len(names), s.size)
	}
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()
	if err := s.sem.Acquire(ctx, int64(len(names))); err != nil {
		return nil, fmt.Errorf("waiting for a database lock slot: %w", err)
	}
	releases := make([]func(), 0, len(names))
	releaseAll := func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
		s.sem.Release(int64(len(names)))
	}
	for _, name := range names {
		release, err := lockOne(ctx, name)
		if err != nil {
			releaseAll()
			return nil, err
		}
		releases = append(releases, release)
	}
	var once sync.Once
	return func() { once.Do(releaseAll) }, nil
}

func padLockNames(padIDs []string) []string {
	names := make([]string, len(padIDs))
	for i, id := range padIDs {
		names[i] = padLockName(id)
	}
	return names
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

// lockAll locks names in order and returns the function that unlocks them.
func (l *processLocks) lockAll(names []string) func() {
	unlocks := make([]func(), len(names))
	for i, name := range names {
		unlocks[i] = l.lock(name)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			for i := len(unlocks) - 1; i >= 0; i-- {
				unlocks[i]()
			}
		})
	}
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
