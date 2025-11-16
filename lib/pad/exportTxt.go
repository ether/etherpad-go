package pad

import (
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/models/pad"
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

func GetTxtFromAText(retrievedPad *pad.Pad, atext apool.AText) error {
	padPool := retrievedPad.Pool
	textLines := SplitRemoveLastRune(atext.Text)
	attribLines, err := changeset.SplitAttributionLines(atext.Attribs, atext.Text)
	if err != nil {
		return err
	}

	props := []string{"heading1", "heading2", "bold", "italic", "underline", "strikethrough"}
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

	getLineTxt := func(text string, attribs string) string {
		propVals := []bool{false, false, false}
		const ENTER = 1
		const STAY = 2
		const LEAVE = 0

		// Use order of tags (b/i/u) as order of nesting, for simplicity
		// and decent nesting.  For example,
		// <b>Just bold<b> <b><i>Bold and italics</i></b> <i>Just italics</i>
		// becomes
		// <b>Just bold <i>Bold and italics</i></b> <i>Just italics</i>
		taker := changeset.NewStringIterator(text)
		assem := changeset.NewStringAssembler()

		idx := 0

		processNextChars := func(numChars int) {
			if numChars <= 0 {
				return
			}

			ops, err := changeset.DeserializeOps(changeset.Subattribution)
		}

	}
}
