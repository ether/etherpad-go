package accounts

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPattern(t *testing.T) {
	cases := []struct {
		pattern, pad string
		want         bool
	}{
		{"", "anything", true},
		{"*", "a/b/c", true},
		{"notes", "notes", true},
		{"notes", "notes2", false},
		{"claude/*", "claude/todo", true},
		{"claude/*", "claude/2026/nov", true},
		{"claude/*", "claudette", false},
		{"claude/*", "x/claude/todo", false},
		{"team-*/protokolle/*", "team-a/protokolle/2026-11", true},
		{"team-*/protokolle/*", "team-a/notes/2026", false},
		{"*-entwurf", "brief-entwurf", true},
		{"*-entwurf", "entwurf-brief", false},
		{"a.b", "aXb", false},          // regexp characters are quoted
		{"(a|b)+", "(a|b)+", true},     // matched as written
		{"g.x$name", "g.x$name", true}, // group pads
		{"pad[1]", "pad1", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, MatchPattern(c.pattern, c.pad), "%q on %q", c.pattern, c.pad)
	}
	assert.NoError(t, ValidatePattern("team-*/x"))
	assert.ErrorIs(t, ValidatePattern("a\nb"), ErrInvalidPattern)
	long := make([]byte, MaxPatternLength+1)
	for i := range long {
		long[i] = 'a'
	}
	assert.ErrorIs(t, ValidatePattern(string(long)), ErrInvalidPattern)
}

func TestPassword(t *testing.T) {
	h, err := HashPassword("correct horse")
	require.NoError(t, err)
	assert.True(t, CheckPassword(h, "correct horse"))
	assert.False(t, CheckPassword(h, "correct hors"))
	h2, _ := HashPassword("correct horse")
	assert.NotEqual(t, h, h2, "salted")
	assert.False(t, CheckPassword("plain", "plain"))
	assert.False(t, CheckPassword("$argon2id$v=19$m=999999999,t=2,p=1$c2FsdA$aGFzaA", "x"), "absurd parameters")
}

func newSeeded(t *testing.T, cfg SeedConfig) (*Manager, *db.MemoryDataStore) {
	t.Helper()
	store := db.NewMemoryDataStore()
	m := NewManager(store)
	require.NoError(t, m.EnsureDefaults(cfg, nil))
	return m, store
}

func grantsOf(t *testing.T, m *Manager, username string) Grants {
	t.Helper()
	a, err := m.AccountByUsername(username)
	require.NoError(t, err)
	require.NotNil(t, a, username)
	g, err := m.GrantsFor(a.ID)
	require.NoError(t, err)
	return g
}

func TestSeedOpenInstanceKeepsEtherpadBehaviour(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateOpen, BootstrapUser: "boss", BootstrapPassword: "very secret"})
	anon, err := m.AnonymousGrants()
	require.NoError(t, err)
	assert.Equal(t, LevelCreate, anon.Level("any/pad"))
	assert.True(t, anon.Has(ScopeChatWrite, "x"))
	assert.False(t, anon.Has(ScopePadDelete, "x"))
	assert.False(t, anon.IsAdmin())

	boss := grantsOf(t, m, "boss")
	assert.True(t, boss.IsAdmin())
	assert.True(t, boss.Has(ScopePadDelete, "x"))
	assert.True(t, boss.Has("plugin:later", "x"), "admin has scopes registered later")
	a, _ := m.AccountByUsername("boss")
	assert.True(t, a.MustChangePassword)

	// Running again changes nothing, the bootstrap admin is not created
	// twice and the template is not applied again.
	require.NoError(t, m.Assign(System, AnonymousID, RoleViewer, "public/*"))
	require.NoError(t, m.EnsureDefaults(SeedConfig{Template: TemplateClosed, BootstrapUser: "other", BootstrapPassword: "very secret"}, nil))
	accounts, _ := m.Accounts()
	assert.Len(t, accounts, 3, "anonymous, authenticated, boss")
	anon, _ = m.AnonymousGrants()
	assert.Equal(t, LevelCreate, anon.Level("x"))
}

