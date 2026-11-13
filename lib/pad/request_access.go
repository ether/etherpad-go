package pad

import (
	"net/url"

	"github.com/ether/etherpad-go/lib/models/clientVars"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/gofiber/fiber/v3"
)

// IntegratorSessionIDCookie reads the sessionID cookie an integrator set
// through the HTTP API's createSession (etherpad-lite #7045, #7755): the
// prefixed name first, then the plain one. The value is percent-decoded; one
// that cannot be decoded counts as absent.
func IntegratorSessionIDCookie(cookie func(key string, defaultValue ...string) string, prefix string) string {
	for _, name := range []string{prefix + "sessionID", "sessionID"} {
		raw := cookie(name)
		if raw == "" {
			continue
		}
		decoded, err := url.PathUnescape(raw)
		if err != nil {
			return ""
		}
		return decoded
	}
	return ""
}

// RequestUser is the user webaccess authenticated for this request, or nil.
func RequestUser(ctx fiber.Ctx) *webaccess.SocketClientRequest {
	user, _ := ctx.Locals(clientVars.WebAccessStore).(*webaccess.SocketClientRequest)
	return user
}

// CheckRequestAccess is CheckAccess for an HTTP request on a pad: with the
// user webaccess authenticated, the token cookie and the integrator's
// sessionID cookie, as etherpad-lite's import and export routes do.
func (s *SecurityManager) CheckRequestAccess(ctx fiber.Ctx, padId string) (*GrantedAccess, error) {
	token := ctx.Cookies("token")
	var session *string
	prefix := ""
	if s.settings != nil {
		prefix = s.settings.Cookie.Prefix
	}
	if id := IntegratorSessionIDCookie(ctx.Cookies, prefix); id != "" {
		session = &id
	}
	return s.CheckAccess(&padId, session, &token, RequestUser(ctx))
}

// RequestCanModify tells whether the request may change the pad: not through
// a read-only id and not with a read-only authorization.
func (s *SecurityManager) RequestCanModify(ctx fiber.Ctx, padId string) bool {
	return UserCanModify(&padId, RequestUser(ctx), *s.ReadOnlyManager, s.settings)
}
