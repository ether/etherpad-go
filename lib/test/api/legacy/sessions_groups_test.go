package legacy

import (
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's api/sessionsAndGroups spec.
func TestLegacySessionsAndGroups(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "groups, authors, sessions, group pads", Test: testLegacySessionsAndGroups},
		testutils.TestRunConfig{Name: "deleteGroup after deleteSession (#5798)", Test: testLegacyDeleteGroupAfterSession},
		testutils.TestRunConfig{Name: "parameter checks", Test: testLegacySessionParamChecks},
	)
	testDb.StartTestDBHandler()
}

func testLegacySessionsAndGroups(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)

	// Group creation and deletion
	groupID := ok(t, c.get("createGroup", nil))["groupID"].(string)
	require.NotEmpty(t, groupID)
	r := c.get("listSessionsOfGroup", map[string]string{"groupID": groupID})
	ok(t, r)
	assert.Nil(t, r.Data, "no sessions: data is null")
	ok(t, c.get("deleteGroup", map[string]string{"groupID": groupID}))

	mapper := makeID()
	mapped := ok(t, c.get("createGroupIfNotExistsFor", map[string]string{"groupMapper": mapper}))["groupID"]
	require.NotEmpty(t, mapped)
	assert.Equal(t, mapped, ok(t, c.get("createGroupIfNotExistsFor", map[string]string{"groupMapper": mapper}))["groupID"])
	// etherpad-lite forgets the mapping when the group is deleted, so the
	// mapper gets a new group. etherpad-go derives the group id from the
	// mapper: the mapper gets the same id again, of a new, empty group.
	ok(t, c.get("createGroupPad", map[string]string{"groupID": mapped.(string), "padName": "kept"}))
	ok(t, c.get("deleteGroup", map[string]string{"groupID": mapped.(string)}))
	failed(t, c.get("getText", map[string]string{"padID": mapped.(string) + "$kept"}), "padID does not exist")
	again := ok(t, c.get("createGroupIfNotExistsFor", map[string]string{"groupMapper": mapper}))["groupID"]
	assert.Len(t, ok(t, c.get("listPads", map[string]string{"groupID": again.(string)}))["padIDs"], 0,
		"the pads of the deleted group do not come back")

	groupID = ok(t, c.get("createGroup", nil))["groupID"].(string)
	authorID := ok(t, c.get("createAuthor", nil))["authorID"].(string)
	require.NotEmpty(t, authorID)
	for i := 0; i < 2; i++ {
		assert.NotEmpty(t, ok(t, c.get("createSession", map[string]string{
			"authorID": authorID, "groupID": groupID, "validUntil": "999999999999",
		}))["sessionID"])
	}
	ok(t, c.get("createGroupPad", map[string]string{"groupID": groupID, "padName": "x1234567"}))
	ok(t, c.get("createGroupPad", map[string]string{"groupID": groupID, "padName": "x12345678"}))
	ok(t, c.get("deleteGroup", map[string]string{"groupID": groupID}))

	// Author creation
	groupID = ok(t, c.get("createGroup", nil))["groupID"].(string)
	assert.NotEmpty(t, ok(t, c.get("createAuthor", nil))["authorID"])
	authorID = ok(t, c.get("createAuthor", map[string]string{"name": "john"}))["authorID"].(string)
	assert.NotEmpty(t, ok(t, c.get("createAuthorIfNotExistsFor", map[string]string{"authorMapper": "chris"}))["authorID"])
	r = c.get("getAuthorName", map[string]string{"authorID": authorID})
	ok(t, r)
	assert.Equal(t, "john", r.Data, "data is the name itself")

	// Sessions
	sessionID := ok(t, c.get("createSession", map[string]string{
		"authorID": authorID, "groupID": groupID, "validUntil": "999999999999",
	}))["sessionID"].(string)
	info := ok(t, c.get("getSessionInfo", map[string]string{"sessionID": sessionID}))
	assert.Equal(t, groupID, info["groupID"])
	assert.Equal(t, authorID, info["authorID"])
	assert.EqualValues(t, 999999999999, info["validUntil"])
	sessions := ok(t, c.get("listSessionsOfGroup", map[string]string{"groupID": groupID}))
	require.Contains(t, sessions, sessionID, "a map from session id to its info")
	assert.Equal(t, authorID, sessions[sessionID].(map[string]any)["authorID"])
	ok(t, c.get("deleteSession", map[string]string{"sessionID": sessionID}))
	failed(t, c.get("getSessionInfo", map[string]string{"sessionID": sessionID}), "sessionID does not exist")

	// Group pad management
	padName := makeID()
	assert.Len(t, ok(t, c.get("listPads", map[string]string{"groupID": groupID}))["padIDs"], 0)
	padID := ok(t, c.get("createGroupPad", map[string]string{"groupID": groupID, "padName": padName}))["padID"].(string)
	assert.Equal(t, groupID+"$"+padName, padID)
	assert.Len(t, ok(t, c.get("listPads", map[string]string{"groupID": groupID}))["padIDs"], 1)
	failed(t, c.get("createGroupPad", map[string]string{"groupID": groupID, "padName": padName}), "padName does already exist")

	// Pad security
	assert.Equal(t, false, ok(t, c.get("getPublicStatus", map[string]string{"padID": padID}))["publicStatus"])
	ok(t, c.get("setPublicStatus", map[string]string{"padID": padID, "publicStatus": "true"}))
	assert.Equal(t, true, ok(t, c.get("getPublicStatus", map[string]string{"padID": padID}))["publicStatus"])

	// Misc
	assert.Len(t, ok(t, c.get("listPadsOfAuthor", map[string]string{"authorID": authorID}))["padIDs"], 0)
	assert.Contains(t, ok(t, c.get("listAllGroups", nil))["groupIDs"], groupID)
}

