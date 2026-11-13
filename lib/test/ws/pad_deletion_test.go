package ws

import (
	"testing"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's socketio spec "Pad deletion token issuance
// (#7926)", plus the rules of handlePadDelete.
func TestPadDeletion(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "deletion token issuance", Test: testDeletionTokenIssuance},
		testutils.TestRunConfig{Name: "who may delete", Test: testWhoMayDelete},
	)
	testDb.StartTestDBHandler()
}

// anonymous is a server without requireAuthentication.
func anonymous(t *testing.T, ds testutils.TestDataStore, extra ...func(*hooks.Hook)) *authnServer {
	s := newAuthnServer(t, ds, false, nil, extra...)
	s.cfg.RequireAuthentication = false
	return s
}

func stableIdentityHook(h *hooks.Hook) {
	h.EnqueueGetAuthorIdHook(func(ctx *events.GetAuthorIdContext) {})
}

func testDeletionTokenIssuance(t *testing.T, ds testutils.TestDataStore) {
	t.Run("an anonymous creator gets a deletion token, once", func(t *testing.T) {
		s := anonymous(t, ds)
		j := s.join(t, "delIssue1", "delIssue1", nil)
		token, ok := j.vars["padDeletionToken"].(string)
		require.True(t, ok, "the creator gets a token for the save-token dialog")
		assert.GreaterOrEqual(t, len(token), 32)
		assert.Equal(t, false, j.vars["canDeleteWithoutToken"])
		assert.Equal(t, true, j.vars["canDeletePad"])

		again := s.joinWithToken(t, "delIssue1", "delIssue1b", "t.authndelIssue1", nil)
		assert.Nil(t, again.vars["padDeletionToken"], "the plaintext is handed out once")
		assert.Equal(t, true, again.vars["canDeletePad"])
	})

	t.Run("no token when allowPadDeletionByAllUsers", func(t *testing.T) {
		s := anonymous(t, ds)
		s.cfg.AllowPadDeletionByAllUsers = true
		j := s.join(t, "delIssue2", "delIssue2", nil)
		assert.Nil(t, j.vars["padDeletionToken"])
		assert.Equal(t, true, j.vars["canDeleteWithoutToken"])
		assert.Equal(t, true, j.vars["canDeletePad"])
	})

	t.Run("a non-creator may not delete by default, may with allowPadDeletionByAllUsers", func(t *testing.T) {
		s := anonymous(t, ds)
		creator := s.join(t, "delIssue3", "delIssue3c", nil)
		assert.Equal(t, true, creator.vars["canDeletePad"])
		other := s.join(t, "delIssue3", "delIssue3o", nil)
		assert.Equal(t, false, other.vars["canDeletePad"])
		assert.Nil(t, other.vars["padDeletionToken"])
		s.cfg.AllowPadDeletionByAllUsers = true
		other2 := s.join(t, "delIssue3", "delIssue3o2", nil)
		assert.Equal(t, true, other2.vars["canDeletePad"])
	})

	t.Run("a read-only viewer may not delete, even with allowPadDeletionByAllUsers", func(t *testing.T) {
		s := anonymous(t, ds)
		s.cfg.AllowPadDeletionByAllUsers = true
		s.join(t, "delIssue4", "delIssue4c", nil)
		roID := s.handler.ReadOnlyManagerForTest().GetReadOnlyId("delIssue4")
		ro := s.join(t, roID, "delIssue4ro", nil)
		assert.Equal(t, true, ro.readOnly)
		assert.Equal(t, false, ro.vars["canDeletePad"])
		assert.Nil(t, ro.vars["padDeletionToken"])

		s.handler.HandlePadDelete(ro.client, padDeleteMessage(roID, ""), s.cfg)
		exists, _ := s.manager.DoesPadExist("delIssue4")
		assert.True(t, *exists, "a read-only session cannot delete without a token")
	})

	t.Run("an authenticated creator without a getAuthorId hook still gets a token", func(t *testing.T) {
		s := newAuthnServer(t, ds, false, nil)
		j := s.join(t, "delIssue5", "delIssue5", socketUser("user"))
		_, ok := j.vars["padDeletionToken"].(string)
		assert.True(t, ok)
		assert.Equal(t, false, j.vars["canDeleteWithoutToken"])
	})

	t.Run("an authenticated creator with a getAuthorId hook gets none (durable identity)", func(t *testing.T) {
		s := newAuthnServer(t, ds, false, nil, stableIdentityHook)
		j := s.join(t, "delIssue6", "delIssue6", socketUser("user"))
		assert.Nil(t, j.vars["padDeletionToken"])
		assert.Equal(t, true, j.vars["canDeleteWithoutToken"])
		assert.Equal(t, true, j.vars["canDeletePad"])
	})

	t.Run("suppressPadDeletionTokenModal keeps the token but hides the dialog", func(t *testing.T) {
		s := anonymous(t, ds)
		s.cfg.SuppressPadDeletionTokenModal = true
		j := s.join(t, "delIssue7", "delIssue7", nil)
		_, ok := j.vars["padDeletionToken"].(string)
		assert.True(t, ok)
		assert.Equal(t, true, j.vars["suppressPadDeletionTokenModal"])
	})
}

