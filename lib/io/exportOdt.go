package io

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	padLib "github.com/ether/etherpad-go/lib/pad"
)

type ExportOdt struct {
	padManager    *padLib.Manager
	authorManager *author.Manager
}

func NewExportOdt(padManager *padLib.Manager, authorManager *author.Manager) *ExportOdt {
	return &ExportOdt{
		padManager:    padManager,
		authorManager: authorManager,
	}
}

type odtTextSegment struct {
	text          string
	bold          bool
	italic        bool
	underline     bool
	strikethrough bool
	authorColor   string
}

type odtParagraph struct {
	segments  []odtTextSegment
	listType  string // "bullet", "number", or ""
	listLevel int    // 1-based level
}

func (e *ExportOdt) GetPadOdtDocument(padId string, optRevNum *int) ([]byte, error) {
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

	// Build author color cache
	authorColors := e.buildAuthorColorCache(&retrievedPad.Pool)

	// Parse all lines
	textLines := padLib.SplitRemoveLastRune(atext.Text)
	attribLines, err := changeset.SplitAttributionLines(atext.Attribs, atext.Text)
	if err != nil {
		return nil, err
	}

	var paragraphs []odtParagraph

	for i, lineText := range textLines {
		var aline string
		if i < len(attribLines) {
			aline = attribLines[i]
		}

		para, err := e.parseLineSegments(lineText, aline, &retrievedPad.Pool, authorColors)
		if err != nil {
			return nil, err
		}

		paragraphs = append(paragraphs, para)
	}

	// Generate ODT
	return e.generateOdt(paragraphs)
}

