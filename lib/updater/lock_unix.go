//go:build !windows

package updater

import (
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
	return proc.Signal(syscall.Signal(0)) == nil
}
