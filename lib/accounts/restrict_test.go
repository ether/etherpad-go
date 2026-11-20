package accounts

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A token never has more than its account: every check needs both.
func TestRestrict(t *testing.T) {
	m, _ := newSeeded(t, SeedConfig{Template: TemplateClosed})
	claude, err := m.CreateAccount(System, NewAccount{Username: "claude", Kind: KindService})
	require.NoError(t, err)
	require.NoError(t, m.Assign(System, claude.ID, RoleEditor, "claude/*"))
	acc, err := m.GrantsFor(claude.ID)
	require.NoError(t, err)

	// the concept's example: delete is not the account's
	tok := acc.Restrict([]string{ScopePadRead, ScopePadWrite, ScopePadCreate, ScopePadDelete}, "", nil)
	assert.True(t, tok.Has(ScopePadWrite, "claude/todo"))
	assert.True(t, tok.Has(ScopePadCreate, "claude/2026/nov"))
	assert.False(t, tok.Has(ScopePadDelete, "claude/todo"), "the account lacks it")
	assert.False(t, tok.Has(ScopePadRead, "team/notes"), "outside the account's pattern")
	assert.Equal(t, LevelCreate, tok.Level("claude/todo"))
	assert.Equal(t, LevelNone, tok.Level("other"))

	// the token's own pattern and scopes cut further
	ro := acc.Restrict([]string{ScopePadRead}, "claude/public/*", nil)
	assert.True(t, ro.Has(ScopePadRead, "claude/public/a"))
	assert.False(t, ro.Has(ScopePadRead, "claude/todo"))
	assert.False(t, ro.Has(ScopePadWrite, "claude/public/a"))
	assert.Equal(t, LevelReadOnly, ro.Level("claude/public/a"))
	assert.False(t, ro.HasGlobal(ScopePadRead))
	assert.True(t, ro.HasAnywhere(ScopePadRead))
	assert.False(t, ro.Empty())
	assert.True(t, acc.Restrict(nil, "", nil).Empty())

	// an admin's token with a pattern gets no admin scopes
	root, err := m.CreateAccount(System, NewAccount{Username: "root", Kind: KindHuman, Password: "root-password"})
	require.NoError(t, err)
	require.NoError(t, m.Assign(System, root.ID, RoleAdmin, ""))
	admin, err := m.GrantsFor(root.ID)
	require.NoError(t, err)
	all := admin.Restrict([]string{AllScopes}, "", nil)
	assert.True(t, all.IsAdmin())
	assert.True(t, all.isAll())
	scoped := admin.Restrict([]string{AllScopes}, "claude/*", nil)
	assert.False(t, scoped.IsAdmin(), "admin scopes need a token for all pads")
	assert.False(t, scoped.HasAnywhere(ScopeAdminUsers))
	assert.True(t, scoped.Has(ScopePadDelete, "claude/x"))
	assert.False(t, scoped.isAll())
	assert.False(t, admin.Restrict([]string{ScopePadRead}, "", nil).isAll())
	assert.Empty(t, scoped.GlobalScopes())
	assert.Contains(t, all.GlobalScopes(), ScopeAdminUsers)

	// limits: the smaller of account and token
	acc.Limits = map[string]int64{"ai.requestsPerDay": 50, "import.maxBytes": 10}
	lim := acc.Restrict([]string{ScopeAIUse}, "", map[string]int64{"ai.requestsPerDay": 5, "import.maxBytes": 100, "own": 1})
	assert.Equal(t, int64(5), lim.Limits["ai.requestsPerDay"])
	assert.Equal(t, int64(10), lim.Limits["import.maxBytes"])
	_, has := lim.Limits["own"]
	assert.False(t, has, "a limit the account lacks is not the token's")

	// a union keeps each part's cut
	both := ro.Union(acc)
	assert.True(t, both.Has(ScopePadWrite, "claude/todo"), "from the uncut part")
	assert.True(t, ro.Union(Grants{}).Has(ScopePadRead, "claude/public/a"))
	assert.False(t, ro.Union(Grants{}).Has(ScopePadRead, "claude/todo"))

	assert.Equal(t, []PatternScopes{{Pattern: "claude/* & claude/public/*", Scopes: []string{ScopePadRead}}}, ro.Summary())
}
