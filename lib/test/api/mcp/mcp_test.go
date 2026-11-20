package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/api/mcp"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/ether/etherpad-go/lib/tokens"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCP(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "tokens decide", Test: testMCPRights},
		testutils.TestRunConfig{Name: "editing keeps the rest", Test: testMCPEditing},
		testutils.TestRunConfig{Name: "chat", Test: testMCPChat},
	)
	testDb.StartTestDBHandler()
}

type env struct {
	t      *testing.T
	ds     testutils.TestDataStore
	keys   *tokens.Manager
	acc    *accounts.Manager
	server *httptest.Server
}

func newEnv(t *testing.T, ds testutils.TestDataStore) *env {
	t.Helper()
	store := ds.ToInitStore()
	acc := accounts.For(ds.DS)
	require.NoError(t, acc.EnsureDefaults(accounts.SeedConfig{Template: accounts.TemplateClosed}, nil))
	keys := tokens.NewManager(ds.DS, acc, nil)
	mcp.New(store, keys).Register(store.C)
	srv := httptest.NewServer(adaptor.FiberApp(store.C))
	t.Cleanup(srv.Close)
	return &env{t: t, ds: ds, keys: keys, acc: acc, server: srv}
}

// account makes an account of kind with role for pads.
func (e *env) account(kind, role, pads string) string {
	e.t.Helper()
	n := accounts.NewAccount{Username: "u-" + utils.RandomString(8), Kind: kind, DisplayName: "Claude " + utils.RandomString(4)}
	if kind == accounts.KindHuman {
		n.Password = "a-long-password-" + utils.RandomString(8)
	}
	a, err := e.acc.CreateAccount(accounts.System, n)
	require.NoError(e.t, err)
	if role != "" {
		require.NoError(e.t, e.acc.Assign(accounts.System, a.ID, role, pads))
	}
	return a.ID
}

func (e *env) token(account string, scopes []string, pads string) string {
	e.t.Helper()
	key, _, err := e.keys.Issue(accounts.System, tokens.Spec{AccountID: account, Name: "mcp", Scopes: scopes, PadPattern: pads})
	require.NoError(e.t, err)
	return key
}

type bearer struct {
	key  string
	base http.RoundTripper
}

func (b bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.key)
	return b.base.RoundTrip(r)
}

func (e *env) connect(key string) *sdk.ClientSession {
	e.t.Helper()
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(context.Background(), &sdk.StreamableClientTransport{Endpoint: e.server.URL + mcp.Path,
		HTTPClient: &http.Client{Transport: bearer{key: key, base: http.DefaultTransport}}, DisableStandaloneSSE: true, MaxRetries: -1}, nil)
	require.NoError(e.t, err)
	e.t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// call runs a tool; it returns its structured answer, or the error text.
func call(t *testing.T, cs *sdk.ClientSession, tool string, args map[string]any) (map[string]any, string) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: tool, Arguments: args})
	require.NoError(t, err)
	if res.IsError {
		var msg []string
		for _, c := range res.Content {
			if tc, ok := c.(*sdk.TextContent); ok {
				msg = append(msg, tc.Text)
			}
		}
		return nil, strings.Join(msg, " ")
	}
	out := map[string]any{}
	data, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &out))
	return out, ""
}

func mustCall(t *testing.T, cs *sdk.ClientSession, tool string, args map[string]any) map[string]any {
	t.Helper()
	out, errText := call(t, cs, tool, args)
	require.Empty(t, errText, tool)
	return out
}

func (e *env) makePad(id, text string) {
	e.t.Helper()
	// A new pad gets its final newline added.
	text = strings.TrimSuffix(text, "\n")
	p, err := e.ds.PadManager.GetPad(id, &text, nil)
	require.NoError(e.t, err)
	require.NotNil(e.t, p)
}

func (e *env) text(id string) string {
	e.t.Helper()
	p, err := e.ds.PadManager.GetPad(id, nil, nil)
	require.NoError(e.t, err)
	return p.Text()
}

