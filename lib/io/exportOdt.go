package io

import (
	"archive/zip"
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	padLib "github.com/ether/etherpad-go/lib/pad"
)

type ExportOdt struct {
	padManager    *padLib.Manager
	authorManager *author.Manager
	Hooks         *hooks.Hook
}

func NewExportOdt(padManager *padLib.Manager, authorManager *author.Manager, hooksSystem *hooks.Hook) *ExportOdt {
	return &ExportOdt{
		padManager:    padManager,
		authorManager: authorManager,
		Hooks:         hooksSystem,
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
	alignment string // "left", "center", "right", "justify"
	heading   string // "h1", "h2", "h3", "h4", "h5", "h6" or ""
}

// 2. Mapping für ODT Heading-Styles
var headingToOdtStyle = map[string]struct {
	styleName    string
	outlineLevel int
}{
	"h1": {"Title", 0},        // Titel (kein Outline-Level)
	"h2": {"Heading_20_1", 1}, // Überschrift 1
	"h3": {"Heading_20_2", 2}, // Überschrift 2
	"h4": {"Heading_20_3", 3}, // Überschrift 3
	"h5": {"Heading_20_4", 4}, // Überschrift 4
	"h6": {"Heading_20_5", 5}, // Überschrift 5
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

	authorColors := e.buildAuthorColorCache(&retrievedPad.Pool)
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

		hookContext := &events.LineOdtForExportContext{
			Apool:      &retrievedPad.Pool,
			AttribLine: &aline,
			Text:       &lineText,
			PadId:      &padId,
			Alignment:  nil,
			Heading:    nil, // Neues Feld
		}
		e.Hooks.ExecuteHooks("getLineOdtForExport", hookContext)

		if hookContext.Alignment != nil {
			para.alignment = *hookContext.Alignment
		}

		if hookContext.Heading != nil {
			para.heading = *hookContext.Heading
		}

		paragraphs = append(paragraphs, para)
	}

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
	files := map[string]string{
		"META-INF/manifest.xml": odtManifest,
		"styles.xml":            odtStylesXML,
		"content.xml":           e.generateContentXML(paragraphs, authorColorSet),
	}

	for name, content := range files {
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
		styleName := fmt.Sprintf("T%d", colorIndex)
		colorStyleMap[color] = styleName
		hexColor := strings.TrimPrefix(color, "#")
		automaticStyles.WriteString(fmt.Sprintf(
			`<style:style style:name="%s" style:family="text"><style:text-properties fo:background-color="#%s"/></style:style>`,
			styleName, hexColor))
		colorIndex++
	}

	// Text formatting styles
	automaticStyles.WriteString(`<style:style style:name="TBold" style:family="text"><style:text-properties fo:font-weight="bold" style:font-weight-asian="bold" style:font-weight-complex="bold"/></style:style>`)
	automaticStyles.WriteString(`<style:style style:name="TItalic" style:family="text"><style:text-properties fo:font-style="italic" style:font-style-asian="italic" style:font-style-complex="italic"/></style:style>`)
	automaticStyles.WriteString(`<style:style style:name="TUnderline" style:family="text"><style:text-properties style:text-underline-style="solid" style:text-underline-width="auto" style:text-underline-color="font-color"/></style:style>`)
	automaticStyles.WriteString(`<style:style style:name="TStrike" style:family="text"><style:text-properties style:text-line-through-style="solid" style:text-line-through-type="single"/></style:style>`)
	automaticStyles.WriteString(`<style:style style:name="TBoldItalic" style:family="text"><style:text-properties fo:font-weight="bold" fo:font-style="italic" style:font-weight-asian="bold" style:font-style-asian="italic" style:font-weight-complex="bold" style:font-style-complex="italic"/></style:style>`)

	// Alignment paragraph styles
	automaticStyles.WriteString(`<style:style style:name="PLeft" style:family="paragraph" style:parent-style-name="Standard"><style:paragraph-properties fo:text-align="start"/></style:style>`)
	automaticStyles.WriteString(`<style:style style:name="PCenter" style:family="paragraph" style:parent-style-name="Standard"><style:paragraph-properties fo:text-align="center"/></style:style>`)
	automaticStyles.WriteString(`<style:style style:name="PRight" style:family="paragraph" style:parent-style-name="Standard"><style:paragraph-properties fo:text-align="end"/></style:style>`)
	automaticStyles.WriteString(`<style:style style:name="PJustify" style:family="paragraph" style:parent-style-name="Standard"><style:paragraph-properties fo:text-align="justify"/></style:style>`)

	// List styles
	automaticStyles.WriteString(`<text:list-style style:name="L1">`)
	automaticStyles.WriteString(`<text:list-level-style-bullet text:level="1" text:style-name="Bullet_20_Symbols" text:bullet-char="•">`)
	automaticStyles.WriteString(`<style:list-level-properties text:list-level-position-and-space-mode="label-alignment">`)
	automaticStyles.WriteString(`<style:list-level-label-alignment text:label-followed-by="listtab" text:list-tab-stop-position="1.27cm" fo:text-indent="-0.635cm" fo:margin-left="1.27cm"/>`)
	automaticStyles.WriteString(`</style:list-level-properties>`)
	automaticStyles.WriteString(`</text:list-level-style-bullet>`)
	automaticStyles.WriteString(`</text:list-style>`)

	automaticStyles.WriteString(`<text:list-style style:name="L2">`)
	automaticStyles.WriteString(`<text:list-level-style-number text:level="1" text:style-name="Numbering_20_Symbols" style:num-suffix="." style:num-format="1">`)
	automaticStyles.WriteString(`<style:list-level-properties text:list-level-position-and-space-mode="label-alignment">`)
	automaticStyles.WriteString(`<style:list-level-label-alignment text:label-followed-by="listtab" text:list-tab-stop-position="1.27cm" fo:text-indent="-0.635cm" fo:margin-left="1.27cm"/>`)
	automaticStyles.WriteString(`</style:list-level-properties>`)
	automaticStyles.WriteString(`</text:list-level-style-number>`)
	automaticStyles.WriteString(`</text:list-style>`)

	// Track list state
	inBulletList := false
	inNumberList := false

	for _, para := range paragraphs {
		// Close any open lists if this is a heading or non-list paragraph
		if para.heading != "" || para.listType == "" {
			if inBulletList {
				bodyContent.WriteString("</text:list>")
				inBulletList = false
			}
			if inNumberList {
				bodyContent.WriteString("</text:list>")
				inNumberList = false
			}
		}

		// Handle headings
		if para.heading != "" {
			headingInfo, ok := headingToOdtStyle[para.heading]
			if !ok {
				headingInfo = headingToOdtStyle["h2"] // Default
			}

			if headingInfo.outlineLevel == 0 {
				// Title (kein text:h, sondern text:p mit Title-Style)
				bodyContent.WriteString(fmt.Sprintf(`<text:p text:style-name="%s">`, headingInfo.styleName))
			} else {
				// Heading mit outline-level
				bodyContent.WriteString(fmt.Sprintf(`<text:h text:style-name="%s" text:outline-level="%d">`,
					headingInfo.styleName, headingInfo.outlineLevel))
			}

			e.writeSegments(&bodyContent, para.segments, colorStyleMap)

			if headingInfo.outlineLevel == 0 {
				bodyContent.WriteString("</text:p>")
			} else {
				bodyContent.WriteString("</text:h>")
			}
			continue
		}

		// Determine paragraph style based on alignment
		paraStyle := "Standard"
		switch para.alignment {
		case "left":
			paraStyle = "PLeft"
		case "center":
			paraStyle = "PCenter"
		case "right":
			paraStyle = "PRight"
		case "justify":
			paraStyle = "PJustify"
		}

		// Handle list transitions
		if para.listType == "bullet" {
			if inNumberList {
				bodyContent.WriteString("</text:list>")
				inNumberList = false
			}
			if !inBulletList {
				bodyContent.WriteString(`<text:list text:style-name="L1">`)
				inBulletList = true
			}
			bodyContent.WriteString("<text:list-item>")
			bodyContent.WriteString(fmt.Sprintf(`<text:p text:style-name="%s">`, paraStyle))
		} else if para.listType == "number" {
			if inBulletList {
				bodyContent.WriteString("</text:list>")
				inBulletList = false
			}
			if !inNumberList {
				bodyContent.WriteString(`<text:list text:style-name="L2">`)
				inNumberList = true
			}
			bodyContent.WriteString("<text:list-item>")
			bodyContent.WriteString(fmt.Sprintf(`<text:p text:style-name="%s">`, paraStyle))
		} else {
			bodyContent.WriteString(fmt.Sprintf(`<text:p text:style-name="%s">`, paraStyle))
		}

		e.writeSegments(&bodyContent, para.segments, colorStyleMap)

		bodyContent.WriteString("</text:p>")
		if para.listType != "" {
			bodyContent.WriteString("</text:list-item>")
		}
	}

	// Close any remaining lists
	if inBulletList {
		bodyContent.WriteString("</text:list>")
	}
	if inNumberList {
		bodyContent.WriteString("</text:list>")
	}

	return fmt.Sprintf(odtContentXMLTemplate, automaticStyles.String(), bodyContent.String())
}

