package ws

import (
	"encoding/json"
	"testing"

	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testutils"
	libws "github.com/ether/etherpad-go/lib/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of the access cases of etherpad-lite's socketio spec ("Normal
// accesses", "Abnormal access attempts", "Authorization levels via authorize
// hook", "... via user settings" and their interaction). The websocket
// request authenticates by itself: the user object is what webaccess builds
// for it, without the pad levels of an earlier /p/<pad> request.

type authnServer struct {
	handler *libws.PadMessageHandler
	hub     *libws.Hub
	cfg     *settings.Settings
	ds      testutils.TestDataStore
	manager *pad.Manager
}

func newAuthnServer(t *testing.T, ds testutils.TestDataStore, authz bool, authorize func(*events.AuthorizeContext), extra ...func(*hooks.Hook)) *authnServer {
	t.Helper()
	cfg := *ds.Settings
	cfg.RequireAuthentication = true
	cfg.RequireAuthorization = authz
	h := hooks.NewHook()
	if authorize != nil {
		h.EnqueueAuthorizeHook(authorize)
	}
	for _, register := range extra {
		register(&h)
	}
	hub := libws.NewHub()
	go hub.Run()
	sess := libws.NewSessionStore()
	manager := pad.NewManager(ds.DS, &h)
	handler := libws.NewPadMessageHandler(ds.DS, &h, manager, &sess, hub, ds.Logger, testutils.GetTestAssets(), &cfg)
	return &authnServer{handler: handler, hub: hub, cfg: &cfg, ds: ds, manager: manager}
}

type authnJoin struct {
	client   *libws.Client
	access   string // accessStatus of a refusal, "" when admitted
	vars     map[string]any
	author   string
	readOnly bool
}

func (s *authnServer) join(t *testing.T, padID, sessionID string, user *webaccess.SocketClientRequest) authnJoin {
	t.Helper()
	return s.joinWithToken(t, padID, sessionID, "t.authn"+sessionID, user)
}

func (s *authnServer) joinWithToken(t *testing.T, padID, sessionID, token string, user *webaccess.SocketClientRequest) authnJoin {
	t.Helper()
	client := createTestClient(s.hub, sessionID, padID, libws.NewActualMockWebSocketconn())
	if user != nil {
		client.WebAccessUser = user
	}
	t.Cleanup(func() { removeClient(s.hub, client) })
	s.handler.SessionStore.InitSessionForTest(sessionID)
	s.handler.HandleMessage(ws.ClientReady{
		Event: "message",
		Data:  ws.ClientReadyData{Component: "pad", Type: "CLIENT_READY", PadID: padID, Token: token},
	}, client, s.cfg, s.ds.Logger)

	j := authnJoin{client: client}
	for _, raw := range drainClientMessages(client) {
		var arr []json.RawMessage
		if json.Unmarshal([]byte(raw), &arr) != nil || len(arr) != 2 {
			continue
		}
		var msg struct {
			Type         string         `json:"type"`
			AccessStatus string         `json:"accessStatus"`
			Data         map[string]any `json:"data"`
		}
		_ = json.Unmarshal(arr[1], &msg)
		if msg.AccessStatus != "" {
			j.access = msg.AccessStatus
		}
		if msg.Type == "CLIENT_VARS" {
			j.vars = msg.Data
		}
	}
	if j.vars != nil {
		j.author, _ = j.vars["userId"].(string)
		j.readOnly, _ = j.vars["readonly"].(bool)
	}
	return j
}

// edit sends one insert and reports whether it was stored.
func (s *authnServer) edit(t *testing.T, j authnJoin, padID string) bool {
	t.Helper()
	p, err := s.manager.GetPad(padID, nil, nil)
	require.NoError(t, err)
	head, length := p.Head, len([]rune(p.Text()))
	s.handler.HandleMessage(ws.UserChange{
		Event: "message",
		Data: ws.UserChangeData{Component: "pad", Type: "USER_CHANGES", Data: ws.UserChangeDataData{
			Apool:     ws.UserChangeDataDataApool{NumToAttrib: map[int][]string{0: {"author", j.author}}, NextNum: 1},
			BaseRev:   head,
			Changeset: "Z:" + toBase36(length) + ">1*0+1$Q",
		}},
	}, j.client, s.cfg, s.ds.Logger)
	p, err = s.manager.GetPad(padID, nil, nil)
	require.NoError(t, err)
	return p.Head > head
}

// existingPad creates padID as an admin would.
func (s *authnServer) existingPad(t *testing.T, padID string) {
	t.Helper()
	text := "existing\n"
	_, err := s.manager.GetPad(padID, &text, nil)
	require.NoError(t, err)
}

func socketUser(name string) *webaccess.SocketClientRequest {
	return &webaccess.SocketClientRequest{Username: &name}
}

func TestSocketAccess(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "normal and abnormal accesses", Test: testSocketNormalAccess},
		testutils.TestRunConfig{Name: "levels via the authorize hook", Test: testSocketAuthorizeLevels},
		testutils.TestRunConfig{Name: "levels via user settings", Test: testSocketUserSettingsLevels},
		testutils.TestRunConfig{Name: "hook and user settings do not elevate each other", Test: testSocketLevelInteraction},
		testutils.TestRunConfig{Name: "duplicate-author handling (#7656)", Test: testSocketDuplicateAuthors},
	)
	testDb.StartTestDBHandler()
}

