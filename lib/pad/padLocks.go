package pad

import (
	"sort"
	"sync"
)

// padLocks serialises access to pads by pad ID.
//
// Every code path that changes a pad (USER_CHANGES, the HTTP API, imports,
// chat, admin actions, retention) must hold the pad's write lock for its whole
// read-modify-write sequence: a changeset is rebased against the current head
// and must be appended before anyone else appends, and pad.Pool holds maps
// that must never be written concurrently. Paths that only read a live pad
// (exports, GET API calls, timeslider requests) take the read lock.
//
// Locks are keyed by ID rather than stored on *Pad because the cached *Pad for
// an ID is replaced on import, copy and unload. Entries are reference counted
// and removed once unused, so the map does not grow with every pad ever seen.
//
// Lock only at entry points, never inside helpers they call: the locks are
// not reentrant.
type padLocks struct {
	mu    sync.Mutex
	locks map[string]*padLock
}

type padLock struct {
	rw   sync.RWMutex
	refs int
}

func newPadLocks() *padLocks {
	return &padLocks{locks: make(map[string]*padLock)}
}

func (l *padLocks) acquire(id string) *padLock {
	l.mu.Lock()
	defer l.mu.Unlock()
	pl, ok := l.locks[id]
	if !ok {
		pl = &padLock{}
		l.locks[id] = pl
	}
	pl.refs++
	return pl
}

func (l *padLocks) release(id string, pl *padLock) {
	l.mu.Lock()
	defer l.mu.Unlock()
	pl.refs--
	if pl.refs == 0 {
		delete(l.locks, id)
	}
}

// size reports how many pad IDs currently have a lock entry (for tests).
func (l *padLocks) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.locks)
}

// sortedUnique returns the distinct non-empty ids in a fixed order, so that
// callers locking several pads (copy, move) can't deadlock each other.
func sortedUnique(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// LockPad takes the write lock of every given pad and returns a function that
// releases them. Use it around any change to a pad.
func (m *Manager) LockPad(ids ...string) (unlock func()) {
	return m.lock(ids, false)
}

// RLockPad takes the read lock of every given pad and returns a function that
// releases them. Use it around reads of a live pad's text, pool or revisions.
func (m *Manager) RLockPad(ids ...string) (unlock func()) {
	return m.lock(ids, true)
}

func (m *Manager) lock(ids []string, read bool) func() {
	ids = sortedUnique(ids)
	held := make([]*padLock, len(ids))
	for i, id := range ids {
		pl := m.padLocks.acquire(id)
		if read {
			pl.rw.RLock()
		} else {
			pl.rw.Lock()
		}
		held[i] = pl
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			for i := len(ids) - 1; i >= 0; i-- {
				if read {
					held[i].rw.RUnlock()
				} else {
					held[i].rw.Unlock()
				}
				m.padLocks.release(ids[i], held[i])
			}
		})
	}
}
