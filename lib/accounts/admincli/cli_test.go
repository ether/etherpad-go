package admincli

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func run(t *testing.T, store db.DataStore, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Main(args, &out, &errOut, store, &settings.Settings{})
	return code, out.String(), errOut.String()
}

var passwordLine = regexp.MustCompile(`One-time password: (\S+)`)

func TestCreateAdminAndLogin(t *testing.T) {
	store := db.NewMemoryDataStore()
	code, out, errOut := run(t, store, "create", "boss")
	require.Equal(t, 0, code, errOut)
	match := passwordLine.FindStringSubmatch(out)
	require.Len(t, match, 2, out)

	m := accounts.For(store)
	a, err := m.Authenticate("boss", match[1])
	require.NoError(t, err)
	assert.True(t, a.MustChangePassword)
	admin, err := m.IsAdmin(a.ID)
	require.NoError(t, err)
	assert.True(t, admin)
	audit, _ := m.Audit(10)
	assert.Equal(t, "cli", audit[0].ActorID)

	code, _, errOut = run(t, store, "create", "boss")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "exists already")
}

func TestCreateWithRoleAndPattern(t *testing.T) {
	store := db.NewMemoryDataStore()
	code, _, errOut := run(t, store, "create", "--role", "editor", "anna", "--pads", "team/*")
	require.Equal(t, 0, code, errOut)
	m := accounts.For(store)
	a, _ := m.AccountByUsername("anna")
	refs, _ := m.Refs(a.ID)
	assert.Equal(t, []accounts.RoleRef{{Role: accounts.RoleEditor, Pads: "team/*"}}, refs)

	code, _, errOut = run(t, store, "create", "x", "--role", "nope")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, `no role "nope"`)
	code, _, _ = run(t, store, "create")
	assert.Equal(t, 2, code)
	code, _, _ = run(t, store, "create", "a", "b")
	assert.Equal(t, 2, code)
	code, _, _ = run(t, store, "frobnicate")
	assert.Equal(t, 2, code)
}

func TestResetPassword(t *testing.T) {
	store := db.NewMemoryDataStore()
	m := accounts.For(store)
	require.NoError(t, m.Ensure(&settings.Settings{}, nil))
	anna, err := m.CreateAccount(accounts.System, accounts.NewAccount{Username: "anna", Kind: accounts.KindHuman, Password: "old password"})
	require.NoError(t, err)
	_, err = m.Authenticate("anna", "old password")
	require.NoError(t, err)
	require.NoError(t, m.SetDisabled(accounts.System, anna.ID, true))

	code, out, errOut := run(t, store, "reset-password", "anna")
	require.Equal(t, 0, code, errOut)
	password := passwordLine.FindStringSubmatch(out)[1]
	_, err = m.Authenticate("anna", "old password")
	assert.ErrorIs(t, err, accounts.ErrInvalidCredentials, "the old password, even remembered, is gone")
	a, err := m.Authenticate("anna", password)
	require.NoError(t, err, "enabled again")
	assert.True(t, a.MustChangePassword)

	code, _, errOut = run(t, store, "reset-password", "nobody")
	assert.Equal(t, 1, code)
	assert.Contains(t, errOut, "no account")
	_, err = m.CreateAccount(accounts.System, accounts.NewAccount{Username: "svc", Kind: accounts.KindService})
	require.NoError(t, err)
	code, _, _ = run(t, store, "reset-password", "svc")
	assert.Equal(t, 1, code, "service accounts have no password")
}

func TestList(t *testing.T) {
	store := db.NewMemoryDataStore()
	_, _, _ = run(t, store, "create", "boss")
	_, _, _ = run(t, store, "create", "anna", "--role", "editor", "--pads", "team/*")
	code, out, _ := run(t, store, "list")
	require.Equal(t, 0, code)
	assert.Regexp(t, `boss\s+human\s+must change password\s+admin`, out)
	assert.Regexp(t, `anna\s+human\s+must change password\s+editor \(team/\*\)`, out)
	assert.Regexp(t, `@anonymous\s+builtin\s+-\s+editor`, out, "the open template")
}
