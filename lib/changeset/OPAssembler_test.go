package changeset

import "testing"

func TestOpAssembler_Append(t *testing.T) {
	var teststring = "a\nb\nc\n"
	var ops = OpsFromText("=", teststring[0:5], nil, nil)

	var oa = NewOpAssembler()
	oa.Append(ops[0])
	if oa.String() != "|2=4" {
		t.Error("Expected |2=4, got ", oa.String())
	}

	oa.Append(ops[1])
	if oa.String() != "|2=4=1" {
		t.Error("Expected |2=4|1, got ", oa.String())
	}
}

func TestOpAssembler_Clear(t *testing.T) {
	var teststring = "a\nb\nc\n"
	var ops = OpsFromText("=", teststring[0:5], nil, nil)

	var oa = NewOpAssembler()
	oa.Append(ops[0])
	oa.Clear()
	if oa.String() != "" {
		t.Error("Expected \"\", got ", oa.String())
	}
}

func TestOpAssembler_String(t *testing.T) {
	var teststring = "a\nb\nc\n"
	var ops = OpsFromText("=", teststring[0:5], nil, nil)

	var oa = NewOpAssembler()
	oa.Append(ops[0])
	if oa.String() != "|2=4" {
		t.Error("Expected |2=4, got ", oa.String())
	}
}

func TestOpAssembler_Append2(t *testing.T) {
	const x = "-c*3*4+6|3=az*asdf0*1*2*3+1=1-1+1*0+1=1-1+1|c=c-1"
	var ops, err = DeserializeOps(x)

	if err != nil {
		t.Error(err)
	}

	if len(*ops) != 13 {
		t.Error("Expected 1, got ", len(*ops))
	}

	var firstOp = (*ops)[0]
	if firstOp.OpCode != "-" && firstOp.Chars != 12 && firstOp.Lines != 0 && firstOp.Attribs != "" {
		t.Error("Expected -, got ", firstOp.OpCode)
	}

	var secondOp = (*ops)[1]

	if secondOp.OpCode != "+" && secondOp.Chars != 6 && secondOp.Lines != 0 && secondOp.Attribs != "*3*4" {
		t.Error("Expected +, got ", secondOp.OpCode)
	}

	var thirdOp = (*ops)[2]
	if thirdOp.OpCode != "=" && thirdOp.Chars != 395 && thirdOp.Lines != 3 && thirdOp.Attribs != "" {
		t.Error("Expected =, got ", thirdOp.OpCode)
	}
}