func shoutKeys(frames []map[string]any) (keys []string) {
	for _, f := range frames {
		data, _ := f["data"].(map[string]any)
		if data["type"] != "shoutMessage" {
			continue
		}
		payload, _ := data["payload"].(map[string]any)
		msg, _ := payload["message"].(map[string]any)
		if k, ok := msg["messageKey"].(string); ok {
			keys = append(keys, k)
		}
	}
	return keys
}

func testWhoMayDelete(t *testing.T, ds testutils.TestDataStore) {
	exists := func(s *authnServer, padID string) bool {
		e, err := s.manager.DoesPadExist(padID)
		require.NoError(t, err)
		return *e
	}

	t.Run("the creator", func(t *testing.T) {
		s := anonymous(t, ds)
		c := s.join(t, "delWho1", "delWho1", nil)
		s.handler.HandlePadDelete(c.client, padDeleteMessage("delWho1", ""), s.cfg)
		assert.False(t, exists(s, "delWho1"))
	})

	t.Run("not another user", func(t *testing.T) {
		s := anonymous(t, ds)
		s.join(t, "delWho2", "delWho2c", nil)
		o := s.join(t, "delWho2", "delWho2o", nil)
		s.handler.HandlePadDelete(o.client, padDeleteMessage("delWho2", ""), s.cfg)
		assert.True(t, exists(s, "delWho2"))
		assert.Equal(t, []string{"pad.deletionToken.notCreator"}, shoutKeys(rawFrames(o.client)))
	})

	t.Run("another user with the token, also from a read-only session", func(t *testing.T) {
		s := anonymous(t, ds)
		c := s.join(t, "delWho3", "delWho3c", nil)
		token := c.vars["padDeletionToken"].(string)
		roID := s.handler.ReadOnlyManagerForTest().GetReadOnlyId("delWho3")
		ro := s.join(t, roID, "delWho3ro", nil)
		s.handler.HandlePadDelete(ro.client, padDeleteMessage(roID, token), s.cfg)
		assert.False(t, exists(s, "delWho3"))
	})

	t.Run("a wrong token fails, even for the creator", func(t *testing.T) {
		s := anonymous(t, ds)
		c := s.join(t, "delWho4", "delWho4", nil)
		s.handler.HandlePadDelete(c.client, padDeleteMessage("delWho4", "wrong-token"), s.cfg)
		assert.True(t, exists(s, "delWho4"))
		assert.Equal(t, []string{"pad.deletionToken.invalid"}, shoutKeys(rawFrames(c.client)))
	})

	t.Run("anyone who can edit, with allowPadDeletionByAllUsers", func(t *testing.T) {
		s := anonymous(t, ds)
		s.cfg.AllowPadDeletionByAllUsers = true
		s.join(t, "delWho5", "delWho5c", nil)
		o := s.join(t, "delWho5", "delWho5o", nil)
		s.handler.HandlePadDelete(o.client, padDeleteMessage("delWho5", ""), s.cfg)
		assert.False(t, exists(s, "delWho5"))
	})

	t.Run("not another pad than the session's", func(t *testing.T) {
		s := anonymous(t, ds)
		s.join(t, "delWho6a", "delWho6a", nil)
		// Same browser (token) on another pad: the creator of both.
		b := s.joinWithToken(t, "delWho6b", "delWho6b", "t.authndelWho6a", nil)
		s.handler.HandlePadDelete(b.client, padDeleteMessage("delWho6a", ""), s.cfg)
		assert.True(t, exists(s, "delWho6a"), "a session deletes only its own pad")
	})
}
