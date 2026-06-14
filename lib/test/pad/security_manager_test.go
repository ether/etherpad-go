package pad

import (
	"testing"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityManagerHooks(t *testing.T) {
	testHandler := testutils.NewTestDBHandler(t)
	testHandler.AddTests(
		testutils.TestRunConfig{
			Name: "onAccessCheck deny returns error",
			Test: testOnAccessCheckDenyReturnsError,
		},
		testutils.TestRunConfig{
			Name: "getAuthorId override sets GrantedAccess AuthorId",
			Test: testGetAuthorIdOverride,
		},
		testutils.TestRunConfig{
			Name: "default no-hooks path grants access with DB-resolved author",
			Test: testDefaultNoHooksGrantsAccess,
		},
	)

	defer testHandler.StartTestDBHandler()
}

// testOnAccessCheckDenyReturnsError registers an onAccessCheck hook that calls
// ctx.Deny() and asserts that CheckAccess returns a non-nil error and nil access.
func testOnAccessCheckDenyReturnsError(t *testing.T, ds testutils.TestDataStore) {
	prevLoadTest := settings.Displayed.LoadTest
	settings.Displayed.LoadTest = true
	defer func() { settings.Displayed.LoadTest = prevLoadTest }()

	// Register a hook on the same hooks instance the SecurityManager uses.
	hookId := ds.Hooks.EnqueueOnAccessCheckHook(func(ctx *events.OnAccessCheckContext) {
		ctx.Deny()
	})
	defer ds.Hooks.DequeueHook(hooks.OnAccessCheckString, hookId)

	padId := "testpad-deny"
	token := "t.denytesttoken12345678"

	granted, err := ds.SecurityManager.CheckAccess(&padId, nil, &token, nil)
	assert.Nil(t, granted, "expected nil GrantedAccess when hook denies")
	assert.Error(t, err, "expected error when hook denies access")
	assert.Contains(t, err.Error(), "onAccessCheck hook denied access")
}

// testGetAuthorIdOverride registers a getAuthorId hook that sets a custom author id
// and asserts that GrantedAccess.AuthorId equals that custom id.
func testGetAuthorIdOverride(t *testing.T, ds testutils.TestDataStore) {
	prevLoadTest := settings.Displayed.LoadTest
	settings.Displayed.LoadTest = true
	defer func() { settings.Displayed.LoadTest = prevLoadTest }()

	const customAuthorId = "a.customFromPlugin"

	hookId := ds.Hooks.EnqueueGetAuthorIdHook(func(ctx *events.GetAuthorIdContext) {
		ctx.SetAuthorId(customAuthorId)
	})
	defer ds.Hooks.DequeueHook(hooks.GetAuthorIdString, hookId)

	padId := "testpad-authoroverride"
	token := "t.authoroverride12345678"

	granted, err := ds.SecurityManager.CheckAccess(&padId, nil, &token, nil)
	require.NoError(t, err)
	require.NotNil(t, granted)
	assert.Equal(t, "grant", granted.AccessStatus)
	assert.Equal(t, customAuthorId, granted.AuthorId)
}

// testDefaultNoHooksGrantsAccess verifies that with no hooks registered,
// CheckAccess falls back to the DB token->author mapping and grants access
// with a non-empty AuthorId.
func testDefaultNoHooksGrantsAccess(t *testing.T, ds testutils.TestDataStore) {
	prevLoadTest := settings.Displayed.LoadTest
	settings.Displayed.LoadTest = true
	defer func() { settings.Displayed.LoadTest = prevLoadTest }()

	padId := "testpad-nohooks"
	token := "t.nohookstoken12345678901"

	granted, err := ds.SecurityManager.CheckAccess(&padId, nil, &token, nil)
	require.NoError(t, err)
	require.NotNil(t, granted)
	assert.Equal(t, "grant", granted.AccessStatus)
	assert.NotEmpty(t, granted.AuthorId)
}
