package changeset

import (
	"errors"
	"fmt"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/utils"
	"reflect"
	"regexp"
	"slices"
	"strings"
)

type Changeset struct {
	OldLen   int
	NewLen   int
	Ops      string
	CharBank string
}

func opsFromText(opcode string, text string, attribs interface{}, pool *apool.APool) []Op {
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

	var equalOps = opsFromText("=", orig[:start], "", nil)
	var deletedOps = opsFromText("-", deleted, "", nil)
	var insertedOps = opsFromText("+", ins, attribs, pool)

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
	var foundHeaders = headerRegex.FindAllString(cs, -1)

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

func ApplyToText(cs string, text string) (*string, error) {
	var unpacked, _ = Unpack(cs)
	if len(text) == unpacked.OldLen {
		return nil, errors.New("mismatched text length")
	}

	var bankIter = NewStringIterator(unpacked.CharBank)
	var strIter = NewStringIterator(text)
	var assem = NewStringAssembler()

	var deserializedOp, _ = DeserializeOps(unpacked.Ops)
	switch deserializedOp.OpCode {
	case "=":
		assem.Append(strIter.Take(deserializedOp.Chars))
		break
	case "-":
		strIter.Skip(deserializedOp.Chars)
		break
	case "+":
		assem.Append(bankIter.Take(deserializedOp.Chars))
		break
	default:
		return nil, errors.New("invalid op type")
	}
	assem.Append(strIter.Take(strIter.Remaining()))
	var stringRep = assem.String()
	return &stringRep, nil
}

func ApplyZip(in1 string, in2 string, callback func(*Op, *Op) Op) string {
	var ops1, _ = DeserializeOps(in1)
	var ops2, _ = DeserializeOps(in2)

	var assem = NewSmartOpAssembler()

	for ops1.OpCode != "" || ops2.OpCode != "" {
		var op = callback(ops1, ops2)
		assem.Append(op)
	}
	assem.EndDocument()
	return assem.String()
}

func DeserializeOps(ops string) (*Op, error) {
	var regex = regexp.MustCompile("((?:\\*[0-9a-z]+)*)(?:\\|([0-9a-z]+))?([-+=])([0-9a-z]+)|(.)")
	var matches = regex.FindAllString(ops, -1)

	if matches[5] == "$" {
		return nil, errors.New("no valid op found")
	}

	var op = NewOp(&matches[3])

	if len(matches[2]) > 0 {
		op.Lines, _ = utils.ParseNum(matches[2])
	} else {
		op.Lines = 0
	}

	op.Chars, _ = utils.ParseNum(matches[4])
	op.Attribs = matches[1]
	return &op, nil
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
			if op.Chars > op.Lines {
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
		res, _ := SlicerZipperFunc(*op1, *op2, pool)
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
	for _, i := range opsFromText("+", text, nil, nil) {
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
