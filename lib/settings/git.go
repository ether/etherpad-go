package settings

import "runtime/debug"

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
