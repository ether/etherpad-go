package utils

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// etherpad-lite settings "randomVersionString determinism (#7213)".
func TestVersionString(t *testing.T) {
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{8}$`), RandomVersionString)
	assert.Equal(t, RandomVersionString, versionString(), "stable for one build")
	t.Setenv("ETHERPAD_VERSION_STRING", "integrator-1")
	assert.Equal(t, "integrator-1", versionString())
}
