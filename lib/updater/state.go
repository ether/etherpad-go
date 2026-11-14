package updater

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// LoadState reads and validates the persisted update state. Any problem
// (missing file, parse error, schema mismatch, an execution the updater
// could not act on) yields a fresh EmptyState so a corrupt or hand-edited
// file can never wedge the updater, or roll back without a backup.
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
	if !validState(s) {
		return EmptyState()
	}
	return s
}

// requiredFields are the Execution fields each status needs: the updater
// acts on them after a restart (a rollback needs BackupPath, a scheduled
// apply its time), etherpad-lite's per-status validation.
var requiredFields = map[ExecutionStatus][]string{
	StatusIdle:                nil,
	StatusScheduled:           {"targetTag", "scheduledFor"},
	StatusPreflight:           {"targetTag"},
	StatusPreflightFailed:     {"targetTag"},
	StatusDraining:            {"targetTag", "drainEndsAt"},
	StatusExecuting:           {"targetTag", "fromVersion"},
	StatusPendingVerification: {"targetTag", "fromVersion", "backupPath", "deadlineAt"},
	StatusVerified:            {"targetTag"},
	StatusRollingBack:         {"targetTag", "backupPath", "reason"},
	StatusRolledBack:          {"targetTag"},
	StatusRollbackFailed:      {"targetTag"},
}

var knownOutcomes = map[string]bool{
	"verified": true, "failed": true, "preflight-failed": true, "rolled-back": true, "rollback-failed": true,
}

func validTime(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

func validState(s UpdateState) bool {
	e := s.Execution
	required, known := requiredFields[e.Status]
	if !known {
		return false
	}
	fields := map[string]string{
		"targetTag": e.TargetTag, "fromVersion": e.FromVersion, "backupPath": e.BackupPath,
		"scheduledFor": e.ScheduledFor, "drainEndsAt": e.DrainEndsAt, "deadlineAt": e.DeadlineAt, "reason": e.Reason,
	}
	for _, f := range required {
		if fields[f] == "" {
			return false
		}
	}
	if e.TargetTag != "" && !IsValidTag(e.TargetTag) {
		return false
	}
	for _, ts := range []string{e.ScheduledFor, e.DrainEndsAt, e.DeadlineAt, e.At, s.LastCheckAt} {
		if !validTime(ts) {
			return false
		}
	}
	if s.BootCount < 0 {
		return false
	}
	if l := s.Latest; l != nil && (l.Version == "" || !IsValidTag(l.Tag)) {
		return false
	}
	if r := s.LastResult; r != nil && (!knownOutcomes[r.Outcome] || r.TargetTag == "" || !validTime(r.At)) {
		return false
	}
	return true
}

// SaveState writes the state atomically (write to a temp file, then rename) so
// a crash mid-write can never leave a truncated state file. The temp file
// has its own name, so two saves at once cannot write into each other's.
func SaveState(path string, s UpdateState) error {
	s.SchemaVersion = SchemaVersion
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(b)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(tmpName, 0o600)
	}
	if werr == nil {
		werr = os.Rename(tmpName, path)
	}
	if werr != nil {
		_ = os.Remove(tmpName)
	}
	return werr
}
