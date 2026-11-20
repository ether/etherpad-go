package tokens

import (
	"sync"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/apikeys"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A replaced key's grace period can be shortened or lengthened, never past
// the expiry it had, nor to "never".
func TestGraceIsCapped(t *testing.T) {
	e := newEnv(t)
	svc := e.service(t, "moodle", accounts.RoleEditor, "")
	_, old, err := e.tok.Issue(e.root, Spec{AccountID: svc.ID, Name: "Moodle", Scopes: []string{accounts.ScopePadRead},
		ExpiresAt: later(e, 30*24*time.Hour)})
	require.NoError(t, err)
	_, _, err = e.tok.Rotate(e.root, old.ID, later(e, 10*24*time.Hour))
	require.NoError(t, err)
	_, err = e.tok.SetExpiry(e.root, old.ID, later(e, 31*24*time.Hour))
	assert.ErrorIs(t, err, ErrGraceTooLong, "past the expiry it had")
	_, err = e.tok.SetExpiry(e.root, old.ID, nil)
	assert.ErrorIs(t, err, ErrGraceTooLong, "never for ever")
	got, err := e.tok.SetExpiry(e.root, old.ID, later(e, 30*24*time.Hour))
	require.NoError(t, err, "up to the expiry it had")
	assert.True(t, got.ExpiresAt.Equal(e.clock.Add(30*24*time.Hour)))

	// a never-expiring key's grace period has no cap, but is no "never"
	_, never, err := e.tok.Issue(e.root, Spec{AccountID: svc.ID, Name: "never", Scopes: []string{accounts.ScopePadRead}})
	require.NoError(t, err)
	_, _, err = e.tok.Rotate(e.root, never.ID, later(e, 10*24*time.Hour))
	require.NoError(t, err)
	_, err = e.tok.SetExpiry(e.root, never.ID, later(e, 400*24*time.Hour))
	require.NoError(t, err)
	_, err = e.tok.SetExpiry(e.root, never.ID, nil)
	assert.ErrorIs(t, err, ErrGraceTooLong)
}

// Nobody gives a token limits above their own.
func TestTokenLimitsWithinTheIssuers(t *testing.T) {
	e := newEnv(t)
	big, err := e.acc.SaveRole(accounts.System, db.Role{Name: "big", Scopes: []string{accounts.ScopePadRead}, Limits: map[string]int64{"ai.requestsPerDay": 1000}})
	require.NoError(t, err)
	small, err := e.acc.SaveRole(accounts.System, db.Role{Name: "small keeper", Scopes: []string{accounts.ScopeAdminAPIKeys, accounts.ScopePadRead},
		Limits: map[string]int64{"ai.requestsPerDay": 10}})
	require.NoError(t, err)
	svc := e.service(t, "bot", big.ID, "")
	kim := e.person(t, "kim", small.ID, "")
	kimActor := e.actor(t, kim.ID)

	_, _, err = e.tok.Issue(kimActor, Spec{AccountID: svc.ID, Name: "x", Scopes: []string{accounts.ScopePadRead}})
	assert.ErrorIs(t, err, ErrLimitTooHigh, "the token would get the account's 1000")
	_, _, err = e.tok.Issue(kimActor, Spec{AccountID: svc.ID, Name: "x", Scopes: []string{accounts.ScopePadRead}, Limits: map[string]int64{"ai.requestsPerDay": 500}})
	assert.ErrorIs(t, err, ErrLimitTooHigh)
	_, _, err = e.tok.Issue(kimActor, Spec{AccountID: svc.ID, Name: "x", Scopes: []string{accounts.ScopePadRead}, Limits: map[string]int64{"ai.requestsPerDay": 10}})
	require.NoError(t, err, "cut down to her own")
	_, _, err = e.tok.Issue(e.root, Spec{AccountID: svc.ID, Name: "x", Scopes: []string{accounts.ScopePadRead}})
	require.NoError(t, err, "an admin has every right")
}

// two instances on one database
func twoInstances(t *testing.T) (*env, *Manager) {
	e := newEnv(t)
	other := NewManager(e.store, accounts.NewManager(e.store), func() settings.Tokens { return e.policy })
	other.now = e.tok.now
	return e, other
}

func TestRotateAcrossInstances(t *testing.T) {
	e, other := twoInstances(t)
	svc := e.service(t, "moodle", accounts.RoleEditor, "")
	for range 10 {
		_, tok, err := e.tok.Issue(e.root, Spec{AccountID: svc.ID, Name: "Moodle", Scopes: []string{accounts.ScopePadRead}})
		require.NoError(t, err)
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i, m := range []*Manager{e.tok, other} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, errs[i] = m.Rotate(accounts.System, tok.ID, nil)
			}()
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
			} else {
				assert.ErrorIs(t, err, ErrNotActive)
			}
		}
		require.Equal(t, 1, ok, "one rotation, one new key: %v", errs)
		list, err := e.store.ListTokens(svc.ID)
		require.NoError(t, err)
		successors := 0
		for _, x := range list {
			if x.Predecessor == tok.ID {
				successors++
			}
		}
		assert.Equal(t, 1, successors)
	}
}

func TestTakeOverAcrossInstances(t *testing.T) {
	e, other := twoInstances(t)
	keys := apikeys.NewManager(e.store)
	for range 5 {
		_, _, err := keys.Create("script")
		require.NoError(t, err)
	}
	var wg sync.WaitGroup
	counts := make([]int, 2)
	for i, m := range []*Manager{e.tok, other} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := m.TakeOverAPIKeys(e.store)
			assert.NoError(t, err)
			counts[i] = n
		}()
	}
	wg.Wait()
	assert.Equal(t, 5, counts[0]+counts[1], "each key once")
	audit, err := e.acc.Audit(100)
	require.NoError(t, err)
	n := 0
	for _, a := range audit {
		if a.Action == "token.take-over" {
			n++
		}
	}
	assert.Equal(t, 5, n, "one audit entry per key")
}

// Two instances starting at once with different APIKEY.txt files: only
// one key becomes a token, the other instance finds the step done.
func TestImportKeyOnceAcrossInstances(t *testing.T) {
	e, other := twoInstances(t)
	var wg sync.WaitGroup
	results := make([]OnceResult, 2)
	for i, m := range []*Manager{e.tok, other} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := m.ImportKeyOnce("apikey-file-test", "APIKEY.txt", func() (string, error) {
				return []string{"key-of-one", "key-of-two"}[i], nil
			})
			assert.NoError(t, err)
			results[i] = r
		}()
	}
	wg.Wait()
	assert.ElementsMatch(t, []OnceResult{OnceStored, OnceDone}, results)
	list, err := e.tok.List(accounts.System, "")
	require.NoError(t, err)
	n := 0
	for _, tk := range list {
		if tk.Name == "APIKEY.txt" {
			n++
		}
	}
	assert.Equal(t, 1, n, "one key taken over")
	// Later starts take nothing over, whatever the file says.
	r, err := other.ImportKeyOnce("apikey-file-test", "APIKEY.txt", func() (string, error) { return "key-of-three", nil })
	require.NoError(t, err)
	assert.Equal(t, OnceDone, r)
}
