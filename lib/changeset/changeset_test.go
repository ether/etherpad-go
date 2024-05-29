package changeset

import "testing"

func TestMakeSplice(t *testing.T) {
	var testString = "a\nb\nc\n"
	var splicedText, _ = MakeSplice(testString, 5, 0, "def", nil, nil)
	var t2, err = ApplyToText(splicedText, testString)
	if err != nil {
		t.Error(err)
	}
	if *t2 != "a\nb\ndef\nc\n" {
		t.Error("Expected a\nb\ndef\nc\n, got ", *t2)
	}
}
