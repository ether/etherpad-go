package ws

import (
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/author"
	db2 "github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func drain(c *Client) []string {
	var out []string
	for {
		select {
		case m := <-c.Send:
			out = append(out, string(m))
		default:
			return out
		}
	}
}

// The CLIENT_READY catch-up must only reach the joining client
// (etherpad-lite #8409); other clients' revisions are advanced by the writer
// that appended the revision.
func TestUpdatePadClientsCanTargetOneClient(t *testing.T) {
	store := db2.NewMemoryDataStore()
	hook := hooks.NewHook()
	padManager := pad.NewManager(store, &hook)
	ss := NewSessionStore()
	hub := NewHub()
	h := &PadMessageHandler{
		SessionStore:  &ss,
		hub:           hub,
		Logger:        zap.NewNop().Sugar(),
		padManager:    padManager,
		authorManager: author.NewManager(store),
	}

	a, err := h.authorManager.CreateAuthor(nil)
	require.NoError(t, err)
	text := "hello\n"
	p, err := padManager.GetPad("catchup-pad", &text, &a.Id)
	require.NoError(t, err)
	require.NoError(t, p.SetText("hello world\n", &a.Id))
	require.Greater(t, p.Head, 0)

	clients := make([]*Client, 2)
	for i, sid := range []string{"catchup-joiner", "catchup-other"} {
		ss.InitSessionForTest(sid)
		ss.SetPadIdForTest(sid, p.Id)
		ss.SetAuthorForTest(sid, a.Id)
		ss.SetRevisionForTest(sid, 0)
		clients[i] = &Client{SessionId: sid, Send: make(chan []byte, 64), Hub: hub}
		hub.Clients[clients[i]] = true
	}
	joiner, other := clients[0], clients[1]

	h.updatePadClients(p, joiner)
	assert.NotEmpty(t, drain(joiner), "joiner must be caught up")
	assert.Empty(t, drain(other), "other clients must not be touched by a join catch-up")
	assert.Equal(t, p.Head, ss.GetSessionForTest(joiner.SessionId).Revision)
	assert.Equal(t, 0, ss.GetSessionForTest(other.SessionId).Revision)

	h.UpdatePadClients(p)
	msgs := drain(other)
	require.NotEmpty(t, msgs)
	assert.True(t, strings.Contains(msgs[0], "NEW_CHANGES"))
	assert.Empty(t, drain(joiner), "joiner was already up to date")
}
