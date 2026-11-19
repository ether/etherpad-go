package ws

import (
	"encoding/json"
	"testing"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/models/ws/admin"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// The admin pages manage accounts, roles and visitors' rights.
func TestAdminAccounts(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "accounts and assignments", Test: testAdminAccountsLifecycle},
		testutils.TestRunConfig{Name: "roles and templates", Test: testAdminRoles},
		testutils.TestRunConfig{Name: "rights of the connection", Test: testAdminAccountsRights},
	)
	defer testDb.StartTestDBHandler()
}

// seedAdmin sets up the built-in roles and an admin; it returns the
// admin's account id.
func seedAdmin(t *testing.T, ds testutils.TestDataStore, username string) string {
	t.Helper()
	m := accounts.For(ds.DS)
	require.NoError(t, m.EnsureDefaults(accounts.SeedConfig{Template: accounts.TemplateOpen}, zap.NewNop().Sugar()))
	a, err := m.CreateAccount(accounts.System, accounts.NewAccount{Username: username, Kind: accounts.KindHuman, Password: "a long enough password"})
	require.NoError(t, err)
	require.NoError(t, m.Assign(accounts.System, a.ID, accounts.RoleAdmin, ""))
	return a.ID
}

func accountRequest(t *testing.T, ds testutils.TestDataStore, me, event string, payload any) (string, json.RawMessage) {
	t.Helper()
	client := &libws.Client{Hub: libws.NewHub(), Conn: ds.MockWebSocket, Send: make(chan []byte, 256), SessionId: "admin", AdminAccount: me}
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	before := len(ds.MockWebSocket.Data)
	pump := startMockWritePump(client, ds.MockWebSocket)
	s := settings.Displayed
	ds.AdminMessageHandler.HandleMessage(admin.EventMessage{Event: event, Data: data}, &s, client)
	pump.Wait()
	require.Len(t, ds.MockWebSocket.Data, before+1, "%s must get exactly one reply", event)
	var resp []json.RawMessage
	require.NoError(t, json.Unmarshal(ds.MockWebSocket.Data[before].Data, &resp))
	require.Len(t, resp, 2)
	var name string
	require.NoError(t, json.Unmarshal(resp[0], &name))
	return name, resp[1]
}

func accountsCall(t *testing.T, ds testutils.TestDataStore, me, event string, data any) libws.AccountsResult {
	t.Helper()
	name, raw := accountRequest(t, ds, me, event, data)
	require.Equal(t, "results:accounts", name)
	var res libws.AccountsResult
	require.NoError(t, json.Unmarshal(raw, &res))
	return res
}

func findAccount(res libws.AccountsResult, username string) *libws.AccountView {
	for i := range res.Accounts {
		if res.Accounts[i].Username == username {
			return &res.Accounts[i]
		}
	}
	return nil
}

func findRole(res libws.AccountsResult, id string) *libws.RoleView {
	for i := range res.Roles {
		if res.Roles[i].ID == id {
			return &res.Roles[i]
		}
	}
	return nil
}

