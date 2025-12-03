package changeset

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/utils"
)

type Changeset struct {
	OldLen   int
	NewLen   int
	Ops      string
	CharBank string
}

func OpsFromAText(atext apool.AText) *[]Op {
	var lastOp *Op = nil
	var attribs, _ = DeserializeOps(atext.Attribs)
	var opsToReturn = make([]Op, 0)

	for _, op := range *attribs {
		if lastOp != nil {
			opsToReturn = append(opsToReturn, *lastOp)
		}
		lastOp = &op
	}

	if lastOp == nil {
		return nil
	}
	// exclude final newline

	if lastOp.Lines <= 1 {
		lastOp.Lines = 0
		lastOp.Chars--
	} else {
		lastNewlineIndex := utils.RuneLastIndex(atext.Text, "\n")
		nextToLastNewlineEnd := utils.RuneLastIndex(utils.RuneSlice(atext.Text, 0, lastNewlineIndex), "\n") + 1
		lastLineLength := utf8.RuneCountInString(atext.Text) - nextToLastNewlineEnd - 1
		lastOp.Lines--
		lastOp.Chars -= lastLineLength + 1
		opsToReturn = append(opsToReturn, *copyOp(*lastOp, nil))
		lastOp.Lines = 0
		lastOp.Chars = lastLineLength
	}
	if lastOp.Chars != 0 {
		opsToReturn = append(opsToReturn, *lastOp)
	}
	return &opsToReturn
}

func OpsFromText(opcode string, text string, attribs interface{}, pool *apool.APool) []Op {
	var opsToReturn = make([]Op, 0)
	var op = NewOp(&opcode)

	if attribs == nil || reflect.ValueOf(attribs).Kind() == reflect.Ptr {
		attribs = []apool.Attribute{}
	}

	switch v := attribs.(type) {
	case string:
		op.Attribs = attribs.(string)
	case []apool.Attribute:
		var emptyValueIsDelete = opcode == "+"
		var attribMap = NewAttributeMap(pool)
		op.Attribs = attribMap.Update(attribs.([]apool.Attribute), &emptyValueIsDelete).String()
	default:
		fmt.Printf("Unknown argument type: %T\n", v)
	}
	var lastNewLinePos = utils.RuneLastIndex(text, "\n")
	if lastNewLinePos < 0 {
		op.Chars = utf8.RuneCountInString(text)
		op.Lines = 0
		opsToReturn = append(opsToReturn, op)
	} else {
		op.Chars = lastNewLinePos + 1
		op.Lines = utils.CountLines(text, '\n')
		opsToReturn = append(opsToReturn, op)
		var op2 = copyOp(op, nil)
		op2.Chars = utf8.RuneCountInString(text) - (lastNewLinePos + 1)
		op2.Lines = 0
		opsToReturn = append(opsToReturn, *op2)
	}
	return opsToReturn
}

func Pack(oldLen int, newLen int, opStr string, bank string) string {
	var lenDiff = newLen - oldLen
	var lenDiffStr = ""
	if lenDiff >= 0 {
		lenDiffStr = ">" + utils.NumToString(lenDiff)
	} else {
		lenDiffStr = "<" + utils.NumToString(-lenDiff)
	}
	var a = make([]string, 0)
	a = append(a, "Z:", utils.NumToString(oldLen), lenDiffStr, opStr, "$", bank)
	return strings.Join(a, "")
}

func MakeSplice(orig string, start int, ndel int, ins string, attribs *string, pool *apool.APool) (string, error) {
	if start < 0 {
		return "", errors.New("start is negative")
	}

	if ndel < 0 {
		return "", errors.New("ndel is negative")
	}

	if start > utf8.RuneCountInString(orig) {
		start = utf8.RuneCountInString(orig)
	}

	if ndel > utf8.RuneCountInString(orig)-start {
		ndel = utf8.RuneCountInString(orig) - start
	}

	deleted := utils.RuneSlice(orig, start, start+ndel)
	var assem = NewSmartOpAssembler()
	var opsGenerated = make([]Op, 0)

	var equalOps = OpsFromText("=", utils.RuneSlice(orig, 0, start), "", nil)
	var deletedOps = OpsFromText("-", deleted, "", nil)
	var insertedOps = OpsFromText("+", ins, attribs, pool)

	opsGenerated = append(opsGenerated, equalOps...)
	opsGenerated = append(opsGenerated, deletedOps...)
	opsGenerated = append(opsGenerated, insertedOps...)
	for _, op := range opsGenerated {
		assem.Append(op)
	}
	assem.EndDocument()
	return Pack(utf8.RuneCountInString(orig), utf8.RuneCountInString(orig)+utf8.RuneCountInString(ins)-ndel, assem.String(), ins), nil
}

