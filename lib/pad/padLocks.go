package pad

import (
	"errors"
	"sort"
	"sync"

	"github.com/ether/etherpad-go/lib/models/pad"
	"go.uber.org/zap"
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
//
// Besides the lock within this process it takes the pad's lock in the
// database, which excludes writers in other Etherpad instances on the same
// database, and then brings the cached pads up to the stored state. Writes
// are safe even if the database lock fails: storing a revision whose number
// is taken fails with db.ErrConflict instead of dropping the change.
func (m *Manager) LockPad(ids ...string) (unlock func()) {
	ids = sortedUnique(ids)
	unlockLocal := m.lock(ids, false)
	releases := make([]func(), 0, len(ids))
	for _, id := range ids {
		release, err := m.store.LockPadExclusive(id)
		if err != nil {
			zap.S().Errorw("could not take the database lock of a pad; relying on conflict detection", "pad", id, "error", err)
			continue
		}
		releases = append(releases, release)
	}
	for _, id := range ids {
		m.refreshCached(id)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			for i := len(releases) - 1; i >= 0; i-- {
				releases[i]()
			}
			unlockLocal()
		})
	}
}

// RLockPad takes the read lock of every given pad and returns a function that
// releases them. Use it around reads of a live pad's text, pool or revisions.
// Cached pads are brought up to the stored state first, so readers see what
// other instances wrote.
func (m *Manager) RLockPad(ids ...string) (unlock func()) {
	ids = sortedUnique(ids)
	for _, id := range ids {
		if m.globalPadCache.GetPad(id) == nil {
			continue
		}
		unlockOne := m.lock([]string{id}, false)
		m.refreshCached(id)
		unlockOne()
	}
	return m.lock(ids, true)
}

// refreshCached brings the cached pad id, if any, up to the stored state,
// and drops it from the cache if it was deleted. The caller holds the pad's
// write lock.
func (m *Manager) refreshCached(id string) {
	cached := m.globalPadCache.GetPad(id)
	if cached == nil {
		return
	}
	if err := cached.Refresh(); err != nil {
		if errors.Is(err, pad.ErrPadGone) {
			m.globalPadCache.DeletePad(id)
			return
		}
		zap.S().Warnw("could not refresh a cached pad", "pad", id, "error", err)
	}
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

// CatchUp brings the cached pad id up to the stored state under the pad's
// local write lock and calls fn with it, to send its clients what other
// instances stored. It reports false if the pad is not cached here (then
// it has no clients here) or was deleted.
func (m *Manager) CatchUp(id string, fn func(p *pad.Pad)) bool {
	unlock := m.lock([]string{id}, false)
	defer unlock()
	m.refreshCached(id)
	cached := m.globalPadCache.GetPad(id)
	if cached == nil {
		return false
	}
	fn(cached)
	return true
}
