package io

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	padModel "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ledongthuc/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

type Importer struct {
	padManager    *pad.Manager
	authorManager *author.Manager
	db            db.DataStore
	logger        *zap.SugaredLogger
}

func NewImporter(padManager *pad.Manager, authorManager *author.Manager, db db.DataStore, logger *zap.SugaredLogger) *Importer {
	return &Importer{
		padManager:    padManager,
		authorManager: authorManager,
		db:            db,
		logger:        logger,
	}
}

type EtherpadImport struct {
	rawData map[string]json.RawMessage
}

func (i *Importer) SetPadRaw(padId string, content []byte, authorId string) error {
	var rawData map[string]json.RawMessage
	if err := json.Unmarshal(content, &rawData); err != nil {
		return errors.New("invalid etherpad JSON: " + err.Error())
	}

	var padData PadData
	var foundPadData bool

	padKeyRegex := regexp.MustCompile(`^pad:[^:]+:$`)
	for key, value := range rawData {
		if padKeyRegex.MatchString(key) {
			// Try to parse as PadData
			if err := json.Unmarshal(value, &padData); err == nil {
				foundPadData = true
				if i.logger != nil {
					i.logger.Infof("Found pad data at key: %s", key)
				}
				break
			}
		}
	}

	if !foundPadData {
		return errors.New("no pad data found in etherpad file")
	}

	_ = i.db.RemovePad(padId)

	pool := apool.NewAPool()
	for numStr, attrib := range padData.Pool.NumToAttrib {
		num, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		if len(attrib) >= 2 {
			pool.NumToAttrib[num] = apool.Attribute{
				Key:   attrib[0],
				Value: attrib[1],
			}
		}
	}
	pool.NextNum = padData.Pool.NextNum

	atext := apool.AText{
		Text:    padData.AText.Text,
		Attribs: padData.AText.Attribs,
	}

	revisions := make(map[int]Revision)
	revisionRegex := regexp.MustCompile(`^pad:[^:]+:revs:(\d+)$`)

	for key, value := range rawData {
		matches := revisionRegex.FindStringSubmatch(key)
		if matches != nil {
			revNum, _ := strconv.Atoi(matches[1])
			var rev Revision
			if err := json.Unmarshal(value, &rev); err == nil {
				revisions[revNum] = rev
			}
		}
	}

	authorRegex := regexp.MustCompile(`^globalAuthor:(.+)$`)
	for key, value := range rawData {
		matches := authorRegex.FindStringSubmatch(key)
		if matches != nil {
			authorIdFromFile := matches[1]
			var authorData GlobalAuthor
			if err := json.Unmarshal(value, &authorData); err == nil {
				existingAuthor, _ := i.authorManager.GetAuthor(authorIdFromFile)
				if existingAuthor == nil {
					i.db.SaveAuthor(db2.AuthorDB{
						ID:        authorIdFromFile,
						Name:      authorData.Name,
						ColorId:   authorData.ColorId,
						Timestamp: authorData.Timestamp,
						PadIDs:    make(map[string]struct{}),
					})
				} else {
					if authorData.ColorId != "" {
						i.authorManager.SetAuthorColor(authorIdFromFile, authorData.ColorId)
					}
					if authorData.Name != nil {
						i.authorManager.SetAuthorName(authorIdFromFile, *authorData.Name)
					}
				}
			}
		}
	}

	dbPad := db2.PadDB{
		SavedRevisions: make(map[int]db2.PadRevision),
		Revisions:      make(map[int]db2.PadSingleRevision),
		RevNum:         padData.Head,
		Pool:           pool.ToPadDB(),
		AText:          db2.AText{Text: atext.Text, Attribs: atext.Attribs},
		ChatHead:       padData.ChatHead,
		PublicStatus:   padData.PublicStatus,
	}

	if err := i.db.CreatePad(padId, dbPad); err != nil {
		return errors.New("failed to save pad: " + err.Error())
	}

	for revNum, rev := range revisions {
		var revAuthor *string
		if rev.Meta.Author != nil {
			revAuthor = rev.Meta.Author
		}
		var timestamp int64
		if rev.Meta.Timestamp != nil {
			timestamp = *rev.Meta.Timestamp
		}

		// Get atext from revision meta if available
		var revAText db2.AText
		if rev.Meta.AText != nil {
			revAText = db2.AText{
				Text:    rev.Meta.AText.Text,
				Attribs: rev.Meta.AText.Attribs,
			}
		}

		// Get pool from revision meta if available
		var revPool db2.RevPool
		if rev.Meta.Pool != nil {
			revPool = db2.RevPool{
				NumToAttrib: rev.Meta.Pool.NumToAttrib,
				AttribToNum: rev.Meta.Pool.AttribToNum,
				NextNum:     rev.Meta.Pool.NextNum,
			}
		}

		err := i.db.SaveRevision(padId, revNum, rev.Changeset, revAText, revPool, revAuthor, timestamp)
		if err != nil {
			if i.logger != nil {
				i.logger.Warnf("Failed to save revision %d: %v", revNum, err)
			}
		}
	}

	newText := "\n"
	retrievedPad, err := i.padManager.GetPad(padId, &newText, nil)
	if err != nil {
		i.logger.Warnf("Could not get pad for chat import: %v", err)
	} else {
		chatRegex := regexp.MustCompile(`^pad:[^:]+:chat:(\d+)$`)
		for key, value := range rawData {
			matches := chatRegex.FindStringSubmatch(key)
			if matches != nil {
				var chat ChatMessage
				if err := json.Unmarshal(value, &chat); err == nil {
					var time int64
					if chat.Time != nil {
						time = *chat.Time
					}
					var userId string
					if chat.UserId != nil {
						userId = *chat.UserId
					}
					retrievedPad.AppendChatMessage(&userId, time, chat.Text)
				}
			}
		}
	}

	return nil
}