func Identity(n int) string {
	return Pack(n, n, "", "")
}

func Unpack(cs string) (*Changeset, error) {
	var headerRegex, _ = regexp.Compile("Z:([0-9a-z]+)([><])([0-9a-z]+)|")
	var foundHeaders = headerRegex.FindStringSubmatch(cs)

	if len(foundHeaders) == 0 {
		return nil, errors.New("no valid header found")
	}

	var oldLen, _ = utils.ParseNum(foundHeaders[1])
	var changeSign int
	if foundHeaders[2] == ">" {
		changeSign = 1
	} else {
		changeSign = -1
	}

	var changeMag, _ = utils.ParseNum(foundHeaders[3])
	var newLen = oldLen + changeSign*changeMag
	var opsStart = utf8.RuneCountInString(foundHeaders[0])
	var opsEnd = utils.RuneIndex(cs, "$")
	if opsEnd < 0 {
		opsEnd = utf8.RuneCountInString(cs)
	}

	return &Changeset{
		oldLen,
		newLen,
		utils.RuneSlice(cs, opsStart, opsEnd),
		utils.RuneSlice(cs, opsEnd+1, utf8.RuneCountInString(cs)),
	}, nil

}

func AttributeTester(attribPair apool.Attribute, pool *apool.APool) func(arg *string) bool {
	var never = func(arg *string) bool {
		return false
	}

	if pool == nil {
		return never
	}

	var trueVal = true
	var attribNum = pool.PutAttrib(attribPair, &trueVal)
	if attribNum < 0 {
		return never
	}

	var tokenStr = "*" + utils.NumToString(attribNum)
	return func(attribs *string) bool {
		idx := strings.Index(*attribs, tokenStr)
		if idx == -1 {
			return false
		}
		endPos := idx + len(tokenStr)
		if endPos >= len(*attribs) {
			return true
		}
		nextChar := rune((*attribs)[endPos])
		return !unicode.IsLetter(nextChar) && !unicode.IsDigit(nextChar) && nextChar != '_'
	}
}

func replaceAttributes(att2 string, callback func(match string) string) (string, map[string]string, error) {
	re := regexp.MustCompile(`\*([0-9a-z]+)`)
	atts := make(map[string]string)

	result := re.ReplaceAllStringFunc(att2, callback)

	return result, atts, nil
}

func followAttributes(att1 string, att2 string, pool *apool.APool) (*string, error) {
	// The merge of two sets of attribute changes to the same text
	// takes the lexically-earlier value if there are two values
	// for the same key.  Otherwise, all key/value changes from
	// both attribute sets are taken.  This operation is the "follow",
	// so a set of changes is produced that can be applied to att1
	// to produce the merged set.
	if att2 == "" && pool == nil {
		return nil, nil
	}

	if att1 == "" {
		return &att2, nil
	}

	var atts = make(map[string]string)
	if _, _, err := replaceAttributes(att2, func(a string) string {
		parsedNum, _ := utils.ParseNum(a)
		var attrib, _ = pool.GetAttrib(parsedNum)
		atts[attrib.Key] = attrib.Value
		return ""
	}); err != nil {
		return nil, err
	}
	if _, _, err := replaceAttributes(att1, func(a string) string {
		parsedNum, _ := utils.ParseNum(a)
		var attrib, _ = pool.GetAttrib(parsedNum)
		var res, ok = atts[attrib.Key]

		if ok && attrib.Value <= res {
			delete(atts, attrib.Key)
		}
		return ""
	}); err != nil {
		return nil, err
	}

	var buf = NewStringAssembler()
	for key, value := range atts {
		buf.Append("*")
		buf.Append(utils.NumToString(pool.PutAttrib(apool.Attribute{
			Key:   key,
			Value: value,
		}, nil)))
	}

	buffStr := buf.String()
	return &buffStr, nil
}

