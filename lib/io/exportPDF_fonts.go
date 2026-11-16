package io

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ether/etherpad-go/lib/io/pdffonts"
	"github.com/signintech/gopdf"
	"go.uber.org/zap"
)

// customFont is a family an operator gave font files for in the
// exportPdfFonts setting (etherpad-lite's setting of that name):
//
//	"exportPdfFonts": {
//	  "Garamond": "fonts/EBGaramond-Regular.ttf",
//	  "Arial": {"regular": "/usr/share/fonts/arial.ttf", "bold": "...", "italic": "...",
//	            "boldItalic": "...", "fallback": "sans-serif"}
//	}
//
// A face without a file uses the regular one; a file that cannot be read or
// loaded falls back to the built-in font of the fallback kind (sans serif
// when none is given). A font never makes an export fail.
type customFont struct {
	files    [4]string // by pdffonts.Variant; "" when not given
	fallback pdffonts.Category
}

// parsePdfFontConfig reads the exportPdfFonts setting. Keys are family names
// (normalised like the pad's), values a file or an object with the faces;
// relative paths are relative to root. Settings keys may arrive lowercased.
func parsePdfFontConfig(raw map[string]any, root string) map[string]customFont {
	out := map[string]customFont{}
	resolve := func(v any) string {
		s, ok := v.(string)
		if !ok || s == "" {
			return ""
		}
		if !filepath.IsAbs(s) {
			s = filepath.Join(root, s)
		}
		return s
	}
	for family, value := range raw {
		key := pdffonts.NormalizeFamily(family)
		if key == "" {
			continue
		}
		var cf customFont
		switch v := value.(type) {
		case string:
			cf.files[pdffonts.Regular] = resolve(v)
		case map[string]any:
			get := func(name string) any {
				for k, val := range v {
					if strings.EqualFold(k, name) {
						return val
					}
				}
				return nil
			}
			cf.files[pdffonts.Regular] = resolve(get("regular"))
			cf.files[pdffonts.Bold] = resolve(get("bold"))
			cf.files[pdffonts.Italic] = resolve(get("italic"))
			cf.files[pdffonts.BoldItalic] = resolve(get("boldItalic"))
			if fb, ok := get("fallback").(string); ok {
				if c, known := fallbackCategory(fb); known {
					cf.fallback = c
				}
			}
		}
		if cf.files[pdffonts.Regular] == "" {
			continue // nothing usable
		}
		out[key] = cf
	}
	return out
}

// fallbackCategory accepts etherpad-lite's built-in names (helvetica, times,
// courier) and any family the mapping knows ("serif", "monospace").
func fallbackCategory(name string) (pdffonts.Category, bool) {
	switch n := pdffonts.NormalizeFamily(name); n {
	case "helvetica":
		return pdffonts.Sans, true
	case "times":
		return pdffonts.Serif, true
	case "courier":
		return pdffonts.Mono, true
	default:
		return pdffonts.CategoryOf(n)
	}
}

// Font files are read once per process. The cache is keyed by the file's
// modification time and size as well, so a font replaced in place, or put
// where a path pointed to nothing, is used by the next export without a
// restart. Failures are not cached, for the same reason.
type cachedFontFile struct {
	stamp string
	data  []byte
}

var fontFileCache sync.Map // path -> cachedFontFile

func readFontFile(path string) ([]byte, error) {
	st, err := os.Stat(path)
	if err != nil {
		fontFileCache.Delete(path)
		return nil, err
	}
	stamp := fmt.Sprintf("%d:%d", st.ModTime().UnixNano(), st.Size())
	if c, ok := fontFileCache.Load(path); ok && c.(cachedFontFile).stamp == stamp {
		return c.(cachedFontFile).data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fontFileCache.Delete(path)
		return nil, err
	}
	fontFileCache.Store(path, cachedFontFile{stamp: stamp, data: data})
	return data, nil
}

// pdfFonts registers the fonts one document uses, on first use: an export
// reads only the faces its text needs.
type pdfFonts struct {
	pdf        *gopdf.GoPdf
	custom     map[string]customFont
	registered map[string]bool
	failed     map[string]bool
	logger     *zap.SugaredLogger
}

func newPdfFonts(pdf *gopdf.GoPdf, custom map[string]customFont, logger *zap.SugaredLogger) *pdfFonts {
	return &pdfFonts{pdf: pdf, custom: custom, registered: map[string]bool{}, failed: map[string]bool{}, logger: logger}
}

func (f *pdfFonts) warn(format string, args ...any) {
	if f.logger != nil {
		f.logger.Warnf(format, args...)
	}
}

// builtin registers a built-in face and returns its name.
func (f *pdfFonts) builtin(c pdffonts.Category, v pdffonts.Variant) (string, error) {
	name := pdffonts.Name(c, v)
	if f.registered[name] {
		return name, nil
	}
	data, err := pdffonts.Builtin(c, v)
	if err != nil {
		return "", err
	}
	if err := f.pdf.AddTTFFontByReader(name, bytes.NewReader(data)); err != nil {
		return "", err
	}
	f.registered[name] = true
	return name, nil
}

// fontFor returns the font for text of family (normalised; "" for the
// pad's default) in a face, registering it first.
func (f *pdfFonts) fontFor(family string, v pdffonts.Variant) (string, error) {
	category := pdffonts.Sans
	if cf, ok := f.custom[family]; ok {
		name := "custom:" + family + ":" + v.String()
		if f.registered[name] {
			return name, nil
		}
		if !f.failed[name] {
			file := cf.files[v]
			if file == "" {
				file = cf.files[pdffonts.Regular]
			}
			data, err := readFontFile(file)
			if err == nil {
				err = f.pdf.AddTTFFontByReader(name, bytes.NewReader(data))
			}
			if err == nil {
				f.registered[name] = true
				return name, nil
			}
			f.warn("PDF export: cannot use font file %q for %q: %v", file, family, err)
			f.failed[name] = true
		}
		category = cf.fallback
	} else if c, ok := pdffonts.CategoryOf(family); ok {
		category = c
	}
	return f.builtin(category, v)
}
