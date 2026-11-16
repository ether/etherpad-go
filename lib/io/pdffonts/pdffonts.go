// Package pdffonts holds the fonts of the PDF export and maps CSS font
// families to them.
//
// The PDF embeds its fonts, so only fonts that may be redistributed can be
// used: the Liberation fonts (SIL OFL 1.1, see LICENSE), metric-compatible
// with Arial, Times New Roman and Courier New. A family the pad names
// (ep_font_family, "Garamond") is set in the one of the three that is the
// same kind of face, like etherpad-lite does with the PDF built-in fonts
// Helvetica, Times and Courier: not the exact face, but the right kind.
// Operators can give real font files for a family with the exportPdfFonts
// setting (lib/io/exportPDF_fonts.go).
package pdffonts

import (
	"embed"
	"regexp"
	"strings"
)

//go:embed *.ttf
var files embed.FS

// Category is the kind of face a family is set in.
type Category int

const (
	Sans Category = iota
	Serif
	Mono
)

// Variant is one of a family's four faces.
type Variant int

const (
	Regular Variant = iota
	Bold
	Italic
	BoldItalic
)

// VariantOf is the face for bold and italic text.
func VariantOf(bold, italic bool) Variant {
	switch {
	case bold && italic:
		return BoldItalic
	case bold:
		return Bold
	case italic:
		return Italic
	}
	return Regular
}

func (v Variant) String() string {
	return [...]string{"Regular", "Bold", "Italic", "BoldItalic"}[v]
}

func (c Category) String() string {
	return [...]string{"LiberationSans", "LiberationSerif", "LiberationMono"}[c]
}

// Name is the font name of a built-in face, also its file name without
// ".ttf".
func Name(c Category, v Variant) string {
	return c.String() + "-" + v.String()
}

// Builtin returns the TTF of a built-in face.
func Builtin(c Category, v Variant) ([]byte, error) {
	return files.ReadFile(Name(c, v) + ".ttf")
}

var categoryOf = map[string]Category{}

func mapFamilies(c Category, names ...string) {
	for _, n := range names {
		categoryOf[n] = c
	}
}

// etherpad-lite's lists (ExportPdfNative.ts): the generic CSS families
// first, then the widely used concrete faces by the kind they are.
func init() {
	mapFamilies(Sans,
		"sans serif", "sans", "system ui", "ui sans serif", "fantasy",
		"helvetica", "helvetica neue", "arial", "arial black", "arial narrow",
		"liberation sans", "nimbus sans", "freesans", "verdana", "tahoma",
		"trebuchet ms", "segoe ui", "calibri", "candara", "geneva", "avant garde",
		"century gothic", "futura", "gill sans", "lucida grande", "lucida sans",
		"lucida sans unicode", "dejavu sans", "noto sans", "open sans", "roboto",
		"lato", "ubuntu", "franklin gothic medium", "impact", "optima", "inter",
	)
	mapFamilies(Serif,
		"serif", "ui serif", "cursive",
		"times", "times new roman", "liberation serif", "nimbus roman", "freeserif",
		"georgia", "garamond", "eb garamond", "palatino", "palatino linotype",
		"book antiqua", "bookman", "bookman old style", "itc bookman",
		"century schoolbook", "new century schoolbook", "cambria", "constantia",
		"baskerville", "didot", "hoefler text", "dejavu serif", "noto serif",
		"pt serif", "merriweather", "charter", "utopia", "rockwell",
	)
	mapFamilies(Mono,
		"monospace", "mono", "ui monospace",
		"courier", "courier new", "liberation mono", "nimbus mono", "freemono",
		"consolas", "monaco", "menlo", "andale mono", "lucida console",
		"dejavu sans mono", "noto sans mono", "roboto mono", "source code pro",
		"fira code", "fira mono", "ibm plex mono", "inconsolata", "pt mono",
		"sf mono", "cascadia code", "cascadia mono", "jetbrains mono",
		"ubuntu mono",
	)
}

// CategoryOf returns the kind of face of a family name (normalised).
func CategoryOf(family string) (Category, bool) {
	c, ok := categoryOf[family]
	return c, ok
}

var separators = regexp.MustCompile(`[\s_-]+`)

// NormalizeFamily lowercases a family name, removes its quotes and treats
// hyphens, underscores and runs of space as one space, so CSS names ("Times
// New Roman") and the names plugins write (times-new-roman) are the same
// key. The name comes from pad content and is only ever looked up, never
// used as a path.
func NormalizeFamily(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, `"'`)
	s = strings.ToLower(s)
	s = separators.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
