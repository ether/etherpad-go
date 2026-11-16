package io

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/io/pdffonts"
	"github.com/ether/etherpad-go/lib/models/pad"
	padLib "github.com/ether/etherpad-go/lib/pad"
	"github.com/signintech/gopdf"
)

// The PDF renderer lays each pad line out into visual lines that fit the
// page (word by word; a word longer than a line is broken by characters),
// then draws them by the line's alignment. Lines of a list item after the
// first are indented below its text, not its bullet.

// piece is a part of a segment placed on one visual line.
type piece struct {
	seg   textSegment
	text  string
	width float64
	size  float64
}

// visualLine is what fits on one line of the page.
type visualLine struct {
	pieces []piece
	width  float64 // with the trailing spaces, as drawn
	// visible is width without the spaces at the end of the line: what
	// fitting and alignment go by.
	visible float64
	height  float64 // the tallest piece's line height
}

type pdfRenderer struct {
	pdf   *gopdf.GoPdf
	fonts *pdfFonts
}

// fontFor returns the font of a segment: its family's face for its bold
// and italic, registered on first use.
func (r *pdfRenderer) fontFor(seg textSegment) string {
	name, err := r.fonts.fontFor(seg.family, pdffonts.VariantOf(seg.bold, seg.italic))
	if err != nil {
		r.fonts.warn("PDF export: font for %q: %v", seg.family, err)
		name, _ = r.fonts.builtin(pdffonts.Sans, pdffonts.Regular)
	}
	return name
}

func (r *pdfRenderer) measure(seg textSegment, size float64, text string) float64 {
	_ = r.pdf.SetFont(r.fontFor(seg), "", size)
	w, _ := r.pdf.MeasureTextWidth(text)
	return w
}

// tokens splits text into words with their trailing spaces, so a break
// never lands inside a word and the spaces stay at the end of a line.
func tokens(text string) []string {
	var out []string
	runes := []rune(text)
	start := 0
	for i := 0; i < len(runes); i++ {
		if unicode.IsSpace(runes[i]) && (i+1 == len(runes) || !unicode.IsSpace(runes[i+1])) {
			out = append(out, string(runes[start:i+1]))
			start = i + 1
		}
	}
	if start < len(runes) {
		out = append(out, string(runes[start:]))
	}
	return out
}

// layout fits segments into lines of firstWidth (the first) and width (the
// others). baseSize is the line's font size where a segment has none.
func (r *pdfRenderer) layout(segments []textSegment, baseSize, firstWidth, width float64) []visualLine {
	lines := []visualLine{{}}
	avail := firstWidth
	cur := func() *visualLine { return &lines[len(lines)-1] }
	add := func(seg textSegment, text string, w, size float64) {
		l := cur()
		// Merge with the previous piece of the same segment.
		if n := len(l.pieces); n > 0 && l.pieces[n-1].seg == seg && l.pieces[n-1].size == size {
			l.pieces[n-1].text += text
			l.pieces[n-1].width += w
		} else {
			l.pieces = append(l.pieces, piece{seg: seg, text: text, width: w, size: size})
		}
		if trimmed := strings.TrimRightFunc(text, unicode.IsSpace); trimmed != "" {
			l.visible = l.width + r.measure(seg, size, trimmed)
		}
		l.width += w
		if h := size * 1.5; h > l.height {
			l.height = h
		}
	}
	newLine := func() {
		lines = append(lines, visualLine{})
		avail = width
	}
	for _, seg := range segments {
		size := baseSize
		if seg.size > 0 {
			size = seg.size
		}
		for _, tok := range tokens(seg.text) {
			w := r.measure(seg, size, tok)
			trimmed := strings.TrimRightFunc(tok, unicode.IsSpace)
			fits := cur().width+r.measure(seg, size, trimmed) <= avail
			if fits || (cur().width == 0 && trimmed == "") {
				add(seg, tok, w, size)
				continue
			}
			if cur().width > 0 {
				newLine()
				if r.measure(seg, size, trimmed) <= avail {
					add(seg, tok, w, size)
					continue
				}
			}
			// Longer than a whole line: break it by characters.
			var part []rune
			for _, ch := range []rune(tok) {
				try := string(append(part, ch))
				fit := strings.TrimRightFunc(try, unicode.IsSpace)
				if len(part) > 0 && cur().width+r.measure(seg, size, fit) > avail && !unicode.IsSpace(ch) {
					add(seg, string(part), r.measure(seg, size, string(part)), size)
					newLine()
					part = part[:0]
				}
				part = append(part, ch)
			}
			if len(part) > 0 {
				add(seg, string(part), r.measure(seg, size, string(part)), size)
			}
		}
	}
	for i := range lines {
		if lines[i].height == 0 {
			lines[i].height = baseSize * 1.5
		}
	}
	return lines
}