func testSocketNormalAccess(t *testing.T, ds testutils.TestDataStore) {
	s := newAuthnServer(t, ds, false, nil)

	// authn user /p/pad -> ok, and the user can edit (this used to be
	// refused: the level of the pad page never reached the websocket).
	j := s.join(t, "authnPad", "authnUser", socketUser("user"))
	require.Empty(t, j.access)
	require.NotNil(t, j.vars)
	assert.False(t, j.readOnly)
	assert.True(t, s.edit(t, j, "authnPad"))

	// authn anonymous -> refused, no pad is created.
	anon := s.join(t, "authnAnonPad", "authnAnon", nil)
	assert.NotEmpty(t, anon.access)
	assert.Nil(t, anon.vars)
	exists, err := s.manager.DoesPadExist("authnAnonPad")
	require.NoError(t, err)
	assert.False(t, *exists)

	// authn anonymous read-only link -> refused.
	roID := s.handler.ReadOnlyManagerForTest().GetReadOnlyId("authnPad")
	anonRO := s.join(t, roID, "authnAnonRO", nil)
	assert.NotEmpty(t, anonRO.access)

	// authn user read-only link -> ok, read-only.
	ro := s.join(t, roID, "authnUserRO", socketUser("user"))
	require.Empty(t, ro.access)
	assert.True(t, ro.readOnly)

	// Pad names with characters that must be percent-encoded.
	odd := s.join(t, "pad&name with spaces?", "authnOdd", socketUser("user"))
	assert.Empty(t, odd.access)
}

