package accounts

import (
	"errors"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/db"
)

type cachedPerson struct {
	account *db.Account
	at      time.Time
}

// maxCachedPeople bounds the cache of Person; past it the cache starts over.
const maxCachedPeople = 10000

// Person returns the account of a logged-in person if it may still be
// logged in: it exists, is a human and is not disabled; else nil. It is
// cached like the grants (CacheTTL, forgotten on every change and on the
// other instances' notices), so a session costs no database read per
// request.
func (m *Manager) Person(accountID string) (*db.Account, error) {
	if accountID == "" || isPseudo(accountID) {
		return nil, nil
	}
	for attempt := 0; ; attempt++ {
		m.mu.Lock()
		if c, ok := m.people[accountID]; ok && m.now().Sub(c.at) < CacheTTL {
			m.mu.Unlock()
			return c.account, nil
		}
		gen := m.gen
		m.mu.Unlock()
		a, err := m.store.GetAccount(accountID)
		if err != nil && !errors.Is(err, db.ErrAccountNotFound) {
			return nil, err
		}
		if a != nil && (a.Kind != KindHuman || a.DisabledAt != nil) {
			a = nil
		}
		m.mu.Lock()
		if m.gen != gen {
			// Changed while reading: what was read may be stale (an
			// account disabled meanwhile). Read again; never cache it.
			m.mu.Unlock()
			if attempt < 2 {
				continue
			}
			return a, nil
		}
		if len(m.people) >= maxCachedPeople {
			m.people = map[string]cachedPerson{}
		}
		m.people[accountID] = cachedPerson{account: a, at: m.now()}
		m.mu.Unlock()
		return a, nil
	}
}

// AuthorOf returns the pad author of a person, creating and linking one on
// first use with create (which gets the display name). The link is made
// under the write lock, so two instances do not give one account two
// authors.
func (m *Manager) AuthorOf(accountID string, create func(name string) (string, error)) (string, error) {
	if a, err := m.Person(accountID); err != nil {
		return "", err
	} else if a == nil {
		return "", ErrForbidden
	} else if a.AuthorID != "" {
		return a.AuthorID, nil
	}
	unlock, err := m.lockWrites()
	if err != nil {
		return "", err
	}
	defer unlock()
	a, err := m.store.GetAccount(accountID)
	if err != nil && !errors.Is(err, db.ErrAccountNotFound) {
		return "", err
	}
	// Checked again under the lock: disabled meanwhile, it gets nothing.
	if a == nil || a.Kind != KindHuman || a.DisabledAt != nil {
		return "", ErrForbidden
	}
	if a.AuthorID != "" {
		return a.AuthorID, nil
	}
	name := a.DisplayName
	if name == "" {
		name = a.Username
	}
	authorID, err := create(name)
	if err != nil {
		return "", err
	}
	a.AuthorID = authorID
	if err := m.store.UpdateAccount(*a); err != nil {
		return "", err
	}
	m.changed()
	return authorID, nil
}

// Holder returns the account a token may act for: a person or a service
// account, existing and not disabled; else nil. Cached like Person.
func (m *Manager) Holder(accountID string) (*db.Account, error) {
	if accountID == "" || isPseudo(accountID) {
		return nil, nil
	}
	m.mu.Lock()
	if c, ok := m.holders[accountID]; ok && m.now().Sub(c.at) < CacheTTL {
		m.mu.Unlock()
		return c.account, nil
	}
	gen := m.gen
	m.mu.Unlock()
	a, err := m.store.GetAccount(accountID)
	if err != nil && !errors.Is(err, db.ErrAccountNotFound) {
		return nil, err
	}
	if a != nil && ((a.Kind != KindHuman && a.Kind != KindService) || a.DisabledAt != nil) {
		a = nil
	}
	m.mu.Lock()
	if m.gen == gen {
		if len(m.holders) >= maxCachedPeople {
			m.holders = map[string]cachedPerson{}
		}
		m.holders[accountID] = cachedPerson{account: a, at: m.now()}
	}
	m.mu.Unlock()
	return a, nil
}

// Generation counts the times the caches were forgotten (every change, and
// every notice of another instance): what is cached with an older one is
// stale.
func (m *Manager) Generation() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.gen
}

// Changed tells this and the other instances that something the rights
// depend on changed outside the manager (e.g. a token was revoked).
func (m *Manager) Changed() { m.changed() }

// Record writes an audit entry for a change made outside the manager.
func (m *Manager) Record(actor Actor, action, target string, details any) error {
	return m.audit(actor, action, target, details)
}

// MayManage reports whether actor may act on the account: itself, or an
// account whose rights the actor all has (no take-over of more rights).
func (m *Manager) MayManage(actor Actor, accountID string) (bool, error) {
	if actor.server {
		return true, nil
	}
	st, err := m.currentState()
	if err != nil {
		return false, err
	}
	return m.canManage(actor, st, accountID), nil
}

// IsServer reports whether the actor is the server itself (System or the
// CLI), not an account.
func (a Actor) IsServer() bool { return a.server }

// MayGive reports whether grants may pass scope on for pads matching
// pattern ("" or "*": all pads): nobody gives what they do not have. "*"
// (all scopes) needs every right.
func (g Grants) MayGive(scope, pattern string) bool {
	if scope == AllScopes {
		return g.isAll()
	}
	if pattern == "*" {
		pattern = ""
	}
	if g.HasGlobal(scope) {
		return true
	}
	if pattern == "" || !padScoped(scope) {
		return false
	}
	return g.hasForPattern(scope, pattern)
}

// within reports whether every pad matching pattern also matches outer:
// the same pattern, or outer is a fixed beginning followed by "*" that
// pattern starts with.
func within(pattern, outer string) bool {
	if pattern == outer {
		return true
	}
	prefix, ok := strings.CutSuffix(outer, "*")
	return ok && !strings.Contains(prefix, "*") && strings.HasPrefix(pattern, prefix)
}

// hasForPattern reports whether an assignment whose pattern covers this
// pattern gives scope.
func (g Grants) hasForPattern(scope, pattern string) bool {
	for _, a := range g.alts {
		if a.hasForPattern(scope, pattern) {
			return true
		}
	}
	if g.inner != nil {
		return g.restrict.scope(scope) && (g.restrict.pattern == "" || within(pattern, g.restrict.pattern)) &&
			g.inner.hasForPattern(scope, pattern)
	}
	for _, gr := range g.grants {
		if within(pattern, gr.pattern) && gr.has(scope) {
			return true
		}
	}
	return false
}

// KnownScope reports whether name is a registered scope.
func KnownScope(name string) bool {
	for _, s := range scopeNames() {
		if s == name {
			return true
		}
	}
	return false
}
