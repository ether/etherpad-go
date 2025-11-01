package test

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
)

func TestMakeSplice(t *testing.T) {
	var testString = "a\nb\nc\n"
	var splicedText, _ = changeset.MakeSplice(testString, 5, 0, "def", nil, nil)
	if splicedText != "Z:6>3|2=4=1+3$def" {
		t.Error("Expected Z:6>3|2=4=1+3$def, got ", splicedText)
	}
	var t2, err = changeset.ApplyToText(splicedText, testString)
	if err != nil {
		t.Error(err)
	}
	if *t2 != "a\nb\ncdef\n" {
		t.Error("Expected a\nb\ncdef\n, got ", *t2)
	}
}

func TestMakeSpliceAtEnd(t *testing.T) {
	var orig = "123"
	var ins = "456"
	var splice, err = changeset.MakeSplice(orig, len(orig), 0, ins, nil, nil)

	if err != nil {
		t.Error("Error making splice" + err.Error())
	}

	atext, err := changeset.ApplyToText(splice, orig)

	if *atext != orig+ins {
		t.Error("They need to be the same")
	}
}

func TestOpsFromTextWithEqual(t *testing.T) {
	var teststring = "a\nb\nc\n"
	var ops = changeset.OpsFromText("=", teststring[0:5], nil, nil)
	if len(ops) != 2 {
		t.Error("Expected 2, got ", len(ops))
	}

	if ops[0].OpCode != "=" {
		t.Error("Expected =, got ", ops[0].OpCode)
	}

	if ops[0].Chars != 4 {
		t.Error("Expected 4, got ", ops[0].Chars)
	}

	if ops[0].Lines != 2 {
		t.Error("Expected 2, got ", ops[0].Lines)
	}

	if ops[1].OpCode != "=" {
		t.Error("Expected =, got ", ops[1].OpCode)
	}

	if ops[1].Chars != 1 {
		t.Error("Expected 1, got ", ops[1].Chars)
	}

	if ops[1].Lines != 0 {
		t.Error("Expected 0, got ", ops[1].Lines)
	}
}

func TestOpsFromTextWithMinus(t *testing.T) {
	var ops = changeset.OpsFromText("-", "", nil, nil)

	if len(ops) != 1 {
		t.Error("Expected 1, got ", len(ops))
	}

	if ops[0].OpCode != "-" {
		t.Error("Expected -, got ", ops[0].OpCode)
	}

	if ops[0].Chars != 0 {
		t.Error("Expected 0, got ", ops[0].Chars)
	}
}

func TestOpsFromTextWithPlus(t *testing.T) {
	var ops = changeset.OpsFromText("+", "def", nil, nil)

	if len(ops) != 1 {
		t.Error("Expected 1, got ", len(ops))
	}

	if ops[0].OpCode != "+" {
		t.Error("Expected +, got ", ops[0].OpCode)
	}

	if ops[0].Chars != 3 {
		t.Error("Expected 3, got ", ops[0].Chars)
	}
}

func TestUnpackChangeset(t *testing.T) {
	var cs = "Z:z>1|2=m=b*0|1+1$\n"
	var unpacked, err = changeset.Unpack(cs)

	if err != nil {
		t.Error("Error unpacking changeset " + err.Error())
		return
	}

	if unpacked.OldLen != 35 {
		t.Error("Expected 35, got ", unpacked.OldLen)
	}

	if unpacked.NewLen != 36 {
		t.Error("Expected 36, got ", unpacked.NewLen)
	}

	if unpacked.Ops != "|2=m=b*0|1+1" {
		t.Error("Expected |2=m=b*0|1+1, got ", unpacked.Ops)
	}

	if unpacked.CharBank != "\n" {
		t.Error("Expected \n, got ", unpacked.CharBank)
	}
}

func TestApplyToAttribution(t *testing.T) {
	runApplyToAttributionTest(1, []string{"bold,", "bold,true"},
		"Z:7>3-1*0=1*1=1=3+4$abcd", "+1*1+1|1+5", "+1*1+1|1+8", t)
	runApplyToAttributionTest(2,
		[]string{"bold,", "bold,true"},
		"Z:g<4*1|1=6*1=5-4$", "|2+g", "*1|1+6*1+5|1+1", t)
}

