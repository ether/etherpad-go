package events

// PreAuthorizeDecision is the aggregated answer of all preAuthorize hook
// callbacks for a request.
type PreAuthorizeDecision int

const (
	// PreAuthorizeDefer means no callback answered: fall through to the regular
	// authenticate/authorize steps (the original's empty result list).
	PreAuthorizeDefer PreAuthorizeDecision = iota
	// PreAuthorizePermit means access was explicitly granted: skip the remaining
	// steps (the original's `return next()` when every result is truthy).
	PreAuthorizePermit
	// PreAuthorizeDeny means access was explicitly denied: respond with 403
	// unless a preAuthzFailure callback overrides the response.
	PreAuthorizeDeny
)

// PreAuthorizeContext is passed to preAuthorize hook callbacks. It is the Go
// counterpart of the original hook context {req, res, next}: callbacks inspect
// Path/RequireAdmin and answer by calling Permit or Deny; calling neither
// defers to the regular authenticate/authorize steps. The classic use case is
// permitting static resource paths so they skip authentication.
//
// Semantics adapted from the original's hooks.aCallFirst: the Go hook system
// runs every registered callback (in unspecified order) instead of stopping at
// the first one that answers, so a single Deny always wins over any number of
// Permits. As in the original, Permits on /admin-auth pages (RequireAdmin) are
// filtered out so plugins cannot accidentally grant admin privileges to the
// general public.
type PreAuthorizeContext struct {
	// Path is the request path, e.g. "/p/mypad" (input, read-only).
	Path string
	// RequireAdmin is true when the request is for an /admin-auth page (input,
	// read-only).
	RequireAdmin bool

	results []bool
}

// Permit explicitly grants access. On admin pages the permit is ignored (see
// the type documentation).
func (c *PreAuthorizeContext) Permit() {
	c.results = append(c.results, true)
}

// Deny explicitly denies access.
func (c *PreAuthorizeContext) Deny() {
	c.results = append(c.results, false)
}

// Decision aggregates the callbacks' answers, mirroring the original's result
// handling: admin pages drop all permits, an empty result list defers, any
// remaining false denies, and all-true permits.
func (c *PreAuthorizeContext) Decision() PreAuthorizeDecision {
	answered := false
	for _, r := range c.results {
		if c.RequireAdmin && r {
			continue // never let a plugin permit grant admin access
		}
		answered = true
		if !r {
			return PreAuthorizeDeny
		}
	}
	if !answered {
		return PreAuthorizeDefer
	}
	return PreAuthorizePermit
}

// PreAuthzFailureContext is passed to preAuthzFailure hook callbacks when a
// preAuthorize callback denied access. A callback can take over the error
// response — the original's "return truthy after writing to res" — by calling
// Respond (optionally adding headers via SetHeader, e.g. a Location header for
// a login redirect). If no callback responds, the default 403 Forbidden is
// sent.
type PreAuthzFailureContext struct {
	// Path is the request path (input, read-only).
	Path string
	// RequireAdmin is true when the request is for an /admin-auth page (input,
	// read-only).
	RequireAdmin bool

	handled bool
	status  int
	body    string
	headers map[string]string
}

// Respond marks the failure as handled and records the response to send
// instead of the default 403 Forbidden.
func (c *PreAuthzFailureContext) Respond(status int, body string) {
	c.handled = true
	c.status = status
	c.body = body
}

// SetHeader records a response header to set alongside the Respond status and
// body.
func (c *PreAuthzFailureContext) SetHeader(key, value string) {
	if c.headers == nil {
		c.headers = make(map[string]string)
	}
	c.headers[key] = value
}

// Handled reports whether a callback overrode the default error response.
func (c *PreAuthzFailureContext) Handled() bool { return c.handled }

// Status returns the recorded response status.
func (c *PreAuthzFailureContext) Status() int { return c.status }

// Body returns the recorded response body.
func (c *PreAuthzFailureContext) Body() string { return c.body }

// Headers returns the recorded response headers (may be nil).
func (c *PreAuthzFailureContext) Headers() map[string]string { return c.headers }
