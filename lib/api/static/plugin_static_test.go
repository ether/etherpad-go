package static

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// etherpad-lite: staticPathTraversal (GHSA-mc8w-wjhw-45x5). /static/plugins/
// needs no login, so it must serve only plugins' static/ folders.
func TestPluginStaticFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	write("plugins/ep_x/static/css/a.css", "css")
	write("plugins/ep_x/package.json", "plugin source")
	write("plugins/ep_x/settings.json", "plugin secret")
	write("settings.json", "server secret")
	cwd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	app := fiber.New()
	app.Get("/static/plugins/*", servePluginStatic)
	get := func(path string) (int, string) {
		req := httptest.NewRequest("GET", "/", nil)
		req.URL.Path, req.URL.RawPath, req.RequestURI = path, path, path
		resp, err := app.Test(req)
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	status, body := get("/static/plugins/ep_x/static/css/a.css")
	assert.Equal(t, 200, status, "a legitimate plugin static asset is served")
	assert.Equal(t, "css", body)

	for _, path := range []string{
		"/static/plugins/ep_x/package.json",
		"/static/plugins/ep_x/settings.json",
		"/static/plugins/ep_x/static/../settings.json",
		"/static/plugins/ep_x/static/..%2f..%2f..%2fsettings.json",
		"/static/plugins/ep_x/static/%2e%2e/%2e%2e/%2e%2e/settings.json",
		"/static/plugins/../settings.json",
		"/static/plugins/%2e%2e/settings.json",
		"/static/plugins/ep_x/static/",
		"/static/plugins/notaplugin/static/x",
		"/static/plugins/ep_x/static//etc/passwd",
		"/static/plugins/ep_x/static/missing.css",
	} {
		status, body := get(path)
		assert.Equal(t, 404, status, path)
		assert.NotContains(t, body, "secret", path)
		assert.NotContains(t, body, "plugins/", "the response does not reveal paths: "+path)
	}
}

// Port of etherpad-lite's sanitizePathname cases for the one file path a
// request names directly besides plugin assets: /admin/locales/<file>,
// which is read from disk in dev mode. Windows separators and drive
// letters count as well, since the server runs there too.
func TestAdminLocaleName(t *testing.T) {
	for _, ok := range []string{"de.json", "en.json", "pt-br.json", "zh-hant.json", "be-tarask.json", "sr_EC.json"} {
		assert.True(t, adminLocaleName.MatchString(ok), ok)
	}
	for _, bad := range []string{
		"", ".", "..", "../", "../foo", "foo/../..", `..\`, `..\foo`, `foo\..\..`, `..\..\settings.json`,
		"/", "/foo", `\`, `\foo`, "C:/foo.json", `C:\foo.json`, "c:/foo.json",
		".json", "..json", "de.json/", "de.json.exe", "de", "settings.json\x00.json", "de .json",
	} {
		assert.False(t, adminLocaleName.MatchString(bad), "%q", bad)
	}
}
