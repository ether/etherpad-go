// Package mcp serves the Model Context Protocol at /mcp: AI clients (Claude,
// …) read and edit pads and use their chat with a token
// (Authorization: Bearer ep_…). What a call may do is the token's: its
// account's rights of the moment, cut down to the token's scopes and pad
// pattern, checked per pad like everywhere else.
//
// The server keeps nothing between requests (stateless Streamable HTTP), so
// every instance of a cluster answers every request and no sticky sessions
// are needed.
package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/api/oidc"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/tokens"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/modelcontextprotocol/go-sdk/auth"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Path is where the MCP server answers.
const Path = "/mcp"

// clientIPHeader carries the client's address, as fiber sees it behind
// trusted proxies, to the token check; it is always overwritten, never
// taken from the request.
const clientIPHeader = "X-Etherpad-Mcp-Client-Ip"

// useKey is where the token's use sits in TokenInfo.Extra.
const useKey = "use"

// maxBody bounds a request: a whole pad's text and a bit.
const maxBody = 2 << 20

// Server is the MCP server of one instance.
type Server struct {
	store    *lib.InitStore
	tokens   *tokens.Manager
	accounts *accounts.Manager
	mcp      *sdk.Server
	oauth    OAuth
}

// OAuth checks the access tokens of MCP clients that connected with a
// login and the person's consent (lib/api/oidc).
type OAuth interface {
	MCPAccessToken(token string) (*oidc.MCPAccess, error)
	MCPOAuthIssuer() (string, bool)
}

// WithOAuth lets MCP clients connect with OAuth, besides tokens.
func (s *Server) WithOAuth(o OAuth) *Server {
	s.oauth = o
	return s
}

// New makes the MCP server; Register serves it.
func New(store *lib.InitStore, keys *tokens.Manager) *Server {
	s := &Server{store: store, tokens: keys, accounts: accounts.For(store.Store)}
	s.mcp = sdk.NewServer(&sdk.Implementation{Name: "etherpad", Title: "Etherpad", Version: store.RetrievedSettings.GitVersion},
		&sdk.ServerOptions{Instructions: instructions})
	s.addTools()
	return s
}

const instructions = `Etherpad pads are collaborative text documents; people may be editing the same pad while you do.
Use list_pads to find pads, read_pad before you change one, and prefer append_text and replace_text over set_text: they keep
the formatting and authorship of the rest of the pad. Every change is a new revision; nothing is lost from the history.
What you may read or change is the token's: a pad you get "forbidden" for is not yours to use.`

// Register serves the MCP server at Path on app.
func (s *Server) Register(app fiber.Router) {
	handler := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return s.mcp }, &sdk.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		// The bearer token is the protection: a page that rebinds a name
		// to this host has no token. A reverse proxy on the same host sends
		// the public Host, which the check would refuse.
		DisableLocalhostProtection: true,
		MaxRequestBodyBytes:        maxBody,
	})
	protected := auth.RequireBearerToken(s.verify, &auth.RequireBearerTokenOptions{AllowMissingExpiration: true})(handler)
	h := adaptor.HTTPHandler(protected)
	app.All(Path, func(c fiber.Ctx) error {
		c.Request().Header.Set(clientIPHeader, c.IP())
		c.Set(fiber.HeaderCacheControl, "no-store")
		if err := h(c); err != nil {
			return err
		}
		// RFC 6750: a 401 names the scheme it wants; with OAuth, where the
		// client learns how to get a token (RFC 9728).
		if c.Response().StatusCode() == fiber.StatusUnauthorized {
			challenge := `Bearer realm="etherpad"`
			if _, ok := s.resourceMetadata(); ok {
				challenge += `, resource_metadata="` + s.publicBase() + ResourceMetadataPath + Path + `"`
			}
			c.Set(fiber.HeaderWWWAuthenticate, challenge)
		}
		return nil
	})
	// Protected resource metadata (RFC 9728), at the root and for /mcp.
	for _, p := range []string{ResourceMetadataPath, ResourceMetadataPath + Path} {
		app.Get(p, s.serveResourceMetadata)
	}
}

// ResourceMetadataPath is OAuth 2.0 protected resource metadata (RFC 9728).
const ResourceMetadataPath = "/.well-known/oauth-protected-resource"

// publicBase is the address Etherpad is reached at: publicURL, else the
// issuer's origin.
func (s *Server) publicBase() string {
	if u := strings.TrimRight(s.store.RetrievedSettings.PublicURL, "/"); u != "" {
		return u
	}
	if s.oauth != nil {
		if issuer, ok := s.oauth.MCPOAuthIssuer(); ok {
			if u, err := url.Parse(issuer); err == nil && u.Host != "" {
				return u.Scheme + "://" + u.Host
			}
		}
	}
	return ""
}

func (s *Server) resourceMetadata() (map[string]any, bool) {
	if s.oauth == nil {
		return nil, false
	}
	issuer, ok := s.oauth.MCPOAuthIssuer()
	base := s.publicBase()
	if !ok || base == "" {
		return nil, false
	}
	return map[string]any{
		"resource":                 base + Path,
		"authorization_servers":    []string{issuer},
		"scopes_supported":         oidc.MCPScopes,
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "Etherpad",
	}, true
}

