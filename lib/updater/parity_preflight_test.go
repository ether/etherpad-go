package updater

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's preflight spec for the single-binary apply: its
// git (clean tree, remote tag), pnpm and Node engine checks have no
// counterpart; the release asset and its signature are checked by the
// executor before anything is replaced (executor_test.go).
func TestParityPreflight(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "etherpad-go")
	require.NoError(t, os.WriteFile(exe, make([]byte, 1<<20), 0o755))
	plenty := func(string) (uint64, error) { return 1 << 40, nil }
	base := PreflightDeps{InstallMethod: InstallBinary, ExePath: exe, LockHeld: func() bool { return false }, DiskFree: plenty}
	run := func(mod func(*PreflightDeps)) PreflightResult {
		d := base
		mod(&d)
		return RunPreflight(d)
	}

	assert.Equal(t, PreflightResult{OK: true}, run(func(*PreflightDeps) {}))
	for _, m := range []InstallMethod{InstallDocker, InstallManaged, InstallAuto} {
		assert.Equal(t, PreflightResult{Reason: "install-method-not-writable"}, run(func(d *PreflightDeps) { d.InstallMethod = m }), m)
	}
	assert.Equal(t, PreflightResult{Reason: "lock-held"}, run(func(d *PreflightDeps) { d.LockHeld = func() bool { return true } }))

	need := requiredSpace(1 << 20)
	assert.Equal(t, PreflightResult{Reason: "insufficient-disk-space"},
		run(func(d *PreflightDeps) { d.DiskFree = func(string) (uint64, error) { return need - 1, nil } }))
	assert.True(t, run(func(d *PreflightDeps) { d.DiskFree = func(string) (uint64, error) { return need, nil } }).OK)
	assert.True(t, run(func(d *PreflightDeps) { d.DiskFree = func(string) (uint64, error) { return 0, errors.New("statfs") } }).OK,
		"an unknown free space does not block")

	// Cheap checks first: the disk is not looked at when the method rules
	// the apply out.
	asked := false
	run(func(d *PreflightDeps) {
		d.InstallMethod = InstallDocker
		d.DiskFree = func(string) (uint64, error) { asked = true; return 0, nil }
	})
	assert.False(t, asked)

	if runtime.GOOS != "windows" && os.Getuid() != 0 {
		require.NoError(t, os.Chmod(dir, 0o555))
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		assert.Equal(t, PreflightResult{Reason: "exe-dir-not-writable"}, run(func(*PreflightDeps) {}))
	}
}

func TestDiskFreeOnHost(t *testing.T) {
	free, err := diskFree(t.TempDir())
	require.NoError(t, err)
	assert.Greater(t, free, uint64(0))
}
