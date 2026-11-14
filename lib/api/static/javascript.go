package static

import (
	"html"
	"io/fs"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// license is one entry of the LibreJS web labels table.
type license struct{ name, url string }

var (
	apache2 = license{"Apache-2.0", "http://www.apache.org/licenses/LICENSE-2.0"}
	expat   = license{"Expat", "http://www.jclark.com/xml/copying.txt"}
	bsd3    = license{"Modified-BSD", "http://www.xfree86.org/3.3.6/COPYRIGHT2.html#5"}
	isc     = license{"ISC", "https://www.isc.org/downloads/software-support-policy/isc-license/"}
	gpl3    = license{"GNU-GPL-3.0-only", "http://www.gnu.org/licenses/gpl-3.0.html"}
)

const sourceURL = "https://github.com/ether/etherpad-go"

type jsBundle struct {
	path     string // served path, relative to the site root
	licenses []license
}

// The bundles and the licenses of what they contain (Etherpad's own code,
// lit, and in the sheet HyperFormula and chevrotain; React and friends in
// the admin), so LibreJS and readers can tell what runs in their browser.
var jsBundles = []jsBundle{
	{"js/welcome/assets/welcome.js", []license{apache2}},
	{"js/pad/assets/pad.js", []license{apache2, expat, bsd3}},
	{"js/timeslider/assets/timeslider.js", []license{apache2, expat, bsd3}},
	{"js/sheet/assets/sheet.js", []license{apache2, expat, bsd3, gpl3}},
}

var adminLicenses = []license{apache2, expat, isc}

// javascriptPage is etherpad-lite's /javascript: the LibreJS web labels
// table (jslicense-labels1) for the scripts Etherpad serves. Links are
// relative, so they work under a reverse proxy prefix.
func javascriptPage(assets fs.FS) string {
	bundles := append([]jsBundle(nil), jsBundles...)
	if entries, err := fs.ReadDir(assets, "assets/js/admin/assets"); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".js") {
				bundles = append(bundles, jsBundle{"admin/assets/" + e.Name(), adminLicenses})
			}
		}
	}
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8">` +
		`<meta name="robots" content="noindex, nofollow">` +
		`<meta name="viewport" content="width=device-width, initial-scale=1">` +
		`<title>JavaScript license information</title></head><body>` +
		`<h1>JavaScript license information</h1><table id="jslicense-labels1">`)
	for _, bundle := range bundles {
		name := bundle.path[strings.LastIndex(bundle.path, "/")+1:]
		b.WriteString(`<tr><td><a href="` + html.EscapeString(bundle.path) + `">` + html.EscapeString(name) + `</a></td><td>`)
		for i, l := range bundle.licenses {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(`<a href="` + l.url + `">` + l.name + `</a>`)
		}
		b.WriteString(`</td><td><a href="` + sourceURL + `">` + sourceURL + `</a></td></tr>`)
	}
	b.WriteString(`</table></body></html>`)
	return b.String()
}

func serveJavascriptPage(assets fs.FS) fiber.Handler {
	page := javascriptPage(assets)
	return func(c fiber.Ctx) error {
		return c.Type("html").SendString(page)
	}
}
