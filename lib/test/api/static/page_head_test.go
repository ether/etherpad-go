package static

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	staticapi "github.com/ether/etherpad-go/lib/api/static"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's specialpages spec (theme-color, dark-mode flash
// prevention) and the head every page needs.
func TestPageHead(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "theme-color", Test: testThemeColor},
		testutils.TestRunConfig{Name: "dark-mode pre-paint script", Test: testPrePaint},
		testutils.TestRunConfig{Name: "charset, viewport, skin variants", Test: testHeadBasics},
		testutils.TestRunConfig{Name: "/javascript license page", Test: testJavascriptPage},
	)
	defer testDb.StartTestDBHandler()
}

func page(t *testing.T, ds testutils.TestDataStore, url string) string {
	t.Helper()
	store := ds.ToInitStore()
	staticapi.Init(store)
	resp, err := store.C.Test(httptest.NewRequest("GET", url, nil))
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	require.Equal(t, 200, resp.StatusCode, string(b))
	return string(b)
}

const (
	lightVariants = "super-light-toolbar super-light-editor light-background"
	prePaint      = "el.classList.add('super-dark-editor', 'dark-background', 'super-dark-toolbar')"
)

func testThemeColor(t *testing.T, ds testutils.TestDataStore) {
	ds.Settings.SkinName = "colibris"
	ds.Settings.SkinVariants = lightVariants
	ds.Settings.EnableDarkMode = true
	for _, url := range []string{"/p/testpad", "/p/testpad/timeslider"} {
		html := page(t, ds, url)
		assert.Contains(t, html, `<meta name="theme-color" content="#ffffff" media="(prefers-color-scheme: light)">`, url)
		assert.Contains(t, html, `<meta name="theme-color" content="#485365" media="(prefers-color-scheme: dark)">`, url)
	}

	ds.Settings.EnableDarkMode = false
	html := page(t, ds, "/p/testpad")
	assert.Contains(t, html, `<meta name="theme-color" content="#ffffff">`, "no dark variant without dark mode")
	assert.NotContains(t, html, "prefers-color-scheme")

	ds.Settings.SkinVariants = "dark-toolbar dark-editor dark-background"
	assert.Contains(t, page(t, ds, "/p/testpad"), `<meta name="theme-color" content="#576273"`, "an explicit dark toolbar")

	ds.Settings.SkinName = "no-skin"
	ds.Settings.SkinVariants = "super-light-toolbar"
	for _, url := range []string{"/p/testpad", "/p/testpad/timeslider"} {
		assert.NotContains(t, page(t, ds, url), "theme-color", "unknown skin colours: %s", url)
	}
}

func testPrePaint(t *testing.T, ds testutils.TestDataStore) {
	ds.Settings.SkinName = "colibris"
	ds.Settings.SkinVariants = lightVariants
	ds.Settings.EnableDarkMode = true
	html := page(t, ds, "/p/testpad")
	scriptAt := strings.Index(html, prePaint)
	cssAt := strings.Index(html, "pad.css")
	require.NotEqual(t, -1, scriptAt, "the pre-paint script is in the pad page")
	assert.True(t, cssAt > scriptAt, "the script comes before pad.css")
	assert.Contains(t, html, "#skinvariantsbuilder", "it skips the skin-variants builder, like pad.ts")
	assert.Contains(t, page(t, ds, "/p/testpad/timeslider"), prePaint)

	ds.Settings.EnableDarkMode = false
	assert.NotContains(t, page(t, ds, "/p/testpad"), prePaint, "no dark mode")
	ds.Settings.EnableDarkMode = true
	ds.Settings.SkinName = "no-skin"
	assert.NotContains(t, page(t, ds, "/p/testpad"), prePaint, "not colibris")
}

func testHeadBasics(t *testing.T, ds testutils.TestDataStore) {
	ds.Settings.SkinName = "colibris"
	ds.Settings.SkinVariants = "dark-toolbar super-dark-editor dark-background"
	ds.Settings.Title = "My Pads"
	manifests := map[string]string{"/": "/manifest.json", "/p/testpad": "../manifest.json", "/p/testpad/timeslider": "../../manifest.json"}
	for _, url := range []string{"/", "/p/testpad", "/p/testpad/timeslider"} {
		html := page(t, ds, url)
		assert.True(t, strings.HasPrefix(html, "<!doctype html>") || strings.HasPrefix(html, "<!DOCTYPE html>"), "%s: standards mode", url)
		assert.Contains(t, html, `<meta charset="utf-8">`, url)
		assert.Contains(t, html, `name="viewport"`, url)
		assert.Contains(t, html, `<link rel="manifest" href="`+manifests[url]+`">`, url)
		assert.Contains(t, html, `lang="en"`, url)
	}
	pad := page(t, ds, "/p/testpad")
	assert.Contains(t, pad, `class="pad dark-toolbar super-dark-editor dark-background"`, "the configured skin variants")
	assert.Contains(t, pad, `<meta name="robots" content="noindex, nofollow">`)
	assert.Contains(t, page(t, ds, "/p/testpad/timeslider"), "<title>My Pads Timeslider</title>")
	assert.NotContains(t, page(t, ds, "/"), "noindex", "the start page may be indexed")
	ts := page(t, ds, "/p/testpad/timeslider")
	assert.NotContains(t, ts, "{translations[", "toolbar titles are translated, not template source")
	assert.Contains(t, ts, `title="Settings"`)
}

// etherpad-lite's /javascript: LibreJS web labels for the served scripts.
func testJavascriptPage(t *testing.T, ds testutils.TestDataStore) {
	html := page(t, ds, "/javascript")
	assert.Contains(t, html, `<table id="jslicense-labels1">`)
	for _, js := range []string{"js/pad/assets/pad.js", "js/timeslider/assets/timeslider.js", "js/welcome/assets/welcome.js", "js/sheet/assets/sheet.js"} {
		assert.Contains(t, html, `href="`+js+`"`, "relative, so it works under a proxy prefix")
	}
	assert.Contains(t, html, `>GNU-GPL-3.0-only</a>`, "the sheet bundles HyperFormula")
	assert.NotContains(t, html, `href="/`)
}
