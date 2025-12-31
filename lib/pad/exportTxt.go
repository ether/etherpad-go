package pad

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/utils"
)

func SplitRemoveLastRune(s string) []string {
	if s == "" {
		return []string{""}
	}
	r := []rune(s)
	if len(r) == 0 {
		return []string{""}
	}
	trimmed := string(r[:len(r)-1])
	return strings.Split(trimmed, "\n")
}

type LineModel struct {
	ListLevel    int
	Text         []rune
	Aline        string
	ListTypeName string
	Start        string
}

func parseListType(listType string) (tag string, num int, ok bool) {
	re := regexp.MustCompile(`^([a-z]+)([0-9]+)`)
	m := re.FindStringSubmatch(listType)
	if m == nil {
		return "", 0, false
	}
	n, _ := strconv.Atoi(m[2])
	return m[1], n, true
}

func analyzeLine(text string, aline string, attrpool apool.APool) (*LineModel, error) {
	line := &LineModel{}

	lineMarker := 0
	line.ListLevel = 0
	if aline != "" {
		ops, err := changeset.DeserializeOps(aline)
		if err != nil {
			return nil, err
		}
		if len(*ops) > 0 {
			op := (*ops)[0]
			attribs := changeset.FromString(op.Attribs, &attrpool)

			listTypeStr := attribs.Get("list")
			if listTypeStr != nil {
				lineMarker = 1
				if tag, n, ok := parseListType(*listTypeStr); ok {
					line.ListTypeName = tag
					line.ListLevel = n
				}
			}

			start := attribs.Get("start")
			if start != nil {
				line.Start = *start
			}
		}
	}

	if lineMarker == 1 {
		runedText := []rune(text)
		if len(runedText) > 0 {
			line.Text = runedText[1:]
		} else {
			line.Text = []rune{}
		}
		lineAline, err := changeset.Subattribution(aline, 1, nil)
		if err != nil {
			return nil, err
		}
		line.Aline = *lineAline
	} else {
		line.Text = []rune(text)
		line.Aline = aline
	}

	return line, nil
}

func GetTxtFromAText(retrievedPad *pad.Pad, atext apool.AText) (*string, error) {
	padPool := retrievedPad.Pool
	textLines := SplitRemoveLastRune(atext.Text)
	attribLines, err := changeset.SplitAttributionLines(atext.Attribs, atext.Text)
	if err != nil {
		return nil, err
	}

	props := []string{"heading1", "heading2", "bold", "italic", "underline", "strikethrough"}
	css := ""
	anumMap := make(map[int]int)
	var thruthy = true
	for index, prop := range props {
		propTrueNum := padPool.PutAttrib(apool.Attribute{
			Key:   prop,
			Value: "true",
		}, &thruthy)
		if propTrueNum >= 0 {
			anumMap[propTrueNum] = index
		}
	}

	getLineTxt := func(text string, attribs string) (*string, error) {
		const ENTER = 1
		const STAY = 2
		const LEAVE = 0
		const TRUE = 3
		const FALSE = 4
		propVals := make([]int, len(props))
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

		idx := 0

		processNextChars := func(numChars int) error {
			if numChars <= 0 {
				// Skip processing
				return nil
			}
			optEnd := idx + numChars
			resultingOps, err := changeset.Subattribution(attribs, idx, &optEnd)
			if err != nil {
				return err
			}
			ops, err := changeset.DeserializeOps(*resultingOps)
			if err != nil {
				return err
			}
			idx += numChars

			for _, op := range *ops {
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

					tags2close := make([]int, 0)

					for i := len(propVals) - 1; i >= 0; i-- {
						if propVals[i] == LEAVE {
							tags2close = append(tags2close, i)
							propVals[i] = FALSE
						} else if propVals[i] == STAY {
							tags2close = append(tags2close, i)
						}
					}

					for index, val := range propVals {
						if val == ENTER || val == STAY {
							propVals[index] = TRUE
						}
					}
				}

				chars := op.Chars
				if op.Lines > 0 {
					chars-- // don't include linebreak char
				}
				s, err := taker.Take(chars)
				if err != nil {
					return err
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
			return nil
		}

		if err := processNextChars(utils.RuneCount(text) - idx); err != nil {
			return nil, err
		}

		result := assem.String()
		return &result, nil
	}

	pieces := []string{css}

	listNumbers := make(map[int]int)
	prevListLevel := 0

	for i, lineText := range textLines {
		line, err := analyzeLine(lineText, attribLines[i], padPool)
		if err != nil {
			return nil, err
		}
		lineContent, err := getLineTxt(string(line.Text), line.Aline)
		if err != nil {
			return nil, err
		}

		if line.ListTypeName == "bullet" {
			var newLineContent = "* " + *lineContent
			lineContent = &newLineContent
		}

		if line.ListTypeName != "number" {
			// We're no longer in an OL so we can reset counting
			for key, _ := range listNumbers {
				delete(listNumbers, key)
			}
		}

		if line.ListLevel > 0 {
			for j := line.ListLevel - 1; j >= 0; j-- {
				pieces = append(pieces, "\t")
			}

			if line.ListTypeName == "number" {
				if line.ListLevel < prevListLevel {
					delete(listNumbers, prevListLevel)
				}

				listNumbers[line.ListLevel] += 1
				if line.ListLevel > 1 {
					for x := 1; x <= line.ListLevel-1; x++ {
						if _, ok := listNumbers[x]; !ok {
							listNumbers[x] = 0
						}
						pieces = append(pieces, fmt.Sprintf("%d.", listNumbers[x]))
					}
				}

				pieces = append(pieces, fmt.Sprintf("%d. ", listNumbers[line.ListLevel]))
				prevListLevel = line.ListLevel
			}

			pieces = append(pieces, *lineContent)
			pieces = append(pieces, "\n")
		} else {
			pieces = append(pieces, *lineContent)
			pieces = append(pieces, "\n")
		}
	}

	joinedStr := strings.Join(pieces, "")
	return &joinedStr, nil
}
