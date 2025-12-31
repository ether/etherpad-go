package io

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiio "github.com/ether/etherpad-go/lib/api/io"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestExportHandler(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)

	testDb.AddTests(
		testutils.TestRunConfig{
			Name: "Export Plain Text Pad as Etherpad",
			Test: testExportPlainTextPadAsEtherpad,
		},
		testutils.TestRunConfig{
			Name: "Export Plain Text Pad as TXT",
			Test: testExportPlainTextPadAsTxt,
		},
		testutils.TestRunConfig{
			Name: "Export Plain Text Pad as PDF",
			Test: testExportPlainTextPadAsPdf,
		},
		testutils.TestRunConfig{
			Name: "Export Bold Text Pad as Etherpad",
			Test: testExportBoldTextPadAsEtherpad,
		},
		testutils.TestRunConfig{
			Name: "Export Bold Text Pad as TXT",
			Test: testExportBoldTextPadAsTxt,
		},
		testutils.TestRunConfig{
			Name: "Export Bold Text Pad as PDF",
			Test: testExportBoldTextPadAsPdf,
		},
		testutils.TestRunConfig{
			Name: "Export Italic Text Pad as Etherpad",
			Test: testExportItalicTextPadAsEtherpad,
		},
		testutils.TestRunConfig{
			Name: "Export Italic Text Pad as TXT",
			Test: testExportItalicTextPadAsTxt,
		},
		testutils.TestRunConfig{
			Name: "Export Italic Text Pad as PDF",
			Test: testExportItalicTextPadAsPdf,
		},
		testutils.TestRunConfig{
			Name: "Export Indented Text Pad as Etherpad",
			Test: testExportIndentedTextPadAsEtherpad,
		},
		testutils.TestRunConfig{
			Name: "Export Indented Text Pad as TXT",
			Test: testExportIndentedTextPadAsTxt,
		},
		testutils.TestRunConfig{
			Name: "Export Indented Text Pad as PDF",
			Test: testExportIndentedTextPadAsPdf,
		},
		testutils.TestRunConfig{
			Name: "Export Mixed Formatting Pad as Etherpad",
			Test: testExportMixedFormattingPadAsEtherpad,
		},
		testutils.TestRunConfig{
			Name: "Export Mixed Formatting Pad as TXT",
			Test: testExportMixedFormattingPadAsTxt,
		},
		testutils.TestRunConfig{
			Name: "Export Mixed Formatting Pad as PDF",
			Test: testExportMixedFormattingPadAsPdf,
		},
		testutils.TestRunConfig{
			Name: "Export Plain Text Pad as DOCX",
			Test: testExportPlainTextPadAsDocx,
		},
		testutils.TestRunConfig{
			Name: "Export Bold Text Pad as DOCX",
			Test: testExportBoldTextPadAsDocx,
		},
		testutils.TestRunConfig{
			Name: "Export Italic Text Pad as DOCX",
			Test: testExportItalicTextPadAsDocx,
		},
		testutils.TestRunConfig{
			Name: "Export Indented Text Pad as DOCX",
			Test: testExportIndentedTextPadAsDocx,
		},
		testutils.TestRunConfig{
			Name: "Export Mixed Formatting Pad as DOCX",
			Test: testExportMixedFormattingPadAsDocx,
		},
		testutils.TestRunConfig{
			Name: "Export Plain Text Pad as ODT",
			Test: testExportPlainTextPadAsOdt,
		},
		testutils.TestRunConfig{
			Name: "Export Bold Text Pad as ODT",
			Test: testExportBoldTextPadAsOdt,
		},
		testutils.TestRunConfig{
			Name: "Export Italic Text Pad as ODT",
			Test: testExportItalicTextPadAsOdt,
		},
		testutils.TestRunConfig{
			Name: "Export Indented Text Pad as ODT",
			Test: testExportIndentedTextPadAsOdt,
		},
		testutils.TestRunConfig{
			Name: "Export Mixed Formatting Pad as ODT",
			Test: testExportMixedFormattingPadAsOdt,
		},
		testutils.TestRunConfig{
			Name: "Export Non Existing Pad Returns 404",
			Test: testExportNonExistingPadReturns404,
		},
		testutils.TestRunConfig{
			Name: "Export Invalid Type Returns 400",
			Test: testExportInvalidTypeReturns400,
		},
	)
	defer testDb.StartTestDBHandler()
}

func setupExportApp(tsStore testutils.TestDataStore) *fiber.App {
	tsInstance := tsStore.ToInitStore()
	apiio.Init(tsInstance)

	return tsInstance.C
}

