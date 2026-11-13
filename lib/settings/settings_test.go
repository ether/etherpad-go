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

// Port of etherpad-lite's settings spec, "Parse plugin settings": EP__a__b
// sets a nested setting, typed.
func TestNestedEnvSettings(t *testing.T) {
	t.Setenv("EP__padOptions__showChat", "false")
	t.Setenv("EP__padOptions__userName", "From Env")
	t.Setenv("EP__title", "Nested")
	t.Setenv("EP__updates__checkIntervalHours", "24")
	t.Setenv("EP____broken", "x")
	cfg, err := ReadConfig()
	require.NoError(t, err)
	require.False(t, cfg.PadOptions.ShowChat)
	require.NotNil(t, cfg.PadOptions.UserName)
	require.Equal(t, "From Env", *cfg.PadOptions.UserName)
	require.True(t, cfg.PadOptions.ShowLineNumbers, "the rest of the section keeps its values")
	require.Equal(t, "Nested", cfg.Title)
	require.Equal(t, 24, cfg.Updates.CheckIntervalHours)
}

// Port of etherpad-lite's settings spec, air-gapped installations (#7911):
// the only outbound calls (release checks) are switched off from the
// environment, without a settings.json.
func TestOfflineFromEnv(t *testing.T) {
	cfg, err := ReadConfig()
	require.NoError(t, err)
	require.Equal(t, "notify", cfg.Updates.Tier, "default")
	require.Equal(t, 6, cfg.Updates.CheckIntervalHours)

	t.Setenv("ETHERPAD_UPDATES_TIER", "off")
	t.Setenv("ETHERPAD_UPDATES_CHECKINTERVALHOURS", "24")
	t.Setenv("ETHERPAD_UPDATES_GITHUBREPO", "acme/etherpad-fork")
	cfg, err = ReadConfig()
	require.NoError(t, err)
	require.Equal(t, "off", cfg.Updates.Tier)
	require.Equal(t, 24, cfg.Updates.CheckIntervalHours)
	require.Equal(t, "acme/etherpad-fork", cfg.Updates.GithubRepo)
}

// padOptions.fadeInactiveAuthorColors defaults to true (#7138).
func TestFadeInactiveAuthorColorsDefault(t *testing.T) {
	cfg, err := ReadConfig()
	require.NoError(t, err)
	require.True(t, cfg.PadOptions.FadeInactiveAuthorColors)
}
