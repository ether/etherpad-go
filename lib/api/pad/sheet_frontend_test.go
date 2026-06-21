package pad

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestHandleSheetOpenRenders(t *testing.T) {
	app := fiber.New()
	app.Get("/s/:pad", func(c fiber.Ctx) error {
		return HandleSheetOpen(c)
	})
	req := httptest.NewRequest("GET", "/s/demo", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "sheet-root") {
		t.Fatalf("expected sheet shell in body, got: %s", string(body))
	}
}
