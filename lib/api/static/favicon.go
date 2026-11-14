package static

import (
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ether/etherpad-go/lib"
	"github.com/gofiber/fiber/v3"
)

// serveFavicon answers /favicon.ico like etherpad-lite: the favicon setting
// (a URL is redirected to, a file path relative to the Etherpad directory
// or absolute is served), else the skin's favicon.ico, else the default.
// A configured file that does not exist falls back, so the old default
// "favicon.ico" keeps working.
func serveFavicon(store *lib.InitStore) fiber.Handler {
	return func(c fiber.Ctx) error {
		s := store.RetrievedSettings
		if s.Favicon != nil && *s.Favicon != "" {
			fav := *s.Favicon
			if u, err := url.Parse(fav); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
				return c.Redirect().Status(fiber.StatusFound).To(fav)
			}
			p := fav
			if !filepath.IsAbs(p) {
				p = filepath.Join(s.Root, p)
			}
			if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() {
				return c.SendFile(p)
			}
		}
		if s.SkinName != "" && !strings.ContainsAny(s.SkinName, `/\`) && s.SkinName != ".." {
			if b, err := fs.ReadFile(store.UiAssets, path.Join("assets/css/skin", s.SkinName, "favicon.ico")); err == nil {
				c.Set(fiber.HeaderContentType, "image/x-icon")
				return c.Send(b)
			}
		}
		b, err := fs.ReadFile(store.UiAssets, "assets/images/favicon.ico")
		if err != nil {
			return c.SendStatus(fiber.StatusNotFound)
		}
		c.Set(fiber.HeaderContentType, "image/x-icon")
		return c.Send(b)
	}
}
