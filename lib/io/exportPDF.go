package io

import (
	"bytes"
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
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/models/pad"
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
		revision, err := retrievedPad.GetRevision(*optRevNum)
		if err != nil {
			return nil, err
		}
		atext = apool.AText{
			Text:    revision.AText.Text,
			Attribs: revision.AText.Attribs,
		}
	}

	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()

	if err := e.loadFonts(&pdf); err != nil {
		return nil, err
	}

	if err := e.renderFormattedText(&pdf, retrievedPad, atext); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	_, err = pdf.WriteTo(&buf)
	if err != nil {
		return nil, err
	}

	pdfBytes, err := e.embedEtherpadData(buf.Bytes(), padId)
	if err != nil {
		return buf.Bytes(), nil
	}

	return pdfBytes, nil
}

func (e *ExportPDF) embedEtherpadData(pdfContent []byte, padId string) ([]byte, error) {
	if e.exportEtherpad == nil {
		return pdfContent, nil
	}

	etherpadExport, err := e.exportEtherpad.GetPadRaw(padId, nil)
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

	err = api.AddAttachmentsFile(inputPdfPath, outputPdfPath, []string{jsonPath}, false, conf)
	if err != nil {
		return nil, fmt.Errorf("could not embed attachment: %w", err)
	}

	outputPdf, err := os.ReadFile(outputPdfPath)
	if err != nil {
		return nil, fmt.Errorf("could not read output pdf: %w", err)
	}

	return outputPdf, nil
}

func (e *ExportPDF) loadFonts(pdf *gopdf.GoPdf) error {
	regularPaths := []string{
		"assets/font/Roboto-Regular.ttf",
		"test_assets/assets/font/Roboto-Regular.ttf",
	}

	var regularBytes []byte
	var err error
	for _, path := range regularPaths {
		regularBytes, err = e.uiAssets.ReadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("failed to read regular font file: %w", err)
	}
	err = pdf.AddTTFFontByReader("Roboto", bytes.NewReader(regularBytes))
	if err != nil {
		return fmt.Errorf("failed to load regular font: %w", err)
	}

	boldPaths := []string{
		"assets/font/Roboto-Bold.ttf",
		"test_assets/assets/font/Roboto-Bold.ttf",
	}

	var boldBytes []byte
	for _, path := range boldPaths {
		boldBytes, err = e.uiAssets.ReadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("failed to read bold font file: %w", err)
	}
	err = pdf.AddTTFFontByReader("Roboto-Bold", bytes.NewReader(boldBytes))
	if err != nil {
		return fmt.Errorf("failed to load bold font: %w", err)
	}

	return nil
}

func (e *ExportPDF) renderFormattedText(pdf *gopdf.GoPdf, retrievedPad *pad.Pad, atext apool.AText) error {
	padPool := retrievedPad.Pool
	textLines := padLib.SplitRemoveLastRune(atext.Text)
	attribLines, err := changeset.SplitAttributionLines(atext.Attribs, atext.Text)
	if err != nil {
		return err
	}

	authorColors := e.buildAuthorColorCache(&padPool)

	pdf.SetX(marginLeft)
	pdf.SetY(marginTop)

	listCounters := make(map[int]int)
	lastListType := ""
	lastListLevel := 0

	for i, lineText := range textLines {
		var aline string
		if i < len(attribLines) {
			aline = attribLines[i]
		}

		segments, info, err := e.parseLineSegments(lineText, aline, &padPool, authorColors)
		if err != nil {
			return err
		}

		// Call hook to allow plugins to modify the line
		padId := retrievedPad.Id
		hookContext := &events.LinePDFForExportContext{
			Apool:      &padPool,
			AttribLine: &aline,
			Text:       &lineText,
			PadId:      &padId,
			Alignment:  nil,
			Heading:    nil,
		}
		e.Hooks.ExecuteHooks("getLinePDFForExport", hookContext)

		if hookContext.Alignment != nil {
			info.alignment = *hookContext.Alignment
		}

		if hookContext.Heading != nil {
			info.heading = *hookContext.Heading
		}

		// Handle heading
		if info.heading != "" {
			if err := e.renderHeading(pdf, segments, info); err != nil {
				return err
			}
			continue
		}

		// Handle list prefix with proper numbering
		if info.listType != "" {
			indent := marginLeft + float64(info.level-1)*20.0
			pdf.SetX(indent)

			if err := pdf.SetFont("Roboto", "", fontSize); err != nil {
				return err
			}

			if info.listType != lastListType || info.level != lastListLevel {
				if info.listType == "number" {
					if lastListType != "number" || info.level != lastListLevel {
						listCounters[info.level] = 0
					}
				}
			}

			if info.listType == "bullet" {
				pdf.Cell(nil, "• ")
			} else if info.listType == "number" {
				listCounters[info.level]++
				pdf.Cell(nil, fmt.Sprintf("%d. ", listCounters[info.level]))
			}

			lastListType = info.listType
			lastListLevel = info.level
		} else {
			lastListType = ""
			lastListLevel = 0

			if info.alignment != "" && info.alignment != "left" {
				totalWidth := e.calculateTotalWidth(pdf, segments, fontSize)
				availableWidth := pageWidth - marginLeft - marginRight
				switch info.alignment {
				case "center":
					pdf.SetX(marginLeft + (availableWidth-totalWidth)/2)
				case "right":
					pdf.SetX(marginLeft + availableWidth - totalWidth)
				default:
					pdf.SetX(marginLeft)
				}
			} else {
				pdf.SetX(marginLeft)
			}
		}

		for _, seg := range segments {
			if err := e.renderSegment(pdf, seg, fontSize, lineHeight); err != nil {
				return err
			}
		}

		pdf.Br(lineHeight)
		pdf.SetX(marginLeft)

		if pdf.GetY() > pageHeight-marginBottom {
			pdf.AddPage()
			pdf.SetX(marginLeft)
			pdf.SetY(marginTop)
		}
	}

	return nil
}

