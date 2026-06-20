package updater

import "github.com/Masterminds/semver/v3"

// updateAvailable reports whether latest is strictly newer than current. A
// current version that is not valid semver (e.g. a dev commit hash) is treated
// as up-to-date so development builds never try to "update".
func updateAvailable(current, latest string) bool {
	if latest == "" {
		return false
	}
	cur, errCur := semver.NewVersion(current)
	lat, errLat := semver.NewVersion(latest)
	if errCur != nil || errLat != nil {
		return false
	}
	return lat.GreaterThan(cur)
}

// isMinorOrMoreBehind reports whether latest is at least a full minor version
// ahead of current (used to decide whether a "severe" notification is due).
func isMinorOrMoreBehind(current, latest string) bool {
	cur, errCur := semver.NewVersion(current)
	lat, errLat := semver.NewVersion(latest)
	if errCur != nil || errLat != nil {
		return false
	}
	if lat.Major() > cur.Major() {
		return true
	}
	return lat.Major() == cur.Major() && lat.Minor() > cur.Minor()
}
