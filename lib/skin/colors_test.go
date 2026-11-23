package skin

import "testing"

// Port of etherpad-lite's SkinColors spec.
func TestToolbarColors(t *testing.T) {
	cases := []struct{ skin, variants, want string }{
		{"colibris", "super-light-toolbar super-light-editor light-background", "#ffffff"},
		{"colibris", "dark-toolbar dark-editor dark-background", "#576273"},
		{"colibris", "", "#ffffff"},
		{"colibris", "light-toolbar super-dark-toolbar", "#485365"}, // the CSS rule defined last wins
		{"no-skin", "super-dark-toolbar", ""},
		{"", "dark-toolbar", ""},
	}
	for _, c := range cases {
		if got := ConfiguredToolbarColor(c.skin, c.variants); got != c.want {
			t.Errorf("ConfiguredToolbarColor(%q, %q) = %q, want %q", c.skin, c.variants, got, c.want)
		}
	}
	if got := DarkToolbarColor("colibris"); got != "#485365" {
		t.Errorf("DarkToolbarColor(colibris) = %q", got)
	}
	if got := DarkToolbarColor("no-skin"); got != "" {
		t.Errorf("DarkToolbarColor(no-skin) = %q", got)
	}
}

func TestToolbarColorForTheme(t *testing.T) {
	cases := []struct {
		skin, theme, variants, want string
	}{
		{"colibris", "etherpad-go", "super-light-toolbar super-light-editor light-background", "#ffffff"},
		{"colibris", "etherpad-go", "light-toolbar", "#f7f9fb"},
		{"colibris", "etherpad-go", "super-dark-toolbar", "#111827"},
		{"colibris", "etherpad-go", "", "#ffffff"},
		// Classic look keeps the colibris colours.
		{"colibris", "colibris", "super-dark-toolbar", "#485365"},
		{"other", "", "super-dark-toolbar", ""},
	}
	for _, c := range cases {
		if got := ConfiguredToolbarColorForTheme(c.skin, c.theme, c.variants); got != c.want {
			t.Errorf("ConfiguredToolbarColorForTheme(%q, %q, %q) = %q, want %q", c.skin, c.theme, c.variants, got, c.want)
		}
	}
	if got := DarkToolbarColorForTheme("colibris", "etherpad-go"); got != "#111827" {
		t.Errorf("dark etherpad-go = %q", got)
	}
	if got := DarkToolbarColorForTheme("colibris", "colibris"); got != "#485365" {
		t.Errorf("dark colibris = %q", got)
	}
}
