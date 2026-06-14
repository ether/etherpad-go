package io

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	apiio "github.com/ether/etherpad-go/lib/api/io"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
)

func TestImportHooks(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)

	testDb.AddTests(
		testutils.TestRunConfig{
			Name: "Hook import handles custom format via SetText",
			Test: testImportHookCustomFormat,
		},
		testutils.TestRunConfig{
			Name: "Hook importEtherpad observes data on etherpad import",
			Test: testImportEtherpadHookObservesData,
		},
		testutils.TestRunConfig{
			Name: "Large text import is not truncated or NUL-padded",
			Test: testImportLargeTextNoTruncation,
		},
	)
	defer testDb.StartTestDBHandler()
}

// buildImportMultipartBody creates a multipart/form-data body with a single "file" field.
func buildImportMultipartBody(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	assert.NoError(t, err)
	_, err = part.Write(content)
	assert.NoError(t, err)
	err = writer.Close()
	assert.NoError(t, err)
	return body, writer.FormDataContentType()
}

// testImportHookCustomFormat registers an import hook that handles ".custom" files
// by calling SetText; drives an import of a .custom file; asserts pad text matches.
func testImportHookCustomFormat(t *testing.T, tsStore testutils.TestDataStore) {
	padId := "importHookCustomPad"
	token := createTestAuthorWithToken(t, tsStore)

	// Register hook: handle ".custom" by injecting text.
	hookId := tsStore.Hooks.EnqueueImportHook(func(ctx *events.ImportContext) {
		if ctx.FileEnding == ".custom" {
			ctx.SetText("hello from plugin")
		}
	})
	defer tsStore.Hooks.DequeueHook(hooks.ImportString, hookId)

	// Set up app with import routes.
	tsInstance := tsStore.ToInitStore()
	apiio.Init(tsInstance)
	app := tsInstance.C

	// Build multipart request with a ".custom" file.
	fileContent := []byte("raw custom content that plugin ignores")
	body, ct := buildImportMultipartBody(t, "test.custom", fileContent)

	req := httptest.NewRequest("POST", "/p/"+padId+"/import", body)
	req.Header.Set("Content-Type", ct)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})

	resp, err := app.Test(req)
	assert.NoError(t, err)

	respBody, _ := io.ReadAll(resp.Body)
	t.Logf("Import response: status=%d body=%s", resp.StatusCode, string(respBody))
	assert.Equal(t, 200, resp.StatusCode, "import of .custom file via hook should succeed")

	// Verify pad text was set to "hello from plugin\n" (importText appends \n).
	retrievedPad, err := tsStore.PadManager.GetPad(padId, nil, nil)
	assert.NoError(t, err)
	assert.Contains(t, retrievedPad.Text(), "hello from plugin",
		"pad text should contain the text injected by the import hook")
}

// testImportEtherpadHookObservesData registers an importEtherpad hook that records
// it fired; drives an import of a minimal .etherpad file; asserts hook fired with
// correct padId and non-empty Data.
func testImportEtherpadHookObservesData(t *testing.T, tsStore testutils.TestDataStore) {
	padId := "importEtherpadHookPad"
	token := createTestAuthorWithToken(t, tsStore)

	var hookFired atomic.Bool
	var capturedPadId string
	var capturedDataLen int

	hookId := tsStore.Hooks.EnqueueImportEtherpadHook(func(ctx *events.ImportEtherpadContext) {
		hookFired.Store(true)
		capturedPadId = ctx.PadId
		capturedDataLen = len(ctx.Data)
	})
	defer tsStore.Hooks.DequeueHook(hooks.ImportEtherpadString, hookId)

	// Build a minimal valid .etherpad JSON.
	srcPadName := "srcPad"
	etherpadJSON := buildMinimalEtherpadJSON(t, srcPadName)

	tsInstance := tsStore.ToInitStore()
	apiio.Init(tsInstance)
	app := tsInstance.C

	body, ct := buildImportMultipartBody(t, "export.etherpad", etherpadJSON)

	req := httptest.NewRequest("POST", "/p/"+padId+"/import", body)
	req.Header.Set("Content-Type", ct)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})

	resp, err := app.Test(req)
	assert.NoError(t, err)

	respBody, _ := io.ReadAll(resp.Body)
	t.Logf("ImportEtherpad response: status=%d body=%s", resp.StatusCode, string(respBody))
	assert.Equal(t, 200, resp.StatusCode, "etherpad import should succeed")

	assert.True(t, hookFired.Load(), "importEtherpad hook must have fired")
	assert.Equal(t, padId, capturedPadId, "hook must receive the destination padId")
	assert.Greater(t, capturedDataLen, 0, "hook Data map must be non-empty")
}

