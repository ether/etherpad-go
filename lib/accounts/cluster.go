package accounts

import (
	"errors"
	"time"

	"github.com/ether/etherpad-go/lib/db"
)

// writeLockName is the database lock that serializes changes over all
// instances of a cluster, so the rules of a change (the last admin, no
// escalation) see the state it changes even when two instances change at
// once.
const writeLockName = "accounts-write"

// lockWait is how long a change waits for another instance's change.
var lockWait = 10 * time.Second

// ErrBusy is returned when another instance holds the write lock too long.
var ErrBusy = errors.New("the accounts are being changed elsewhere, try again")

// lockWrites takes the process lock and, when the store has one, the
// database lock; the returned function releases both.
func (m *Manager) lockWrites() (func(), error) {
	m.wmu.Lock()
	locker, ok := m.store.(db.LeaderMethods)
	if !ok {
		return m.wmu.Unlock, nil
	}
	deadline := time.Now().Add(lockWait)
	for wait := 10 * time.Millisecond; ; wait = min(wait*2, 250*time.Millisecond) {
		lease, held, err := locker.TryLeadership(writeLockName)
		if err != nil {
			m.wmu.Unlock()
			return nil, err
		}
		if held {
			// Another instance may have changed the roles a moment ago
			// and its notice not be here yet: decide on what is stored.
			m.mu.Lock()
			m.roles = nil
			m.grants = map[string]cachedGrants{}
			m.people = map[string]cachedPerson{}
			m.gen++
			m.mu.Unlock()
			return func() {
				lease.Release()
				m.wmu.Unlock()
			}, nil
		}
		if time.Now().After(deadline) {
			m.wmu.Unlock()
			return nil, ErrBusy
		}
		time.Sleep(wait)
	}
}

// OnChange sets what runs after every change, e.g. telling the other
// instances of a cluster to forget their caches (Invalidate).
func (m *Manager) OnChange(fn func()) {
	m.mu.Lock()
	m.onChange = fn
	m.mu.Unlock()
}

// changed forgets this process's caches and tells the others.
func (m *Manager) changed() {
	m.Invalidate()
	m.mu.Lock()
	fn := m.onChange
	m.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// lockFor takes the write lock for a change by actor and returns the actor
// as it is now: its account still active, its rights read from what is
// stored. A change that waited for the lock thus cannot use rights taken
// away in the meantime.
func (m *Manager) lockFor(actor Actor) (func(), Actor, error) {
	unlock, err := m.lockWrites()
	if err != nil {
		return nil, Actor{}, err
	}
	if actor.server {
		return unlock, actor, nil
	}
	fresh, err := m.CurrentActor(actor.ID)
	if err != nil {
		unlock()
		return nil, Actor{}, err
	}
	return unlock, fresh, nil
}

// CurrentActor returns the actor of an account with its rights as they are
// now; ErrForbidden for an unknown, disabled or built-in account.
func (m *Manager) CurrentActor(accountID string) (Actor, error) {
	a, err := m.store.GetAccount(accountID)
	if err != nil && !errors.Is(err, db.ErrAccountNotFound) {
		return Actor{}, err
	}
	if a == nil || a.DisabledAt != nil || (a.Kind != KindHuman && a.Kind != KindService) {
		return Actor{}, ErrForbidden
	}
	g, err := m.GrantsFor(a.ID)
	if err != nil {
		return Actor{}, err
	}
	return Actor{ID: a.ID, Grants: g}, nil
}
