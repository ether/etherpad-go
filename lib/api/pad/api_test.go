package pad

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/gofiber/fiber/v2"
)

func TestPadApi(t *testing.T) {
	testDBHandler := testutils.NewTestDBHandler(t)

	testDBHandler.AddTests(testutils.TestRunConfig{
		Name: "Test Get On Text",
		Test: testGetOnText,
	},
		testutils.TestRunConfig{
			Name: "Test Get Of AttribPool On Non Existing Pad",
			Test: testGetOfAttribPoolOnNonExistingPad,
		},
		testutils.TestRunConfig{
			Name: "Test Get Of AttribPool On Existing Pad",
			Test: testGetOfAttribPoolOnExistingPad,
		},
	)
}

func testGetOnText(t *testing.T, tsStore testutils.TestDataStore) {
	app := fiber.New()
	Init(app, tsStore.PadMessageHandler, tsStore.PadManager)
	req := httptest.NewRequest("GET", "/pads/123/text", nil)

	resp, _ := app.Test(req, 10)

	if resp.StatusCode != 404 {
		t.Errorf("Expected status code 404, got %v", resp.StatusCode)
	}
}

func testGetOfAttribPoolOnNonExistingPad(t *testing.T, tsStore testutils.TestDataStore) {
	app := fiber.New()

	Init(app, tsStore.PadMessageHandler, tsStore.PadManager)
	req := httptest.NewRequest("GET", "/pads/123/attributePool", nil)

	resp, _ := app.Test(req, 10)

	if resp.StatusCode != 404 {
		t.Errorf("Expected status code 404, got %v", resp.StatusCode)
	}
}

func testGetOfAttribPoolOnExistingPad(t *testing.T, tsStore testutils.TestDataStore) {
	var padText = "hallo"
	var _, err = tsStore.PadManager.GetPad("123", &padText, nil)
	if err != nil {
		t.Errorf("Error creating pad")
	}

	app := fiber.New()
	Init(app, tsStore.PadMessageHandler, tsStore.PadManager)
	req := httptest.NewRequest("GET", "/pads/123/attributePool", nil)

	resp, _ := app.Test(req, 10)

	var poolResponse AttributePoolResponse
	err = json.NewDecoder(resp.Body).Decode(&poolResponse)

	if err != nil {
		t.Errorf("Error decoding response")
	}

	if poolResponse.Pool.NextNum != 1 {
		t.Errorf("Expected next number to be 1, got %v", poolResponse.Pool.NextNum)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %v", resp.StatusCode)
	}
}
