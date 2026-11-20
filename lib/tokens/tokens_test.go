package tokens

import (
	"strings"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type env struct {
	store  *db.MemoryDataStore
	acc    *accounts.Manager
	tok    *Manager
	policy settings.Tokens
	clock  time.Time
	root   accounts.Actor
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{store: db.NewMemoryDataStore(), clock: time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)}
	e.acc = accounts.NewManager(e.store)
	require.NoError(t, e.acc.EnsureDefaults(accounts.SeedConfig{Template: accounts.TemplateClosed}, nil))
	// Only the own roles count here, not those of everyone logged in.
	require.NoError(t, e.acc.Unassign(accounts.System, accounts.AuthenticatedID, accounts.RoleEditor, ""))
	e.tok = NewManager(e.store, e.acc, func() settings.Tokens { return e.policy })
	e.tok.now = func() time.Time { return e.clock }
	root := e.person(t, "root", accounts.RoleAdmin, "")
	e.root = e.actor(t, root.ID)
	return e
}

func (e *env) person(t *testing.T, name, role, pads string) *db.Account {
	t.Helper()
	a, err := e.acc.CreateAccount(accounts.System, accounts.NewAccount{Username: name, Kind: accounts.KindHuman, Password: name + "-password"})
	require.NoError(t, err)
	if role != "" {
		require.NoError(t, e.acc.Assign(accounts.System, a.ID, role, pads))
	}
	return a
}

func (e *env) service(t *testing.T, name, role, pads string) *db.Account {
	t.Helper()
	a, err := e.acc.CreateAccount(accounts.System, accounts.NewAccount{Username: name, Kind: accounts.KindService})
	require.NoError(t, err)
	if role != "" {
		require.NoError(t, e.acc.Assign(accounts.System, a.ID, role, pads))
	}
	return a
}

func (e *env) actor(t *testing.T, id string) accounts.Actor {
	t.Helper()
	a, err := e.acc.CurrentActor(id)
	require.NoError(t, err)
	return a
}

func later(e *env, d time.Duration) *time.Time {
	x := e.clock.Add(d)
	return &x
}

// The concept's example: the claude service account edits under claude/*;
// its token asks for delete too, which the account lacks.
func TestTokenRightsAreTheAccountsCutDown(t *testing.T) {
	e := newEnv(t)
	claude := e.service(t, "claude", accounts.RoleEditor, "claude/*")
	key, tok, err := e.tok.Issue(e.root, Spec{AccountID: claude.ID, Name: "Claude Desktop",
		Scopes: []string{accounts.ScopePadRead, accounts.ScopePadWrite, accounts.ScopePadCreate, accounts.ScopePadDelete}})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(key, KeyPrefix))
	assert.Len(t, key, len(KeyPrefix)+keyLength)
	assert.Equal(t, key[:shownPrefix], tok.Prefix)
	assert.Equal(t, Hash(key), tok.Hash)
	assert.Nil(t, tok.ExpiresAt, "never, as the policy allows")

	use, err := e.tok.Authenticate(key, "203.0.113.7")
	require.NoError(t, err)
	assert.Equal(t, claude.ID, use.Account.ID)
	assert.True(t, use.Grants.Has(accounts.ScopePadWrite, "claude/todo"))
	assert.False(t, use.Grants.Has(accounts.ScopePadDelete, "claude/todo"), "the account lacks it")
	assert.False(t, use.Grants.Has(accounts.ScopePadRead, "team/notes"), "a service account is not one of everyone logged in")

	// taking the role away makes the token useless at once
	require.NoError(t, e.acc.Unassign(accounts.System, claude.ID, accounts.RoleEditor, "claude/*"))
	use, err = e.tok.Authenticate(key, "")
	require.NoError(t, err)
	assert.False(t, use.Grants.Has(accounts.ScopePadRead, "claude/todo"))

	// a disabled account's tokens stop working
	require.NoError(t, e.acc.SetDisabled(accounts.System, claude.ID, true))
	_, err = e.tok.Authenticate(key, "")
	assert.ErrorIs(t, err, ErrInvalid)

	_, err = e.tok.Authenticate("ep_nope", "")
	assert.ErrorIs(t, err, ErrInvalid)
	_, err = e.tok.Authenticate("  ", "")
	assert.ErrorIs(t, err, ErrInvalid)
}

