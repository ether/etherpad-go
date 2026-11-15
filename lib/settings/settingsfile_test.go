package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingsFile(t *testing.T) {
	wd := t.TempDir()
	dir := t.TempDir()
	t.Chdir(wd)

	assert.Equal(t, "settings.json", SettingsFile(""), "no directory: the working directory")
	assert.Equal(t, "settings.json", SettingsWriteFile(""))

	// A directory without a settings.json: still the one in the working
	// directory (a file mounted at /settings.json), but saves go to dir.
	require.NoError(t, os.WriteFile(filepath.Join(wd, "settings.json"), []byte("{}"), 0o644))
	assert.Equal(t, "settings.json", SettingsFile(dir))
	assert.Equal(t, filepath.Join(dir, "settings.json"), SettingsWriteFile(dir))

	// Once saved there, it is the one read.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}"), 0o644))
	assert.Equal(t, filepath.Join(dir, "settings.json"), SettingsFile(dir))
	assert.Equal(t, filepath.Join(dir, "settings.json"), SettingsWriteFile(dir))
}

func TestReadConfigReadsTheSettingsPath(t *testing.T) {
	t.Chdir(t.TempDir())
	dir := t.TempDir()
	t.Setenv(SettingsPathEnv, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.json"),
		[]byte(`{ "title": "From the settings path", /* comment */ }`), 0o644))
	s, err := ReadConfig()
	require.NoError(t, err)
	assert.Equal(t, "From the settings path", s.Title)
}