// draw writes one visual line at x, y; the pieces' backgrounds (author
// colours), decorations and link areas go with them.
func (r *pdfRenderer) draw(line visualLine, x, y float64) error {
	for _, p := range line.pieces {
		if err := r.pdf.SetFont(r.fontFor(p.seg), "", p.size); err != nil {
			return err
		}
		// Bottom-aligned on the line: a smaller piece sits on the same
		// baseline as a bigger one.
		top := y + line.height - p.size*1.5
		if p.seg.authorColor != "" {
			if cr, cg, cb, err := parseHexColor(p.seg.authorColor); err == nil {
				r.pdf.SetFillColor(cr, cg, cb)
				r.pdf.Rectangle(x, top, x+p.width, top+p.size*1.5-4, "F", 0, 0)
			}
		}
		cr, cg, cb := uint8(0), uint8(0), uint8(0)
		if p.seg.color != nil {
			cr, cg, cb = p.seg.color[0], p.seg.color[1], p.seg.color[2]
		}
		if p.seg.link != "" && p.seg.color == nil {
			cr, cg, cb = 29, 78, 216 // like a link in the browser
		}
		r.pdf.SetTextColor(cr, cg, cb)
		r.pdf.SetStrokeColor(cr, cg, cb)
		r.pdf.SetXY(x, top)
		if err := r.pdf.Cell(nil, p.text); err != nil {
			return err
		}
		textWidth := r.measure(p.seg, p.size, strings.TrimRightFunc(p.text, unicode.IsSpace))
		if p.seg.underline || p.seg.link != "" {
			r.pdf.Line(x, top+p.size-1, x+textWidth, top+p.size-1)
		}
		if p.seg.strikethrough {
			r.pdf.Line(x, top+p.size/2-1, x+textWidth, top+p.size/2-1)
		}
		if p.seg.link != "" && textWidth > 0 {
			r.pdf.AddExternalLink(p.seg.link, x, top, textWidth, p.size*1.2)
		}
		x += p.width
	}
	r.pdf.SetTextColor(0, 0, 0)
	r.pdf.SetStrokeColor(0, 0, 0)
	return nil
}

func (r *pdfRenderer) ensureSpace(height float64) {
	if r.pdf.GetY()+height > pageHeight-marginBottom {
		r.pdf.AddPage()
		r.pdf.SetXY(marginLeft, marginTop)
	}
}

// paragraph lays out and draws one pad line. prefix (a list bullet or
// number) is drawn at indent; the text follows it, and further lines start
// below the text.
func (r *pdfRenderer) paragraph(segments []textSegment, size float64, alignment string, indent float64, prefix string, bold bool) error {
	if bold {
		for i := range segments {
			segments[i].bold = true
		}
	}
	textLeft := marginLeft + indent
	if prefix != "" {
		textLeft += r.measure(textSegment{}, size, prefix)
	}
	width := pageWidth - marginRight - textLeft
	lines := r.layout(segments, size, width, width)
	for i, line := range lines {
		r.ensureSpace(line.height)
		y := r.pdf.GetY()
		if i == 0 && prefix != "" {
			if err := r.pdf.SetFont(r.fontFor(textSegment{}), "", size); err != nil {
				return err
			}
			r.pdf.SetXY(marginLeft+indent, y+line.height-size*1.5)
			if err := r.pdf.Cell(nil, prefix); err != nil {
				return err
			}
		}
		x := textLeft
		switch alignment {
		case "center":
			x = textLeft + (width-line.visible)/2
		case "right":
			x = textLeft + width - line.visible
		}
		if err := r.draw(line, x, y); err != nil {
			return err
		}
		r.pdf.SetXY(marginLeft, y+line.height)
	}
	return nil
}

func (e *ExportPDF) renderFormattedText(pdf *gopdf.GoPdf, fonts *pdfFonts, retrievedPad *pad.Pad, atext apool.AText) error {
	r := &pdfRenderer{pdf: pdf, fonts: fonts}
	padPool := retrievedPad.Pool
	textLines := padLib.SplitRemoveLastRune(atext.Text)
	attribLines, err := changeset.SplitAttributionLines(atext.Attribs, atext.Text)
	if err != nil {
		return err
	}
	authorColors := e.buildAuthorColorCache(&padPool)
	pdf.SetXY(marginLeft, marginTop)

	listCounters := map[int]int{}
	lastListType, lastListLevel := "", 0
	for i, lineText := range textLines {
		var aline string
		if i < len(attribLines) {
			aline = attribLines[i]
		}
		segments, info, err := e.parseLineSegments(lineText, aline, &padPool, authorColors)
		if err != nil {
			return err
		}
		padId := retrievedPad.Id
		hookContext := &events.LinePDFForExportContext{Apool: &padPool, AttribLine: &aline, Text: &lineText, PadId: &padId}
		e.Hooks.ExecuteHooks("getLinePDFForExport", hookContext)
		if hookContext.Alignment != nil {
			info.alignment = *hookContext.Alignment
		}
		if hookContext.Heading != nil {
			info.heading = *hookContext.Heading
		}

		if info.heading != "" {
			style, ok := headingStyles[info.heading]
			if !ok {
				style = headingStyles["Heading1"]
			}
			pdf.SetY(pdf.GetY() + style.marginTop)
			if err := r.paragraph(segments, style.fontSize, info.alignment, 0, "", style.bold); err != nil {
				return err
			}
			pdf.SetY(pdf.GetY() + style.marginBottom)
			lastListType, lastListLevel = "", 0
			continue
		}

		prefix, indent := "", 0.0
		if info.listType != "" {
			indent = float64(info.level-1) * 20.0
			if info.listType == "number" && (lastListType != "number" || info.level != lastListLevel) {
				if lastListType != "number" || info.level > lastListLevel {
					listCounters[info.level] = 0
				}
			}
			if info.listType == "bullet" {
				prefix = "• "
			} else {
				listCounters[info.level]++
				prefix = fmt.Sprintf("%d. ", listCounters[info.level])
			}
			lastListType, lastListLevel = info.listType, info.level
		} else {
			lastListType, lastListLevel = "", 0
		}
		if len(segments) == 0 && prefix == "" {
			// An empty line keeps its height; an empty list item still
			// gets its bullet or number (paragraph).
			r.ensureSpace(lineHeight)
			pdf.SetXY(marginLeft, pdf.GetY()+lineHeight)
			continue
		}
		if err := r.paragraph(segments, fontSize, info.alignment, indent, prefix, false); err != nil {
			return err
		}
	}
	return nil
}
