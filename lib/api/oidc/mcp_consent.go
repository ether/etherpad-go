package oidc

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/ether/etherpad-go/assets/login"
	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/api/constants"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// A person's consent for an MCP client is a grant: the client, the scopes
// and pads allowed. Every token of the client carries its grant's id; the
// tokens work only while the grant exists, so ending it in "My tokens"
// stops the client at once, its refresh tokens too. The grants of an
// account are one record in the OIDC key/value store, changed under the
// accounts' write lock (shared by a cluster).

const mcpGrantsKeyPrefix = "mcp-grants:"

// MCPGrant is a person's consent for an MCP client.
type MCPGrant struct {
	ID         string    `json:"id"`
	ClientID   string    `json:"clientId"`
	ClientName string    `json:"clientName"`
	Scopes     []string  `json:"scopes"`
	PadPattern string    `json:"padPattern"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (s *Storage) mcpGrants(accountID string) ([]MCPGrant, error) {
	raw, err := s.db.GetOIDCStorageValue(mcpGrantsKeyPrefix + accountID)
	if err != nil || raw == nil {
		return nil, err
	}
	var grants []MCPGrant
	if err := json.Unmarshal([]byte(*raw), &grants); err != nil {
		return nil, err
	}
	return grants, nil
}

func (s *Storage) putMCPGrants(accountID string, grants []MCPGrant) error {
	if len(grants) == 0 {
		return s.db.DeleteOIDCStorageValue(mcpGrantsKeyPrefix + accountID)
	}
	data, err := json.Marshal(grants)
	if err != nil {
		return err
	}
	return s.db.SetOIDCStorageValue(mcpGrantsKeyPrefix+accountID, string(data))
}

// addMCPGrant records a consent; a new consent for the same client
// replaces the old one (and so ends its tokens).
func (s *Storage) addMCPGrant(accountID string, g MCPGrant) error {
	return s.accounts.Locked(func() error {
		grants, err := s.mcpGrants(accountID)
		if err != nil {
			return err
		}
		grants = slices.DeleteFunc(grants, func(x MCPGrant) bool { return x.ClientID == g.ClientID })
		return s.putMCPGrants(accountID, append(grants, g))
	})
}

// MCPGrants are the MCP clients a person allowed, newest first.
func (a *Authenticator) MCPGrants(accountID string) ([]MCPGrant, error) {
	grants, err := a.storage.mcpGrants(accountID)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(grants, func(x, y MCPGrant) int { return y.CreatedAt.Compare(x.CreatedAt) })
	return grants, nil
}

// RevokeMCPGrant ends a person's consent: the client's tokens stop at once.
func (a *Authenticator) RevokeMCPGrant(accountID, grantID string) error {
	return a.storage.accounts.Locked(func() error {
		grants, err := a.storage.mcpGrants(accountID)
		if err != nil {
			return err
		}
		n := len(grants)
		grants = slices.DeleteFunc(grants, func(x MCPGrant) bool { return x.ID == grantID })
		if len(grants) == n {
			return ErrGrantNotFound
		}
		return a.storage.putMCPGrants(accountID, grants)
	})
}

// mcpGrantValid reports whether the grant of an MCP token still exists.
func (s *Storage) mcpGrantValid(accountID, grantID string) bool {
	if grantID == "" {
		return false
	}
	grants, err := s.mcpGrants(accountID)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(grants, func(g MCPGrant) bool { return g.ID == grantID })
}

// ErrGrantNotFound: the person has no such consent.
var ErrGrantNotFound = errors.New("no such app")

// --- the consent page ------------------------------------------------------

// finish completes a login: at once for the admin UI and the pads, after
// the person's consent for an MCP client.
func (a *Authenticator) finish(rw http.ResponseWriter, req *http.Request, ar *authRequest, accountID string) {
	if c, ok := a.storage.client(ar.ClientID); ok && c.mcp {
		// Connecting an app gives it a token in the person's name: only for
		// who may have tokens (tokens:own), who also sees and ends the
		// access in "My tokens".
		if g, err := a.storage.accounts.GrantsFor(accountID); err != nil || !g.HasGlobal(accounts.ScopeTokensOwn) {
			_ = a.storage.DeleteAuthRequest(req.Context(), ar.ID)
			op.AuthRequestError(rw, req, ar, oidc.ErrAccessDenied().WithDescription("this account may not connect apps"), a.provider)
			return
		}
		token := utils.RandomString(32)
		if err := a.storage.awaitConsent(ar.ID, accountID, token); err != nil {
			http.Error(rw, "The login request is invalid or has expired. Please start the login again.", http.StatusBadRequest)
			return
		}
		ar.ConsentFor = accountID
		a.renderConsent(rw, req, ar, c, token, nil, nil, ar.PadPattern)
		return
	}
	a.complete(rw, req, ar, accountID)
}

func (s *Storage) awaitConsent(id, accountID, token string) error {
	ar, err := s.getAuthRequest(id)
	if err != nil {
		return err
	}
	ar.ConsentFor = accountID
	ar.ConsentToken = hashToken(token)
	return s.putAuthRequest(ar)
}

// grantConsent finishes an auth request with the scopes and pads the person
// allowed, and records the grant.
func (s *Storage) grantConsent(id, accountID string, scopes []string, pads string, c *Client) error {
	ar, err := s.getAuthRequest(id)
	if err != nil {
		return err
	}
	if ar.ConsentFor != accountID || ar.IsDone {
		return errNotFound
	}
	g := MCPGrant{ID: utils.RandomString(24), ClientID: c.id, ClientName: c.name, Scopes: scopes, PadPattern: pads, CreatedAt: s.now()}
	if err := s.addMCPGrant(accountID, g); err != nil {
		return err
	}
	// offline_access: the client renews its tokens without the person.
	ar.Scopes = append(slices.Clone(scopes), oidc.ScopeOfflineAccess, mcpGrantScope+g.ID)
	ar.PadPattern = pads
	ar.UserID = accountID
	ar.ConsentFor, ar.ConsentToken = "", ""
	ar.IsDone = true
	ar.AuthTime = s.now()
	return s.putAuthRequest(ar)
}

// mcpGrantScope carries the grant id in a token's scopes (internal: never
// asked for by a client, IsScopeAllowed does not allow it).
const mcpGrantScope = "etherpad-grant:"

func grantOf(scopes []string) string {
	for _, s := range scopes {
		if id, ok := strings.CutPrefix(s, mcpGrantScope); ok {
			return id
		}
	}
	return ""
}

func consentAllowed(ar *authRequest, token string) bool {
	return !ar.IsDone && ar.ConsentFor != "" && ar.ConsentToken != "" && token != "" &&
		hashToken(token) == ar.ConsentToken
}

// consentScopes are the scopes the page offers: the MCP scopes the person
// has somewhere; checked the ones the client asked for (all, if it asked
// for none of them).
func (a *Authenticator) consentScopes(accountID string, requested []string, checked []string) []login.ConsentScope {
	g, err := a.storage.accounts.GrantsFor(accountID)
	if err != nil {
		return nil
	}
	asked := slices.ContainsFunc(requested, func(s string) bool { return slices.Contains(MCPScopes, s) })
	desc := map[string]string{}
	for _, s := range accounts.Scopes() {
		desc[s.Name] = s.Description
	}
	var out []login.ConsentScope
	for _, s := range MCPScopes {
		if !g.HasAnywhere(s) {
			continue
		}
		on := !asked || slices.Contains(requested, s)
		if checked != nil {
			on = slices.Contains(checked, s)
		}
		out = append(out, login.ConsentScope{Name: s, Description: desc[s], Checked: on})
	}
	return out
}

func (a *Authenticator) renderConsent(rw http.ResponseWriter, req *http.Request, ar *authRequest, c *Client, token string,
	checked []string, errorMessage *string, pads string) {
	person := ar.ConsentFor
	if acc := a.storage.activePerson(ar.ConsentFor); acc != nil {
		person = acc.Username
		if acc.DisplayName != "" {
			person = acc.DisplayName
		}
	}
	host := ar.RedirectURI
	if u, err := url.Parse(ar.RedirectURI); err == nil && u.Host != "" {
		host = u.Host
	}
	params := url.Values{"authRequestID": {ar.ID}, "consent_token": {token}}
	rw.Header().Set("Content-Type", constants.ContentTypeHTML)
	rw.Header().Set("Cache-Control", "no-store")
	// The page must not be framed: a click on "Erlauben" is the consent.
	rw.Header().Set("X-Frame-Options", "DENY")
	rw.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
	scopes := a.consentScopes(ar.ConsentFor, ar.Scopes, checked)
	if err := login.Consent(c.name, host, person, scopes, pads, params, errorMessage).Render(req.Context(), rw); err != nil {
		http.Error(rw, "rendering the consent page failed", http.StatusInternalServerError)
	}
}

// consent handles the consent form.
func (a *Authenticator) consent(rw http.ResponseWriter, req *http.Request, ar *authRequest) {
	token := req.PostForm.Get("consent_token")
	c, ok := a.storage.client(ar.ClientID)
	if !ok || !c.mcp || !consentAllowed(ar, token) || a.storage.activePerson(ar.ConsentFor) == nil {
		http.Error(rw, "The login request is invalid or has expired. Please start the login again.", http.StatusBadRequest)
		return
	}
	if req.PostForm.Get("decision") != "allow" {
		_ = a.storage.DeleteAuthRequest(req.Context(), ar.ID)
		op.AuthRequestError(rw, req, ar, oidc.ErrAccessDenied().WithDescription("the person did not allow the access"), a.provider)
		return
	}
	offered := a.consentScopes(ar.ConsentFor, ar.Scopes, nil)
	var scopes []string
	for _, s := range req.PostForm["scope"] {
		if slices.ContainsFunc(offered, func(o login.ConsentScope) bool { return o.Name == s }) && !slices.Contains(scopes, s) {
			scopes = append(scopes, s)
		}
	}
	pads := strings.TrimSpace(req.PostForm.Get("pads"))
	fail := func(msg string) {
		a.renderConsent(rw, req, ar, c, token, scopes, &msg, pads)
	}
	if len(scopes) == 0 {
		fail("Wähle mindestens ein Recht.")
		return
	}
	if pads != "" {
		if err := accounts.ValidatePattern(pads); err != nil {
			fail("Das Pad-Muster ist ungültig.")
			return
		}
	}
	if err := a.storage.grantConsent(ar.ID, ar.ConsentFor, scopes, pads, c); err != nil {
		http.Error(rw, "The login request is invalid or has expired. Please start the login again.", http.StatusBadRequest)
		return
	}
	http.Redirect(rw, req, callbackPath+"?id="+url.QueryEscape(ar.ID), http.StatusFound)
}

// --- tokens at /mcp ----------------------------------------------------------

// MCPAccess is what an OAuth access token allows at /mcp.
type MCPAccess struct {
	Account    *db.Account
	ClientID   string
	Scopes     []string
	PadPattern string
	ExpiresAt  time.Time
}

// ErrNotMCPToken: the token is no access token of an MCP client.
var ErrNotMCPToken = errors.New("not an MCP access token")

// MCPAccessToken checks an access token for /mcp: issued to an MCP client,
// unexpired, for a person who is still active, under a grant that still
// exists. The rights are the person's of the moment, cut down to it.
func (a *Authenticator) MCPAccessToken(token string) (*MCPAccess, error) {
	if token == "" || a.provider == nil || !a.mcpOAuthEnabled() {
		return nil, ErrNotMCPToken
	}
	plain, err := a.provider.Crypto().Decrypt(token)
	if err != nil {
		return nil, ErrNotMCPToken
	}
	id, subject, found := strings.Cut(plain, ":")
	if !found {
		return nil, ErrNotMCPToken
	}
	t, err := a.storage.getAccessToken(id)
	if err != nil || t.Subject != subject {
		return nil, ErrNotMCPToken
	}
	if c, ok := a.storage.client(t.ClientID); !ok || !c.mcp {
		return nil, ErrNotMCPToken
	}
	acc := a.storage.activePerson(t.Subject)
	if acc == nil || !a.storage.mcpGrantValid(acc.ID, grantOf(t.Scopes)) {
		return nil, ErrNotMCPToken
	}
	var scopes []string
	for _, s := range t.Scopes {
		if slices.Contains(MCPScopes, s) {
			scopes = append(scopes, s)
		}
	}
	return &MCPAccess{Account: acc, ClientID: t.ClientID, Scopes: scopes, PadPattern: t.PadPattern, ExpiresAt: t.ExpiresAt}, nil
}
