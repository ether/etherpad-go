package changeset

import (
	"github.com/ether/etherpad-go/lib/apool"
	"testing"
)

func TestSmartOpAssembler_Append(t *testing.T) {
	var teststring = "a\nb\nc\n"
	var equals = OpsFromText("=", teststring[0:5], nil, nil)
	var deletes = OpsFromText("-", "", nil, nil)
	var adds = OpsFromText("+", "def", nil, nil)

	var soa = NewSmartOpAssembler()
	var totals = append(append(equals, deletes...), adds...)
	for _, op := range totals {
		soa.Append(op)
	}

	soa.EndDocument()
	var res = soa.String()

	if res != "|2=4=1+3" {
		t.Error("Expected |2=4=1+3, got ", res)
	}
}

func TestSmartOpAssembler_AppendBaseline(t *testing.T) {
	var x = "-c*3*4+6|3=az*asdf0*1*2*3+1=1-1+1*0+1=1-1+1|c=c-1"
	var smartOps = NewSmartOpAssembler()
	ops, _ := DeserializeOps(x)

	for _, op := range *ops {
		smartOps.Append(op)
	}

	smartOps.EndDocument()

	if smartOps.String() != x {
		t.Error("Expected ", x, ", got ", smartOps.String())
	}
}

func TestSmartOpAssembler_Merge_ConsecutiveOps(t *testing.T) {
	var x = "-c-6-1-9=5"
	var assembler = NewSmartOpAssembler()
	ops, _ := DeserializeOps(x)
	for _, op := range *ops {
		assembler.Append(op)
	}
	assembler.EndDocument()

	if assembler.String() != "-s" {
		t.Error("Expected -c-16=5, got ", assembler.String())
	}
}

func TestSmartOpAssembler_Merge_ConsecutiveOps2(t *testing.T) {
	var x = "-c-6|1-1|9-f-k=5"
	var assembler = NewSmartOpAssembler()
	ops, _ := DeserializeOps(x)
	for _, op := range *ops {
		assembler.Append(op)
	}
	assembler.EndDocument()
	if assembler.String() != "|a-y-k" {
		t.Error("Expected -c-16=5, got ", assembler.String())
	}
}

func TestSmartOpAssemblerMergeConsecutiveEqualWithMultiline(t *testing.T) {
	const x = "-c*3*4=6*2*4|1=1*3*4|9=f*3*4|2=2*3*4=a*3*4=1=k=5"
	var assembler = NewSmartOpAssembler()
	ops, _ := DeserializeOps(x)
	for _, op := range *ops {
		assembler.Append(op)
	}

	assembler.EndDocument()
	if assembler.String() != "-c*3*4=6*2*4|1=1*3*4|b=h*3*4=b" {
		t.Error("Expected -c*3*4=6*2*4|1=1*3*4|b=h*3*4=b, got ", assembler.String())
	}
}

func TestSmartOpAssemlerIgnorePlusOpsWithOpsChars0(t *testing.T) {
	const x = "-c*3*4+6*3*4+0*3*4+1+0*3*4+1"
	var assembler = NewSmartOpAssembler()
	ops, _ := DeserializeOps(x)
	for _, op := range *ops {
		assembler.Append(op)
	}

	assembler.EndDocument()
	if assembler.String() != "-c*3*4+8" {
		t.Error("Expected -c*3*4+8, got ", assembler.String())
	}
}

func TestSmartOpAssembler_Merge_Consecutive_Equals_Ops_Without_Multiline(t *testing.T) {
	var x = "-c*3*4=6*2*4=1*3*4=f*3*4=2*3*4=a=k=5"
	var assembler = NewSmartOpAssembler()
	ops, _ := DeserializeOps(x)
	for _, op := range *ops {
		assembler.Append(op)
	}
	assembler.EndDocument()
	if assembler.String() != "-c*3*4=6*2*4=1*3*4=r" {
		t.Error("Expected -c*3*4=6*2*4=1*3*4=r, got ", assembler.String())
	}
}

func TestSmartAssembler_Ignore_Minus_Ops_With_Ops_Chars0(t *testing.T) {
	var x = "-c-6-0-1-0-1"
	var assembler = NewSmartOpAssembler()
	ops, _ := DeserializeOps(x)
	for _, op := range *ops {
		assembler.Append(op)
	}
	assembler.EndDocument()
	if assembler.String() != "-k" {
		t.Error("Expected -k, got ", assembler.String())
	}
}

func TestSmartAssembler_Clear_Should_Empty_Internal_Assembler(t *testing.T) {
	const x = "-c*3*4+6|3=az*asdf0*1*2*3+1=1-1+1*0+1=1-1+1|c=c-1"

	var ops, err = DeserializeOps(x)
	if err != nil {
		t.Error(err)
	}

	var assembler = NewSmartOpAssembler()
	var opsExtracted = *ops
	nextElement, opsExtracted := opsExtracted[0], opsExtracted[1:]
	assembler.Append(nextElement)
	nextElement, opsExtracted = opsExtracted[0], opsExtracted[1:]
	assembler.Append(nextElement)
	nextElement, opsExtracted = opsExtracted[0], opsExtracted[1:]
	assembler.Append(nextElement)
	assembler.Clear()
	nextElement, opsExtracted = opsExtracted[0], opsExtracted[1:]
	assembler.Append(nextElement)
	nextElement, opsExtracted = opsExtracted[0], opsExtracted[1:]
	assembler.Append(nextElement)
	assembler.Clear()

	for _, op := range opsExtracted {
		assembler.Append(op)
	}

	assembler.EndDocument()
	if assembler.String() != "-1+1*0+1=1-1+1|c=c-1" {
		t.Error("Expected -1+1*0+1=1-1+1|c=c-1, got ", assembler.String())
	}
}

func testAppendATextToAssembler(t *testing.T, testId int, atext apool.AText, correctOps string) {
	var assembler = NewSmartOpAssembler()
	var ops = OpsFromAText(atext)

	for _, op := range *ops {
		assembler.Append(op)
	}
	if assembler.String() != correctOps {
		t.Errorf("Test %d: Expected %s, got %s", testId, correctOps, assembler.String())
	}
}

func TestAppendATextToAssembler(t *testing.T) {
	testAppendATextToAssembler(t, 1, apool.AText{
		Text:    "\n",
		Attribs: "|1+1",
	}, "")
	testAppendATextToAssembler(t, 2, apool.AText{
		Text:    "\n\n",
		Attribs: "|2+2",
	}, "|1+1")
	testAppendATextToAssembler(t, 3, apool.AText{
		Text:    "\n\n",
		Attribs: "*x|2+2",
	}, "*x|1+1")
	testAppendATextToAssembler(t, 4, apool.AText{
		Text:    "\n\n",
		Attribs: "*x|1+1|1+1",
	}, "*x|1+1")
	testAppendATextToAssembler(t, 5, apool.AText{
		Text:    "foo\n",
		Attribs: "|1+4",
	}, "+3")
	testAppendATextToAssembler(t, 6, apool.AText{
		Text:    "\nfoo\n",
		Attribs: "|2+5",
	}, "|1+1+3")
	testAppendATextToAssembler(t, 7, apool.AText{
		Text:    "\nfoo\n",
		Attribs: "*x|2+5",
	}, "*x|1+1*x+3")
	testAppendATextToAssembler(t, 8, apool.AText{
		Text:    "\n\n\nfoo\n",
		Attribs: "|2+2*x|2+5",
	}, "|2+2*x|1+1*x+3")
}
