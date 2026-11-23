package settings

import "testing"

func TestPadThemeResolved(t *testing.T) {
	cases := []struct {
		name, skin, want string
	}{
		{"", "colibris", PadThemeEtherpadGo},
		{"", "", PadThemeEtherpadGo},
		{"etherpad-go", "colibris", PadThemeEtherpadGo},
		{"Colibris", "colibris", PadThemeColibris},
		{" colibris ", "colibris", PadThemeColibris},
		{"unknown", "colibris", PadThemeEtherpadGo},
		// Other skins bring their own colours.
		{"etherpad-go", "my-skin", ""},
		{"colibris", "my-skin", ""},
	}
	for _, c := range cases {
		if got := (PadTheme{Name: c.name}).Resolved(c.skin); got != c.want {
			t.Errorf("Resolved(%q, skin %q) = %q, want %q", c.name, c.skin, got, c.want)
		}
	}
}

func TestPadThemeAccent(t *testing.T) {
	cases := map[string]string{
		"":                    "",
		"#7C3AED":             "#7c3aed",
		" #abc ":              "#abc",
		"#abcd":               "",
		"red":                 "",
		"#123456; color: red": "",
		"#123456\"><script>":  "",
		"var(--x)":            "",
		"#1234567":            "",
	}
	for in, want := range cases {
		if got := (PadTheme{AccentColor: in}).Accent(); got != want {
			t.Errorf("Accent(%q) = %q, want %q", in, got, want)
		}
	}
}
