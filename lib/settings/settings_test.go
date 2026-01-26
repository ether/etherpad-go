package settings

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultsAreApplied(t *testing.T) {

	cfg, err := ReadConfig()
	require.NoError(t, err)

	require.Equal(t, "Etherpad", cfg.Title)
	require.Equal(t, "9001", cfg.Port)
	require.True(t, cfg.ShowRecentPads)
	require.True(t, cfg.EnableDarkMode)
	require.False(t, cfg.RequireSession)
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("ETHERPAD_PORT", "9999")

	cfg, err := ReadConfig()
	require.NoError(t, err)
	require.Equal(t, "9999", cfg.Port)
}
