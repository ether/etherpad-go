package accounts

import (
	"errors"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

// MaxPatternLength is the longest pad pattern.
const MaxPatternLength = 255

// ErrInvalidPattern is returned for a pad pattern that cannot be used.
var ErrInvalidPattern = errors.New("invalid pad pattern")

// A pad pattern is a pad name in which * stands for any number of
// characters, "/" included. "" (no pattern) and "*" match every pad. The
// pattern becomes an anchored regular expression with everything but *
// quoted: people never write regular expressions, so no pattern can make
// matching slow.
type pattern struct {
	all bool
	re  *regexp.Regexp
}

var patternCache sync.Map // string → *pattern

// ValidatePattern checks a pad pattern.
func ValidatePattern(p string) error {
	if utf8.RuneCountInString(p) > MaxPatternLength || !utf8.ValidString(p) {
		return ErrInvalidPattern
	}
	if strings.ContainsAny(p, "\x00\n\r") {
		return ErrInvalidPattern
	}
	return nil
}

func compilePattern(p string) *pattern {
	if cached, ok := patternCache.Load(p); ok {
		return cached.(*pattern)
	}
	var c *pattern
	if p == "" || strings.Trim(p, "*") == "" {
		c = &pattern{all: true}
	} else {
		parts := strings.Split(p, "*")
		for i, part := range parts {
			parts[i] = regexp.QuoteMeta(part)
		}
		c = &pattern{re: regexp.MustCompile(`^` + strings.Join(parts, `.*`) + `$`)}
	}
	patternCache.Store(p, c)
	return c
}

// MatchPattern reports whether pad pattern p matches padId.
func MatchPattern(p, padId string) bool {
	c := compilePattern(p)
	return c.all || c.re.MatchString(padId)
}