func Follow(c string, rebasedChangeset string, reverseInsertOrder bool, pool *apool.APool) (*string, error) {
	unpacked1, err := Unpack(c)
	if err != nil {
		return nil, err
	}
	unpacked2, err := Unpack(rebasedChangeset)
	if err != nil {
		return nil, err
	}

	var len1 = unpacked1.OldLen
	var len2 = unpacked2.OldLen
	if len1 != len2 {
		return nil, errors.New("mismatched lengths in Follow")
	}

	var chars1 = NewStringIterator(unpacked1.CharBank)
	var chars2 = NewStringIterator(unpacked2.CharBank)

	var oldLen = unpacked1.NewLen
	var oldPos = 0
	var newLen = 0

	hasInsertFirst := func(attrib string) bool {
		return AttributeTester(apool.Attribute{
			Key:   "insertorder",
			Value: "first",
		}, pool)(&attrib)
	}

	newOps, err := ApplyZip(unpacked1.Ops, unpacked2.Ops, func(op1, op2 *Op) (*Op, error) {
		var opOut = NewOp(nil)
		if op1.OpCode == "+" || op2.OpCode == "+" {
			var whichToDo int
			if op2.OpCode != "+" {
				whichToDo = 1
			} else if op1.OpCode != "+" {
				whichToDo = 2
			} else {
				firstChar1, err := chars1.Peek(1)
				if err != nil {
					return nil, err
				}
				firstChar2, err := chars2.Peek(1)
				if err != nil {
					return nil, err
				}

				var insertFirst1 = hasInsertFirst(op1.Attribs)
				var insertFirst2 = hasInsertFirst(op2.Attribs)

				if insertFirst1 && !insertFirst2 {
					whichToDo = 1
				} else if insertFirst2 && !insertFirst1 {
					whichToDo = 2
				} else if *firstChar1 == "\n" && *firstChar2 != "\n" {
					whichToDo = 2
				} else if *firstChar1 != "\n" && *firstChar2 == "\n" {
					whichToDo = 1
				} else if reverseInsertOrder {
					// break symmetry:
					whichToDo = 2
				} else {
					whichToDo = 1
				}
			}

			if whichToDo == 1 {
				err := chars1.Skip(op1.Chars)
				if err != nil {
					panic(err)
				}
				opOut.OpCode = "="
				opOut.Lines = op1.Lines
				opOut.Chars = op1.Chars
				opOut.Attribs = ""
				op1.OpCode = ""
			} else {
				// whichToDo == 2
				if err := chars2.Skip(op2.Chars); err != nil {
					return nil, err
				}
				copyOp(*op2, &opOut)
				op2.OpCode = ""
			}
		} else if op1.OpCode == "-" {
			if op2.OpCode == "" {
				op1.OpCode = ""
			} else if op1.Chars <= op2.Chars {
				op2.Chars -= op1.Chars
				op2.Lines -= op1.Lines
				op1.OpCode = ""

				if op2.Chars == 0 {
					op2.OpCode = ""
				}
			} else {
				op1.Chars -= op2.Chars
				op1.Lines -= op2.Lines
				op2.OpCode = ""
			}
		} else if op2.OpCode == "-" {
			copyOp(*op2, &opOut)

			if op1.OpCode == "" {
				op2.OpCode = ""
			} else if op2.Chars <= op1.Chars {
				// delete part or all of a keep
				op1.Chars -= op2.Chars
				op1.Lines -= op2.Lines
				op2.OpCode = ""
				if op1.Chars == 0 {
					op1.OpCode = ""
				}
			} else {
				// delete all of a keep, and keep going
				opOut.Lines = op1.Lines
				opOut.Chars = op1.Chars
				op2.Lines -= op1.Lines
				op2.Chars -= op1.Chars
				op1.OpCode = ""
			}
		} else if op1.OpCode == "" {
			copyOp(*op2, &opOut)
			op2.OpCode = ""
		} else if op2.OpCode == "" {
			// @NOTE: Critical bugfix for EPL issue #1625. We do not copy op1 here
			// in order to prevent attributes from leaking into result changesets.
			// copyOp(op1, opOut);
			op1.OpCode = ""
		} else {
			// both keeps
			opOut.OpCode = "="
			opOutAttrib, err := followAttributes(op1.Attribs, op2.Attribs, pool)
			if err != nil {
				return nil, err
			}

			if opOutAttrib != nil {
				opOut.Attribs = *opOutAttrib
			}

			if op1.Chars <= op2.Chars {
				opOut.Chars = op1.Chars
				opOut.Lines = op1.Lines
				op2.Chars -= op1.Chars
				op2.Lines -= op1.Lines
				op1.OpCode = ""
				if op2.Chars == 0 {
					op2.OpCode = ""
				}
			} else {
				opOut.Chars = op2.Chars
				opOut.Lines = op2.Lines
				op1.Chars -= op2.Chars
				op1.Lines -= op2.Lines
				op2.OpCode = ""
			}
		}

		switch {
		case opOut.OpCode == "=":
			{
				oldPos += opOut.Chars
				newLen += opOut.Chars
				break
			}
		case opOut.OpCode == "-":
			{
				oldPos += opOut.Chars
				break
			}
		case opOut.OpCode == "+":
			{
				newLen += opOut.Chars
				break
			}
		}
		return &opOut, nil
	})
	if err != nil {
		return nil, err
	}

	newLen += oldLen - oldPos
	packedFollow := Pack(oldLen, newLen, *newOps, unpacked2.CharBank)
	return &packedFollow, nil
}