// SetPadHTML imports HTML content into a pad
// Note: This currently imports only the text content. Full formatting support
// would require complex changeset generation which is error-prone.
// For full formatting preservation, export/import via .etherpad format is recommended.
func (i *Importer) SetPadHTML(pad *padModel.Pad, htmlContent string, authorId string) error {
	// Parse HTML and extract text
	text, err := i.htmlToText(htmlContent)
	if err != nil {
		return err
	}

	// If no content, just set empty
	if text == "" || text == "\n" {
		return pad.SetText("\n", &authorId)
	}

	// Set the plain text
	return pad.SetText(text, &authorId)
}

// applyAttributeToRange applies an attribute to a range of text in the pad
func (i *Importer) applyAttributeToRange(pad *padModel.Pad, start, length int, attribStr string, authorId *string) error {
	if length <= 0 {
		return nil
	}

	text := pad.Text()
	textLen := len([]rune(text))

	if start >= textLen || start+length > textLen {
		return nil
	}

	// Build a changeset that keeps text before, applies attribute to range, keeps rest
	var ops strings.Builder

	// Keep before (if any)
	if start > 0 {
		beforeText := string([]rune(text)[:start])
		lines := strings.Count(beforeText, "\n")
		if lines > 0 {
			ops.WriteString("|")
			ops.WriteString(strconv.Itoa(lines))
		}
		ops.WriteString("=")
		ops.WriteString(strconv.Itoa(start))
	}

	// Apply attribute to the range
	rangeText := string([]rune(text)[start : start+length])
	lines := strings.Count(rangeText, "\n")
	ops.WriteString(attribStr)
	if lines > 0 {
		ops.WriteString("|")
		ops.WriteString(strconv.Itoa(lines))
	}
	ops.WriteString("=")
	ops.WriteString(strconv.Itoa(length))

	// Build the changeset
	cs := "Z:" + strconv.FormatInt(int64(textLen), 36) + ">0" + ops.String() + "$"

	// Apply the changeset
	_, err := pad.AppendRevision(cs, authorId)
	return err
}

// htmlParseResult contains the parsed HTML content
type htmlParseResult struct {
	text     string
	segments []htmlSegment
}

// htmlSegment represents a segment of text with attributes
type htmlSegment struct {
	start      int
	length     int
	attributes map[string]string
}

