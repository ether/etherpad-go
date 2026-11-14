package updater

import (
	"os"
	"path/filepath"
)

// installProbe is what detectInstallMethod looks at, so tests can point it
// at a temp directory.
type installProbe struct {
	markers []string            // files that only exist inside a container
	getenv  func(string) string // environment
	exe     func() (string, error)
}

var hostProbe = installProbe{
	// Docker writes /.dockerenv, Podman and CRI-O /run/.containerenv;
	// containerd (most Kubernetes clusters) writes neither, but Kubernetes
	// sets KUBERNETES_SERVICE_HOST in every pod.
	markers: []string{"/.dockerenv", "/run/.containerenv"},
	getenv:  os.Getenv,
	exe:     os.Executable,
}

// DetectInstallMethod determines whether an in-place update is possible. An
// explicit, non-auto override is returned verbatim.
//
// Detection order: a container is never self-updated (the orchestrator owns
// the image, and a replaced binary would be gone after the next restart),
// then a standalone executable whose directory is writable is updatable as a
// binary, otherwise the install is treated as read-only/managed.
func DetectInstallMethod(override InstallMethod) InstallMethod {
	return detectInstallMethod(override, hostProbe)
}

func detectInstallMethod(override InstallMethod, p installProbe) InstallMethod {
	if override != "" && override != InstallAuto {
		return override
	}
	for _, m := range p.markers {
		if _, err := os.Stat(m); err == nil {
			return InstallDocker
		}
	}
	if p.getenv("KUBERNETES_SERVICE_HOST") != "" {
		return InstallDocker
	}
	exe, err := p.exe()
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
