package settings

import (
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
)

func GetGitCommit() string {
	if os.Getenv("DEVELOPMENT_MODE") == "true" {
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
	} else {
		return GitVersion()
	}

}

func GitVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok || bi == nil {
		return ""
	}

	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}

	var rev, modified string
	for _, s := range bi.Settings {
		if s.Key == "vcs.revision" {
			rev = s.Value
		}
		if s.Key == "vcs.modified" {
			modified = s.Value
		}
	}
	if rev != "" {
		if modified == "true" {
			return rev + "-dirty"
		}
		return rev
	}

	return ""
}
