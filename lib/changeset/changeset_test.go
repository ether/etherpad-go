package changeset

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/test/testutils/general"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMakeSplice(t *testing.T) {
	var testString = "a\nb\nc\n"
	var splicedText, _ = MakeSplice(testString, 5, 0, "def", nil, nil)
	if splicedText != "Z:6>3|2=4=1+3$def" {
		t.Error("Expected Z:6>3|2=4=1+3$def, got ", splicedText)
	}
	var t2, err = ApplyToText(splicedText, testString)
	if err != nil {
		t.Error(err)
	}
	if *t2 != "a\nb\ncdef\n" {
		t.Error("Expected a\nb\ncdef\n, got ", *t2)
	}
}

func TestAttributeTesterWithNilPool(t *testing.T) {
	testArg := "bold,true"
	returnedFunc := AttributeTester(apool.Attribute{}, nil)

	if returnedFunc(&testArg) != false {
		t.Error("Expected false when pool is nil")
	}
}

func TestFollowAttributes_Att2EmptyAndPoolNil(t *testing.T) {
	result, err := followAttributes("test", "", nil)
	require.NoError(t, err)
	assert.Equal(t, *result, "")
}

func TestFollowAttributes_Att1Empty(t *testing.T) {
	pool := apool.NewAPool()
	att2 := "*0*1"
	result, err := followAttributes("", att2, &pool)
	require.NoError(t, err)
	assert.Equal(t, att2, *result)
}

func TestFollowAttributes_DetailedDebug(t *testing.T) {
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "color", Value: "red"}, nil)
	pool.PutAttrib(apool.Attribute{Key: "size", Value: "large"}, nil)

	att1 := "*0" // color: red
	att2 := "*1" // size: large

	atts := make(map[string]string)

	_, _, err := replaceAttributes(att2, func(a string) (*string, error) {
		parsedNum, _ := utils.ParseNum(a)
		attrib, _ := pool.GetAttrib(parsedNum)
		atts[attrib.Key] = attrib.Value
		emptyStr := ""
		return &emptyStr, nil
	})
	require.NoError(t, err)

	_, _, err = replaceAttributes(att1, func(a string) (*string, error) {
		parsedNum, _ := utils.ParseNum(a)
		attrib, _ := pool.GetAttrib(parsedNum)
		res, ok := atts[attrib.Key]

		if ok && attrib.Value <= res {
			delete(atts, attrib.Key)
		} else {
		}
		emptyStr := ""
		return &emptyStr, nil
	})
	require.NoError(t, err)
}

func TestFollowAttributes_ConflictLexicalEarlierInAtt1(t *testing.T) {
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "color", Value: "blue"}, nil)
	pool.PutAttrib(apool.Attribute{Key: "color", Value: "red"}, nil)

	att1 := "*0"
	att2 := "*1"

	result, err := followAttributes(att1, att2, &pool)
	require.NoError(t, err)
	assert.Equal(t, "", *result)
}

func TestFollowAttributes_ConflictLexicalEarlierInAtt2(t *testing.T) {
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "color", Value: "red"}, nil)
	pool.PutAttrib(apool.Attribute{Key: "color", Value: "blue"}, nil)

	att1 := "*0" // color: red
	att2 := "*1" // color: blue (lexically earlier than red)

	result, err := followAttributes(att1, att2, &pool)
	require.NoError(t, err)
	assert.Equal(t, "*1", *result)
}

func TestFollowAttributes_MultipleAttributes(t *testing.T) {
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "color", Value: "blue"}, nil)
	pool.PutAttrib(apool.Attribute{Key: "size", Value: "large"}, nil)
	pool.PutAttrib(apool.Attribute{Key: "color", Value: "red"}, nil)

	att1 := "*0*1" // color: blue, size: large
	att2 := "*2"   // color: red

	result, err := followAttributes(att1, att2, &pool)
	require.NoError(t, err)
	assert.Equal(t, "", *result)
}

func TestFollowAttributes_ErrorParsingAtt2(t *testing.T) {
	pool := apool.NewAPool()
	att2 := "*invalid"

	_, err := followAttributes("*0", att2, &pool)
	assert.Error(t, err)
}

func TestFollowAttributes_ErrorParsingAtt1(t *testing.T) {
	pool := apool.NewAPool()
	att1 := "*invalid"
	att2 := "*0"

	_, err := followAttributes(att1, att2, &pool)
	assert.Error(t, err)
}

func TestFollowAttributes_EmptyAtt2WithPool(t *testing.T) {
	pool := apool.NewAPool()
	result, err := followAttributes("*0", "", &pool)
	require.NoError(t, err)
	assert.Equal(t, "", *result)
}

func TestFollowAttributes_Att2NilEquivalent(t *testing.T) {
	pool := apool.NewAPool()
	result, err := followAttributes("*0", "", &pool)
	require.NoError(t, err)
	assert.Equal(t, "", *result)
}

