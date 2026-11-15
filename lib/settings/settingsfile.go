package settings

import (
	"os"
	"path/filepath"
)

// SettingsPathEnv names the directory of settings.json (the Docker image
// sets /data, which its user can write).
const SettingsPathEnv = "ETHERPAD_SETTINGS_PATH"

// SettingsFile is the settings.json read for dir (ETHERPAD_SETTINGS_PATH,
// or the server's root in the admin UI): the one in dir when dir holds
// one, otherwise settings.json in the working directory (a file mounted
// there keeps working).
func SettingsFile(dir string) string {
	if dir != "" {
		p := filepath.Join(dir, "settings.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "settings.json"
}

// SettingsWriteFile is where the admin UI saves settings.json for dir: in
// dir (SettingsFile then reads it from there), or the working directory.
func SettingsWriteFile(dir string) string {
	if dir != "" {
		return filepath.Join(dir, "settings.json")
	}
	return "settings.json"
}
