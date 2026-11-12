package pad

import (
	"net/url"
	"strings"

	padManager "github.com/ether/etherpad-go/lib/pad"
	"github.com/gofiber/fiber/v3"
)

// GuardPadURL is etherpad-lite's padurlsanitize for routes with a :pad
// parameter. It redirects to the sanitized id when one exists ("/p/foo:bar"
// to "/p/foo_bar", whitespace to "_") and answers 404 for an id that is not
// valid and names no existing pad, since opening a pad URL creates the pad.
// It returns true when it has answered the request.
func GuardPadURL(c fiber.Ctx, pm *padManager.Manager) (bool, error) {
	raw := c.Params("pad")
	padID := raw
	if unescaped, err := url.PathUnescape(raw); err == nil {
		padID = unescaped
	}

	sanitized, err := pm.SanitizePadId(padID)
	if err != nil {
		return true, err
	}
	ok, err := pm.CanOpenPadId(sanitized)
	if err != nil {
		return true, err
	}
	if !ok {
		return true, c.Status(fiber.StatusNotFound).SendString("Such a padname is forbidden")
	}
	if sanitized == padID {
		return false, nil
	}

	// A relative Location keeps a reverse proxy's path prefix intact: step up
	// once per segment that follows the pad id ("/timeslider", "/export/txt").
	path := c.Path()
	rest := ""
	if i := strings.Index(path, "/p/"); i >= 0 {
		afterPrefix := path[i+len("/p/"):]
		if j := strings.Index(afterPrefix, "/"); j >= 0 {
			rest = afterPrefix[j:]
		}
	}
	location := strings.Repeat("../", strings.Count(rest, "/")) + url.PathEscape(sanitized) + rest
	if q := string(c.Request().URI().QueryString()); q != "" {
		location += "?" + q
	}
	c.Set(fiber.HeaderLocation, location)
	return true, c.Status(fiber.StatusFound).SendString(`You should be redirected to <a href="` + location + `">` + location + `</a>`)
}