func testAdminAccountsLifecycle(t *testing.T, ds testutils.TestDataStore) {
	me := seedAdmin(t, ds, "root")
	res := accountsCall(t, ds, me, "accountsLoad", nil)
	require.Empty(t, res.Error)
	assert.Equal(t, me, res.Me)
	root := findAccount(res, "root")
	require.NotNil(t, root)
	assert.True(t, root.Admin)
	assert.NotNil(t, findAccount(res, "@"+accounts.AnonymousID), "the visitors are listed")
	assert.NotEmpty(t, res.Scopes)
	raw, _ := json.Marshal(res)
	assert.NotContains(t, string(raw), "argon2", "no password hashes")

	// a person gets a one-time password, shown once
	res = accountsCall(t, ds, me, "accountCreate", map[string]any{"username": "anna", "displayName": "Anna", "kind": "human", "role": "editor", "pads": "team/*"})
	require.Empty(t, res.Error)
	require.NotNil(t, res.Password)
	assert.Equal(t, "anna", res.Password.Username)
	anna := findAccount(res, "anna")
	require.NotNil(t, anna)
	assert.True(t, anna.MustChangePassword)
	require.Len(t, anna.Roles, 1)
	assert.Equal(t, libws.AssignmentView{Role: "editor", Pads: "team/*", CreatedBy: me, CreatedAt: anna.Roles[0].CreatedAt}, anna.Roles[0])
	m := accounts.For(ds.DS)
	_, err := m.Authenticate("anna", res.Password.Password)
	require.NoError(t, err, "the one-time password works")
	assert.Nil(t, accountsCall(t, ds, me, "accountsLoad", nil).Password)

	// a service account has no password
	res = accountsCall(t, ds, me, "accountCreate", map[string]any{"username": "moodle", "kind": "service"})
	require.Empty(t, res.Error)
	assert.Nil(t, res.Password)
	assert.False(t, findAccount(res, "moodle").HasPassword)

	// a role that does not exist leaves no account behind
	res = accountsCall(t, ds, me, "accountCreate", map[string]any{"username": "ghost", "kind": "human", "role": "nope"})
	assert.Equal(t, "role not found", res.Error)
	assert.Nil(t, findAccount(res, "ghost"))

	res = accountsCall(t, ds, me, "accountCreate", map[string]any{"username": "anna", "kind": "human"})
	assert.Equal(t, "that name is already taken", res.Error)

	res = accountsCall(t, ds, me, "accountUpdate", map[string]any{"id": anna.ID, "displayName": "Anna B.", "email": "anna@example.org"})
	require.Empty(t, res.Error)
	assert.Equal(t, "Anna B.", findAccount(res, "anna").DisplayName)
	assert.Equal(t, "anna@example.org", findAccount(res, "anna").Email)
	res = accountsCall(t, ds, me, "accountUpdate", map[string]any{"id": anna.ID, "email": "no mail"})
	assert.Equal(t, accounts.ErrInvalidProfile.Error(), res.Error)

	res = accountsCall(t, ds, me, "assignmentAdd", map[string]any{"accountId": anna.ID, "role": "ai-user", "pads": ""})
	require.Empty(t, res.Error)
	assert.Len(t, findAccount(res, "anna").Roles, 2)
	res = accountsCall(t, ds, me, "assignmentRemove", map[string]any{"accountId": anna.ID, "role": "editor", "pads": "team/*"})
	require.Empty(t, res.Error)
	require.Len(t, findAccount(res, "anna").Roles, 1)
	assert.Equal(t, "ai-user", findAccount(res, "anna").Roles[0].Role)

	res = accountsCall(t, ds, me, "accountResetPassword", map[string]any{"id": anna.ID})
	require.Empty(t, res.Error)
	require.NotNil(t, res.Password)
	_, err = m.Authenticate("anna", res.Password.Password)
	require.NoError(t, err)

	res = accountsCall(t, ds, me, "accountSetDisabled", map[string]any{"id": anna.ID, "disabled": true})
	require.Empty(t, res.Error)
	assert.True(t, findAccount(res, "anna").Disabled)

	// the last admin stays
	res = accountsCall(t, ds, me, "accountSetDisabled", map[string]any{"id": me, "disabled": true})
	assert.Equal(t, accounts.ErrLastAdmin.Error(), res.Error)
	res = accountsCall(t, ds, me, "accountDelete", map[string]any{"id": me})
	assert.Equal(t, accounts.ErrLastAdmin.Error(), res.Error)

	res = accountsCall(t, ds, me, "accountDelete", map[string]any{"id": anna.ID})
	require.Empty(t, res.Error)
	assert.Nil(t, findAccount(res, "anna"))

	res = accountsCall(t, ds, me, "accountCreate", "not an object")
	assert.Equal(t, "invalid request", res.Error)

	// every change is in the audit log, with the names of the accounts
	name, raw := accountRequest(t, ds, me, "auditLoad", map[string]int{"limit": 100})
	require.Equal(t, "results:audit", name)
	var audit libws.AuditResult
	require.NoError(t, json.Unmarshal(raw, &audit))
	require.Empty(t, audit.Error)
	actions := map[string]bool{}
	for _, e := range audit.Entries {
		actions[e.Action] = true
	}
	for _, want := range []string{"account.create", "account.profile", "assignment.add", "assignment.remove", "account.reset-password", "account.disable", "account.delete"} {
		assert.True(t, actions[want], want)
	}
	assert.Equal(t, "root", audit.Names[me])
}

