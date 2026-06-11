package author

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/api/author"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthor(t *testing.T) {
	testDb := testutils.NewTestDBHandler(t)

	testDb.AddTests(testutils.TestRunConfig{
		Name: "Create Author Successfully",
		Test: testCreateAuthorNoName,
	},
		testutils.TestRunConfig{
			Name: "Create Author No Body",
			Test: testCreateAuthorNoBody,
		},
		testutils.TestRunConfig{
			Name: "Get Not Existing Author",
			Test: testGetNotExistingAuthor,
		},
		testutils.TestRunConfig{
			Name: "Get Existing Author",
			Test: testGetExistingAuthor,
		},
		testutils.TestRunConfig{
			Name: "Get Author Pad IDs",
			Test: testGetAuthorPadIDS,
		},
		// New tests
		testutils.TestRunConfig{
			Name: "Create Author If Not Exists For - New Author",
			Test: testCreateAuthorIfNotExistsForNew,
		},
		testutils.TestRunConfig{
			Name: "Create Author If Not Exists For - Existing Author",
			Test: testCreateAuthorIfNotExistsForExisting,
		},
		testutils.TestRunConfig{
			Name: "Get Author Name",
			Test: testGetAuthorName,
		},
		testutils.TestRunConfig{
			Name: "Get Author Name Not Found",
			Test: testGetAuthorNameNotFound,
		},
		testutils.TestRunConfig{
			Name: "Anonymize Author",
			Test: testAnonymizeAuthor,
		},
		testutils.TestRunConfig{
			Name: "Anonymize Author Not Found",
			Test: testAnonymizeAuthorNotFound,
		},
		testutils.TestRunConfig{
			Name: "Anonymize Author Idempotent",
			Test: testAnonymizeAuthorIdempotent,
		},
	)
	defer testDb.StartTestDBHandler()
}

func testCreateAuthorNoName(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	author.Init(initStore)
	var dto = author.CreateDto{}
	marshall, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/admin/api/author", bytes.NewBuffer(marshall))

	resp, _ := initStore.C.Test(req)
	require.NotNil(t, resp)
	if resp.StatusCode != 400 {
		t.Errorf("should deny creation of author without required fields, got %d", resp.StatusCode)
	}
}

func testCreateAuthorNoBody(t *testing.T, tsStore testutils.TestDataStore) {
	author.Init(tsStore.ToInitStore())
	req := httptest.NewRequest("POST", "/admin/api/author", nil)

	resp, _ := tsStore.App.Test(req)
	require.NotNil(t, resp)
	if resp.StatusCode != 400 {
		t.Errorf("should deny creation of author with nil body, got %d", resp.StatusCode)
	}
}

func testGetNotExistingAuthor(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	author.Init(initStore)
	req := httptest.NewRequest("GET", "/admin/api/author/unknownAuthorId", nil)

	resp, err := initStore.C.Test(req)
	if err != nil {
		t.Errorf("error getting not existing author: %v", err)
	}
	require.NotNil(t, resp)
	if resp.StatusCode != 404 {
		t.Errorf("should return 404 for not existing author, got %d", resp.StatusCode)
	}
}