func (e *ExportPDF) renderHeading(pdf *gopdf.GoPdf, segments []textSegment, info lineInfo) error {
	style, ok := headingStyles[info.heading]
	if !ok {
		style = headingStyles["h1"]
	}

	// Add top margin
	pdf.SetY(pdf.GetY() + style.marginTop)

	// Check for page break
	if pdf.GetY() > pageHeight-marginBottom-style.lineHeight {
		pdf.AddPage()
		pdf.SetX(marginLeft)
		pdf.SetY(marginTop)
	}

	// Calculate alignment
	if info.alignment != "" && info.alignment != "left" {
		totalWidth := e.calculateTotalWidth(pdf, segments, style.fontSize)
		availableWidth := pageWidth - marginLeft - marginRight
		switch info.alignment {
		case "center":
			pdf.SetX(marginLeft + (availableWidth-totalWidth)/2)
		case "right":
			pdf.SetX(marginLeft + availableWidth - totalWidth)
		default:
			pdf.SetX(marginLeft)
		}
	} else {
		pdf.SetX(marginLeft)
	}

	// Render segments with heading style
	for _, seg := range segments {
		// Force bold for headings
		seg.bold = style.bold || seg.bold
		if err := e.renderSegment(pdf, seg, style.fontSize, style.lineHeight); err != nil {
			return err
		}
	}

	pdf.Br(style.lineHeight + style.marginBottom)
	pdf.SetX(marginLeft)

	return nil
}

func (e *ExportPDF) calculateTotalWidth(pdf *gopdf.GoPdf, segments []textSegment, size float64) float64 {
	totalWidth := 0.0
	for _, seg := range segments {
		fontName := "Roboto"
		if seg.bold {
			fontName = "Roboto-Bold"
		}
		pdf.SetFont(fontName, "", size)
		w, _ := pdf.MeasureTextWidth(seg.text)
		totalWidth += w
	}
	return totalWidth
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
		return segments, info, nil
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

		segments = append(segments, seg)
		pos = endPos
	}

	if pos < len(textRunes) {
		segments = append(segments, textSegment{text: string(textRunes[pos:])})
	}

	return segments, info, nil
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

func (e *ExportPDF) renderSegment(pdf *gopdf.GoPdf, seg textSegment, size float64, height float64) error {
	fontName := "Roboto"
	if seg.bold {
		fontName = "Roboto-Bold"
	}

	if err := pdf.SetFont(fontName, "", size); err != nil {
		return err
	}

	startX := pdf.GetX()
	startY := pdf.GetY()

	textWidth, _ := pdf.MeasureTextWidth(seg.text)
	endX := startX + textWidth

	if seg.authorColor != "" {
		r, g, b, err := parseHexColor(seg.authorColor)
		if err == nil {
			pdf.SetFillColor(r, g, b)
			pdf.Rectangle(startX, startY, endX, startY+height-4, "F", 0, 0)
		}
	}

	pdf.SetTextColor(0, 0, 0)
	pdf.SetStrokeColor(0, 0, 0)

	pdf.Cell(nil, seg.text)

	if seg.underline {
		pdf.Line(startX, startY+size-1, endX, startY+size-1)
	}

	if seg.strikethrough {
		pdf.Line(startX, startY+size/2-1, endX, startY+size/2-1)
	}

	return nil
}
