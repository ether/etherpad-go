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

type textSegment struct {
	text          string
	bold          bool
	italic        bool
	underline     bool
	strikethrough bool
	authorColor   string
}

type listInfo struct {
	listType string // "bullet", "number", or ""
	level    int    // 1-based level for nested lists
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

	// Embed Etherpad JSON data as attachment for lossless import
	pdfBytes, err := e.embedEtherpadData(buf.Bytes(), padId)
	if err != nil {
		// If embedding fails, return the PDF without the attachment
		return buf.Bytes(), nil
	}

	return pdfBytes, nil
}

// embedEtherpadData embeds the Etherpad JSON export as a PDF attachment
// This allows for lossless import similar to ZUGFeRD format (but with JSON instead of XML)
func (e *ExportPDF) embedEtherpadData(pdfContent []byte, padId string) ([]byte, error) {
	if e.exportEtherpad == nil {
		return pdfContent, nil
	}

	// Get the Etherpad export data
	etherpadExport, err := e.exportEtherpad.GetPadRaw(padId, nil)
	if err != nil {
		return nil, fmt.Errorf("could not get etherpad export: %w", err)
	}

	// Convert to JSON
	jsonData, err := json.Marshal(etherpadExport)
	if err != nil {
		return nil, fmt.Errorf("could not marshal etherpad data: %w", err)
	}

	// Create temporary files for pdfcpu (it requires file paths)
	tempDir, err := os.MkdirTemp("", "etherpad-pdf-*")
	if err != nil {
		return nil, fmt.Errorf("could not create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Write JSON to temp file
	jsonPath := tempDir + "/etherpad.json"
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return nil, fmt.Errorf("could not write json temp file: %w", err)
	}

	// Write input PDF to temp file
	inputPdfPath := tempDir + "/input.pdf"
	if err := os.WriteFile(inputPdfPath, pdfContent, 0644); err != nil {
		return nil, fmt.Errorf("could not write pdf temp file: %w", err)
	}

	// Output PDF path
	outputPdfPath := tempDir + "/output.pdf"

	// Configure pdfcpu
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed

	// Add the JSON as an embedded file attachment
	err = api.AddAttachmentsFile(inputPdfPath, outputPdfPath, []string{jsonPath}, false, conf)
	if err != nil {
		return nil, fmt.Errorf("could not embed attachment: %w", err)
	}

	// Read the output PDF
	outputPdf, err := os.ReadFile(outputPdfPath)
	if err != nil {
		return nil, fmt.Errorf("could not read output pdf: %w", err)
	}

	return outputPdf, nil
}

func (e *ExportPDF) loadFonts(pdf *gopdf.GoPdf) error {
	// Try multiple paths to support both production and test environments
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

	// Build author color cache
	authorColors := e.buildAuthorColorCache(&padPool)

	pdf.SetX(marginLeft)
	pdf.SetY(marginTop)

	// Track numbered list counters for different levels
	listCounters := make(map[int]int)
	lastListType := ""
	lastListLevel := 0

	for i, lineText := range textLines {
		var aline string
		if i < len(attribLines) {
			aline = attribLines[i]
		}

		segments, listInfo, err := e.parseLineSegments(lineText, aline, &padPool, authorColors)
		if err != nil {
			return err
		}

		// Handle list prefix with proper numbering
		if listInfo.listType != "" {
			// Calculate indentation based on list level
			indent := marginLeft + float64(listInfo.level-1)*20.0
			pdf.SetX(indent)

			if err := pdf.SetFont("Roboto", "", fontSize); err != nil {
				return err
			}

			// Reset counter if list type or level changed
			if listInfo.listType != lastListType || listInfo.level != lastListLevel {
				if listInfo.listType == "number" {
					// Reset counter for new numbered lists at this level
					if lastListType != "number" || listInfo.level != lastListLevel {
						listCounters[listInfo.level] = 0
					}
				}
			}

			if listInfo.listType == "bullet" {
				pdf.Cell(nil, "• ")
			} else if listInfo.listType == "number" {
				listCounters[listInfo.level]++
				pdf.Cell(nil, fmt.Sprintf("%d. ", listCounters[listInfo.level]))
			}

			lastListType = listInfo.listType
			lastListLevel = listInfo.level
		} else {
			// Reset list tracking when not in a list
			lastListType = ""
			lastListLevel = 0
			pdf.SetX(marginLeft)
		}

		for _, seg := range segments {
			if err := e.renderSegment(pdf, seg); err != nil {
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

func (e *ExportPDF) buildAuthorColorCache(padPool *apool.APool) map[string]string {
	authorColors := make(map[string]string)

	for _, attr := range padPool.NumToAttrib {
		if attr.Key == "author" && attr.Value != "" {
			authorId := attr.Value
			if _, exists := authorColors[authorId]; !exists {
				// Try to get author color from database
				if authorData, err := e.authorManager.GetAuthor(authorId); err == nil {
					authorColors[authorId] = authorData.ColorId
				}
			}
		}
	}

	return authorColors
}

func (e *ExportPDF) parseLineSegments(text string, aline string, padPool *apool.APool, authorColors map[string]string) ([]textSegment, listInfo, error) {
	var segments []textSegment
	info := listInfo{}

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
			listTypeStr := attribs.Get("list")
			if listTypeStr != nil {
				// Parse list type and level (e.g., "bullet1", "number2")
				listVal := *listTypeStr
				if strings.HasPrefix(listVal, "bullet") {
					info.listType = "bullet"
					// Extract level from suffix (e.g., "bullet2" -> level 2)
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

				// Remove the leading * marker from list items
				if len(text) > 0 && text[0] == '*' {
					text = text[1:]
				}
				newAline, err := changeset.Subattribution(aline, 1, nil)
				if err != nil {
					return nil, info, err
				}
				aline = *newAline
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

		// Parse attributes
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

// parseHexColor converts a hex color string to RGB values
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

func (e *ExportPDF) renderSegment(pdf *gopdf.GoPdf, seg textSegment) error {
	fontName := "Roboto"
	if seg.bold {
		fontName = "Roboto-Bold"
	}

	if err := pdf.SetFont(fontName, "", fontSize); err != nil {
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
			pdf.Rectangle(startX, startY, endX, startY+lineHeight-4, "F", 0, 0)
		}
	}

	pdf.SetTextColor(0, 0, 0)
	pdf.SetStrokeColor(0, 0, 0)

	pdf.Cell(nil, seg.text)

	if seg.underline {
		pdf.Line(startX, startY+fontSize-1, endX, startY+fontSize-1)
	}

	if seg.strikethrough {
		pdf.Line(startX, startY+fontSize/2-1, endX, startY+fontSize/2-1)
	}

	return nil
}