// parseHTMLWithFormatting parses HTML and extracts text with formatting information
func (i *Importer) parseHTMLWithFormatting(htmlContent string) (*htmlParseResult, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	result := &htmlParseResult{
		segments: make([]htmlSegment, 0),
	}

	var sb strings.Builder
	activeAttrs := make(map[string]string)
	i.extractHTMLWithFormatting(doc, &sb, result, activeAttrs, 0)

	result.text = sb.String()

	// Clean up multiple newlines
	// We need to adjust segment positions if we clean up the text
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	result.text = multipleNewlines.ReplaceAllString(result.text, "\n\n")

	// Ensure text ends with newline
	if !strings.HasSuffix(result.text, "\n") {
		result.text += "\n"
	}

	return result, nil
}

// extractHTMLWithFormatting recursively extracts text and formatting from HTML nodes
func (i *Importer) extractHTMLWithFormatting(n *html.Node, sb *strings.Builder, result *htmlParseResult, activeAttrs map[string]string, depth int) {
	// Skip certain elements
	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "head":
			return
		}
	}

	// Handle text nodes
	if n.Type == html.TextNode {
		text := n.Data
		// Don't trim space inside inline elements, but clean up excessive whitespace
		text = regexp.MustCompile(`[\t\r]+`).ReplaceAllString(text, " ")

		if text != "" && text != " " {
			startPos := sb.Len()
			// Count runes for proper position
			startRunes := len([]rune(sb.String()))
			sb.WriteString(text)

			// If there are active attributes, create a segment
			if len(activeAttrs) > 0 && len(text) > 0 {
				// Copy attributes
				attrs := make(map[string]string)
				for k, v := range activeAttrs {
					attrs[k] = v
				}
				result.segments = append(result.segments, htmlSegment{
					start:      startRunes,
					length:     len([]rune(text)),
					attributes: attrs,
				})
			}
			_ = startPos // unused but kept for clarity
		}
		return
	}

	// Handle element nodes - determine formatting
	newAttrs := make(map[string]string)
	for k, v := range activeAttrs {
		newAttrs[k] = v
	}

	listType := ""
	isListItem := false
	isBlock := false

	if n.Type == html.ElementNode {
		switch n.Data {
		case "strong", "b":
			newAttrs["bold"] = "true"
		case "em", "i":
			newAttrs["italic"] = "true"
		case "u":
			newAttrs["underline"] = "true"
		case "s", "strike", "del":
			newAttrs["strikethrough"] = "true"
		case "h1":
			newAttrs["heading"] = "h1"
			isBlock = true
		case "h2":
			newAttrs["heading"] = "h2"
			isBlock = true
		case "h3", "h4", "h5", "h6":
			newAttrs["heading"] = "h2" // Map all to h2 for simplicity
			isBlock = true
		case "ul":
			// Check for class to determine list type
			listType = "bullet"
			for _, attr := range n.Attr {
				if attr.Key == "class" {
					if strings.Contains(attr.Val, "bullet") {
						listType = "bullet"
					} else if strings.Contains(attr.Val, "indent") {
						listType = "indent"
					}
				}
			}
		case "ol":
			listType = "number"
		case "li":
			isListItem = true
			isBlock = true
		case "p", "div":
			isBlock = true
		case "br":
			sb.WriteString("\n")
			return
		}
	}

	// Handle list items - add list marker
	if isListItem {
		// Find parent list type
		parent := n.Parent
		for parent != nil {
			if parent.Type == html.ElementNode {
				if parent.Data == "ul" {
					for _, attr := range parent.Attr {
						if attr.Key == "class" {
							if strings.Contains(attr.Val, "bullet") {
								newAttrs["list"] = "bullet1"
							} else if strings.Contains(attr.Val, "indent") {
								newAttrs["list"] = "indent1"
							} else {
								newAttrs["list"] = "bullet1"
							}
							break
						}
					}
					if _, ok := newAttrs["list"]; !ok {
						newAttrs["list"] = "bullet1"
					}
					break
				} else if parent.Data == "ol" {
					newAttrs["list"] = "number1"
					break
				}
			}
			parent = parent.Parent
		}

		// Add list marker character
		sb.WriteString("*")
	}

	// Process children
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		i.extractHTMLWithFormatting(c, sb, result, newAttrs, depth+1)
	}

	// Add newline after block elements
	if isBlock {
		// Only add newline if we don't already end with one
		str := sb.String()
		if len(str) > 0 && !strings.HasSuffix(str, "\n") {
			sb.WriteString("\n")
		}
	}

	// Inherit list type for nested processing
	_ = listType
}

