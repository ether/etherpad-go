// Package skin knows what the server needs about the skins: the toolbar
// colour of the colibris variants, for the theme-color meta.
package skin

import "strings"

// toolbarColorsInCSSOrder mirrors --bg-color of the toolbar variants in
// colibris' pad-variants.css and ui/src/js/skin_variants.ts. When several
// *-toolbar tokens are set, the rule defined last in the CSS wins.
var toolbarColorsInCSSOrder = [][2]string{
	{"super-light-toolbar", "#ffffff"},
	{"light-toolbar", "#f2f3f4"},
	{"super-dark-toolbar", "#485365"},
	{"dark-toolbar", "#576273"},
}

// DefaultToolbarColor is colibris' toolbar colour without a variant.
const DefaultToolbarColor = "#ffffff"

// ToolbarColorForTokens is the toolbar colour of a set of skin variants.
func ToolbarColorForTokens(tokens []string) string {
	set := map[string]bool{}
	for _, t := range tokens {
		set[t] = true
	}
	color := DefaultToolbarColor
	for _, vc := range toolbarColorsInCSSOrder {
		if set[vc[0]] {
			color = vc[1]
		}
	}
	return color
}

// ConfiguredToolbarColor is the toolbar colour at first paint, or "" for a
// skin other than colibris, whose colours are not known.
func ConfiguredToolbarColor(skinName, skinVariants string) string {
	if skinName != "colibris" {
		return ""
	}
	return ToolbarColorForTokens(strings.Fields(skinVariants))
}

// DarkToolbarColor is the toolbar colour colibris switches to on a client
// in dark mode, or "" for another skin.
func DarkToolbarColor(skinName string) string {
	if skinName != "colibris" {
		return ""
	}
	return ToolbarColorForTokens([]string{"super-dark-toolbar"})
}