func createPool(attribs []string) apool.APool {
	var foundPool = apool.NewAPool()
	for _, attrib := range attribs {
		var splitAttrib = strings.Split(attrib, ",")
		foundPool.PutAttrib(apool.Attribute{
			Key:   splitAttrib[0],
			Value: splitAttrib[1],
		}, nil)
	}
	return foundPool
}

func runApplyToAttributionTest(testId int, attribs []string, cs string, inAttr string, outCorrect string, t *testing.T) {
	var p = createPool(attribs)
	var resCS, err = changeset.CheckRep(cs)

	if err != nil {
		t.Error("CheckRep threw an error" + err.Error())
		return
	}

	result := changeset.ApplyToAttribution(*resCS, inAttr, p)

	if result != outCorrect {
		t.Error(testId, "Error comparing attributions original: "+*resCS+" "+result+" vs "+outCorrect)
	}
}

func TestMoveOpsToNewPool(t *testing.T) {
	var pool1 = apool.NewAPool()
	var pool2 = apool.NewAPool()

	pool1.PutAttrib(apool.Attribute{
		Key:   "baz",
		Value: "qux",
	}, nil)

	pool1.PutAttrib(apool.Attribute{
		Key:   "foo",
		Value: "bar",
	}, nil)

	pool2.PutAttrib(apool.Attribute{
		Key:   "foo",
		Value: "bar",
	}, nil)

	var changesetMoved = changeset.MoveOpsToNewPool("Z:1>2*1+1*0+1$ab", &pool1, &pool2)

	if changesetMoved != "Z:1>2*0+1*1+1$ab" {
		t.Error("Error in MoveOpsToNewPool")
	}

	var changesetMoved2 = changeset.MoveOpsToNewPool("*1+1*0+1", &pool1, &pool2)
	if changesetMoved2 != "*0+1*1+1" {
		t.Error("Error in MoveOpsToNewPool")
	}
}

func TestCompose(t *testing.T) {
	t.Skip()
	var p = apool.NewAPool()
	var startText = "\n\n\ntxs\nlyqizxohxosniewgzmf\nn\nieztehfrnd\nmdzr\n"

	var x1 = []string{
		"Z:19>q|1=1|4+s+8|2=2=2|1-2-8|2=e=1-2|1=8=4+2$\nkcaekgsd\njyu\nukkrfvsmufpjo\ncjabwrrdef",
		"\n\nkcaekgsd\njyu\nukkrfvsmufpjo\ncjabwrrd\n\ntxxosniewgzmf\nn\nitehfrnd\nmdzref\n",
	}

	var change1 = x1[0]
	var text1 = x1[1]

	var x2 = []string{
		"Z:1z<1b|2=2+1=5|1-4-3+3|1=1=2-1+2=7|6-13-6$fthonw",
		"\n\nfkkt\nmhlmmeqvexugyrd\n\n\nho\nuknwrfvsmufsebed\n",
	}

	var change2 = x2[0]
	var text2 = x2[1]

	var x3 = []string{
		"Z:o>l|2=2=2-3=2|4+j|1=3=b+5$\nmhlmmeqvexugyrd\n\n\nsebed",
		"\n\nfkkt\nmhlmmeqvexugyrd\n\n\nho\nuknwrfvsmufsebed\n",
	}

	var change3 = x3[0]
	var text3 = x3[1]

	var firstChange = changeset.Compose(change1, change2, p)
	var change12, _ = changeset.CheckRep(firstChange)

	var change23, _ = changeset.CheckRep(changeset.Compose(change2, change3, p))

	var change123, _ = changeset.CheckRep(changeset.Compose(*change12, change3, p))
	var change123a, _ = changeset.CheckRep(changeset.Compose(change1, *change23, p))

	if change123a != change123 {
		t.Error("Error in Compose")
	}

	appliedText1, _ := changeset.ApplyToText(*change123, startText)

	if *appliedText1 != text2 {
		t.Error("Error in ApplyToText")
	}

	appliedText2, _ := changeset.ApplyToText(*change23, text1)

	if *appliedText2 != text3 {
		t.Error("Error in ApplyToText")
	}

	appliedText3, _ := changeset.ApplyToText(*change123, startText)

	if *appliedText3 != text3 {
		t.Error("Error in ApplyToText")
	}
}