func TestSeedClosedInstanceWithLegacyUsers(t *testing.T) {
	no := false
	m, _ := newSeeded(t, SeedConfig{Template: TemplateClosed, LegacyUsers: map[string]LegacyUser{
		"admin":    {Password: "pw-admin", IsAdmin: true},
		"writer":   {Password: "pw-writer"},
		"reader":   {Password: "pw-reader", ReadOnly: true},
		"nocreate": {Password: "pw-nocreate", CanCreate: &no},
		"plugin":   {IsAdmin: true}, // a plugin authenticates it
	}})
	anon, _ := m.AnonymousGrants()
	assert.True(t, anon.Empty(), "closed: visitors may do nothing")
	assert.Equal(t, LevelCreate, grantsOf(t, m, "writer").Level("p"))
	assert.Equal(t, LevelReadOnly, grantsOf(t, m, "reader").Level("p"))
	assert.Equal(t, LevelModify, grantsOf(t, m, "nocreate").Level("p"))
	assert.True(t, grantsOf(t, m, "admin").IsAdmin())
	assert.True(t, grantsOf(t, m, "plugin").IsAdmin())
	// Read-only users exist: logged-in people as a whole get nothing more.
	auth, _ := m.AuthenticatedGrants()
	assert.True(t, auth.Empty())

	_, err := m.Authenticate("writer", "pw-writer")
	assert.NoError(t, err, "passwords were hashed and still work")
	_, err = m.Authenticate("plugin", "")
	assert.ErrorIs(t, err, ErrInvalidCredentials, "no password, no login")
}

func TestSeedClosedInstanceLetsLoggedInPeopleEdit(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateClosed, LegacyUsers: map[string]LegacyUser{
		"admin": {Password: "pw-admin", IsAdmin: true},
	}})
	auth, _ := m.AuthenticatedGrants()
	assert.Equal(t, LevelCreate, auth.Level("p"), "as with requireAuthentication: every logged-in person edits")
	anon, _ := m.AnonymousGrants()
	assert.Equal(t, LevelNone, anon.Level("p"))
}

func TestBootstrapDespitePasswordlessAdmin(t *testing.T) {
	// settings.template.json has an "admin" without password.
	m, _ := newSeeded(t, SeedConfig{Template: TemplateOpen, BootstrapUser: "boss", BootstrapPassword: "very secret",
		LegacyUsers: map[string]LegacyUser{"admin": {IsAdmin: true}, "user": {}}})
	assert.True(t, grantsOf(t, m, "boss").IsAdmin())
	assert.True(t, grantsOf(t, m, "admin").IsAdmin())
}

func TestSeedReadTemplateAndPatterns(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateRead, BootstrapUser: "boss", BootstrapPassword: "very secret"})
	require.NoError(t, m.Assign(System, AnonymousID, RoleEditor, "public/*"))
	anon, _ := m.AnonymousGrants()
	assert.Equal(t, LevelReadOnly, anon.Level("team/notes"))
	assert.Equal(t, LevelCreate, anon.Level("public/flyer"))
	assert.True(t, anon.HasAnywhere(ScopePadWrite))
	assert.False(t, anon.HasGlobal(ScopePadWrite))

	claude, err := m.CreateAccount(System, NewAccount{Username: "claude", Kind: KindService})
	require.NoError(t, err)
	require.NoError(t, m.Assign(System, claude.ID, RoleEditor, "claude/*"))
	g, _ := m.GrantsFor(claude.ID)
	assert.Equal(t, LevelCreate, g.Level("claude/todo"))
	assert.Equal(t, LevelReadOnly, g.Level("other"), "still a visitor elsewhere")
	assert.Contains(t, g.Summary(), PatternScopes{Pattern: "claude/*", Scopes: []string{ScopeChatWrite, ScopePadCreate, ScopePadRead, ScopePadWrite}})

	// Scopes that are not pad scoped do not count from a pattern.
	require.NoError(t, m.Assign(System, claude.ID, RoleAdmin, "claude/*"))
	g, _ = m.GrantsFor(claude.ID)
	assert.False(t, g.IsAdmin())
	assert.True(t, g.Has(ScopePadDelete, "claude/x"))
	assert.False(t, g.Has(ScopePadDelete, "other"))
}