func OldLen(cs string) int {
	var unpacked, _ = Unpack(cs)
	return unpacked.OldLen
}

func CheckRep(cs string) (*string, error) {
	var unpacked, err = Unpack(cs)

	if err != nil {
		return nil, err
	}

	var oldLen = unpacked.OldLen
	var newLen = unpacked.NewLen
	var ops = unpacked.Ops
	var charBank = unpacked.CharBank

	var assem = NewSmartOpAssembler()
	var oldPos = 0
	var calcNewLen = 0

	extractedOps, err := DeserializeOps(ops)

	if err != nil {
		return nil, err
	}

	for _, o := range *extractedOps {
		switch o.OpCode {
		case "=":
			{
				oldPos += o.Chars
				calcNewLen += o.Chars
			}
		case "-":
			{
				oldPos += o.Chars
				if !(oldPos <= oldLen) {
					return nil, errors.New("oldPos > oldLen in changeset")
				}
			}
		case "+":
			bankRunes := []rune(charBank)
			if len(bankRunes) < o.Chars {
				return nil, errors.New("invalid changeset: not enough chars in charBank")
			}
			var chars = bankRunes[0:o.Chars]
			var nlines = utils.CountLinesRunes(chars, '\n')
			if nlines != o.Lines {
				return nil, errors.New("invalid changeset: number of newlines in insert op does not match the charBank")
			}

			if !(o.Lines == 0 || utils.EndsWithNewLine(chars)) {
				return nil, errors.New("invalid changeset: multiline insert op does not end with a new line")
			}

			charBank = string(bankRunes[o.Chars:])
			calcNewLen += o.Chars
			if calcNewLen > newLen {
				return nil, errors.New("CalcNewLen > NewLen in cs")
			}
		default:
			return nil, errors.New("invalid changeset: Unknown opcode")
		}
		assem.Append(o)
	}

	calcNewLen += oldLen - oldPos
	if !(calcNewLen == newLen) {
		return nil, errors.New("invalid changeset claimed length does not match actual length")
	}

	if !(charBank == "") {
		return nil, errors.New("Invalid changeset excess characters in the charbank")
	}
	assem.EndDocument()

	var noramlized = Pack(oldLen, calcNewLen, assem.String(), unpacked.CharBank)
	if !(noramlized == cs) {
		return nil, errors.New("invalid changeset: not in canonical form")
	}
	return &cs, nil
}

