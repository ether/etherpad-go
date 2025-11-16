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
)

func TestCreateAuthorNoName(t *testing.T) {
	app := fiber.New()
	testMemoryUtils := testutils.InitMemoryUtils()
	author.Init(app, testMemoryUtils.DB, testMemoryUtils.Validator)
	var dto = author.CreateDto{}
	marshall, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/author", bytes.NewBuffer(marshall))

	resp, _ := app.Test(req, 10)
	if resp.StatusCode != 400 {
		t.Errorf("should deny creation of author without required fields, got %d", resp.StatusCode)
	}
}

func TestCreateAuthorNoBody(t *testing.T) {
	app := fiber.New()
	testMemoryUtils := testutils.InitMemoryUtils()
	author.Init(app, testMemoryUtils.DB, testMemoryUtils.Validator)
	req := httptest.NewRequest("POST", "/author", nil)

	resp, _ := app.Test(req, 10)
	if resp.StatusCode != 400 {
		t.Errorf("should deny creation of author with nil body, got %d", resp.StatusCode)
	}
}

func TestGetNotExistingAuthor(t *testing.T) {
	app := fiber.New()
	testMemoryUtils := testutils.InitMemoryUtils()
	author.Init(app, testMemoryUtils.DB, testMemoryUtils.Validator)
	req := httptest.NewRequest("GET", "/author/unknownAuthorId", nil)

	resp, _ := app.Test(req, 10)
	if resp.StatusCode != 404 {
		t.Errorf("should return 404 for not existing author, got %d", resp.StatusCode)
	}
}

func TestGetExistingAuthor(t *testing.T) {
	app := fiber.New()
	testMemoryUtils := testutils.InitMemoryUtils()
	author.Init(app, testMemoryUtils.DB, testMemoryUtils.Validator)

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

func TestGetAuthorPadIDS(t *testing.T) {
	app := fiber.New()
	testMemoryUtils := testutils.InitMemoryUtils()
	author.Init(app, testMemoryUtils.DB, testMemoryUtils.Validator)
	dbAuthorToSave := testutils.GenerateDBAuthor()
	testMemoryUtils.DB.SaveAuthor(dbAuthorToSave)
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
