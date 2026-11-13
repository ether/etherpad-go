package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's padOptionsLegacyDefaults: padOptions.userName and
// userColor are strings; the false that old settings files wrote means "no
// preset". They used to be *bool here, so a configured name did not load.
func TestPadOptionsLegacyDefaults(t *testing.T) {
	read := func(t *testing.T, json string) *Settings {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.json"), []byte(json), 0o600))
		t.Chdir(dir)
		cfg, err := ReadConfig()
		require.NoError(t, err)
		return cfg
	}
	t.Run("false becomes no preset", func(t *testing.T) {
		cfg := read(t, `{"padOptions": {"userName": false, "userColor": false}}`)
		assert.Nil(t, cfg.PadOptions.UserName)
		assert.Nil(t, cfg.PadOptions.UserColor)
	})
	t.Run("strings stay", func(t *testing.T) {
		cfg := read(t, `{"padOptions": {"userName": "Etherpad User", "userColor": "#ff9900"}}`)
		require.NotNil(t, cfg.PadOptions.UserName)
		assert.Equal(t, "Etherpad User", *cfg.PadOptions.UserName)
		require.NotNil(t, cfg.PadOptions.UserColor)
		assert.Equal(t, "#ff9900", *cfg.PadOptions.UserColor)
	})
	t.Run("null stays null", func(t *testing.T) {
		cfg := read(t, `{"padOptions": {"userName": null, "userColor": null}}`)
		assert.Nil(t, cfg.PadOptions.UserName)
		assert.Nil(t, cfg.PadOptions.UserColor)
	})
	t.Run("other options that use false are untouched", func(t *testing.T) {
		cfg := read(t, `{"padOptions": {"showChat": false, "showLineNumbers": false, "noColors": true}}`)
		assert.False(t, cfg.PadOptions.ShowChat)
		assert.False(t, cfg.PadOptions.ShowLineNumbers)
		assert.True(t, cfg.PadOptions.NoColors)
		assert.True(t, cfg.PadOptions.ShowControls, "defaults for the rest")
	})
	t.Run("the string \"false\" is left to the client", func(t *testing.T) {
		cfg := read(t, `{"padOptions": {"userName": "false"}}`)
		require.NotNil(t, cfg.PadOptions.UserName)
		assert.Equal(t, "false", *cfg.PadOptions.UserName)
	})
}
