package changeset

import (
	"errors"
	"fmt"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/utils"
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
	var ops1Counter = 0
	var ops2Counter = 0
	for len(*ops1) > ops1Counter || len(*ops2) > ops2Counter {
		var opsToUse1 Op
		if len(*ops1) == ops1Counter {
			opsToUse1 = NewOp(nil)
		} else {
			opsToUse1 = (*ops1)[ops1Counter]
			ops1Counter++
		}

		var opsToUse2 Op
		if len(*ops2) == ops2Counter {
			opsToUse2 = NewOp(nil)
		} else {
			opsToUse2 = (*ops2)[ops2Counter]
			ops2Counter++
		}
		var res = callback(&opsToUse1, &opsToUse2)
		if res.OpCode != "" {
			assem.Append(res)
		}
	}
	assem.EndDocument()
	return assem.String()
}

func DeserializeOps(ops string) (*[]Op, error) {
	var regex = regexp.MustCompile("((?:\\*[0-9a-z]+)*)(?:\\|([0-9a-z]+))?([-+=])([0-9a-z]+)|(.)")
	var matches = regex.FindAllStringSubmatch(ops, -1)
	var opsToReturn = make([]Op, 0)

	for _, match := range matches {
		if match[5] == "$" {
			return nil, errors.New("no valid op found")
		}

		var op = NewOp(&match[3])

		if len(match[2]) > 0 {
			op.Lines, _ = utils.ParseNum(match[2])
		} else {
			op.Lines = 0
		}

		op.Chars, _ = utils.ParseNum(match[4])
		op.Attribs = match[1]
		opsToReturn = append(opsToReturn, op)
	}
	return &opsToReturn, nil
}

func ComposeAttributes(attribs1 string, attribs2 string, resultIsMutation bool, pool apool.APool) string {
	if attribs1 == "" && resultIsMutation {
		return attribs2
	}
	if attribs2 == "" {
		return attribs1
	}
	var attrMap = FromString(attribs1, pool)
	var negatedResultIsMutation = !resultIsMutation
	return attrMap.UpdateFromString(attribs2, &negatedResultIsMutation).String()
}

func SlicerZipperFunc(attOp Op, csOp Op, pool apool.APool) (*Op, error) {
	var opOut = NewOp(nil)
	if attOp.OpCode == "" {
		copyOp(csOp, &opOut)
		csOp.OpCode = ""
	} else if csOp.OpCode == "" {
		copyOp(attOp, &opOut)
		attOp.OpCode = ""
	} else if attOp.OpCode == "-" {
		copyOp(attOp, &opOut)
		attOp.OpCode = ""
	} else if csOp.OpCode == "+" {
		copyOp(attOp, &opOut)
		csOp.OpCode = ""
	} else {
		var opsToIterate = []Op{attOp, csOp}
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
		slices.SortFunc(opsToIterate, func(a, b Op) int {
			return a.Chars - b.Chars
		})
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

func ApplyToAttribution(cs string, astr string, pool apool.APool) string {
	var unpacked, _ = Unpack(cs)
	return ApplyZip(astr, unpacked.Ops, func(op1, op2 *Op) Op {
		res, err := SlicerZipperFunc(*op1, *op2, pool)

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
		Attribs: ApplyToAttribution(cs, attribs, pool),
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

func moveOpsToNewPool(cs string, oldPool apool.APool, newPool apool.APool) string {
	var dollarPos = strings.Index(cs, "$")
	if dollarPos < 0 {
		dollarPos = len(cs)
	}

	var upToDollar = cs[:dollarPos]
	var fromDollar = cs[dollarPos:]
	var regex = regexp.MustCompile("\\*([0-9a-z]+)")
	var resultingString = regex.ReplaceAllStringFunc(upToDollar, func(match string) string {
		oldNum, _ := strconv.ParseInt(match[1:], 36, 64) // Parse the number from base 36 to base 10
		pair := oldPool.GetAttrib(int(oldNum))

		newNum := newPool.PutAttrib(pair, nil)
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
	var newCS = moveOpsToNewPool(cs, pool, *newPool)
	return PrepareForWireStruct{
		Pool:       *newPool,
		Translated: newCS,
	}
}
