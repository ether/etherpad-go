package changeset

import (
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
