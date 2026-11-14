package static

import (
	"os"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRebaseFontURLs(t *testing.T) {
	in := `a{src:url("../../../font/A.ttf") format("truetype"),url(../../../font/fontawesome-etherpad.woff?2)}` +
		`b{src:url(./font/B.otf)}c{background:url(data:image/svg+xml;base64,AA==)}`
	got := string(rebaseFontURLs([]byte(in), "/css/static"))
	assert.Contains(t, got, `url("../../font/A.ttf")`)
	assert.Contains(t, got, `url(../../font/fontawesome-etherpad.woff?2)`)
	assert.Contains(t, got, `url(../../font/B.otf)`)
	assert.Contains(t, got, `url(data:image/svg+xml;base64,AA==)`, "data URLs stay")
	assert.Contains(t, string(rebaseFontURLs([]byte(in), "/css/skin/colibris")), `url("../../../font/A.ttf")`)
}

// The dev mode bundles (devMode) get font URLs that reach assets/font from
// where they are served, also under a proxy prefix.
func TestDevCSSFontURLs(t *testing.T) {
	root := "../../.."
	urlRe := regexp.MustCompile(`url\(["']?([^"')]+)["']?\)`)
	for _, b := range devCSSBundles {
		css, err := buildDevCSS(root, b)
		require.NoError(t, err, b.entry)
		fonts := 0
		for _, m := range urlRe.FindAllStringSubmatch(string(css), -1) {
			u := m[1]
			if strings.HasPrefix(u, "data:") || !regexp.MustCompile(`\.(ttf|otf|woff2?|eot)([?#]|$)`).MatchString(u) {
				continue
			}
			fonts++
			clean := regexp.MustCompile(`[?#].*$`).ReplaceAllString(u, "")
			served := path.Join(b.servedAt, clean)
			assert.Equal(t, "/sub"+served, path.Join("/sub"+b.servedAt, clean), "%s: %s leaves the prefix", b.entry, u)
			_, err := os.Stat(path.Join(root, "assets", served))
			assert.NoError(t, err, "%s: %s", b.entry, u)
		}
		if b.servedAt == "/css/static" {
			assert.Greater(t, fonts, 5, "the pad stylesheet names its fonts")
		}
	}
}