func testSocketAuthorizeLevels(t *testing.T, ds testutils.TestDataStore) {
	for _, tc := range []struct {
		name               string
		level              string
		editOnly, existing bool
		wantAccess         bool
		wantReadOnly       bool
	}{
		{"level create -> can create", "create", false, false, true, false},
		{"level modify -> can modify", "modify", false, true, true, false},
		{"level create, editOnly -> unable to create", "create", true, false, false, false},
		{"level modify -> unable to create", "modify", false, false, false, false},
		{"level readOnly -> unable to create", "readOnly", false, false, false, false},
		{"level readOnly -> unable to modify", "readOnly", false, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newAuthnServer(t, ds, true, func(c *events.AuthorizeContext) { c.Grant(tc.level) })
			s.cfg.EditOnly = tc.editOnly
			padID := "authzPad" + tc.level + boolName(tc.editOnly) + boolName(tc.existing)
			if tc.existing {
				s.existingPad(t, padID)
			}
			j := s.join(t, padID, "authz"+padID, socketUser("user"))
			if !tc.wantAccess {
				assert.NotEmpty(t, j.access)
				return
			}
			require.Empty(t, j.access)
			assert.Equal(t, tc.wantReadOnly, j.readOnly)
			assert.Equal(t, !tc.wantReadOnly, s.edit(t, j, padID), "an edit is stored only when the user may modify")
		})
	}

	t.Run("no answer from the hook -> refused", func(t *testing.T) {
		s := newAuthnServer(t, ds, true, func(*events.AuthorizeContext) {})
		assert.NotEmpty(t, s.join(t, "authzDefer", "authzDefer", socketUser("user")).access)
	})

	t.Run("the hook gets the pad id", func(t *testing.T) {
		var seen string
		s := newAuthnServer(t, ds, true, func(c *events.AuthorizeContext) { seen = c.PadId; c.Grant("create") })
		s.join(t, "authzSeen", "authzSeen", socketUser("user"))
		assert.Equal(t, "authzSeen", seen)
	})

	t.Run("an admin needs no authorization", func(t *testing.T) {
		s := newAuthnServer(t, ds, true, nil)
		admin := socketUser("admin")
		admin.IsAdmin = true
		j := s.join(t, "authzAdmin", "authzAdmin", admin)
		require.Empty(t, j.access)
		assert.True(t, s.edit(t, j, "authzAdmin"))
	})
}

func boolName(b bool) string {
	if b {
		return "T"
	}
	return "F"
}

func testSocketUserSettingsLevels(t *testing.T, ds testutils.TestDataStore) {
	tru, fal := true, false
	for _, tc := range []struct {
		name                string
		readOnly, canCreate *bool
		existing            bool
		wantAccess          bool
		wantReadOnly        bool
	}{
		{"canCreate=true -> can create and modify", nil, &tru, false, true, false},
		{"canCreate=false -> unable to create", nil, &fal, false, false, false},
		{"canCreate=false -> can modify", nil, &fal, true, true, false},
		{"readOnly=true -> unable to create", &tru, nil, false, false, false},
		{"readOnly=true -> unable to modify", &tru, nil, true, true, true},
		{"readOnly=false -> can create and modify", &fal, nil, false, true, false},
		{"readOnly=true, canCreate=true -> unable to create", &tru, &tru, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newAuthnServer(t, ds, false, nil)
			padID := "userLevel" + ptrName(tc.readOnly) + ptrName(tc.canCreate) + boolName(tc.existing)
			if tc.existing {
				s.existingPad(t, padID)
			}
			user := socketUser("user")
			user.ReadOnly, user.CanCreate = tc.readOnly, tc.canCreate
			j := s.join(t, padID, "u"+padID, user)
			if !tc.wantAccess {
				assert.NotEmpty(t, j.access)
				return
			}
			require.Empty(t, j.access)
			assert.Equal(t, tc.wantReadOnly, j.readOnly)
			assert.Equal(t, !tc.wantReadOnly, s.edit(t, j, padID))
		})
	}
}

func ptrName(b *bool) string {
	if b == nil {
		return "N"
	}
	return boolName(*b)
}

func testSocketLevelInteraction(t *testing.T, ds testutils.TestDataStore) {
	t.Run("the authorize hook does not elevate a read-only user", func(t *testing.T) {
		s := newAuthnServer(t, ds, true, func(c *events.AuthorizeContext) { c.Grant("create") })
		s.existingPad(t, "elevate1")
		tru := true
		user := socketUser("user")
		user.ReadOnly = &tru
		j := s.join(t, "elevate1", "elevate1", user)
		require.Empty(t, j.access)
		assert.True(t, j.readOnly)
		assert.False(t, s.edit(t, j, "elevate1"))
	})
	t.Run("user settings do not elevate the hook's level", func(t *testing.T) {
		s := newAuthnServer(t, ds, true, func(c *events.AuthorizeContext) { c.Grant("readOnly") })
		s.existingPad(t, "elevate2")
		fal := false
		user := socketUser("user")
		user.ReadOnly = &fal
		j := s.join(t, "elevate2", "elevate2", user)
		require.Empty(t, j.access)
		assert.True(t, j.readOnly)
		assert.False(t, s.edit(t, j, "elevate2"))
	})
}

