package utils

import (
	"net/url"

	"github.com/gofiber/fiber/v3"
)

// RequestProxyPath is the path prefix of a page request (see ProxyPath).
// Pages that put it into their HTML vary on the headers it reads, so a
// shared cache in front of Etherpad cannot hand one client's prefix to
// another; with a prefix the answer is not stored at all.
func RequestProxyPath(c fiber.Ctx, trustProxy bool) string {
	for _, h := range ProxyPathHeaders(trustProxy) {
		c.Vary(h)
	}
	base := ProxyPath(func(n string) string { return c.Get(n) }, trustProxy)
	if base != "" {
		c.Set(fiber.HeaderCacheControl, "private, no-store")
	}
	return base
}

// PadParam is the :pad route parameter as a pad name: the router leaves
// some escapes in it ("100%25" for 100%), so it is unescaped once, as
// GuardPadURL does; a value that is not valid escaping stays as it is.
func PadParam(c fiber.Ctx) string {
	raw := c.Params("pad")
	if unescaped, err := url.PathUnescape(raw); err == nil {
		return unescaped
	}
	return raw
}
