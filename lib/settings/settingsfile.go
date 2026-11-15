package settings

import (
	"errors"
	"fmt"
	"os"
)

const (
	// SettingsPathEnv is the server's root: assets, APIKEY.txt and relative
	// favicon paths are looked up there.
	SettingsPathEnv = "ETHERPAD_SETTINGS_PATH"
	// SettingsFileEnv is the settings.json to read and to save from the
	// admin UI (the Docker image: /data/settings.json, which its
	// unprivileged user can write). Unset: settings.json in the working
	// directory.
	SettingsFileEnv = "ETHERPAD_SETTINGS_FILE"
)

const workingDirSettings = "settings.json"

// settingsFiles are the settings.json the server reads ("" when there is
// none) and the one the admin UI saves to. Reading and saving go by the
// same rule:
//
//   - ETHERPAD_SETTINGS_FILE set: that file; while it does not exist yet,
//     settings.json in the working directory is read (a file mounted at
//     /settings.json keeps working) and the first save creates it;
//   - otherwise settings.json in the working directory, for both.
//
// Only a missing file counts as missing: any other error (no permission,
// say) is returned, so the server does not start on defaults, without the
// authentication its settings ask for.
func settingsFiles() (read, save string, err error) {
	save = workingDirSettings
	if f := os.Getenv(SettingsFileEnv); f != "" {
		save = f
	}
	for _, candidate := range []string{save, workingDirSettings} {
		_, statErr := os.Stat(candidate)
		if statErr == nil {
			return candidate, save, nil
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return "", save, fmt.Errorf("settings file %s: %w", candidate, statErr)
		}
		if candidate == workingDirSettings {
			break
		}
	}
	return "", save, nil
}
