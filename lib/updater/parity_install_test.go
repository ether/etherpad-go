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

// Port of etherpad-lite's InstallMethodDetector spec, for a single binary
// instead of a git checkout or npm install.
func TestParityInstallMethod(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "etherpad-go")
	require.NoError(t, os.WriteFile(exe, []byte("bin"), 0o755))
	env := map[string]string{}
	probe := installProbe{
		markers: []string{filepath.Join(dir, ".dockerenv"), filepath.Join(dir, ".containerenv")},
		getenv:  func(k string) string { return env[k] },
		exe:     func() (string, error) { return exe, nil },
	}

	for _, m := range []InstallMethod{InstallBinary, InstallDocker, InstallManaged} {
		assert.Equal(t, m, detectInstallMethod(m, probe), "an override wins")
	}
	assert.Equal(t, InstallBinary, detectInstallMethod(InstallAuto, probe), "a writable directory")
	assert.Equal(t, InstallBinary, detectInstallMethod("", probe))

	env["KUBERNETES_SERVICE_HOST"] = "10.0.0.1"
	assert.Equal(t, InstallDocker, detectInstallMethod(InstallAuto, probe), "a Kubernetes pod (containerd writes no marker)")
	delete(env, "KUBERNETES_SERVICE_HOST")

	for _, marker := range probe.markers {
		require.NoError(t, os.WriteFile(marker, nil, 0o644))
		assert.Equal(t, InstallDocker, detectInstallMethod(InstallAuto, probe), "%s, before the writable directory", marker)
		require.NoError(t, os.Remove(marker))
	}

	failing := probe
	failing.exe = func() (string, error) { return "", errors.New("no executable") }
	assert.Equal(t, InstallManaged, detectInstallMethod(InstallAuto, failing))

	if runtime.GOOS != "windows" && os.Getuid() != 0 {
		require.NoError(t, os.Chmod(dir, 0o555))
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		assert.Equal(t, InstallManaged, detectInstallMethod(InstallAuto, probe), "a read-only directory")
	}
}