func (e *ExportOdt) writeSegments(bodyContent *strings.Builder, segments []odtTextSegment, colorStyleMap map[string]string) {
	for _, seg := range segments {
		styleName := e.getStyleName(seg, colorStyleMap)
		if styleName != "" {
			bodyContent.WriteString(fmt.Sprintf(`<text:span text:style-name="%s">`, styleName))
			bodyContent.WriteString(escapeXMLOdt(seg.text))
			bodyContent.WriteString("</text:span>")
		} else {
			bodyContent.WriteString(escapeXMLOdt(seg.text))
		}
	}
}

func (e *ExportOdt) getStyleName(seg odtTextSegment, colorStyleMap map[string]string) string {
	// Priority: author color, then formatting
	if seg.authorColor != "" {
		return colorStyleMap[seg.authorColor]
	}
	if seg.bold && seg.italic {
		return "TBoldItalic"
	}
	if seg.bold {
		return "TBold"
	}
	if seg.italic {
		return "TItalic"
	}
	if seg.underline {
		return "TUnderline"
	}
	if seg.strikethrough {
		return "TStrike"
	}
	return ""
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

	// Check for list markers and alignment
	if aline != "" {
		ops, err := changeset.DeserializeOps(aline)
		if err != nil {
			return para, err
		}
		if len(*ops) > 0 {
			op := (*ops)[0]
			attribs := changeset.FromString(op.Attribs, padPool)

			// Check for align attribute and remove leading * marker
			alignStr := attribs.Get("align")
			if alignStr != nil {
				para.alignment = *alignStr
				// Remove the leading * marker for aligned lines
				if len(text) > 0 && text[0] == '*' {
					text = text[1:]
					newAline, err := changeset.Subattribution(aline, 1, nil)
					if err != nil {
						return para, err
					}
					aline = *newAline
				}
			}

			listTypeStr := attribs.Get("list")
			if listTypeStr != nil {
				para.listType, para.listLevel = parseListTypeOdt(*listTypeStr)

				// Remove leading * if not already removed by align
				if len(text) > 0 && text[0] == '*' {
					text = text[1:]
					newAline, err := changeset.Subattribution(aline, 1, nil)
					if err != nil {
						return para, err
					}
					aline = *newAline
				}
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
	re := regexp.MustCompile(`^([a-z]+)([0-9]+)`)
	m := re.FindStringSubmatch(listAttr)
	if m == nil {
		return "", 0
	}

	level, _ := strconv.Atoi(m[2])
	tag := m[1]

	switch tag {
	case "bullet":
		return "bullet", level
	case "number":
		return "number", level
	case "indent":
		return "bullet", level
	default:
		return "bullet", level
	}
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
  xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0"
  office:version="1.2">
  <office:styles>
    <style:default-style style:family="paragraph">
      <style:paragraph-properties fo:margin-top="0cm" fo:margin-bottom="0cm"/>
      <style:text-properties fo:font-size="12pt" fo:font-family="Arial"/>
    </style:default-style>
    <style:style style:name="Standard" style:family="paragraph"/>
    <style:style style:name="Bullet_20_Symbols" style:display-name="Bullet Symbols" style:family="text"/>
    <style:style style:name="Numbering_20_Symbols" style:display-name="Numbering Symbols" style:family="text"/>
    <style:style style:name="Title" style:family="paragraph" style:class="chapter">
      <style:paragraph-properties fo:margin-top="0cm" fo:margin-bottom="0.5cm" fo:text-align="center"/>
      <style:text-properties fo:font-size="28pt" fo:font-weight="bold" style:font-size-asian="28pt" style:font-weight-asian="bold" style:font-size-complex="28pt" style:font-weight-complex="bold"/>
    </style:style>
    <style:style style:name="Heading_20_1" style:display-name="Heading 1" style:family="paragraph" style:parent-style-name="Standard" style:class="text">
      <style:paragraph-properties fo:margin-top="0.423cm" fo:margin-bottom="0.212cm" fo:keep-with-next="always"/>
      <style:text-properties fo:font-size="24pt" fo:font-weight="bold" style:font-size-asian="24pt" style:font-weight-asian="bold" style:font-size-complex="24pt" style:font-weight-complex="bold"/>
    </style:style>
    <style:style style:name="Heading_20_2" style:display-name="Heading 2" style:family="paragraph" style:parent-style-name="Standard" style:class="text">
      <style:paragraph-properties fo:margin-top="0.353cm" fo:margin-bottom="0.212cm" fo:keep-with-next="always"/>
      <style:text-properties fo:font-size="18pt" fo:font-weight="bold" style:font-size-asian="18pt" style:font-weight-asian="bold" style:font-size-complex="18pt" style:font-weight-complex="bold"/>
    </style:style>
    <style:style style:name="Heading_20_3" style:display-name="Heading 3" style:family="paragraph" style:parent-style-name="Standard" style:class="text">
      <style:paragraph-properties fo:margin-top="0.247cm" fo:margin-bottom="0.212cm" fo:keep-with-next="always"/>
      <style:text-properties fo:font-size="14pt" fo:font-weight="bold" style:font-size-asian="14pt" style:font-weight-asian="bold" style:font-size-complex="14pt" style:font-weight-complex="bold"/>
    </style:style>
    <style:style style:name="Heading_20_4" style:display-name="Heading 4" style:family="paragraph" style:parent-style-name="Standard" style:class="text">
      <style:paragraph-properties fo:margin-top="0.212cm" fo:margin-bottom="0.212cm" fo:keep-with-next="always"/>
      <style:text-properties fo:font-size="12pt" fo:font-weight="bold" fo:font-style="italic" style:font-size-asian="12pt" style:font-weight-asian="bold" style:font-style-asian="italic" style:font-size-complex="12pt" style:font-weight-complex="bold" style:font-style-complex="italic"/>
    </style:style>
    <style:style style:name="Heading_20_5" style:display-name="Heading 5" style:family="paragraph" style:parent-style-name="Standard" style:class="text">
      <style:paragraph-properties fo:margin-top="0.212cm" fo:margin-bottom="0.212cm" fo:keep-with-next="always"/>
      <style:text-properties fo:font-size="10pt" fo:font-weight="bold" style:font-size-asian="10pt" style:font-weight-asian="bold" style:font-size-complex="10pt" style:font-weight-complex="bold"/>
    </style:style>
  </office:styles>
  <office:outline-styles>
    <text:outline-style style:name="Outline">
      <text:outline-level-style text:level="1" style:num-format=""/>
      <text:outline-level-style text:level="2" style:num-format=""/>
      <text:outline-level-style text:level="3" style:num-format=""/>
      <text:outline-level-style text:level="4" style:num-format=""/>
      <text:outline-level-style text:level="5" style:num-format=""/>
    </text:outline-style>
  </office:outline-styles>
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
