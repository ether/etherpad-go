package pad

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
)

func TestHandleSheetOpenRenders(t *testing.T) {
	app := fiber.New()
	app.Get("/s/:pad", func(c fiber.Ctx) error {
		return HandleSheetOpen(c, nil)
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

func TestSheetPageTheme(t *testing.T) {
	s := &settings.Settings{EnableDarkMode: true}
	if got := SheetPage(s).HTMLClass(); got != "theme-etherpad-go sheet-dark-allowed" {
		t.Errorf("default = %q", got)
	}
	s.PadTheme = settings.PadTheme{Name: "colibris", AccentColor: "#7C3AED"}
	s.EnableDarkMode = false
	p := SheetPage(s)
	if got := p.HTMLClass(); got != "theme-colibris" {
		t.Errorf("colibris = %q", got)
	}
	if p.Style != "--primary-color: #7c3aed;" {
		t.Errorf("style = %q", p.Style)
	}
	s.PadTheme.AccentColor = "red; x: y"
	if got := SheetPage(s).Style; got != "" {
		t.Errorf("invalid accent = %q", got)
	}
	if got := SheetPage(nil).HTMLClass(); got != "" {
		t.Errorf("nil settings = %q", got)
	}

	app := fiber.New()
	app.Get("/s/:pad", func(c fiber.Ctx) error { return HandleSheetOpen(c, &settings.Settings{EnableDarkMode: true}) })
	resp, err := app.Test(httptest.NewRequest("GET", "/s/demo", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `class="theme-etherpad-go sheet-dark-allowed"`) {
		t.Errorf("html class missing: %s", body)
	}
}