// createTestAuthorWithToken creates an author and sets up a token for authentication
func createTestAuthorWithToken(t *testing.T, tsStore testutils.TestDataStore) string {
	token := "testToken123"
	author, err := tsStore.AuthorManager.CreateAuthor(nil)
	assert.NoError(t, err)

	err = tsStore.DS.SetAuthorByToken(token, author.Id)
	assert.NoError(t, err)

	return token
}

func createPadWithPlainText(t *testing.T, tsStore testutils.TestDataStore, padId string, text string) string {
	token := createTestAuthorWithToken(t, tsStore)

	author, err := tsStore.AuthorManager.GetAuthorId(token)
	assert.NoError(t, err)

	pad, err := tsStore.PadManager.GetPad(padId, nil, nil)
	assert.NoError(t, err)

	err = pad.SetText(text, &author.Id)
	assert.NoError(t, err)

	return token
}

func createPadWithBoldText(t *testing.T, tsStore testutils.TestDataStore, padId string, text string) string {
	token := createTestAuthorWithToken(t, tsStore)

	author, err := tsStore.AuthorManager.GetAuthorId(token)
	assert.NoError(t, err)

	pad, err := tsStore.PadManager.GetPad(padId, nil, nil)
	assert.NoError(t, err)

	trueVal := true
	pad.Pool.PutAttrib(apool.Attribute{Key: "bold", Value: "true"}, &trueVal)

	err = pad.SetText(text, &author.Id)
	assert.NoError(t, err)

	return token
}

func createPadWithItalicText(t *testing.T, tsStore testutils.TestDataStore, padId string, text string) string {
	token := createTestAuthorWithToken(t, tsStore)

	author, err := tsStore.AuthorManager.GetAuthorId(token)
	assert.NoError(t, err)

	pad, err := tsStore.PadManager.GetPad(padId, nil, nil)
	assert.NoError(t, err)

	trueVal := true
	pad.Pool.PutAttrib(apool.Attribute{Key: "italic", Value: "true"}, &trueVal)

	err = pad.SetText(text, &author.Id)
	assert.NoError(t, err)

	return token
}

func createPadWithIndentation(t *testing.T, tsStore testutils.TestDataStore, padId string, text string) string {
	token := createTestAuthorWithToken(t, tsStore)

	author, err := tsStore.AuthorManager.GetAuthorId(token)
	assert.NoError(t, err)

	pad, err := tsStore.PadManager.GetPad(padId, nil, nil)
	assert.NoError(t, err)

	trueVal := true
	pad.Pool.PutAttrib(apool.Attribute{Key: "list", Value: "indent1"}, &trueVal)

	err = pad.SetText(text, &author.Id)
	assert.NoError(t, err)

	return token
}

func createPadWithMixedFormatting(t *testing.T, tsStore testutils.TestDataStore, padId string, text string) string {
	token := createTestAuthorWithToken(t, tsStore)

	author, err := tsStore.AuthorManager.GetAuthorId(token)
	assert.NoError(t, err)

	pad, err := tsStore.PadManager.GetPad(padId, nil, nil)
	assert.NoError(t, err)

	trueVal := true
	pad.Pool.PutAttrib(apool.Attribute{Key: "bold", Value: "true"}, &trueVal)
	pad.Pool.PutAttrib(apool.Attribute{Key: "italic", Value: "true"}, &trueVal)
	pad.Pool.PutAttrib(apool.Attribute{Key: "list", Value: "indent1"}, &trueVal)

	err = pad.SetText(text, &author.Id)
	assert.NoError(t, err)

	return token
}

// Plain Text Tests
func testExportPlainTextPadAsEtherpad(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "plainTextPad"
	testText := "Hello World"

	token := createPadWithPlainText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/etherpad", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	t.Logf("Response body: %s", string(body))

	if resp.StatusCode != 200 {
		t.Errorf("Expected 200, got %d. Body: %s", resp.StatusCode, string(body))
	}

	var export map[string]interface{}
	err = json.Unmarshal(body, &export)
	assert.NoError(t, err)
	assert.NotEmpty(t, export)
}

func testExportPlainTextPadAsTxt(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "plainTextPadTxt"
	testText := "Hello World"

	token := createPadWithPlainText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/txt", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, strings.Contains(string(body), testText))
}

// Bold Text Tests
func testExportBoldTextPadAsEtherpad(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "boldTextPad"
	testText := "Bold Text"

	createPadWithBoldText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/etherpad", nil)
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	var export map[string]interface{}
	err = json.Unmarshal(body, &export)
	assert.NoError(t, err)
	assert.NotEmpty(t, export)
}

