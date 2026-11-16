package io

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ether/etherpad-go/lib/io/pdffonts"
	"github.com/signintech/gopdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePdfFontConfig(t *testing.T) {
	cfg := parsePdfFontConfig(map[string]any{
		"Garamond": "fonts/garamond.ttf",
		"Arial": map[string]any{
			"regular": "/abs/arial.ttf", "bold": "arialbd.ttf",
			"bolditalic": "arialbi.ttf", // settings keys may arrive lowercased
			"fallback":   "serif",
		},
		"NoRegular": map[string]any{"bold": "x.ttf"},
		"":          "ignored.ttf",
	}, "/root")

	require.Contains(t, cfg, "garamond")
	assert.Equal(t, "/root/fonts/garamond.ttf", cfg["garamond"].files[pdffonts.Regular])
	assert.Equal(t, pdffonts.Sans, cfg["garamond"].fallback, "no fallback: sans serif")

	require.Contains(t, cfg, "arial")
	a := cfg["arial"]
	assert.Equal(t, "/abs/arial.ttf", a.files[pdffonts.Regular])
	assert.Equal(t, "/root/arialbd.ttf", a.files[pdffonts.Bold])
	assert.Equal(t, "/root/arialbi.ttf", a.files[pdffonts.BoldItalic])
	assert.Empty(t, a.files[pdffonts.Italic])
	assert.Equal(t, pdffonts.Serif, a.fallback)

	assert.NotContains(t, cfg, "noregular", "a family needs a regular face")
	assert.Len(t, cfg, 2)

	c, ok := fallbackCategory("Courier")
	assert.True(t, ok)
	assert.Equal(t, pdffonts.Mono, c, "etherpad-lite's built-in names")
}

func newTestFonts(t *testing.T, custom map[string]customFont) *pdfFonts {
	t.Helper()
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()
	return newPdfFonts(pdf, custom, nil)
}

func TestPdfFontsBuiltins(t *testing.T) {
	f := newTestFonts(t, nil)
	for _, tc := range []struct {
		family string
		v      pdffonts.Variant
		want   string
	}{
		{"", pdffonts.Regular, "LiberationSans-Regular"},
		{"", pdffonts.Italic, "LiberationSans-Italic"},
		{"times new roman", pdffonts.BoldItalic, "LiberationSerif-BoldItalic"},
		{"garamond", pdffonts.Regular, "LiberationSerif-Regular"},
		{"courier", pdffonts.Bold, "LiberationMono-Bold"},
		{"no such family", pdffonts.Regular, "LiberationSans-Regular"},
	} {
		got, err := f.fontFor(tc.family, tc.v)
		require.NoError(t, err)
		assert.Equal(t, tc.want, got, "%q %s", tc.family, tc.v)
	}
}

func TestPdfFontsCustom(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular.ttf")
	data, err := pdffonts.Builtin(pdffonts.Serif, pdffonts.Regular)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(regular, data, 0o644))

	f := newTestFonts(t, parsePdfFontConfig(map[string]any{
		"Fancy":   map[string]any{"regular": regular},
		"Missing": map[string]any{"regular": filepath.Join(dir, "nope.ttf"), "fallback": "monospace"},
		// A configured family wins over the built-in mapping.
		"Arial": regular,
	}, dir))

	got, err := f.fontFor("fancy", pdffonts.Regular)
	require.NoError(t, err)
	assert.Equal(t, "custom:fancy:Regular", got)
	got, err = f.fontFor("fancy", pdffonts.Bold)
	require.NoError(t, err)
	assert.Equal(t, "custom:fancy:Bold", got, "no bold file: the regular one")

	got, err = f.fontFor("missing", pdffonts.Italic)
	require.NoError(t, err)
	assert.Equal(t, "LiberationMono-Italic", got, "an unreadable file falls back, the export goes on")

	got, err = f.fontFor("arial", pdffonts.Regular)
	require.NoError(t, err)
	assert.Equal(t, "custom:arial:Regular", got)
}

// A font replaced in place is used by the next export.
func TestFontFileCacheNoticesChanges(t *testing.T) {
	file := filepath.Join(t.TempDir(), "f.ttf")
	require.NoError(t, os.WriteFile(file, []byte("one"), 0o644))
	got, err := readFontFile(file)
	require.NoError(t, err)
	assert.Equal(t, "one", string(got))
	require.NoError(t, os.WriteFile(file, []byte("second"), 0o644))
	got, err = readFontFile(file)
	require.NoError(t, err)
	assert.Equal(t, "second", string(got))
	require.NoError(t, os.Remove(file))
	_, err = readFontFile(file)
	assert.Error(t, err)
}
