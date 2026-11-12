package pad

import (
	"testing"

	"github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPadSettingsStore(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "pad-wide settings are stored and survive a reload", Test: testPadSettingsStored},
		testutils.TestRunConfig{Name: "a pad save does not overwrite them", Test: testPadSettingsSurviveSave},
		testutils.TestRunConfig{Name: "setting them on a missing pad fails", Test: testPadSettingsMissingPad},
	)
	testDb.StartTestDBHandler()
}

var storeDefaults = pad.PadSettingsDefaults{ShowChat: true, ShowLineNumbers: true, FadeInactiveAuthorColors: true}

func testPadSettingsStored(t *testing.T, ts testutils.TestDataStore) {
	text := "settings\n"
	p, err := ts.PadManager.GetPad("settingsStored", &text, nil)
	require.NoError(t, err)
	_, err = p.SetPadSettings(map[string]any{"showChat": false, "view": map[string]any{"rtlIsTrue": true}}, storeDefaults)
	require.NoError(t, err)

	ts.PadManager.UnloadPad("settingsStored")
	reloaded, err := ts.PadManager.GetPad("settingsStored", nil, nil)
	require.NoError(t, err)
	got := reloaded.GetPadSettings(storeDefaults)
	assert.Equal(t, false, got["showChat"])
	assert.Equal(t, true, got["view"].(map[string]any)["rtlIsTrue"])
	assert.Equal(t, true, got["view"].(map[string]any)["showLineNumbers"])

	version, err := ts.DS.GetPadVersion("settingsStored")
	require.NoError(t, err)
	assert.True(t, reloaded.MatchesVersion(version))
}

func testPadSettingsSurviveSave(t *testing.T, ts testutils.TestDataStore) {
	text := "settings\n"
	p, err := ts.PadManager.GetPad("settingsSave", &text, nil)
	require.NoError(t, err)
	// A second, stale copy of the pad, as on another instance.
	stale := *p
	_, err = p.SetPadSettings(map[string]any{"enforceSettings": true}, storeDefaults)
	require.NoError(t, err)

	require.NoError(t, stale.Save())

	version, err := ts.DS.GetPadVersion("settingsSave")
	require.NoError(t, err)
	assert.Contains(t, version.Meta.Settings, `"enforceSettings":true`)
	assert.False(t, stale.MatchesVersion(version), "the stale copy notices the change")
}

func testPadSettingsMissingPad(t *testing.T, ts testutils.TestDataStore) {
	assert.Error(t, ts.DS.SetPadSettings("noSuchPadForSettings", `{"showChat":false}`))
}
