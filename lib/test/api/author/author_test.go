package author

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/api/author"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/gofiber/fiber/v2"
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
	app := fiber.New()
	author.Init(app, tsStore.DS, tsStore.Validator)
	var dto = author.CreateDto{}
	marshall, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/author", bytes.NewBuffer(marshall))

	resp, _ := app.Test(req, 10)
	if resp.StatusCode != 400 {
		t.Errorf("should deny creation of author without required fields, got %d", resp.StatusCode)
	}
}

func testCreateAuthorNoBody(t *testing.T, tsStore testutils.TestDataStore) {
	app := fiber.New()
	author.Init(app, tsStore.DS, tsStore.Validator)
	req := httptest.NewRequest("POST", "/author", nil)

	resp, _ := app.Test(req, 10)
	if resp.StatusCode != 400 {
		t.Errorf("should deny creation of author with nil body, got %d", resp.StatusCode)
	}
}

func testGetNotExistingAuthor(t *testing.T, tsStore testutils.TestDataStore) {
	app := fiber.New()
	author.Init(app, tsStore.DS, tsStore.Validator)
	req := httptest.NewRequest("GET", "/author/unknownAuthorId", nil)

	resp, _ := app.Test(req, 10)
	if resp.StatusCode != 404 {
		t.Errorf("should return 404 for not existing author, got %d", resp.StatusCode)
	}
}

func testGetExistingAuthor(t *testing.T, tsStore testutils.TestDataStore) {
	app := fiber.New()
	author.Init(app, tsStore.DS, tsStore.Validator)

	// create author first
	var dto = author.CreateDto{
		Name: "testAuthor",
	}
	marshall, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/author", bytes.NewBuffer(marshall))
	resp, err := app.Test(req, 10)
	if err != nil {
		t.Errorf("error creating author: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("should create author, got %d", resp.StatusCode)
	}

	var createdAuthor author.CreateDtoResponse
	bytesOFCreate, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bytesOFCreate, &createdAuthor)

	req = httptest.NewRequest("GET", "/author/"+createdAuthor.AuthorId, nil)

	resp, _ = app.Test(req, 10)
	if resp.StatusCode != 200 {
		t.Errorf("should return the created author, got %d", resp.StatusCode)
	}
}

func testGetAuthorPadIDS(t *testing.T, tsStore testutils.TestDataStore) {
	t.Skip()
	// Skip because we cannot yet map pads to authors
	app := fiber.New()
	author.Init(app, tsStore.DS, tsStore.Validator)
	dbAuthorToSave := testutils.GenerateDBAuthor()
	assert.NoError(t, tsStore.DS.SaveAuthor(dbAuthorToSave))
	req := httptest.NewRequest("GET", "/author/"+dbAuthorToSave.ID+"/pads", nil)

	resp, _ := app.Test(req, 10)
	if resp.StatusCode != 200 {
		t.Errorf("should return 200 for existing author pads, got %d", resp.StatusCode)
	}

	var padsResponse map[string]struct{}
	bytesOfResponse, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bytesOfResponse, &padsResponse)
	if len(padsResponse) != len(dbAuthorToSave.PadIDs) {
		t.Errorf("should return all pad IDs of author, expected %d got %d", len(dbAuthorToSave.PadIDs), len(padsResponse))
	}
}
