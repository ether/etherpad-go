package io

import (
	"bytes"
	"context"
	"html"
	"regexp"
	"strings"

	"github.com/ether/etherpad-go/assets/export"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	padModel "github.com/ether/etherpad-go/lib/models/pad"
	padLib "github.com/ether/etherpad-go/lib/pad"
)

type ExportHtml struct {
	PadManager    *padLib.Manager
	AuthorManager *author.Manager
	Hooks         *hooks.Hook
}

func NewExportHtml(padManager *padLib.Manager, authorManager *author.Manager, hooks *hooks.Hook) *ExportHtml {
	return &ExportHtml{
		PadManager:    padManager,
		AuthorManager: authorManager,
		Hooks:         hooks,
	}
}

// HTML Tags mapped to their property names
var htmlTags = []string{"h1", "h2", "strong", "em", "u", "s"}
var htmlProps = []string{"heading1", "heading2", "bold", "italic", "underline", "strikethrough"}

type openList struct {
	level    int
	listType string
}

// GetPadHTMLDocument returns the full HTML document for a pad
func (e *ExportHtml) GetPadHTMLDocument(padId string, revNum *int, readOnlyId *string) (string, error) {
	retrievedPad, err := e.PadManager.GetPad(padId, nil, nil)
	if err != nil {
		return "", err
	}

	htmlContent, err := e.GetPadHTML(retrievedPad, revNum, nil)
	if err != nil {
		return "", err
	}

	displayId := padId
	if readOnlyId != nil {
		displayId = *readOnlyId
	}

	// Render the template
	var buf bytes.Buffer
	err = export.ExportTemplate(escapeHTMLContent(displayId), "", htmlContent).Render(context.Background(), &buf)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GetPadHTML returns the HTML content for a pad (without document wrapper)
func (e *ExportHtml) GetPadHTML(pad *padModel.Pad, revNum *int, authorColors map[string]string) (string, error) {
	atext := pad.AText

	if revNum != nil {
		revision, err := pad.GetRevision(*revNum)
		if err != nil {
			return "", err
		}
		atext = apool.AText{
			Text:    revision.AText.Text,
			Attribs: revision.AText.Attribs,
		}
	}

	return e.getHTMLFromAtext(pad.Id, &pad.Pool, atext, authorColors)
}

// getHTMLFromAtext converts an AText to HTML
func (e *ExportHtml) getHTMLFromAtext(padId string, padPool *apool.APool, atext apool.AText, authorColors map[string]string) (string, error) {
	textLines := padLib.SplitRemoveLastRune(atext.Text)
	attribLines, err := changeset.SplitAttributionLines(atext.Attribs, atext.Text)
	if err != nil {
		return "", err
	}

	// Create local copies of tags and props that can be extended
	tags := make([]string, len(htmlTags))
	copy(tags, htmlTags)
	props := make([]interface{}, len(htmlProps))
	for i, p := range htmlProps {
		props[i] = p
	}

	// Maps attribute numbers to property indices
	anumMap := make(map[int]int)
	var css strings.Builder

	stripDotFromAuthorID := func(id string) string {
		return strings.ReplaceAll(id, ".", "_")
	}

	// Build author color styles if author colors are provided
	if authorColors != nil {
		css.WriteString("<style>\n")

		for num, attr := range padPool.NumToAttrib {
			if attr.Key == "author" && attr.Value != "" {
				propName := "author" + stripDotFromAuthorID(attr.Value)
				newIndex := len(props)
				props = append(props, propName)
				anumMap[num] = newIndex

				if color, ok := authorColors[attr.Value]; ok {
					css.WriteString("." + propName + " {background-color: " + color + "}\n")
				}
			} else if attr.Key == "removed" {
				propName := "removed"
				newIndex := len(props)
				props = append(props, propName)
				anumMap[num] = newIndex

				css.WriteString(".removed {text-decoration: line-through; " +
					"-ms-filter:'progid:DXImageTransform.Microsoft.Alpha(Opacity=80)'; " +
					"filter: alpha(opacity=80); " +
					"opacity: 0.8; " +
					"}\n")
			}
		}

		css.WriteString("</style>")
	}

	// Map properties to attribute numbers
	var trueVal = true
	for i, propName := range props {
		var attrib apool.Attribute
		switch p := propName.(type) {
		case string:
			attrib = apool.Attribute{Key: p, Value: "true"}
		case []string:
			if len(p) >= 2 {
				attrib = apool.Attribute{Key: p[0], Value: p[1]}
			}
		}

		propTrueNum := padPool.PutAttrib(attrib, &trueVal)
		if propTrueNum >= 0 {
			anumMap[propTrueNum] = i
		}
	}

	// getLineHTML converts a single line to HTML
	getLineHTML := func(text string, aline string) (string, error) {
		taker := changeset.NewStringIterator(text)
		var assem changeset.StringAssembler = changeset.NewStringAssembler()
		openTags := make([]int, 0)

		getSpanClassFor := func(i int) string {
			if authorColors == nil {
				return ""
			}

			if i >= len(props) {
				return ""
			}

			property := props[i]
			propStr, ok := property.(string)
			if !ok {
				return ""
			}

			if strings.HasPrefix(propStr, "author") {
				return stripDotFromAuthorID(propStr)
			}

			if propStr == "removed" {
				return "removed"
			}

			return ""
		}

		isSpanWithData := func(i int) bool {
			if i >= len(props) {
				return false
			}
			_, ok := props[i].([]string)
			return ok
		}

		emitOpenTag := func(i int) {
			openTags = append([]int{i}, openTags...)
			spanClass := getSpanClassFor(i)

			if spanClass != "" {
				assem.Append("<span class=\"")
				assem.Append(spanClass)
				assem.Append("\">")
			} else if i < len(tags) {
				assem.Append("<")
				assem.Append(tags[i])
				assem.Append(">")
			}
		}

		emitCloseTag := func(i int) {
			if len(openTags) > 0 {
				openTags = openTags[1:]
			}
			spanClass := getSpanClassFor(i)
			spanWithData := isSpanWithData(i)

			if spanClass != "" || spanWithData {
				assem.Append("</span>")
			} else if i < len(tags) {
				assem.Append("</")
				assem.Append(tags[i])
				assem.Append(">")
			}
		}

		// Find URLs in text
		urls := findURLs(text)

		idx := 0

		processNextChars := func(numChars int) error {
			if numChars <= 0 {
				return nil
			}

			optEnd := idx + numChars
			resultingOps, err := changeset.Subattribution(aline, idx, &optEnd)
			if err != nil {
				return err
			}

			ops, err := changeset.DeserializeOps(*resultingOps)
			if err != nil {
				return err
			}
			idx += numChars

			for _, op := range *ops {
				usedAttribs := make([]int, 0)

				// Mark all attribs as used
				attribNums, err := changeset.DecodeAttribString(op.Attribs)
				if err != nil {
					return err
				}

				for _, a := range attribNums {
					if propIdx, ok := anumMap[a]; ok {
						usedAttribs = append(usedAttribs, propIdx)
					}
				}

				// Find outermost tag that is no longer used
				outermostTag := -1
				for i := len(openTags) - 1; i >= 0; i-- {
					if !containsInt(usedAttribs, openTags[i]) {
						outermostTag = i
						break
					}
				}

				// Close all tags up to the outermost
				if outermostTag != -1 {
					for outermostTag >= 0 {
						emitCloseTag(openTags[0])
						outermostTag--
					}
				}

				// Open all tags that are used but not open
				for _, usedAttrib := range usedAttribs {
					if !containsInt(openTags, usedAttrib) {
						emitOpenTag(usedAttrib)
					}
				}

				chars := op.Chars
				if op.Lines > 0 {
					chars--
				}

				s, err := taker.Take(chars)
				if err != nil {
					return err
				}

				// Remove character with code 12 (form feed)
				cleanedStr := strings.ReplaceAll(*s, string(rune(12)), "")

				assem.Append(encodeWhitespace(html.EscapeString(cleanedStr)))
			}

			// Close all remaining open tags
			for len(openTags) > 0 {
				emitCloseTag(openTags[0])
			}

			return nil
		}

		// Process text with URLs
		if len(urls) > 0 {
			for _, urlData := range urls {
				startIndex := urlData.start
				url := urlData.url
				urlLength := len([]rune(url))

				if err := processNextChars(startIndex - idx); err != nil {
					return "", err
				}

				assem.Append("<a href=\"")
				assem.Append(escapeHTMLAttribute(url))
				assem.Append("\" rel=\"noreferrer noopener\">")

				if err := processNextChars(urlLength); err != nil {
					return "", err
				}

				assem.Append("</a>")
			}
		}

		if err := processNextChars(len([]rune(text)) - idx); err != nil {
			return "", err
		}

		return processSpaces(assem.String()), nil
	}

	var pieces []string
	pieces = append(pieces, css.String())

	var openLists []openList

	for i := 0; i < len(textLines); i++ {
		var aline string
		if i < len(attribLines) {
			aline = attribLines[i]
		}

		line, err := padLib.AnalyzeLine(textLines[i], aline, *padPool)
		if err != nil {
			return "", err
		}

		lineContent, err := getLineHTML(string(line.Text), line.Aline)
		if err != nil {
			return "", err
		}

		// If we are inside a list
		if line.ListLevel > 0 {
			hookContext := &events.LineHtmlForExportContext{
				Line:        line,
				LineContent: &lineContent,
				Apool:       padPool,
				AttribLine:  &attribLines[i],
				Text:        &textLines[i],
				PadId:       &padId,
			}

			var prevLine *padLib.LineModel
			var nextLine *padLib.LineModel

			if i > 0 && i-1 < len(attribLines) {
				prevLine, _ = padLib.AnalyzeLine(textLines[i-1], attribLines[i-1], *padPool)
			}
			if i+1 < len(textLines) {
				nextAline := ""
				if i+1 < len(attribLines) {
					nextAline = attribLines[i+1]
				}
				nextLine, _ = padLib.AnalyzeLine(textLines[i+1], nextAline, *padPool)
			}

			e.Hooks.ExecuteHooks("getLineHTMLForExport", hookContext)

			// To create list parent elements
			if prevLine == nil || prevLine.ListLevel != line.ListLevel || line.ListTypeName != prevLine.ListTypeName {
				exists := listExists(openLists, line.ListLevel, line.ListTypeName)
				if !exists {
					prevLevel := 0
					if prevLine != nil && prevLine.ListLevel > 0 {
						prevLevel = prevLine.ListLevel
					}
					if prevLine != nil && line.ListTypeName != prevLine.ListTypeName {
						prevLevel = 0
					}

					for diff := prevLevel; diff < line.ListLevel; diff++ {
						openLists = append(openLists, openList{level: diff, listType: line.ListTypeName})
						if len(pieces) > 0 {
							prevPiece := pieces[len(pieces)-1]
							if strings.HasPrefix(prevPiece, "<ul") ||
								strings.HasPrefix(prevPiece, "<ol") ||
								strings.HasPrefix(prevPiece, "</li>") {
								if nextLine == nil || !(nextLine.ListTypeName == "number" && string(nextLine.Text) == "") {
									pieces = append(pieces, "<li>")
								}
							}
						}

						if line.ListTypeName == "number" {
							// We introduce line.start here, this is useful for continuing
							// Ordered list line numbers
							if line.Start != "" {
								pieces = append(pieces, "<ol start=\""+line.Start+"\" class=\""+line.ListTypeName+"\">")
							} else {
								pieces = append(pieces, "<ol class=\""+line.ListTypeName+"\">")
							}
						} else {
							pieces = append(pieces, "<ul class=\""+line.ListTypeName+"\">")
						}
					}
				}
			}

			// if we're going up a level we shouldn't be adding..
			if hookContext.LineContent != nil && *hookContext.LineContent != "" {
				pieces = append(pieces, "<li>", *hookContext.LineContent)
			}

			// To close list elements
			if nextLine != nil &&
				nextLine.ListLevel == line.ListLevel &&
				line.ListTypeName == nextLine.ListTypeName {
				if hookContext.LineContent != nil && *hookContext.LineContent != "" {
					if nextLine.ListTypeName == "number" && string(nextLine.Text) == "" {
						// don't do anything because the next item is a nested ol opener so we need to
						// keep the li open
					} else {
						pieces = append(pieces, "</li>")
					}
				}
			}

			if nextLine == nil ||
				nextLine.ListLevel == 0 ||
				nextLine.ListLevel < line.ListLevel ||
				line.ListTypeName != nextLine.ListTypeName {
				nextLevel := 0
				if nextLine != nil && nextLine.ListLevel > 0 {
					nextLevel = nextLine.ListLevel
				}
				if nextLine != nil && line.ListTypeName != nextLine.ListTypeName {
					nextLevel = 0
				}

				for diff := nextLevel; diff < line.ListLevel; diff++ {
					openLists = filterList(openLists, diff, line.ListTypeName)

					if len(pieces) > 0 {
						lastPiece := pieces[len(pieces)-1]
						if strings.HasPrefix(lastPiece, "</ul") || strings.HasPrefix(lastPiece, "</ol") {
							pieces = append(pieces, "</li>")
						}
					}

					if line.ListTypeName == "number" {
						pieces = append(pieces, "</ol>")
					} else {
						pieces = append(pieces, "</ul>")
					}
				}
			}
		} else {
			// outside any list
			hookContext := &events.LineHtmlForExportContext{
				Line:        line,
				LineContent: &lineContent,
				Apool:       padPool,
				AttribLine:  &attribLines[i],
				Text:        &textLines[i],
				PadId:       &padId,
			}

			e.Hooks.ExecuteHooks("getLineHTMLForExport", hookContext)
			pieces = append(pieces, *hookContext.LineContent, "<br>")
		}
	}

	return strings.Join(pieces, ""), nil
}

// containsInt checks if a slice contains an integer
func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// listExists checks if a list with given level and type exists
func listExists(lists []openList, level int, listType string) bool {
	for _, l := range lists {
		if l.level == level && l.listType == listType {
			return true
		}
	}
	return false
}

// filterList removes lists matching level and type
func filterList(lists []openList, level int, listType string) []openList {
	result := make([]openList, 0)
	for _, l := range lists {
		if l.level != level || l.listType != listType {
			result = append(result, l)
		}
	}
	return result
}

type urlMatch struct {
	start int
	url   string
}

// findURLs finds all URLs in text and returns their positions
func findURLs(text string) []urlMatch {
	// Simple URL regex pattern
	urlRegex := regexp.MustCompile(`https?://[^\s<>"']+|www\.[^\s<>"']+`)
	matches := urlRegex.FindAllStringIndex(text, -1)

	var urls []urlMatch
	textRunes := []rune(text)

	for _, match := range matches {
		// Convert byte indices to rune indices
		startBytes := match[0]
		endBytes := match[1]

		startRunes := len([]rune(text[:startBytes]))
		url := text[startBytes:endBytes]

		// Clean up trailing punctuation
		url = strings.TrimRight(url, ".,;:!?")

		urls = append(urls, urlMatch{
			start: startRunes,
			url:   string(textRunes[startRunes : startRunes+len([]rune(url))]),
		})
	}

	return urls
}

// escapeHTMLContent escapes HTML content
func escapeHTMLContent(s string) string {
	return html.EscapeString(s)
}

// escapeHTMLAttribute escapes an HTML attribute value
func escapeHTMLAttribute(s string) string {
	s = html.EscapeString(s)
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// encodeWhitespace encodes whitespace for proper HTML display
func encodeWhitespace(s string) string {
	// Replace tabs with spaces
	s = strings.ReplaceAll(s, "\t", "    ")
	return s
}

// processSpaces handles space processing for HTML export
func processSpaces(s string) string {
	if !strings.Contains(s, "<") && !strings.Contains(s, " ") {
		return s
	}

	// Match HTML tags, spaces, or other content
	re := regexp.MustCompile(`<[^>]*>?| |[^ <]+`)
	parts := re.FindAllString(s, -1)

	if len(parts) == 0 {
		return s
	}

	// Process spaces: end of line and multiple spaces get &nbsp;
	endOfLine := true
	beforeSpace := false

	for i := len(parts) - 1; i >= 0; i-- {
		p := parts[i]
		if p == " " {
			if endOfLine || beforeSpace {
				parts[i] = "&nbsp;"
			}
			endOfLine = false
			beforeSpace = true
		} else if len(p) > 0 && p[0] != '<' {
			endOfLine = false
			beforeSpace = false
		}
	}

	// Beginning of line gets &nbsp;
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		if p == " " {
			parts[i] = "&nbsp;"
			break
		} else if len(p) > 0 && p[0] != '<' {
			break
		}
	}

	return strings.Join(parts, "")
}

// Export returns a rendered HTML string (for interface compatibility)
func (e *ExportHtml) Export(padId string, revNum *int) (string, error) {
	return e.GetPadHTMLDocument(padId, revNum, nil)
}
