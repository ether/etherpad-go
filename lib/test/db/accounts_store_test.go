package db

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The account store on every database.
func TestAccountStore(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "accounts", Test: testAccountStore},
		testutils.TestRunConfig{Name: "account manager", Test: testAccountManagerOnStore},
	)
	testDb.StartTestDBHandler()
}

func testAccountStore(t *testing.T, ds testutils.TestDataStore) {
	s := ds.DS
	created := time.UnixMilli(time.Now().UnixMilli())
	a := db.Account{ID: "acc.1", Username: "Anna", DisplayName: "Anna B.", Email: "a@example.org", Kind: "human",
		PasswordHash: "$argon2id$x", MustChangePassword: true, AuthorID: "a.xyz", CreatedAt: created}
	require.NoError(t, s.CreateAccount(a))
	assert.ErrorIs(t, s.CreateAccount(db.Account{ID: "acc.2", Username: "Anna", Kind: "human", CreatedAt: created}), db.ErrConflict)
	assert.ErrorIs(t, s.CreateAccount(db.Account{ID: "acc.1", Username: "other", Kind: "human", CreatedAt: created}), db.ErrConflict)

	got, err := s.GetAccount("acc.1")
	require.NoError(t, err)
	assert.Equal(t, a, *got)
	got, err = s.GetAccountByUsername("Anna")
	require.NoError(t, err)
	require.NotNil(t, got)
	missing, err := s.GetAccountByUsername("anna")
	require.NoError(t, err)
	assert.Nil(t, missing, "usernames compare as written")

	login := created.Add(time.Minute)
	a.LastLoginAt = &login
	a.DisabledAt = &login
	a.MustChangePassword = false
	require.NoError(t, s.UpdateAccount(a))
	require.NoError(t, s.UpdateAccount(a), "unchanged update is fine")
	got, _ = s.GetAccount("acc.1")
	assert.Equal(t, a, *got)
	assert.ErrorIs(t, s.UpdateAccount(db.Account{ID: "nope", Username: "x", CreatedAt: created}), db.ErrAccountNotFound)

	touched := created.Add(time.Hour)
	require.NoError(t, s.TouchLogin("acc.1", touched))
	got, _ = s.GetAccount("acc.1")
	assert.Equal(t, touched, *got.LastLoginAt)
	assert.NotNil(t, got.DisabledAt, "only the login time changes")
	a.LastLoginAt = &touched

	require.NoError(t, s.CreateAccount(db.Account{ID: "acc.2", Username: "Ben", Kind: "service", CreatedAt: created.Add(time.Millisecond)}))
	list, err := s.ListAccounts()
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "acc.1", list[0].ID)

	// Roles with JSON fields.
	r := db.Role{ID: "editor", Name: "editor", Description: "Edit", Builtin: true, Scopes: []string{"pad:write"},
		Includes: []string{"viewer"}, Limits: map[string]int64{"ai.requests": 50}}
	require.NoError(t, s.CreateRole(db.Role{ID: "viewer", Name: "viewer", Scopes: []string{"pad:read"}}))
	require.NoError(t, s.CreateRole(r))
	assert.ErrorIs(t, s.CreateRole(db.Role{ID: "x", Name: "editor"}), db.ErrConflict)
	gr, err := s.GetRole("editor")
	require.NoError(t, err)
	assert.Equal(t, r, *gr)
	viewer, _ := s.GetRole("viewer")
	assert.Equal(t, []string{}, viewer.Includes)
	assert.Equal(t, map[string]int64{}, viewer.Limits)
	r.Scopes = append(r.Scopes, "pad:create")
	require.NoError(t, s.UpdateRole(r))
	gr, _ = s.GetRole("editor")
	assert.Equal(t, []string{"pad:write", "pad:create"}, gr.Scopes)
	assert.ErrorIs(t, s.UpdateRole(db.Role{ID: "nope", Name: "nope"}), db.ErrRoleNotFound)
	roles, _ := s.ListRoles()
	assert.Len(t, roles, 2)

	// Assignments.
	require.NoError(t, s.AddRoleAssignment(db.RoleAssignment{AccountID: "acc.1", RoleID: "editor", PadPattern: "team/*", CreatedBy: "system", CreatedAt: created}))
	require.NoError(t, s.AddRoleAssignment(db.RoleAssignment{AccountID: "acc.1", RoleID: "viewer", PadPattern: "", CreatedBy: "system", CreatedAt: created}))
	require.NoError(t, s.AddRoleAssignment(db.RoleAssignment{AccountID: "acc.2", RoleID: "viewer", PadPattern: "", CreatedBy: "system", CreatedAt: created}))
	assert.ErrorIs(t, s.AddRoleAssignment(db.RoleAssignment{AccountID: "acc.1", RoleID: "editor", PadPattern: "team/*", CreatedAt: created}), db.ErrConflict)
	as, err := s.ListRoleAssignments("acc.1")
	require.NoError(t, err)
	assert.Len(t, as, 2)
	all, _ := s.ListRoleAssignments("")
	assert.Len(t, all, 3)
	require.NoError(t, s.RemoveRoleAssignment("acc.1", "editor", "team/*"))
	assert.ErrorIs(t, s.RemoveRoleAssignment("acc.1", "editor", "team/*"), db.ErrAssignmentNotFound)

	// Deleting a role removes its assignments and its inclusion.
	require.NoError(t, s.DeleteRole("viewer"))
	gr, _ = s.GetRole("editor")
	assert.Equal(t, []string{}, gr.Includes)
	all, _ = s.ListRoleAssignments("")
	assert.Empty(t, all)
	assert.ErrorIs(t, s.DeleteRole("viewer"), db.ErrRoleNotFound)

	require.NoError(t, s.AddRoleAssignment(db.RoleAssignment{AccountID: "acc.2", RoleID: "editor", CreatedBy: "system", CreatedAt: created}))
	require.NoError(t, s.DeleteAccount("acc.2"))
	all, _ = s.ListRoleAssignments("")
	assert.Empty(t, all)
	assert.ErrorIs(t, s.DeleteAccount("acc.2"), db.ErrAccountNotFound)

	// Audit, newest first.
	for i, action := range []string{"a", "b", "c"} {
		require.NoError(t, s.AppendAudit(db.AuditEntry{ID: action, At: created.Add(time.Duration(i) * time.Second), ActorID: "x", Action: action, Target: "t", Details: "{}"}))
	}
	assert.ErrorIs(t, s.AppendAudit(db.AuditEntry{ID: "a", At: created, Details: "{}"}), db.ErrConflict)
	has, err := s.HasAuditEntry("b")
	require.NoError(t, err)
	assert.True(t, has)
	has, _ = s.HasAuditEntry("zzz")
	assert.False(t, has)
	entries, err := s.ListAudit(2)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "c", entries[0].Action)
	assert.Equal(t, "b", entries[1].Action)
}