func testMCPRights(t *testing.T, ds testutils.TestDataStore) {
	e := newEnv(t, ds)
	e.makePad("team-a", "alpha\n")
	e.makePad("team-b", "beta\n")
	e.makePad("private", "secret\n")

	// Without a token, or with a wrong one: 401 before any MCP.
	for _, auth := range []string{"", "Bearer ep_" + strings.Repeat("x", 40)} {
		req, _ := http.NewRequest(http.MethodPost, e.server.URL+mcp.Path, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, auth)
		assert.Contains(t, resp.Header.Get("WWW-Authenticate"), "Bearer")
	}

	// An editor of all pads, with a token for team-* that may only read.
	svc := e.account(accounts.KindService, accounts.RoleEditor, "")
	reader := e.connect(e.token(svc, []string{accounts.ScopePadRead}, "team-*"))
	tools, err := reader.ListTools(context.Background(), nil)
	require.NoError(t, err)
	names := []string{}
	for _, tl := range tools.Tools {
		names = append(names, tl.Name)
	}
	assert.Subset(t, names, []string{"list_pads", "read_pad", "create_pad", "append_text", "replace_text", "set_text", "read_chat", "send_chat"})

	out := mustCall(t, reader, "list_pads", nil)
	data, _ := json.Marshal(out["pads"])
	assert.JSONEq(t, `[{"id":"team-a","writable":false},{"id":"team-b","writable":false}]`, string(data))
	out = mustCall(t, reader, "list_pads", map[string]any{"query": "B"})
	data, _ = json.Marshal(out["pads"])
	assert.JSONEq(t, `[{"id":"team-b","writable":false}]`, string(data))

	out = mustCall(t, reader, "read_pad", map[string]any{"pad": "team-a"})
	assert.Equal(t, "alpha\n", out["text"])
	_, errText := call(t, reader, "read_pad", map[string]any{"pad": "private"})
	assert.Contains(t, errText, "forbidden")
	_, errText = call(t, reader, "append_text", map[string]any{"pad": "team-a", "text": "x"})
	assert.Contains(t, errText, "forbidden")
	_, errText = call(t, reader, "read_pad", map[string]any{"pad": "team-nope"})
	assert.Contains(t, errText, "does not exist")
	assert.Equal(t, "alpha\n", e.text("team-a"))

	// The token can never do more than its account: a viewer's token with
	// pad:write may not write.
	viewer := e.account(accounts.KindService, accounts.RoleViewer, "")
	cs := e.connect(e.token(viewer, []string{accounts.ScopePadRead, accounts.ScopePadWrite}, ""))
	_, errText = call(t, cs, "append_text", map[string]any{"pad": "team-a", "text": "x"})
	assert.Contains(t, errText, "forbidden")

	// Creating needs pad:create, also from an editor.
	writer := e.connect(e.token(svc, []string{accounts.ScopePadRead, accounts.ScopePadWrite}, "team-*"))
	_, errText = call(t, writer, "create_pad", map[string]any{"pad": "team-c", "text": "new"})
	assert.Contains(t, errText, "forbidden")
	creator := e.connect(e.token(svc, []string{accounts.ScopePadCreate, accounts.ScopePadRead}, "team-*"))
	out = mustCall(t, creator, "create_pad", map[string]any{"pad": "team-c", "text": "new\n"})
	assert.Equal(t, "team-c", out["pad"])
	assert.Equal(t, "new\n", e.text("team-c"))
	_, errText = call(t, creator, "create_pad", map[string]any{"pad": "team-c"})
	assert.Contains(t, errText, "already exists")
	_, errText = call(t, creator, "create_pad", map[string]any{"pad": "elsewhere"})
	assert.Contains(t, errText, "forbidden")

	// A revoked token stops at once.
	key := e.token(svc, []string{accounts.ScopePadRead}, "")
	cs = e.connect(key)
	mustCall(t, cs, "read_pad", map[string]any{"pad": "private"})
	list, err := e.keys.List(accounts.System, svc)
	require.NoError(t, err)
	for _, tk := range list {
		if strings.HasPrefix(key, tk.Prefix) {
			require.NoError(t, e.keys.Revoke(accounts.System, tk.ID))
		}
	}
	_, err = cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "read_pad", Arguments: map[string]any{"pad": "private"}})
	assert.Error(t, err)
}

