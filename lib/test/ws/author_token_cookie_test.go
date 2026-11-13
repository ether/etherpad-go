package ws

import (
	"testing"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The HttpOnly author token cookie of the websocket handshake decides the
// author; the token in CLIENT_READY only counts without one.
func TestAuthorTokenCookieDecides(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "cookie token", Test: func(t *testing.T, ds testutils.TestDataStore) {
		cfg := padSettingsConfig(ds)
		join := func(t *testing.T, session, cookieToken, msgToken string) string {
			client := createTestClient(ds.Hub, session, "tokenCookiePad", libws.NewActualMockWebSocketconn())
			client.CookieToken = cookieToken
			t.Cleanup(func() { removeClient(ds.Hub, client) })
			ds.PadMessageHandler.SessionStore.InitSessionForTest(session)
			ds.PadMessageHandler.HandleMessage(ws.ClientReady{Event: "message", Data: ws.ClientReadyData{
				Component: "pad", Type: "CLIENT_READY", PadID: "tokenCookiePad", Token: msgToken}}, client, cfg, ds.Logger)
			for _, f := range frames(client) {
				if f.Type == "CLIENT_VARS" {
					id, _ := f.Data["userId"].(string)
					return id
				}
			}
			t.Fatal("no CLIENT_VARS")
			return ""
		}
		viaCookie := join(t, "tc1", "t.cookieToken1234567", "")
		require.NotEmpty(t, viaCookie)
		assert.Equal(t, viaCookie, join(t, "tc2", "t.cookieToken1234567", "t.somethingElse12345"), "the cookie wins")
		legacy := join(t, "tc3", "", "t.legacyMessageToken1")
		assert.Equal(t, legacy, join(t, "tc4", "", "t.legacyMessageToken1"), "older clients keep their identity")
		assert.NotEqual(t, viaCookie, legacy)
	}})
	testDb.StartTestDBHandler()
}
