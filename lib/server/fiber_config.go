package server

import "github.com/gofiber/fiber/v3"

// FiberConfig is the HTTP server's configuration.
//
// Immutable: without it, the strings fiber hands out (route parameters,
// cookies, form values) point into its request buffer, which is reused for
// the next request. etherpad-go keeps pad ids from requests beyond the
// request (the pad cache, pad locks, the memory datastore), so a pad id
// taken from one request turned into another request's pad id afterwards:
// an import into pad "d" made pad "s" disappear from the memory store, and
// a cached pad could be found under another pad's id.
//
// trustProxy is the trustProxy setting (etherpad-lite: express "trust
// proxy"): behind a reverse proxy the client's address, the scheme (https)
// and the host come from its X-Forwarded-* headers. It used to be ignored,
// so every client had the proxy's address (rate limiting and logs) and
// cookies never got Secure.
func FiberConfig(trustProxy ...bool) fiber.Config {
	cfg := fiber.Config{Immutable: true}
	if len(trustProxy) > 0 && trustProxy[0] {
		cfg.TrustProxy = true
		cfg.TrustProxyConfig = fiber.TrustProxyConfig{
			// Whoever enables it says the server is only reachable
			// through the proxy, as with express' "trust proxy" = true.
			Proxies:  []string{"0.0.0.0/0", "::/0"},
			Loopback: true, Private: true, LinkLocal: true,
		}
		cfg.ProxyHeader = fiber.HeaderXForwardedFor
		cfg.EnableIPValidation = true
	}
	return cfg
}
