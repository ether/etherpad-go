package author

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxNameLength is the longest author display name, in Unicode code points
// (etherpad-lite's MAX_AUTHOR_NAME_LENGTH). Names come from unauthenticated
// socket messages and go to every client that opens a pad, so the server
// bounds them, not the size limit of a message (GHSA-w4w9-j4c3-hjf6). The
// pad's name inputs have the same maxlength.
const MaxNameLength = 128

// SanitizeName trims a display name and cuts it to MaxNameLength code
// points; a blank name is nil. It is applied when a name is stored and
// again when one is read, so names stored before the cap are not sent out
// at their full length.
func SanitizeName(name string) *string {
	// Leading space first, then cut by bytes, so a huge name is never
	// walked rune by rune.
	name = strings.TrimLeftFunc(name, unicode.IsSpace)
	if len(name) > MaxNameLength*utf8.UTFMax*2 {
		name = name[:MaxNameLength*utf8.UTFMax*2]
	}
	s := strings.TrimSpace(name)
	if utf8.RuneCountInString(s) > MaxNameLength {
		runes := []rune(s)
		s = strings.TrimSpace(string(runes[:MaxNameLength]))
	}
	// A cut through a multi-byte character leaves an invalid tail.
	s = strings.ToValidUTF8(s, "")
	if s == "" {
		return nil
	}
	return &s
}

// sanitizeNamePtr is SanitizeName for an optional name.
func sanitizeNamePtr(name *string) *string {
	if name == nil {
		return nil
	}
	return SanitizeName(*name)
}