func testMCPEditing(t *testing.T, ds testutils.TestDataStore) {
	e := newEnv(t, ds)
	e.makePad("notes", "one two one\nthree\n")
	svc := e.account(accounts.KindService, accounts.RoleEditor, "")
	cs := e.connect(e.token(svc, []string{accounts.ScopePadRead, accounts.ScopePadWrite}, ""))

	out := mustCall(t, cs, "append_text", map[string]any{"pad": "notes", "text": "\nfour"})
	assert.Equal(t, "one two one\nthree\nfour\n", e.text("notes"))
	rev := out["revision"].(float64)

	_, errText := call(t, cs, "replace_text", map[string]any{"pad": "notes", "find": "one", "replace": "1"})
	assert.Contains(t, errText, "occurs 2 times")
	_, errText = call(t, cs, "replace_text", map[string]any{"pad": "notes", "find": "missing", "replace": "x"})
	assert.Contains(t, errText, "does not occur")
	out = mustCall(t, cs, "replace_text", map[string]any{"pad": "notes", "find": "two", "replace": "zwei"})
	assert.Greater(t, out["revision"].(float64), rev)
	out = mustCall(t, cs, "replace_text", map[string]any{"pad": "notes", "find": "one", "replace": "eins", "all": true})
	assert.Equal(t, float64(2), out["replaced"])
	assert.Equal(t, "eins zwei eins\nthree\nfour\n", e.text("notes"))
	mustCall(t, cs, "replace_text", map[string]any{"pad": "notes", "find": "three\n", "replace": ""})
	assert.Equal(t, "eins zwei eins\nfour\n", e.text("notes"))

	// Non-ASCII offsets are characters, not bytes.
	e.makePad("umlaut", "Grüße an Jörg\n")
	mustCall(t, cs, "replace_text", map[string]any{"pad": "umlaut", "find": "Jörg", "replace": "Anna"})
	assert.Equal(t, "Grüße an Anna\n", e.text("umlaut"))

	mustCall(t, cs, "set_text", map[string]any{"pad": "notes", "text": "fresh"})
	assert.Equal(t, "fresh\n", e.text("notes"))

	// The changes are the service account's author, under its own name.
	a, err := e.acc.Holder(svc)
	require.NoError(t, err)
	require.NotEmpty(t, a.AuthorID)
	p, err := e.ds.PadManager.GetPad("notes", nil, nil)
	require.NoError(t, err)
	authors := p.GetAllAuthors()
	assert.Contains(t, authors, a.AuthorID)

	// A person's token writes as the person, the same author as in the pad.
	person := e.account(accounts.KindHuman, accounts.RoleEditor, "")
	cs = e.connect(e.token(person, []string{accounts.ScopePadRead, accounts.ScopePadWrite}, ""))
	mustCall(t, cs, "append_text", map[string]any{"pad": "notes", "text": " by a person"})
	human, err := e.acc.Holder(person)
	require.NoError(t, err)
	require.NotEmpty(t, human.AuthorID)
	assert.NotEqual(t, a.AuthorID, human.AuthorID)
}

func testMCPChat(t *testing.T, ds testutils.TestDataStore) {
	e := newEnv(t, ds)
	e.makePad("talk", "hello\n")
	svc := e.account(accounts.KindService, accounts.RoleEditor, "")
	cs := e.connect(e.token(svc, []string{accounts.ScopePadRead, accounts.ScopeChatWrite}, ""))

	out := mustCall(t, cs, "read_chat", map[string]any{"pad": "talk"})
	assert.Empty(t, out["messages"])
	for _, m := range []string{"first", "second", "third"} {
		mustCall(t, cs, "send_chat", map[string]any{"pad": "talk", "text": m})
	}
	out = mustCall(t, cs, "read_chat", map[string]any{"pad": "talk", "limit": 2})
	msgs := out["messages"].([]any)
	require.Len(t, msgs, 2)
	assert.Equal(t, "second", msgs[0].(map[string]any)["text"])
	assert.Equal(t, "third", msgs[1].(map[string]any)["text"])
	assert.True(t, strings.HasPrefix(msgs[1].(map[string]any)["author"].(string), "Claude "))

	// No chat on a pad that does not exist: it is not created by it.
	_, errText := call(t, cs, "send_chat", map[string]any{"pad": "ghost", "text": "boo"})
	assert.Contains(t, errText, "does not exist")
	exists, err := ds.PadManager.DoesPadExist("ghost")
	require.NoError(t, err)
	assert.False(t, *exists)

	// chat:write is its own right.
	reader := e.connect(e.token(svc, []string{accounts.ScopePadRead}, ""))
	_, errText = call(t, reader, "send_chat", map[string]any{"pad": "talk", "text": "x"})
	assert.Contains(t, errText, "forbidden")
}