// htmlToText converts HTML to plain text, preserving basic structure (legacy method)
func (i *Importer) htmlToText(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	i.extractText(doc, &sb)

	text := sb.String()

	// Clean up multiple newlines
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	text = multipleNewlines.ReplaceAllString(text, "\n\n")

	// Ensure text ends with newline
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}

	return text, nil
}

// extractText recursively extracts text from HTML nodes
func (i *Importer) extractText(n *html.Node, sb *strings.Builder) {
	i.extractTextWithContext(n, sb, false, 0)
}

// extractTextWithContext recursively extracts text from HTML nodes with list context
func (i *Importer) extractTextWithContext(n *html.Node, sb *strings.Builder, inList bool, listCounter int) int {
	if n.Type == html.TextNode {
		text := n.Data
		// Clean up whitespace but preserve single spaces
		text = regexp.MustCompile(`[\t\r\n]+`).ReplaceAllString(text, " ")
		text = strings.TrimSpace(text)
		if text != "" {
			sb.WriteString(text)
		}
		return listCounter
	}

	// Handle block elements - add newlines
	isBlock := false
	isListItem := false
	isOrderedList := false
	isUnorderedList := false

	if n.Type == html.ElementNode {
		switch n.Data {
		case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "tr":
			isBlock = true
		case "br":
			sb.WriteString("\n")
			return listCounter
		case "li":
			isListItem = true
			isBlock = true
		case "ol":
			isOrderedList = true
		case "ul":
			isUnorderedList = true
		case "script", "style", "head":
			// Skip these elements entirely
			return listCounter
		}
	}

	// Add list marker before list item content
	if isListItem {
		// Find parent list type
		parent := n.Parent
		for parent != nil {
			if parent.Type == html.ElementNode {
				if parent.Data == "ol" {
					listCounter++
					sb.WriteString(strconv.Itoa(listCounter))
					sb.WriteString(". ")
					break
				} else if parent.Data == "ul" {
					sb.WriteString("• ")
					break
				}
			}
			parent = parent.Parent
		}
	}

	// Reset counter for new ordered list
	if isOrderedList {
		listCounter = 0
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		listCounter = i.extractTextWithContext(c, sb, inList || isUnorderedList || isOrderedList, listCounter)
	}

	if isBlock {
		// Only add newline if we don't already end with one
		str := sb.String()
		if len(str) > 0 && !strings.HasSuffix(str, "\n") {
			sb.WriteString("\n")
		}
	}

	return listCounter
}

// SetPadText imports plain text into a pad
func (i *Importer) SetPadText(pad *padModel.Pad, text string, authorId string) error {
	// Ensure text ends with newline
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}

	return pad.SetText(text, &authorId)
}

// ExtractTextFromDocx extracts text content from a DOCX file
func (i *Importer) ExtractTextFromDocx(content []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", errors.New("invalid DOCX file: " + err.Error())
	}

	// Find and read the document.xml file
	var documentXML []byte
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			documentXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
			break
		}
	}

	if documentXML == nil {
		return "", errors.New("no document.xml found in DOCX")
	}

	// Parse the XML and extract text
	return i.extractTextFromDocxXML(documentXML)
}

// extractTextFromDocxXML extracts text from DOCX XML content
func (i *Importer) extractTextFromDocxXML(xmlContent []byte) (string, error) {
	var sb strings.Builder
	decoder := xml.NewDecoder(bytes.NewReader(xmlContent))

	inParagraph := false
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		switch t := token.(type) {
		case xml.StartElement:
			// w:p is paragraph, w:br is break
			if t.Name.Local == "p" {
				inParagraph = true
			}
		case xml.EndElement:
			if t.Name.Local == "p" && inParagraph {
				sb.WriteString("\n")
				inParagraph = false
			}
		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" {
				sb.WriteString(text)
			}
		}
	}

	result := sb.String()
	// Clean up multiple newlines
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	result = multipleNewlines.ReplaceAllString(result, "\n\n")

	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	return result, nil
}

