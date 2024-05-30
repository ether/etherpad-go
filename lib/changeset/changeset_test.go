package changeset

import "testing"

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
