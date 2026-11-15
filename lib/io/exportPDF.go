package io

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/hooks"
	padLib "github.com/ether/etherpad-go/lib/pad"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/signintech/gopdf"
)

type ExportPDF struct {
	exportTxt      *ExportTxt
	exportEtherpad *ExportEtherpad
	uiAssets       embed.FS
	padManager     *padLib.Manager
	authorManager  *author.Manager
	Hooks          *hooks.Hook
}

const (
	pageWidth    = 595.28 // A4 width in points
	pageHeight   = 841.89 // A4 height in points
	marginLeft   = 40.0
	marginRight  = 40.0
	marginTop    = 40.0
	marginBottom = 40.0
	fontSize     = 12.0
	lineHeight   = 18.0
)

// Heading-Konfiguration
type headingStyle struct {
	fontSize     float64
	lineHeight   float64
	bold         bool
	marginTop    float64
	marginBottom float64
}

var headingStyles = map[string]headingStyle{
	"Heading1": {fontSize: 28, lineHeight: 36, bold: true, marginTop: 24, marginBottom: 12},
	"Heading2": {fontSize: 24, lineHeight: 32, bold: true, marginTop: 20, marginBottom: 10},
	"Heading3": {fontSize: 20, lineHeight: 28, bold: true, marginTop: 16, marginBottom: 8},
	"Heading4": {fontSize: 16, lineHeight: 24, bold: true, marginTop: 12, marginBottom: 6},
	"Heading5": {fontSize: 14, lineHeight: 20, bold: true, marginTop: 10, marginBottom: 4},
	"Heading6": {fontSize: 12, lineHeight: 18, bold: true, marginTop: 8, marginBottom: 4},
}

type textSegment struct {
	text          string
	bold          bool
	italic        bool
	underline     bool
	strikethrough bool
	authorColor   string
	// link is the target of a URL in the text (as the HTML export links it).
	link string
	// mono: ep_font_family's courier or monospace, set in RobotoMono. The
	// other families are proprietary fonts the PDF cannot embed.
	mono bool
	// size is ep_font_size's point size; 0 means the line's size.
	size float64
	// color is ep_font_color's text colour; nil means black.
	color *[3]uint8
}

type lineInfo struct {
	listType  string // "bullet", "number", or ""
	level     int    // 1-based level for nested lists
	alignment string // "left", "center", "right", "justify"
	heading   string // "h1", "h2", "h3", "h4", "h5", "h6" or ""
}

func (e *ExportPDF) GetPadPdfDocument(padId string, optRevNum *int) ([]byte, error) {
	retrievedPad, err := e.padManager.GetPad(padId, nil, nil)
	if err != nil {
		return nil, err
	}

	atext := retrievedPad.AText
	if optRevNum != nil {
		revAText := retrievedPad.GetInternalRevisionAText(*optRevNum)
		if revAText == nil {
			return nil, fmt.Errorf("revision %d of pad %s not found", *optRevNum, retrievedPad.Id)
		}
		atext = *revAText
	}

	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()

	mono, err := e.loadFonts(&pdf)
	if err != nil {
		return nil, err
	}

	if err := e.renderFormattedText(&pdf, mono, retrievedPad, atext); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	_, err = pdf.WriteTo(&buf)
	if err != nil {
		return nil, err
	}

	pdfBytes, err := e.embedEtherpadData(buf.Bytes(), padId, optRevNum)
	if err != nil {
		return buf.Bytes(), nil
	}

	return pdfBytes, nil
}

