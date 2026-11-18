package sheetio

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestFromThisSite(t *testing.T) {
	app := fiber.New()
	app.Post("/", func(c fiber.Ctx) error {
		if err := fromThisSite(c); err != nil {
			return err
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
	for _, tc := range []struct {
		name          string
		fetchSite     string
		origin        string
		wantForbidden bool
	}{
		{"same origin", "same-origin", "http://pad.example", false},
		{"typed by the user", "none", "", false},
		{"other site", "cross-site", "https://evil.example", true},
		{"other subdomain", "same-site", "http://x.pad.example", true},
		{"old browser, own origin", "", "http://pad.example", false},
		{"old browser, other origin", "", "https://evil.example", true},
		{"no browser", "", "", false},
	} {
		req := httptest.NewRequest("POST", "http://pad.example/", nil)
		if tc.fetchSite != "" {
			req.Header.Set("Sec-Fetch-Site", tc.fetchSite)
		}
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if got := res.StatusCode == fiber.StatusForbidden; got != tc.wantForbidden {
			t.Errorf("%s: status %d", tc.name, res.StatusCode)
		}
	}
}
