package swagger

import (
	"io"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func get(t *testing.T, app *fiber.App, path string) (int, string, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", path, nil))
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("Content-Type"), string(b)
}

// The pages load everything from the server: no web fonts, no CDN, no
// online validator.
func TestPagesAreSelfContained(t *testing.T) {
	app := fiber.New()
	Init(app)
	external := regexp.MustCompile(`(?i)(href|src)=["']?(https?:)?//`)
	for _, page := range []struct{ base, doc string }{{"/swagger", "doc.json"}, {"/api-docs", "/api-docs.json"}} {
		status, ct, body := get(t, app, page.base+"/index.html")
		require.Equal(t, 200, status, page.base)
		assert.Contains(t, ct, "text/html")
		assert.False(t, external.MatchString(body), "%s links outside: %s", page.base, external.FindString(body))
		assert.Contains(t, body, "validatorUrl: null")
		assert.Contains(t, body, `"`+page.doc+`"`)

		for _, asset := range []string{"swagger-ui.css", "swagger-ui-bundle.js", "swagger-ui-standalone-preset.js", "favicon-32x32.png"} {
			status, _, _ := get(t, app, page.base+"/"+asset)
			assert.Equal(t, 200, status, page.base+"/"+asset)
		}
		status, _, _ = get(t, app, page.base+"/../../etc/passwd")
		assert.NotEqual(t, 200, status)
	}

	resp, err := app.Test(httptest.NewRequest("GET", "/api-docs", nil))
	require.NoError(t, err)
	assert.Equal(t, 301, resp.StatusCode)
	assert.Equal(t, "api-docs/index.html", resp.Header.Get("Location"))
	resp, err = app.Test(httptest.NewRequest("GET", "/api-docs/", nil))
	require.NoError(t, err)
	assert.Equal(t, "index.html", resp.Header.Get("Location"))

	status, ct, body := get(t, app, "/swagger/doc.json")
	assert.Equal(t, 200, status)
	assert.Contains(t, ct, "json")
	assert.Contains(t, body, `"paths"`)
	status, _, _ = get(t, app, "/api-docs/doc.json")
	assert.Equal(t, 404, status, "only /swagger serves the swag document")
}