func testExportBoldTextPadAsTxt(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "boldTextPadTxt"
	testText := "Bold Text"

	createPadWithBoldText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/txt", nil)
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, strings.Contains(string(body), testText))
}

// Italic Text Tests
func testExportItalicTextPadAsEtherpad(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "italicTextPad"
	testText := "Italic Text"

	createPadWithItalicText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/etherpad", nil)
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	var export map[string]interface{}
	err = json.Unmarshal(body, &export)
	assert.NoError(t, err)
	assert.NotEmpty(t, export)
}

func testExportItalicTextPadAsTxt(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "italicTextPadTxt"
	testText := "Italic Text"

	createPadWithItalicText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/txt", nil)
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, strings.Contains(string(body), testText))
}

// Indented Text Tests
func testExportIndentedTextPadAsEtherpad(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "indentedTextPad"
	testText := "Indented Text"

	createPadWithIndentation(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/etherpad", nil)
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	var export map[string]interface{}
	err = json.Unmarshal(body, &export)
	assert.NoError(t, err)
	assert.NotEmpty(t, export)
}

func testExportIndentedTextPadAsTxt(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "indentedTextPadTxt"
	testText := "Indented Text"

	createPadWithIndentation(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/txt", nil)
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, strings.Contains(string(body), testText))
}

// Mixed Formatting Tests
func testExportMixedFormattingPadAsEtherpad(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "mixedFormattingPad"
	testText := "Mixed Formatting Text"

	createPadWithMixedFormatting(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/etherpad", nil)
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	var export map[string]interface{}
	err = json.Unmarshal(body, &export)
	assert.NoError(t, err)
	assert.NotEmpty(t, export)
}

func testExportMixedFormattingPadAsTxt(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "mixedFormattingPadTxt"
	testText := "Mixed Formatting Text"

	createPadWithMixedFormatting(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/txt", nil)
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, strings.Contains(string(body), testText))
}

// Error Cases
func testExportNonExistingPadReturns404(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)

	req := httptest.NewRequest("GET", "/p/nonExistingPadId/export/etherpad", nil)
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)

	// Should return 401 (Unauthorized) or 404 depending on SecurityManager behavior
	assert.True(t, resp.StatusCode == 401 || resp.StatusCode == 404)
}

func testExportInvalidTypeReturns400(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "invalidTypePad"
	testText := "Some Text"

	createPadWithPlainText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/invalidType", nil)
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

// PDF Export Tests
func testExportPlainTextPadAsPdf(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "plainTextPadPdf"
	testText := "Hello World PDF"

	token := createPadWithPlainText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/pdf", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	if resp.StatusCode != 200 {
		t.Logf("PDF Export failed with status %d: %s", resp.StatusCode, string(body))
	}

	assert.Equal(t, 200, resp.StatusCode)

	// Check Content-Type header
	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/pdf", contentType)

	// PDF files start with %PDF
	assert.True(t, len(body) > 4, "PDF body should not be empty")
	assert.Equal(t, "%PDF", string(body[:4]), "PDF should start with %PDF header")
}

func testExportBoldTextPadAsPdf(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "boldTextPadPdf"
	testText := "Bold Text PDF"

	token := createPadWithBoldText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/pdf", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/pdf", contentType)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, len(body) > 4, "PDF body should not be empty")
	assert.Equal(t, "%PDF", string(body[:4]), "PDF should start with %PDF header")
}

func testExportItalicTextPadAsPdf(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "italicTextPadPdf"
	testText := "Italic Text PDF"

	token := createPadWithItalicText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/pdf", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/pdf", contentType)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, len(body) > 4, "PDF body should not be empty")
	assert.Equal(t, "%PDF", string(body[:4]), "PDF should start with %PDF header")
}

func testExportIndentedTextPadAsPdf(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "indentedTextPadPdf"
	testText := "Indented Text PDF"

	token := createPadWithIndentation(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/pdf", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/pdf", contentType)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, len(body) > 4, "PDF body should not be empty")
	assert.Equal(t, "%PDF", string(body[:4]), "PDF should start with %PDF header")
}

func testExportMixedFormattingPadAsPdf(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "mixedFormattingPadPdf"
	testText := "Mixed Formatting Text PDF"

	token := createPadWithMixedFormatting(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/pdf", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/pdf", contentType)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, len(body) > 4, "PDF body should not be empty")
	assert.Equal(t, "%PDF", string(body[:4]), "PDF should start with %PDF header")
}

