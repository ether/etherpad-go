package pad

import (
	"testing"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite src/tests/backend/specs/nullAuthorToken.ts
// (GHSA-746x-9w89-7xv6): a caller without an author token must not be mapped
// to one shared author.
func TestNullAuthorToken(t *testing.T) {
	for _, loadTest := range []bool{false, true} {
		name := "normal"
		if loadTest {
			name = "loadTest"
		}
		t.Run(name, func(t *testing.T) {
			for _, conf := range []testutils.TestRunConfig{
				{Name: "checkAccess does not map a missing token to a shared author", Test: func(t *testing.T, ds testutils.TestDataStore) {
					ds.Settings.LoadTest = loadTest
					testMissingTokenGetsOwnAuthor(t, ds)
				}},
				{Name: "missing token still reaches getAuthorId hooks as empty", Test: func(t *testing.T, ds testutils.TestDataStore) {
					ds.Settings.LoadTest = loadTest
					testMissingTokenReachesHooksEmpty(t, ds)
				}},
				{Name: "malformed token is still rejected", Test: func(t *testing.T, ds testutils.TestDataStore) {
					ds.Settings.LoadTest = loadTest
					testMalformedTokenRejected(t, ds, loadTest)
				}},
			} {
				testutils.RunSequentialMemorySQLite(t, conf)
			}
		})
	}
}

func testMissingTokenGetsOwnAuthor(t *testing.T, ds testutils.TestDataStore) {
	padId := "nullToken-shared-author"
	empty := ""
	a, err := ds.SecurityManager.CheckAccess(&padId, nil, nil, nil)
	require.NoError(t, err)
	b, err := ds.SecurityManager.CheckAccess(&padId, nil, &empty, nil)
	require.NoError(t, err)
	c, err := ds.SecurityManager.CheckAccess(&padId, nil, &empty, nil)
	require.NoError(t, err)
	assert.Equal(t, "grant", a.AccessStatus)
	assert.Equal(t, "grant", b.AccessStatus)
	assert.NotEqual(t, a.AuthorId, b.AuthorId)
	assert.NotEqual(t, b.AuthorId, c.AuthorId)

	// A real token keeps resolving to the same author.
	token := "t.nullTokenStableAuthor1"
	d1, err := ds.SecurityManager.CheckAccess(&padId, nil, &token, nil)
	require.NoError(t, err)
	d2, err := ds.SecurityManager.CheckAccess(&padId, nil, &token, nil)
	require.NoError(t, err)
	assert.Equal(t, d1.AuthorId, d2.AuthorId)
}

func testMissingTokenReachesHooksEmpty(t *testing.T, ds testutils.TestDataStore) {
	var seen []string
	hookId := ds.Hooks.EnqueueGetAuthorIdHook(func(ctx *events.GetAuthorIdContext) {
		seen = append(seen, ctx.Token)
	})
	defer ds.Hooks.DequeueHook(hooks.GetAuthorIdString, hookId)

	padId := "nullToken-hook"
	_, err := ds.SecurityManager.CheckAccess(&padId, nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{""}, seen, "hooks must see the missing token, not the generated one")
}

func testMalformedTokenRejected(t *testing.T, ds testutils.TestDataStore, loadTest bool) {
	if loadTest {
		t.Skip("load-test mode skips token validation by design")
	}
	padId := "nullToken-malformed"
	bad := "not-a-token"
	granted, err := ds.SecurityManager.CheckAccess(&padId, nil, &bad, nil)
	assert.Error(t, err)
	assert.Nil(t, granted)
}
