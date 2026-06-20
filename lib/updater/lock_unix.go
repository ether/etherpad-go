//go:build !windows

package updater

import (
	"errors"
	"os"
	"syscall"
)

// processAlive uses signal 0 (no signal sent) to probe for the process.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	// nil  -> the process exists and we may signal it.
	// EPERM -> it exists but is owned by another user (still alive).
	// ESRCH (and anything else) -> not running.
	return err == nil || errors.Is(err, syscall.EPERM)
}
