package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/api/mcp"
	"github.com/ether/etherpad-go/lib/api/oidc"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/ether/etherpad-go/lib/tokens"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOAuth stands for the provider: one access token of a person.
type fakeOAuth struct {
	token  string
	access *oidc.MCPAccess
	on     bool
}

func (f *fakeOAuth) MCPAccessToken(token string) (*oidc.MCPAccess, error) {
	if !f.on || token != f.token {
		return nil, oidc.ErrNotMCPToken
	}
	return f.access, nil
}

func (f *fakeOAuth) MCPOAuthIssuer() (string, bool) { return "https://pads.example", f.on }

func TestMCPOAuth(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "oauth", Test: testMCPOAuth})
	testDb.StartTestDBHandler()
}

func testMCPOAuth(t *testing.T, ds testutils.TestDataStore) {
	store := ds.ToInitStore()
	acc := accounts.For(ds.DS)
	require.NoError(t, acc.EnsureDefaults(accounts.SeedConfig{Template: accounts.TemplateClosed}, nil))
	keys := tokens.NewManager(ds.DS, acc, nil)
	person, err := acc.CreateAccount(accounts.System, accounts.NewAccount{Username: "p-" + utils.RandomString(6),
		Kind: accounts.KindHuman, Password: "a-long-password-123"})
	require.NoError(t, err)
	require.NoError(t, acc.Assign(accounts.System, person.ID, accounts.RoleEditor, ""))
	person, err = acc.Person(person.ID)
	require.NoError(t, err)
	fake := &fakeOAuth{token: "oauth-" + utils.RandomString(20), on: true,
		access: &oidc.MCPAccess{Account: person, ClientID: "mcp-x", Scopes: []string{accounts.ScopePadRead},
			PadPattern: "team-*", ExpiresAt: time.Now().Add(time.Hour)}}
	store.RetrievedSettings.PublicURL = "https://pads.example/"
	mcp.New(store, keys).WithOAuth(fake).Register(store.C)
	srv := httptest.NewServer(adaptor.FiberApp(store.C))
	t.Cleanup(srv.Close)
	e := &env{t: t, ds: ds, keys: keys, acc: acc, server: srv}
	e.makePad("team-a", "alpha")
	e.makePad("private", "secret")

	// Without a token: where to get one (RFC 9728).
	req, _ := http.NewRequest(http.MethodPost, srv.URL+mcp.Path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("WWW-Authenticate"),
		`resource_metadata="https://pads.example/.well-known/oauth-protected-resource/mcp"`)

	for _, p := range []string{mcp.ResourceMetadataPath, mcp.ResourceMetadataPath + mcp.Path} {
		resp, err = http.Get(srv.URL + p)
		require.NoError(t, err)
		var doc map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&doc))
		_ = resp.Body.Close()
		assert.Equal(t, "https://pads.example/mcp", doc["resource"])
		assert.Equal(t, []any{"https://pads.example"}, doc["authorization_servers"])
	}

	// The OAuth token: the person's rights, cut down to what they allowed.
	cs := e.connect(fake.token)
	out := mustCall(t, cs, "list_pads", nil)
	data, _ := json.Marshal(out["pads"])
	assert.JSONEq(t, `[{"id":"team-a","writable":false}]`, string(data))
	mustCall(t, cs, "read_pad", map[string]any{"pad": "team-a"})
	_, errText := call(t, cs, "read_pad", map[string]any{"pad": "private"})
	assert.Contains(t, errText, "forbidden")
	_, errText = call(t, cs, "append_text", map[string]any{"pad": "team-a", "text": "x"})
	assert.Contains(t, errText, "forbidden")

	// Keys of the HTTP API keep working beside it, also without ep_.
	legacyKey := "plain-" + utils.RandomString(10)
	_, err = keys.ImportKey(legacyKey, "APIKEY.txt")
	require.NoError(t, err)
	mustCall(t, e.connect(legacyKey), "read_pad", map[string]any{"pad": "private"})

	// Off: no metadata, and OAuth tokens are refused.
	fake.on = false
	resp, err = http.Get(srv.URL + mcp.ResourceMetadataPath)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	_, err = cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "read_pad", Arguments: map[string]any{"pad": "team-a"}})
	assert.Error(t, err)
}
