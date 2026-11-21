package oidc

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/oidc"
)

// MCP clients (AI clients such as claude.ai connectors) register
// themselves (RFC 7591, dynamic client registration) and then act for a
// person who logs in and agrees on Etherpad's consent page. They are
// public clients: PKCE (S256) instead of a secret. Their tokens work at
// /mcp only, with the scopes and pads the person allowed, never beyond the
// person's own rights.

const (
	registerPath = "/oauth2/register"
	// mcpClientKeyPrefix keys a registered client in the OIDC key/value
	// store, which every instance of a cluster shares.
	mcpClientKeyPrefix = "mcp-client:"

	maxRegisterBody    = 16 << 10
	maxRedirectURIs    = 10
	maxClientNameRunes = 100

	// unusedClientLifetime: a registration nobody connected with is
	// removed after it (on its next lookup).
	unusedClientLifetime = 24 * time.Hour
	// Registrations per address and per instance in registerWindow.
	registerPerIP       = 10
	registerPerInstance = 200
	registerWindow      = time.Hour

	// ClientIPHeader carries the client's address, as fiber sees it behind
	// trusted proxies, to the registration (overwritten, never taken from
	// the request).
	ClientIPHeader = "X-Etherpad-Oidc-Client-Ip"
)

// MCPScopes are the scopes an MCP client may ask for and a person may
// allow: what the MCP server's tools need.
var MCPScopes = []string{accounts.ScopePadRead, accounts.ScopePadWrite, accounts.ScopePadCreate, accounts.ScopeChatWrite}

// mcpClientRecord is a registered MCP client as stored.
type mcpClientRecord struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	RedirectURIs []string  `json:"redirectUris"`
	CreatedAt    time.Time `json:"createdAt"`
	// UsedAt: when a person first connected it; zero for a registration
	// nobody used, which is removed after unusedClientLifetime.
	UsedAt time.Time `json:"usedAt,omitempty"`
}

// mcpClients caches registered clients and counts registrations.
type mcpClients struct {
	m sync.Map // id -> *Client

	mu      sync.Mutex
	recent  []time.Time            // registrations of this instance
	perAddr map[string][]time.Time // by client address
}

// allowRegistration counts a registration from addr, unless the address
// or the instance registered too many in registerWindow.
func (c *mcpClients) allowRegistration(addr string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	keep := func(ts []time.Time) []time.Time {
		return slices.DeleteFunc(ts, func(t time.Time) bool { return now.Sub(t) >= registerWindow })
	}
	if c.perAddr == nil {
		c.perAddr = map[string][]time.Time{}
	}
	c.recent = keep(c.recent)
	mine := keep(c.perAddr[addr])
	if len(c.recent) >= registerPerInstance || len(mine) >= registerPerIP {
		c.perAddr[addr] = mine
		return false
	}
	c.recent = append(c.recent, now)
	c.perAddr[addr] = append(mine, now)
	// Forget addresses that have nothing in the window.
	for a, ts := range c.perAddr {
		if len(keep(ts)) == 0 {
			delete(c.perAddr, a)
		}
	}
	return true
}

func newMCPClient(r mcpClientRecord) *Client {
	c := &Client{
		id:           r.ID,
		redirectURIs: append([]string(nil), r.RedirectURIs...),
		grantTypes:   []oidc.GrantType{oidc.GrantTypeCode, oidc.GrantTypeRefreshToken},
		mcp:          true,
		name:         r.Name,
		record:       r,
		settings:     settings.SSOClient{ClientId: r.ID, DisplayName: r.Name},
	}
	for _, uri := range c.redirectURIs {
		if strings.HasPrefix(uri, "http://") {
			c.devMode = true
		}
	}
	return c
}

// mcpClient returns a registered MCP client.
func (s *Storage) mcpClient(id string) (*Client, bool) {
	if !strings.HasPrefix(id, "mcp-") {
		return nil, false
	}
	if c, ok := s.mcp.m.Load(id); ok {
		client := c.(*Client)
		if !s.unusedTooLong(client.record) {
			return client, true
		}
		s.mcp.m.Delete(id)
	}
	raw, err := s.db.GetOIDCStorageValue(mcpClientKeyPrefix + id)
	if err != nil || raw == nil {
		return nil, false
	}
	var r mcpClientRecord
	if json.Unmarshal([]byte(*raw), &r) != nil || r.ID != id {
		return nil, false
	}
	if s.unusedTooLong(r) {
		// Registered, never connected: gone, so registrations do not pile up.
		_ = s.db.DeleteOIDCStorageValue(mcpClientKeyPrefix + id)
		return nil, false
	}
	c := newMCPClient(r)
	s.mcp.m.Store(id, c)
	return c, true
}

func (s *Storage) unusedTooLong(r mcpClientRecord) bool {
	return r.UsedAt.IsZero() && s.now().Sub(r.CreatedAt) > unusedClientLifetime
}

// markMCPClientUsed records that a person connected the client, which then
// stays.
func (s *Storage) markMCPClientUsed(id string) {
	c, ok := s.mcpClient(id)
	if !ok || !c.record.UsedAt.IsZero() {
		return
	}
	r := c.record
	r.UsedAt = s.now()
	data, err := json.Marshal(r)
	if err != nil {
		return
	}
	if s.db.SetOIDCStorageValue(mcpClientKeyPrefix+id, string(data)) == nil {
		s.mcp.m.Store(id, newMCPClient(r))
	}
}

