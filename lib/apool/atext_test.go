package apool

import "testing"

func TestATextsEqual(t *testing.T) {
	var atexts = map[bool][]AText{
		true: {
			{Text: "Hello", Attribs: "bold"},
			{Text: "Hello", Attribs: "bold"},
		},
		false: {
			{Text: "Hello", Attribs: "bold"},
			{Text: "Hello", Attribs: "italic"},
		},
	}
	for isEqual, aTextsToCompare := range atexts {
		result := ATextsEqual(aTextsToCompare[0], aTextsToCompare[1])
		if result != isEqual {
			t.Errorf("Expected equality: %v, got: %v", isEqual, result)
		}
	}
}