func TestFollowAttributes_BuildStringFromMap(t *testing.T) {
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "color", Value: "red"}, nil)
	pool.PutAttrib(apool.Attribute{Key: "size", Value: "small"}, nil)

	att1 := "*0" // color: red
	att2 := "*1" // size: small

	result, err := followAttributes(att1, att2, &pool)
	require.NoError(t, err)
	assert.Contains(t, *result, "*")
}

func TestAttributeTesterWithNegativeNumAttribPool(t *testing.T) {
	testArg := "bold,true"
	poolToTest := apool.NewAPool()
	poolToTest.NextNum = -50
	returnedFunc := AttributeTester(apool.Attribute{
		Key:   "bold",
		Value: "true",
	}, &poolToTest)

	if returnedFunc(&testArg) != false {
		t.Error("Expected false when pool is nil")
	}
}

func TestAttributeTesterWithInvalidAttribString(t *testing.T) {
	poolToTest := apool.NewAPool()
	poolToTest.NextNum = 50
	returnedFunc := AttributeTester(apool.Attribute{
		Key:   "bold",
		Value: "true",
	}, &poolToTest)

	tests := []string{
		"*a",
		"*a|1+5",
		"foo*a",
		"*a\n",
		"*a1",
		"*ab",
		"*a0",
		"a*ab",
	}

	for _, test := range tests {
		result := returnedFunc(&test)
		if result != false {
			t.Error("Expected false for test string: " + test)
		}
	}
}

func TestFollow(t *testing.T) {
	for i := 0; i < 30; i++ {
		t.Run("Follow test "+fmt.Sprint(i), func(t *testing.T) {
			p := apool.NewAPool()
			startText := general.RandomMultiline(10, 20) + "\n"

			cs1, _ := RandomTestChangeset(startText, false)
			cs2, _ := RandomTestChangeset(startText, false)

			newFollowedStr1, err := Follow(cs1, cs2, false, &p)
			if err != nil {
				t.Fatal("Error in Follow: " + err.Error())
			}
			newFollowedStr2, err := Follow(cs2, cs1, true, &p)
			if err != nil {
				t.Fatal("Error in Follow: " + err.Error())
			}

			afb, err := CheckRep(*newFollowedStr1)
			if err != nil {
				t.Fatal("Error in CheckRep: " + err.Error())
			}
			bfa, err := CheckRep(*newFollowedStr2)
			if err != nil {
				t.Fatal("Error in CheckRep: " + err.Error())
			}

			compose1, err := Compose(cs1, *afb, nil)
			if err != nil {
				t.Fatal("Error in Compose: " + err.Error())
			}
			compose2, err := Compose(cs2, *bfa, nil)
			if err != nil {
				t.Fatal("Error in Compose: " + err.Error())
			}
			merge1, err := CheckRep(*compose1)
			if err != nil {
				t.Fatal("Error in CheckRep: " + err.Error())
			}
			merge2, err := CheckRep(*compose2)
			if err != nil {
				t.Fatal("Error in CheckRep: " + err.Error())
			}
			if *merge1 != *merge2 {
				t.Fatalf("Followed changesets do not match:\n%s\n%s", *merge1, *merge2)
			}
		})
	}
}

func TestAttributeTesterWithValidAttribString(t *testing.T) {
	poolToTest := apool.NewAPool()
	for i := 0; i < 10; i++ {
		poolToTest.PutAttrib(apool.Attribute{Key: "dummy", Value: fmt.Sprint(i)}, nil)
	}
	puttedAttrib := apool.Attribute{Key: "bold", Value: "true"}
	poolToTest.PutAttrib(puttedAttrib, nil)

	returnedFunc := AttributeTester(puttedAttrib, &poolToTest)

	tests := []string{"*a", "*a*b", "*a|1+5"}
	for _, test := range tests {
		result := returnedFunc(&test)
		if result != true {
			t.Error("Expected true for test string: " + test)
		}
	}
}

func TestMakeSpliceAtEnd(t *testing.T) {
	var orig = "123"
	var ins = "456"
	var splice, err = MakeSplice(orig, utf8.RuneCountInString(orig), 0, ins, nil, nil)

	if err != nil {
		t.Error("Error making splice" + err.Error())
	}

	atext, err := ApplyToText(splice, orig)

	if *atext != orig+ins {
		t.Error("They need to be the same")
	}
}

func testSubAttribution(testId int, astr string, start int, end *int, correctOutput string, t *testing.T) {
	t.Run("SubAttribution test "+fmt.Sprint(testId), func(t *testing.T) {
		var result, err = Subattribution(astr, start, end)
		if err != nil {
			t.Error("Error in Subattribution " + err.Error())
		}
		if *result != correctOutput {
			t.Error("Error in Subattribution expected " + correctOutput + " got " + *result)
		}
	})
}

