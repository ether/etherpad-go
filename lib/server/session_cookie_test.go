package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// etherpad-lite: sessionIdCookie (the handshake side; the CLIENT_READY side
// is in lib/test/ws/session_id_cookie_test.go).
func TestIntegratorSessionIDCookie(t *testing.T) {
	jar := func(cookies map[string]string) func(string, ...string) string {
		return func(key string, _ ...string) string { return cookies[key] }
	}
	assert.Equal(t, "s.aaaaaaaaaaaaaaaa", integratorSessionIDCookie(jar(map[string]string{"sessionID": "s.aaaaaaaaaaaaaaaa"}), ""),
		"reads sessionID from the handshake cookie")
	assert.Equal(t, "s.bbbbbbbbbbbbbbbb", integratorSessionIDCookie(jar(map[string]string{"ep_sessionID": "s.bbbbbbbbbbbbbbbb"}), "ep_"),
		"honours the configured cookie prefix")
	assert.Equal(t, "s.prefixed", integratorSessionIDCookie(jar(map[string]string{"ep_sessionID": "s.prefixed", "sessionID": "s.plain"}), "ep_"),
		"the prefixed cookie wins")
	assert.Equal(t, "", integratorSessionIDCookie(jar(nil), "ep_"), "nothing when there is no cookie")
	assert.Equal(t, "", integratorSessionIDCookie(jar(map[string]string{"sessionID": "%ZZ"}), ""),
		"a malformed cookie counts as absent")
	assert.Equal(t, "s.a,b", integratorSessionIDCookie(jar(map[string]string{"sessionID": "s.a%2Cb"}), ""),
		"the value is percent-decoded")
}