// rawFrames returns what a client received, decoded.
func rawFrames(client *libws.Client) []map[string]any {
	var out []map[string]any
	for _, raw := range drainClientMessages(client) {
		var arr []json.RawMessage
		if json.Unmarshal([]byte(raw), &arr) != nil || len(arr) != 2 {
			continue
		}
		var msg map[string]any
		if json.Unmarshal(arr[1], &msg) == nil {
			out = append(out, msg)
		}
	}
	return out
}

func disconnects(frames []map[string]any) (out []string) {
	for _, f := range frames {
		if d, ok := f["disconnect"].(string); ok {
			out = append(out, d)
		}
	}
	return out
}

func userLeaves(frames []map[string]any) (out []string) {
	for _, f := range frames {
		data, _ := f["data"].(map[string]any)
		if data["type"] == "USER_LEAVE" {
			info, _ := data["userInfo"].(map[string]any)
			id, _ := info["userId"].(string)
			out = append(out, id)
		}
	}
	return out
}

func testSocketDuplicateAuthors(t *testing.T, ds testutils.TestDataStore) {
	const token = "t.sameBrowserToken0000"

	t.Run("cookie identity: a second socket of the same author kicks the first", func(t *testing.T) {
		s := newAuthnServer(t, ds, false, nil)
		s.cfg.RequireAuthentication = false
		a := s.joinWithToken(t, "dupCookie", "dupCookieA", token, nil)
		require.NotNil(t, a.vars)
		b := s.joinWithToken(t, "dupCookie", "dupCookieB", token, nil)
		require.NotNil(t, b.vars)
		assert.Equal(t, a.author, b.author)
		assert.Equal(t, []string{"userdup"}, disconnects(rawFrames(a.client)))
	})

	t.Run("authenticated identity: a second socket does not kick the first", func(t *testing.T) {
		s := newAuthnServer(t, ds, false, nil)
		a := s.joinWithToken(t, "dupAuthn", "dupAuthnA", token, socketUser("user"))
		require.NotNil(t, a.vars)
		b := s.joinWithToken(t, "dupAuthn", "dupAuthnB", token, socketUser("user"))
		require.NotNil(t, b.vars)
		assert.Equal(t, a.author, b.author)
		assert.Empty(t, disconnects(rawFrames(a.client)))
	})

	t.Run("authenticated identity: closing one socket does not remove the author for the other", func(t *testing.T) {
		s := newAuthnServer(t, ds, false, nil)
		a := s.joinWithToken(t, "dupLeave", "dupLeaveA", token, socketUser("user"))
		b := s.joinWithToken(t, "dupLeave", "dupLeaveB", token, socketUser("user"))
		rawFrames(b.client)
		s.handler.HandleDisconnectOfPadClient(a.client, s.cfg, s.ds.Logger)
		assert.Empty(t, userLeaves(rawFrames(b.client)))

		// When the last one leaves, a third user hears it.
		c := s.join(t, "dupLeave", "dupLeaveC", socketUser("other"))
		rawFrames(c.client)
		s.handler.HandleDisconnectOfPadClient(b.client, s.cfg, s.ds.Logger)
		assert.Equal(t, []string{b.author}, userLeaves(rawFrames(c.client)))
	})

	t.Run("different authors: closing one socket announces USER_LEAVE", func(t *testing.T) {
		s := newAuthnServer(t, ds, false, nil)
		s.cfg.RequireAuthentication = false
		a := s.join(t, "dupOther", "dupOtherA", nil)
		b := s.join(t, "dupOther", "dupOtherB", nil)
		require.NotEqual(t, a.author, b.author)
		rawFrames(b.client)
		s.handler.HandleDisconnectOfPadClient(a.client, s.cfg, s.ds.Logger)
		assert.Equal(t, []string{a.author}, userLeaves(rawFrames(b.client)))
	})
}