func TestSubAttribution(t *testing.T) {
	zero := 0
	one := 1
	two := 2
	three := 3
	four := 4
	five := 5
	six := 6
	seven := 7

	testSubAttribution(1, "+1", 0, &zero, "", t)
	testSubAttribution(2, "+1", 0, &one, "+1", t)
	testSubAttribution(3, "+1", 0, nil, "+1", t)
	testSubAttribution(4, "|1+1", 0, &zero, "", t)
	testSubAttribution(5, "|1+1", 0, &one, "|1+1", t)
	testSubAttribution(6, "|1+1", 0, nil, "|1+1", t)
	testSubAttribution(7, "*0+1", 0, &zero, "", t)
	testSubAttribution(8, "*0+1", 0, &one, "*0+1", t)
	testSubAttribution(9, "*0+1", 0, nil, "*0+1", t)
	testSubAttribution(10, "*0|1+1", 0, &zero, "", t)
	testSubAttribution(11, "*0|1+1", 0, &one, "*0|1+1", t)
	testSubAttribution(12, "*0|1+1", 0, nil, "*0|1+1", t)
	testSubAttribution(13, "*0+2+1*1+3", 0, &one, "*0+1", t)
	testSubAttribution(14, "*0+2+1*1+3", 0, &two, "*0+2", t)
	testSubAttribution(15, "*0+2+1*1+3", 0, &three, "*0+2+1", t)
	testSubAttribution(16, "*0+2+1*1+3", 0, &four, "*0+2+1*1+1", t)
	testSubAttribution(17, "*0+2+1*1+3", 0, &five, "*0+2+1*1+2", t)
	testSubAttribution(18, "*0+2+1*1+3", 0, &six, "*0+2+1*1+3", t)
	testSubAttribution(19, "*0+2+1*1+3", 0, &seven, "*0+2+1*1+3", t)
	testSubAttribution(20, "*0+2+1*1+3", 0, nil, "*0+2+1*1+3", t)
	testSubAttribution(21, "*0+2+1*1+3", 1, nil, "*0+1+1*1+3", t)
	testSubAttribution(22, "*0+2+1*1+3", 2, nil, "+1*1+3", t)
	testSubAttribution(23, "*0+2+1*1+3", 3, nil, "*1+3", t)
	testSubAttribution(24, "*0+2+1*1+3", 4, nil, "*1+2", t)
	testSubAttribution(25, "*0+2+1*1+3", 5, nil, "*1+1", t)
	testSubAttribution(26, "*0+2+1*1+3", 6, nil, "", t)
	testSubAttribution(27, "*0+2+1*1|1+3", 0, &one, "*0+1", t)
	testSubAttribution(28, "*0+2+1*1|1+3", 0, &two, "*0+2", t)
	testSubAttribution(29, "*0+2+1*1|1+3", 0, &three, "*0+2+1", t)
	testSubAttribution(30, "*0+2+1*1|1+3", 0, &four, "*0+2+1*1+1", t)
	testSubAttribution(31, "*0+2+1*1|1+3", 0, &five, "*0+2+1*1+2", t)
	testSubAttribution(32, "*0+2+1*1|1+3", 0, &six, "*0+2+1*1|1+3", t)
	testSubAttribution(33, "*0+2+1*1|1+3", 0, &seven, "*0+2+1*1|1+3", t)
	testSubAttribution(34, "*0+2+1*1|1+3", 0, nil, "*0+2+1*1|1+3", t)
	testSubAttribution(35, "*0+2+1*1|1+3", 1, nil, "*0+1+1*1|1+3", t)
	testSubAttribution(36, "*0+2+1*1|1+3", 2, nil, "+1*1|1+3", t)
	testSubAttribution(37, "*0+2+1*1|1+3", 3, nil, "*1|1+3", t)
	testSubAttribution(38, "*0+2+1*1|1+3", 4, nil, "*1|1+2", t)
	testSubAttribution(39, "*0+2+1*1|1+3", 5, nil, "*1|1+1", t)
	testSubAttribution(40, "*0+2+1*1|1+3", 1, &five, "*0+1+1*1+2", t)
	testSubAttribution(41, "*0+2+1*1|1+3", 2, &six, "+1*1|1+3", t)
	testSubAttribution(42, "*0+2+1*1+3", 2, &six, "+1*1+3", t)
}

