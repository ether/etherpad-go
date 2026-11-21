package oidc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
)

// asMetadataPath is OAuth 2.0 authorization server metadata (RFC 8414),
// which MCP clients read; it is the discovery document with the
// registration endpoint.
const asMetadataPath = "/.well-known/oauth-authorization-server"

// asMetadataPaths: at the root, and with the issuer's path appended
// (RFC 8414 section 3).
func (a *Authenticator) asMetadataPaths() []string {
	paths := []string{asMetadataPath}
	if u, err := url.Parse(a.issuer); err == nil && strings.Trim(u.Path, "/") != "" {
		paths = append(paths, asMetadataPath+strings.TrimRight(u.Path, "/"))
	}
	return paths
}

// serveMetadata serves the provider's discovery document, with what MCP
// clients need added: the registration endpoint, "none" as a token
// endpoint auth method (public clients) and S256.
func (a *Authenticator) serveMetadata(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		rw.Header().Set("Allow", "GET, HEAD")
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	inner := req.Clone(req.Context())
	inner.Method = http.MethodGet
	inner.URL.Path = discoveryPath
	rec := httptest.NewRecorder()
	a.provider.ServeHTTP(rec, inner)
	if rec.Code != http.StatusOK {
		rw.WriteHeader(rec.Code)
		_, _ = rw.Write(rec.Body.Bytes())
		return
	}
	doc := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		http.Error(rw, "discovery failed", http.StatusInternalServerError)
		return
	}
	if a.mcpOAuthEnabled() {
		doc["registration_endpoint"] = originOf(a.issuer) + registerPath
		doc["token_endpoint_auth_methods_supported"] = addString(doc["token_endpoint_auth_methods_supported"], "none")
		doc["code_challenge_methods_supported"] = addString(doc["code_challenge_methods_supported"], "S256")
		doc["scopes_supported"] = addString(doc["scopes_supported"], append(slices.Clone(MCPScopes), "offline_access")...)
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Header().Set("Cache-Control", "public, max-age=300")
	if req.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(rw).Encode(doc)
}

func addString(v any, add ...string) []string {
	var out []string
	switch list := v.(type) {
	case []any:
		for _, x := range list {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, list...)
	}
	for _, s := range add {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// checkMCPAuthRequest holds an MCP client's auth request to PKCE with
// S256, and asks for the MCP scopes when it names none. It answers the
// request itself (false) when the request is refused.
func (a *Authenticator) checkMCPAuthRequest(rw http.ResponseWriter, req *http.Request) bool {
	q := req.URL.Query()
	clientID := q.Get("client_id")
	if req.Method == http.MethodPost {
		if err := req.ParseForm(); err == nil && req.PostForm.Get("client_id") != "" {
			clientID = req.PostForm.Get("client_id")
			q = req.PostForm
		}
	}
	c, ok := a.storage.client(clientID)
	if !ok || !c.mcp {
		return true
	}
	if !a.mcpOAuthEnabled() {
		http.Error(rw, "MCP clients cannot connect to this server", http.StatusForbidden)
		return false
	}
	if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		http.Error(rw, "PKCE with S256 is required", http.StatusBadRequest)
		return false
	}
	if req.Method == http.MethodGet && strings.TrimSpace(q.Get("scope")) == "" {
		q.Set("scope", strings.Join(MCPScopes, " "))
		req.URL.RawQuery = q.Encode()
	}
	return true
}

// MCPOAuthIssuer is the issuer MCP clients get their tokens from, and
// whether they may.
func (a *Authenticator) MCPOAuthIssuer() (string, bool) {
	return a.issuer, a.mcpOAuthEnabled()
}
