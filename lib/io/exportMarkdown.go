package io

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	pad2 "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/pad"
	"mvdan.cc/xurls/v2"
)

type ExportMarkdown struct {
	PadManager *pad.Manager
}

func NewExportMarkdown(padManager *pad.Manager) *ExportMarkdown {
	return &ExportMarkdown{
		PadManager: padManager,
	}
}

// getMarkdownFromAtext übersetzt einen Pad-Inhalt (AText) in Markdown
func (em *ExportMarkdown) getMarkdownFromAtext(retrievedPad *pad2.Pad, atext apool.AText) string {
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

	headingtags := []string{"# ", "## ", "### ", "#### ", "##### ", "###### ", "    "}
	headingprops := [][]string{
		{"heading", "h1"},
		{"heading", "h2"},
		{"heading", "h3"},
		{"heading", "h4"},
		{"heading", "h5"},
		{"heading", "h6"},
		{"heading", "code"},
	}
	headinganumMap := map[int]int{}
	for i, prop := range headingprops {
		propTrueNum := padPool.PutAttrib(apool.Attribute{Key: prop[0], Value: prop[1]}, &thruthy)
		if propTrueNum >= 0 {
			headinganumMap[propTrueNum] = i
		}
	}

	var pieces []string
	var lists [][]interface{}

	for i := 0; i < len(textLines); i++ {
		line, _ := em.analyzeLine(&textLines[i], &attribLines[i], &padPool)
		lineContent, err := em.getLineMarkdown(string(line.Text), line.Aline, tags, anumMap, headinganumMap, headingtags)
		if err != nil {
			return ""
		}
		if line.ListLevel > 0 {
			// TODO: continue here implementing nested lists properly
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
			listTypeString := attribMap.Get("list")
			if listTypeString != nil {
				lineMarker = 1
				listType := markdownListTypeRegex.FindStringSubmatch(*listTypeString)
				if len(listType) == 2 {
					line.ListTypeName = listType[1]
					line.ListLevel, err = strconv.Atoi(listType[2])
					if err != nil {
						println("Error in analyzing line", err.Error())
						return nil, err
					}
				}
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

func (em *ExportMarkdown) getLineMarkdown(text string, attribs string, tags []string, anumMap, headinganumMap map[int]int, headingtags []string) (string, error) {
	const ENTER = 1
	const STAY = 2
	const LEAVE = 0
	const TRUE = 3
	const FALSE = 4
	propVals := make([]int, len(headingtags))
	for i := range propVals {
		propVals[i] = FALSE
	}

	// Use order of tags (b/i/u) as order of nesting, for simplicity
	// and decent nesting.  For example,
	// <b>Just bold<b> <b><i>Bold and italics</i></b> <i>Just italics</i>
	// becomes
	// <b>Just bold <i>Bold and italics</i></b> <i>Just italics</i>
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

	var heading *string
	var deletedAsterisk = false
	optEnd := 1
	subAttribs, err := changeset.Subattribution(attribs, 0, &optEnd)
	if err != nil {
		return "", err
	}
	iter2, err := changeset.DeserializeOps(*subAttribs)
	if err != nil {
		return "", err
	}

	for _, it := range *iter2 {
		a, err := changeset.DecodeAttribString(it.Attribs)
		if err != nil {
			return "", err
		}
		for _, attr := range a {
			i := headinganumMap[attr]
			headingFound := headingtags[i]
			heading = &headingFound
		}
	}

	if heading != nil {
		assem.Append(*heading)
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

			if heading != nil && !deletedAsterisk {
				replacedFirstAsterik := (*s)[1:]
				s = &replacedFirstAsterik
				deletedAsterisk = true
			}
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
	return assem.String(), nil
}

func (em *ExportMarkdown) GetPadMarkdownDocument(padID string, revNum *int) (*string, error) {
	retrievedPad, err := em.PadManager.GetPad(padID, nil, nil)
	if err != nil {
		return nil, err
	}
	markdown, err := em.getPadMarkdown(*retrievedPad, revNum)
	if err != nil {
		return nil, err
	}
	return &markdown, nil
}

func (em *ExportMarkdown) getPadMarkdown(pad pad2.Pad, revNum *int) (string, error) {
	var atext apool.AText
	if revNum != nil {
		retrievedAtext := pad.GetInternalRevisionAText(*revNum)
		if retrievedAtext != nil {
			atext = *retrievedAtext
		}
	} else {
		atext = pad.AText
	}
	markdown := em.getMarkdownFromAtext(&pad, atext)
	return markdown, nil
}

func spaces(n int) string {
	return strings.Repeat(" ", n)
}
