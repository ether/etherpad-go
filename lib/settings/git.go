package settings

import (
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

func GetGitCommit(setting *Settings) string {
	if setting.DevMode {
		gitPath := ".git"
		info, err := os.Stat(gitPath)
		if err != nil {
			log.Printf("Can't access .git: %v", err)
			return ""
		}

		if info.Mode().IsRegular() {
			data, err := os.ReadFile(gitPath)
			if err != nil {
				log.Printf("Can't read .git file: %v", err)
				return ""
			}
			parts := strings.SplitN(string(data), ":", 2)
			if len(parts) != 2 {
				log.Printf("unexpected .git file format")
				return ""
			}
			gitPath = strings.TrimSpace(parts[1])
			if !filepath.IsAbs(gitPath) {
				gitPath = filepath.Join(".", gitPath)
			}
		}

		headPath := filepath.Join(gitPath, "HEAD")
		headData, err := os.ReadFile(headPath)
		if err != nil {
			log.Printf("Can't read HEAD: %v", err)
			return ""
		}
		head := strings.TrimSpace(string(headData))

		var commit string
		if strings.HasPrefix(head, "ref: ") {
			ref := strings.TrimSpace(head[len("ref: "):])
			refPath := filepath.Join(gitPath, ref)
			refData, err := os.ReadFile(refPath)
			if err != nil {
				log.Printf("Can't read ref %s: %v", refPath, err)
				return ""
			}
			commit = strings.TrimSpace(string(refData))
		} else {
			commit = head
		}

		if len(commit) > 7 {
			commit = commit[:7]
		}
		return commit
	}

	return GitVersion()

}

func BuildInfo() (version, releaseID string) {
	bi, ok := debug.ReadBuildInfo()
	if !ok || bi == nil {
		return "", ""
	}

	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		version = bi.Main.Version
	}

	var modified bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			releaseID = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}

	if releaseID != "" && modified {
		releaseID += "-dirty"
	}

	return version, releaseID
}

func GitVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok || bi == nil {
		return ""
	}

	// Prefer proper module version (e.g. v1.2.3)
	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}

	var (
		rev      string
		modified bool
	)

	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}

	if rev == "" {
		return ""
	}

	if modified {
		return rev + "-dirty"
	}

	return rev
}
