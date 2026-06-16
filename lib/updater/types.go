// Package updater implements Etherpad-Go's self-update subsystem: a tiered
// (off / notify / manual / auto / autonomous) updater that checks GitHub
// releases, and — for writable single-binary installs — downloads the matching
// release binary, verifies it, atomically replaces the running executable and
// exits with code 75 so a process supervisor restarts into the new version.
//
// It is a Go-appropriate port of Etherpad-lite's src/node/updater subsystem.
// Because Etherpad-Go ships as a single binary (rather than a git checkout
// updated with pnpm), the apply mechanism is binary self-replacement instead of
// git/pnpm, but the surrounding state machine, policy gates, scheduler,
// maintenance windows, session draining, rollback and crash-loop guard mirror
// the upstream design.
package updater

// ExitCodeRestart is returned to the OS after a successful apply or a rollback
// so that the supervisor (systemd, Docker, Windows service, ...) restarts the
// process into the swapped-in binary.
const ExitCodeRestart = 75

// SchemaVersion is the persisted update-state schema version.
const SchemaVersion = 1

// Tier controls how autonomous the updater is.
type Tier string

const (
	TierOff        Tier = "off"        // do nothing
	TierNotify     Tier = "notify"     // check + surface availability only
	TierManual     Tier = "manual"     // admin can trigger an apply
	TierAuto       Tier = "auto"       // scheduler auto-applies after a grace window
	TierAutonomous Tier = "autonomous" // scheduler auto-applies within a maintenance window
)

// InstallMethod describes how Etherpad-Go was installed, which determines
// whether an in-place update is possible.
type InstallMethod string

const (
	InstallAuto    InstallMethod = "auto"    // detect at runtime
	InstallBinary  InstallMethod = "binary"  // writable standalone executable
	InstallDocker  InstallMethod = "docker"  // container; rollout handled by orchestrator
	InstallManaged InstallMethod = "managed" // read-only / package-managed
)

// ExecutionStatus is the persisted state-machine position of an apply attempt.
type ExecutionStatus string

const (
	StatusIdle                ExecutionStatus = "idle"
	StatusScheduled           ExecutionStatus = "scheduled"
	StatusPreflight           ExecutionStatus = "preflight"
	StatusPreflightFailed     ExecutionStatus = "preflight-failed"
	StatusDraining            ExecutionStatus = "draining"
	StatusExecuting           ExecutionStatus = "executing"
	StatusPendingVerification ExecutionStatus = "pending-verification"
	StatusVerified            ExecutionStatus = "verified"
	StatusRollingBack         ExecutionStatus = "rolling-back"
	StatusRolledBack          ExecutionStatus = "rolled-back"
	StatusRollbackFailed      ExecutionStatus = "rollback-failed" // terminal: blocks auto until acknowledged
)

// ReleaseInfo is the subset of a GitHub release the updater tracks.
type ReleaseInfo struct {
	Version     string `json:"version"` // tag without a leading "v"
	Tag         string `json:"tag"`
	Body        string `json:"body"`
	PublishedAt string `json:"publishedAt"`
	Prerelease  bool   `json:"prerelease"`
	HTMLURL     string `json:"htmlUrl"`
}

// Execution captures the current apply attempt. Fields are only meaningful for
// certain Status values.
type Execution struct {
	Status       ExecutionStatus `json:"status"`
	TargetTag    string          `json:"targetTag,omitempty"`
	FromVersion  string          `json:"fromVersion,omitempty"`
	BackupPath   string          `json:"backupPath,omitempty"`   // path to the backed-up previous binary
	ScheduledFor string          `json:"scheduledFor,omitempty"` // RFC3339
	DrainEndsAt  string          `json:"drainEndsAt,omitempty"`  // RFC3339
	DeadlineAt   string          `json:"deadlineAt,omitempty"`   // RFC3339, health-check deadline
	Reason       string          `json:"reason,omitempty"`
	At           string          `json:"at,omitempty"` // RFC3339, time the status was entered
}

// LastResult records the outcome of the most recent completed apply attempt for
// display in the admin UI.
type LastResult struct {
	TargetTag   string `json:"targetTag"`
	FromVersion string `json:"fromVersion"`
	Outcome     string `json:"outcome"`
	Reason      string `json:"reason,omitempty"`
	At          string `json:"at"`
}

// UpdateState is the full persisted state, written atomically to a JSON file.
type UpdateState struct {
	SchemaVersion int          `json:"schemaVersion"`
	LastCheckAt   string       `json:"lastCheckAt,omitempty"`
	LastETag      string       `json:"lastEtag,omitempty"`
	Latest        *ReleaseInfo `json:"latest,omitempty"`
	Execution     Execution    `json:"execution"`
	BootCount     int          `json:"bootCount"`
	LastResult    *LastResult  `json:"lastResult,omitempty"`
}

// EmptyState returns the initial state for a fresh install or unreadable file.
func EmptyState() UpdateState {
	return UpdateState{
		SchemaVersion: SchemaVersion,
		Execution:     Execution{Status: StatusIdle},
	}
}
