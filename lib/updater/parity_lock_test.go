package updater

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's updater lock spec.
func TestParityLock(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "update.lock")
	write := func(content string) { require.NoError(t, os.WriteFile(lockPath, []byte(content), 0o600)) }
	acquire := func() bool {
		ok, err := acquireLock(lockPath)
		require.NoError(t, err)
		return ok
	}

	t.Run("acquires and releases", func(t *testing.T) {
		assert.True(t, acquire())
		assert.True(t, lockHeld(lockPath))
		releaseLock(lockPath)
		assert.False(t, lockHeld(lockPath))
	})
	t.Run("a second acquire while held", func(t *testing.T) {
		assert.True(t, acquire())
		assert.False(t, acquire())
		releaseLock(lockPath)
	})
	t.Run("stale: the pid is gone", func(t *testing.T) {
		write(`{"pid":2147483646,"at":"` + time.Now().UTC().Format(time.RFC3339) + `"}`)
		assert.False(t, lockHeld(lockPath))
		assert.True(t, acquire())
		releaseLock(lockPath)
	})
	t.Run("stale: older than the TTL", func(t *testing.T) {
		write(`{"pid":1,"at":"` + time.Now().Add(-3*time.Hour).UTC().Format(time.RFC3339) + `"}`)
		assert.True(t, acquire())
		releaseLock(lockPath)
	})
	t.Run("stale: unparseable or without fields", func(t *testing.T) {
		for _, c := range []string{"garbage", `{"somethingElse":true}`, ""} {
			write(c)
			assert.True(t, acquire(), "%q", c)
			releaseLock(lockPath)
		}
	})
	t.Run("release is idempotent", func(t *testing.T) {
		releaseLock(lockPath)
		releaseLock(lockPath)
		assert.False(t, lockHeld(lockPath))
	})
	t.Run("creates the parent directory", func(t *testing.T) {
		nested := filepath.Join(dir, "a", "b", "update.lock")
		ok, err := acquireLock(nested)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.True(t, lockHeld(nested))
		releaseLock(nested)
	})
}
