package oidc

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/url"
	"slices"
	"strings"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/settings"
)

// PadCallbackPath is where the pads' login comes back to with a code.
const PadCallbackPath = "/auth/callback"

// clientTypeUser is the SSO client the pads log in with.
const clientTypeUser = "user"

// ErrLoginFailed is any failure to redeem a pad login code; the reason is
// not told to the browser.
var ErrLoginFailed = errors.New("the login could not be completed")

// userClientOf returns the SSO client of type "user", or nil.
func userClientOf(s *settings.Settings) *settings.SSOClient {
	if s.SSO == nil {
		return nil
	}
	for i := range s.SSO.Clients {
		if s.SSO.Clients[i].Type == clientTypeUser {
			return &s.SSO.Clients[i]
		}
	}
	return nil
}

// PadLoginAvailable reports whether the pads can log in: a client of type
// "user" that may use the authorization code grant is configured.
func (a *Authenticator) PadLoginAvailable() bool {
	_, ok := a.padClient()
	return ok
}

func (a *Authenticator) padClient() (*Client, bool) {
	sso := userClientOf(a.retrievedSettings)
	if sso == nil {
		return nil, false
	}
	c, ok := a.storage.clients[sso.ClientId]
	if !ok || !slices.Contains(c.grantTypes, "authorization_code") {
		return nil, false
	}
	return c, true
}

// PadCallbackURL is the redirect URI of the pads' login. It is allowed for
// the user client without being listed in its redirect_uris: it is this
// server's own page, on the issuer's origin.
func (a *Authenticator) PadCallbackURL() string {
	return originOf(a.issuer) + PadCallbackPath
}

// allowPadCallback adds the pads' callback to the user client's redirect
// URIs.
func allowPadCallback(clients map[string]*Client, s *settings.Settings, issuer string) {
	sso := userClientOf(s)
	if sso == nil {
		return
	}
	c, ok := clients[sso.ClientId]
	if !ok {
		return
	}
	cb := originOf(issuer) + PadCallbackPath
	if !slices.Contains(c.redirectURIs, cb) {
		c.redirectURIs = append(c.redirectURIs, cb)
	}
	if strings.HasPrefix(cb, "http://") {
		c.devMode = true
	}
}

// PKCEChallenge is the S256 code challenge of a verifier.
func PKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// PadAuthorizeURL is the provider's authorization URL for a pad login with
// the given state and PKCE verifier.
func (a *Authenticator) PadAuthorizeURL(state, verifier string) (string, error) {
	c, ok := a.padClient()
	if !ok {
		return "", ErrLoginFailed
	}
	q := url.Values{}
	q.Set("client_id", c.id)
	q.Set("redirect_uri", a.PadCallbackURL())
	q.Set("response_type", "code")
	q.Set("scope", "openid profile")
	q.Set("state", state)
	q.Set("code_challenge", PKCEChallenge(verifier))
	q.Set("code_challenge_method", "S256")
	return originOf(a.issuer) + authPath + "?" + q.Encode(), nil
}

// RedeemPadCode exchanges the code of a pad login for the account that
// logged in, without the token endpoint: the server is its own client, so
// it checks the code where it is stored. The code must belong to the user
// client and the pads' callback, match the PKCE verifier, and belong to a
// finished login of an active person. It works only once.
func (a *Authenticator) RedeemPadCode(code, verifier string) (*db.Account, error) {
	c, ok := a.padClient()
	if !ok || code == "" || verifier == "" {
		return nil, ErrLoginFailed
	}
	row, err := a.storage.db.GetAuthCode(hashToken(code))
	if err != nil || row == nil {
		return nil, ErrLoginFailed
	}
	ar, err := a.storage.getAuthRequest(row.RequestID)
	if err != nil {
		return nil, ErrLoginFailed
	}
	// Used up whatever follows: a code is tried once.
	_ = a.storage.db.InvalidateAuthCode(hashToken(code))
	_ = a.storage.db.DeleteOIDCSession(ar.ID)
	if ar.ClientID != c.id || ar.RedirectURI != a.PadCallbackURL() || !ar.IsDone || ar.UserID == "" {
		return nil, ErrLoginFailed
	}
	if ar.CodeChallenge == nil || ar.CodeChallenge.Method != "S256" ||
		subtle.ConstantTimeCompare([]byte(ar.CodeChallenge.Challenge), []byte(PKCEChallenge(verifier))) != 1 {
		return nil, ErrLoginFailed
	}
	person := a.storage.activePerson(ar.UserID)
	if person == nil || person.MustChangePassword {
		return nil, ErrLoginFailed
	}
	return person, nil
}

// Accounts returns the account manager the provider logs in with.
func (a *Authenticator) Accounts() *accounts.Manager { return a.storage.accounts }
