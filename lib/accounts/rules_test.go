package accounts

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// soleAdminThrough makes boss the only admin, through a custom role that
// includes admin (directly or through middle).
func soleAdminThrough(t *testing.T, middle bool) (*Manager, *db.Account, *db.Role, *db.Role) {
	t.Helper()
	m, _ := newSeeded(t, SeedConfig{Template: TemplateClosed})
	var mid *db.Role
	includes := []string{RoleAdmin}
	if middle {
		var err error
		mid, err = m.SaveRole(System, db.Role{Name: "middle", Includes: []string{RoleAdmin}})
		require.NoError(t, err)
		includes = []string{mid.ID}
	}
	wrapper, err := m.SaveRole(System, db.Role{Name: "wrapper", Includes: includes})
	require.NoError(t, err)
	boss, err := m.CreateAccount(System, NewAccount{Username: "boss", Kind: KindHuman, Password: "long enough"})
	require.NoError(t, err)
	require.NoError(t, m.Assign(System, boss.ID, wrapper.ID, ""))
	return m, boss, wrapper, mid
}

func TestEditingARoleKeepsAnAdmin(t *testing.T) {
	m, _, wrapper, _ := soleAdminThrough(t, false)
	wrapper.Includes = nil
	_, err := m.SaveRole(System, *wrapper)
	assert.ErrorIs(t, err, ErrLastAdmin)
}

func TestDeletingAnIncludedRoleKeepsAnAdmin(t *testing.T) {
	m, _, _, mid := soleAdminThrough(t, true)
	assert.ErrorIs(t, m.DeleteRole(System, mid.ID), ErrLastAdmin)
}

func TestDeletingASharedAdminRoleKeepsAnAdmin(t *testing.T) {
	m, _, wrapper, _ := soleAdminThrough(t, false)
	second, err := m.CreateAccount(System, NewAccount{Username: "second", Kind: KindHuman})
	require.NoError(t, err)
	require.NoError(t, m.Assign(System, second.ID, wrapper.ID, ""))
	assert.ErrorIs(t, m.DeleteRole(System, wrapper.ID), ErrLastAdmin, "both admins hold only this role")
}

func TestConcurrentChangesKeepAnAdmin(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateClosed})
	a, _ := m.CreateAccount(System, NewAccount{Username: "a", Kind: KindHuman})
	b, _ := m.CreateAccount(System, NewAccount{Username: "b", Kind: KindHuman})
	require.NoError(t, m.Assign(System, a.ID, RoleAdmin, ""))
	require.NoError(t, m.Assign(System, b.ID, RoleAdmin, ""))
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, id := range []string{a.ID, b.ID} {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			errs[i] = m.SetDisabled(System, id, true)
		}(i, id)
	}
	wg.Wait()
	assert.True(t, errors.Is(errs[0], ErrLastAdmin) != errors.Is(errs[1], ErrLastAdmin), "exactly one is refused: %v", errs)
}

func TestNoTakeOverOfStrongerAccounts(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateClosed, BootstrapUser: "boss", BootstrapPassword: "very secret"})
	mgrRole, err := m.SaveRole(System, db.Role{Name: "user-manager", Scopes: []string{ScopeAdminUsers}, Includes: []string{RoleEditor}})
	require.NoError(t, err)
	mgr, _ := m.CreateAccount(System, NewAccount{Username: "mgr", Kind: KindHuman, Password: "long enough"})
	require.NoError(t, m.Assign(System, mgr.ID, mgrRole.ID, ""))
	actor := Actor{ID: mgr.ID, Grants: grantsOf(t, m, "mgr")}
	boss, _ := m.AccountByUsername("boss")

	assert.ErrorIs(t, m.SetPassword(actor, boss.ID, "taken over!", false), ErrEscalation)
	_, err = m.ResetPassword(actor, boss.ID)
	assert.ErrorIs(t, err, ErrEscalation)
	assert.ErrorIs(t, m.SetDisabled(actor, boss.ID, true), ErrEscalation)
	assert.ErrorIs(t, m.DeleteAccount(actor, boss.ID), ErrEscalation)
	// A weaker account it may manage, and its own password.
	anna, _ := m.CreateAccount(actor, NewAccount{Username: "anna", Kind: KindHuman})
	require.NoError(t, m.Assign(actor, anna.ID, RoleEditor, ""))
	assert.NoError(t, m.SetPassword(actor, anna.ID, "anna's new one", true))
	assert.NoError(t, m.SetPassword(actor, mgr.ID, "my own new one", false))
}

