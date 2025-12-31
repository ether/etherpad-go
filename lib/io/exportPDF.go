package io

import (
	"bytes"
	"embed"
	"fmt"
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/models/pad"
	padLib "github.com/ether/etherpad-go/lib/pad"
	"github.com/signintech/gopdf"
)

type ExportPDF struct {
	exportTxt  *ExportTxt
	uiAssets   embed.FS
	padManager *padLib.Manager
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

	return buf.Bytes(), nil
}

func (e *ExportPDF) loadFonts(pdf *gopdf.GoPdf) error {
	regularBytes, err := e.uiAssets.ReadFile("assets/font/Roboto-Regular.ttf")
	if err != nil {
		return fmt.Errorf("failed to read regular font file: %w", err)
	}
	err = pdf.AddTTFFontByReader("Roboto", bytes.NewReader(regularBytes))
	if err != nil {
		return fmt.Errorf("failed to load regular font: %w", err)
	}

	boldBytes, err := e.uiAssets.ReadFile("assets/font/Roboto-Bold.ttf")
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

	pdf.SetX(marginLeft)
	pdf.SetY(marginTop)

	for i, lineText := range textLines {
		var aline string
		if i < len(attribLines) {
			aline = attribLines[i]
		}

		segments, listPrefix, err := e.parseLineSegments(lineText, aline, &padPool)
		if err != nil {
			return err
		}

		if listPrefix != "" {
			if err := pdf.SetFont("Roboto", "", fontSize); err != nil {
				return err
			}
			pdf.Cell(nil, listPrefix)
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

func (e *ExportPDF) parseLineSegments(text string, aline string, padPool *apool.APool) ([]textSegment, string, error) {
	var segments []textSegment
	listPrefix := ""

	if text == "" {
		return segments, listPrefix, nil
	}

	if aline != "" {
		ops, err := changeset.DeserializeOps(aline)
		if err != nil {
			return nil, "", err
		}
		if len(*ops) > 0 {
			op := (*ops)[0]
			attribs := changeset.FromString(op.Attribs, padPool)
			listTypeStr := attribs.Get("list")
			if listTypeStr != nil {
				if strings.HasPrefix(*listTypeStr, "bullet") {
					listPrefix = "• "
				} else if strings.HasPrefix(*listTypeStr, "number") {
					listPrefix = "  "
				}
				if len(text) > 0 {
					text = text[1:]
				}
				newAline, err := changeset.Subattribution(aline, 1, nil)
				if err != nil {
					return nil, "", err
				}
				aline = *newAline
			}
		}
	}

	if aline == "" || text == "" {
		if text != "" {
			segments = append(segments, textSegment{text: text})
		}
		return segments, listPrefix, nil
	}

	ops, err := changeset.DeserializeOps(aline)
	if err != nil {
		return nil, "", err
	}

	textRunes := []rune(text)
	pos := 0

	for _, op := range *ops {
		if pos >= len(textRunes) {
			break
		}

		chars := op.Chars
		if op.Lines > 0 {
			chars-- // Don't include linebreak
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

		segments = append(segments, seg)
		pos = endPos
	}

	if pos < len(textRunes) {
		segments = append(segments, textSegment{text: string(textRunes[pos:])})
	}

	return segments, listPrefix, nil
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

	pdf.Cell(nil, seg.text)

	textWidth, _ := pdf.MeasureTextWidth(seg.text)
	endX := startX + textWidth

	if seg.underline {
		pdf.Line(startX, startY+fontSize-1, endX, startY+fontSize-1)
	}

	if seg.strikethrough {
		pdf.Line(startX, startY+fontSize/2-1, endX, startY+fontSize/2-1)
	}

	return nil
}