func TestLimitsTakeTheMostGenerous(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateClosed, BootstrapUser: "boss", BootstrapPassword: "very secret"})
	ai, _ := m.Roles()
	r := ai[RoleAIUser]
	r.Limits = map[string]int64{"ai.requests": 50, "import.mb": 10}
	_, err := m.SaveRole(System, r)
	require.NoError(t, err)
	imp, err := m.SaveRole(System, db.Role{Name: "importer", Limits: map[string]int64{"ai.requests": 5, "import.mb": 100}})
	require.NoError(t, err)
	anna, _ := m.CreateAccount(System, NewAccount{Username: "anna", Kind: KindHuman})
	require.NoError(t, m.Assign(System, anna.ID, RoleAIUser, ""))
	require.NoError(t, m.Assign(System, anna.ID, imp.ID, ""))
	g, _ := m.GrantsFor(anna.ID)
	assert.Equal(t, map[string]int64{"ai.requests": 50, "import.mb": 100}, g.Limits)
}

func adminActor(t *testing.T, m *Manager, username string) Actor {
	a, _ := m.AccountByUsername(username)
	return Actor{ID: a.ID, Grants: grantsOf(t, m, username)}
}

func TestProtectionRules(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateOpen, BootstrapUser: "boss", BootstrapPassword: "very secret"})
	boss := adminActor(t, m, "boss")

	// The last admin stays.
	assert.ErrorIs(t, m.Unassign(boss, boss.ID, RoleAdmin, ""), ErrLastAdmin)
	assert.ErrorIs(t, m.SetDisabled(boss, boss.ID, true), ErrLastAdmin)
	assert.ErrorIs(t, m.DeleteAccount(boss, boss.ID), ErrLastAdmin)
	second, err := m.CreateAccount(boss, NewAccount{Username: "second", Kind: KindHuman, Password: "long enough"})
	require.NoError(t, err)
	require.NoError(t, m.Assign(boss, second.ID, RoleAdmin, ""))
	require.NoError(t, m.SetDisabled(boss, second.ID, true))
	assert.ErrorIs(t, m.Unassign(boss, boss.ID, RoleAdmin, ""), ErrLastAdmin, "a disabled admin does not count")
	require.NoError(t, m.SetDisabled(boss, second.ID, false))
	require.NoError(t, m.Unassign(boss, boss.ID, RoleAdmin, ""))
	assert.ErrorIs(t, m.Unassign(System, second.ID, RoleAdmin, ""), ErrLastAdmin)

	// The admin role is fixed, built-in roles stay.
	roles, _ := m.Roles()
	adm := roles[RoleAdmin]
	adm.Scopes = nil
	_, err = m.SaveRole(System, adm)
	assert.ErrorIs(t, err, ErrAdminRoleFixed)
	assert.ErrorIs(t, m.DeleteRole(System, RoleAdmin), ErrAdminRoleFixed)
	assert.ErrorIs(t, m.DeleteRole(System, RoleViewer), ErrBuiltinRole)

	// Visitors never get admin scopes, not even through an included role.
	assert.ErrorIs(t, m.Assign(System, AnonymousID, RoleAdmin, ""), ErrAnonymousScope)
	ed := roles[RoleEditor]
	ed.Scopes = append(ed.Scopes, ScopeAdminUsers)
	_, err = m.SaveRole(System, ed)
	assert.ErrorIs(t, err, ErrAnonymousScope, "anonymous holds editor")
	sneaky, err := m.SaveRole(System, db.Role{Name: "sneaky", Scopes: []string{ScopePadRead}})
	require.NoError(t, err)
	require.NoError(t, m.Assign(System, AnonymousID, sneaky.ID, "x/*"))
	sneaky.Includes = []string{RoleAdmin}
	_, err = m.SaveRole(System, *sneaky)
	assert.ErrorIs(t, err, ErrAnonymousScope, "through an included role")
	assert.ErrorIs(t, m.Assign(System, AuthenticatedID, RoleAdmin, ""), ErrAnonymousScope)
	assert.ErrorIs(t, m.SetDisabled(System, AnonymousID, true), ErrAnonymousAccount)

	// No cycles.
	v := roles[RoleViewer]
	v.Includes = []string{RoleModerator}
	_, err = m.SaveRole(System, v)
	assert.ErrorIs(t, err, ErrRoleCycle)
}

