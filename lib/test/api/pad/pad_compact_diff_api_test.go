package pad

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPadCompactAndDiffAPI(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)

	testDb.AddTests(
		// compactPad
		testutils.TestRunConfig{
			Name: "CompactPad shrinks revision history and keeps text",
			Test: testCompactPadSuccess,
		},
		testutils.TestRunConfig{
			Name: "CompactPad pad not found returns 404",
			Test: testCompactPadNotFound,
		},
		testutils.TestRunConfig{
			Name: "CompactPad invalid keepRevisions returns 400",
			Test: testCompactPadInvalidKeepRevisions,
		},
		// createDiffHTML
		testutils.TestRunConfig{
			Name: "CreateDiffHTML returns diff html and authors",
			Test: testCreateDiffHTMLSuccess,
		},
		testutils.TestRunConfig{
			Name: "CreateDiffHTML invalid revisions return 400",
			Test: testCreateDiffHTMLInvalidRevs,
		},
		testutils.TestRunConfig{
			Name: "CreateDiffHTML pad not found returns 404",
			Test: testCreateDiffHTMLNotFound,
		},
	)

	defer testDb.StartTestDBHandler()
}

// setPadTextViaAPI updates the pad text through the public admin API, creating
// one new revision per call.
func setPadTextViaAPI(t *testing.T, initStore *lib.InitStore, padId string, text string, authorId string) {
	t.Helper()

	reqBody := pad.SetTextRequest{
		Text:     text,
		AuthorId: authorId,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/api/pads/"+padId+"/text", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

// getRevisionsCountViaAPI returns the pad's head revision number via the API.
func getRevisionsCountViaAPI(t *testing.T, initStore *lib.InitStore, padId string) int {
	t.Helper()

	req := httptest.NewRequest("GET", "/admin/api/pads/"+padId+"/revisionsCount", nil)
	resp, err := initStore.C.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response struct {
		Revisions int `json:"revisions"`
	}
	body, _ := io.ReadAll(resp.Body)
	assert.NoError(t, json.Unmarshal(body, &response))
	return response.Revisions
}

// getCurrentPadText returns the pad's current text via the API.
func getCurrentPadText(t *testing.T, initStore *lib.InitStore, padId string) string {
	t.Helper()

	req := httptest.NewRequest("GET", "/admin/api/pads/"+padId+"/text", nil)
	resp, err := initStore.C.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response pad.TextResponse
	body, _ := io.ReadAll(resp.Body)
	assert.NoError(t, json.Unmarshal(body, &response))
	return response.Text
}

// compactPadViaAPI issues the compact request and returns the response status.
func compactPadViaAPI(t *testing.T, initStore *lib.InitStore, padId string, body []byte) int {
	t.Helper()

	req := httptest.NewRequest("POST", "/admin/api/pads/"+padId+"/compact", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req, fiber.TestConfig{Timeout: 30 * time.Second})

	require.NoError(t, err)
	if resp.StatusCode == 500 {
		debugBody, _ := io.ReadAll(resp.Body)
		t.Logf("compact returned 500: %s", string(debugBody))
	}
	return resp.StatusCode
}

// ========== compactPad ==========

func testCompactPadSuccess(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	testAuthor, err := tsStore.AuthorManager.CreateAuthor(nil)
	assert.NoError(t, err)

	createTestPad(t, tsStore, "compactpad", "Initial\n")
	for i := 1; i <= 4; i++ {
		setPadTextViaAPI(t, initStore, "compactpad", fmt.Sprintf("Version %d", i), testAuthor.Id)
	}
	assert.Equal(t, 4, getRevisionsCountViaAPI(t, initStore, "compactpad"))

	body, _ := json.Marshal(pad.CompactPadRequest{KeepRevisions: 2})
	status := compactPadViaAPI(t, initStore, "compactpad", body)
	assert.Equal(t, 200, status)

	// The revision history must have been collapsed to the last 2 revisions
	assert.Equal(t, 2, getRevisionsCountViaAPI(t, initStore, "compactpad"))

	// The pad text must be preserved
	assert.Contains(t, getCurrentPadText(t, initStore, "compactpad"), "Version 4")
}

func testCompactPadNotFound(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	body, _ := json.Marshal(pad.CompactPadRequest{KeepRevisions: 1})
	status := compactPadViaAPI(t, initStore, "compactnonexistentpad", body)
	assert.Equal(t, 404, status)
}

func testCompactPadInvalidKeepRevisions(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	testAuthor, err := tsStore.AuthorManager.CreateAuthor(nil)
	assert.NoError(t, err)

	createTestPad(t, tsStore, "compactvalpad", "Initial\n")
	setPadTextViaAPI(t, initStore, "compactvalpad", "Version 1", testAuthor.Id)
	setPadTextViaAPI(t, initStore, "compactvalpad", "Version 2", testAuthor.Id)
	// head is now 2

	// keepRevisions must be >= 1
	body, _ := json.Marshal(pad.CompactPadRequest{KeepRevisions: 0})
	assert.Equal(t, 400, compactPadViaAPI(t, initStore, "compactvalpad", body))

	// keepRevisions missing defaults to 0 and is rejected
	assert.Equal(t, 400, compactPadViaAPI(t, initStore, "compactvalpad", []byte("{}")))

	// keepRevisions must be lower than the head revision
	body, _ = json.Marshal(pad.CompactPadRequest{KeepRevisions: 2})
	assert.Equal(t, 400, compactPadViaAPI(t, initStore, "compactvalpad", body))

	body, _ = json.Marshal(pad.CompactPadRequest{KeepRevisions: 5})
	assert.Equal(t, 400, compactPadViaAPI(t, initStore, "compactvalpad", body))

	// nothing was deleted
	assert.Equal(t, 2, getRevisionsCountViaAPI(t, initStore, "compactvalpad"))
}

// ========== createDiffHTML ==========

func testCreateDiffHTMLSuccess(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	testAuthor, err := tsStore.AuthorManager.CreateAuthor(nil)
	assert.NoError(t, err)

	createTestPad(t, tsStore, "diffpad", "First line\n")
	setPadTextViaAPI(t, initStore, "diffpad", "Second version text", testAuthor.Id)

	req := httptest.NewRequest("GET", "/admin/api/pads/diffpad/diffHTML?startRev=0&endRev=1", nil)
	resp, err := initStore.C.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response pad.DiffHTMLResponse
	respBody, _ := io.ReadAll(resp.Body)
	assert.NoError(t, json.Unmarshal(respBody, &response))

	assert.NotEmpty(t, response.HTML)
	// the inserted text must show up in the diff
	assert.Contains(t, response.HTML, "Second version text")
	// the deleted text is re-inserted carrying the 'removed' attribute
	assert.Contains(t, response.HTML, "First line")
	assert.Contains(t, response.HTML, "removed")
	// the author of the change is reported
	assert.Contains(t, response.Authors, testAuthor.Id)

	// omitting endRev defaults to the head revision
	req = httptest.NewRequest("GET", "/admin/api/pads/diffpad/diffHTML?startRev=0", nil)
	resp, err = initStore.C.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var responseNoEnd pad.DiffHTMLResponse
	respBody, _ = io.ReadAll(resp.Body)
	assert.NoError(t, json.Unmarshal(respBody, &responseNoEnd))
	// The CSS block ordering of the export is map-iteration dependent, so only
	// compare the diff content itself.
	assert.Contains(t, responseNoEnd.HTML, "Second version text")
	assert.Contains(t, responseNoEnd.HTML, "First line")
	assert.Contains(t, responseNoEnd.HTML, "removed")
	assert.Contains(t, responseNoEnd.Authors, testAuthor.Id)
}

func testCreateDiffHTMLInvalidRevs(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	testAuthor, err := tsStore.AuthorManager.CreateAuthor(nil)
	assert.NoError(t, err)

	createTestPad(t, tsStore, "diffvalpad", "First line\n")
	setPadTextViaAPI(t, initStore, "diffvalpad", "Changed text", testAuthor.Id)

	for _, query := range []string{
		"",                     // startRev is required
		"?startRev=abc",        // startRev must be a number
		"?startRev=-1",         // startRev must not be negative
		"?startRev=1&endRev=0", // endRev must not be lower than startRev
	} {
		req := httptest.NewRequest("GET", "/admin/api/pads/diffvalpad/diffHTML"+query, nil)
		resp, err := initStore.C.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, 400, resp.StatusCode, "query: %q", query)
	}
}

func testCreateDiffHTMLNotFound(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	req := httptest.NewRequest("GET", "/admin/api/pads/diffnonexistentpad/diffHTML?startRev=0", nil)
	resp, err := initStore.C.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}
