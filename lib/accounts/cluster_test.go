package accounts

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Two instances on one database: what one changes, both decide on.
func TestLastAdminAcrossInstances(t *testing.T) {
	a, store := newSeeded(t, SeedConfig{Template: TemplateOpen})
	b := NewManager(store)
	var ids []string
	for _, name := range []string{"root1", "root2"} {
		acc, err := a.CreateAccount(System, NewAccount{Username: name, Kind: KindHuman, Password: "long enough password"})
		require.NoError(t, err)
		require.NoError(t, a.Assign(System, acc.ID, RoleAdmin, ""))
		ids = append(ids, acc.ID)
	}
	// b knows both admins before either is disabled
	ok, err := b.IsAdmin(ids[1])
	require.NoError(t, err)
	require.True(t, ok)

	for range 20 {
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i, m := range []*Manager{a, b} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = m.SetDisabled(System, ids[i], true)
			}()
		}
		wg.Wait()
		failed := 0
		for _, err := range errs {
			if err != nil {
				assert.ErrorIs(t, err, ErrLastAdmin)
				failed++
			}
		}
		require.Equal(t, 1, failed, "exactly one admin stays: %v", errs)
		for _, id := range ids {
			require.NoError(t, a.SetDisabled(System, id, false))
		}
	}
}

func TestWriteLockWaitsForTheOtherInstance(t *testing.T) {
	a, store := newSeeded(t, SeedConfig{Template: TemplateOpen})
	lease, held, err := store.TryLeadership(writeLockName)
	require.NoError(t, err)
	require.True(t, held)
	done := make(chan error, 1)
	go func() {
		_, err := a.CreateAccount(System, NewAccount{Username: "late", Kind: KindService})
		done <- err
	}()
	select {
	case <-done:
		t.Fatal("changed while another instance held the lock")
	case <-time.After(100 * time.Millisecond):
	}
	lease.Release()
	require.NoError(t, <-done)

	old := lockWait
	lockWait = 50 * time.Millisecond
	defer func() { lockWait = old }()
	lease, _, _ = store.TryLeadership(writeLockName)
	defer lease.Release()
	_, err = a.CreateAccount(System, NewAccount{Username: "later", Kind: KindService})
	assert.ErrorIs(t, err, ErrBusy)
}

func TestOnChangeTellsTheOthers(t *testing.T) {
	a, store := newSeeded(t, SeedConfig{Template: TemplateOpen})
	b := NewManager(store)
	var calls atomic.Int32
	a.OnChange(func() { calls.Add(1) })
	g, err := b.AnonymousGrants()
	require.NoError(t, err)
	require.True(t, g.Has(ScopePadWrite, "x"))

	require.NoError(t, a.ApplyTemplate(System, TemplateRead))
	assert.Positive(t, calls.Load())
	// what the bus does on the other instance
	b.Invalidate()
	g, err = b.AnonymousGrants()
	require.NoError(t, err)
	assert.False(t, g.Has(ScopePadWrite, "x"))
	assert.True(t, g.Has(ScopePadRead, "x"))

	n := calls.Load()
	_, err = a.UpdateProfile(System, AnonymousID, Profile{})
	assert.ErrorIs(t, err, ErrAnonymousAccount)
	assert.Equal(t, n, calls.Load(), "nothing changed, nothing told")
}

func TestUpdateProfile(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateOpen})
	anna, err := m.CreateAccount(System, NewAccount{Username: "anna", Kind: KindHuman, Password: "long enough password"})
	require.NoError(t, err)
	self := Actor{ID: anna.ID, Grants: grantsOf(t, m, "anna")}
	name, mail := "Anna", "anna@example.org"
	a, err := m.UpdateProfile(self, anna.ID, Profile{DisplayName: &name, Email: &mail})
	require.NoError(t, err)
	assert.Equal(t, "Anna", a.DisplayName)
	assert.Equal(t, "anna@example.org", a.Email)
	newName := "anna2"
	_, err = m.UpdateProfile(self, anna.ID, Profile{Username: &newName})
	assert.ErrorIs(t, err, ErrForbidden, "a username is the admins' to change")
	a, err = m.UpdateProfile(System, anna.ID, Profile{Username: &newName})
	require.NoError(t, err)
	assert.Equal(t, "anna2", a.Username)
	bad := "x:y"
	_, err = m.UpdateProfile(System, anna.ID, Profile{Username: &bad})
	assert.ErrorIs(t, err, ErrInvalidUsername)
}

// A change that waited for the lock uses the rights of now, not those
// read before.
func TestStaleActorIsCheckedAgain(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateOpen})
	var ids []string
	for _, name := range []string{"root1", "root2"} {
		acc, err := m.CreateAccount(System, NewAccount{Username: name, Kind: KindHuman, Password: "long enough password"})
		require.NoError(t, err)
		require.NoError(t, m.Assign(System, acc.ID, RoleAdmin, ""))
		ids = append(ids, acc.ID)
	}
	stale, err := m.CurrentActor(ids[1])
	require.NoError(t, err)
	require.True(t, stale.Grants.IsAdmin())

	require.NoError(t, m.SetDisabled(System, ids[1], true))
	_, err = m.CreateAccount(stale, NewAccount{Username: "late", Kind: KindService})
	assert.ErrorIs(t, err, ErrForbidden, "disabled meanwhile")

	require.NoError(t, m.SetDisabled(System, ids[1], false))
	stale, err = m.CurrentActor(ids[1])
	require.NoError(t, err)
	require.NoError(t, m.Unassign(System, ids[1], RoleAdmin, ""))
	_, err = m.CreateAccount(stale, NewAccount{Username: "late", Kind: KindService})
	assert.ErrorIs(t, err, ErrForbidden, "rights taken away meanwhile")

	_, err = m.CurrentActor(AnonymousID)
	assert.ErrorIs(t, err, ErrForbidden)
	_, err = m.CurrentActor("acc.nope")
	assert.ErrorIs(t, err, ErrForbidden)
}
