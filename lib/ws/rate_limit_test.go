package ws

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// A commit over the rate limit is answered with disconnect "rateLimited",
// as in etherpad-lite, so the client shows it instead of waiting for an
// acknowledgement forever.
func TestRateLimitedCommitTellsTheClient(t *testing.T) {
	for _, msg := range []string{
		`{"event":"message","data":{"type":"COLLABROOM","component":"pad","data":{"type":"USER_CHANGES","baseRev":0,"changeset":"Z:1>1+1$a"}}}`,
		`{"event":"message","data":{"type":"COLLABROOM","component":"sheet","data":{"type":"SHEET_OP","baseRev":0,"op":{}}}}`,
	} {
		c := &Client{Send: make(chan []byte, 4), ClientIP: "203.0.113." + time.Now().Format("150405.000000")}
		s := settings.Settings{CommitRateLimiting: settings.CommitRateLimiting{Duration: 60, Points: 0}}
		c.dispatchMessage([]byte(msg), &s, zap.NewNop().Sugar())
		select {
		case got := <-c.Send:
			assert.JSONEq(t, `["message",{"disconnect":"rateLimited"}]`, string(got))
		default:
			require.Fail(t, "no message to the rate limited client")
		}
	}
}
