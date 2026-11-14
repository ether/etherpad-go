package static

import (
	"io"
	"net/http/httptest"
	"regexp"
	"testing"

	staticapi "github.com/ether/etherpad-go/lib/api/static"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's socialMeta spec: the Open Graph and Twitter Card
// tags on the served pages (lib/socialmeta has the unit spec).
func TestSocialMetaPages(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "pad page", Test: testSocialMetaPad},
		testutils.TestRunConfig{Name: "pad names", Test: testSocialMetaPadNames},
		testutils.TestRunConfig{Name: "timeslider and home", Test: testSocialMetaOtherPages},
		testutils.TestRunConfig{Name: "description override", Test: testSocialMetaOverride},
	)
	defer testDb.StartTestDBHandler()
}

func ogTag(html, prop string) (string, bool) {
	m := regexp.MustCompile(`<meta\s+(?:property|name)="` + regexp.QuoteMeta(prop) + `"\s+content="([^"]*)">`).FindStringSubmatch(html)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func socialPage(t *testing.T, ds testutils.TestDataStore, url, lang string) string {
	t.Helper()
	store := ds.ToInitStore()
	// The server loads the locales at startup (server.go); the socialmeta
	// descriptions come from them.
	hooks.ExpressPreSession(fiber.New(), store.UiAssets)
	staticapi.Init(store)
	req := httptest.NewRequest("GET", url, nil)
	if lang != "" {
		req.Header.Set("Accept-Language", lang)
	}
	resp, err := store.C.Test(req)
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	require.Equal(t, 200, resp.StatusCode, string(b))
	return string(b)
}

func mustOg(t *testing.T, html, prop string) string {
	t.Helper()
	v, ok := ogTag(html, prop)
	require.True(t, ok, "missing %s", prop)
	return v
}

func testSocialMetaPad(t *testing.T, ds testutils.TestDataStore) {
	ds.Settings.Title = "Etherpad Test"
	html := socialPage(t, ds, "/p/TestPad7599", "en")
	assert.Equal(t, "TestPad7599 | Etherpad Test", mustOg(t, html, "og:title"))
	assert.Regexp(t, `(?i)collaborative`, mustOg(t, html, "og:description"), "from the i18n catalog")
	assert.Regexp(t, `/favicon\.ico$`, mustOg(t, html, "og:image"))
	assert.Equal(t, "Etherpad Test logo", mustOg(t, html, "og:image:alt"))
	assert.Regexp(t, `^en`, mustOg(t, html, "og:locale"))
	assert.Equal(t, "summary", mustOg(t, html, "twitter:card"))

	ja := socialPage(t, ds, "/p/TestPad7599", "ja")
	assert.Regexp(t, `(?i)collaborative`, mustOg(t, ja, "og:description"), "no translation: English, not empty")
}

func testSocialMetaPadNames(t *testing.T, ds testutils.TestDataStore) {
	title := mustOg(t, socialPage(t, ds, "/p/Has%2DDash7599", ""), "og:title")
	assert.Regexp(t, `^Has-Dash7599 \| `, title, "the decoded pad name")

	title = mustOg(t, socialPage(t, ds, "/p/100%25Test", ""), "og:title")
	assert.Regexp(t, `^100%Test \| `, title, `a literal "%" renders`)

	// A crafted pad id may be refused or rendered, but never injects markup.
	store := ds.ToInitStore()
	staticapi.Init(store)
	resp, err := store.C.Test(httptest.NewRequest("GET", "/p/%3Cscript%3Ealert(1)%3C%2Fscript%3E", nil))
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	if v, ok := ogTag(string(b), "og:title"); ok {
		assert.NotRegexp(t, `(?i)<script>`, v)
	}
	assert.NotContains(t, string(b), "<script>alert(1)")
}

func testSocialMetaOtherPages(t *testing.T, ds testutils.TestDataStore) {
	ds.Settings.Title = "Etherpad Test"
	ts := socialPage(t, ds, "/p/TestPad7599/timeslider", "")
	assert.Equal(t, "TestPad7599 (history) | Etherpad Test", mustOg(t, ts, "og:title"))

	home := socialPage(t, ds, "/", "")
	assert.Equal(t, "Etherpad Test", mustOg(t, home, "og:title"))
}

func testSocialMetaOverride(t *testing.T, ds testutils.TestDataStore) {
	ds.Settings.SocialMeta.Description = "Custom blurb for issue 7599"
	html := socialPage(t, ds, "/p/TestPad7599", "")
	assert.Equal(t, "Custom blurb for issue 7599", mustOg(t, html, "og:description"))
	assert.Equal(t, "Custom blurb for issue 7599", mustOg(t, html, "twitter:description"))

	ds.Settings.SocialMeta.Description = "Operator wins"
	assert.Equal(t, "Operator wins", mustOg(t, socialPage(t, ds, "/p/TestPad7599", "de"), "og:description"),
		"beats Accept-Language")

	ds.Settings.SocialMeta.Description = "   "
	assert.Regexp(t, `(?i)collaborative`, mustOg(t, socialPage(t, ds, "/p/TestPad7599", "en"), "og:description"),
		"a blank override does not silence the preview")

	ds.Settings.SocialMeta.Description = `A & B <c> "d"`
	html = socialPage(t, ds, "/p/TestPad7599", "")
	assert.NotRegexp(t, `content="[^"]*<c>`, html)
	assert.Equal(t, "A &amp; B &lt;c&gt; &quot;d&quot;", mustOg(t, html, "og:description"))

	ds.Settings.SocialMeta.Description = "Home too"
	assert.Equal(t, "Home too", mustOg(t, socialPage(t, ds, "/", ""), "og:description"))
	assert.Equal(t, "Home too", mustOg(t, socialPage(t, ds, "/p/TestPad7599/timeslider", ""), "og:description"))
}
