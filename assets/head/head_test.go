package head

import (
	"context"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/settings"
)

func TestThemeOnHTML(t *testing.T) {
	s := &settings.Settings{SkinName: "colibris"}
	i := For(s, "en", "../", true)
	if i.Theme != settings.PadThemeEtherpadGo {
		t.Fatalf("default theme = %q", i.Theme)
	}
	if got := i.HTMLClass(); got != "pad theme-etherpad-go super-light-toolbar super-light-editor light-background" {
		t.Errorf("HTMLClass = %q", got)
	}
	if got := i.HTMLStyle(); got != "" {
		t.Errorf("HTMLStyle without accent = %q", got)
	}

	s.PadTheme = settings.PadTheme{Name: "colibris", AccentColor: "#7C3AED"}
	s.SkinVariants = "super-dark-toolbar"
	i = For(s, "en", "../", true)
	if got := i.HTMLClass(); got != "pad theme-colibris super-dark-toolbar" {
		t.Errorf("HTMLClass = %q", got)
	}
	if got := i.HTMLStyle(); got != "--primary-color: #7c3aed; --primary-text-color: #ffffff;" {
		t.Errorf("HTMLStyle = %q", got)
	}

	// Invalid accent colours never reach the style attribute.
	s.PadTheme.AccentColor = "#000; background: url(x)"
	if got := For(s, "en", "../", true).HTMLStyle(); got != "" {
		t.Errorf("HTMLStyle with invalid accent = %q", got)
	}

	// Another skin: no theme class, and the accent does not override its colours.
	s.SkinName = "custom"
	s.PadTheme.AccentColor = "#7c3aed"
	custom := For(s, "en", "../", true)
	if got := custom.HTMLClass(); strings.Contains(got, "theme-") {
		t.Errorf("HTMLClass on custom skin = %q", got)
	}
	if got := custom.HTMLStyle(); got != "" {
		t.Errorf("HTMLStyle on custom skin = %q", got)
	}
}

func TestViewportReachesUnderTheNotch(t *testing.T) {
	var b strings.Builder
	s := &settings.Settings{SkinName: "colibris"}
	if err := Meta(For(s, "en", "../", true)).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	html := b.String()
	for _, want := range []string{"viewport-fit=cover", `name="apple-mobile-web-app-capable"`, "apple-mobile-web-app-status-bar-style"} {
		if !strings.Contains(html, want) {
			t.Errorf("head misses %s", want)
		}
	}
}