func ApplyToText(cs string, text string) (*string, error) {
	var unpacked, _ = Unpack(cs)
	if utf8.RuneCountInString(text) != unpacked.OldLen {
		return nil, errors.New("mismatched text length")
	}

	var bankIter = NewStringIterator(unpacked.CharBank)
	var strIter = NewStringIterator(text)
	var assem = NewStringAssembler()

	var deserializedOp, err = DeserializeOps(unpacked.Ops)
	if err != nil {
		return nil, err
	}

	for _, op := range *deserializedOp {
		switch op.OpCode {
		case "+":
			peekedChars, err := bankIter.Peek(op.Chars)
			if err != nil {
				return nil, err
			}
			if op.Lines != len(strings.Split(*peekedChars, "\n"))-1 {
				return nil, errors.New("newline count is wrong in op +; cs:${cs} and text:${str}")
			}
			takenAssem, err := bankIter.Take(op.Chars)
			if err != nil {
				return nil, err
			}
			assem.Append(*takenAssem)
			break
		case "-":
			peekedStr, err := strIter.Peek(op.Chars)
			if err != nil {
				return nil, err
			}
			if op.Lines != len(strings.Split(*peekedStr, "\n"))-1 {
				return nil, errors.New("newline count is wrong in op -; cs:${cs} and text:${str}")
			}
			err = strIter.Skip(op.Chars)
			if err != nil {
				return nil, err
			}
			break
		case "=":
			peekedStr, err := strIter.Peek(op.Chars)
			if err != nil {
				return nil, err
			}
			if op.Lines != len(strings.Split(*peekedStr, "\n"))-1 {
				return nil, errors.New("newline count is wrong in op -; cs:${cs} and text:${str}")
			}
			iter, err := strIter.Take(op.Chars)
			if err != nil {
				return nil, err
			}
			assem.Append(*iter)
			break
		default:
			return nil, errors.New("invalid op type")
		}
	}
	takenRemaining, err := strIter.Take(strIter.Remaining())
	if err != nil {
		return nil, err
	}
	assem.Append(*takenRemaining)
	var stringRep = assem.String()
	return &stringRep, nil
}

func ApplyZip(in1 string, in2 string, callback func(*Op, *Op) (*Op, error)) (*string, error) {
	var ops1, err = DeserializeOps(in1)
	if err != nil {
		return nil, err
	}
	ops2, err := DeserializeOps(in2)
	if err != nil {
		return nil, err
	}

	var assem = NewSmartOpAssembler()
	var ops1Iterator = Iterator[Op]{
		ops: *ops1,
	}

	var ops2Iterator = Iterator[Op]{
		ops: *ops2,
	}
	// Process both ops slices concurrently
	op1, done1 := ops1Iterator.Next()
	op2, done2 := ops2Iterator.Next()
	var counter = 0
	for {
		counter += 1
		if done1 && done2 {
			break
		}

		if !done1 && op1.OpCode == "" {
			op1, done1 = ops1Iterator.Next()
		}
		if !done2 && op2.OpCode == "" {
			op2, done2 = ops2Iterator.Next()
		}

		if done1 {
			var op1Temp = NewOp(nil)
			op1 = &op1Temp
		}

		if done2 {
			var op2Temp = NewOp(nil)
			op2 = &op2Temp
		}

		if op1.OpCode == "" && op2.OpCode == "" {
			break
		}

		opOut, err := callback(op1, op2)
		if err != nil {
			return nil, err
		}
		if opOut.OpCode != "" {
			assem.Append(*opOut)
		}
	}
	assem.EndDocument()
	stringified := assem.String()
	return &stringified, nil
}

func Subattribution(astr string, start int, optEnd *int) (*string, error) {
	attOps, err := DeserializeOps(astr)
	if err != nil {
		return nil, err
	}

	assem := NewSmartOpAssembler()
	ops := *attOps

	// Iterator über ops
	idx := 0
	attOp := NewOp(nil) // leerer op
	csOp := NewOp(nil)

	// Hilfsfunktion: liefert nächstes attOp falls vorhanden
	nextAttOp := func() bool {
		if idx < len(ops) {
			attOp = ops[idx]
			idx++
			return true
		}
		// keine weiteren Ops -> leerer attOp
		attOp = NewOp(nil)
		return false
	}

	doCsOp := func() error {
		if csOp.Chars == 0 {
			return nil
		}
		for csOp.OpCode != "" && (attOp.OpCode != "" || idx < len(ops)) {
			if attOp.OpCode == "" {
				// hole nächsten attOp
				nextAttOp()
			}
			if csOp.OpCode != "" && attOp.OpCode != "" && csOp.Chars >= attOp.Chars && attOp.Lines > 0 && csOp.Lines <= 0 {
				csOp.Lines++
			}

			opOut, err := SlicerZipperFunc(&attOp, &csOp, nil)
			if err != nil {
				return err
			}
			if opOut.OpCode != "" {
				assem.Append(*opOut)
			}
		}
		return nil
	}

	csOp.OpCode = "-"
	csOp.Chars = start

	if err := doCsOp(); err != nil {
		return nil, err
	}

	if optEnd == nil {
		if attOp.OpCode != "" {
			assem.Append(attOp)
		}
		for idx < len(ops) {
			assem.Append(ops[idx])
			idx++
		}
	} else {
		if *optEnd < start {
			return nil, errors.New("optEnd < start")
		}
		csOp.OpCode = "="
		csOp.Chars = *optEnd - start
		if err := doCsOp(); err != nil {
			return nil, err
		}
	}

	res := assem.String()
	return &res, nil
}

