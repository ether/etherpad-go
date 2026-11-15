package io

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/ledongthuc/pdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportPdfLayout(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{
		Name: "PDF export wraps long lines, links URLs and uses monospace",
		Test: testExportPdfLayout,
	})
	defer testDb.StartTestDBHandler()
}

// pdfLine is one pad line for buildAText: its text and the pool numbers of
// its attributes; a line marker ("*") with marker attributes comes first.
type pdfLine struct {
	text    string
	attribs []int
	marker  []int
}

func attribRefs(nums []int) string {
	var sb strings.Builder
	for _, n := range nums {
		sb.WriteString("*" + strconv.FormatInt(int64(n), 36))
	}
	return sb.String()
}

func buildAText(lines []pdfLine) apool.AText {
	var text, attribs strings.Builder
	for _, l := range lines {
		if l.marker != nil {
			text.WriteString("*")
			attribs.WriteString(attribRefs(l.marker) + "+1")
		}
		text.WriteString(l.text + "\n")
		attribs.WriteString(attribRefs(l.attribs) + "+" + strconv.FormatInt(int64(utf8.RuneCountInString(l.text)), 36))
		attribs.WriteString("|1+1")
	}
	return apool.AText{Text: text.String(), Attribs: attribs.String()}
}

func testExportPdfLayout(t *testing.T, ds testutils.TestDataStore) {
	app := setupExportApp(ds)
	padId := "pdfLayoutPad"
	token := createPadWithPlainText(t, ds, padId, "x")

	src := apool.NewAPool()
	mono := src.PutAttrib(apool.Attribute{Key: "font-family", Value: "courier"}, nil)
	lmkr := src.PutAttrib(apool.Attribute{Key: "lmkr", Value: "1"}, nil)
	list := src.PutAttrib(apool.Attribute{Key: "list", Value: "bullet1"}, nil)
	order := src.PutAttrib(apool.Attribute{Key: "insertorder", Value: "first"}, nil)
	number := src.PutAttrib(apool.Attribute{Key: "list", Value: "number1"}, nil)

	long := strings.TrimSpace(strings.Repeat("lorem ipsum dolor ", 40))
	atext := buildAText([]pdfLine{
		{text: "Visit https://example.com/x now"},
		{text: long},
		{text: "monospaced", attribs: []int{mono}},
		{text: "item " + long, marker: []int{lmkr, list, order}},
		{text: "first", marker: []int{lmkr, number, order}},
		{text: "", marker: []int{lmkr, number, order}},
		{text: "third", marker: []int{lmkr, number, order}},
	})

	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	_, err = p.ReplaceContent(atext, &src, nil)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/pdf", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode, string(body))
	if d := os.Getenv("EP_PDF_DUMP"); d != "" {
		_ = os.WriteFile(d, body, 0o644)
	}

	// The URL is a link annotation.
	assert.True(t, bytes.Contains(body, []byte("https://example.com/x")), "PDF has the link's URI")
	assert.True(t, bytes.Contains(body, []byte("/URI")), "PDF has a URI action")

	r, err := pdf.NewReader(bytes.NewReader(body), int64(len(body)))
	require.NoError(t, err)

	const pageWidth, margin = 595.28, 40.0
	lines := map[float64]bool{}
	fonts := map[string]bool{}
	var all strings.Builder
	for n := 1; n <= r.NumPage(); n++ {
		page := r.Page(n)
		if page.V.IsNull() {
			continue
		}
		for _, ch := range page.Content().Text {
			if strings.TrimSpace(ch.S) == "" {
				continue
			}
			all.WriteString(ch.S)
			fonts[ch.Font] = true
			lines[float64(n)*10000+ch.Y] = true
			assert.GreaterOrEqual(t, ch.X, margin-0.5, "%q starts left of the margin", ch.S)
			assert.LessOrEqual(t, ch.X+ch.W, pageWidth-margin+0.5, "%q runs past the right margin", ch.S)
		}
	}

	// All the text arrived, and the two long lines took many lines.
	text := all.String()
	assert.Equal(t, 2*strings.Count(long, "lorem"), strings.Count(text, "lorem"))
	assert.Contains(t, text, "monospaced")
	// The empty list item keeps its number.
	assert.Regexp(t, `1\.\W*first\W*2\.\W*3\.\W*third`, text)
	assert.Greater(t, len(lines), 10, "long lines wrap")

	var monoFont bool
	for f := range fonts {
		if strings.Contains(f, "Mono") {
			monoFont = true
		}
	}
	assert.True(t, monoFont, "monospace text uses RobotoMono, fonts: %v", fonts)
}
