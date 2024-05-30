package changeset

import "testing"

func TestMergingOpAssembler_Append(t *testing.T) {
	var teststring = "a\nb\nc\n"
	var equals = OpsFromText("=", teststring[0:5], nil, nil)
	var deletes = OpsFromText("-", "", nil, nil)
	var adds = OpsFromText("+", "def", nil, nil)

	var moa = NewMergingOpAssembler()
	var totals = append(append(equals, deletes...), adds...)
	for _, op := range totals {
		moa.Append(op)
	}

	moa.EndDocument()
	var res = moa.String()

	if res != "|2=4=1+3" {
		t.Error("Expected |2=4=1+3, got ", res)
	}
}

func TestMergingOpAssembler_EndDocument(t *testing.T) {
	var teststring = "a\nb\nc\n"
	var equals = OpsFromText("=", teststring[0:5], nil, nil)
	var deletes = OpsFromText("-", "", nil, nil)
	var adds = OpsFromText("+", "def", nil, nil)

	var moa = NewMergingOpAssembler()
	var totals = append(append(equals, deletes...), adds...)
	for _, op := range totals {
		moa.Append(op)
	}

	moa.EndDocument()
	var res = moa.String()

	if res != "|2=4=1+3" {
		t.Error("Expected |2=4=1+3, got ", res)
	}
}

func TestMergingOpAssembler_String(t *testing.T) {
	var teststring = "a\nb\nc\n"
	var equals = OpsFromText("=", teststring[0:5], nil, nil)
	var deletes = OpsFromText("-", "", nil, nil)
	var adds = OpsFromText("+", "def", nil, nil)

	var moa = NewMergingOpAssembler()
	var totals = append(append(equals, deletes...), adds...)
	for _, op := range totals {
		moa.Append(op)
	}

	moa.EndDocument()
	var res = moa.String()

	if res != "|2=4=1+3" {
		t.Error("Expected |2=4=1+3, got ", res)
	}
}