func DeserializeOps(ops string) (*[]Op, error) {
	var regex = regexp.MustCompile(`((?:\*[0-9a-z]+)*)(?:\|([0-9a-z]+))?([-+=])([0-9a-z]+)|(.)`)
	var opsToReturn = make([]Op, 0)
	matches := regex.FindAllStringSubmatch(ops, -1)

	for _, match := range matches {
		if match[5] == "$" {
			continue
		}
		if match[5] != "" {
			return nil, errors.New("invalid op string")
		}
		var opMatch = match[3]
		var op = NewOp(&opMatch)

		var lines string
		if match[2] != "" {
			lines = match[2]
		} else {
			lines = "0"
		}

		op.Lines, _ = utils.ParseNum(lines)
		op.Chars, _ = utils.ParseNum(match[4])
		op.Attribs = match[1]
		opsToReturn = append(opsToReturn, op)
	}
	return &opsToReturn, nil
}

func Compose(cs1 string, cs2 string, pool *apool.APool) (*string, error) {
	var unpacked1, _ = Unpack(cs1)
	var unpacked2, _ = Unpack(cs2)
	var len1 = unpacked1.OldLen
	var len2 = unpacked1.NewLen

	if len2 != unpacked2.OldLen {
		return nil, errors.New("mismatched lengths in compose")
	}

	var len3 = unpacked2.NewLen
	var bankIter1 = NewStringIterator(unpacked1.CharBank)
	var bankIter2 = NewStringIterator(unpacked2.CharBank)
	var bankAssem = NewStringAssembler()

	var newOps, err = ApplyZip(unpacked1.Ops, unpacked2.Ops, func(op1, op2 *Op) (*Op, error) {
		var op1code = op1.OpCode
		var op2code = op2.OpCode

		if op1code == "+" && op2code == "-" {
			if err := bankIter1.Skip(int(math.Min(float64(op1.Chars), float64(op2.Chars)))); err != nil {
				return nil, err
			}
		}

		var opOut, err = SlicerZipperFunc(op1, op2, pool)

		if err != nil {
			panic(fmt.Sprintf("Error in SlicerZipperFunc: %v", err))
		}

		if opOut.OpCode == "+" {
			if op2code == "+" {
				takenFromBankIter2, err := bankIter2.Take(opOut.Chars)
				if err != nil {
					return nil, err
				}
				bankAssem.Append(*takenFromBankIter2)
			} else {
				takenFromBankIter1, err := bankIter1.Take(opOut.Chars)
				if err != nil {
					return nil, err
				}
				bankAssem.Append(*takenFromBankIter1)
			}
		}
		return opOut, nil
	})
	if err != nil {
		return nil, err
	}

	packedChangeset := Pack(len1, len3, *newOps, bankAssem.String())

	return &packedChangeset, nil
}

