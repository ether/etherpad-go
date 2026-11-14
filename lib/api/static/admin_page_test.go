package static

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const viteAdminIndex = `<!DOCTYPE html><html><head><link rel="icon" href="/favicon.ico" />` +
	`<script type="module" crossorigin src="/admin/assets/index-x.js"></script>` +
	`<link rel="stylesheet" crossorigin href="/admin/assets/index-x.css"></head><body><div id="root"></div></body></html>`

func adminGet(t *testing.T, trust bool, headers map[string]string) (string, string, []string) {
	t.Helper()
	app := fiber.New()
	app.Get("/admin/", func(c fiber.Ctx) error {
		return c.Type("html").SendString(adminPage(c, viteAdminIndex, trust))
	})
	req := httptest.NewRequest("GET", "/admin/", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	return string(b), resp.Header.Get("Cache-Control"), resp.Header.Values("Vary")
}

// The admin SPA behind a reverse proxy prefix (etherpad-lite's admin
// variant of the proxy-path cache poisoning, GHSA-fjgc-3mj7-8rg8).
func TestAdminPageProxyPath(t *testing.T) {
	html, cache, vary := adminGet(t, false, nil)
	assert.Equal(t, viteAdminIndex, html, "unchanged without a prefix")
	assert.Empty(t, cache)
	assert.Contains(t, vary, "x-proxy-path")

	html, cache, _ = adminGet(t, false, map[string]string{"x-proxy-path": "/sub"})
	assert.Contains(t, html, `src="/sub/admin/assets/index-x.js"`)
	assert.Contains(t, html, `href="/sub/admin/assets/index-x.css"`)
	assert.Contains(t, html, `href="/sub/favicon.ico"`)
	assert.NotContains(t, html, `"/admin/`)
	assert.Equal(t, "private, no-store", cache, "a prefixed page is not stored by shared caches")

	html, _, _ = adminGet(t, false, map[string]string{"x-forwarded-prefix": "/fwd"})
	assert.Equal(t, viteAdminIndex, html, "x-forwarded-prefix needs trustProxy")
	html, _, _ = adminGet(t, true, map[string]string{"x-forwarded-prefix": "/fwd"})
	assert.Contains(t, html, `src="/fwd/admin/assets/index-x.js"`)

	html, _, _ = adminGet(t, false, map[string]string{"x-proxy-path": `/x"><script>alert(1)</script>`})
	assert.NotContains(t, html, "<script>alert")
	assert.Contains(t, html, `src="/xscriptalert1/script/admin/assets/index-x.js"`)
}
