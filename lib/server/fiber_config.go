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
func FiberConfig() fiber.Config {
	return fiber.Config{Immutable: true}
}
