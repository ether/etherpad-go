package updater

import "path/filepath"

// PreflightDeps are the inputs to RunPreflight. All checks run before any file
// is touched, so a failed preflight is fully reversible.
type PreflightDeps struct {
	InstallMethod InstallMethod
	ExePath       string
	LockHeld      func() bool
}

// PreflightResult reports whether an apply may proceed.
type PreflightResult struct {
	OK     bool
	Reason string
}

// RunPreflight validates the environment before an apply. Cheap, reversible
// checks first.
func RunPreflight(d PreflightDeps) PreflightResult {
	if d.InstallMethod != InstallBinary {
		return PreflightResult{Reason: "install-method-not-writable"}
	}
	if !isWritableDir(filepath.Dir(d.ExePath)) {
		return PreflightResult{Reason: "exe-dir-not-writable"}
	}
	if d.LockHeld != nil && d.LockHeld() {
		return PreflightResult{Reason: "lock-held"}
	}
	return PreflightResult{OK: true}
}