func TestNoHigherLimitsThanOwn(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateClosed})
	editorRole, err := m.SaveRole(System, db.Role{Name: "role-editor", Scopes: []string{ScopeAdminRoles, ScopeAIUse}, Limits: map[string]int64{"ai.requests": 10}})
	require.NoError(t, err)
	ed, _ := m.CreateAccount(System, NewAccount{Username: "ed", Kind: KindHuman})
	require.NoError(t, m.Assign(System, ed.ID, editorRole.ID, ""))
	actor := Actor{ID: ed.ID, Grants: grantsOf(t, m, "ed")}
	_, err = m.SaveRole(actor, db.Role{Name: "more", Scopes: []string{ScopeAIUse}, Limits: map[string]int64{"ai.requests": 1000}})
	assert.ErrorIs(t, err, ErrEscalation)
	_, err = m.SaveRole(actor, db.Role{Name: "unlimited", Scopes: []string{ScopeAIUse}, Limits: map[string]int64{"import.mb": 1}})
	assert.ErrorIs(t, err, ErrEscalation, "a limit it has none of")
	_, err = m.SaveRole(actor, db.Role{Name: "less", Scopes: []string{ScopeAIUse}, Limits: map[string]int64{"ai.requests": 5}})
	assert.NoError(t, err)
}

func TestInvalidTemplateKeepsRights(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateRead})
	require.NoError(t, m.Assign(System, AnonymousID, RoleEditor, "public/*"))
	assert.Error(t, m.ApplyTemplate(System, "wide-open"))
	anon, _ := m.AnonymousGrants()
	assert.Equal(t, LevelCreate, anon.Level("public/x"))
	assert.Equal(t, LevelReadOnly, anon.Level("x"))
}

func TestParallelGuessesAreLimited(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateClosed, BootstrapUser: "boss", BootstrapPassword: "very secret"})
	var wg sync.WaitGroup
	var mu sync.Mutex
	checked := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := m.Authenticate("boss", fmt.Sprintf("guess %d", i))
			if errors.Is(err, ErrInvalidCredentials) {
				mu.Lock()
				checked++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	assert.LessOrEqual(t, checked, maxFailures, "at most %d passwords are tried", maxFailures)
	_, err := m.Authenticate("boss", "very secret")
	assert.ErrorIs(t, err, ErrLocked)
}

func TestThrottleIsBounded(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateClosed})
	now := time.Now()
	m.mu.Lock()
	for i := 0; i < maxThrottled+10; i++ {
		m.throttleLocked(fmt.Sprintf("user-%d", i), now)
	}
	n := len(m.throttle)
	m.mu.Unlock()
	assert.LessOrEqual(t, n, maxThrottled)
}

func TestPasswordMatchesIsNoLoginAttempt(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateClosed, BootstrapUser: "boss", BootstrapPassword: "very secret"})
	boss, _ := m.AccountByUsername("boss")
	for i := 0; i < maxFailures+2; i++ {
		assert.False(t, m.PasswordMatches(boss, "not it"))
	}
	assert.True(t, m.PasswordMatches(boss, "very secret"))
	_, err := m.Authenticate("boss", "very secret")
	assert.NoError(t, err)
}

