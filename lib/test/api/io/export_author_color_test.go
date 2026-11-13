package io

import (
	"strings"
	"testing"

	libio "github.com/ether/etherpad-go/lib/io"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// etherpad-lite exportHtmlAuthorColorXss, with the colour in the database
// (the import cases are TestSanitizeImportedColorId in lib/io).
func TestExportAuthorColorXss(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "a malformed colour in the database is dropped", Test: testExportDropsMalformedColor},
		testutils.TestRunConfig{Name: "a valid colour is exported", Test: testExportKeepsValidColor},
	)
	testDb.StartTestDBHandler()
}

const colorPayload = `red}</style><script>alert(1)</script><style>{`

func exportHTMLOf(t *testing.T, ds testutils.TestDataStore, padId string) string {
	t.Helper()
	html, err := libio.NewExportHtml(ds.PadManager, ds.AuthorManager, ds.Hooks).GetPadHTMLDocument(padId, nil, nil)
	require.NoError(t, err)
	return html
}

func testExportDropsMalformedColor(t *testing.T, ds testutils.TestDataStore) {
	authorId := createPadBy(t, ds, "colorXssPad")
	// A record from before import validation.
	require.NoError(t, ds.AuthorManager.SetAuthorColor(authorId, colorPayload))
	html := exportHTMLOf(t, ds, "colorXssPad")
	assert.NotContains(t, html, "<script>")
	assert.NotContains(t, html, colorPayload)
	assert.Contains(t, html, "original")
}

func testExportKeepsValidColor(t *testing.T, ds testutils.TestDataStore) {
	authorId := createPadBy(t, ds, "colorValidPad")
	require.NoError(t, ds.AuthorManager.SetAuthorColor(authorId, "#abc123"))
	html := exportHTMLOf(t, ds, "colorValidPad")
	cls := "author" + strings.ReplaceAll(authorId, ".", "_")
	assert.Contains(t, html, "."+cls+" {background-color: #abc123}")
	assert.Contains(t, html, `<span class="`+cls+`">original</span>`)
}