func testLegacyDeleteGroupAfterSession(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	groupID := ok(t, c.get("createGroup", nil))["groupID"].(string)
	authorID := ok(t, c.get("createAuthor", map[string]string{"name": "testuser"}))["authorID"].(string)
	sessionID := ok(t, c.get("createSession", map[string]string{
		"authorID": authorID, "groupID": groupID, "validUntil": "999999999999",
	}))["sessionID"].(string)
	ok(t, c.get("deleteSession", map[string]string{"sessionID": sessionID}))
	other := ok(t, c.get("createSession", map[string]string{
		"authorID": authorID, "groupID": groupID, "validUntil": "999999999999",
	}))["sessionID"].(string)
	ok(t, c.get("deleteGroup", map[string]string{"groupID": groupID}))
	failed(t, c.get("getSessionInfo", map[string]string{"sessionID": other}), "sessionID does not exist")
}

func testLegacySessionParamChecks(t *testing.T, ds testutils.TestDataStore) {
	c := newClient(t, ds)
	groupID := ok(t, c.get("createGroup", nil))["groupID"].(string)
	authorID := ok(t, c.get("createAuthor", nil))["authorID"].(string)
	session := func(validUntil string) map[string]string {
		return map[string]string{"authorID": authorID, "groupID": groupID, "validUntil": validUntil}
	}
	failed(t, c.get("createSession", session("abc")), "validUntil is not a number")
	failed(t, c.get("createSession", session("-1")), "validUntil is a negative number")
	failed(t, c.get("createSession", session("99999999999.5")), "validUntil is a float value")
	failed(t, c.get("createSession", session("1")), "validUntil is in the past")
	failed(t, c.get("createSession", map[string]string{"authorID": authorID, "groupID": "g.nope", "validUntil": "999999999999"}),
		"groupID does not exist")
	failed(t, c.get("createSession", map[string]string{"authorID": "a.nope", "groupID": groupID, "validUntil": "999999999999"}),
		"authorID does not exist")
	failed(t, c.get("deleteGroup", map[string]string{"groupID": "g.nope"}), "groupID does not exist")
	failed(t, c.get("listPads", map[string]string{"groupID": "g.nope"}), "groupID does not exist")
	failed(t, c.get("createGroupIfNotExistsFor", nil), "groupMapper is not a string")
	failed(t, c.get("deleteSession", map[string]string{"sessionID": "s.nope"}), "sessionID does not exist")
}