func TestSlicerZipperFunc(t *testing.T) {
	var numToAttrib = make(map[int]apool.Attribute)
	var attribToNum = make(map[apool.Attribute]int)

	var attrib1 = apool.Attribute{
		Key:   "bold",
		Value: "",
	}

	var attrib2 = apool.Attribute{
		Key:   "bold",
		Value: "true",
	}

	attribToNum[attrib1] = 0
	attribToNum[attrib2] = 1
	numToAttrib[0] = apool.Attribute{
		Key:   "bold",
		Value: "",
	}

	numToAttrib[1] = apool.Attribute{
		Key:   "bold",
		Value: "true",
	}

	var pool = apool.APool{
		NumToAttrib: numToAttrib,
		NextNum:     2,
		AttribToNum: attribToNum,
	}

	var op1 = changeset.Op{
		OpCode:  "+",
		Chars:   1,
		Lines:   0,
		Attribs: "",
	}

	var op2 = changeset.Op{
		OpCode:  "-",
		Chars:   1,
		Lines:   0,
		Attribs: "",
	}

	ops, err := changeset.SlicerZipperFunc(&op1, &op2, &pool)

	if err != nil {
		t.Error("Error in SlicerZipperFunc " + err.Error())
		return
	}

	if ops.OpCode != "" && ops.Chars != 0 && ops.Lines != 0 && ops.Attribs != "" {
		t.Error("Expected empty string, got ", ops)
	}
}

func stringToOps(str string) string {
	var assem = changeset.NewMergingOpAssembler()
	var opCode = "+"
	var o = changeset.NewOp(&opCode)
	o.Chars = 1

	for i := 0; i < len(str); i++ {
		var char = str[i]
		if char == '\n' {
			o.Lines = 1
		} else {
			o.Lines = 0
		}

		if char == 'a' || char == 'b' {
			o.Attribs = "*" + string(char)
		} else {
			o.Attribs = ""
		}
		assem.Append(o)
	}

	return assem.String()
}

func testSplitJoinAttributionLines(t *testing.T) {
	var regexSplitLines = regexp.MustCompile("[^\n]*\n")
	var doc = `hsdxvuhehpo


lkrfrk


ezaxyidzrqi
ivmxtsnewx
imme
`
	var theJoined = stringToOps(doc)

	var expectedSplit = []string{
		"|1+c", "|1+1",
		"|1+1", "|1+7",
		"|1+1", "|1+1",
		"+2*a+1|1+9", "|1+b",
		"|1+5",
	}

	if theJoined != "|6+n+2*a+1|3+p" {
		t.Error("Error in stringToOps")
	}

	var theSplitTemporary = regexSplitLines.FindAllString(theJoined, -1)
	var theSplit = make([]string, len(theSplitTemporary))
	for i, v := range theSplitTemporary {
		theSplit[i] = stringToOps(v)
	}

	var res, err = changeset.SplitAttributionLines(theJoined, doc)
	var res2 = changeset.JoinAttributionLines(theSplit)

	if err != nil {
		t.Error("Error in SplitAttributionLines " + err.Error())
	}

	if !slices.Equal(res, expectedSplit) {
		t.Error("Error in SplitAttributionLines")
	}

	if res2 != theJoined {
		t.Error("Error in JoinAttributionLines")
	}

}

func TestSplitJoinAttributionLines(t *testing.T) {
	t.Skip()
	testSplitJoinAttributionLines(t)
}

func TestComposeAttributes(t *testing.T) {
	t.Skip()
	var p = apool.NewAPool()
	p.PutAttrib(apool.Attribute{
		Key:   "bold",
		Value: "",
	}, nil)
	p.PutAttrib(apool.Attribute{
		Key:   "bold",
		Value: "true",
	}, nil)
	var cs1, _ = changeset.CheckRep("Z:2>1*1+1*1=1$x")
	var cs2, _ = changeset.CheckRep("Z:3>0*0|1=3$")
	var comp = changeset.Compose(*cs1, *cs2, p)
	var cs12, _ = changeset.CheckRep(comp)

	if *cs12 != "Z:2>1+1*0|1=2$x" {
		t.Error("Error in ComposeAttributes")
	}
}