func (s *Server) serveResourceMetadata(c fiber.Ctx) error {
	doc, ok := s.resourceMetadata()
	if !ok {
		return c.SendStatus(fiber.StatusNotFound)
	}
	c.Set(fiber.HeaderCacheControl, "public, max-age=300")
	c.Set(fiber.HeaderAccessControlAllowOrigin, "*")
	return c.JSON(doc)
}

// verify checks a token for the MCP server: a token of the HTTP API, or an
// OAuth access token of an MCP client.
func (s *Server) verify(ctx context.Context, key string, r *http.Request) (*auth.TokenInfo, error) {
	// Keys taken over from earlier releases may lack the ep_ prefix: what
	// is no OAuth token is checked as a key.
	if !strings.HasPrefix(key, tokens.KeyPrefix) && s.oauth != nil {
		if info, err := s.verifyOAuth(key); err == nil {
			return info, nil
		} else if !errors.Is(err, errNotOAuth) {
			return nil, err
		}
	}
	use, err := s.tokens.Authenticate(key, r.Header.Get(clientIPHeader))
	if err != nil {
		if errors.Is(err, tokens.ErrInvalid) {
			return nil, auth.ErrInvalidToken
		}
		s.store.Logger.Warnf("mcp: checking a token: %v", err)
		return nil, fmt.Errorf("%w: the token could not be checked", auth.ErrInvalidToken)
	}
	info := &auth.TokenInfo{UserID: use.Account.ID, Extra: map[string]any{useKey: use}}
	if use.Token.ExpiresAt != nil {
		info.Expiration = *use.Token.ExpiresAt
	}
	return info, nil
}

// useOf is the token of a tool call.
func useOf(req *sdk.CallToolRequest) (*tokens.Use, error) {
	if req == nil || req.Extra == nil || req.Extra.TokenInfo == nil {
		return nil, errNoToken
	}
	use, ok := req.Extra.TokenInfo.Extra[useKey].(*tokens.Use)
	if !ok || use == nil {
		return nil, errNoToken
	}
	return use, nil
}

var (
	errNotOAuth  = errors.New("not an OAuth access token")
	errNoToken   = errors.New("no token")
	errForbidden = errors.New("forbidden: the token may not do that on this pad")
)

// padID checks a pad id from a client and returns the pad's real id. The
// token needs scope on both the id as given and the real one, as in the
// HTTP API, so a pattern cannot be passed by a spelling the server changes.
func (s *Server) padID(use *tokens.Use, raw, scope string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("pad is required")
	}
	id, err := s.store.PadManager.SanitizePadId(raw)
	if err != nil {
		return "", fmt.Errorf("invalid pad id %q", raw)
	}
	if ok, err := s.store.PadManager.CanOpenPadId(id); err != nil {
		return "", err
	} else if !ok {
		return "", fmt.Errorf("invalid pad id %q", raw)
	}
	if !use.Grants.Has(scope, raw) || !use.Grants.Has(scope, id) {
		return "", errForbidden
	}
	return id, nil
}

// authorOf is the author the token writes as: its account's (a person's,
// or a service account's own).
func (s *Server) authorOf(use *tokens.Use) (string, error) {
	id, err := s.accounts.HolderAuthorOf(use.Account.ID, func(name string) (string, error) {
		created, err := s.store.AuthorManager.CreateAuthor(&name)
		if err != nil {
			return "", err
		}
		return created.Id, nil
	})
	if err != nil {
		return "", fmt.Errorf("the token's author: %w", err)
	}
	if s.store.AuthorManager.IsErased(id) {
		return "", errors.New("the author of this token's account was erased; it can no longer write")
	}
	return id, nil
}

func ms(t *int64) string {
	if t == nil {
		return ""
	}
	return time.UnixMilli(*t).UTC().Format(time.RFC3339)
}

// verifyOAuth checks an OAuth access token: the person's rights of the
// moment, cut down to the scopes and pads they allowed the client.
func (s *Server) verifyOAuth(key string) (*auth.TokenInfo, error) {
	access, err := s.oauth.MCPAccessToken(key)
	if err != nil {
		return nil, errNotOAuth
	}
	g, err := s.accounts.GrantsFor(access.Account.ID)
	if err != nil {
		s.store.Logger.Warnf("mcp: rights of %s: %v", access.Account.ID, err)
		return nil, fmt.Errorf("%w: the token could not be checked", auth.ErrInvalidToken)
	}
	expires := access.ExpiresAt
	use := &tokens.Use{
		Token: &db.Token{ID: "oauth:" + access.ClientID, AccountID: access.Account.ID, Name: access.ClientID,
			Scopes: access.Scopes, PadPattern: access.PadPattern, ExpiresAt: &expires, ClientID: access.ClientID},
		Account: access.Account,
		Grants:  g.Restrict(access.Scopes, access.PadPattern, nil),
	}
	return &auth.TokenInfo{UserID: access.Account.ID, Expiration: expires, Extra: map[string]any{useKey: use}}, nil
}
