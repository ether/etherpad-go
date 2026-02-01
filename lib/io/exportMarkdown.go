package io

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	pad2 "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/pad"
	"mvdan.cc/xurls/v2"
)

type ExportMarkdown struct {
	PadManager *pad.Manager
	Hooks      *hooks.Hook
}

func NewExportMarkdown(padManager *pad.Manager, hooksSystem *hooks.Hook) *ExportMarkdown {
	return &ExportMarkdown{
		PadManager: padManager,
		Hooks:      hooksSystem,
	}
}

// Mapping von Heading zu Markdown-Prefix
var headingToMarkdown = map[string]string{
	"h1": "# ",
	"h2": "## ",
	"h3": "### ",
	"h4": "#### ",
	"h5": "##### ",
	"h6": "###### ",
}

// getMarkdownFromAtext übersetzt einen Pad-Inhalt (AText) in Markdown
func (em *ExportMarkdown) getMarkdownFromAtext(retrievedPad *pad2.Pad, atext apool.AText, padId string) string {
	padPool := retrievedPad.Pool
	textLines := pad.SplitRemoveLastRune(atext.Text)
	attribLines, _ := changeset.SplitAttributionLines(atext.Attribs, atext.Text)
	tags := []string{"**", "*", "[]", "~~"}
	props := []string{"bold", "italic", "underline", "strikethrough"}
	anumMap := map[int]int{}
	thruthy := true
	for i, propName := range props {
		propTrueNum := padPool.PutAttrib(apool.Attribute{Key: propName, Value: "true"}, &thruthy)
		if propTrueNum >= 0 {
			anumMap[propTrueNum] = i
		}
	}

	var pieces []string
	var lists [][]interface{}

	for i := 0; i < len(textLines); i++ {
		var aline string
		if i < len(attribLines) {
			aline = attribLines[i]
		}

		line, _ := em.analyzeLine(&textLines[i], &aline, &padPool)

		// Call hook to allow plugins to modify the line (e.g., set heading)
		hookContext := &events.LineMarkdownForExportContext{
			Apool:      &padPool,
			AttribLine: &aline,
			Text:       &textLines[i],
			PadId:      &padId,
			Heading:    nil,
		}
		em.Hooks.ExecuteHooks("getLineMarkdownForExport", hookContext)

		// Determine heading prefix
		var headingPrefix string
		if hookContext.Heading != nil {
			if prefix, ok := headingToMarkdown[*hookContext.Heading]; ok {
				headingPrefix = prefix
			}
		}

		lineContent, err := em.getLineMarkdown(string(line.Text), line.Aline, tags, anumMap, headingPrefix)
		if err != nil {
			return ""
		}

		if line.ListLevel > 0 {
			whichList := len(lists)
			for j := len(lists) - 1; j >= 0; j-- {
				if line.ListLevel <= lists[j][0].(int) {
					whichList = j
				}
			}
			if whichList >= len(lists) {
				lists = append(lists, []interface{}{line.ListLevel, line.ListTypeName})
			}
			if line.ListTypeName == "number" {
				pieces = append(pieces, "\n"+spaces(line.ListLevel*4)+"1. "+lineContent)
			} else {
				pieces = append(pieces, "\n"+spaces(line.ListLevel*4)+"* "+lineContent)
			}
		} else if headingPrefix != "" {
			// Headings get extra newline before for better readability
			pieces = append(pieces, "\n"+lineContent+"\n")
		} else {
			pieces = append(pieces, "\n"+lineContent+"\n")
		}
	}
	return strings.Join(pieces, "")
}

var markdownListTypeRegex = regexp.MustCompile(`^([a-z]+)([12345678])`)

func (em *ExportMarkdown) analyzeLine(text, aline *string, attribPool *apool.APool) (*pad.LineModel, error) {
	var line pad.LineModel
	lineMarker := 0
	line.ListLevel = 0
	if aline != nil {
		opIter, err := changeset.DeserializeOps(*aline)
		if err != nil {
			return nil, err
		}
		for _, op := range *opIter {
			attribMap := changeset.FromString(op.Attribs, attribPool)

			// Check for list
			listTypeString := attribMap.Get("list")
			if listTypeString != nil {
				lineMarker = 1
				listType := markdownListTypeRegex.FindStringSubmatch(*listTypeString)
				if len(listType) >= 3 {
					line.ListTypeName = listType[1]
					line.ListLevel, err = strconv.Atoi(listType[2])
					if err != nil {
						println("Error in analyzing line", err.Error())
						return nil, err
					}
				}
			}

			// Check for heading (also has lineMarker)
			headingStr := attribMap.Get("heading")
			if headingStr != nil {
				lineMarker = 1
			}
		}
	}
	if lineMarker != 0 {
		line.Text = []rune((*text)[1:])
		subAttribLine, err := changeset.Subattribution(*aline, 1, nil)
		if err != nil {
			return nil, err
		}
		line.Aline = *subAttribLine
	} else {
		line.Text = []rune(*text)
		if aline != nil {
			line.Aline = *aline
		}
	}
	return &line, nil
}

type FindURLPair struct {
	StartIndex int
	Url        string
}

func findUrlsWithIndex(text string) []FindURLPair {
	rxStrict := xurls.Relaxed()
	matches := rxStrict.FindAllStringIndex(text, -1)
	var result []FindURLPair
	for _, match := range matches {
		start := match[0]
		end := match[1]
		url := text[start:end]
		result = append(result, FindURLPair{StartIndex: start, Url: url})
	}
	return result
}

