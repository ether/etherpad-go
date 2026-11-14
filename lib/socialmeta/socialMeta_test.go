package socialmeta

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's socialMeta-unit spec (the pure helpers of
// src/node/utils/socialMeta.ts).

// tag returns the content of the meta tag prop, still HTML-escaped as in
// the page, or "" and false.
func tag(html, prop string) (string, bool) {
	re := regexp.MustCompile(`<meta\s+(?:property|name)="` + regexp.QuoteMeta(prop) + `"\s+content="([^"]*)">`)
	m := re.FindStringSubmatch(html)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func mustTag(t *testing.T, html, prop string) string {
	t.Helper()
	v, ok := tag(html, prop)
	require.True(t, ok, "missing %s in\n%s", prop, html)
	return v
}

func req(mod ...func(*RequestInfo)) RequestInfo {
	r := RequestInfo{Scheme: "https", Host: "pad.example", Path: "/p/Foo", AcceptLanguage: "en"}
	for _, m := range mod {
		m(&r)
	}
	return r
}

var (
	baseSettings = Settings{Title: "Etherpad"}
	enLocales    = Locales{"en": {"pad.social.description": "English desc"}}
	onlyEn       = map[string]struct{}{"en": {}}
)

func langs(l ...string) map[string]struct{} {
	m := map[string]struct{}{}
	for _, x := range l {
		m[x] = struct{}{}
	}
	return m
}

func pad(name string) Opts {
	return Opts{Req: req(), Settings: baseSettings, AvailableLangs: onlyEn, Locales: enLocales, Kind: KindPad, PadName: name}
}

func TestRenderTags(t *testing.T) {
	t.Run("emits all 13 OG + Twitter Card tags", func(t *testing.T) {
		html := Render(pad("Foo"))
		for _, p := range []string{"og:type", "og:site_name", "og:title", "og:description", "og:url", "og:image",
			"og:image:alt", "og:locale", "twitter:card", "twitter:title", "twitter:description", "twitter:image", "twitter:image:alt"} {
			mustTag(t, html, p)
		}
		assert.Equal(t, 13, strings.Count(html, "<meta "))
	})

	t.Run("HTML-escapes every interpolated value", func(t *testing.T) {
		evil := `"><script>alert(1)</script>`
		o := pad(evil)
		o.Settings = Settings{Title: evil, Favicon: "https://x/" + evil, DescriptionOverride: evil}
		o.Req.Path = "/p/" + evil
		html := Render(o)
		assert.NotRegexp(t, `(?i)<script>`, html)
		assert.NotContains(t, html, `"><script`)
		assert.Contains(t, html, "&lt;script&gt;alert(1)&lt;/script&gt;")
	})

	t.Run("og:locale as xx_XX for region tags", func(t *testing.T) {
		o := pad("P")
		o.Req.AcceptLanguage = "pt-BR"
		o.AvailableLangs = langs("en", "pt-BR")
		assert.Equal(t, "pt_BR", mustTag(t, Render(o), "og:locale"))
	})

	t.Run("og:locale as just primary for bare lang tags", func(t *testing.T) {
		o := pad("P")
		o.Req.AcceptLanguage = "fr"
		o.AvailableLangs = langs("en", "fr")
		assert.Equal(t, "fr", mustTag(t, Render(o), "og:locale"))
	})

	t.Run("twitter:card is always summary", func(t *testing.T) {
		for _, k := range []Kind{KindHome, KindPad, KindTimeslider} {
			o := pad("P")
			o.Kind = k
			assert.Equal(t, "summary", mustTag(t, Render(o), "twitter:card"), k)
		}
	})
}

func TestRenderTitle(t *testing.T) {
	assert.Equal(t, "MyPad | Etherpad", mustTag(t, Render(pad("MyPad")), "og:title"))

	ts := pad("MyPad")
	ts.Kind = KindTimeslider
	assert.Equal(t, "MyPad (history) | Etherpad", mustTag(t, Render(ts), "og:title"))

	home := Opts{Req: req(func(r *RequestInfo) { r.Path = "/" }), Settings: baseSettings, AvailableLangs: onlyEn, Locales: enLocales, Kind: KindHome}
	assert.Equal(t, "Etherpad", mustTag(t, Render(home), "og:title"))

	noTitle := pad("P")
	noTitle.Settings.Title = ""
	html := Render(noTitle)
	assert.Equal(t, "P | Etherpad", mustTag(t, html, "og:title"), `default site name "Etherpad"`)
	assert.Equal(t, "Etherpad", mustTag(t, html, "og:site_name"))
	assert.Equal(t, mustTag(t, html, "og:title"), mustTag(t, html, "twitter:title"))
}

func TestRenderDescription(t *testing.T) {
	enDe := Locales{"en": {"pad.social.description": "En"}, "de": {"pad.social.description": "De"}}
	with := func(accept string, available map[string]struct{}, locales Locales, override any) string {
		o := pad("P")
		o.Req.AcceptLanguage = accept
		o.AvailableLangs = available
		o.Locales = locales
		o.Settings.DescriptionOverride = override
		return mustTag(t, Render(o), "og:description")
	}

	assert.Equal(t, "De", with("de", langs("en", "de"), enDe, nil), "exact locale match wins")
	assert.Equal(t, "De", with("de-CH", langs("en", "de", "de-CH"), enDe, nil), "region tag falls back to primary subtag")
	assert.Equal(t, "En", with("ja", langs("en", "ja"), Locales{"en": {"pad.social.description": "En"}}, nil), "unknown locale falls back to English")
	assert.Equal(t, "", with("en", onlyEn, Locales{}, nil), "empty if the catalog has no entry")
	assert.Equal(t, "En", with("ja;q=0.9, *", langs("en"), enDe, nil), "no available language: English")

	t.Run("override", func(t *testing.T) {
		o := pad("P")
		o.Req.AcceptLanguage = "de"
		o.AvailableLangs = langs("en", "de")
		o.Locales = enDe
		o.Settings.DescriptionOverride = "Operator-set blurb"
		html := Render(o)
		assert.Equal(t, "Operator-set blurb", mustTag(t, html, "og:description"), "beats the negotiated language")
		assert.Equal(t, "Operator-set blurb", mustTag(t, html, "twitter:description"))

		assert.Equal(t, "De", with("de", langs("en", "de"), enDe, nil), "nil override: catalog")
		for _, blank := range []string{"", "   ", "\t\n"} {
			assert.Equal(t, "Catalog wins", with("en", onlyEn, Locales{"en": {"pad.social.description": "Catalog wins"}}, blank),
				"blank override %q must not silence the preview", blank)
		}
		assert.Equal(t, "A &amp; B &quot;&lt;C&gt;&quot;", with("en", onlyEn, enLocales, `A & B "<C>"`), "escaped")
		assert.Equal(t, "Catalog", with("en", onlyEn, Locales{"en": {"pad.social.description": "Catalog"}}, nil), "missing block is unset")
		assert.Equal(t, "2026", with("en", onlyEn, enLocales, 2026), "a number from an env var")
		assert.Equal(t, "true", with("en", onlyEn, enLocales, true), "a boolean from an env var")
	})
}

func TestRenderImage(t *testing.T) {
	assert.Equal(t, "https://pad.example/favicon.ico", mustTag(t, Render(pad("P")), "og:image"))

	cdn := pad("P")
	cdn.Settings.Favicon = "https://cdn.example/icon.png"
	html := Render(cdn)
	assert.Equal(t, "https://cdn.example/icon.png", mustTag(t, html, "og:image"), "absolute favicon verbatim")
	assert.Equal(t, "https://cdn.example/icon.png", mustTag(t, html, "twitter:image"))

	local := pad("P")
	local.Settings.Favicon = "custom/icon.png"
	assert.Equal(t, "https://pad.example/favicon.ico", mustTag(t, Render(local), "og:image"),
		"a file favicon is served at /favicon.ico")

	named := pad("P")
	named.Settings.Title = "MyPad Server"
	html = Render(named)
	assert.Equal(t, "MyPad Server logo", mustTag(t, html, "og:image:alt"))
	assert.Equal(t, "MyPad Server logo", mustTag(t, html, "twitter:image:alt"))
}

func TestRenderURL(t *testing.T) {
	render := func(o Opts) string { return mustTag(t, Render(o), "og:url") }

	o := pad("Foo")
	o.Req.Scheme = "http"
	assert.Equal(t, "http://pad.example/p/Foo", render(o), "from scheme, host and path")

	o = pad("Foo")
	o.Req.Path = "/p/Foo?utm_source=tweet"
	assert.Equal(t, "https://pad.example/p/Foo", render(o), "no query string")

	o = pad("Foo")
	o.Req.Scheme, o.Req.Host = "http", "evil.com"
	o.Settings.PublicURL = "https://pad.canonical.example"
	html := Render(o)
	assert.Equal(t, "https://pad.canonical.example/p/Foo", mustTag(t, html, "og:url"), "publicURL beats the request")
	assert.Equal(t, "https://pad.canonical.example/favicon.ico", mustTag(t, html, "og:image"))

	o = pad("Foo")
	o.Settings.PublicURL = "https://pad.example///"
	assert.Equal(t, "https://pad.example/p/Foo", render(o), "trailing slashes of publicURL")

	for _, bad := range []string{"pad.example", "http:///foo", "https://user@pad.example", "javascript:alert(1)", "https://pad.example/path"} {
		o = pad("Foo")
		o.Settings.PublicURL = bad
		assert.Equal(t, "https://pad.example/p/Foo", render(o), "malformed publicURL %q: the request", bad)
	}

	for _, bad := range []string{"evil.com\r\nX-Injected: 1", "user@evil.com", "<script>", "*", "", strings.Repeat("a", 300)} {
		o = pad("Foo")
		o.Req.Host = bad
		u := render(o)
		assert.NotContains(t, u, "\n")
		assert.NotContains(t, u, "\r")
		assert.NotContains(t, u, "&lt;")
		assert.NotContains(t, u, "@")
		assert.True(t, strings.HasPrefix(u, "https://localhost/"), "host %q: %s", bad, u)
	}
	o = pad("Foo")
	o.Req.Host = "pad.example:9001"
	assert.Equal(t, "https://pad.example:9001/p/Foo", render(o), "a port is fine")

	o = pad("Foo")
	o.Req.Scheme = "javascript"
	assert.Equal(t, "http://pad.example/p/Foo", render(o), "only http or https")

	o = pad("100%")
	o.Req.Path = "/p/100%"
	assert.Equal(t, "100% | Etherpad", mustTag(t, Render(o), "og:title"), `a literal "%" is not decoded again`)
}

func TestRenderXSS(t *testing.T) {
	html := Render(pad("<img src=x onerror=alert(1)>"))
	assert.NotContains(t, html, "<img src=x")
	assert.Contains(t, html, "&lt;img")

	html = Render(pad(`X"><script>alert(1)</script>`))
	assert.NotRegexp(t, `"><script`, html)
	assert.Contains(t, html, "&quot;")
}

func TestRenderProxyPath(t *testing.T) {
	o := pad("scratch")
	o.Req.Path = "/p/scratch"
	o.Req.ProxyPath = "/api/hassio_ingress/abc"
	assert.Contains(t, Render(o), `content="https://pad.example/api/hassio_ingress/abc/p/scratch"`)

	o.Req.ProxyPath = "/sub"
	assert.Contains(t, Render(o), `content="https://pad.example/sub/favicon.ico"`)

	o.Settings.PublicURL = "https://pad.canonical.example"
	html := Render(o)
	assert.Contains(t, html, `content="https://pad.canonical.example/p/scratch"`, "publicURL wins over the prefix")
	assert.NotContains(t, html, "/sub/")

	o = pad("scratch")
	o.Req.Path = "/p/scratch"
	assert.Contains(t, Render(o), `content="https://pad.example/p/scratch"`, "no prefix: today's URL shape")
}
