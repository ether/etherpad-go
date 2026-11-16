package pdffonts

import (
	"bytes"
	"testing"

	"github.com/signintech/gopdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeFamily(t *testing.T) {
	for in, want := range map[string]string{
		`"Times New Roman"`: "times new roman",
		"times-new-roman":   "times new roman",
		"  Courier_New ":    "courier new",
		"'Helvetica  Neue'": "helvetica neue",
		"":                  "",
	} {
		assert.Equal(t, want, NormalizeFamily(in), in)
	}
}

// Every family ep_font_family writes lands on a kind of face.
func TestFontFamilyPluginValues(t *testing.T) {
	for family, want := range map[string]Category{
		"arial": Sans, "avant-garde": Sans, "bookman": Serif, "calibri": Sans,
		"courier": Mono, "garamond": Serif, "helvetica": Sans, "monospace": Mono,
		"palatino": Serif, "times-new-roman": Serif,
	} {
		got, ok := CategoryOf(NormalizeFamily(family))
		require.True(t, ok, family)
		assert.Equal(t, want, got, family)
	}
	_, ok := CategoryOf("constructor")
	assert.False(t, ok, "an unknown name is no family")
}

func TestBuiltinsLoad(t *testing.T) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	for _, c := range []Category{Sans, Serif, Mono} {
		for _, v := range []Variant{Regular, Bold, Italic, BoldItalic} {
			data, err := Builtin(c, v)
			require.NoError(t, err, Name(c, v))
			require.NoError(t, pdf.AddTTFFontByReader(Name(c, v), bytes.NewReader(data)), Name(c, v))
		}
	}
	assert.Equal(t, BoldItalic, VariantOf(true, true))
	assert.Equal(t, Italic, VariantOf(false, true))
}