func (em *ExportMarkdown) getLineMarkdown(text string, attribs string, tags []string, anumMap map[int]int, headingPrefix string) (string, error) {
	const ENTER = 1
	const STAY = 2
	const LEAVE = 0
	const TRUE = 3
	const FALSE = 4
	propVals := make([]int, len(tags))
	for i := range propVals {
		propVals[i] = FALSE
	}

	taker := changeset.NewStringIterator(text)
	assem := changeset.NewStringAssembler()

	openTags := []int{}
	emitOpenTag := func(i int) {
		openTags = append([]int{i}, openTags...)
		assem.Append(tags[i])
	}

	emitCloseTag := func(i int) {
		openTags = openTags[1:]
		assem.Append(tags[i])
	}

	orderdCloseTags := func(tags2close []int) {
		for i := 0; i < len(openTags); i++ {
			for j := 0; j < len(tags2close); j++ {
				if tags2close[j] == openTags[i] {
					emitCloseTag(tags2close[j])
					i--
					break
				}
			}
		}
	}

	// Add heading prefix if set
	if headingPrefix != "" {
		assem.Append(headingPrefix)
	}

	urls := findUrlsWithIndex(text)
	idx := 0

	processNextChars := func(numChars int) error {
		if numChars <= 0 {
			return nil
		}
		optEnd := idx + numChars
		subAttrib, err := changeset.Subattribution(attribs, idx, &optEnd)
		if err != nil {
			return err
		}
		iters, err := changeset.DeserializeOps(*subAttrib)
		if err != nil {
			return err
		}
		idx += numChars
		for _, op := range *iters {
			propChanged := false
			decodedAttribString, err := changeset.DecodeAttribString(op.Attribs)
			if err != nil {
				return err
			}
			for _, a := range decodedAttribString {
				if index, ok := anumMap[a]; ok {
					if propVals[index] == FALSE {
						propVals[index] = ENTER
						propChanged = true
					} else {
						propVals[index] = STAY
					}
				}
			}

			for i, val := range propVals {
				if val == TRUE {
					propVals[i] = LEAVE
					propChanged = true
				} else if val == STAY {
					propVals[i] = TRUE
				}
			}

			if propChanged {
				left := false
				for i, val := range propVals {
					if !left {
						if val == LEAVE {
							left = true
						}
					} else if val == TRUE {
						propVals[i] = STAY
					}
				}

				var tags2close = make([]int, 0)

				for i := len(propVals) - 1; i >= 0; i-- {
					if propVals[i] == LEAVE {
						tags2close = append(tags2close, i)
						propVals[i] = FALSE
					} else if propVals[i] == STAY {
						tags2close = append(tags2close, i)
					}
				}

				orderdCloseTags(tags2close)

				for i := 0; i < len(propVals); i++ {
					if propVals[i] == ENTER || propVals[i] == STAY {
						emitOpenTag(i)
						propVals[i] = TRUE
					}
				}
			}
			chars := op.Chars
			if op.Lines != 0 {
				chars-- // exclude newline at end of line, if present
			}
			s, err := taker.Take(chars)
			if err != nil {
				return err
			}
			replacedStr := strings.ReplaceAll(*s, string(rune(12)), "")
			s = &replacedStr

			assem.Append(*s)
		}

		tags2close := make([]int, 0)
		for i := len(propVals) - 1; i >= 0; i-- {
			if propVals[i] == TRUE {
				tags2close = append(tags2close, i)
				propVals[i] = FALSE
			}
		}

		orderdCloseTags(tags2close)
		return nil
	}

	for _, url := range urls {
		startIndex := url.StartIndex
		currentUrl := url.Url
		urlLength := len(currentUrl)
		if err := processNextChars(startIndex - idx); err != nil {
			return "", err
		}
		assem.Append(fmt.Sprintf("[%s](", currentUrl))
		if err := processNextChars(urlLength); err != nil {
			return "", err
		}
		assem.Append(")")
	}

	if err := processNextChars(len(text) - idx); err != nil {
		return "", err
	}

	assemStr := assem.String()
	assemStr = strings.ReplaceAll(assemStr, "&", "\\&")
	assemStr = strings.ReplaceAll(assemStr, "_", "\\_")
	return assemStr, nil
}

func (em *ExportMarkdown) GetPadMarkdownDocument(padID string, revNum *int) (*string, error) {
	retrievedPad, err := em.PadManager.GetPad(padID, nil, nil)
	if err != nil {
		return nil, err
	}
	markdown, err := em.getPadMarkdown(*retrievedPad, revNum, padID)
	if err != nil {
		return nil, err
	}
	return &markdown, nil
}

func (em *ExportMarkdown) getPadMarkdown(pad pad2.Pad, revNum *int, padId string) (string, error) {
	var atext apool.AText
	if revNum != nil {
		retrievedAtext := pad.GetInternalRevisionAText(*revNum)
		if retrievedAtext != nil {
			atext = *retrievedAtext
		}
	} else {
		atext = pad.AText
	}
	markdown := em.getMarkdownFromAtext(&pad, atext, padId)
	return markdown, nil
}

func spaces(n int) string {
	return strings.Repeat(" ", n)
}