// testImportLargeTextNoTruncation guards the doImport file-read: a single
// file.Read is not guaranteed to fill the buffer, so a sizable upload could end
// up truncated or NUL-padded. Import a large .txt and assert both ends survive
// and no NUL bytes leak in from an under-filled read buffer.
func testImportLargeTextNoTruncation(t *testing.T, tsStore testutils.TestDataStore) {
	padId := "importLargeTextPad"
	token := createTestAuthorWithToken(t, tsStore)

	tsInstance := tsStore.ToInitStore()
	apiio.Init(tsInstance)
	app := tsInstance.C

	const endMarker = "END-OF-IMPORT-MARKER"
	content := "START-OF-IMPORT-MARKER\n" +
		strings.Repeat("etherpad import payload line\n", 1024) +
		endMarker // ~28 KB: large enough to plausibly span reads, under MySQL's 64 KB TEXT limit

	body, ct := buildImportMultipartBody(t, "big.txt", []byte(content))
	req := httptest.NewRequest("POST", "/p/"+padId+"/import", body)
	req.Header.Set("Content-Type", ct)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})

	resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second})
	assert.NoError(t, err)
	respBody, _ := io.ReadAll(resp.Body)
	assert.Equal(t, 200, resp.StatusCode, "large .txt import should succeed: %s", string(respBody))

	retrievedPad, err := tsStore.PadManager.GetPad(padId, nil, nil)
	assert.NoError(t, err)
	text := retrievedPad.Text()
	assert.Contains(t, text, "START-OF-IMPORT-MARKER", "start of imported content must be present")
	assert.Contains(t, text, endMarker, "end of imported content must be present (no truncation)")
	assert.NotContains(t, text, "\x00", "imported content must not contain NUL bytes from an under-filled read buffer")
}

// buildMinimalEtherpadJSON constructs a minimal but valid .etherpad export JSON.
func buildMinimalEtherpadJSON(t *testing.T, srcPadName string) []byte {
	t.Helper()

	padKey := fmt.Sprintf("pad:%s:", srcPadName)
	rev0Key := fmt.Sprintf("pad:%s:revs:0", srcPadName)

	padData := map[string]any{
		"atext": map[string]any{
			"text":    "Hello etherpad\n",
			"attribs": "*0*1*2+g|1+1",
		},
		"pool": map[string]any{
			"numToAttrib": map[string]any{
				"0": []string{"author", "a.test"},
				"1": []string{"bold", ""},
				"2": []string{"insertorder", "first"},
			},
			"nextNum": 3,
		},
		"head":           0,
		"chatHead":       0,
		"publicStatus":   false,
		"savedRevisions": []any{},
	}

	rev0 := map[string]any{
		"changeset": "Z:1>g|1+g$Hello etherpad\n",
		"meta": map[string]any{
			"author":    "a.test",
			"timestamp": 1700000000000,
		},
	}

	export := map[string]any{
		padKey:  padData,
		rev0Key: rev0,
	}

	data, err := json.Marshal(export)
	assert.NoError(t, err)
	return data
}
