package server

import "net/url"

// integratorSessionIDCookie reads the sessionID cookie an integrator set
// through the HTTP API's createSession (etherpad-lite #7045, #7755): the
// prefixed name first, then the plain one. The value is percent-decoded; one
// that cannot be decoded counts as absent, so CLIENT_READY falls back to the
// deprecated message field instead of failing.
func integratorSessionIDCookie(cookie func(key string, defaultValue ...string) string, prefix string) string {
	for _, name := range []string{prefix + "sessionID", "sessionID"} {
		raw := cookie(name)
		if raw == "" {
			continue
		}
		decoded, err := url.PathUnescape(raw)
		if err != nil {
			return ""
		}
		return decoded
	}
	return ""
}
