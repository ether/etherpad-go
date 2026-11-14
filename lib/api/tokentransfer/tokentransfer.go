// Package tokentransfer moves an author's identity to another browser
// (etherpad-lite's /tokenTransfer): the first browser asks for a code, the
// second redeems it within five minutes, once, and gets the same author
// token as an HttpOnly cookie plus the pad preferences. The token never
// appears in a response body.
package tokentransfer

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
)

// TTL is how long a code can be redeemed.
const TTL = 5 * time.Minute

// maxPrefs bounds the preferences carried along.
const maxPrefs = 8 << 10

var idPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Handler serves POST /tokenTransfer and GET /tokenTransfer/:id.
type Handler struct {
	Store    db.DataStore
	Settings *settings.Settings
	Now      func() time.Time
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// normalizePrefs returns canonical JSON for a preferences object, or "".
func normalizePrefs(s string) string {
	if s == "" || len(s) > maxPrefs {
		return ""
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil || v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Create answers POST /tokenTransfer: a code for the requester's author
// token cookie (it is HttpOnly, so the browser cannot send it any other way).
func (h *Handler) Create(c fiber.Ctx) error {
	if !sameOrigin(c) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cross-site request"})
	}
	token := pad.AuthorTokenFromCookies(c.Cookies, h.Settings)
	if token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No author cookie to transfer"})
	}
	var body struct {
		Prefs     string `json:"prefs"`
		PrefsHttp string `json:"prefsHttp"` // etherpad-lite's name
	}
	_ = json.Unmarshal(c.Body(), &body)
	prefs := normalizePrefs(body.Prefs)
	if prefs == "" {
		prefs = normalizePrefs(body.PrefsHttp)
	}
	id, err := newID()
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	now := h.now()
	// Expired codes are dropped as new ones are made.
	_ = h.Store.RemoveTokenTransfersBefore(now.Add(-TTL))
	if err := h.Store.CreateTokenTransfer(db.TokenTransfer{ID: id, Token: token, Prefs: prefs, CreatedAt: now}); err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(fiber.Map{"id": id})
}

// sameOrigin reports whether the request comes from a page of this site.
// Browsers send Sec-Fetch-Site; older ones at least Origin on a POST. A
// request with neither (curl, a test) is not from another site's page.
func sameOrigin(c fiber.Ctx) bool {
	if site := c.Get("Sec-Fetch-Site"); site != "" {
		return site == "same-origin"
	}
	origin := c.Get(fiber.HeaderOrigin)
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, c.Host())
}

// Redeem answers POST /tokenTransfer/:id from a page of this site: 404 for
// an unknown or used code, 410 for an expired one; otherwise this browser
// takes over the author token. Not a GET, and never from another site: a
// link, an image or a form elsewhere must not swap a visitor's identity for
// one the attacker also holds. The welcome page asks before it redeems.
func (h *Handler) Redeem(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	if !sameOrigin(c) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cross-site request"})
	}
	id := c.Params("id")
	if !idPattern.MatchString(id) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Token not found"})
	}
	t, err := h.Store.TakeTokenTransfer(id)
	if errors.Is(err, db.ErrTokenTransferNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Token not found"})
	}
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	if t.CreatedAt.IsZero() || h.now().Sub(t.CreatedAt) > TTL {
		return c.Status(fiber.StatusGone).JSON(fiber.Map{"error": "Token expired"})
	}
	pad.SetAuthorTokenCookieLax(c, h.Settings, t.Token)
	prefs := normalizePrefs(t.Prefs)
	return c.JSON(fiber.Map{"ok": true, "prefs": prefs, "prefsHttp": prefs})
}

// Register adds the routes.
func Register(app fiber.Router, h *Handler) {
	app.Post("/tokenTransfer", h.Create)
	app.Post("/tokenTransfer/:id", h.Redeem)
	// etherpad-lite redeems with GET; here that would let any link swap the
	// identity, so it is refused.
	app.Get("/tokenTransfer/:id", func(c fiber.Ctx) error {
		c.Set(fiber.HeaderAllow, fiber.MethodPost)
		return c.Status(fiber.StatusMethodNotAllowed).JSON(fiber.Map{"error": "Use POST"})
	})
}