func testAdminRoles(t *testing.T, ds testutils.TestDataStore) {
	me := seedAdmin(t, ds, "root")
	res := accountsCall(t, ds, me, "roleSave", map[string]any{"name": "Claude users", "description": "AI under claude/", "scopes": []string{"ai:use", "pad:read"}, "limits": map[string]int64{"ai.requestsPerDay": 50}})
	require.Empty(t, res.Error)
	var custom *libws.RoleView
	for i := range res.Roles {
		if res.Roles[i].Name == "Claude users" {
			custom = &res.Roles[i]
		}
	}
	require.NotNil(t, custom)
	assert.False(t, custom.Builtin)
	assert.Equal(t, []string{"ai:use", "pad:read"}, custom.Scopes)
	assert.Equal(t, int64(50), custom.Limits["ai.requestsPerDay"])

	// the admin role is fixed, built-in roles can be changed and reset
	assert.True(t, findRole(res, accounts.RoleAdmin).Fixed)
	res = accountsCall(t, ds, me, "roleSave", map[string]any{"id": accounts.RoleAdmin, "name": "admin", "scopes": []string{}})
	assert.Equal(t, accounts.ErrAdminRoleFixed.Error(), res.Error)
	res = accountsCall(t, ds, me, "roleSave", map[string]any{"id": accounts.RoleViewer, "name": "viewer", "description": "Read pads", "scopes": []string{"pad:read", "chat:write"}})
	require.Empty(t, res.Error)
	assert.True(t, findRole(res, accounts.RoleViewer).Modified)
	res = accountsCall(t, ds, me, "roleReset", map[string]any{"id": accounts.RoleViewer})
	require.Empty(t, res.Error)
	assert.False(t, findRole(res, accounts.RoleViewer).Modified, "%+v", *findRole(res, accounts.RoleViewer))
	assert.Equal(t, []string{"pad:read"}, findRole(res, accounts.RoleViewer).Scopes)
	res = accountsCall(t, ds, me, "roleReset", map[string]any{"id": custom.ID})
	assert.Equal(t, accounts.ErrNotBuiltin.Error(), res.Error)
	res = accountsCall(t, ds, me, "roleDelete", map[string]any{"id": accounts.RoleEditor})
	assert.Equal(t, accounts.ErrBuiltinRole.Error(), res.Error)

	// visitors never get admin scopes
	res = accountsCall(t, ds, me, "assignmentAdd", map[string]any{"accountId": accounts.AnonymousID, "role": accounts.RoleAdmin})
	assert.Equal(t, accounts.ErrAnonymousScope.Error(), res.Error)

	res = accountsCall(t, ds, me, "templateApply", map[string]any{"template": "read"})
	require.Empty(t, res.Error)
	anon := findAccount(res, "@"+accounts.AnonymousID)
	require.Len(t, anon.Roles, 1)
	assert.Equal(t, accounts.RoleViewer, anon.Roles[0].Role)
	res = accountsCall(t, ds, me, "templateApply", map[string]any{"template": "nope"})
	assert.NotEmpty(t, res.Error)

	res = accountsCall(t, ds, me, "roleDelete", map[string]any{"id": custom.ID})
	require.Empty(t, res.Error)
	assert.Nil(t, findRole(res, custom.ID))
}

func testAdminAccountsRights(t *testing.T, ds testutils.TestDataStore) {
	me := seedAdmin(t, ds, "root")
	m := accounts.For(ds.DS)

	// a connection without an account changes nothing
	res := accountsCall(t, ds, "", "accountCreate", map[string]any{"username": "x", "kind": "human"})
	assert.Equal(t, accounts.ErrForbidden.Error(), res.Error)

	// a user manager who is no admin cannot hand out admin
	mgr, err := m.SaveRole(accounts.System, db.Role{Name: "user manager", Scopes: []string{accounts.ScopeAdminUsers, accounts.ScopePadRead}})
	require.NoError(t, err)
	res = accountsCall(t, ds, me, "accountCreate", map[string]any{"username": "eve", "kind": "human", "role": mgr.ID})
	require.Empty(t, res.Error)
	eve := findAccount(res, "eve").ID
	res = accountsCall(t, ds, eve, "assignmentAdd", map[string]any{"accountId": eve, "role": accounts.RoleAdmin})
	assert.Equal(t, accounts.ErrEscalation.Error(), res.Error)
	res = accountsCall(t, ds, eve, "accountResetPassword", map[string]any{"id": me})
	assert.Equal(t, accounts.ErrEscalation.Error(), res.Error, "nobody takes over an account with more rights")

	// rights are those of now, not of the login
	res = accountsCall(t, ds, me, "accountSetDisabled", map[string]any{"id": eve, "disabled": true})
	require.Empty(t, res.Error)
	res = accountsCall(t, ds, eve, "accountCreate", map[string]any{"username": "y", "kind": "service"})
	assert.Equal(t, accounts.ErrForbidden.Error(), res.Error)
	name, raw := accountRequest(t, ds, eve, "auditLoad", nil)
	require.Equal(t, "results:audit", name)
	var audit libws.AuditResult
	require.NoError(t, json.Unmarshal(raw, &audit))
	assert.Equal(t, accounts.ErrForbidden.Error(), audit.Error)
	assert.Empty(t, audit.Entries)
}
