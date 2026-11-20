package ws

import (
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An open pad does not keep writing after its login session logged out or
// its account was disabled.
func TestLoginSessionsDisconnect(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "logout closes the session's connections", Test: testLogoutDisconnects},
		testutils.TestRunConfig{Name: "inactive accounts are disconnected", Test: testInactiveAccountsDisconnect},
	)
	testDb.StartTestDBHandler()
}

func loginClient(t *testing.T, ds testutils.TestDataStore, sessionID, accountID, loginSession string) *libws.Client {
	conn := libws.NewActualMockWebSocketconn()
	client := createTestClient(ds.Hub, sessionID, "loginPad", conn)
	t.Cleanup(func() {
		if ds.Hub.HasClient(client) {
			removeClient(ds.Hub, client)
		}
	})
	if accountID != "" {
		name := accountID
		client.WebAccessUser = &webaccess.SocketClientRequest{Username: &name, AccountID: accountID}
	}
	client.LoginSession = loginSession
	return client
}

// queued reads what a client was sent, up to the closed channel.
func queued(c *libws.Client) []string {
	var out []string
	for {
		select {
		case m, ok := <-c.Send:
			if !ok {
				return append(out, "<closed>")
			}
			out = append(out, string(m))
		default:
			return out
		}
	}
}

// revoked waits until the hub let the client go.
func revoked(t *testing.T, ds testutils.TestDataStore, c *libws.Client) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !ds.Hub.HasClient(c) {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func testLogoutDisconnects(t *testing.T, ds testutils.TestDataStore) {
	key := libws.LoginSessionKey("session-of-anna")
	assert.NotEqual(t, "session-of-anna", key, "the id itself is not passed around")
	assert.Empty(t, libws.LoginSessionKey(""))
	laptop := loginClient(t, ds, "ws-l1", "acc.anna", key)
	phone := loginClient(t, ds, "ws-l2", "acc.anna", libws.LoginSessionKey("other-session"))
	visitor := loginClient(t, ds, "ws-l3", "", "")
	_ = queued(laptop)

	ds.PadMessageHandler.DisconnectLogout(key)

	require.True(t, revoked(t, ds, laptop))
	assert.Equal(t, []string{`["message",{"disconnect":"kicked"}]`, "<closed>"}, queued(laptop),
		"told why, then the connection ends after the message went out")
	assert.True(t, ds.Hub.HasClient(phone), "another login of the same person stays")
	assert.True(t, ds.Hub.HasClient(visitor))
}

func testInactiveAccountsDisconnect(t *testing.T, ds testutils.TestDataStore) {
	anna := loginClient(t, ds, "ws-a1", "acc.anna", "k1")
	anna2 := loginClient(t, ds, "ws-a2", "acc.anna", "k2")
	bob := loginClient(t, ds, "ws-b1", "acc.bob", "k3")
	visitor := loginClient(t, ds, "ws-v1", "", "")
	asked := map[string]int{}

	ds.PadMessageHandler.DisconnectInactiveAccounts(func(id string) bool {
		asked[id]++
		return id != "acc.anna"
	})

	assert.True(t, revoked(t, ds, anna))
	assert.True(t, revoked(t, ds, anna2))
	assert.True(t, ds.Hub.HasClient(bob))
	assert.True(t, ds.Hub.HasClient(visitor))
	assert.Equal(t, map[string]int{"acc.anna": 1, "acc.bob": 1}, asked, "each account looked up once")
}
