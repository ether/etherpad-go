package server

import "github.com/ether/etherpad-go/lib/pad"

// integratorSessionIDCookie reads the sessionID cookie an integrator set
// through the HTTP API's createSession; see pad.IntegratorSessionIDCookie.
func integratorSessionIDCookie(cookie func(key string, defaultValue ...string) string, prefix string) string {
	return pad.IntegratorSessionIDCookie(cookie, prefix)
}