func ComposeAttributes(attribs1 string, attribs2 string, resultIsMutation bool, pool *apool.APool) string {
	// att1 and att2 are strings like "*3*f*1c", asMutation is a boolean.
	// Sometimes attribute (key,value) pairs are treated as attribute presence
	// information, while other times they are treated as operations that
	// mutate a set of attributes, and this affects whether an empty value
	// is a deletion or a change.
	// Examples, of the form (att1Items, att2Items, resultIsMutation) -> result
	// ([], [(bold, )], true) -> [(bold, )]
	// ([], [(bold, )], false) -> []
	// ([], [(bold, true)], true) -> [(bold, true)]
	// ([], [(bold, true)], false) -> [(bold, true)]
	// ([(bold, true)], [(bold, )], true) -> [(bold, )]
	// ([(bold, true)], [(bold, )], false) -> []
	// pool can be null if att2 has no attributes.
	if attribs1 == "" && resultIsMutation {
		// In the case of a mutation (i.e. composing two exportss),
		// an att2 composed with an empy att1 is just att2.  If att1
		// is part of an attribution string, then att2 may remove
		// attributes that are already gone, so don't do this optimization.
		return attribs2
	}
	if attribs2 == "" {
		return attribs1
	}
	var attrMap = FromString(attribs1, pool)
	var negatedResultIsMutation = !resultIsMutation
	return attrMap.UpdateFromString(attribs2, &negatedResultIsMutation).String()
}

func SlicerZipperFunc(attOp *Op, csOp *Op, pool *apool.APool) (*Op, error) {
	var opOut = NewOp(nil)
	if attOp.OpCode == "" {
		copyOp(*csOp, &opOut)
		csOp.OpCode = ""
	} else if csOp.OpCode == "" {
		copyOp(*attOp, &opOut)
		attOp.OpCode = ""
	} else if attOp.OpCode == "-" {
		copyOp(*attOp, &opOut)
		attOp.OpCode = ""
	} else if csOp.OpCode == "+" {
		copyOp(*csOp, &opOut)
		csOp.OpCode = ""
	} else {
		var opsToIterate = []*Op{attOp, csOp}
		for _, op := range opsToIterate {
			if !(op.Chars >= op.Lines) {
				return nil, errors.New("op has more characters than lines")
			}
		}

		var condition bool
		if attOp.Chars < csOp.Chars {
			condition = attOp.Lines <= csOp.Lines
		} else if attOp.Chars > csOp.Chars {
			condition = attOp.Lines >= csOp.Lines
		} else {
			condition = attOp.Lines == csOp.Lines
		}

		if !condition {
			return nil, errors.New("line count mismatch when composing changesets A*B; ")
		}

		if !slices.Contains([]string{"=", "+"}, attOp.OpCode) {
			return nil, errors.New("unexpected opcode in op: " + attOp.String())
		}

		if !slices.Contains([]string{"=", "-"}, csOp.OpCode) {
			return nil, errors.New("unexpected opcode in op: " + csOp.String())
		}

		if attOp.OpCode == "+" {
			if csOp.OpCode == "-" {
				opOut.OpCode = ""
			} else if csOp.OpCode == "=" {
				opOut.OpCode = "+"
			}
		} else if attOp.OpCode == "=" {
			if csOp.OpCode == "-" {
				opOut.OpCode = "-"
			} else if csOp.OpCode == "=" {
				opOut.OpCode = "="
			}
		}

		// Sort ascending
		if opsToIterate[0].Chars > opsToIterate[1].Chars {
			opsToIterate = []*Op{csOp, attOp}
		}

		var fullyConsumedOp = opsToIterate[0]
		var partiallyConsumedOp = opsToIterate[1]

		opOut.Chars = fullyConsumedOp.Chars
		opOut.Lines = fullyConsumedOp.Lines
		if csOp.OpCode == "-" {
			opOut.Attribs = csOp.Attribs
		} else {
			opOut.Attribs = ComposeAttributes(attOp.Attribs, csOp.Attribs, attOp.OpCode == "=", pool)
		}
		partiallyConsumedOp.Chars -= fullyConsumedOp.Chars
		partiallyConsumedOp.Lines -= fullyConsumedOp.Lines
		if partiallyConsumedOp.Chars == 0 {
			partiallyConsumedOp.OpCode = ""
		}
		fullyConsumedOp.OpCode = ""
	}
	return &opOut, nil
}

func ApplyToAttribution(cs string, astr string, pool apool.APool) (*string, error) {
	var unpacked, _ = Unpack(cs)
	return ApplyZip(astr, unpacked.Ops, func(op1, op2 *Op) (*Op, error) {
		res, err := SlicerZipperFunc(op1, op2, &pool)

		if err != nil {
			return nil, err
		}

		return res, nil
	})
}

