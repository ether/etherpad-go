package exporter

import (
	"errors"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	pad2 "github.com/ether/etherpad-go/lib/models/pad"
	"strings"
)

func getPadTXT(pad pad2.Pad, revNum *int) (*string, error) {
	var atext = pad.AText

	if revNum != nil {
		atextRef := pad.GetInternalRevisionAText(*revNum)
		if atextRef == nil {
			return nil, errors.New("revision number is higher than head")
		}
		atext = *atextRef
	}
	return getTXTFromAText(pad, atext)
}

func getTXTFromAText(pad pad2.Pad, atext apool.AText) (*string, error) {
	var apool = pad.Pool
	var textlines = strings.Split(atext.Text, "\n")
	var attribLines, err = changeset.SplitAttributionLines(atext.Attribs, atext.Text)

	if err != nil {
		return nil, err
	}

	var props = []string{"heading1", "heading2", "bold", "italic", "underline", "strikethrough"}
	var anumMap = make(map[int]int)
	for i, prop := range props {
		var trueVar = true
		var propTrueNum = apool.PutAttrib(apool.Attribute{Key: prop, Value: "true"}, &trueVar)
		if propTrueNum >= 0 {
			anumMap[propTrueNum] = i
		}
	}

	getLineTXT := func(text string, attribs string) {
		propVals := []bool{false, false, false}
		const ENTER = 1
		const STAY = 2
		const LEAVE = 0

		var taker = changeset.NewStringIterator(text)
		var assem = changeset.NewStringAssembler()

		var idx = 0

		processNextChars := func(numChars int) {
			if numChars <= 0 {
				return
			}

			var ops = changeset.DeserializeOps()

		}

	}

}
