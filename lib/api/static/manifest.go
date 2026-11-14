package static

import (
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
)

type manifestIcon struct {
	Src   string `json:"src"`
	Sizes string `json:"sizes"`
	Type  string `json:"type"`
}

type webManifest struct {
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	Description     string         `json:"description"`
	Icons           []manifestIcon `json:"icons"`
	StartURL        string         `json:"start_url"`
	Display         string         `json:"display"`
	ThemeColor      string         `json:"theme_color"`
	BackgroundColor string         `json:"background_color"`
}

// buildManifest is etherpad-lite's PWA manifest (hooks/express/pwa.ts),
// with every path behind the proxy prefix so an installed app opens and
// finds its icons under it.
func buildManifest(s *settings.Settings, proxyPath string) webManifest {
	name := s.Title
	if name == "" {
		name = "Etherpad"
	}
	return webManifest{
		Name:        name,
		ShortName:   s.Title,
		Description: "A collaborative online editor",
		Icons: []manifestIcon{
			{Src: proxyPath + "/images/fond.jpg", Sizes: "512x512", Type: "image/jpeg"},
			{Src: proxyPath + "/favicon.ico", Sizes: "64x64 32x32 24x24 16x16", Type: "image/x-icon"},
		},
		StartURL:        proxyPath + "/",
		Display:         "fullscreen",
		ThemeColor:      "#0f775b",
		BackgroundColor: "#0f775b",
	}
}

func serveManifest(s *settings.Settings) fiber.Handler {
	return func(c fiber.Ctx) error {
		m := buildManifest(s, utils.RequestProxyPath(c, s.TrustProxy))
		c.Set(fiber.HeaderContentType, "application/manifest+json; charset=utf-8")
		return c.JSON(m, "application/manifest+json; charset=utf-8")
	}
}
