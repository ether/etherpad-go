package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Port of etherpad-lite's anonymizeIp spec.
func TestAnonymizeIP(t *testing.T) {
	cases := []struct{ ip, mode, want string }{
		{"1.2.3.4", "anonymous", "ANONYMOUS"},
		{"2001:db8::1", "anonymous", "ANONYMOUS"},
		{"1.2.3.4", "full", "1.2.3.4"},
		{"2001:db8::1", "full", "2001:db8::1"},
		{"1.2.3.4", "truncated", "1.2.3.0"},
		{"2001:db8::1", "truncated", "2001:db8::"},
		{"2001:db8:1:2:3:4:5:6", "truncated", "2001:db8:1::"},
		{"::ffff:1.2.3.4", "truncated", "::ffff:1.2.3.0"},
		{"not-an-ip", "truncated", "ANONYMOUS"},
		{"8.8.8.8", "lolnope", "ANONYMOUS"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, AnonymizeIP(c.ip, c.mode), "%s %s", c.ip, c.mode)
	}
	for _, mode := range []string{"full", "truncated", "anonymous"} {
		assert.Equal(t, "ANONYMOUS", AnonymizeIP("", mode), mode)
	}
}
