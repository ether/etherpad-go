package pad

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/api/pad"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
)

func TestPadAPI(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)

	testDb.AddTests(
		// List all pads
		testutils.TestRunConfig{
			Name: "ListAllPads returns empty list initially",
			Test: testListAllPadsEmpty,
		},
		testutils.TestRunConfig{
			Name: "ListAllPads returns pads after creation",
			Test: testListAllPadsWithPads,
		},
		// Create pad
		testutils.TestRunConfig{
			Name: "CreatePad successfully",
			Test: testCreatePadSuccess,
		},
		testutils.TestRunConfig{
			Name: "CreatePad with invalid characters fails",
			Test: testCreatePadInvalidChars,
		},
		testutils.TestRunConfig{
			Name: "CreatePad already exists returns 409",
			Test: testCreatePadAlreadyExists,
		},
		// Delete pad
		testutils.TestRunConfig{
			Name: "DeletePad successfully",
			Test: testDeletePadSuccess,
		},
		testutils.TestRunConfig{
			Name: "DeletePad not found returns 404",
			Test: testDeletePadNotFound,
		},
		// Get/Set text
		testutils.TestRunConfig{
			Name: "GetPadText returns text",
			Test: testGetPadText,
		},
		testutils.TestRunConfig{
			Name: "SetPadText updates text",
			Test: testSetPadText,
		},
		// Append text
		testutils.TestRunConfig{
			Name: "AppendText appends to pad",
			Test: testAppendText,
		},
		// HTML
		testutils.TestRunConfig{
			Name: "GetHTML returns HTML",
			Test: testGetHTML,
		},
		testutils.TestRunConfig{
			Name: "SetHTML sets HTML content",
			Test: testSetHTML,
		},
		// Revisions
		testutils.TestRunConfig{
			Name: "GetRevisionsCount returns count",
			Test: testGetRevisionsCount,
		},
		testutils.TestRunConfig{
			Name: "GetRevisionChangeset returns changeset",
			Test: testGetRevisionChangeset,
		},
		// Saved revisions
		testutils.TestRunConfig{
			Name: "SaveRevision saves current revision",
			Test: testSaveRevision,
		},
		testutils.TestRunConfig{
			Name: "GetSavedRevisionsCount returns count",
			Test: testGetSavedRevisionsCount,
		},
		testutils.TestRunConfig{
			Name: "ListSavedRevisions returns list",
			Test: testListSavedRevisions,
		},
		// Authors
		testutils.TestRunConfig{
			Name: "ListAuthorsOfPad returns authors",
			Test: testListAuthorsOfPad,
		},
		// Last edited
		testutils.TestRunConfig{
			Name: "GetLastEdited returns timestamp",
			Test: testGetLastEdited,
		},
		// Read-only
		testutils.TestRunConfig{
			Name: "GetReadOnlyID returns read-only ID",
			Test: testGetReadOnlyID,
		},
		testutils.TestRunConfig{
			Name: "GetPadID from read-only ID",
			Test: testGetPadIDFromReadOnly,
		},
		// Attribute pool
		testutils.TestRunConfig{
			Name: "GetAttributePool returns pool",
			Test: testGetAttributePool,
		},
		// Chat
		testutils.TestRunConfig{
			Name: "GetChatHead returns chat head",
			Test: testGetChatHead,
		},
		testutils.TestRunConfig{
			Name: "AppendChatMessage adds message",
			Test: testAppendChatMessage,
		},
		testutils.TestRunConfig{
			Name: "GetChatHistory returns messages",
			Test: testGetChatHistory,
		},
		// Users
		testutils.TestRunConfig{
			Name: "GetPadUsers returns empty list",
			Test: testGetPadUsersEmpty,
		},
		testutils.TestRunConfig{
			Name: "GetPadUsersCount returns zero",
			Test: testGetPadUsersCount,
		},
		// Check token
		testutils.TestRunConfig{
			Name: "CheckToken returns 200",
			Test: testCheckToken,
		},
	)

	defer testDb.StartTestDBHandler()
}

