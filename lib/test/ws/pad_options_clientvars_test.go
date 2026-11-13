package ws

import (
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The padOptions of settings.json reach the client; CLIENT_VARS used to
// carry hard-coded defaults whatever the settings said.
func TestPadOptionsFromSettings(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "padOptions", Test: func(t *testing.T, ds testutils.TestDataStore) {
		cfg := padSettingsConfig(ds)
		name, color := "Preset Name", "#123456"
		cfg.PadOptions.ShowChat = false
		cfg.PadOptions.NoColors = true
		cfg.PadOptions.UserName = &name
		cfg.PadOptions.UserColor = &color
		j := joinWithSettings(t, ds.PadMessageHandler, ds.Hub, cfg, ds, "padOptionsPad", "padOpt", nil)
		opts, ok := j.vars["padOptions"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, false, opts["showChat"])
		assert.Equal(t, true, opts["noColors"])
		assert.Equal(t, "Preset Name", opts["userName"])
		assert.Equal(t, "#123456", opts["userColor"])
		assert.Equal(t, true, opts["showLineNumbers"])
	}})
	testDb.StartTestDBHandler()
}
