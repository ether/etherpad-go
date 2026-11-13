package server

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// trustProxy: the client's address and scheme come from the proxy headers.
func TestTrustProxy(t *testing.T) {
	for _, trust := range []bool{false, true} {
		app := fiber.New(FiberConfig(trust))
		app.Get("/", func(c fiber.Ctx) error { return c.SendString(c.IP() + " " + c.Scheme()) })
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		req.Header.Set("X-Forwarded-Proto", "https")
		resp, err := app.Test(req)
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		if trust {
			assert.Equal(t, "203.0.113.7 https", string(body))
		} else {
			assert.NotContains(t, string(body), "203.0.113.7")
			assert.Contains(t, string(body), "http")
		}
	}
}