func TestWhoMayIssue(t *testing.T) {
	e := newEnv(t)
	anna := e.person(t, "anna", accounts.RoleEditor, "team/*")
	bob := e.person(t, "bob", "", "")
	annaActor := e.actor(t, anna.ID)
	spec := Spec{AccountID: anna.ID, Name: "Laptop", Scopes: []string{accounts.ScopePadRead}, PadPattern: "team/*"}

	// own tokens need tokens:own
	_, _, err := e.tok.Issue(annaActor, spec)
	assert.ErrorIs(t, err, ErrForbidden)
	own, err := e.acc.SaveRole(accounts.System, db.Role{Name: "token user", Scopes: []string{accounts.ScopeTokensOwn}})
	require.NoError(t, err)
	require.NoError(t, e.acc.Assign(accounts.System, anna.ID, own.ID, ""))
	annaActor = e.actor(t, anna.ID)
	_, _, err = e.tok.Issue(annaActor, spec)
	require.NoError(t, err)
	_, _, err = e.tok.Issue(annaActor, Spec{AccountID: anna.ID, Name: "x", Scopes: []string{accounts.ScopePadRead}})
	assert.ErrorIs(t, err, ErrEscalation, "she reads team/* only, not all pads")

	// within her own rights only: her pattern, or a narrower one
	_, _, err = e.tok.Issue(annaActor, Spec{AccountID: anna.ID, Name: "x", Scopes: []string{accounts.ScopePadDelete}, PadPattern: "team/*"})
	assert.ErrorIs(t, err, ErrEscalation)
	_, _, err = e.tok.Issue(annaActor, Spec{AccountID: anna.ID, Name: "x", Scopes: []string{accounts.ScopePadWrite}, PadPattern: "team/minutes/*"})
	require.NoError(t, err, "a pattern within her own")
	_, _, err = e.tok.Issue(annaActor, Spec{AccountID: anna.ID, Name: "x", Scopes: []string{accounts.ScopePadWrite}, PadPattern: "other/*"})
	assert.ErrorIs(t, err, ErrEscalation)
	_, _, err = e.tok.Issue(annaActor, Spec{AccountID: anna.ID, Name: "x", Scopes: []string{accounts.AllScopes}})
	assert.ErrorIs(t, err, ErrEscalation, "all scopes need every right")
	// …and never for others
	_, _, err = e.tok.Issue(annaActor, Spec{AccountID: bob.ID, Name: "x", Scopes: []string{accounts.ScopePadRead}})
	assert.ErrorIs(t, err, ErrForbidden)

	// a key manager who is no admin cannot issue for an admin
	keys, err := e.acc.SaveRole(accounts.System, db.Role{Name: "key keeper", Scopes: []string{accounts.ScopeAdminAPIKeys, accounts.ScopePadRead}})
	require.NoError(t, err)
	kim := e.person(t, "kim", keys.ID, "")
	kimActor := e.actor(t, kim.ID)
	rootID := e.root.ID
	_, _, err = e.tok.Issue(kimActor, Spec{AccountID: rootID, Name: "x", Scopes: []string{accounts.ScopePadRead}})
	assert.ErrorIs(t, err, ErrForbidden)
	svc := e.service(t, "backup", "", "")
	_, _, err = e.tok.Issue(kimActor, Spec{AccountID: svc.ID, Name: "x", Scopes: []string{accounts.ScopePadWrite}})
	assert.ErrorIs(t, err, ErrEscalation)
	_, _, err = e.tok.Issue(kimActor, Spec{AccountID: svc.ID, Name: "backup", Scopes: []string{accounts.ScopePadRead}})
	require.NoError(t, err)

	// checks of the request itself
	for want, s := range map[error]Spec{
		ErrNoName:       {AccountID: svc.ID, Name: "  ", Scopes: []string{accounts.ScopePadRead}},
		ErrNoScopes:     {AccountID: svc.ID, Name: "x"},
		ErrUnknownScope: {AccountID: svc.ID, Name: "x", Scopes: []string{"pad:fly"}},
		ErrNoAccount:    {AccountID: accounts.AnonymousID, Name: "x", Scopes: []string{accounts.ScopePadRead}},
		ErrInvalidLimit: {AccountID: svc.ID, Name: "x", Scopes: []string{accounts.ScopePadRead}, Limits: map[string]int64{"a": -1}},
	} {
		_, _, err := e.tok.Issue(e.root, s)
		assert.ErrorIs(t, err, want)
	}
	_, _, err = e.tok.Issue(e.root, Spec{AccountID: svc.ID, Name: strings.Repeat("x", MaxName+1), Scopes: []string{accounts.ScopePadRead}})
	assert.ErrorIs(t, err, ErrNameTooLong)

	// listing: own with tokens:own, all with admin:apikeys
	mine, err := e.tok.List(annaActor, anna.ID)
	require.NoError(t, err)
	assert.Len(t, mine, 2)
	for _, x := range mine {
		assert.Empty(t, x.Hash, "hashes are not handed out")
	}
	_, err = e.tok.List(annaActor, "")
	assert.ErrorIs(t, err, ErrForbidden)
	all, err := e.tok.List(kimActor, "")
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestExpiryPolicy(t *testing.T) {
	e := newEnv(t)
	svc := e.service(t, "moodle", accounts.RoleEditor, "")
	anna := e.person(t, "anna", accounts.RoleEditor, "")
	read := []string{accounts.ScopePadRead}

	e.policy = settings.Tokens{Service: settings.TokenPolicy{MaxLifetimeDays: 90}, Personal: settings.TokenPolicy{RequireExpiry: true}}
	_, _, err := e.tok.Issue(e.root, Spec{AccountID: svc.ID, Name: "x", Scopes: read})
	assert.ErrorIs(t, err, ErrExpiryRequired, "a maximum means no 'never'")
	_, _, err = e.tok.Issue(e.root, Spec{AccountID: svc.ID, Name: "x", Scopes: read, ExpiresAt: later(e, 91*24*time.Hour)})
	assert.ErrorIs(t, err, ErrExpiryTooLate)
	_, _, err = e.tok.Issue(e.root, Spec{AccountID: svc.ID, Name: "x", Scopes: read, ExpiresAt: later(e, -time.Minute)})
	assert.ErrorIs(t, err, ErrExpiryPast)
	_, _, err = e.tok.Issue(e.root, Spec{AccountID: svc.ID, Name: "x", Scopes: read, ExpiresAt: later(e, 90*24*time.Hour)})
	require.NoError(t, err)
	_, _, err = e.tok.Issue(e.root, Spec{AccountID: anna.ID, Name: "x", Scopes: read})
	assert.ErrorIs(t, err, ErrExpiryRequired, "personal tokens have their own rule")
	key, _, err := e.tok.Issue(e.root, Spec{AccountID: anna.ID, Name: "x", Scopes: read, ExpiresAt: later(e, time.Hour)})
	require.NoError(t, err)

	_, err = e.tok.Authenticate(key, "")
	require.NoError(t, err)
	e.clock = e.clock.Add(time.Hour)
	_, err = e.tok.Authenticate(key, "")
	assert.ErrorIs(t, err, ErrInvalid, "expired")
}

func TestRotateWithGracePeriod(t *testing.T) {
	e := newEnv(t)
	svc := e.service(t, "moodle", accounts.RoleEditor, "")
	oldKey, old, err := e.tok.Issue(e.root, Spec{AccountID: svc.ID, Name: "Moodle", Scopes: []string{accounts.ScopePadRead},
		ExpiresAt: later(e, 30*24*time.Hour), PadPattern: "course/*"})
	require.NoError(t, err)

	e.clock = e.clock.Add(24 * time.Hour)
	newKey, fresh, err := e.tok.Rotate(e.root, old.ID, later(e, 10*24*time.Hour))
	require.NoError(t, err)
	assert.NotEqual(t, oldKey, newKey)
	assert.Equal(t, old.ID, fresh.Predecessor)
	assert.Equal(t, "course/*", fresh.PadPattern)
	assert.Equal(t, old.Scopes, fresh.Scopes)
	assert.True(t, fresh.ExpiresAt.Equal(e.clock.Add(30*24*time.Hour)), "as long as the old one was issued for")

	// both work in the grace period
	_, err = e.tok.Authenticate(oldKey, "")
	require.NoError(t, err)
	_, err = e.tok.Authenticate(newKey, "")
	require.NoError(t, err)
	stored, err := e.store.GetToken(old.ID)
	require.NoError(t, err)
	assert.Equal(t, fresh.ID, stored.ReplacedBy)
	assert.True(t, stored.ExpiresAt.Equal(e.clock.Add(10*24*time.Hour)))
	_, _, err = e.tok.Rotate(e.root, old.ID, nil)
	assert.ErrorIs(t, err, ErrNotActive, "a replaced key is not rotated again")

	// the grace period can be shortened, or lengthened up to the policy
	_, err = e.tok.SetExpiry(e.root, old.ID, later(e, time.Hour))
	require.NoError(t, err)
	e.clock = e.clock.Add(time.Hour)
	_, err = e.tok.Authenticate(oldKey, "")
	assert.ErrorIs(t, err, ErrInvalid)
	_, err = e.tok.SetExpiry(e.root, old.ID, later(e, time.Hour))
	assert.ErrorIs(t, err, ErrNotActive, "an ended key stays ended")
	_, err = e.tok.Authenticate(newKey, "")
	require.NoError(t, err)

	// right away: the old key ends now
	k3, third, err := e.tok.Rotate(e.root, fresh.ID, nil)
	require.NoError(t, err)
	_, err = e.tok.Authenticate(newKey, "")
	assert.ErrorIs(t, err, ErrInvalid)
	_, err = e.tok.Authenticate(k3, "")
	require.NoError(t, err)

	// a never-expiring key rotates to a never-expiring one, within the policy
	_, never, err := e.tok.Issue(e.root, Spec{AccountID: svc.ID, Name: "never", Scopes: []string{accounts.ScopePadRead}})
	require.NoError(t, err)
	e.policy.Service.MaxLifetimeDays = 7
	_, rotated, err := e.tok.Rotate(e.root, never.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, rotated.ExpiresAt, "the policy changed since")
	assert.True(t, rotated.ExpiresAt.Equal(e.clock.Add(7*24*time.Hour)))

	// revoked is revoked
	require.NoError(t, e.tok.Revoke(e.root, third.ID))
	_, err = e.tok.Authenticate(k3, "")
	assert.ErrorIs(t, err, ErrInvalid)
	require.NoError(t, e.tok.Revoke(e.root, third.ID), "twice is fine")
	_, _, err = e.tok.Rotate(e.root, third.ID, nil)
	assert.ErrorIs(t, err, ErrNotActive)

	audit, err := e.acc.Audit(100)
	require.NoError(t, err)
	actions := map[string]bool{}
	for _, a := range audit {
		actions[a.Action] = true
	}
	for _, want := range []string{"token.issue", "token.rotate", "token.expiry", "token.revoke"} {
		assert.True(t, actions[want], want)
	}
}

// A key is looked up once and cached, but every change counts at once.
func TestCacheAndLastUse(t *testing.T) {
	e := newEnv(t)
	svc := e.service(t, "moodle", accounts.RoleEditor, "")
	key, tok, err := e.tok.Issue(e.root, Spec{AccountID: svc.ID, Name: "Moodle", Scopes: []string{accounts.ScopePadRead}})
	require.NoError(t, err)
	_, err = e.tok.Authenticate(key, "198.51.100.1")
	require.NoError(t, err)
	stored, _ := e.store.GetToken(tok.ID)
	require.NotNil(t, stored.LastUsedAt)
	assert.Equal(t, "198.51.100.1", stored.LastUsedIP)

	e.clock = e.clock.Add(10 * time.Second)
	_, err = e.tok.Authenticate(key, "198.51.100.2")
	require.NoError(t, err)
	stored, _ = e.store.GetToken(tok.ID)
	assert.Equal(t, "198.51.100.1", stored.LastUsedIP, "written at most every minute")

	// revoked behind the manager's back by another instance: its notice
	// (Invalidate) makes it count at once
	now := e.clock
	stored.RevokedAt = &now
	require.NoError(t, e.store.UpdateToken(*stored))
	_, err = e.tok.Authenticate(key, "")
	require.NoError(t, err, "cached until the notice")
	e.acc.Invalidate()
	_, err = e.tok.Authenticate(key, "")
	assert.ErrorIs(t, err, ErrInvalid)
}
