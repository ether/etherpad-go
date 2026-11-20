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
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/accounts"
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
		// RFC 6750: a 401 names the scheme it wants.
		if c.Response().StatusCode() == fiber.StatusUnauthorized && len(c.Response().Header.Peek(fiber.HeaderWWWAuthenticate)) == 0 {
			c.Set(fiber.HeaderWWWAuthenticate, `Bearer realm="etherpad"`)
		}
		return nil
	})
}

// verify checks a token for the MCP server.
func (s *Server) verify(_ context.Context, key string, r *http.Request) (*auth.TokenInfo, error) {
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
