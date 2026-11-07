package oidc

import (
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"golang.org/x/crypto/bcrypt"
)

const (
	// loginPath is where the provider sends the browser to authenticate an
	// auth request; see LoginHandler.
	loginPath = "/oauth2/login"

	idTokenLifetime      = time.Hour
	accessTokenLifetime  = 30 * time.Minute
	refreshTokenLifetime = 30 * 24 * time.Hour
	authRequestLifetime  = 10 * time.Minute

	// legacyOfflineScope is what the admin UI used to request with fosite.
	// It is treated as offline_access.
	legacyOfflineScope = "offline"
)

// Client is an SSO client from the settings.
type Client struct {
	id           string
	secretHash   []byte
	redirectURIs []string
	grantTypes   []oidc.GrantType
	confidential bool
	devMode      bool
	settings     settings.SSOClient
}

// newClient maps an SSO client from the settings. Clients that may use the
// client_credentials grant are confidential and authenticate with their
// secret; all others (like the admin UI) are public clients that must use
// PKCE, as before with fosite.
func newClient(sso settings.SSOClient) (*Client, error) {
	c := &Client{
		id:           sso.ClientId,
		redirectURIs: append([]string(nil), sso.RedirectUris...),
		confidential: slices.Contains(sso.GrantTypes, string(oidc.GrantTypeClientCredentials)),
		settings:     sso,
	}
	for _, g := range sso.GrantTypes {
		c.grantTypes = append(c.grantTypes, oidc.GrantType(g))
	}
	// The admin UI renews its session with the refresh token grant.
	if slices.Contains(c.grantTypes, oidc.GrantTypeCode) && !slices.Contains(c.grantTypes, oidc.GrantTypeRefreshToken) {
		c.grantTypes = append(c.grantTypes, oidc.GrantTypeRefreshToken)
	}
	if sso.ClientSecret != nil && *sso.ClientSecret != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(*sso.ClientSecret), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		c.secretHash = hash
	}
	// The redirect URIs are matched exactly, so an http:// URI the admin
	// configured (local or LAN setups) is allowed.
	for _, uri := range c.redirectURIs {
		if strings.HasPrefix(uri, "http://") {
			c.devMode = true
		}
	}
	return c, nil
}

func (c *Client) checkSecret(secret string) bool {
	return c.secretHash != nil && bcrypt.CompareHashAndPassword(c.secretHash, []byte(secret)) == nil
}

func (c *Client) GetID() string                    { return c.id }
func (c *Client) RedirectURIs() []string           { return c.redirectURIs }
func (c *Client) PostLogoutRedirectURIs() []string { return c.redirectURIs }

func (c *Client) ApplicationType() op.ApplicationType {
	if c.confidential {
		return op.ApplicationTypeWeb
	}
	return op.ApplicationTypeUserAgent
}

func (c *Client) AuthMethod() oidc.AuthMethod {
	if c.confidential {
		return oidc.AuthMethodBasic
	}
	return oidc.AuthMethodNone
}

func (c *Client) ResponseTypes() []oidc.ResponseType {
	return []oidc.ResponseType{oidc.ResponseTypeCode}
}

func (c *Client) GrantTypes() []oidc.GrantType { return c.grantTypes }

func (c *Client) LoginURL(authRequestID string) string {
	return loginPath + "?authRequestID=" + url.QueryEscape(authRequestID)
}

func (c *Client) AccessTokenType() op.AccessTokenType { return op.AccessTokenTypeBearer }
func (c *Client) IDTokenLifetime() time.Duration      { return idTokenLifetime }
func (c *Client) DevMode() bool                       { return c.devMode }

func (c *Client) RestrictAdditionalIdTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string { return scopes }
}

func (c *Client) RestrictAdditionalAccessTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string { return scopes }
}

// IsScopeAllowed is asked for scopes beyond the standard OIDC ones.
func (c *Client) IsScopeAllowed(scope string) bool { return scope == legacyOfflineScope }

// IDTokenUserinfoClaimsAssertion puts the userinfo claims into the ID token,
// which the admin UI uses as its bearer token.
func (c *Client) IDTokenUserinfoClaimsAssertion() bool { return true }
func (c *Client) ClockSkew() time.Duration             { return 0 }

var _ op.Client = (*Client)(nil)
