package static

import (
	"testing"

	"github.com/ether/etherpad-go/lib/settings"
)

func TestManifestColoursFollowTheTheme(t *testing.T) {
	s := &settings.Settings{SkinName: "colibris"}
	if m := buildManifest(s, ""); m.ThemeColor != "#ffffff" || m.BackgroundColor != "#ffffff" {
		t.Errorf("etherpad-go theme: %q / %q", m.ThemeColor, m.BackgroundColor)
	}
	s.PadTheme.Name = settings.PadThemeColibris
	if m := buildManifest(s, ""); m.ThemeColor != "#0f775b" {
		t.Errorf("colibris theme: %q", m.ThemeColor)
	}
}
