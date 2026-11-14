package updater

import (
	"strings"

	"github.com/Masterminds/semver/v3"
)

// parseVersion parses MAJOR.MINOR.PATCH with an optional leading "v",
// pre-release and build metadata. Unlike semver.NewVersion it does not
// complete "2.7" to 2.7.0 or accept "2.7.1.4": a tag that is not a full
// version is not a release to update to (etherpad-lite's parseSemver).
func parseVersion(v string) (*semver.Version, bool) {
	parsed, err := semver.StrictNewVersion(strings.TrimPrefix(strings.TrimSpace(v), "v"))
	if err != nil {
		return nil, false
	}
	return parsed, true
}

// updateAvailable reports whether latest is strictly newer than current. A
// current version that is not valid semver (e.g. a dev commit hash) is treated
// as up-to-date so development builds never try to "update". A pre-release
// sorts before its release (2.7.3-rc.1 < 2.7.3), so an rc install is offered
// the final release; etherpad-lite ignores the pre-release part.
func updateAvailable(current, latest string) bool {
	if latest == "" {
		return false
	}
	cur, okCur := parseVersion(current)
	lat, okLat := parseVersion(latest)
	if !okCur || !okLat {
		return false
	}
	return lat.GreaterThan(cur)
}

// isMinorOrMoreBehind reports whether latest is at least a full minor version
// ahead of current (used to decide whether a "severe" notification is due).
func isMinorOrMoreBehind(current, latest string) bool {
	cur, okCur := parseVersion(current)
	lat, okLat := parseVersion(latest)
	if !okCur || !okLat {
		return false
	}
	if lat.Major() > cur.Major() {
		return true
	}
	return lat.Major() == cur.Major() && lat.Minor() > cur.Minor()
}
