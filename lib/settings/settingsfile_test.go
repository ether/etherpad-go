package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingsFiles(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)
	file := filepath.Join(t.TempDir(), "settings.json")

	read, save, err := settingsFiles()
	require.NoError(t, err)
	assert.Equal(t, "", read, "none yet")
	assert.Equal(t, "settings.json", save, "unset: the working directory")

	require.NoError(t, os.WriteFile("settings.json", []byte("{}"), 0o644))
	read, save, err = settingsFiles()
	require.NoError(t, err)
	assert.Equal(t, "settings.json", read)
	assert.Equal(t, "settings.json", save, "read and saved in the same place")

	// ETHERPAD_SETTINGS_FILE not there yet: the working directory's file is
	// read (one mounted at /settings.json), the first save creates it.
	t.Setenv(SettingsFileEnv, file)
	read, save, err = settingsFiles()
	require.NoError(t, err)
	assert.Equal(t, "settings.json", read)
	assert.Equal(t, file, save)

	require.NoError(t, os.WriteFile(file, []byte("{}"), 0o644))
	read, save, err = settingsFiles()
	require.NoError(t, err)
	assert.Equal(t, file, read)
	assert.Equal(t, file, save)

	// ETHERPAD_SETTINGS_PATH (the root for assets, APIKEY.txt and the
	// favicon) has no say in where settings.json is.
	t.Setenv(SettingsPathEnv, t.TempDir())
	read, _, err = settingsFiles()
	require.NoError(t, err)
	assert.Equal(t, file, read)
}

// A settings file that is there but cannot be reached must stop the
// start: falling back to defaults would drop requireAuthentication.
func TestSettingsFilesRefusesUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	t.Chdir(t.TempDir())
	locked := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(locked, "settings.json"), []byte(`{"requireAuthentication": true}`), 0o644))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	t.Setenv(SettingsFileEnv, filepath.Join(locked, "settings.json"))

	_, _, err := settingsFiles()
	assert.ErrorIs(t, err, os.ErrPermission)
	_, err = ReadConfig()
	assert.Error(t, err)
}

func TestReadConfigReadsTheSettingsFile(t *testing.T) {
	t.Chdir(t.TempDir())
	file := filepath.Join(t.TempDir(), "settings.json")
	t.Setenv(SettingsFileEnv, file)
	require.NoError(t, os.WriteFile(file,
		[]byte(`{ "title": "From the settings file", /* comment */ }`), 0o644))
	s, err := ReadConfig()
	require.NoError(t, err)
	assert.Equal(t, "From the settings file", s.Title)
	assert.Equal(t, file, s.SettingsFile)
	assert.Equal(t, file, s.SettingsSaveFile)
}
