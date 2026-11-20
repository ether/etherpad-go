package legacy

import (
	"testing"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A token may call what its scopes and pad pattern allow, on its pads.
func TestLegacyTokenScopes(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "scoped tokens", Test: testLegacyTokenScopes})
	testDb.StartTestDBHandler()
}

func testLegacyTokenScopes(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	admin := c.key
	// Visitors may do nothing: only the accounts' own roles count.
	require.NoError(t, accounts.For(ds.DS).ApplyTemplate(accounts.System, accounts.TemplateClosed))
	for _, p := range []string{"claude-notes", "team-minutes", "claude-old"} {
		require.Equal(t, 0, c.get("createPad", map[string]string{"padID": p, "text": "hello " + p}).Code)
	}

	// the concept's claude: edits claude-*; the token asks for delete too
	c.key = c.issue(accounts.RoleEditor, "claude-*", []string{accounts.ScopePadRead, accounts.ScopePadWrite, accounts.ScopePadCreate, accounts.ScopePadDelete}, "")
	call := func(fn string, params map[string]string) (int, int) {
		status, r, _ := c.getStatus(fn, params)
		return status, r.Code
	}
	status, code := call("getText", map[string]string{"padID": "claude-notes"})
	assert.Equal(t, 200, status)
	assert.Equal(t, 0, code)
	status, code = call("setText", map[string]string{"padID": "claude-notes", "text": "new"})
	assert.Equal(t, 0, code, "write on its pads")
	status, code = call("createPad", map[string]string{"padID": "claude-new", "text": "x"})
	assert.Equal(t, 0, code, "create on its pads")
	status, code = call("getText", map[string]string{"padID": "team-minutes"})
	assert.Equal(t, 403, status, "not another's pad")
	assert.Equal(t, 4, code)
	status, _ = call("deletePad", map[string]string{"padID": "claude-old"})
	assert.Equal(t, 403, status, "delete is not the account's")
	status, _ = call("listAllPads", nil)
	assert.Equal(t, 403, status, "the list of all pads needs admin:pads")
	status, _ = call("createGroup", nil)
	assert.Equal(t, 403, status)
	status, _ = call("copyPad", map[string]string{"sourceID": "claude-notes", "destinationID": "team-copy"})
	assert.Equal(t, 403, status, "no copy out of its pads")
	status, code = call("copyPad", map[string]string{"sourceID": "claude-notes", "destinationID": "claude-copy"})
	assert.Equal(t, 0, code)
	status, _ = call("movePad", map[string]string{"sourceID": "claude-copy", "destinationID": "claude-moved"})
	assert.Equal(t, 403, status, "moving deletes the source")
	status, code = call("checkToken", nil)
	assert.Equal(t, 0, code, "any valid token")

	// a read-only token on a narrower pattern
	c.key = c.issue(accounts.RoleEditor, "claude-*", []string{accounts.ScopePadRead}, "claude-notes")
	status, code = call("getText", map[string]string{"padID": "claude-notes"})
	assert.Equal(t, 0, code)
	status, _ = call("setText", map[string]string{"padID": "claude-notes", "text": "x"})
	assert.Equal(t, 403, status)
	status, _ = call("getText", map[string]string{"padID": "claude-old"})
	assert.Equal(t, 403, status, "outside the token's pattern")

	// read-only ids are not pads to a scoped token
	c.key = admin
	roID := c.get("getReadOnlyID", map[string]string{"padID": "claude-notes"}).Data.(map[string]any)["readOnlyID"].(string)
	c.key = c.issue(accounts.RoleEditor, "", []string{accounts.ScopePadRead}, "")
	status, _ = call("getText", map[string]string{"padID": roID})
	assert.Equal(t, 403, status)
	status, code = call("getText", map[string]string{"padID": "team-minutes"})
	assert.Equal(t, 0, code, "an editor everywhere reads everywhere")
}
