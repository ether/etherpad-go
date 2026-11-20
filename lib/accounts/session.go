package accounts

import (
	"errors"
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
	m.mu.Lock()
	if c, ok := m.people[accountID]; ok && m.now().Sub(c.at) < CacheTTL {
		m.mu.Unlock()
		return c.account, nil
	}
	m.mu.Unlock()
	a, err := m.store.GetAccount(accountID)
	if err != nil && !errors.Is(err, db.ErrAccountNotFound) {
		return nil, err
	}
	if a != nil && (a.Kind != KindHuman || a.DisabledAt != nil) {
		a = nil
	}
	m.mu.Lock()
	if len(m.people) >= maxCachedPeople {
		m.people = map[string]cachedPerson{}
	}
	m.people[accountID] = cachedPerson{account: a, at: m.now()}
	m.mu.Unlock()
	return a, nil
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
	if err != nil {
		return "", err
	}
	if a == nil {
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
