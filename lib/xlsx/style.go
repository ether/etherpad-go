package xlsx

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/xuri/excelize/v2"
)

// Mapping between the sheet model's allowlisted style props (see
// sheet.ValidateProps) and excelize styles / xlsx dimensions.

// px <-> xlsx unit conversions. Column width is in "character" units
// (px ≈ w*7+5 for the default Calibri 11); row height is in points.
func colWidthToPx(w float64) int   { return int(math.Round(w*7 + 5)) }
func pxToColWidth(px int) float64  { return float64(px-5) / 7 }
func rowHeightToPx(h float64) int  { return int(math.Round(h * 96 / 72)) }
func pxToRowHeight(px int) float64 { return float64(px) * 72 / 96 }

// numFmtToCode maps the model's symbolic numFmt (general|text|date|
// number[:d]|currency[:d]|percent[:d]) to an xlsx format code.
func numFmtToCode(numFmt string) string {
	kind, dec, hasDec := strings.Cut(numFmt, ":")
	frac := ""
	if hasDec {
		if n := atoiSafe(dec); n > 0 {
			frac = "." + strings.Repeat("0", n)
		}
	}
	switch kind {
	case "text":
		return "@"
	case "date":
		return "m/d/yyyy" // matches the client's en-US display (format.ts)
	case "number":
		return "#,##0" + frac
	case "currency":
		if !hasDec {
			frac = ".00" // Intl currency default
		}
		return "$#,##0" + frac
	case "percent":
		return "0" + frac + "%"
	}
	return "" // general
}

var fracRe = regexp.MustCompile(`\.(0+)`)

// builtin xlsx numFmt ids -> format codes for the ones we can represent.
var builtinNumFmt = map[int]string{
	1: "0", 2: "0.00", 3: "#,##0", 4: "#,##0.00",
	5: "$#,##0", 6: "$#,##0", 7: "$#,##0.00", 8: "$#,##0.00",
	9: "0%", 10: "0.00%",
	14: "m/d/yyyy", 15: "d-mmm-yy", 16: "d-mmm", 17: "mmm-yy", 22: "m/d/yy h:mm",
	37: "#,##0", 38: "#,##0", 39: "#,##0.00", 40: "#,##0.00",
	44: "$#,##0.00", 49: "@",
}

// codeToNumFmt classifies an xlsx format code back into the symbolic model
// vocabulary; "" means general / unrepresentable (prop omitted).
func codeToNumFmt(code string) string {
	if code == "" || strings.EqualFold(code, "general") {
		return ""
	}
	if code == "@" {
		return "text"
	}
	dec := 0
	if m := fracRe.FindStringSubmatch(code); m != nil {
		dec = len(m[1])
	}
	if dec > 99 {
		dec = 99 // validator caps at 2 digits
	}
	low := strings.ToLower(code)
	switch {
	case strings.ContainsAny(low, "ymd") && !strings.Contains(low, "red"): // date letters; "[Red]" is a color, not a date
		return "date"
	case strings.Contains(code, "%"):
		return fmt.Sprintf("percent:%d", dec)
	case strings.ContainsAny(code, "$€£") || strings.Contains(code, "[$"):
		return fmt.Sprintf("currency:%d", dec)
	case strings.ContainsAny(code, "0#"):
		return fmt.Sprintf("number:%d", dec)
	}
	return ""
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// normalizeHex converts an excelize color ("FF0000", "FFFF0000" ARGB, with or
// without '#') to "#rrggbb", or "" if it isn't a plain hex color.
func normalizeHex(c string) string {
	c = strings.TrimPrefix(c, "#")
	if len(c) == 8 {
		c = c[2:] // drop ARGB alpha
	}
	if len(c) != 6 {
		return ""
	}
	for _, r := range c {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return ""
		}
	}
	return "#" + strings.ToLower(c)
}

