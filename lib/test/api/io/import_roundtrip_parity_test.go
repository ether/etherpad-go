package io

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's import spec (#7538): export -> import -> export
// round trips over the HTTP routes and HTML heading layout.
func TestImportRoundTripParity(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "round trips", Test: testImportRoundTrips},
		testutils.TestRunConfig{Name: "HTML headings", Test: testImportHTMLHeadings},
	)
	testDb.StartTestDBHandler()
}

const rtSample = "Line one\nLine two\n\nAfter blank\n"

var authorClass = regexp.MustCompile(`authora_[A-Za-z0-9_-]+`)

func bodyOf(html string) string {
	m := regexp.MustCompile(`(?s)<body>(.*?)</body>`).FindStringSubmatch(html)
	if m == nil {
		return ""
	}
	// The importer is another author than the creator of the source.
	body := authorClass.ReplaceAllString(m[1], "author")
	body = regexp.MustCompile(`(?s)<style>.*?</style>`).ReplaceAllString(body, "")
	return strings.TrimSpace(body)
}

func testImportRoundTrips(t *testing.T, ds testutils.TestDataStore) {
	app := ieApp(t, ds, false)
	seed := func(t *testing.T, padId string) {
		resp, body := ieImport(t, app, padId, "seed.txt", []byte(rtSample), "")
		require.Equal(t, 200, resp.StatusCode, string(body))
	}
	roundTrip := func(t *testing.T, src, dst, exportType, ext string) (a, c []byte) {
		seed(t, src)
		resp, a := ieExport(t, app, "/p/"+src+"/export/"+exportType, "")
		require.Equal(t, 200, resp.StatusCode)
		resp, body := ieImport(t, app, dst, "file."+ext, a, "")
		require.Equal(t, 200, resp.StatusCode, string(body))
		resp, c = ieExport(t, app, "/p/"+dst+"/export/"+exportType, "")
		require.Equal(t, 200, resp.StatusCode)
		return a, c
	}
	t.Run("txt", func(t *testing.T) {
		a, c := roundTrip(t, "rtTxtSrc", "rtTxtDst", "txt", "txt")
		assert.Equal(t, string(a), string(c))
	})
	t.Run("etherpad", func(t *testing.T) {
		roundTrip(t, "rtEpSrc", "rtEpDst", "etherpad", "etherpad")
		assert.Equal(t, iePadText_(t, ds, "rtEpSrc"), iePadText_(t, ds, "rtEpDst"))
	})
	t.Run("html", func(t *testing.T) {
		a, c := roundTrip(t, "rtHtmlSrc", "rtHtmlDst", "html", "html")
		assert.Equal(t, bodyOf(string(a)), bodyOf(string(c)))
		assert.Equal(t, iePadText_(t, ds, "rtHtmlSrc"), iePadText_(t, ds, "rtHtmlDst"))
	})
	for _, tc := range []struct{ name, exportType, ext string }{
		{"docx", "docx", "docx"}, {"odt", "odt", "odt"},
	} {
		t.Run(tc.name+" (line text)", func(t *testing.T) {
			roundTrip(t, "rt"+tc.name+"Src", "rt"+tc.name+"Dst", tc.exportType, tc.ext)
			norm := func(s string) string { return strings.TrimRight(s, "\n") + "\n" }
			assert.Equal(t, norm(iePadText_(t, ds, "rt"+tc.name+"Src")), norm(iePadText_(t, ds, "rt"+tc.name+"Dst")))
		})
	}
	t.Run("pdf export (sanity)", func(t *testing.T) {
		seed(t, "rtPdf")
		resp, body := ieExport(t, app, "/p/rtPdf/export/pdf", "")
		require.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "%PDF-", string(body[:5]))
	})
}

func testImportHTMLHeadings(t *testing.T, ds testutils.TestDataStore) {
	app := ieApp(t, ds, false)
	lines := func(t *testing.T, padId, html string) []string {
		resp, body := ieImport(t, app, padId, "file.html", []byte(html), "")
		require.Equal(t, 200, resp.StatusCode, string(body))
		out := strings.Split(strings.TrimRight(iePadText_(t, ds, padId), "\n"), "\n")
		for i, l := range out {
			out[i] = strings.TrimPrefix(l, "*") // line marker of a heading
		}
		return out
	}
	t.Run("no blank line between H1 and H2", func(t *testing.T) {
		assert.Equal(t, []string{"A", "B"}, lines(t, "headingsAdjacent", `<html><body><h1>A</h1><h2>B</h2></body></html>`))
	})
	t.Run("blank lines between headings are kept (export shape)", func(t *testing.T) {
		html := `<html><body><h1 style='text-align:right'>A</h1><br><p style='text-align:right'></p><br><p style='text-align:right'></p><br><h2 style='text-align:center'>B</h2><br></body></html>`
		assert.Equal(t, []string{"A", "", "", "B"}, lines(t, "headingsBlank", html))
	})
	t.Run("adjacent headings stay separate lines", func(t *testing.T) {
		assert.Equal(t, []string{"Welcome", "This pad", "Code line"}, lines(t, "headingsSeparate", `<h1>Welcome</h1><h2>This pad</h2><p>Code line</p>`))
	})
}
