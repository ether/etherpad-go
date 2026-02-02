package utils

import (
	"github.com/Masterminds/semver/v3"
)

// IsUpdateAvailable returns true if the latest version is strictly newer than the current version.
// It handles semantic versions and treats non-semver current versions (like commit hashes)
// as development builds that are considered up-to-date.
func IsUpdateAvailable(current, latest string) bool {
	if latest == "" {
		return false
	}

	currVer, errCurr := semver.NewVersion(current)
	lateVer, errLate := semver.NewVersion(latest)

	// If both are valid semver, compare them
	if errCurr == nil && errLate == nil {
		return lateVer.GreaterThan(currVer)
	}

	// If current version is not a valid semver, it's likely a development build (commit hash).
	// We treat these as up-to-date relative to any tag.
	if errCurr != nil {
		return false
	}

	// If current is semver but latest is not (unexpected), we say no update.
	if errLate != nil {
		return false
	}

	// Default to no update
	return false
}
