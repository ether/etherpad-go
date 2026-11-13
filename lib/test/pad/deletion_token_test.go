package pad

import (
	"sync"
	"testing"

	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's padDeletionManager spec, plus removal and moves.
func TestDeletionTokens(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "createDeletionTokenIfAbsent", Test: testCreateDeletionToken},
		testutils.TestRunConfig{Name: "isValidDeletionToken", Test: testIsValidDeletionToken},
		testutils.TestRunConfig{Name: "removeDeletionToken", Test: testRemoveDeletionToken},
		testutils.TestRunConfig{Name: "removing the pad removes its token", Test: testRemovePadRemovesToken},
		testutils.TestRunConfig{Name: "transfer to a moved pad", Test: testTransferDeletionToken},
	)
	testDb.StartTestDBHandler()
}

func testCreateDeletionToken(t *testing.T, ts testutils.TestDataStore) {
	tokens := pad.NewDeletionTokens(ts.DS)

	first, err := tokens.CreateIfAbsent("dtFirst")
	require.NoError(t, err)
	require.NotNil(t, first, "returns a token on the first call")
	assert.GreaterOrEqual(t, len(*first), 32)

	again, err := tokens.CreateIfAbsent("dtFirst")
	require.NoError(t, err)
	assert.Nil(t, again, "returns nothing on later calls for the same pad")

	other, err := tokens.CreateIfAbsent("dtOther")
	require.NoError(t, err)
	require.NotNil(t, other)
	assert.NotEqual(t, *first, *other, "different pads get different tokens")

	// Concurrent calls for one pad hand out at most one token, and it works.
	var wg sync.WaitGroup
	results := make([]*string, 8)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], _ = tokens.CreateIfAbsent("dtConcurrent")
		}(i)
	}
	wg.Wait()
	var handedOut []string
	for _, r := range results {
		if r != nil {
			handedOut = append(handedOut, *r)
		}
	}
	require.Len(t, handedOut, 1)
	assert.True(t, tokens.IsValid("dtConcurrent", handedOut[0]))
}

func testIsValidDeletionToken(t *testing.T, ts testutils.TestDataStore) {
	tokens := pad.NewDeletionTokens(ts.DS)
	token, err := tokens.CreateIfAbsent("dtValid")
	require.NoError(t, err)
	require.NotNil(t, token)

	assert.True(t, tokens.IsValid("dtValid", *token), "accepts the pad's token")
	other, _ := tokens.CreateIfAbsent("dtValidOther")
	assert.False(t, tokens.IsValid("dtValidOther", *token), "rejects a token of another pad")
	assert.True(t, tokens.IsValid("dtValidOther", *other))
	assert.False(t, tokens.IsValid("dtValid", ""), "rejects an empty token")
	assert.False(t, tokens.IsValid("dtValid", *token+"x"))
	assert.False(t, tokens.IsValid("dtNever", *token), "false for a pad that never had a token")

	// Only the hash is stored.
	raw, err := ts.DS.GetOIDCStorageValue("pad:dtValid:deletionToken")
	require.NoError(t, err)
	require.NotNil(t, raw)
	assert.NotContains(t, *raw, *token)
}

func testRemoveDeletionToken(t *testing.T, ts testutils.TestDataStore) {
	tokens := pad.NewDeletionTokens(ts.DS)
	token, err := tokens.CreateIfAbsent("dtRemove")
	require.NoError(t, err)
	require.NoError(t, tokens.Remove("dtRemove"))
	assert.False(t, tokens.IsValid("dtRemove", *token), "invalidates the token")
	assert.NoError(t, tokens.Remove("dtNoToken"), "safe when there is none")
}

func testRemovePadRemovesToken(t *testing.T, ts testutils.TestDataStore) {
	text := "x\n"
	_, err := ts.PadManager.GetPad("dtPadRemove", &text, nil)
	require.NoError(t, err)
	tokens := pad.NewDeletionTokens(ts.DS)
	token, err := tokens.CreateIfAbsent("dtPadRemove")
	require.NoError(t, err)

	require.NoError(t, ts.PadManager.RemovePad("dtPadRemove"))
	assert.False(t, tokens.IsValid("dtPadRemove", *token))
	// A new pad under the same id gets a token of its own.
	fresh, err := tokens.CreateIfAbsent("dtPadRemove")
	require.NoError(t, err)
	assert.NotNil(t, fresh)
}

func testTransferDeletionToken(t *testing.T, ts testutils.TestDataStore) {
	tokens := pad.NewDeletionTokens(ts.DS)
	token, err := tokens.CreateIfAbsent("dtMoveSrc")
	require.NoError(t, err)
	require.NoError(t, tokens.Transfer("dtMoveSrc", "dtMoveDst"))
	assert.True(t, tokens.IsValid("dtMoveDst", *token), "the saved token works for the moved pad")
	again, _ := tokens.CreateIfAbsent("dtMoveDst")
	assert.Nil(t, again, "and the creator is not handed a second one")

	// A destination that already handed out a token keeps it.
	dstToken, _ := tokens.CreateIfAbsent("dtMoveDst2")
	require.NoError(t, tokens.Transfer("dtMoveSrc", "dtMoveDst2"))
	assert.True(t, tokens.IsValid("dtMoveDst2", *dstToken))
	assert.False(t, tokens.IsValid("dtMoveDst2", *token))

	// Nothing to transfer: no error, nothing created.
	require.NoError(t, tokens.Transfer("dtNoSrc", "dtMoveDst3"))
	fresh, _ := tokens.CreateIfAbsent("dtMoveDst3")
	assert.NotNil(t, fresh)
}
