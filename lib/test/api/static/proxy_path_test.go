package static

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	staticapi "github.com/ether/etherpad-go/lib/api/static"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ports of etherpad-lite's urlBasePath, proxyPathVary, proxyPathRedirect
// and pwaManifest specs: Etherpad behind a reverse proxy that serves it
// under a path prefix.
func TestProxyPath(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "no prefix: today's URLs", Test: testProxyPathNone},
		testutils.TestRunConfig{Name: "x-proxy-path prefixes the pages", Test: testProxyPathPages},
		testutils.TestRunConfig{Name: "trust-gated headers", Test: testProxyPathTrust},
		testutils.TestRunConfig{Name: "Vary on the pages", Test: testProxyPathVary},
		testutils.TestRunConfig{Name: "hostile prefixes", Test: testProxyPathHostile},
		testutils.TestRunConfig{Name: "manifest", Test: testManifest},
		testutils.TestRunConfig{Name: "trailing slash", Test: testPadTrailingSlash},
	)
	defer testDb.StartTestDBHandler()
}

func proxyGet(t *testing.T, ds testutils.TestDataStore, url string, headers map[string]string) (*http.Response, string) {
	t.Helper()
	store := ds.ToInitStore()
	staticapi.Init(store)
	req := httptest.NewRequest("GET", url, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := store.C.Test(req)
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	require.Equal(t, 200, resp.StatusCode, string(b))
	return resp, string(b)
}

func varyList(resp *http.Response) []string {
	var out []string
	for _, v := range resp.Header.Values("Vary") {
		for _, p := range strings.Split(v, ",") {
			out = append(out, strings.ToLower(strings.TrimSpace(p)))
		}
	}
	return out
}

func manifest(t *testing.T, ds testutils.TestDataStore, headers map[string]string) (startURL string, icons []string) {
	t.Helper()
	resp, body := proxyGet(t, ds, "/manifest.json", headers)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/manifest+json")
	var m struct {
		StartURL string `json:"start_url"`
		Icons    []struct {
			Src string `json:"src"`
		} `json:"icons"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &m), body)
	for _, i := range m.Icons {
		icons = append(icons, i.Src)
	}
	require.NotEmpty(t, icons)
	return m.StartURL, icons
}

func testProxyPathNone(t *testing.T, ds testutils.TestDataStore) {
	_, home := proxyGet(t, ds, "/", nil)
	assert.Contains(t, home, `href="/manifest.json"`)
	assert.Contains(t, home, `<base href="/">`)

	_, pad := proxyGet(t, ds, "/p/UrlBasePathTest", nil)
	assert.Contains(t, pad, `action="../ep/pad/reconnect"`, "relative, so any prefix works")
	assert.Contains(t, pad, `href="../manifest.json"`)
	assert.Contains(t, pad, `content="http://example.com/p/UrlBasePathTest"`, "og:url")

	start, icons := manifest(t, ds, nil)
	assert.Equal(t, "/", start)
	for _, s := range icons {
		assert.True(t, strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "//"), s)
	}
}

func testProxyPathPages(t *testing.T, ds testutils.TestDataStore) {
	h := map[string]string{"x-proxy-path": "/sub"}

	resp, home := proxyGet(t, ds, "/", h)
	assert.Contains(t, home, `href="/sub/manifest.json"`)
	assert.NotContains(t, home, `href="/manifest.json"`)
	assert.Contains(t, home, `<base href="/sub/">`, "the welcome script's p/<name> links")
	assert.Contains(t, home, `content="http://example.com/sub/"`, "og:url")
	assert.Equal(t, "private, no-store", resp.Header.Get("Cache-Control"))

	_, pad := proxyGet(t, ds, "/p/UrlBasePathTest", h)
	assert.Contains(t, pad, `action="../ep/pad/reconnect"`)
	assert.Contains(t, pad, `href="../manifest.json"`)
	assert.Contains(t, pad, `content="http://example.com/sub/p/UrlBasePathTest"`, "og:url")
	assert.Contains(t, pad, `content="http://example.com/sub/favicon.ico"`, "og:image")

	_, ts := proxyGet(t, ds, "/p/UrlBasePathTest/timeslider", h)
	assert.Contains(t, ts, `content="http://example.com/sub/p/UrlBasePathTest/timeslider"`)
	assert.Contains(t, ts, `href="../../manifest.json"`)

	start, icons := manifest(t, ds, h)
	assert.Equal(t, "/sub/", start)
	for _, s := range icons {
		assert.True(t, strings.HasPrefix(s, "/sub/"), s)
	}

	// A configured publicURL is the canonical address, as in etherpad-lite.
	ds.Settings.PublicURL = "https://pads.example.org"
	_, pad = proxyGet(t, ds, "/p/UrlBasePathTest", h)
	assert.Contains(t, pad, `content="https://pads.example.org/p/UrlBasePathTest"`)
}

func testProxyPathTrust(t *testing.T, ds testutils.TestDataStore) {
	h := map[string]string{"x-ingress-path": "/api/hassio_ingress/abc", "x-forwarded-prefix": "/fwd"}

	ds.Settings.TrustProxy = false
	_, pad := proxyGet(t, ds, "/p/UrlBasePathTest", h)
	assert.NotContains(t, pad, "/api/hassio_ingress/")
	assert.NotContains(t, pad, "/fwd/")
	start, _ := manifest(t, ds, h)
	assert.Equal(t, "/", start, "ignored without trustProxy")

	ds.Settings.TrustProxy = true
	_, pad = proxyGet(t, ds, "/p/UrlBasePathTest", h)
	assert.Contains(t, pad, "http://example.com/fwd/p/UrlBasePathTest", "x-forwarded-prefix comes first")
	start, _ = manifest(t, ds, map[string]string{"x-ingress-path": "/api/hassio_ingress/abc"})
	assert.Equal(t, "/api/hassio_ingress/abc/", start)
}

func testProxyPathVary(t *testing.T, ds testutils.TestDataStore) {
	for _, url := range []string{"/", "/p/ProxyPathVaryTest", "/p/ProxyPathVaryTest/timeslider", "/manifest.json"} {
		ds.Settings.TrustProxy = false
		resp, _ := proxyGet(t, ds, url, nil)
		vary := varyList(resp)
		assert.Contains(t, vary, "x-proxy-path", url)
		assert.NotContains(t, vary, "x-forwarded-prefix", "only what is read: %s", url)
		assert.NotContains(t, vary, "x-ingress-path", url)
		assert.Empty(t, resp.Header.Get("Cache-Control"), "cacheable without a prefix: %s", url)

		ds.Settings.TrustProxy = true
		resp, _ = proxyGet(t, ds, url, nil)
		vary = varyList(resp)
		for _, h := range []string{"x-proxy-path", "x-forwarded-prefix", "x-ingress-path"} {
			assert.Contains(t, vary, h, url)
		}
	}
}

func testProxyPathHostile(t *testing.T, ds testutils.TestDataStore) {
	for raw, want := range map[string]string{
		"//evil.example":           "/evil.example/",
		"///evil.example/x":        "/evil.example/x/",
		"https://evil.example":     "/https//evil.example/",
		`/a"><script>alert(1)</sc`: "/ascriptalert1/sc/",
		"/sub/../admin":            "/",
		"/":                        "/",
		"/sub/":                    "/sub/",
	} {
		start, icons := manifest(t, ds, map[string]string{"x-proxy-path": raw})
		assert.Equal(t, want, start, raw)
		for _, s := range icons {
			assert.False(t, strings.HasPrefix(s, "//"), "%q: protocol-relative %s", raw, s)
			assert.False(t, strings.Contains(s, "//images/") || strings.Contains(s, "//favicon"), "%q: prefix ends with a slash: %s", raw, s)
		}
		_, home := proxyGet(t, ds, "/", map[string]string{"x-proxy-path": raw})
		assert.NotContains(t, home, "<script>alert", raw)
		assert.NotContains(t, home, `href="//`, raw)
	}
}

func testManifest(t *testing.T, ds testutils.TestDataStore) {
	ds.Settings.Title = "My Pads"
	_, body := proxyGet(t, ds, "/manifest.json", nil)
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &m))
	assert.Equal(t, "My Pads", m["name"])
	assert.Equal(t, "My Pads", m["short_name"])
	assert.Equal(t, "fullscreen", m["display"])

	ds.Settings.Title = ""
	_, body = proxyGet(t, ds, "/manifest.json", nil)
	require.NoError(t, json.Unmarshal([]byte(body), &m))
	assert.Equal(t, "Etherpad", m["name"])
}

// The pad page's URLs are relative to /p/<pad>; /p/<pad>/ would resolve
// them one level too deep, so it redirects to the pad (relative, keeping a
// prefix).
func testPadTrailingSlash(t *testing.T, ds testutils.TestDataStore) {
	store := ds.ToInitStore()
	staticapi.Init(store)
	for path, want := range map[string]string{"/p/SlashPad/": "../SlashPad", "/p/SlashPad/timeslider/": "../timeslider", "/s/SlashSheet/": "../SlashSheet", "/p/SlashPad/?x=1": "../SlashPad?x=1"} {
		resp, err := store.C.Test(httptest.NewRequest("GET", path, nil))
		require.NoError(t, err)
		assert.Equal(t, 302, resp.StatusCode, path)
		assert.Equal(t, want, resp.Header.Get("Location"), path)
	}
}
