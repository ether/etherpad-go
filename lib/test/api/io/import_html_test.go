package io

import (
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/changeset"
	libio "github.com/ether/etherpad-go/lib/io"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HTML import keeps formatting (it used to import plain text only, for
// file imports and the setHTML API alike).
func TestImportHTMLFormatting(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "inline formatting, lists, headings, alignment", Test: testImportHTMLFormatting},
		testutils.TestRunConfig{Name: "HTML export then import gives the same pad", Test: testImportHTMLRoundTrip},
		testutils.TestRunConfig{Name: "line breaks, whitespace and skipped elements", Test: testImportHTMLText},
	)
	testDb.StartTestDBHandler()
}

// lineAttrs describes each line of the pad as "text [attr=value ...]",
// with the inline attributes of its runs.
func describePad(t *testing.T, ds testutils.TestDataStore, padId string) []string {
	t.Helper()
	ds.PadManager.UnloadPad(padId)
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	require.NoError(t, p.Check())
	lines := strings.Split(strings.TrimSuffix(p.AText.Text, "\n"), "\n")
	alines, err := changeset.SplitAttributionLines(p.AText.Attribs, p.AText.Text)
	require.NoError(t, err)
	var out []string
	for i, line := range lines {
		ops, err := changeset.DeserializeOps(alines[i])
		require.NoError(t, err)
		var desc []string
		pos := 0
		runes := []rune(line + "\n")
		for _, op := range *ops {
			m := changeset.FromString(op.Attribs, &p.Pool)
			var attrs []string
			for k, v := range m.Iter() {
				if k == "author" || k == "insertorder" || k == "lmkr" {
					continue
				}
				attrs = append(attrs, k+"="+v)
			}
			seg := strings.TrimSuffix(string(runes[pos:pos+op.Chars]), "\n")
			pos += op.Chars
			if seg == "" {
				continue
			}
			sortStrings(attrs)
			if len(attrs) > 0 {
				desc = append(desc, seg+"{"+strings.Join(attrs, ",")+"}")
			} else {
				desc = append(desc, seg)
			}
		}
		out = append(out, strings.Join(desc, ""))
	}
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func importHTML(t *testing.T, ds testutils.TestDataStore, padId, src string) {
	t.Helper()
	a, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	p, err := ds.PadManager.GetPad(padId, nil, &a.Id)
	require.NoError(t, err)
	require.NoError(t, ds.Importer.SetPadHTML(p, src, a.Id))
}

func testImportHTMLFormatting(t *testing.T, ds testutils.TestDataStore) {
	importHTML(t, ds, "htmlFormatting", `<html><body>
<h1>Title</h1>
<p>plain <b>bold</b> <i>italic</i> <u>under</u> <s>gone</s> <strong><em>both</em></strong></p>
<ul class="bullet"><li>one</li><li>two<ul class="bullet"><li>nested</li></ul></li></ul>
<ol class="number"><li>first</li><li>second</li></ol>
<ul class="indent"><li>indented</li></ul>
<p style="text-align: center">centered</p>
<h2 style="text-align:right">Right heading</h2>
</body></html>`)
	assert.Equal(t, []string{
		"*{heading=h1}Title",
		"plain bold{bold=true} italic{italic=true} under{underline=true} gone{strikethrough=true} both{bold=true,italic=true}",
		"*{list=bullet1}one",
		"*{list=bullet1}two",
		"*{list=bullet2}nested",
		"*{list=number1}first",
		"*{list=number1}second",
		"*{list=indent1}indented",
		"*{align=center}centered",
		"*{align=right,heading=h2}Right heading",
	}, describePad(t, ds, "htmlFormatting"))
}

func testImportHTMLRoundTrip(t *testing.T, ds testutils.TestDataStore) {
	// Core formatting only: headings and alignment are exported by the
	// ep_heading and ep_align plugins.
	src := `<p>Title</p><p>a <b>bold</b> word</p><ul class="bullet"><li>x<ul class="bullet"><li>y</li></ul></li></ul><ol><li>n</li></ol><p></p><p>end</p>`
	importHTML(t, ds, "htmlRoundTripA", src)
	p, err := ds.PadManager.GetPad("htmlRoundTripA", nil, nil)
	require.NoError(t, err)
	html, err := (&libio.ExportHtml{PadManager: ds.PadManager, AuthorManager: ds.AuthorManager, Hooks: ds.Hooks}).GetPadHTML(p, nil, map[string]string{})
	require.NoError(t, err)
	importHTML(t, ds, "htmlRoundTripB", html)
	assert.Equal(t, describePad(t, ds, "htmlRoundTripA"), describePad(t, ds, "htmlRoundTripB"), html)
}

func testImportHTMLText(t *testing.T, ds testutils.TestDataStore) {
	importHTML(t, ds, "htmlText", `<html><head><title>no</title><style>p{}</style><script>alert(1)</script></head>
<body>first   line<br>second<br><br>after empty<pre>keep   this
two</pre>  trailing   </body></html>`)
	assert.Equal(t, []string{"first line", "second", "", "after empty", "keep   this", "two", "trailing"}, describePad(t, ds, "htmlText"))
}
