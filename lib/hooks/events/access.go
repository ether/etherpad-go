package events

// OnAccessCheckContext is passed to onAccessCheck hooks when access to a concrete
// pad is being checked (socket pad access). Mirrors the original's "any false
// denies" semantics: a single Deny() denies access.
type OnAccessCheckContext struct {
	PadId         string
	Token         string
	SessionCookie string

	denied bool
}

// Deny denies access to the pad.
func (c *OnAccessCheckContext) Deny() { c.denied = true }

// Denied reports whether any callback denied access.
func (c *OnAccessCheckContext) Denied() bool { return c.denied }

// GetAuthorIdContext is passed to getAuthorId hooks to let a plugin supply or
// override the author id resolved from a token. First non-empty answer wins; if
// none answer, the caller falls back to the database token->author mapping. User
// is the authenticated user (exposed as any; plugins type-assert) or nil.
type GetAuthorIdContext struct {
	Token string
	User  any

	authorId string
}

// SetAuthorId records the author id; the first non-empty value wins.
func (c *GetAuthorIdContext) SetAuthorId(id string) {
	if c.authorId == "" {
		c.authorId = id
	}
}

// AuthorId returns the resolved author id, or "" if no callback supplied one.
func (c *GetAuthorIdContext) AuthorId() string { return c.authorId }

// AuthenticateContext is passed to authenticate hooks during HTTP authentication,
// before the built-in basic-auth check. The first callback to answer wins: a
// callback calls Authenticate(username) to confirm a user or Reject() to fail
// authentication; calling neither defers to the next callback / built-in basic
// auth. GetHeader reads a request header by key. InputUsername and InputPassword
// are the credentials supplied by the client; Username() returns the identity
// confirmed by the winning Authenticate() call.
type AuthenticateContext struct {
	InputUsername string
	InputPassword string
	Path          string
	RequireAdmin  bool
	GetHeader     func(key string) string

	answered bool
	rejected bool
	username string
}

// Authenticate confirms the given username as authenticated (first answer wins).
func (c *AuthenticateContext) Authenticate(username string) {
	if !c.answered {
		c.answered = true
		c.username = username
	}
}

// Reject fails authentication (first answer wins).
func (c *AuthenticateContext) Reject() {
	if !c.answered {
		c.answered = true
		c.rejected = true
	}
}

// Answered reports whether a callback made an authentication decision.
func (c *AuthenticateContext) Answered() bool { return c.answered }

// Rejected reports whether the decision was an explicit rejection.
func (c *AuthenticateContext) Rejected() bool { return c.rejected }

// Username returns the authenticated username (valid when Answered && !Rejected).
func (c *AuthenticateContext) Username() string { return c.username }

// AuthorizeDecision is the aggregated answer of all authorize hook callbacks.
type AuthorizeDecision int

const (
	// AuthorizeDefer means no callback answered: fall through to built-in logic.
	AuthorizeDefer AuthorizeDecision = iota
	// AuthorizeGrant means a callback granted access at Level().
	AuthorizeGrant
	// AuthorizeDeny means a callback denied access (wins over any grant).
	AuthorizeDeny
)

// AuthorizeContext is passed to authorize hooks during post-authentication
// authorization. A callback may Grant a level ("create"/"modify"/"readOnly") or
// Deny(). Deny wins over any grant; otherwise the first granted level is used; if
// no callback answers, the decision defers to the built-in logic. User is the
// authenticated user (any; plugins type-assert) or nil.
type AuthorizeContext struct {
	Path         string
	PadId        string
	RequireAdmin bool
	User         any

	granted      bool
	grantedLevel string
	denied       bool
}

// Grant grants access at the given level (first grant wins): "create", "modify",
// or "readOnly".
func (c *AuthorizeContext) Grant(level string) {
	if !c.granted {
		c.granted = true
		c.grantedLevel = level
	}
}

// Deny denies authorization.
func (c *AuthorizeContext) Deny() { c.denied = true }

// Decision aggregates the callbacks' answers: Deny wins, else Grant if any grant,
// else Defer.
func (c *AuthorizeContext) Decision() AuthorizeDecision {
	if c.denied {
		return AuthorizeDeny
	}
	if c.granted {
		return AuthorizeGrant
	}
	return AuthorizeDefer
}

// Level returns the granted authorization level (valid when Decision()==AuthorizeGrant).
func (c *AuthorizeContext) Level() string { return c.grantedLevel }

// AuthnFailureContext is passed to authnFailure hooks when authentication fails.
// A callback can take over the error response (instead of the default 401) by
// calling Respond (optionally adding headers via SetHeader, e.g. a Location
// header for a login redirect).
type AuthnFailureContext struct {
	Path         string
	RequireAdmin bool

	handled bool
	status  int
	body    string
	headers map[string]string
}

// Respond marks the failure as handled and records the response to send.
func (c *AuthnFailureContext) Respond(status int, body string) {
	c.handled = true
	c.status = status
	c.body = body
}

// SetHeader records a response header to set alongside the Respond status/body.
func (c *AuthnFailureContext) SetHeader(key, value string) {
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	c.headers[key] = value
}

// Handled reports whether a callback overrode the default error response.
func (c *AuthnFailureContext) Handled() bool              { return c.handled }
func (c *AuthnFailureContext) Status() int                { return c.status }
func (c *AuthnFailureContext) Body() string               { return c.body }
func (c *AuthnFailureContext) Headers() map[string]string { return c.headers }

// AuthzFailureContext is passed to authzFailure hooks when authorization fails.
// A callback can take over the error response (instead of the default 403) by
// calling Respond (optionally adding headers via SetHeader).
type AuthzFailureContext struct {
	Path         string
	RequireAdmin bool

	handled bool
	status  int
	body    string
	headers map[string]string
}

// Respond marks the failure as handled and records the response to send.
func (c *AuthzFailureContext) Respond(status int, body string) {
	c.handled = true
	c.status = status
	c.body = body
}

// SetHeader records a response header to set alongside the Respond status/body.
func (c *AuthzFailureContext) SetHeader(key, value string) {
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	c.headers[key] = value
}

// Handled reports whether a callback overrode the default error response.
func (c *AuthzFailureContext) Handled() bool              { return c.handled }
func (c *AuthzFailureContext) Status() int                { return c.status }
func (c *AuthzFailureContext) Body() string               { return c.body }
func (c *AuthzFailureContext) Headers() map[string]string { return c.headers }