func TestOpsFromTextWithEqual(t *testing.T) {
	var teststring = "a\nb\nc\n"
	var ops = OpsFromText("=", teststring[0:5], nil, nil)
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
	var ops = OpsFromText("-", "", nil, nil)

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
	var ops = OpsFromText("+", "def", nil, nil)

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
	var unpacked, err = Unpack(cs)

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

func TestApplyToAText(t *testing.T) {
	var cs = "Z:1>2*0+2$sa"
	var atext = apool.AText{
		Text:    "\n",
		Attribs: "|1+1",
	}

	var p = apool.NewAPool()
	p.NumToAttrib = map[int]apool.Attribute{}
	p.NumToAttrib[0] = apool.Attribute{
		Key:   "author",
		Value: "a.1ukWCzcdcCbywn32",
	}
	p.AttribToNum = map[apool.Attribute]int{}
	p.AttribToNum[apool.Attribute{
		Key:   "author",
		Value: "a.1ukWCzcdcCbywn32",
	}] = 0
	p.NextNum = 1
	var result, err = ApplyToAText(cs, atext, p)
	if err != nil {
		t.Error("Error in ApplyToAText " + err.Error())
		return
	}
	if result.Text != "sa\n" {
		t.Error("Error in ApplyToAText text")
	}
	if result.Attribs != "*0+2|1+1" {
		t.Error("Error in ApplyToAText attribs")
	}
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
	var resCS, err = CheckRep(cs)

	if err != nil {
		t.Error("CheckRep threw an error" + err.Error())
		return
	}

	result, err := ApplyToAttribution(*resCS, inAttr, p)
	if err != nil {
		t.Error(testId, "Error applying to attribution "+err.Error())
		return
	}

	if *result != outCorrect {
		t.Error(testId, "Error comparing attributions original: "+*resCS+" "+*result+" vs "+outCorrect)
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

	var changesetMoved = MoveOpsToNewPool("Z:1>2*1+1*0+1$ab", &pool1, &pool2)

	if changesetMoved != "Z:1>2*0+1*1+1$ab" {
		t.Error("Error in MoveOpsToNewPool")
	}

	var changesetMoved2 = MoveOpsToNewPool("*1+1*0+1", &pool1, &pool2)
	if changesetMoved2 != "*0+1*1+1" {
		t.Error("Error in MoveOpsToNewPool")
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

	var op1 = Op{
		OpCode:  "+",
		Chars:   1,
		Lines:   0,
		Attribs: "",
	}

	var op2 = Op{
		OpCode:  "-",
		Chars:   1,
		Lines:   0,
		Attribs: "",
	}

	ops, err := SlicerZipperFunc(&op1, &op2, &pool)

	if err != nil {
		t.Error("Error in SlicerZipperFunc " + err.Error())
		return
	}

	if ops.OpCode != "" && ops.Chars != 0 && ops.Lines != 0 && ops.Attribs != "" {
		t.Error("Expected empty string, got ", ops)
	}
}

func stringToOps(str string) string {
	var assem = NewMergingOpAssembler()
	var opCode = "+"
	var o = NewOp(&opCode)
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

func TestComposeAttributes(t *testing.T) {
	var p = apool.NewAPool()
	p.PutAttrib(apool.Attribute{
		Key:   "bold",
		Value: "",
	}, nil)
	p.PutAttrib(apool.Attribute{
		Key:   "bold",
		Value: "true",
	}, nil)
	cs1, err := CheckRep("Z:2>1*1+1*1=1$x")
	if err != nil {
		t.Error("Error in CheckRep " + err.Error())
		return
	}
	cs2, err := CheckRep("Z:3>0*0|1=3$")
	if err != nil {
		t.Error("Error in CheckRep " + err.Error())
		return
	}
	comp, err := Compose(*cs1, *cs2, &p)
	if err != nil {
		t.Error("Error in ComposeAttributes " + err.Error())
		return
	}
	var cs12, _ = CheckRep(*comp)

	if *cs12 != "Z:2>1+1*0|1=2$x" {
		t.Error("Error in ComposeAttributes")
	}
}

func TestDeserializeOps(t *testing.T) {
	var changesetToCheck = "-1*0=1*1=1=3+4"
	res, err := DeserializeOps(changesetToCheck)
	assert.NoError(t, err)

	if len(*res) != 5 {
		t.Error("too short", len(*res))
	}
}

func TestRegexMatcher2(t *testing.T) {
	input := "-1*0=1*1=1=3+4"
	pattern := `((?:\*[0-9a-z]+)*)(?:\|([0-9a-z]+))?([-+=])([0-9a-z]+)|(.)`
	regex := regexp.MustCompile(pattern)
	var i = regex.FindAllStringSubmatch(input, -1)

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
	var ops, _ = DeserializeOps(input)
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

	cs1, err := CheckRep("Z:2>1*1+1*1=1$x")
	if err != nil {
		t.Error("Error in CheckRep", err)
		return
	}
	cs2, err := CheckRep("Z:3>0*0|1=3$")
	if err != nil {
		t.Error("Error in CheckRep", err)
		return
	}

	Compose(*cs1, *cs2, &pool)

}
