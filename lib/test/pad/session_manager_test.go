package pad

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionManager(t *testing.T) {
	testHandler := testutils.NewTestDBHandler(t)
	testHandler.AddTests(
		testutils.TestRunConfig{
			Name: "FindAuthorID returns author of a valid session",
			Test: testFindAuthorIDValidSession,
		},
		testutils.TestRunConfig{
			Name: "FindAuthorID copes with quoted multi-id cookies",
			Test: testFindAuthorIDQuotedMultiCookie,
		},
		testutils.TestRunConfig{
			Name: "FindAuthorID rejects expired sessions and wrong groups",
			Test: testFindAuthorIDExpiredAndWrongGroup,
		},
		testutils.TestRunConfig{
			Name: "CheckAccess gates private group pads on a session",
			Test: testCheckAccessPrivateGroupPad,
		},
	)

	defer testHandler.StartTestDBHandler()
}

func setupSessionFixture(t *testing.T, ds testutils.TestDataStore, groupId string) (*pad.SessionManager, string) {
	t.Helper()
	require.NoError(t, ds.DS.SaveGroup(groupId))
	createdAuthor, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	return pad.NewSessionManager(ds.DS), createdAuthor.Id
}

func testFindAuthorIDValidSession(t *testing.T, ds testutils.TestDataStore) {
	groupId := "g.findauthor1234567"[:18]
	sm, authorId := setupSessionFixture(t, ds, groupId)

	sessionId, err := sm.CreateSession(groupId, authorId, time.Now().Unix()+3600)
	require.NoError(t, err)
	require.Equal(t, "s.", sessionId[:2])

	found := sm.FindAuthorID(groupId, &sessionId)
	require.NotNil(t, found)
	assert.Equal(t, authorId, *found)

	// No cookie -> no author
	assert.Nil(t, sm.FindAuthorID(groupId, nil))

	exists, err := sm.DoesSessionExist(sessionId)
	require.NoError(t, err)
	assert.True(t, exists)
	exists, err = sm.DoesSessionExist("s.doesnotexist12345")
	require.NoError(t, err)
	assert.False(t, exists)
}

func testFindAuthorIDQuotedMultiCookie(t *testing.T, ds testutils.TestDataStore) {
	groupId := "g.findquoted123456"[:18]
	sm, authorId := setupSessionFixture(t, ds, groupId)

	sessionId, err := sm.CreateSession(groupId, authorId, time.Now().Unix()+3600)
	require.NoError(t, err)

	// RFC 6265 servers may quote the cookie value; it may also carry a
	// comma-separated list of session ids (upstream #3819).
	cookie := `"s.unknown1234567890,` + sessionId + `"`
	found := sm.FindAuthorID(groupId, &cookie)
	require.NotNil(t, found)
	assert.Equal(t, authorId, *found)
}

func testFindAuthorIDExpiredAndWrongGroup(t *testing.T, ds testutils.TestDataStore) {
	groupId := "g.findexpired12345"[:18]
	otherGroup := "g.othergroup123456"[:18]
	sm, authorId := setupSessionFixture(t, ds, groupId)
	require.NoError(t, ds.DS.SaveGroup(otherGroup))

	expiredId, err := sm.CreateSession(groupId, authorId, time.Now().Unix()-10)
	require.NoError(t, err)
	assert.Nil(t, sm.FindAuthorID(groupId, &expiredId), "expired session must not grant access")

	validId, err := sm.CreateSession(groupId, authorId, time.Now().Unix()+3600)
	require.NoError(t, err)
	assert.Nil(t, sm.FindAuthorID(otherGroup, &validId), "session of another group must not match")
}

func testCheckAccessPrivateGroupPad(t *testing.T, ds testutils.TestDataStore) {
	prevLoadTest := settings.Displayed.LoadTest
	settings.Displayed.LoadTest = false
	defer func() { settings.Displayed.LoadTest = prevLoadTest }()

	groupId := "g.checkaccess12345"[:18]
	sm, authorId := setupSessionFixture(t, ds, groupId)

	// Create a private (default) group pad.
	padId := groupId + "$secret"
	_, err := ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)

	token := "t.checkaccesstoken123456"

	// Without a session cookie access to the private group pad is denied.
	_, err = ds.SecurityManager.CheckAccess(&padId, nil, &token, nil)
	assert.Error(t, err)

	// With a valid session for the pad's group access is granted.
	sessionId, err := sm.CreateSession(groupId, authorId, time.Now().Unix()+3600)
	require.NoError(t, err)
	granted, err := ds.SecurityManager.CheckAccess(&padId, &sessionId, &token, nil)
	require.NoError(t, err)
	require.NotNil(t, granted)
	assert.Equal(t, "grant", granted.AccessStatus)
}

func TestNormalizeAuthzLevel(t *testing.T) {
	// readOnly and modify used to fall through the switch and be rejected
	// (missing fallthrough semantics in the Go port); only create worked.
	for _, level := range []string{"readOnly", "modify", "create"} {
		normalized, err := pad.NormalizeAuthzLevel(level)
		require.NoError(t, err, level)
		require.NotNil(t, normalized, level)
		assert.Equal(t, level, *normalized)
	}

	// Original: true normalizes to "create".
	normalized, err := pad.NormalizeAuthzLevel(true)
	require.NoError(t, err)
	require.NotNil(t, normalized)
	assert.Equal(t, "create", *normalized)

	// false / unknown levels are denied.
	_, err = pad.NormalizeAuthzLevel(false)
	assert.Error(t, err)
	_, err = pad.NormalizeAuthzLevel("bogus")
	assert.Error(t, err)
}

func TestUserCanModifyNilAuthorizations(t *testing.T) {
	// Used to panic("This should not happen") on a nil PadAuthorizations map;
	// must deny instead of crashing the request.
	prevRequireAuth := settings.Displayed.RequireAuthentication
	settings.Displayed.RequireAuthentication = true
	defer func() { settings.Displayed.RequireAuthentication = prevRequireAuth }()

	padId := "normalpad"
	readOnly := false
	req := &webaccess.SocketClientRequest{ReadOnly: &readOnly, PadAuthorizations: nil}

	memDS := db.NewMemoryDataStore()
	rom := pad.NewReadOnlyManager(memDS)

	assert.NotPanics(t, func() {
		assert.False(t, pad.UserCanModify(&padId, req, *rom))
	})
}
