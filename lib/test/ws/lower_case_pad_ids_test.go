package ws

import (
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's lowerCasePadIds spec. The setting existed but did
// nothing.
func TestLowerCasePadIds(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "socket", Test: testLowerCasePadIdsSocket},
	)
	testDb.StartTestDBHandler()
}

func collabText(vars map[string]any) string {
	cv, _ := vars["collab_client_vars"].(map[string]any)
	it, _ := cv["initialAttributedText"].(map[string]any)
	s, _ := it["text"].(string)
	return s
}

func testLowerCasePadIdsSocket(t *testing.T, ds testutils.TestDataStore) {
	cfg := padSettingsConfig(ds)
	mkPad := func(id, text string) {
		_, err := ds.PadManager.GetPad(id, &text, nil)
		require.NoError(t, err)
	}
	mkPad("ALREADYexistingPad", "oldpad")
	mkPad("alreadyexistingpad", "newpad")
	mkPad("maliciousattempt", "attempt")
	ds.Settings.LowerCasePadIDs = true
	cfg.LowerCasePadIDs = true

	t.Run("keeps old pads accessible", func(t *testing.T) {
		old := joinWithSettings(t, ds.PadMessageHandler, ds.Hub, cfg, ds, "ALREADYexistingPad", "lcOld", nil)
		assert.Equal(t, "ALREADYexistingPad", old.vars["padId"])
		assert.Equal(t, "oldpad\n", collabText(old.vars))
		fresh := joinWithSettings(t, ds.PadMessageHandler, ds.Hub, cfg, ds, "alreadyexistingpad", "lcNew", nil)
		assert.Equal(t, "alreadyexistingpad", fresh.vars["padId"])
		assert.Equal(t, "newpad\n", collabText(fresh.vars))
	})
	t.Run("no different-case pad is created through the socket", func(t *testing.T) {
		j := joinWithSettings(t, ds.PadMessageHandler, ds.Hub, cfg, ds, "MaliciousAttempt", "lcMal", nil)
		assert.Equal(t, "attempt\n", collabText(j.vars))
		exists, err := ds.PadManager.DoesPadExist("MaliciousAttempt")
		require.NoError(t, err)
		assert.False(t, *exists)
	})
}
