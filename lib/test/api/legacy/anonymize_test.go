package legacy

import (
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's api/anonymizeAuthor spec.
func TestLegacyAnonymizeAuthor(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "anonymizeAuthor", Test: testLegacyAnonymizeAuthor},
	)
	testDb.StartTestDBHandler()
}

func testLegacyAnonymizeAuthor(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	authorID := ok(t, c.get("createAuthorIfNotExistsFor", map[string]string{"authorMapper": "lms-42", "name": "Alice"}))["authorID"].(string)

	// Off by default: an API key cannot erase authors.
	r := c.get("anonymizeAuthor", map[string]string{"authorID": authorID})
	require.Equal(t, 1, r.Code)
	assert.Contains(t, r.Message, "gdprAuthorErasure.enabled")
	assert.Equal(t, "Alice", c.get("getAuthorName", map[string]string{"authorID": authorID}).Data)

	ds.Settings.GdprAuthorErasure.Enabled = true
	failed(t, c.get("anonymizeAuthor", map[string]string{"authorID": ""}), "authorID is required")
	failed(t, c.get("anonymizeAuthor", map[string]string{"authorID": "a.doesNotExist"}), "authorID does not exist")

	data := ok(t, c.get("anonymizeAuthor", map[string]string{"authorID": authorID}))
	assert.EqualValues(t, 0, data["affectedPads"])
	assert.EqualValues(t, 1, data["removedExternalMappings"])
	assert.Contains(t, data, "removedTokenMappings")
	assert.Contains(t, data, "clearedChatMessages")
	assert.Nil(t, c.get("getAuthorName", map[string]string{"authorID": authorID}).Data, "the name is gone")

	// The mapper leads to a new author now.
	again := ok(t, c.get("createAuthorIfNotExistsFor", map[string]string{"authorMapper": "lms-42"}))["authorID"]
	assert.NotEqual(t, authorID, again)
}
