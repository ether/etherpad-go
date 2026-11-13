package io

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
)

// Port of etherpad-lite's export_list spec: HTML import, then HTML export.
func TestExportListParity(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(testutils.TestRunConfig{Name: "lists", Test: testExportLists})
	testDb.StartTestDBHandler()
}

func listRoundTrip(t *testing.T, ds testutils.TestDataStore, padId, body string) string {
	t.Helper()
	importHTML(t, ds, padId, "<html><body>"+body+"</body></html>")
	return exportHTMLOf(t, ds, padId)
}

func testExportLists(t *testing.T, ds testutils.TestDataStore) {
	t.Run("indent lines export with list-style-type:none", func(t *testing.T) {
		html := listRoundTrip(t, ds, "listIndent", `<ul class="indent"><li>indented line 1</li><li>indented line 2</li></ul>`)
		assert.Contains(t, html, `class="indent"`)
		assert.Contains(t, html, "list-style-type")
	})
	t.Run("numbering continues across a bullet (explicit start)", func(t *testing.T) {
		html := listRoundTrip(t, ds, "listStart", `<ol class="number"><li>First</li></ol><ul class="bullet"><li>Bullet A</li></ul><ol start="2" class="number"><li>Second</li></ol>`)
		assert.Contains(t, html, `start="2"`)
	})
	t.Run("numbering continues across a bullet (no start)", func(t *testing.T) {
		html := listRoundTrip(t, ds, "listNoStart", `<ol class="number"><li>First</li></ol><ul class="bullet"><li>Bullet A</li></ul><ol class="number"><li>Second</li></ol>`)
		assert.Contains(t, html, `start="2"`)
	})
	t.Run("numbering continues with several items", func(t *testing.T) {
		html := listRoundTrip(t, ds, "listMulti", `<ol class="number"><li>First</li><li>Second</li></ol><ul class="bullet"><li>Bullet</li></ul><ol class="number"><li>Third</li></ol>`)
		assert.Contains(t, html, `start="3"`)
	})
	t.Run("nested counters reset when a level closes", func(t *testing.T) {
		html := listRoundTrip(t, ds, "listNested", `<ol class="number"><li>Parent A<ol class="number"><li>Child A1</li><li>Child A2</li></ol></li><li>Parent B<ol class="number"><li>Child B1</li></ol></li></ol>`)
		ols := regexp.MustCompile(`<ol[^>]*class="number"[^>]*>`).FindAllString(html, -1)
		if assert.GreaterOrEqual(t, len(ols), 3, html) {
			assert.False(t, strings.Contains(ols[len(ols)-1], `start="3"`), html)
		}
	})
}
