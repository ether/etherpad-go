package updater

import (
	"encoding/json"
	"os"
	"time"
)

type lockInfo struct {
	PID int    `json:"pid"`
	At  string `json:"at"`
}

// lockStaleAfter bounds how long a lock from a (possibly crashed) process is
// honored before it is considered stale and reaped.
const lockStaleAfter = 2 * time.Hour

// acquireLock atomically creates a lock file. It returns false (without error)
// if a live, non-stale lock is held by another process. A lock that is stale
// (older than lockStaleAfter) or owned by a dead PID is reaped and re-acquired.
func acquireLock(path string) (bool, error) {
	if held, err := tryCreate(path); err != nil {
		return false, err
	} else if held {
		return true, nil
	}
	if lockHeld(path) {
		return false, nil
	}
	// Stale: reap and retry once.
	_ = os.Remove(path)
	return tryCreate(path)
}

func tryCreate(path string) (bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, err
	}
	info := lockInfo{PID: os.Getpid(), At: time.Now().UTC().Format(time.RFC3339)}
	b, _ := json.Marshal(info)
	_, werr := f.Write(b)
	cerr := f.Close()
	if werr != nil {
		return false, werr
	}
	return true, cerr
}

func releaseLock(path string) { _ = os.Remove(path) }

func lockHeld(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var info lockInfo
	if err := json.Unmarshal(b, &info); err != nil {
		return false // unreadable -> treat as stale
	}
	if info.PID == os.Getpid() {
		return true
	}
	if at, err := time.Parse(time.RFC3339, info.At); err == nil && time.Since(at) > lockStaleAfter {
		return false
	}
	return processAlive(info.PID)
}

// processAlive reports whether a process with the given pid is currently
// running. It is implemented per-platform (lock_unix.go / lock_windows.go) so a
// crashed instance's lock is reaped promptly instead of blocking until the
// staleness TTL elapses.