func TestNoEscalation(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateClosed, BootstrapUser: "boss", BootstrapPassword: "very secret"})
	// A user manager who is no admin.
	mgrRole, err := m.SaveRole(System, db.Role{Name: "user-manager", Scopes: []string{ScopeAdminUsers}, Includes: []string{RoleEditor}})
	require.NoError(t, err)
	mgr, _ := m.CreateAccount(System, NewAccount{Username: "mgr", Kind: KindHuman})
	require.NoError(t, m.Assign(System, mgr.ID, mgrRole.ID, ""))
	actor := Actor{ID: mgr.ID, Grants: grantsOf(t, m, "mgr")}

	anna, err := m.CreateAccount(actor, NewAccount{Username: "anna", Kind: KindHuman})
	require.NoError(t, err)
	require.NoError(t, m.Assign(actor, anna.ID, RoleEditor, "team/*"), "what it has itself")
	assert.ErrorIs(t, m.Assign(actor, anna.ID, RoleModerator, ""), ErrEscalation)
	assert.ErrorIs(t, m.Assign(actor, mgr.ID, RoleAdmin, ""), ErrEscalation, "not even itself")
	_, err = m.SaveRole(actor, db.Role{Name: "x"})
	assert.ErrorIs(t, err, ErrForbidden, "no admin:roles")
	editorOnly := Actor{ID: anna.ID, Grants: grantsOf(t, m, "anna")}
	_, err = m.CreateAccount(editorOnly, NewAccount{Username: "eve", Kind: KindHuman})
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestAuthenticate(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateClosed, BootstrapUser: "boss", BootstrapPassword: "very secret"})
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }

	a, err := m.Authenticate("boss", "very secret")
	require.NoError(t, err)
	assert.NotNil(t, a.LastLoginAt)
	_, err = m.Authenticate("boss", "wrong")
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = m.Authenticate("nobody", "x")
	assert.ErrorIs(t, err, ErrInvalidCredentials)

	svc, _ := m.CreateAccount(System, NewAccount{Username: "svc", Kind: KindService})
	_, err = m.CreateAccount(System, NewAccount{Username: "svc2", Kind: KindService, Password: "long enough"})
	assert.ErrorIs(t, err, ErrNoPassword)
	assert.ErrorIs(t, m.SetPassword(System, svc.ID, "long enough", false), ErrNoPassword)
	_, err = m.Authenticate("@anonymous", "")
	assert.ErrorIs(t, err, ErrInvalidCredentials)

	// Locked after repeated failures, also for the right password.
	for i := 0; i < maxFailures; i++ {
		_, _ = m.Authenticate("boss", "wrong")
	}
	m.Invalidate() // forget the remembered good login
	_, err = m.Authenticate("boss", "very secret")
	assert.ErrorIs(t, err, ErrLocked)
	now = now.Add(failureWindow + time.Second)
	_, err = m.Authenticate("boss", "very secret")
	assert.NoError(t, err)

	// A disabled account cannot log in, not even with a remembered login.
	anna, _ := m.CreateAccount(System, NewAccount{Username: "anna", Kind: KindHuman, Password: "anna's secret"})
	_, err = m.Authenticate("anna", "anna's secret")
	require.NoError(t, err)
	require.NoError(t, m.SetDisabled(System, anna.ID, true))
	_, err = m.Authenticate("anna", "anna's secret")
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	assert.ErrorIs(t, m.SetPassword(System, anna.ID, "short", false), ErrPasswordTooShort)
}

func TestGrantsCacheAndTokenRefs(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateRead, BootstrapUser: "boss", BootstrapPassword: "very secret"})
	now := time.Now()
	m.now = func() time.Time { return now }
	anna, _ := m.CreateAccount(System, NewAccount{Username: "anna", Kind: KindHuman})
	g, _ := m.GrantsFor(anna.ID)
	assert.Equal(t, LevelReadOnly, g.Level("p"))
	require.NoError(t, m.Assign(System, anna.ID, RoleEditor, ""))
	g, _ = m.GrantsFor(anna.ID)
	assert.Equal(t, LevelCreate, g.Level("p"), "a change here is seen at once")

	refs, _ := m.Refs(anna.ID)
	assert.Equal(t, []RoleRef{{Role: RoleEditor}}, refs)
	g, err := m.GrantsFromRefs([]RoleRef{{Role: RoleModerator, Pads: "team/*"}})
	require.NoError(t, err)
	assert.True(t, g.Has(ScopePadDelete, "team/a"))
	assert.Equal(t, LevelReadOnly, g.Level("elsewhere"), "the visitors' roles come with it")
	assert.False(t, g.IsAdmin())
}