func TestLegacyImportOnceAndRepaired(t *testing.T) {
	store := db.NewMemoryDataStore()
	m := NewManager(store)
	// A take-over interrupted after creating "writer" without its role.
	require.NoError(t, store.CreateAccount(db.Account{ID: "acc.w", Username: "writer", Kind: KindHuman, CreatedAt: time.Now()}))
	users := map[string]LegacyUser{"writer": {Password: "pw-writer"}, "reader": {Password: "pw-reader", ReadOnly: true}}
	require.NoError(t, m.EnsureDefaults(SeedConfig{Template: TemplateClosed, LegacyUsers: users}, nil))
	assert.Equal(t, LevelCreate, grantsOf(t, m, "writer").Level("p"), "the missing role was given")
	assert.Equal(t, LevelReadOnly, grantsOf(t, m, "reader").Level("p"), "the remaining user was taken over")

	// Deleted later, it does not come back with the next start.
	reader, _ := m.AccountByUsername("reader")
	require.NoError(t, m.DeleteAccount(System, reader.ID))
	require.NoError(t, NewManager(store).EnsureDefaults(SeedConfig{Template: TemplateClosed, LegacyUsers: users}, nil))
	again, _ := m.AccountByUsername("reader")
	assert.Nil(t, again)

	// Users added to settings.users after the first start are not read.
	fresh := db.NewMemoryDataStore()
	require.NoError(t, NewManager(fresh).EnsureDefaults(SeedConfig{Template: TemplateOpen}, nil))
	m2 := NewManager(fresh)
	require.NoError(t, m2.EnsureDefaults(SeedConfig{Template: TemplateOpen, LegacyUsers: users}, nil))
	late, _ := m2.AccountByUsername("writer")
	assert.Nil(t, late)
}

func TestBootstrapFinishesAnInterruptedStart(t *testing.T) {
	store := db.NewMemoryDataStore()
	hash, _ := HashPassword("very secret")
	// Created by a start that stopped before giving the role.
	require.NoError(t, store.CreateAccount(db.Account{ID: "acc.b", Username: "boss", Kind: KindHuman, PasswordHash: hash, MustChangePassword: true, CreatedAt: time.Now()}))
	m := NewManager(store)
	require.NoError(t, m.EnsureDefaults(SeedConfig{Template: TemplateClosed, BootstrapUser: "boss", BootstrapPassword: "very secret"}, nil))
	assert.True(t, grantsOf(t, m, "boss").IsAdmin())

	// An account by that name with its own password is not taken over.
	store2 := db.NewMemoryDataStore()
	own, _ := HashPassword("own password")
	require.NoError(t, store2.CreateAccount(db.Account{ID: "acc.x", Username: "boss", Kind: KindHuman, PasswordHash: own, CreatedAt: time.Now()}))
	m2 := NewManager(store2)
	require.NoError(t, m2.EnsureDefaults(SeedConfig{Template: TemplateClosed, BootstrapUser: "boss", BootstrapPassword: "very secret"}, nil))
	assert.False(t, grantsOf(t, m2, "boss").IsAdmin())
}

func TestBootstrapUsesThePasswordlessAdminOfTheTemplate(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateOpen, BootstrapUser: "admin", BootstrapPassword: "very secret",
		LegacyUsers: map[string]LegacyUser{"admin": {IsAdmin: true}, "user": {}}})
	a, err := m.Authenticate("admin", "very secret")
	require.NoError(t, err)
	assert.True(t, a.MustChangePassword)
	assert.True(t, grantsOf(t, m, "admin").IsAdmin())
}

type failingAudit struct{ *db.MemoryDataStore }

func (failingAudit) AppendAudit(db.AuditEntry) error { return errors.New("disk full") }

func TestAuditFailureIsReported(t *testing.T) {
	m := NewManager(failingAudit{db.NewMemoryDataStore()})
	_, err := m.CreateAccount(System, NewAccount{Username: "anna", Kind: KindHuman})
	assert.ErrorIs(t, err, ErrAuditFailed)
	a, _ := m.AccountByUsername("anna")
	assert.NotNil(t, a, "the change itself was saved")
}
