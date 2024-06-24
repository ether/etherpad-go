package test

import (
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/utils"
	"math/rand/v2"
)

func randomInlineString(length int) string {
	var txt = changeset.NewStringAssembler()
	for i := 0; i < length; i++ {
		txt.Append(utils.NumToString(rand.IntN(26) + 97))
	}
	return txt.String()
}

func RandomMultiline(approxMaxLines int, approxMaxCols int) string {
	var numParts = rand.IntN(approxMaxLines+2) + 1
	var txt = changeset.NewStringAssembler()
	var coinFlip = rand.IntN(2)

	if coinFlip == 0 {
		txt.Append("\n")
	} else {
		txt.Append("")
	}

	for i := 0; i < numParts; i++ {
		if i%2 == 0 {
			if rand.IntN(10) > 0 {
				txt.Append(randomInlineString(rand.IntN(approxMaxCols) + 1))
			} else {
				txt.Append("\n")
			}
		} else {
			txt.Append("\n")
		}
	}
	return txt.String()
}