func ApplyToAText(cs string, atext apool.AText, pool apool.APool) (*apool.AText, error) {
	text, err := ApplyToText(cs, atext.Text)
	if err != nil {
		return nil, err
	}
	attribs, err := ApplyToAttribution(cs, atext.Attribs, pool)

	if err != nil {
		return nil, err
	}

	return &apool.AText{
		Text:    *text,
		Attribs: *attribs,
	}, nil
}

func MakeAttribution(text string) string {
	var assem = NewSmartOpAssembler()
	for _, i := range OpsFromText("+", text, nil, nil) {
		assem.Append(i)
	}

	return assem.String()
}

func MakeAText(str string, attribs *string) apool.AText {
	var aTextAttrib = ""
	if attribs != nil {
		aTextAttrib = *attribs
	} else {
		aTextAttrib = MakeAttribution(str)
	}

	return apool.AText{
		Text:    str,
		Attribs: aTextAttrib,
	}
}

func CloneAText(atext apool.AText) apool.AText {
	return apool.AText{
		Text:    atext.Text,
		Attribs: atext.Attribs,
	}
}

func MoveOpsToNewPool(cs string, oldPool *apool.APool, newPool *apool.APool) string {
	dollarPos := utils.RuneIndex(cs, "$")
	if dollarPos < 0 {
		dollarPos = utf8.RuneCountInString(cs)
	}
	upToDollar := utils.RuneSlice(cs, 0, dollarPos)
	fromDollar := utils.RuneSlice(cs, dollarPos, utf8.RuneCountInString(cs))

	re := regexp.MustCompile(`\*([0-9a-z]+)`)
	result := re.ReplaceAllStringFunc(upToDollar, func(match string) string {
		sub := re.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		a := sub[1]

		oldNum, err := utils.ParseNum(a)
		if err != nil {
			// ungültige Nummer -> entfernen wie im JS-Beispiel
			return ""
		}

		pair, _ := oldPool.GetAttrib(oldNum)
		if pair == nil {
			// Attribut eventuell nicht im alten Pool -> wie JS: entfernen
			return ""
		}

		newNum := newPool.PutAttrib(*pair, nil)
		return "*" + utils.NumToString(newNum)
	})

	return result + fromDollar
}

type PrepareForWireStruct struct {
	Translated string
	Pool       apool.APool
}

func PrepareForWire(cs string, pool apool.APool) PrepareForWireStruct {
	var newPool = apool.NewAPool()
	var newCS = MoveOpsToNewPool(cs, &pool, &newPool)
	return PrepareForWireStruct{
		Pool:       newPool,
		Translated: newCS,
	}
}

var splitTextRegex = regexp.MustCompile(`[^\n]*\n|[^\n]+$`)

func SplitTextLines(text string) []string {
	matches := splitTextRegex.FindAllString(text, -1)
	if matches == nil {
		return []string{}
	}
	return matches
}

func SplitAttributionLines(attrOps string, text string) ([]string, error) {
	var assem = NewMergingOpAssembler()
	var lines = make([]string, 0)
	var pos = 0
	appendOp := func(op Op) {
		assem.Append(op)
		if op.Lines > 0 {
			lines = append(lines, assem.String())
			assem.Clear()
		}
		pos += op.Chars
	}

	var deserializedOps, _ = DeserializeOps(attrOps)

	for _, op := range *deserializedOps {
		var numChars = op.Chars
		var numLines = op.Lines
		for numLines > 1 {
			rest := utils.RuneSlice(text, pos, utf8.RuneCountInString(text))
			relIdx := utils.RuneIndex(rest, "\n")
			if !(relIdx >= 0) {
				return nil, errors.New("newLineEnd <= 0 in splitAttributionLines")
			}
			var newlineEnd = pos + relIdx + 1
			op.Chars = newlineEnd - pos
			op.Lines = 1
			appendOp(op)
			numChars -= op.Chars
			numLines -= op.Lines
		}

		if numLines == 1 {
			op.Chars = numChars
			op.Lines = 1
		}
		appendOp(op)
	}

	return lines, nil
}

func JoinAttributionLines(theAlines []string) string {
	var assem = NewMergingOpAssembler()

	for _, line := range theAlines {
		var deserializedOps, _ = DeserializeOps(line)
		for _, op := range *deserializedOps {
			assem.Append(op)
		}
	}
	return assem.String()
}