func testGetExistingAuthor(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	author.Init(initStore)

	// create author first
	var dto = author.CreateDto{
		Name: "testAuthor",
	}
	marshall, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/admin/api/author", bytes.NewBuffer(marshall))
	resp, err := initStore.C.Test(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	if resp.StatusCode != 200 {
		t.Errorf("should create author, got %d", resp.StatusCode)
	}

	var createdAuthor author.CreateDtoResponse
	bytesOFCreate, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bytesOFCreate, &createdAuthor)

	req = httptest.NewRequest("GET", "/admin/api/author/"+createdAuthor.AuthorId, nil)

	resp, err = initStore.C.Test(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	if resp.StatusCode != 200 {
		t.Errorf("should return the created author, got %d", resp.StatusCode)
	}
}

func testGetAuthorPadIDS(t *testing.T, tsStore testutils.TestDataStore) {
	author.Init(tsStore.ToInitStore())
	dbAuthorToSave := testutils.GenerateDBAuthor()
	assert.NoError(t, tsStore.DS.SaveAuthor(dbAuthorToSave))
	padText := "Hallo123\n"
	_, err := tsStore.PadManager.GetPad("pad123", &padText, &dbAuthorToSave.ID)
	assert.NoError(t, err)
	req := httptest.NewRequest("GET", "/admin/api/author/"+dbAuthorToSave.ID+"/pads", nil)

	resp, err := tsStore.App.Test(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	if resp.StatusCode != 200 {
		t.Errorf("should return 200 for existing author pads, got %d", resp.StatusCode)
	}

	var padsResponse []string
	bytesOfResponse, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bytesOfResponse, &padsResponse)
	if len(padsResponse) == 0 {
		t.Errorf("expected at least one pad ID for author, got %d", len(padsResponse))
	}
}

// ========== Create Author If Not Exists For ==========

func testCreateAuthorIfNotExistsForNew(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	author.Init(initStore)

	reqBody := author.CreateAuthorIfNotExistsForRequest{
		AuthorMapper: "testMapper123",
		Name:         "Test Author",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/api/author/createIfNotExistsFor", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req)

	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, 200, resp.StatusCode)

	var response author.CreateDtoResponse
	respBody, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(respBody, &response)

	assert.NotEmpty(t, response.AuthorId)
	assert.True(t, len(response.AuthorId) > 2)
}

func testCreateAuthorIfNotExistsForExisting(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	author.Init(initStore)

	// First call - create
	reqBody := author.CreateAuthorIfNotExistsForRequest{
		AuthorMapper: "existingMapper456",
		Name:         "Original Name",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/api/author/createIfNotExistsFor", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := initStore.C.Test(req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 200, resp.StatusCode)

	var firstResponse author.CreateDtoResponse
	respBody, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(respBody, &firstResponse)

	// Second call - should return same author
	reqBody2 := author.CreateAuthorIfNotExistsForRequest{
		AuthorMapper: "existingMapper456",
		Name:         "Updated Name",
	}
	body2, _ := json.Marshal(reqBody2)

	req2 := httptest.NewRequest("POST", "/admin/api/author/createIfNotExistsFor", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := initStore.C.Test(req2)

	require.NoError(t, err)
	require.NotNil(t, resp2)
	assert.Equal(t, 200, resp2.StatusCode)

	var secondResponse author.CreateDtoResponse
	respBody2, _ := io.ReadAll(resp2.Body)
	_ = json.Unmarshal(respBody2, &secondResponse)

	assert.Equal(t, firstResponse.AuthorId, secondResponse.AuthorId)
}

// ========== Get Author Name ==========

func testGetAuthorName(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	author.Init(initStore)

	// Create author with name first
	testName := "Test Author Name"
	createdAuthor, err := tsStore.AuthorManager.CreateAuthor(&testName)
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/admin/api/author/"+createdAuthor.Id+"/name", nil)
	resp, err := initStore.C.Test(req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 200, resp.StatusCode)

	var response author.AuthorNameResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &response)

	assert.Equal(t, testName, response.AuthorName)
}

func testGetAuthorNameNotFound(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	author.Init(initStore)

	req := httptest.NewRequest("GET", "/admin/api/author/a.nonexistent12345/name", nil)
	resp, err := initStore.C.Test(req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 404, resp.StatusCode)
}

// ========== Anonymize Author (GDPR Art. 17 erasure) ==========

func testAnonymizeAuthor(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	author.Init(initStore)

	// Author with a name, color, token binding and a chat message on a pad.
	testName := "GDPR Test Author"
	createdAuthor, err := tsStore.AuthorManager.CreateAuthor(&testName)
	require.NoError(t, err)
	require.NoError(t, tsStore.AuthorManager.SetAuthorColor(createdAuthor.Id, "#123abc"))
	require.NoError(t, tsStore.DS.SetAuthorByToken("api-anonymize-token", createdAuthor.Id))

	padText := "anonymize pad text\n"
	padId := "anonymizeApiPad"
	_, err = tsStore.PadManager.GetPad(padId, &padText, &createdAuthor.Id)
	require.NoError(t, err)
	require.NoError(t, tsStore.DS.SaveChatMessage(padId, 0, &createdAuthor.Id, 4711, "identifying chat text"))

	req := httptest.NewRequest("POST", "/admin/api/author/"+createdAuthor.Id+"/anonymize", nil)
	resp, err := initStore.C.Test(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 200, resp.StatusCode)

	// Name is scrubbed but the author record still exists.
	req = httptest.NewRequest("GET", "/admin/api/author/"+createdAuthor.Id+"/name", nil)
	resp, err = initStore.C.Test(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 200, resp.StatusCode)

	var nameResponse author.AuthorNameResponse
	body, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &nameResponse)
	assert.Equal(t, "", nameResponse.AuthorName, "author name must be scrubbed")

	// Token binding is severed.
	_, err = tsStore.DS.GetAuthorByToken("api-anonymize-token")
	assert.Error(t, err, "token must no longer resolve to the author")

	// Chat message survives, authorship is nulled.
	chats, err := tsStore.DS.GetChatsOfPad(padId, 0, 0)
	require.NoError(t, err)
	require.Len(t, *chats, 1)
	assert.Nil(t, (*chats)[0].AuthorId, "chat authorship must be nulled")
	assert.Equal(t, "identifying chat text", (*chats)[0].Message)
}

func testAnonymizeAuthorNotFound(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	author.Init(initStore)

	req := httptest.NewRequest("POST", "/admin/api/author/a.unknownAuthor9876/anonymize", nil)
	resp, err := initStore.C.Test(req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 404, resp.StatusCode)
}

func testAnonymizeAuthorIdempotent(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	author.Init(initStore)

	testName := "GDPR Idempotent Author"
	createdAuthor, err := tsStore.AuthorManager.CreateAuthor(&testName)
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/admin/api/author/"+createdAuthor.Id+"/anonymize", nil)
		resp, err := initStore.C.Test(req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, 200, resp.StatusCode, "anonymize call %d must succeed", i+1)
	}
}
