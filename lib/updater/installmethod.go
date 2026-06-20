package updater

import (
	"os"
	"path/filepath"
)

// DetectInstallMethod determines whether an in-place update is possible. An
// explicit, non-auto override is returned verbatim.
//
// Detection order: a Docker environment is never self-updated (the orchestrator
// owns the image), then a standalone executable whose directory is writable is
// updatable as a binary, otherwise the install is treated as read-only/managed.
func DetectInstallMethod(override InstallMethod) InstallMethod {
	if override != "" && override != InstallAuto {
		return override
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return InstallDocker
	}
	exe, err := os.Executable()
	if err != nil {
		return InstallManaged
	}
	// The running executable cannot be opened for writing on Windows, but it can
	// be renamed/replaced as long as its directory is writable — which is
	// exactly what the executor relies on. So writability of the directory is
	// the correct test, not write access to the file itself.
	if isWritableDir(filepath.Dir(exe)) {
		return InstallBinary
	}
	return InstallManaged
}

func isWritableDir(dir string) bool {
	f, err := os.CreateTemp(dir, ".ep-update-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}
