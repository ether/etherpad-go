package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"unicode/utf8"
)

var base64Url = regexp.MustCompile("^[A-Za-z0-9+/]*={0,2}$")

// RandomVersionString is the cache-busting token of the asset URLs
// ("pad.js?v=..."). It is the same for every instance running the same
// build (etherpad-lite #7213: a random value per start made every restart
// and every instance behind a load balancer bust the browsers' caches) and
// can be set with ETHERPAD_VERSION_STRING.
var RandomVersionString = versionString()

func versionString() string {
	if v := os.Getenv("ETHERPAD_VERSION_STRING"); v != "" {
		return v
	}
	seed := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		seed = info.Main.Version
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" || s.Key == "vcs.time" || s.Key == "vcs.modified" {
				seed += "|" + s.Value
			}
		}
	}
	if exe, err := os.Executable(); err == nil {
		if st, err := os.Stat(exe); err == nil {
			seed += "|" + strconv.FormatInt(st.Size(), 10) + "|" + st.ModTime().UTC().String()
		}
	}
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:])[:8]
}

func IsValidAuthorToken(token string) bool {
	if !strings.HasPrefix(token, "t.") {
		return false
	}

	var v = token[2:]
	return utf8.RuneCountInString(v) > 0 && base64Url.MatchString(v)

}
