package accounts

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// racyStore runs during the next account read, between reading and
// returning, what another request would do meanwhile.
type racyStore struct {
	*db.MemoryDataStore
	during func()
}

func (r *racyStore) GetAccount(id string) (*db.Account, error) {
	a, err := r.MemoryDataStore.GetAccount(id)
	if f := r.during; f != nil {
		r.during = nil
		f()
	}
	return a, err
}

// An account disabled while Person reads it is not cached as active.
func TestPersonIsNotCachedAcrossAChange(t *testing.T) {
	inner := db.NewMemoryDataStore()
	store := &racyStore{MemoryDataStore: inner}
	m := NewManager(store)
	require.NoError(t, m.EnsureDefaults(SeedConfig{Template: TemplateOpen}, nil))
	anna, err := m.CreateAccount(System, NewAccount{Username: "anna", Kind: KindHuman, Password: "anna-password"})
	require.NoError(t, err)

	store.during = func() { require.NoError(t, m.SetDisabled(System, anna.ID, true)) }
	a, err := m.Person(anna.ID)
	require.NoError(t, err)
	assert.Nil(t, a, "read again after the change")
	a, err = m.Person(anna.ID)
	require.NoError(t, err)
	assert.Nil(t, a)
}

// AuthorOf checks the account again under the lock.
func TestAuthorOfADisabledAccount(t *testing.T) {
	m, store := newSeeded(t, SeedConfig{Template: TemplateOpen})
	anna, err := m.CreateAccount(System, NewAccount{Username: "anna", Kind: KindHuman, Password: "anna-password"})
	require.NoError(t, err)
	a, err := m.Person(anna.ID) // cached as active
	require.NoError(t, err)
	require.NotNil(t, a)
	// Disabled by another instance whose notice has not arrived yet.
	stored, err := store.GetAccount(anna.ID)
	require.NoError(t, err)
	now := time.Now()
	stored.DisabledAt = &now
	require.NoError(t, store.UpdateAccount(*stored))

	created := false
	_, err = m.AuthorOf(anna.ID, func(string) (string, error) { created = true; return "a.new", nil })
	assert.ErrorIs(t, err, ErrForbidden)
	assert.False(t, created, "no author for a disabled account")
	stored, err = store.GetAccount(anna.ID)
	require.NoError(t, err)
	assert.Empty(t, stored.AuthorID)
}