// ExtractTextFromOdt extracts text content from an ODT file
func (i *Importer) ExtractTextFromOdt(content []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", errors.New("invalid ODT file: " + err.Error())
	}

	// Find and read the content.xml file
	var contentXML []byte
	for _, file := range reader.File {
		if file.Name == "content.xml" {
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			contentXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
			break
		}
	}

	if contentXML == nil {
		return "", errors.New("no content.xml found in ODT")
	}

	// Parse the XML and extract text
	return i.extractTextFromOdtXML(contentXML)
}

// extractTextFromOdtXML extracts text from ODT XML content
func (i *Importer) extractTextFromOdtXML(xmlContent []byte) (string, error) {
	var sb strings.Builder
	decoder := xml.NewDecoder(bytes.NewReader(xmlContent))

	inParagraph := false
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		switch t := token.(type) {
		case xml.StartElement:
			// text:p is paragraph, text:h is heading, text:line-break is break
			if t.Name.Local == "p" || t.Name.Local == "h" {
				inParagraph = true
			} else if t.Name.Local == "line-break" {
				sb.WriteString("\n")
			} else if t.Name.Local == "tab" {
				sb.WriteString("\t")
			} else if t.Name.Local == "s" {
				// Space element - check for count attribute
				count := 1
				for _, attr := range t.Attr {
					if attr.Name.Local == "c" {
						if n, err := strconv.Atoi(attr.Value); err == nil {
							count = n
						}
					}
				}
				sb.WriteString(strings.Repeat(" ", count))
			}
		case xml.EndElement:
			if (t.Name.Local == "p" || t.Name.Local == "h") && inParagraph {
				sb.WriteString("\n")
				inParagraph = false
			}
		case xml.CharData:
			text := string(t)
			if text != "" && inParagraph {
				sb.WriteString(text)
			}
		}
	}

	result := sb.String()
	// Clean up multiple newlines
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	result = multipleNewlines.ReplaceAllString(result, "\n\n")

	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	return result, nil
}

// ExtractTextFromRtf extracts text content from an RTF file
func (i *Importer) ExtractTextFromRtf(content []byte) (string, error) {
	// RTF parsing is complex, we use a simplified approach
	// that strips RTF control words and extracts plain text
	text := string(content)

	// Remove RTF header
	if !strings.HasPrefix(text, "{\\rtf") {
		return "", errors.New("invalid RTF file")
	}

	var sb strings.Builder
	inGroup := 0
	skipGroup := false
	i2 := 0

	for i2 < len(text) {
		ch := text[i2]

		if ch == '{' {
			inGroup++
			// Check if this is a group to skip (like \fonttbl, \colortbl, etc.)
			if i2+1 < len(text) && text[i2+1] == '\\' {
				rest := text[i2+1:]
				if strings.HasPrefix(rest, "\\fonttbl") ||
					strings.HasPrefix(rest, "\\colortbl") ||
					strings.HasPrefix(rest, "\\stylesheet") ||
					strings.HasPrefix(rest, "\\info") ||
					strings.HasPrefix(rest, "\\*") {
					skipGroup = true
				}
			}
			i2++
			continue
		}

		if ch == '}' {
			inGroup--
			if inGroup == 0 {
				skipGroup = false
			}
			i2++
			continue
		}

		if skipGroup {
			i2++
			continue
		}

		if ch == '\\' {
			// Control word
			i2++
			if i2 >= len(text) {
				break
			}

			// Special characters
			if text[i2] == '\'' && i2+2 < len(text) {
				// Hex character
				i2 += 3
				continue
			}

			// Check for special control words
			if strings.HasPrefix(text[i2:], "par") || strings.HasPrefix(text[i2:], "line") {
				sb.WriteString("\n")
			} else if strings.HasPrefix(text[i2:], "tab") {
				sb.WriteString("\t")
			}

			// Skip the control word
			for i2 < len(text) && ((text[i2] >= 'a' && text[i2] <= 'z') || (text[i2] >= 'A' && text[i2] <= 'Z')) {
				i2++
			}
			// Skip optional numeric parameter
			for i2 < len(text) && ((text[i2] >= '0' && text[i2] <= '9') || text[i2] == '-') {
				i2++
			}
			// Skip optional space after control word
			if i2 < len(text) && text[i2] == ' ' {
				i2++
			}
			continue
		}

		if ch == '\n' || ch == '\r' {
			i2++
			continue
		}

		// Regular character
		sb.WriteByte(ch)
		i2++
	}

	result := sb.String()
	// Clean up multiple newlines
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	result = multipleNewlines.ReplaceAllString(result, "\n\n")

	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	return result, nil
}