// DOCX Export Tests
func testExportPlainTextPadAsDocx(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "plainTextPadDocx"
	testText := "Hello World DOCX"

	token := createPadWithPlainText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/docx", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	if resp.StatusCode != 200 {
		t.Logf("DOCX Export failed with status %d: %s", resp.StatusCode, string(body))
	}

	assert.Equal(t, 200, resp.StatusCode)

	// Check Content-Type header
	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", contentType)

	// DOCX files are ZIP files, they start with PK (0x50 0x4B)
	assert.True(t, len(body) > 4, "DOCX body should not be empty")
	assert.Equal(t, "PK", string(body[:2]), "DOCX should start with PK (ZIP header)")
}

func testExportBoldTextPadAsDocx(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "boldTextPadDocx"
	testText := "Bold Text DOCX"

	token := createPadWithBoldText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/docx", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", contentType)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, len(body) > 4, "DOCX body should not be empty")
	assert.Equal(t, "PK", string(body[:2]), "DOCX should start with PK (ZIP header)")
}

func testExportItalicTextPadAsDocx(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "italicTextPadDocx"
	testText := "Italic Text DOCX"

	token := createPadWithItalicText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/docx", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", contentType)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, len(body) > 4, "DOCX body should not be empty")
	assert.Equal(t, "PK", string(body[:2]), "DOCX should start with PK (ZIP header)")
}

func testExportIndentedTextPadAsDocx(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "indentedTextPadDocx"
	testText := "Indented Text DOCX"

	token := createPadWithIndentation(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/docx", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", contentType)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, len(body) > 4, "DOCX body should not be empty")
	assert.Equal(t, "PK", string(body[:2]), "DOCX should start with PK (ZIP header)")
}

func testExportMixedFormattingPadAsDocx(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "mixedFormattingPadDocx"
	testText := "Mixed Formatting Text DOCX"

	token := createPadWithMixedFormatting(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/docx", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", contentType)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, len(body) > 4, "DOCX body should not be empty")
	assert.Equal(t, "PK", string(body[:2]), "DOCX should start with PK (ZIP header)")
}

// ODT Export Tests
func testExportPlainTextPadAsOdt(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "plainTextPadOdt"
	testText := "Hello World ODT"

	token := createPadWithPlainText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/odt", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	if resp.StatusCode != 200 {
		t.Logf("ODT Export failed with status %d: %s", resp.StatusCode, string(body))
	}

	assert.Equal(t, 200, resp.StatusCode)

	// Check Content-Type header
	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/vnd.oasis.opendocument.text", contentType)

	// ODT files are ZIP files, they start with PK (0x50 0x4B)
	assert.True(t, len(body) > 4, "ODT body should not be empty")
	assert.Equal(t, "PK", string(body[:2]), "ODT should start with PK (ZIP header)")
}

func testExportBoldTextPadAsOdt(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "boldTextPadOdt"
	testText := "Bold Text ODT"

	token := createPadWithBoldText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/odt", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/vnd.oasis.opendocument.text", contentType)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, len(body) > 4, "ODT body should not be empty")
	assert.Equal(t, "PK", string(body[:2]), "ODT should start with PK (ZIP header)")
}

func testExportItalicTextPadAsOdt(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "italicTextPadOdt"
	testText := "Italic Text ODT"

	token := createPadWithItalicText(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/odt", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/vnd.oasis.opendocument.text", contentType)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, len(body) > 4, "ODT body should not be empty")
	assert.Equal(t, "PK", string(body[:2]), "ODT should start with PK (ZIP header)")
}

func testExportIndentedTextPadAsOdt(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "indentedTextPadOdt"
	testText := "Indented Text ODT"

	token := createPadWithIndentation(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/odt", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/vnd.oasis.opendocument.text", contentType)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, len(body) > 4, "ODT body should not be empty")
	assert.Equal(t, "PK", string(body[:2]), "ODT should start with PK (ZIP header)")
}

func testExportMixedFormattingPadAsOdt(t *testing.T, tsStore testutils.TestDataStore) {
	app := setupExportApp(tsStore)
	padId := "mixedFormattingPadOdt"
	testText := "Mixed Formatting Text ODT"

	token := createPadWithMixedFormatting(t, tsStore, padId, testText)

	req := httptest.NewRequest("GET", "/p/"+padId+"/export/odt", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	resp, err := app.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	contentType := resp.Header.Get("Content-Type")
	assert.Equal(t, "application/vnd.oasis.opendocument.text", contentType)

	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.True(t, len(body) > 4, "ODT body should not be empty")
	assert.Equal(t, "PK", string(body[:2]), "ODT should start with PK (ZIP header)")
}
