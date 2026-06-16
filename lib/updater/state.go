package updater

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LoadState reads and validates the persisted update state. Any problem
// (missing file, parse error, schema mismatch) yields a fresh EmptyState so a
// corrupt file can never wedge the updater.
func LoadState(path string) UpdateState {
	b, err := os.ReadFile(path)
	if err != nil {
		return EmptyState()
	}
	var s UpdateState
	if err := json.Unmarshal(b, &s); err != nil {
		return EmptyState()
	}
	if s.SchemaVersion != SchemaVersion {
		return EmptyState()
	}
	if s.Execution.Status == "" {
		s.Execution.Status = StatusIdle
	}
	return s
}

// SaveState writes the state atomically (write to a temp file, then rename) so
// a crash mid-write can never leave a truncated state file.
func SaveState(path string, s UpdateState) error {
	s.SchemaVersion = SchemaVersion
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