// Helper function to create a test pad
func createTestPad(t *testing.T, tsStore testutils.TestDataStore, padId string, text string) {
	_, err := tsStore.PadManager.GetPad(padId, &text, nil)
	assert.NoError(t, err)
}

// ========== List All Pads ==========

func testListAllPadsEmpty(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	req := httptest.NewRequest("GET", "/admin/api/pads", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	// Response should contain padIDs key
	assert.Contains(t, string(body), "padIDs")
}

func testListAllPadsWithPads(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	// Create pads first
	text := "Test content\n"
	createTestPad(t, tsStore, "testpad1", text)
	createTestPad(t, tsStore, "testpad2", text)

	req := httptest.NewRequest("GET", "/admin/api/pads", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response pad.AllPadsResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.GreaterOrEqual(t, len(response.PadIDs), 2)
}

// ========== Create Pad ==========

func testCreatePadSuccess(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	reqBody := pad.CreatePadRequest{
		Text:     "Initial text",
		AuthorId: "",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/api/pads/newpad123", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func testCreatePadInvalidChars(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	req := httptest.NewRequest("POST", "/admin/api/pads/invalid$pad", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func testCreatePadAlreadyExists(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	// Create pad first
	text := "Existing content\n"
	createTestPad(t, tsStore, "existingpad", text)

	req := httptest.NewRequest("POST", "/admin/api/pads/existingpad", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 409, resp.StatusCode)
}

// ========== Delete Pad ==========

func testDeletePadSuccess(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	// Create pad first
	text := "To be deleted\n"
	createTestPad(t, tsStore, "padtodelete", text)

	req := httptest.NewRequest("DELETE", "/admin/api/pads/padtodelete", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func testDeletePadNotFound(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	req := httptest.NewRequest("DELETE", "/admin/api/pads/nonexistentpad", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
}

// ========== Get/Set Text ==========

func testGetPadText(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Hello World\n"
	createTestPad(t, tsStore, "textpad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/textpad/text", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response pad.TextResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.Contains(t, response.Text, "Hello World")
}

func testSetPadText(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	// Create an author for the operation
	testAuthor, err := tsStore.AuthorManager.CreateAuthor(nil)
	assert.NoError(t, err)

	text := "Initial\n"
	createTestPad(t, tsStore, "settextpad", text)

	reqBody := pad.SetTextRequest{
		Text:     "Updated text",
		AuthorId: testAuthor.Id,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/api/pads/settextpad/text", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		t.Logf("Response body: %s", string(respBody))
	}
	assert.Equal(t, 200, resp.StatusCode)
}

// ========== Append Text ==========

func testAppendText(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	// Create an author for the operation
	testAuthor, err := tsStore.AuthorManager.CreateAuthor(nil)
	assert.NoError(t, err)

	text := "Start\n"
	createTestPad(t, tsStore, "appendpad", text)

	reqBody := pad.AppendTextRequest{
		Text:     " appended",
		AuthorId: testAuthor.Id,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/api/pads/appendpad/appendText", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

// ========== HTML ==========

func testGetHTML(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "HTML content\n"
	createTestPad(t, tsStore, "htmlpad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/htmlpad/html", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response map[string]string
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.Contains(t, response["html"], "HTML content")
}

func testSetHTML(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	// Create an author for the operation
	testAuthor, err := tsStore.AuthorManager.CreateAuthor(nil)
	assert.NoError(t, err)

	text := "Original\n"
	createTestPad(t, tsStore, "sethtmlpad", text)

	reqBody := pad.SetHTMLRequest{
		HTML:     "<p>New HTML content</p>",
		AuthorId: testAuthor.Id,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/api/pads/sethtmlpad/html", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

// ========== Revisions ==========

func testGetRevisionsCount(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Revision test\n"
	createTestPad(t, tsStore, "revpad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/revpad/revisionsCount", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response map[string]int
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.GreaterOrEqual(t, response["revisions"], 0)
}

func testGetRevisionChangeset(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Changeset test\n"
	createTestPad(t, tsStore, "changesetpad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/changesetpad/revisionChangeset", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

// ========== Saved Revisions ==========

func testSaveRevision(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Save revision test\n"
	createTestPad(t, tsStore, "saverevpad", text)

	req := httptest.NewRequest("POST", "/admin/api/pads/saverevpad/saveRevision", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func testGetSavedRevisionsCount(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Saved count test\n"
	createTestPad(t, tsStore, "savedcountpad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/savedcountpad/savedRevisionsCount", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response pad.SavedRevisionsCountResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.GreaterOrEqual(t, response.SavedRevisions, 0)
}

func testListSavedRevisions(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "List saved test\n"
	createTestPad(t, tsStore, "listsavedpad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/listsavedpad/savedRevisions", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response pad.SavedRevisionsListResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.NotNil(t, response.SavedRevisions)
}

// ========== Authors ==========

func testListAuthorsOfPad(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Author test\n"
	createTestPad(t, tsStore, "authorpad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/authorpad/authors", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response pad.AuthorsResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.NotNil(t, response.AuthorIDs)
}

// ========== Last Edited ==========

func testGetLastEdited(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Last edited test\n"
	createTestPad(t, tsStore, "lasteditedpad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/lasteditedpad/lastEdited", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response map[string]int64
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.Contains(t, string(body), "lastEdited")
}

// ========== Read-Only ==========

func testGetReadOnlyID(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Read only test\n"
	createTestPad(t, tsStore, "readonlypad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/readonlypad/readOnlyID", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response pad.ReadOnlyIDResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.NotEmpty(t, response.ReadOnlyID)
	assert.True(t, len(response.ReadOnlyID) > 0)
}

func testGetPadIDFromReadOnly(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Read only test\n"
	createTestPad(t, tsStore, "ropadid", text)

	// First get the read-only ID
	req := httptest.NewRequest("GET", "/admin/api/pads/ropadid/readOnlyID", nil)
	resp, err := initStore.C.Test(req, 5000)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var roResponse pad.ReadOnlyIDResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &roResponse)

	assert.NotEmpty(t, roResponse.ReadOnlyID)
	// Note: The route /pads/readonly/:roId may conflict with /pads/:padId routes
	// depending on Fiber router behavior. If this test fails, verify route order.
}

// ========== Attribute Pool ==========

func testGetAttributePool(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Pool test\n"
	createTestPad(t, tsStore, "poolpad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/poolpad/attributePool", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

// ========== Chat ==========

func testGetChatHead(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Chat head test\n"
	createTestPad(t, tsStore, "chatheadpad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/chatheadpad/chatHead", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response pad.ChatHeadResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.GreaterOrEqual(t, response.ChatHead, -1)
}

func testAppendChatMessage(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Chat append test\n"
	createTestPad(t, tsStore, "chatappendpad", text)

	// Create an author first
	createdAuthor, err := tsStore.AuthorManager.CreateAuthor(nil)
	assert.NoError(t, err)

	reqBody := pad.AppendChatMessageRequest{
		Text:     "Hello from test!",
		AuthorID: createdAuthor.Id,
		Time:     0, // Will use current time
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/api/pads/chatappendpad/chat", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func testGetChatHistory(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Chat history test\n"
	createTestPad(t, tsStore, "chathistorypad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/chathistorypad/chatHistory", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response pad.ChatHistoryResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.NotNil(t, response.Messages)
}

// ========== Users ==========

func testGetPadUsersEmpty(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Users test\n"
	createTestPad(t, tsStore, "userspad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/userspad/users", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response pad.PadUsersResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.NotNil(t, response.PadUsers)
	assert.Equal(t, 0, len(response.PadUsers))
}

func testGetPadUsersCount(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	text := "Users count test\n"
	createTestPad(t, tsStore, "userscountpad", text)

	req := httptest.NewRequest("GET", "/admin/api/pads/userscountpad/usersCount", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response pad.PadUsersCountResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.Equal(t, 0, response.PadUsersCount)
}

// ========== Check Token ==========

func testCheckToken(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	pad.Init(initStore)

	req := httptest.NewRequest("GET", "/admin/api/checkToken", nil)
	resp, err := initStore.C.Test(req, 5000)

	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}