// ExtractTextFromPdf extracts text content from a PDF file using the ledongthuc/pdf library
func (i *Importer) ExtractTextFromPdf(content []byte) (string, error) {
	// Check for PDF header
	if len(content) < 4 || string(content[:4]) != "%PDF" {
		return "", errors.New("invalid PDF file")
	}

	// Create a reader from the content
	reader := bytes.NewReader(content)

	// Open the PDF
	pdfReader, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("could not open PDF: %w", err)
	}

	var sb strings.Builder

	// Extract text from all pages
	numPages := pdfReader.NumPage()
	for pageNum := 1; pageNum <= numPages; pageNum++ {
		page := pdfReader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			// Try to continue with other pages even if one fails
			i.logger.Warnf("Could not extract text from page %d: %v", pageNum, err)
			continue
		}

		if text != "" {
			sb.WriteString(text)
			// Add page separator if not the last page
			if pageNum < numPages {
				sb.WriteString("\n")
			}
		}
	}

	result := sb.String()

	// If no text was extracted, return an error
	if strings.TrimSpace(result) == "" {
		return "", errors.New("could not extract text from PDF - the PDF may be image-based or encrypted")
	}

	// Clean up multiple newlines
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	result = multipleNewlines.ReplaceAllString(result, "\n\n")

	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	return result, nil
}

// ExtractEtherpadFromPdf extracts embedded Etherpad JSON data from a PDF file
// This is similar to how ZUGFeRD embeds XML in PDF, but uses JSON for Etherpad data
// Returns nil if no embedded Etherpad data is found
func (i *Importer) ExtractEtherpadFromPdf(content []byte) ([]byte, error) {
	// Check for PDF header
	if len(content) < 4 || string(content[:4]) != "%PDF" {
		return nil, errors.New("invalid PDF file")
	}

	// Create temporary directory for pdfcpu operations
	tempDir, err := os.MkdirTemp("", "etherpad-pdf-import-*")
	if err != nil {
		return nil, fmt.Errorf("could not create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Write PDF to temp file
	inputPdfPath := tempDir + "/input.pdf"
	if err := os.WriteFile(inputPdfPath, content, 0644); err != nil {
		return nil, fmt.Errorf("could not write pdf temp file: %w", err)
	}

	// Configure pdfcpu for relaxed validation
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed

	// Extract attachments from the PDF
	outputDir := tempDir + "/attachments"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("could not create output dir: %w", err)
	}

	err = api.ExtractAttachmentsFile(inputPdfPath, outputDir, []string{"etherpad.json"}, conf)
	if err != nil {
		// Try extracting all attachments if specific one fails
		err = api.ExtractAttachmentsFile(inputPdfPath, outputDir, nil, conf)
		if err != nil {
			return nil, fmt.Errorf("could not extract attachments: %w", err)
		}
	}

	// Look for etherpad.json in the output directory
	jsonPath := outputDir + "/etherpad.json"
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, errors.New("no embedded Etherpad data found in PDF")
	}

	i.logger.Info("Found embedded etherpad.json in PDF")
	return jsonData, nil
}

// ExtractTextFromEtherpadJson extracts the plain text content from Etherpad JSON export
func (i *Importer) ExtractTextFromEtherpadJson(content []byte) (string, error) {
	var rawData map[string]json.RawMessage
	if err := json.Unmarshal(content, &rawData); err != nil {
		return "", fmt.Errorf("could not parse etherpad JSON: %w", err)
	}

	padRegex := regexp.MustCompile(`^pad:.+:$`)
	for key, value := range rawData {
		if padRegex.MatchString(key) {
			var padData PadData
			if err := json.Unmarshal(value, &padData); err == nil && padData.AText.Text != "" {
				if i.logger != nil {
					i.logger.Infof("Extracted text from Etherpad JSON (key: %s, length: %d)", key, len(padData.AText.Text))
				}
				return padData.AText.Text, nil
			}
		}
	}

	return "", errors.New("no text content found in etherpad JSON")
}