func TestDeserializeOps(t *testing.T) {
	var changesetToCheck = "-1*0=1*1=1=3+4"
	res, err := changeset.DeserializeOps(changesetToCheck)
	if err != nil {
		t.Error("Error should be nil")
	}

	if len(*res) != 5 {
		t.Error("too short", len(*res))
	}
}

// Utility function to print full match details
func printMatchDetails(matches [][]string, input string) {
	for i, match := range matches {
		fmt.Printf("Match %d is:\n", i)
		for j, group := range match {
			if i == 0 {
				// Full Match
				fmt.Printf("  '%v'\n", group)
			} else {
				fmt.Printf("  Group %d: '%v'\n", j, group)
			}
		}
		fmt.Printf("  index: %d\n", matchIndex(input, match[0]))
	}
}

// Helper function to find match index
func matchIndex(input, match string) int {
	return regexp.MustCompile(regexp.QuoteMeta(match)).FindStringIndex(input)[0]
}

func TestRegexMatcher(t *testing.T) {
	input := "+1*1+1|1+5"
	pattern := `((?:\*[0-9a-z]+)*)(?:\|([0-9a-z]+))?([-+=])([0-9a-z]+)|(.)`
	regex := regexp.MustCompile(pattern)

	var i = regex.FindAllStringSubmatch(input, -1)
	printMatchDetails(i, input)

}

func TestRegexMatcher2(t *testing.T) {
	input := "-1*0=1*1=1=3+4"
	pattern := `((?:\*[0-9a-z]+)*)(?:\|([0-9a-z]+))?([-+=])([0-9a-z]+)|(.)`
	regex := regexp.MustCompile(pattern)
	var i = regex.FindAllStringSubmatch(input, -1)

	// First regex matching
	if i[0][0] != "-1" && i[0][1] != "" && i[0][2] != "" && i[0][3] != "-" && i[0][4] != "1" && i[0][5] != "" {
		t.Error("Not correctly resolved")
	}

	if i[1][0] != "*0=1" && i[1][1] != "*0" && i[1][2] != "" && i[1][3] != "=" && i[1][4] != "1" && i[1][5] != "" {
		t.Error("Not correctly resolved")
	}

	if len(i) != 5 {
		t.Error("Too short")
	}
}

func TestRegexMatcher3(t *testing.T) {
	input := "+1*1+1|1+5"
	pattern := `((?:\*[0-9a-z]+)*)(?:\|([0-9a-z]+))?([-+=])([0-9a-z]+)|(.)`
	regex := regexp.MustCompile(pattern)
	var i = regex.FindAllStringSubmatch(input, -1)

	if len(i) != 3 {
		t.Error("Wrong length")
	}
}

func TestSerializeChangeset(t *testing.T) {
	input := "+1*1+1|1+5"
	var ops, _ = changeset.DeserializeOps(input)
	var deserializedOps = *ops
	if deserializedOps[0].OpCode != "+" &&
		deserializedOps[0].Chars != 1 &&
		deserializedOps[0].Lines != 0 &&
		deserializedOps[0].Attribs != "" {
		t.Error("Invalid deserialized")
	}

	if deserializedOps[1].OpCode != "" &&
		deserializedOps[1].Chars != 1 &&
		deserializedOps[1].Lines != 0 &&
		deserializedOps[1].Attribs != "" {
		t.Error("Invalid deserialized")
	}

	if deserializedOps[2].OpCode != "+" &&
		deserializedOps[2].Chars != 5 &&
		deserializedOps[2].Lines != 1 &&
		deserializedOps[2].Attribs != "" {
		t.Error("Invalid deserialized")
	}
}

func SimpleComposeAttributes(t *testing.T) {
	var pool = apool.NewAPool()
	pool.PutAttrib(apool.Attribute{
		Key:   "bold",
		Value: "",
	}, nil)
	pool.PutAttrib(apool.Attribute{
		Key:   "bold",
		Value: "true",
	}, nil)

	cs1, err := changeset.CheckRep("Z:2>1*1+1*1=1$x")
	if err != nil {
		t.Error("Error in CheckRep", err)
		return
	}
	cs2, err := changeset.CheckRep("Z:3>0*0|1=3$")
	if err != nil {
		t.Error("Error in CheckRep", err)
		return
	}

	changeset.Compose(*cs1, *cs2, pool)

}
