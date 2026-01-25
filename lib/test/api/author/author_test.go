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
	)
	defer testDb.StartTestDBHandler()
}

func testCreateAuthorNoName(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	author.Init(initStore)
	var dto = author.CreateDto{}
	marshall, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/admin/api/author", bytes.NewBuffer(marshall))

	resp, _ := initStore.C.Test(req, 10)
	if resp.StatusCode != 400 {
		t.Errorf("should deny creation of author without required fields, got %d", resp.StatusCode)
	}
}

func testCreateAuthorNoBody(t *testing.T, tsStore testutils.TestDataStore) {
	author.Init(tsStore.ToInitStore())
	req := httptest.NewRequest("POST", "/admin/api/author", nil)

	resp, _ := tsStore.App.Test(req, 10)
	if resp.StatusCode != 400 {
		t.Errorf("should deny creation of author with nil body, got %d", resp.StatusCode)
	}
}

func testGetNotExistingAuthor(t *testing.T, tsStore testutils.TestDataStore) {
	initStore := tsStore.ToInitStore()
	author.Init(initStore)
	req := httptest.NewRequest("GET", "/admin/api/author/unknownAuthorId", nil)

	resp, err := initStore.C.Test(req, 100)
	if err != nil {
		t.Errorf("error getting not existing author: %v", err)
	}
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
	resp, err := initStore.C.Test(req, 100)
	if err != nil {
		t.Errorf("error creating author: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("should create author, got %d", resp.StatusCode)
	}

	var createdAuthor author.CreateDtoResponse
	bytesOFCreate, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bytesOFCreate, &createdAuthor)

	req = httptest.NewRequest("GET", "/admin/api/author/"+createdAuthor.AuthorId, nil)

	resp, err = initStore.C.Test(req, 100)
	if err != nil {
		t.Errorf("error getting author: %v", err)
	}
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

	resp, err := tsStore.App.Test(req, 100)
	if err != nil {
		t.Errorf("error getting author pads: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("should return 200 for existing author pads, got %d", resp.StatusCode)
	}

	var padsResponse map[string]struct{}
	bytesOfResponse, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bytesOfResponse, &padsResponse)
	if len(padsResponse) != 0 {
		t.Errorf("should return all pad IDs of author, expected %d got %d", 0, len(padsResponse))
	}
}
