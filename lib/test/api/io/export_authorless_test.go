package io

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/require"
)

// Pads whose history names authors the export cannot resolve: the export
// must still carry the pad (etherpad.json in the PDF, the .etherpad file).
func TestExportAuthorlessHistory(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)
	testDb.AddTests(
		testutils.TestRunConfig{Name: "someone only chats", Test: testExportChatOnlyAuthor},
		testutils.TestRunConfig{Name: "chat of an erased author", Test: testExportErasedChatAuthor},
		testutils.TestRunConfig{Name: "text of an author without a record", Test: testExportUnknownAuthor},
		testutils.TestRunConfig{Name: "content replaced without an author", Test: testExportReplacedWithoutAuthor},
	)
	defer testDb.StartTestDBHandler()
}

func exportPdfAndPad(t *testing.T, ds testutils.TestDataStore, padId, token string) {
	t.Helper()
	app := setupExportApp(ds)
	for _, format := range []string{"etherpad", "pdf"} {
		req := httptest.NewRequest("GET", "/p/"+padId+"/export/"+format, nil)
		req.AddCookie(&http.Cookie{Name: "token", Value: token})
		resp, err := app.Test(req)
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		require.Equal(t, 200, resp.StatusCode, "%s: %s", format, string(body))
		if format == "pdf" {
			_, err := ds.Importer.ExtractEtherpadFromPdf(body)
			require.NoError(t, err, "the PDF has the pad attached")
		}
	}
}

func testExportChatOnlyAuthor(t *testing.T, ds testutils.TestDataStore) {
	padId := "chatOnlyPad"
	token := createPadWithPlainText(t, ds, padId, "hello")
	chatter, err := ds.AuthorManager.CreateAuthor(nil)
	require.NoError(t, err)
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	_, err = p.AppendChatMessage(&chatter.Id, 1, "I only chat")
	require.NoError(t, err)
	exportPdfAndPad(t, ds, padId, token)
}

func testExportErasedChatAuthor(t *testing.T, ds testutils.TestDataStore) {
	padId := "erasedChatPad"
	token := createPadWithPlainText(t, ds, padId, "hello")
	writer, err := ds.AuthorManager.GetAuthorId(token)
	require.NoError(t, err)
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	_, err = p.AppendChatMessage(&writer.Id, 1, "soon anonymous")
	require.NoError(t, err)
	require.NoError(t, ds.AuthorManager.AnonymizeAuthor(writer.Id))
	exportPdfAndPad(t, ds, padId, "")
}

// The pool names an author the database does not know (an .etherpad file
// without its globalAuthor records, say).
func testExportUnknownAuthor(t *testing.T, ds testutils.TestDataStore) {
	padId := "unknownAuthorPad"
	token := createPadWithPlainText(t, ds, padId, "hello")
	src := apool.NewAPool()
	ghost := src.PutAttrib(apool.Attribute{Key: "author", Value: "a.ghost00000000000"}, nil)
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	_, err = p.ReplaceContent(buildAText([]pdfLine{{text: "written by nobody we know", attribs: []int{ghost}}}), &src, nil)
	require.NoError(t, err)
	exportPdfAndPad(t, ds, padId, token)
}

func testExportReplacedWithoutAuthor(t *testing.T, ds testutils.TestDataStore) {
	padId := "replacedPad"
	token := createPadWithPlainText(t, ds, padId, "hello")
	src := apool.NewAPool()
	p, err := ds.PadManager.GetPad(padId, nil, nil)
	require.NoError(t, err)
	_, err = p.ReplaceContent(buildAText([]pdfLine{{text: "replaced"}}), &src, nil)
	require.NoError(t, err)
	exportPdfAndPad(t, ds, padId, token)
}
