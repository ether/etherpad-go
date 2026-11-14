package updater

import (
	"os"
	"path/filepath"
)

// PreflightDeps are the inputs to RunPreflight. All checks run before any file
// is touched, so a failed preflight is fully reversible.
type PreflightDeps struct {
	InstallMethod InstallMethod
	ExePath       string
	LockHeld      func() bool
	// DiskFree reports the free space in a directory; nil uses the host's.
	// An error skips the check (the download then fails on its own).
	DiskFree func(dir string) (uint64, error)
}

// PreflightResult reports whether an apply may proceed.
type PreflightResult struct {
	OK     bool
	Reason string
}

// diskMargin is kept free on top of the download: a full disk would also
// stop the database and logs of the running instance.
const diskMargin = 50 << 20

// requiredSpace is what an apply writes next to the executable: the new
// binary (about as big as the current one) and its temp file, plus the
// margin. The old binary is renamed to the backup, not copied.
func requiredSpace(exeSize int64) uint64 {
	return 2*uint64(exeSize) + diskMargin
}

// RunPreflight validates the environment before an apply. Cheap, reversible
// checks first (etherpad-lite's preflight: install method, lock, disk space;
// its git, pnpm and Node checks have no counterpart for a single binary).
func RunPreflight(d PreflightDeps) PreflightResult {
	if d.InstallMethod != InstallBinary {
		return PreflightResult{Reason: "install-method-not-writable"}
	}
	if d.LockHeld != nil && d.LockHeld() {
		return PreflightResult{Reason: "lock-held"}
	}
	dir := filepath.Dir(d.ExePath)
	if !isWritableDir(dir) {
		return PreflightResult{Reason: "exe-dir-not-writable"}
	}
	free := d.DiskFree
	if free == nil {
		free = diskFree
	}
	if info, err := os.Stat(d.ExePath); err == nil {
		if avail, err := free(dir); err == nil && avail < requiredSpace(info.Size()) {
			return PreflightResult{Reason: "insufficient-disk-space"}
		}
	}
	return PreflightResult{OK: true}
}
