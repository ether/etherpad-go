package pad

import (
	"encoding/json"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/gofiber/fiber/v2"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGetOnText(t *testing.T) {
	os.Setenv("ETHERPAD_DB_TYPE", "memory")
	app := fiber.New()
	Init(app)
	req := httptest.NewRequest("GET", "/pads/123/text", nil)

	resp, _ := app.Test(req, 10)

	if resp.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %v", resp.StatusCode)
	}
}

func TestGetOfAttribPoolOnNonExistingPad(t *testing.T) {
	os.Setenv("ETHERPAD_DB_TYPE", "memory")
	app := fiber.New()
	Init(app)
	req := httptest.NewRequest("GET", "/pads/123/attributePool", nil)

	resp, _ := app.Test(req, 10)

	if resp.StatusCode != 404 {
		t.Errorf("Expected status code 404, got %v", resp.StatusCode)
	}
}

func TestGetOfAttribPoolOnExistingPad(t *testing.T) {
	os.Setenv("ETHERPAD_DB_TYPE", "memory")
	var manager = pad.NewManager()
	var padText = "hallo"
	var _, err = manager.GetPad("123", &padText, nil)
	if err != nil {
		t.Errorf("Error creating pad")
	}

	app := fiber.New()
	Init(app)
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
