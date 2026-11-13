package ws

import (
	"testing"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
)

// etherpad-lite: sessionIdCookie, the CLIENT_READY side: the handshake
// cookie wins over the deprecated message field, which is the fallback.
func TestIntegratorSessionIDPreference(t *testing.T) {
	testutils.RunSequentialMemorySQLite(t, testutils.TestRunConfig{
		Name: "integrator sessionID from cookie or message",
		Test: func(t *testing.T, ds testutils.TestDataStore) {
			ready := func(sessionID, cookie, field string) string {
				client := createTestClient(ds.Hub, sessionID, "sidPad", libws.NewActualMockWebSocketconn())
				client.IntegratorSessionID = cookie
				t.Cleanup(func() { removeClient(ds.Hub, client) })
				ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionID)
				ds.PadMessageHandler.HandleMessage(ws.ClientReady{
					Event: "message",
					Data: ws.ClientReadyData{Component: "pad", Type: "CLIENT_READY", PadID: "sidPad",
						Token: "t.sid" + sessionID, SessionID: field},
				}, client, ds.Settings, ds.Logger)
				return ds.PadMessageHandler.SessionStore.IntegratorSessionIDForTest(sessionID)
			}
			assert.Equal(t, "s.dddddddddddddddd", ready("sidA", "s.dddddddddddddddd", "s.eeeeeeeeeeeeeeee"), "the cookie wins")
			assert.Equal(t, "s.cccccccccccccccc", ready("sidB", "", "s.cccccccccccccccc"), "falls back to the message field")
			assert.Equal(t, "", ready("sidC", "", ""), "nothing when neither is given")
		},
	})
}