func (e *ExportOdt) generateOdt(paragraphs []odtParagraph) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Collect unique author colors for automatic styles
	authorColorSet := make(map[string]bool)
	for _, para := range paragraphs {
		for _, seg := range para.segments {
			if seg.authorColor != "" {
				authorColorSet[seg.authorColor] = true
			}
		}
	}

	// Add required ODT files
	files := map[string]string{
		"mimetype":              odtMimetype,
		"META-INF/manifest.xml": odtManifest,
		"styles.xml":            odtStylesXML,
		"content.xml":           e.generateContentXML(paragraphs, authorColorSet),
	}

	// mimetype must be first and uncompressed
	mimetypeWriter, err := zipWriter.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store, // No compression
	})
	if err != nil {
		return nil, err
	}
	_, err = mimetypeWriter.Write([]byte(odtMimetype))
	if err != nil {
		return nil, err
	}

	// Add other files
	for name, content := range files {
		if name == "mimetype" {
			continue // Already added
		}
		writer, err := zipWriter.Create(name)
		if err != nil {
			return nil, err
		}
		_, err = writer.Write([]byte(content))
		if err != nil {
			return nil, err
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (e *ExportOdt) generateContentXML(paragraphs []odtParagraph, authorColors map[string]bool) string {
	var automaticStyles strings.Builder
	var bodyContent strings.Builder

	// Generate automatic styles for author colors
	colorIndex := 0
	colorStyleMap := make(map[string]string)
	for color := range authorColors {
		styleName := fmt.Sprintf("AuthorColor%d", colorIndex)
		colorStyleMap[color] = styleName
		hexColor := strings.TrimPrefix(color, "#")
		automaticStyles.WriteString(fmt.Sprintf(
			`<style:style style:name="%s" style:family="text"><style:text-properties fo:background-color="#%s"/></style:style>`,
			styleName, hexColor))
		colorIndex++
	}

	// Generate styles for formatting combinations
	automaticStyles.WriteString(`<style:style style:name="Bold" style:family="text"><style:text-properties fo:font-weight="bold"/></style:style>`)
	automaticStyles.WriteString(`<style:style style:name="Italic" style:family="text"><style:text-properties fo:font-style="italic"/></style:style>`)
	automaticStyles.WriteString(`<style:style style:name="Underline" style:family="text"><style:text-properties style:text-underline-style="solid" style:text-underline-width="auto"/></style:style>`)
	automaticStyles.WriteString(`<style:style style:name="Strikethrough" style:family="text"><style:text-properties style:text-line-through-style="solid"/></style:style>`)

	// Track list state
	inBulletList := false
	inNumberList := false

	for _, para := range paragraphs {
		// Handle list transitions
		if para.listType == "bullet" {
			if inNumberList {
				bodyContent.WriteString("</text:list>")
				inNumberList = false
			}
			if !inBulletList {
				bodyContent.WriteString(`<text:list text:style-name="BulletList">`)
				inBulletList = true
			}
			bodyContent.WriteString("<text:list-item><text:p>")
		} else if para.listType == "number" {
			if inBulletList {
				bodyContent.WriteString("</text:list>")
				inBulletList = false
			}
			if !inNumberList {
				bodyContent.WriteString(`<text:list text:style-name="NumberList">`)
				inNumberList = true
			}
			bodyContent.WriteString("<text:list-item><text:p>")
		} else {
			// Close any open lists
			if inBulletList {
				bodyContent.WriteString("</text:list>")
				inBulletList = false
			}
			if inNumberList {
				bodyContent.WriteString("</text:list>")
				inNumberList = false
			}
			bodyContent.WriteString("<text:p>")
		}

		for _, seg := range para.segments {
			// Determine style name based on formatting
			var styles []string
			if seg.bold {
				styles = append(styles, "Bold")
			}
			if seg.italic {
				styles = append(styles, "Italic")
			}
			if seg.underline {
				styles = append(styles, "Underline")
			}
			if seg.strikethrough {
				styles = append(styles, "Strikethrough")
			}

			// Build text:span with appropriate styling
			if len(styles) > 0 || seg.authorColor != "" {
				bodyContent.WriteString("<text:span")

				// For simplicity, we'll inline the styles
				bodyContent.WriteString(" text:style-name=\"")
				if seg.authorColor != "" {
					bodyContent.WriteString(colorStyleMap[seg.authorColor])
				} else if len(styles) > 0 {
					bodyContent.WriteString(styles[0])
				}
				bodyContent.WriteString("\">")

				// If we have author color and formatting, nest spans
				if seg.authorColor != "" && len(styles) > 0 {
					for _, style := range styles {
						bodyContent.WriteString(fmt.Sprintf(`<text:span text:style-name="%s">`, style))
					}
				}

				bodyContent.WriteString(escapeXMLOdt(seg.text))

				if seg.authorColor != "" && len(styles) > 0 {
					for range styles {
						bodyContent.WriteString("</text:span>")
					}
				}
				bodyContent.WriteString("</text:span>")
			} else {
				bodyContent.WriteString(escapeXMLOdt(seg.text))
			}
		}

		if para.listType != "" {
			bodyContent.WriteString("</text:p></text:list-item>")
		} else {
			bodyContent.WriteString("</text:p>")
		}
	}

	// Close any remaining open lists
	if inBulletList {
		bodyContent.WriteString("</text:list>")
	}
	if inNumberList {
		bodyContent.WriteString("</text:list>")
	}

	return fmt.Sprintf(odtContentXMLTemplate, automaticStyles.String(), bodyContent.String())
}

func escapeXMLOdt(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func (e *ExportOdt) buildAuthorColorCache(padPool *apool.APool) map[string]string {
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

func (e *ExportOdt) parseLineSegments(text string, aline string, padPool *apool.APool, authorColors map[string]string) (odtParagraph, error) {
	para := odtParagraph{}

	if text == "" {
		return para, nil
	}

	// Check for list markers
	if aline != "" {
		ops, err := changeset.DeserializeOps(aline)
		if err != nil {
			return para, err
		}
		if len(*ops) > 0 {
			op := (*ops)[0]
			attribs := changeset.FromString(op.Attribs, padPool)
			listTypeStr := attribs.Get("list")
			if listTypeStr != nil {
				para.listType, para.listLevel = parseListTypeOdt(*listTypeStr)

				if len(text) > 0 {
					text = text[1:]
				}
				newAline, err := changeset.Subattribution(aline, 1, nil)
				if err != nil {
					return para, err
				}
				aline = *newAline
			}
		}
	}

	if aline == "" || text == "" {
		if text != "" {
			para.segments = append(para.segments, odtTextSegment{text: text})
		}
		return para, nil
	}

	ops, err := changeset.DeserializeOps(aline)
	if err != nil {
		return para, err
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
		seg := odtTextSegment{text: segText}

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

		// Get author color
		if authorId := attribs.Get("author"); authorId != nil && *authorId != "" {
			if clr, exists := authorColors[*authorId]; exists {
				seg.authorColor = clr
			}
		}

		para.segments = append(para.segments, seg)
		pos = endPos
	}

	if pos < len(textRunes) {
		para.segments = append(para.segments, odtTextSegment{text: string(textRunes[pos:])})
	}

	return para, nil
}

func parseListTypeOdt(listAttr string) (string, int) {
	if strings.HasPrefix(listAttr, "bullet") {
		level := 1
		if len(listAttr) > 6 {
			if l, err := fmt.Sscanf(listAttr[6:], "%d", &level); err != nil || l != 1 {
				level = 1
			}
		}
		return "bullet", level
	} else if strings.HasPrefix(listAttr, "number") {
		level := 1
		if len(listAttr) > 6 {
			if l, err := fmt.Sscanf(listAttr[6:], "%d", &level); err != nil || l != 1 {
				level = 1
			}
		}
		return "number", level
	}
	return "", 0
}

// ODT file constants
const odtMimetype = "application/vnd.oasis.opendocument.text"

const odtManifest = `<?xml version="1.0" encoding="UTF-8"?>
<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.2">
  <manifest:file-entry manifest:full-path="/" manifest:media-type="application/vnd.oasis.opendocument.text"/>
  <manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/>
  <manifest:file-entry manifest:full-path="styles.xml" manifest:media-type="text/xml"/>
</manifest:manifest>`

const odtStylesXML = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-styles xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
  xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0"
  xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0"
  xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0">
  <office:styles>
    <style:style style:name="Standard" style:family="paragraph">
      <style:paragraph-properties fo:margin-top="0cm" fo:margin-bottom="0.212cm"/>
      <style:text-properties fo:font-size="12pt" style:font-name="Arial"/>
    </style:style>
    <text:list-style style:name="BulletList">
      <text:list-level-style-bullet text:level="1" text:bullet-char="•">
        <style:list-level-properties text:list-level-position-and-space-mode="label-alignment">
          <style:list-level-label-alignment text:label-followed-by="listtab" fo:text-indent="-0.635cm" fo:margin-left="1.27cm"/>
        </style:list-level-properties>
      </text:list-level-style-bullet>
    </text:list-style>
    <text:list-style style:name="NumberList">
      <text:list-level-style-number text:level="1" style:num-suffix="." style:num-format="1">
        <style:list-level-properties text:list-level-position-and-space-mode="label-alignment">
          <style:list-level-label-alignment text:label-followed-by="listtab" fo:text-indent="-0.635cm" fo:margin-left="1.27cm"/>
        </style:list-level-properties>
      </text:list-level-style-number>
    </text:list-style>
  </office:styles>
</office:document-styles>`

const odtContentXMLTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
  xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0"
  xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0"
  xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0"
  office:version="1.2">
  <office:automatic-styles>
    %s
  </office:automatic-styles>
  <office:body>
    <office:text>
      %s
    </office:text>
  </office:body>
</office:document-content>`
