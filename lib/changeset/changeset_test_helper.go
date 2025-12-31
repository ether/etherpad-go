package changeset

import (
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/test/testutils/general"
)

func RandomTestChangeset(origText string, withAttribs bool) (string, string) {
	var charBank = NewStringAssembler()
	textLeft := origText
	var outTextAssem = NewStringAssembler()
	opAssem := NewSmartOpAssembler()
	oldLen := len(origText)

	nextOp := NewOp(nil)

	appendMultilineOp := func(opcode string, txt string) {
		nextOp.OpCode = opcode
		if withAttribs {
			nextOp.Attribs = randomTwoPropAttribs(opcode)
		}

		re := regexp.MustCompile(`\n|[^\n]+`)
		matches := re.FindAllString(txt, -1)
		for _, t := range matches {
			if t == "\n" {
				nextOp.Chars = 1
				nextOp.Lines = 1
				opAssem.Append(nextOp)
			} else {
				nextOp.Chars = len(t)
				nextOp.Lines = 0
				opAssem.Append(nextOp)
			}
		}
	}

	doOp := func() {
		o := randomStringOperation(len(textLeft))
		if o.insert != "" {
			txt := o.insert
			charBank.Append(txt)
			outTextAssem.Append(txt)
			appendMultilineOp("+", txt)
		} else if o.skip > 0 {
			txt := textLeft[:o.skip]
			textLeft = textLeft[o.skip:]
			outTextAssem.Append(txt)
			appendMultilineOp("=", txt)
		} else if o.remove > 0 {
			txt := textLeft[:o.remove]
			textLeft = textLeft[o.remove:]
			appendMultilineOp("-", txt)
		}
	}

	for len(textLeft) > 1 {
		doOp()
	}
	for i := 0; i < 5; i++ {
		doOp()
	}

	outText := outTextAssem.String() + "\n"
	opAssem.EndDocument()
	cs := Pack(oldLen, len(outText), opAssem.String(), charBank.String())
	_, err := CheckRep(cs)
	if err != nil {
		println("Original Text:", origText)
		println("Generated Changeset:", cs)
		panic("Generated invalid changeset: " + err.Error())
	}
	return cs, outText
}

// stringOperation represents a random string operation.
type stringOperation struct {
	insert string
	skip   int
	remove int
}

func randomStringOperation(numCharsLeft int) stringOperation {
	var result stringOperation

	switch rand.Intn(11) {
	case 0:
		// insert char
		result = stringOperation{
			insert: general.RandomInlineString(1),
		}
	case 1:
		// delete char
		result = stringOperation{
			remove: 1,
		}
	case 2:
		// skip char
		result = stringOperation{
			skip: 1,
		}
	case 3:
		// insert small
		result = stringOperation{
			insert: general.RandomInlineString(rand.Intn(4) + 1),
		}
	case 4:
		// delete small
		result = stringOperation{
			remove: rand.Intn(4) + 1,
		}
	case 5:
		// skip small
		result = stringOperation{
			skip: rand.Intn(4) + 1,
		}
	case 6:
		// insert multiline
		result = stringOperation{
			insert: general.RandomMultiline(5, 20),
		}
	case 7:
		// delete multiline
		result = stringOperation{
			remove: int(float64(numCharsLeft) * rand.Float64() * rand.Float64()),
		}
	case 8:
		// skip multiline
		result = stringOperation{
			skip: int(float64(numCharsLeft) * rand.Float64() * rand.Float64()),
		}
	case 9:
		// delete to end
		result = stringOperation{
			remove: numCharsLeft,
		}
	case 10:
		// skip to end
		result = stringOperation{
			skip: numCharsLeft,
		}
	}

	maxOrig := numCharsLeft - 1
	if result.remove > 0 {
		result.remove = minFunc(result.remove, maxOrig)
	} else if result.skip > 0 {
		result.skip = minFunc(result.skip, maxOrig)
	}

	return result
}

// minFunc returns the smaller of two integers.
func minFunc(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// randomTwoPropAttribs generates random attributes for testing.
// Uses numeric pool references (0-3) which correspond to the test pool attributes.
func randomTwoPropAttribs(opcode string) string {
	if opcode == "-" {
		return ""
	}

	// Use numeric pool indices (0-3 are typical test pool entries)
	// Create a shuffled list of available indices to avoid duplicates
	indices := []int{0, 1, 2, 3}
	rand.Shuffle(len(indices), func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})

	numProps := rand.Intn(3) // 0, 1, or 2 attributes

	// Select and sort the attributes to ensure canonical order
	selectedIndices := indices[:numProps]
	sort.Ints(selectedIndices)

	var attribs strings.Builder
	for _, idx := range selectedIndices {
		attribs.WriteString("*")
		attribs.WriteString(strconv.Itoa(idx))
	}

	return attribs.String()
}
