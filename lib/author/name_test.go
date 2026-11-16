package author

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// etherpad-lite's authorNameCap spec, sanitizeAuthorName.
func TestSanitizeName(t *testing.T) {
	n := SanitizeName("Alice")
	require.NotNil(t, n)
	assert.Equal(t, "Alice", *n)

	n = SanitizeName("  Bob \t\n")
	require.NotNil(t, n)
	assert.Equal(t, "Bob", *n, "trims whitespace")

	n = SanitizeName(strings.Repeat("x", 1_000_000))
	require.NotNil(t, n)
	assert.Equal(t, MaxNameLength, utf8.RuneCountInString(*n), "caps an oversized name")

	n = SanitizeName(strings.Repeat(" ", 1000) + strings.Repeat("y", 500))
	require.NotNil(t, n)
	assert.Equal(t, strings.Repeat("y", MaxNameLength), *n, "caps after leading whitespace")

	// Multi-byte characters (emoji: 4 bytes, a surrogate pair in JS) are
	// never split.
	n = SanitizeName(strings.Repeat("😀", 300))
	require.NotNil(t, n)
	assert.Equal(t, strings.Repeat("😀", MaxNameLength), *n)
	assert.True(t, utf8.ValidString(*n))

	for _, blank := range []string{"", "   ", "\t\n"} {
		assert.Nil(t, SanitizeName(blank), "%q is no name", blank)
	}
	assert.Nil(t, sanitizeNamePtr(nil))
}
