package ws

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/ether/etherpad-go/lib/cluster"
	"github.com/ether/etherpad-go/lib/models/webaccess"
)

// LoginSessionKey is how connections and cluster notices name a pad login
// session: a hash, so the session id itself is not passed around.
func LoginSessionKey(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(sum[:16])
}

// accountOf is the account a connection was opened for, "" for none.
func accountOf(c *Client) string {
	u, _ := c.WebAccessUser.(*webaccess.SocketClientRequest)
	if u == nil {
		return ""
	}
	return u.AccountID
}

// DisconnectLogout closes the connections opened with a pad login session
// that ended, here and on the other instances: an open pad does not keep
// writing with the rights of a logout.
func (p *PadMessageHandler) DisconnectLogout(key string) {
	if key == "" {
		return
	}
	p.disconnectLoginLocal(key)
	p.publish(cluster.Event{Kind: cluster.KindLogout, Session: key})
}

func (p *PadMessageHandler) disconnectLoginLocal(key string) {
	if key == "" {
		return
	}
	p.disconnectWhere(func(c *Client) bool { return c.LoginSession == key })
}

// DisconnectInactiveAccounts closes this instance's connections of
// accounts that may no longer be logged in (active says). Every instance
// runs it on a change of the accounts, its own or another's.
func (p *PadMessageHandler) DisconnectInactiveAccounts(active func(accountID string) bool) {
	known := map[string]bool{}
	p.disconnectWhere(func(c *Client) bool {
		id := accountOf(c)
		if id == "" {
			return false
		}
		ok, seen := known[id]
		if !seen {
			ok = active(id)
			known[id] = ok
		}
		return !ok
	})
}

func (p *PadMessageHandler) disconnectWhere(match func(*Client) bool) {
	var victims []*Client
	p.hub.ClientsRWMutex.RLock()
	for c := range p.hub.Clients {
		if c != nil && match(c) {
			victims = append(victims, c)
		}
	}
	p.hub.ClientsRWMutex.RUnlock()
	for _, c := range victims {
		p.revoke(c)
	}
}

// revoke ends a connection whose rights are gone: its session takes no
// further messages at once, the client is told ("kicked") and then the
// connection closes, after the message went out (closing first would drop
// it, and the client would reconnect without a word).
func (p *PadMessageHandler) revoke(c *Client) {
	if c.SessionId != "" {
		p.SessionStore.resetSession(c.SessionId)
	}
	sendDisconnectMessage(c, "kicked")
	c.Leave()
}
