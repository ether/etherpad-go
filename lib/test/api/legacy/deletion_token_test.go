package legacy

import (
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ports of etherpad-lite's api/deletePad and api/movePadDeletionToken specs.
func TestLegacyDeletionToken(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "deletePad and deletion tokens", Test: testLegacyDeletePadToken},
		testutils.TestRunConfig{Name: "no token when requireAuthentication is on", Test: testLegacyNoTokenWithAuth},
		testutils.TestRunConfig{Name: "no token when everyone may delete", Test: testLegacyNoTokenAllMayDelete},
		testutils.TestRunConfig{Name: "movePad and copyPad with deletion tokens", Test: testLegacyMovePadToken},
	)
	testDb.StartTestDBHandler()
}

func token(t *testing.T, c *client, padID string) string {
	t.Helper()
	tok, _ := ok(t, c.get("createPad", map[string]string{"padID": padID}))["deletionToken"].(string)
	return tok
}

func testLegacyDeletePadToken(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	padID := makeID()
	tok := token(t, c, padID)
	assert.GreaterOrEqual(t, len(tok), 32)
	ok(t, c.get("deletePad", map[string]string{"padID": padID, "deletionToken": tok}))
	failed(t, c.get("getText", map[string]string{"padID": padID}), "padID does not exist")

	padID = makeID()
	token(t, c, padID)
	r := c.get("deletePad", map[string]string{"padID": padID, "deletionToken": "not-the-real-token"})
	failed(t, r, "")
	assert.Contains(t, r.Message, "invalid deletionToken")
	// the API caller is trusted when it gives no token
	ok(t, c.get("deletePad", map[string]string{"padID": padID}))

	ds.Settings.AllowPadDeletionByAllUsers = true
	padID = makeID()
	token(t, c, padID)
	ok(t, c.get("deletePad", map[string]string{"padID": padID, "deletionToken": "bogus"}))
}

func testLegacyNoTokenWithAuth(t *testing.T, ds testutils.TestDataStore) {
	ds.Settings.RequireAuthentication = true
	c := newClient(t, ds)
	padID := makeID()
	data := ok(t, c.get("createPad", map[string]string{"padID": padID}))
	require.Contains(t, data, "deletionToken")
	assert.Nil(t, data["deletionToken"])
}

func testLegacyNoTokenAllMayDelete(t *testing.T, ds testutils.TestDataStore) {
	ds.Settings.AllowPadDeletionByAllUsers = true
	c := newClient(t, ds)
	padID := makeID()
	data := ok(t, c.get("createPad", map[string]string{"padID": padID}))
	require.Contains(t, data, "deletionToken")
	assert.Nil(t, data["deletionToken"])
}

func testLegacyMovePadToken(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)

	// movePad carries the token to the destination (#7995)
	src := makeID()
	tok := token(t, c, src)
	ok(t, c.get("movePad", map[string]string{"sourceID": src, "destinationID": src + "_dst"}))
	ok(t, c.get("deletePad", map[string]string{"padID": src + "_dst", "deletionToken": tok}))

	// movePad --force replaces the destination's own token
	src = makeID()
	dst := src + "_existing"
	srcTok := token(t, c, src)
	dstTok := token(t, c, dst)
	require.NotEqual(t, srcTok, dstTok)
	ok(t, c.get("movePad", map[string]string{"sourceID": src, "destinationID": dst, "force": "true"}))
	failed(t, c.get("deletePad", map[string]string{"padID": dst, "deletionToken": dstTok}), "invalid deletionToken")
	ok(t, c.get("deletePad", map[string]string{"padID": dst, "deletionToken": srcTok}))

	// copyPad does not share the token with the copy
	src = makeID()
	tok = token(t, c, src)
	ok(t, c.get("copyPad", map[string]string{"sourceID": src, "destinationID": src + "_copy"}))
	failed(t, c.get("deletePad", map[string]string{"padID": src + "_copy", "deletionToken": tok}), "invalid deletionToken")
}