// expandHex turns a model color ("#abc" or "#aabbcc") into excelize's
// six-digit form without '#'.
func expandHex(c string) string {
	c = strings.TrimPrefix(c, "#")
	if len(c) == 3 {
		c = string([]byte{c[0], c[0], c[1], c[1], c[2], c[2]})
	}
	return c
}

// propsToStyle builds an excelize style from model props.
func propsToStyle(props map[string]string) *excelize.Style {
	st := &excelize.Style{}
	font := &excelize.Font{}
	hasFont := false
	if props["bold"] == "1" {
		font.Bold, hasFont = true, true
	}
	if props["italic"] == "1" {
		font.Italic, hasFont = true, true
	}
	if props["underline"] == "1" {
		font.Underline, hasFont = "single", true
	}
	if c := props["color"]; c != "" {
		font.Color, hasFont = expandHex(c), true
	}
	if fam := props["fontFamily"]; fam != "" {
		font.Family, hasFont = fam, true
	}
	if sz := atoiSafe(props["fontSize"]); sz > 0 {
		font.Size, hasFont = float64(sz), true
	}
	if hasFont {
		st.Font = font
	}
	if bg := props["bg"]; bg != "" {
		st.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{expandHex(bg)}}
	}
	if props["align"] != "" || props["wrap"] == "1" {
		st.Alignment = &excelize.Alignment{Horizontal: props["align"], WrapText: props["wrap"] == "1"}
	}
	if props["border"] == "all" {
		for _, side := range []string{"left", "right", "top", "bottom"} {
			st.Border = append(st.Border, excelize.Border{Type: side, Style: 1, Color: "000000"})
		}
	}
	if code := numFmtToCode(props["numFmt"]); code != "" {
		st.CustomNumFmt = &code
	}
	return st
}

// styleToProps converts an excelize style back to model props. Only values the
// allowlist accepts are emitted; everything else is dropped silently.
func styleToProps(st *excelize.Style) map[string]string {
	props := map[string]string{}
	if f := st.Font; f != nil {
		if f.Bold {
			props["bold"] = "1"
		}
		if f.Italic {
			props["italic"] = "1"
		}
		if f.Underline != "" && f.Underline != "none" {
			props["underline"] = "1"
		}
		if c := normalizeHex(f.Color); c != "" {
			props["color"] = c
		}
		if f.Family != "" {
			props["fontFamily"] = f.Family
		}
		if sz := int(math.Round(f.Size)); sz >= 6 && sz <= 96 {
			props["fontSize"] = fmt.Sprintf("%d", sz)
		}
	}
	if st.Fill.Type == "pattern" && st.Fill.Pattern > 0 && len(st.Fill.Color) > 0 {
		if c := normalizeHex(st.Fill.Color[0]); c != "" {
			props["bg"] = c
		}
	}
	if a := st.Alignment; a != nil {
		if a.Horizontal == "left" || a.Horizontal == "center" || a.Horizontal == "right" {
			props["align"] = a.Horizontal
		}
		if a.WrapText {
			props["wrap"] = "1"
		}
	}
	sides := 0
	for _, b := range st.Border {
		if b.Style > 0 && (b.Type == "left" || b.Type == "right" || b.Type == "top" || b.Type == "bottom") {
			sides++
		}
	}
	if sides == 4 {
		props["border"] = "all"
	}
	code := ""
	if st.CustomNumFmt != nil {
		code = *st.CustomNumFmt
	} else if st.NumFmt > 0 {
		code = builtinNumFmt[st.NumFmt]
	}
	if nf := codeToNumFmt(code); nf != "" {
		props["numFmt"] = nf
	}
	// Final gate: uploaded files are untrusted and props become inline CSS on
	// every viewer's DOM, so re-check against the same allowlist ops go through.
	for k, v := range props {
		if sheet.ValidateProps(map[string]string{k: v}) != nil {
			delete(props, k)
		}
	}
	return props
}