// embedEtherpadData attaches the .etherpad export to the PDF. For a
// revision-bound PDF the attachment is bound to the same revision, so it
// can't leak later edits or chat.
func (e *ExportPDF) embedEtherpadData(pdfContent []byte, padId string, optRevNum *int) ([]byte, error) {
	if e.exportEtherpad == nil {
		return pdfContent, nil
	}

	etherpadExport, err := e.exportEtherpad.GetPadRaw(padId, nil, optRevNum)
	if err != nil {
		return nil, fmt.Errorf("could not get etherpad export: %w", err)
	}

	jsonData, err := json.Marshal(etherpadExport)
	if err != nil {
		return nil, fmt.Errorf("could not marshal etherpad data: %w", err)
	}

	tempDir, err := os.MkdirTemp("", "etherpad-pdf-*")
	if err != nil {
		return nil, fmt.Errorf("could not create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	jsonPath := tempDir + "/etherpad.json"
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("could not write json temp file: %w", err)
	}

	inputPdfPath := tempDir + "/input.pdf"
	if err := os.WriteFile(inputPdfPath, pdfContent, 0644); err != nil {
		return nil, fmt.Errorf("could not write pdf temp file: %w", err)
	}

	outputPdfPath := tempDir + "/output.pdf"

	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed

	ctx := context.Background()
	err = api.AddAttachmentsFile(ctx, inputPdfPath, outputPdfPath, []string{jsonPath}, false, conf)
	if err != nil {
		return nil, fmt.Errorf("could not embed attachment: %w", err)
	}

	outputPdf, err := os.ReadFile(outputPdfPath)
	if err != nil {
		return nil, fmt.Errorf("could not read output pdf: %w", err)
	}

	return outputPdf, nil
}

// loadFonts adds the fonts; mono reports whether RobotoMono is there.
func (e *ExportPDF) loadFonts(pdf *gopdf.GoPdf) (mono bool, err error) {
	regularPaths := []string{
		"assets/font/Roboto-Regular.ttf",
	}

	var regularBytes []byte
	for _, path := range regularPaths {
		regularBytes, err = e.uiAssets.ReadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		return false, fmt.Errorf("failed to read regular font file: %w", err)
	}
	err = pdf.AddTTFFontByReader("Roboto", bytes.NewReader(regularBytes))
	if err != nil {
		return false, fmt.Errorf("failed to load regular font: %w", err)
	}

	boldPaths := []string{
		"assets/font/Roboto-Bold.ttf",
	}

	var boldBytes []byte
	for _, path := range boldPaths {
		boldBytes, err = e.uiAssets.ReadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		return false, fmt.Errorf("failed to read bold font file: %w", err)
	}
	err = pdf.AddTTFFontByReader("Roboto-Bold", bytes.NewReader(boldBytes))
	if err != nil {
		return false, fmt.Errorf("failed to load bold font: %w", err)
	}
	loaded := 0
	for name, path := range map[string]string{"RobotoMono": "assets/font/RobotoMono-Regular.ttf", "RobotoMono-Bold": "assets/font/RobotoMono-Bold.ttf"} {
		b, err := e.uiAssets.ReadFile(path)
		if err != nil {
			// Not in every asset set (tests): monospace text falls back to
			// Roboto.
			continue
		}
		if err := pdf.AddTTFFontByReader(name, bytes.NewReader(b)); err != nil {
			return false, fmt.Errorf("failed to load %s: %w", name, err)
		}
		loaded++
	}

	return loaded == 2, nil
}

func (e *ExportPDF) buildAuthorColorCache(padPool *apool.APool) map[string]string {
	authorColors := make(map[string]string)

	for _, attr := range padPool.NumToAttrib {
		if attr.Key == "author" && attr.Value != "" {
			authorId := attr.Value
			if _, exists := authorColors[authorId]; !exists {
				if authorData, err := e.authorManager.GetAuthor(authorId); err == nil {
					authorColors[authorId] = authorData.ColorId
				}
			}
		}
	}

	return authorColors
}

func (e *ExportPDF) parseLineSegments(text string, aline string, padPool *apool.APool, authorColors map[string]string) ([]textSegment, lineInfo, error) {
	var segments []textSegment
	info := lineInfo{}

	if text == "" {
		return segments, info, nil
	}

	if aline != "" {
		ops, err := changeset.DeserializeOps(aline)
		if err != nil {
			return nil, info, err
		}
		if len(*ops) > 0 {
			op := (*ops)[0]
			attribs := changeset.FromString(op.Attribs, padPool)

			alignStr := attribs.Get("align")
			if alignStr != nil {
				info.alignment = *alignStr
				if len(text) > 0 && text[0] == '*' {
					text = text[1:]
					newAline, err := changeset.Subattribution(aline, 1, nil)
					if err != nil {
						return nil, info, err
					}
					aline = *newAline
				}
			}

			listTypeStr := attribs.Get("list")
			if listTypeStr != nil {
				listVal := *listTypeStr
				if strings.HasPrefix(listVal, "bullet") {
					info.listType = "bullet"
					levelStr := strings.TrimPrefix(listVal, "bullet")
					if level, err := strconv.Atoi(levelStr); err == nil && level > 0 {
						info.level = level
					} else {
						info.level = 1
					}
				} else if strings.HasPrefix(listVal, "number") {
					info.listType = "number"
					levelStr := strings.TrimPrefix(listVal, "number")
					if level, err := strconv.Atoi(levelStr); err == nil && level > 0 {
						info.level = level
					} else {
						info.level = 1
					}
				}

				if len(text) > 0 && text[0] == '*' {
					text = text[1:]
					newAline, err := changeset.Subattribution(aline, 1, nil)
					if err != nil {
						return nil, info, err
					}
					aline = *newAline
				}
			}
		}
	}

	if aline == "" || text == "" {
		if text != "" {
			segments = append(segments, textSegment{text: text})
		}
		return splitLinks(segments), info, nil
	}

	ops, err := changeset.DeserializeOps(aline)
	if err != nil {
		return nil, info, err
	}

	textRunes := []rune(text)
	pos := 0

	for _, op := range *ops {
		if pos >= len(textRunes) {
			break
		}

		chars := op.Chars
		if op.Lines > 0 {
			chars--
		}
		if chars <= 0 {
			continue
		}

		endPos := pos + chars
		if endPos > len(textRunes) {
			endPos = len(textRunes)
		}

		segText := string(textRunes[pos:endPos])
		seg := textSegment{text: segText}

		attribs := changeset.FromString(op.Attribs, padPool)
		if attribs.Get("bold") != nil {
			seg.bold = true
		}
		if attribs.Get("italic") != nil {
			seg.italic = true
		}
		if attribs.Get("underline") != nil {
			seg.underline = true
		}
		if attribs.Get("strikethrough") != nil {
			seg.strikethrough = true
		}

		if authorId := attribs.Get("author"); authorId != nil && *authorId != "" {
			if color, exists := authorColors[*authorId]; exists {
				seg.authorColor = color
			}
		}
		if family := attribs.Get("font-family"); family != nil {
			switch strings.ToLower(*family) {
			case "courier", "monospace":
				seg.mono = true
			}
		}
		if size := attribs.Get("font-size"); size != nil {
			if px, err := strconv.ParseFloat(strings.TrimSuffix(*size, "px"), 64); err == nil && px >= 4 && px <= 96 {
				seg.size = px * 0.75 // CSS px to pt
			}
		}
		if c := attribs.Get("color"); c != nil {
			if rgb, ok := fontColors[strings.ToLower(*c)]; ok {
				seg.color = &rgb
			}
		}

		segments = append(segments, seg)
		pos = endPos
	}

	if pos < len(textRunes) {
		segments = append(segments, textSegment{text: string(textRunes[pos:])})
	}

	return splitLinks(segments), info, nil
}

// fontColors are ep_font_color's colours: the CSS named colours its
// stylesheet gives them (plugins/ep_font_color/static/css/color.css).
var fontColors = map[string][3]uint8{
	"black":  {0, 0, 0},
	"red":    {255, 0, 0},
	"green":  {0, 128, 0},
	"blue":   {0, 0, 255},
	"yellow": {255, 255, 0},
	"orange": {255, 165, 0},
}

// splitLinks cuts the segments at the URLs in the line's text (found like
// the HTML export finds them) and marks the URL parts as links.
func splitLinks(segments []textSegment) []textSegment {
	var full strings.Builder
	for _, seg := range segments {
		full.WriteString(seg.text)
	}
	urls := findURLs(full.String())
	if len(urls) == 0 {
		return segments
	}
	type span struct {
		start, end int
		href       string
	}
	spans := make([]span, 0, len(urls))
	for _, u := range urls {
		href := u.url
		if strings.HasPrefix(strings.ToLower(href), "www.") {
			href = "http://" + href
		}
		spans = append(spans, span{u.start, u.start + len([]rune(u.url)), href})
	}
	var out []textSegment
	pos := 0
	for _, seg := range segments {
		runes := []rune(seg.text)
		segStart, segEnd := pos, pos+len(runes)
		cut := segStart
		for _, sp := range spans {
			if sp.end <= cut || sp.start >= segEnd {
				continue
			}
			from, to := max(sp.start, cut), min(sp.end, segEnd)
			if from > cut {
				part := seg
				part.text = string(runes[cut-segStart : from-segStart])
				out = append(out, part)
			}
			part := seg
			part.text = string(runes[from-segStart : to-segStart])
			part.link = sp.href
			out = append(out, part)
			cut = to
		}
		if cut < segEnd {
			part := seg
			part.text = string(runes[cut-segStart:])
			out = append(out, part)
		}
		pos = segEnd
	}
	return out
}

func parseHexColor(hex string) (r, g, b uint8, err error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex color: %s", hex)
	}

	rVal, err := strconv.ParseUint(hex[0:2], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}
	gVal, err := strconv.ParseUint(hex[2:4], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}
	bVal, err := strconv.ParseUint(hex[4:6], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}

	return uint8(rVal), uint8(gVal), uint8(bVal), nil
}
