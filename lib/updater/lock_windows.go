//go:build windows

package updater

import "golang.org/x/sys/windows"

// stillActive is the exit code Windows reports for a process that is still
// running (STILL_ACTIVE).
const stillActive = 259

// processAlive checks liveness via OpenProcess + GetExitCodeProcess so a
// crashed instance's lock is reaped promptly rather than waiting out the TTL.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// Access denied implies the process exists (owned by someone else);
		// any other error means it could not be found.
		return err == windows.ERROR_ACCESS_DENIED
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}
