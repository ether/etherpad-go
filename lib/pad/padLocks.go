package pad

import (
	"errors"
	"fmt"
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

// ErrPadUnavailable is returned by LockPad and RLockPad when the pad cannot
// be locked or brought up to the stored state, typically because the
// database is unreachable. The caller must not touch the pad then: its cached
// copy may be behind what other instances stored.
var ErrPadUnavailable = errors.New("pad temporarily unavailable")

// LockPad takes the write lock of every given pad and returns a function that
// releases them. Use it around any change to a pad.
//
// Besides the lock within this process it takes the pads' locks in the
// database, all or none, which excludes writers in other Etherpad instances
// on the same database, and then brings the cached pads up to the stored
// state. If any of that fails nothing is held and the error wraps
// ErrPadUnavailable.
func (m *Manager) LockPad(ids ...string) (unlock func(), err error) {
	ids = sortedUnique(ids)
	unlockLocal := m.lock(ids, false)
	release, err := m.store.LockPadsExclusive(ids)
	if err != nil {
		unlockLocal()
		return nil, fmt.Errorf("%w: locking %v: %v", ErrPadUnavailable, ids, err)
	}
	m.markDBLocked(ids, 1)
	for _, id := range ids {
		if err := m.refreshCached(id); err != nil {
			m.markDBLocked(ids, -1)
			release()
			unlockLocal()
			return nil, err
		}
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			m.markDBLocked(ids, -1)
			release()
			unlockLocal()
		})
	}, nil
}

// maxReadRefreshes bounds how often RLockPad refreshes a pad that keeps
// changing; after that the reader gets the state of its last refresh.
const maxReadRefreshes = 2

// RLockPad takes the read lock of every given pad and returns a function that
// releases them. Use it around reads of a live pad's text, pool or revisions.
// Readers share the lock: a cached pad is compared with the stored version
// under the read lock, and only a pad that is behind (another instance wrote
// to it) is refreshed under the write lock first.
func (m *Manager) RLockPad(ids ...string) (unlock func(), err error) {
	ids = sortedUnique(ids)
	for round := 0; ; round++ {
		unlockRead := m.lock(ids, true)
		stale, err := m.staleCached(ids)
		if err != nil {
			unlockRead()
			return nil, err
		}
		if len(stale) == 0 || round == maxReadRefreshes {
			return unlockRead, nil
		}
		unlockRead()
		for _, id := range stale {
			unlockOne := m.lock([]string{id}, false)
			err := m.refreshCached(id)
			unlockOne()
			if err != nil {
				return nil, err
			}
		}
	}
}

// staleCached returns the ids whose cached pad differs from the stored
// state. The caller holds the pads' read locks.
func (m *Manager) staleCached(ids []string) ([]string, error) {
	var stale []string
	for _, id := range ids {
		cached := m.globalPadCache.GetPad(id)
		if cached == nil {
			continue
		}
		v, err := m.store.GetPadVersion(id)
		if err != nil {
			return nil, fmt.Errorf("%w: reading the version of %s: %v", ErrPadUnavailable, id, err)
		}
		if !cached.MatchesVersion(v) {
			stale = append(stale, id)
		}
	}
	return stale, nil
}

// refreshCached brings the cached pad id, if any, up to the stored state,
// and drops it from the cache if it was deleted. If the refresh fails the
// pad is dropped from the cache as well, so the next access loads it from
// the store instead of using a copy that may be behind, and the error is
// returned. The caller holds the pad's write lock.
func (m *Manager) refreshCached(id string) error {
	cached := m.globalPadCache.GetPad(id)
	if cached == nil {
		return nil
	}
	if err := cached.Refresh(); err != nil {
		m.globalPadCache.DeletePad(id)
		if errors.Is(err, pad.ErrPadGone) {
			return nil
		}
		zap.S().Warnw("could not refresh a cached pad", "pad", id, "error", err)
		return fmt.Errorf("%w: refreshing %s: %v", ErrPadUnavailable, id, err)
	}
	return nil
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
	if err := m.refreshCached(id); err != nil {
		return false
	}
	cached := m.globalPadCache.GetPad(id)
	if cached == nil {
		return false
	}
	fn(cached)
	return true
}

// UnavailableStatus is the HTTP status for ErrPadUnavailable: the client may
// retry, and a load balancer may send the retry to another instance.
const UnavailableStatus = 503
