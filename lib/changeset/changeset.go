package changeset

import (
	"errors"
	"fmt"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/utils"
	"math"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
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
		lastNewlineIndex := strings.LastIndex(atext.Text, "\n")
		nextToLastNewlineEnd := strings.LastIndex(atext.Text[:lastNewlineIndex], "\n") + 1
		lastLineLength := len(atext.Text) - nextToLastNewlineEnd - 1
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
	var lastNewLinePos = strings.LastIndex(text, "\n")
	if lastNewLinePos < 0 {
		op.Chars = len(text)
		op.Lines = 0
		opsToReturn = append(opsToReturn, op)
	} else {
		op.Chars = lastNewLinePos + 1
		op.Lines = utils.CountLines(text, '\n')
		opsToReturn = append(opsToReturn, op)
		var op2 = copyOp(op, nil)
		op2.Chars = len(text) - (lastNewLinePos + 1)
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

	if start > len(orig) {
		start = len(orig)
	}

	if ndel > len(orig)-start {
		ndel = len(orig) - start
	}

	var deleted = orig[start : start+ndel]
	var assem = NewSmartOpAssembler()
	var opsGenerated = make([]Op, 0)

	var equalOps = OpsFromText("=", orig[:start], "", nil)
	var deletedOps = OpsFromText("-", deleted, "", nil)
	var insertedOps = OpsFromText("+", ins, attribs, pool)

	opsGenerated = append(opsGenerated, equalOps...)
	opsGenerated = append(opsGenerated, deletedOps...)
	opsGenerated = append(opsGenerated, insertedOps...)
	for _, op := range opsGenerated {
		assem.Append(op)
	}
	assem.EndDocument()
	return Pack(len(orig), len(orig)+len(ins)-ndel, assem.String(), ins), nil
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
	var opsStart = len(foundHeaders[0])
	var opsEnd = strings.Index(cs, "$")
	if opsEnd < 0 {
		opsEnd = len(cs)
	}

	return &Changeset{
		oldLen,
		newLen,
		cs[opsStart:opsEnd],
		cs[opsEnd+1:],
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

	var re = regexp.MustCompile("\\*" + strconv.FormatInt(int64(attribNum), 36) + "(?!\\w)")
	return func(attribs *string) bool {
		return re.MatchString(*attribs)
	}
}

func replaceAttributes(att2 string, callback func(match string) string) (string, map[string]string, error) {
	re := regexp.MustCompile(`\*([0-9a-z]+)`)
	atts := make(map[string]string)

	result := re.ReplaceAllStringFunc(att2, callback)

	return result, atts, nil
}

func followAttributes(att1 string, att2 string, pool *apool.APool) string {
	// The merge of two sets of attribute changes to the same text
	// takes the lexically-earlier value if there are two values
	// for the same key.  Otherwise, all key/value changes from
	// both attribute sets are taken.  This operation is the "follow",
	// so a set of changes is produced that can be applied to att1
	// to produce the merged set.
	if att2 == "" && pool == nil {
		return ""
	}

	if att1 == "" {
		return att2
	}

	var atts = make(map[string]string)
	replaceAttributes(att2, func(a string) string {
		parsedNum, _ := utils.ParseNum(a)
		var attrib, _ = pool.GetAttrib(parsedNum)
		atts[attrib.Key] = attrib.Value
		return ""
	})
	replaceAttributes(att1, func(a string) string {
		parsedNum, _ := utils.ParseNum(a)
		var attrib, _ = pool.GetAttrib(parsedNum)
		var res, ok = atts[attrib.Key]

		if ok && attrib.Value <= res {
			delete(atts, attrib.Key)
		}
		return ""
	})

	var buf = NewStringAssembler()
	for key, value := range atts {
		buf.Append("*")
		buf.Append(utils.NumToString(pool.PutAttrib(apool.Attribute{
			Key:   key,
			Value: value,
		}, nil)))
	}

	return buf.String()
}

func Follow(c string, rebasedChangeset string, reverseInsertOrder bool, pool *apool.APool) string {
	var unpacked1, _ = Unpack(c)
	var unpacked2, _ = Unpack(rebasedChangeset)

	var len1 = unpacked1.OldLen
	var len2 = unpacked2.NewLen
	if len1 != len2 {
		panic("mismatched lengths in follow")
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

	newOps := ApplyZip(unpacked1.Ops, unpacked2.Ops, func(op1, op2 *Op) Op {
		var opOut = NewOp(nil)
		if op1.OpCode == "+" || op2.OpCode == "+" {
			var whichToDo int

			if op2.OpCode != "+" {
				whichToDo = 1
			} else if op1.OpCode != "+" {
				whichToDo = 2
			} else {
				var firstChar1 = chars1.Peek(1)
				var firstChar2 = chars2.Peek(1)

				var insertFirst1 = hasInsertFirst(op1.Attribs)
				var insertFirst2 = hasInsertFirst(op2.Attribs)

				if insertFirst1 && !insertFirst2 {
					whichToDo = 1
				} else if insertFirst2 && !insertFirst1 {
					whichToDo = 2
				} else if firstChar1 == "\n" && firstChar2 != "\n" {
					whichToDo = 2
				} else if firstChar1 != "\n" && firstChar2 == "\n" {
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
				chars2.Skip(op2.Chars)
				copyOp(*op2, &opOut)
				op2.OpCode = ""
			}
		} else if op1.OpCode == "-" {
			if op2.OpCode != "" {
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
			opOut.OpCode = ""
			opOut.Attribs = followAttributes(op1.Attribs, op2.Attribs, pool)
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
		return opOut
	})

	newLen += oldLen - oldPos
	return Pack(oldLen, newLen, newOps, unpacked2.CharBank)
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
			{
				if !(len(charBank) >= o.Chars) {
					return nil, errors.New("invalid changeset: not enough chars in charBank")
				}
				var chars = charBank[0:o.Chars]
				var nlines = utils.CountLines(chars, '\n')
				if !(nlines == o.Lines) {
					return nil, errors.New("invalid changeset: number of newlines in insert op does not match the charBank")
				}

				if !(o.Lines == 0 || strings.HasSuffix(chars, "\n")) {
					return nil, errors.New("invalid changeset: multiline insert op does not end with a new line")
				}

				charBank = charBank[o.Chars:]
				calcNewLen += o.Chars
				if !(calcNewLen <= newLen) {
					return nil, errors.New("CalcNewLen > NewLen in cs")
				}
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
	if len(text) != unpacked.OldLen {
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
			if op.Lines != len(strings.Split(bankIter.Peek(op.Chars), "\n"))-1 {
				return nil, errors.New("newline count is wrong in op +; cs:${cs} and text:${str}")
			}
			assem.Append(bankIter.Take(op.Chars))
			break
		case "-":
			if op.Lines != len(strings.Split(strIter.Peek(op.Chars), "\n"))-1 {
				return nil, errors.New("newline count is wrong in op -; cs:${cs} and text:${str}")
			}
			err := strIter.Skip(op.Chars)
			if err != nil {
				return nil, err
			}
			break
		case "=":
			if op.Lines != len(strings.Split(strIter.Peek(op.Chars), "\n"))-1 {
				return nil, errors.New("newline count is wrong in op -; cs:${cs} and text:${str}")
			}
			var iter = strIter.Take(op.Chars)
			assem.Append(iter)
			break
		default:
			return nil, errors.New("invalid op type")
		}
	}
	assem.Append(strIter.Take(strIter.Remaining()))
	var stringRep = assem.String()
	return &stringRep, nil
}

func ApplyZip(in1 string, in2 string, callback func(*Op, *Op) Op) string {
	var ops1, _ = DeserializeOps(in1)
	var ops2, _ = DeserializeOps(in2)

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

		opOut := callback(op1, op2)
		if opOut.OpCode != "" {
			assem.Append(opOut)
		}
	}
	assem.EndDocument()
	return assem.String()
}

// Helper function to find match index
func matchIndex(input, match string) int {
	return regexp.MustCompile(regexp.QuoteMeta(match)).FindStringIndex(input)[0]
}

func Subattribution(astr string, start int, end *int) (*string, error) {
	var attOps, err = DeserializeOps(astr)
	var counter = 0

	if err != nil {
		return nil, err
	}

	attOpsUnrefed := *attOps

	var attOpsNext = attOpsUnrefed[counter]
	var assem = NewSmartOpAssembler()
	var attOp = NewOp(nil)
	var csOp = NewOp(nil)

	doCspOp := func() {
		if csOp.Chars == 0 {
			return
		}

		for csOp.OpCode != "" && (attOp.OpCode != "" || counter < len(attOpsUnrefed)) {

			if attOp.OpCode == "" {
				attOp = attOpsUnrefed[counter]
				counter++
			}
		}

		if csOp.OpCode != "" && attOp.OpCode != "" && csOp.Chars >= attOp.Chars &&
			attOp.Lines > 0 && csOp.Lines > 0 {
			csOp.Lines++
		}

		var opOut, err = SlicerZipperFunc(&attOp, &csOp, nil)
		if err != nil {
			println("Error encountered", err.Error())
			return
		}
		if opOut.OpCode != "" {
			assem.Append(*opOut)
		}
	}

	csOp.OpCode = "-"
	csOp.Chars = start
	doCspOp()

	if end == nil {
		if attOp.OpCode != "" {
			assem.Append(attOp)
		}

		for attOp := range attOps {
			assem.Append(attOp)
		}

	}

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
			panic("Invalid operation")
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

func Compose(cs1 string, cs2 string, pool apool.APool) string {
	var unpacked1, _ = Unpack(cs1)
	var unpacked2, _ = Unpack(cs2)
	var len1 = unpacked1.OldLen
	var len2 = unpacked1.NewLen

	if len2 != unpacked2.OldLen {
		panic("mismatched new length in cs2")
	}

	var len3 = unpacked2.NewLen
	var bankIter1 = NewStringIterator(unpacked1.CharBank)
	var bankIter2 = NewStringIterator(unpacked2.CharBank)
	var bankAssem = NewStringAssembler()

	var newOps = ApplyZip(unpacked1.Ops, unpacked2.Ops, func(op1, op2 *Op) Op {
		var op1code = op1.OpCode
		var op2code = op2.OpCode

		if op1code == "+" && op2code == "-" {
			bankIter1.Skip(int(math.Min(float64(op1.Chars), float64(op2.Chars))))
		}

		var opOut, _ = SlicerZipperFunc(op1, op2, pool)

		if opOut.OpCode == "+" {
			bankAssem.Append(bankIter2.Take(opOut.Chars))
		} else {
			bankAssem.Append(bankIter1.Take(opOut.Chars))
		}

		return *opOut
	})

	return Pack(len1, len3, newOps, bankAssem.String())
}

func ComposeAttributes(attribs1 string, attribs2 string, resultIsMutation bool, pool apool.APool) string {
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
			panic("line count mismatch when composing changesets A*B; ")
		}

		if !slices.Contains([]string{"=", "+"}, attOp.OpCode) {
			panic("unexpected opcode in op: " + attOp.String())
		}

		if !slices.Contains([]string{"=", "-"}, csOp.OpCode) {
			panic("unexpected opcode in op: " + csOp.String())
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
			opOut.Attribs = ComposeAttributes(attOp.Attribs, csOp.Attribs, attOp.OpCode == "=", *pool)
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

func ApplyToAttribution(cs string, astr string, pool apool.APool) string {
	var unpacked, _ = Unpack(cs)
	return ApplyZip(astr, unpacked.Ops, func(op1, op2 *Op) Op {
		res, err := SlicerZipperFunc(op1, op2, pool)

		if err != nil {
			println("Error is" + err.Error())
		}

		return *res
	})
}

func ApplyToAText(cs string, atext apool.AText, pool apool.APool) apool.AText {
	var text, err = ApplyToText(cs, atext.Text)
	var attribs = ApplyToAttribution(cs, atext.Attribs, pool)

	if err != nil {
		panic(err)
	}

	return apool.AText{
		Text:    *text,
		Attribs: attribs,
	}
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

func MoveOpsToNewPool(cs string, oldPool apool.APool, newPool apool.APool) string {
	var dollarPos = strings.Index(cs, "$")
	if dollarPos < 0 {
		dollarPos = len(cs)
	}

	var upToDollar = cs[:dollarPos]
	var fromDollar = cs[dollarPos:]
	var regex = regexp.MustCompile("\\*([0-9a-z]+)")
	var resultingString = regex.ReplaceAllStringFunc(upToDollar, func(match string) string {
		oldNum, _ := strconv.ParseInt(match[1:], 36, 64) // Parse the number from base 36 to base 10
		pair, err := oldPool.GetAttrib(int(oldNum))

		if err != nil {
			panic(err)
		}

		newNum := newPool.PutAttrib(*pair, nil)
		return "*" + strconv.FormatInt(int64(newNum), 36)
	}) + fromDollar
	return resultingString
}

type PrepareForWireStruct struct {
	Translated string
	Pool       apool.APool
}

func PrepareForWire(cs string, pool apool.APool) PrepareForWireStruct {
	var newPool = apool.NewAPool()
	var newCS = MoveOpsToNewPool(cs, pool, *newPool)
	return PrepareForWireStruct{
		Pool:       *newPool,
		Translated: newCS,
	}
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
			var newlineEnd = strings.Index(text[pos:], "\n") + 1
			if !(newlineEnd > 0) {
				return nil, errors.New("newLineEnd <= 0 in splitAttributionLines")
			}
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