func testAccountManagerOnStore(t *testing.T, ds testutils.TestDataStore) {
	m := accounts.NewManager(ds.DS)
	cfg := accounts.SeedConfig{Template: accounts.TemplateOpen, BootstrapUser: "boss", BootstrapPassword: "very secret",
		LegacyUsers: map[string]accounts.LegacyUser{"old": {Password: "old-pw"}}}
	require.NoError(t, m.EnsureDefaults(cfg, nil))
	// A second instance starting on the same database.
	require.NoError(t, accounts.NewManager(ds.DS).EnsureDefaults(cfg, nil))

	old, err := m.Authenticate("old", "old-pw")
	require.NoError(t, err)
	g, err := m.GrantsFor(old.ID)
	require.NoError(t, err)
	assert.Equal(t, accounts.LevelCreate, g.Level("p"))
	boss := mustAccount(t, m, "boss")
	require.NotNil(t, boss, "settings.users had no admin: the bootstrap admin is created")
	bg, _ := m.GrantsFor(boss.ID)
	assert.True(t, bg.IsAdmin())
	anon, _ := m.AnonymousGrants()
	assert.Equal(t, accounts.LevelCreate, anon.Level("p"))
	list, _ := m.Accounts()
	assert.Len(t, list, 4, "anonymous, authenticated, old, boss: once each")
	audit, _ := m.Audit(100)
	assert.NotEmpty(t, audit)
}

func mustAccount(t *testing.T, m *accounts.Manager, username string) *db.Account {
	a, err := m.AccountByUsername(username)
	require.NoError(t, err)
	return a
}
