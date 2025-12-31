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
	padLib "github.com/ether/etherpad-go/lib/pad"
)

type ExportDocx struct {
	padManager    *padLib.Manager
	authorManager *author.Manager
}

func NewExportDocx(padManager *padLib.Manager, authorManager *author.Manager) *ExportDocx {
	return &ExportDocx{
		padManager:    padManager,
		authorManager: authorManager,
	}
}

type docxTextSegment struct {
	text          string
	bold          bool
	italic        bool
	underline     bool
	strikethrough bool
	authorColor   string
}

type docxParagraph struct {
	segments  []docxTextSegment
	listType  string // "bullet", "number", or ""
	listLevel int    // 0-based level
}

func (e *ExportDocx) GetPadDocxDocument(padId string, optRevNum *int) ([]byte, error) {
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

	var paragraphs []docxParagraph

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

	// Generate DOCX
	return e.generateDocx(paragraphs)
}

func (e *ExportDocx) generateDocx(paragraphs []docxParagraph) ([]byte, error) {
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// Add required DOCX files
	files := map[string]string{
		"[Content_Types].xml":          contentTypesXML,
		"_rels/.rels":                  relsXML,
		"word/_rels/document.xml.rels": wordRelsXML,
		"word/styles.xml":              stylesXML,
		"word/numbering.xml":           numberingXML,
		"word/document.xml":            e.generateDocumentXML(paragraphs),
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

func (e *ExportDocx) generateDocumentXML(paragraphs []docxParagraph) string {
	var bodyContent strings.Builder

	for _, para := range paragraphs {
		bodyContent.WriteString("<w:p>")

		// Add paragraph properties for lists
		if para.listType != "" {
			bodyContent.WriteString("<w:pPr>")
			// numId 1 = bullet, numId 2 = numbered
			numId := 1
			if para.listType == "number" {
				numId = 2
			}
			bodyContent.WriteString(fmt.Sprintf(`<w:numPr><w:ilvl w:val="%d"/><w:numId w:val="%d"/></w:numPr>`, para.listLevel-1, numId))
			bodyContent.WriteString("</w:pPr>")
		}

		for _, seg := range para.segments {
			bodyContent.WriteString("<w:r>")

			// Run properties
			if seg.bold || seg.italic || seg.underline || seg.strikethrough || seg.authorColor != "" {
				bodyContent.WriteString("<w:rPr>")
				if seg.bold {
					bodyContent.WriteString("<w:b/>")
				}
				if seg.italic {
					bodyContent.WriteString("<w:i/>")
				}
				if seg.underline {
					bodyContent.WriteString(`<w:u w:val="single"/>`)
				}
				if seg.strikethrough {
					bodyContent.WriteString("<w:strike/>")
				}
				if seg.authorColor != "" {
					hexColor := strings.TrimPrefix(seg.authorColor, "#")
					bodyContent.WriteString(fmt.Sprintf(`<w:shd w:val="clear" w:fill="%s"/>`, hexColor))
				}
				bodyContent.WriteString("</w:rPr>")
			}

			bodyContent.WriteString("<w:t xml:space=\"preserve\">")
			bodyContent.WriteString(escapeXML(seg.text))
			bodyContent.WriteString("</w:t></w:r>")
		}

		bodyContent.WriteString("</w:p>")
	}

	return fmt.Sprintf(documentXMLTemplate, bodyContent.String())
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func (e *ExportDocx) buildAuthorColorCache(padPool *apool.APool) map[string]string {
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

func (e *ExportDocx) parseLineSegments(text string, aline string, padPool *apool.APool, authorColors map[string]string) (docxParagraph, error) {
	para := docxParagraph{}

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
				// Parse list type and level (e.g., "bullet1", "number2")
				para.listType, para.listLevel = parseListType(*listTypeStr)

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
			para.segments = append(para.segments, docxTextSegment{text: text})
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
		seg := docxTextSegment{text: segText}

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
		para.segments = append(para.segments, docxTextSegment{text: string(textRunes[pos:])})
	}

	return para, nil
}

// parseListType extracts the list type and level from a list attribute value
// e.g., "bullet1" -> ("bullet", 1), "number2" -> ("number", 2)
func parseListType(listAttr string) (string, int) {
	// Use regex to match any list type like bullet1, number1, indent1, etc.
	re := regexp.MustCompile(`^([a-z]+)([0-9]+)`)
	m := re.FindStringSubmatch(listAttr)
	if m == nil {
		return "", 0
	}

	level, _ := strconv.Atoi(m[2])
	tag := m[1]

	// Map Etherpad list types to DOCX list types
	switch tag {
	case "bullet":
		return "bullet", level
	case "number":
		return "number", level
	case "indent":
		// indent is treated as bullet list without bullet char
		return "bullet", level
	default:
		// Unknown list type, treat as bullet
		return "bullet", level
	}
}

// DOCX XML Templates
const contentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>
</Types>`

const relsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`

const wordRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>
</Relationships>`

const stylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docDefaults>
    <w:rPrDefault>
      <w:rPr>
        <w:rFonts w:ascii="Arial" w:hAnsi="Arial" w:cs="Arial"/>
        <w:sz w:val="24"/>
        <w:szCs w:val="24"/>
      </w:rPr>
    </w:rPrDefault>
  </w:docDefaults>
</w:styles>`

const numberingXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <!-- Abstract numbering definition for bullets -->
  <w:abstractNum w:abstractNumId="0">
    <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="◦"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="1440" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="2"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="▪"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="2160" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="3"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="2880" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="4"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="◦"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="3600" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="5"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="▪"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="4320" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="6"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="5040" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="7"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="◦"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="5760" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="8"><w:start w:val="1"/><w:numFmt w:val="bullet"/><w:lvlText w:val="▪"/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="6480" w:hanging="360"/></w:pPr></w:lvl>
  </w:abstractNum>
  <!-- Abstract numbering definition for numbered lists -->
  <w:abstractNum w:abstractNumId="1">
    <w:lvl w:ilvl="0"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="1"><w:start w:val="1"/><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="%2."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="1440" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="2"><w:start w:val="1"/><w:numFmt w:val="lowerRoman"/><w:lvlText w:val="%3."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="2160" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="3"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%4."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="2880" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="4"><w:start w:val="1"/><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="%5."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="3600" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="5"><w:start w:val="1"/><w:numFmt w:val="lowerRoman"/><w:lvlText w:val="%6."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="4320" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="6"><w:start w:val="1"/><w:numFmt w:val="decimal"/><w:lvlText w:val="%7."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="5040" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="7"><w:start w:val="1"/><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="%8."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="5760" w:hanging="360"/></w:pPr></w:lvl>
    <w:lvl w:ilvl="8"><w:start w:val="1"/><w:numFmt w:val="lowerRoman"/><w:lvlText w:val="%9."/><w:lvlJc w:val="left"/><w:pPr><w:ind w:left="6480" w:hanging="360"/></w:pPr></w:lvl>
  </w:abstractNum>
  <!-- Concrete numbering instances -->
  <w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>
  <w:num w:numId="2"><w:abstractNumId w:val="1"/></w:num>
</w:numbering>`

const documentXMLTemplate = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    %s
  </w:body>
</w:document>`