// client returns a client from the settings or a registered MCP client.
func (s *Storage) client(id string) (*Client, bool) {
	if c, ok := s.clients[id]; ok {
		return c, true
	}
	return s.mcpClient(id)
}

// redirectURIAllowed: https, or http to this machine (native clients,
// RFC 8252); no fragments, no credentials.
func redirectURIAllowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Fragment != "" || u.User != nil || u.Host == "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		h := u.Hostname()
		return h == "localhost" || h == "127.0.0.1" || h == "::1"
	}
	return false
}

type registrationRequest struct {
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	Scope                   string   `json:"scope"`
}

type registrationError struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

func registrationFailed(rw http.ResponseWriter, code, description string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Cache-Control", "no-store")
	rw.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(rw).Encode(registrationError{Error: code, Description: description})
}

// validateRegistration checks a registration and returns the client to
// store.
func validateRegistration(req registrationRequest) (mcpClientRecord, string, string) {
	var r mcpClientRecord
	if len(req.RedirectURIs) == 0 || len(req.RedirectURIs) > maxRedirectURIs {
		return r, "invalid_redirect_uri", fmt.Sprintf("1 to %d redirect_uris are required", maxRedirectURIs)
	}
	for _, uri := range req.RedirectURIs {
		if !redirectURIAllowed(uri) {
			return r, "invalid_redirect_uri", "redirect_uris must be https, or http to localhost: " + uri
		}
	}
	// Public clients only: they prove themselves with PKCE. A client that
	// asks for a secret is told so in the answer (RFC 7591 3.2.1).
	for _, g := range req.GrantTypes {
		if g != string(oidc.GrantTypeCode) && g != string(oidc.GrantTypeRefreshToken) {
			return r, "invalid_client_metadata", "only the authorization_code and refresh_token grants are supported"
		}
	}
	for _, t := range req.ResponseTypes {
		if t != string(oidc.ResponseTypeCode) {
			return r, "invalid_client_metadata", "only the code response type is supported"
		}
	}
	name := strings.TrimSpace(req.ClientName)
	if utf8.RuneCountInString(name) > maxClientNameRunes {
		return r, "invalid_client_metadata", fmt.Sprintf("client_name has more than %d characters", maxClientNameRunes)
	}
	for _, c := range name {
		if c < 0x20 || c == 0x7f {
			return r, "invalid_client_metadata", "client_name has control characters"
		}
	}
	return mcpClientRecord{ID: "mcp-" + uuid.NewString(), Name: name, RedirectURIs: slices.Clone(req.RedirectURIs)}, "", ""
}

// RegisterHandler answers dynamic client registration (RFC 7591) for MCP
// clients.
func (a *Authenticator) RegisterHandler(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		rw.Header().Set("Allow", http.MethodPost)
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.mcpOAuthEnabled() {
		http.Error(rw, "dynamic client registration is off", http.StatusForbidden)
		return
	}
	addr := req.Header.Get(ClientIPHeader)
	if addr == "" {
		addr = req.RemoteAddr
		if host, _, err := net.SplitHostPort(addr); err == nil {
			addr = host
		}
	}
	if !a.storage.mcp.allowRegistration(addr, a.storage.now()) {
		rw.Header().Set("Retry-After", "3600")
		http.Error(rw, "too many registrations, try again later", http.StatusTooManyRequests)
		return
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, maxRegisterBody+1))
	if err != nil || len(body) > maxRegisterBody {
		registrationFailed(rw, "invalid_client_metadata", "the request is too large")
		return
	}
	var r registrationRequest
	if err := json.Unmarshal(body, &r); err != nil {
		registrationFailed(rw, "invalid_client_metadata", "the request is no JSON object")
		return
	}
	rec, code, description := validateRegistration(r)
	if code != "" {
		registrationFailed(rw, code, description)
		return
	}
	rec.CreatedAt = a.storage.now()
	data, err := json.Marshal(rec)
	if err != nil {
		http.Error(rw, "registration failed", http.StatusInternalServerError)
		return
	}
	if _, err := a.storage.db.AddOIDCStorageValue(mcpClientKeyPrefix+rec.ID, string(data)); err != nil {
		http.Error(rw, "registration failed", http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Cache-Control", "no-store")
	rw.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(rw).Encode(map[string]any{
		"client_id":                  rec.ID,
		"client_id_issued_at":        rec.CreatedAt.Unix(),
		"client_name":                rec.Name,
		"redirect_uris":              rec.RedirectURIs,
		"grant_types":                []string{string(oidc.GrantTypeCode), string(oidc.GrantTypeRefreshToken)},
		"response_types":             []string{string(oidc.ResponseTypeCode)},
		"token_endpoint_auth_method": "none",
		"scope":                      strings.Join(MCPScopes, " "),
	})
}

// mcpOAuthEnabled: MCP clients may register and connect unless the
// settings turn it off (mcp.oauth: false).
func (a *Authenticator) mcpOAuthEnabled() bool {
	return a.retrievedSettings.MCP.OAuthEnabled()
}
