package utils

import (
	"regexp"
	"strings"
)

var base64Url = regexp.MustCompile("^[A-Za-z0-9+/]*={0,2}$")

func IsValidAuthorToken(token string) bool {
	if !strings.HasPrefix(token, "t.") {
		return false
	}

	var v = token[2:]
	return len(v) > 0 && base64Url.MatchString(v)

}
