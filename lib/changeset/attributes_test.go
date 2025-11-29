package changeset

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestRejectsInvalidAttributeString(t *testing.T) {
	var testcases = []string{"x", "*0+1", "*A", "*0$" + "*", "0", "*-1"}
	for _, testcase := range testcases {
		_, err := DecodeAttribString(testcase)

		if err == nil {
			t.Error("Expected error, got nil")
		}
	}
}

func TestStringToAttribWithThreeAttributes(t *testing.T) {
	var attribStr = []string{"key1", "value1", "value2"}
	_, err := StringToAttrib(attribStr)

	if err == nil {
		t.Error("Expected error because three attributes are passed, got nil")
	}
}

func TestAcceptsValidAttributeString(t *testing.T) {
	n := 37
	keys := make([]int, n)
	for i := range keys {
		keys[i] = i
	}
	var mappings = map[string][]int{
		"":    {},
		"*0":  {0},
		"*a":  {10},
		"*z":  {35},
		"*10": {36},
		"*0*1*2*3*4*5*6*7*8*9*a*b*c*d*e*f*g*h*i*j*k*l*m*n*o*p*q*r*s*t*u*v*w*x*y*z*10": keys,
	}

	for key, value := range mappings {
		attribs, err := DecodeAttribString(key)
		if err != nil {
			t.Error("Expected nil, got ", err)
		}
		if !slices.Equal(attribs, value) {
			t.Error("Expected ", value, ", got ", attribs)
		}
	}
}

func TestEncodeAttribString(t *testing.T) {
	var res, err = encodeAttribString([]int{0, 1})

	if err != nil {
		t.Error("Expected nil, got ", err)
	}

	if *res != "*0*1" {
		t.Error("Expected *0*1, got ", res)
	}
}

func TestEncodeRejectsInvalidInput(t *testing.T) {
	var testCases = [][]int{
		{-1},
	}

	for _, testCase := range testCases {
		_, err := encodeAttribString(testCase)
		if err == nil {
			t.Error("Expected error, got nil")
		}
	}
}

func TestAcceptsValidAttributeStringInEncode(t *testing.T) {
	n := 37
	keys := make([]int, n)
	for i := range keys {
		keys[i] = i
	}
	var mappings = map[string][]int{
		"":    {},
		"*0":  {0},
		"*a":  {10},
		"*z":  {35},
		"*10": {36},
		"*0*1*2*3*4*5*6*7*8*9*a*b*c*d*e*f*g*h*i*j*k*l*m*n*o*p*q*r*s*t*u*v*w*x*y*z*10": keys,
	}

	for key, value := range mappings {
		attribs, err := encodeAttribString(value)
		if err != nil {
			t.Error("Expected nil, got ", err)
		}
		if *attribs != key {
			t.Error("Expected ", value, ", got ", attribs)
		}
	}
}

func TestRejectsInvalidAttribsFromNums(t *testing.T) {
	var pool, _ = PrepareAttribPool(t)
	var testcases = []int{
		-1,
		9999,
	}

	for _, testcase := range testcases {
		_, err := attribsFromNums([]int{testcase}, pool)
		if err == nil {
			t.Error("Expected error, got nil " + strconv.Itoa(testcase))
		}
	}
}

func TestAcceptsValidInputs(t *testing.T) {
	var pool, attribs = PrepareAttribPool(t)
	var testCases = [][]int{
		{0},
		{1},
		{0, 1},
		{1, 0},
	}

	var testCases2 = [][][]string{
		{attribs[0]},
		{attribs[1]},
		{attribs[0], attribs[1]},
		{attribs[1], attribs[0]},
	}

	for i, testCase := range testCases {
		attrib, err := attribsFromNums(testCase, pool)
		if err != nil {
			t.Error("Expected nil, got ", err)
		}

		for j, attrib := range *attrib {
			var convertedAttrib, _ = StringToAttrib(testCases2[i][j])
			if attrib != *convertedAttrib {
				t.Error("Expected ", testCases2[i][j], ", got ", attrib)
			}
		}
	}
}

func TestReuseExistingPoolEntries(t *testing.T) {
	var pool, attribs = PrepareAttribPool(t)

	var testCases = [][]int{
		{0},
		{1},
		{0, 1},
		{1, 0},
	}

	var testCases2 = [][][]string{
		{attribs[0]},
		{attribs[1]},
		{attribs[0], attribs[1]},
		{attribs[1], attribs[0]},
	}

	for i, testCase := range testCases {
		attribRetrieved, err := attribsFromNums(testCase, pool)

		if err != nil {
			t.Error("Expected nil, got ", err)
		}

		for j, attrib := range *attribRetrieved {
			var convertedAttrib, _ = StringToAttrib(testCases2[i][j])
			if attrib != *convertedAttrib {
				t.Error("Expected ", testCases2[i][j], ", got ", attrib)
			}
		}

		if getPoolSize(t) != len(attribs) {
			t.Error("Expected ", len(attribs)-1, ", got ", getPoolSize(t))
		}
	}
}

func TestDecodeAttribString_ErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
		shouldError   bool
	}{
		{
			name:        "invalid character uppercase letter",
			input:       "*A",
			shouldError: true,
		},
		{
			name:        "invalid character special symbol",
			input:       "*#",
			shouldError: true,
		},
		{
			name:        "invalid character space",
			input:       "* ",
			shouldError: true,
		},
		{
			name:        "invalid character punctuation",
			input:       "*!",
			shouldError: true,
		},
		{
			name:        "asterisk without following characters",
			input:       "*",
			shouldError: true,
		},
		{
			name:        "multiple asterisks",
			input:       "**",
			shouldError: true,
		},
		{
			name:        "asterisk with mixed valid and invalid",
			input:       "*1X",
			shouldError: true,
		},
		{
			name:        "unicode characters",
			input:       "*1ü",
			shouldError: true,
		},
		{
			name:        "negative sign",
			input:       "*-1",
			shouldError: true,
		},
		{
			name:        "plus sign",
			input:       "*+1",
			shouldError: true,
		},
		{
			name:        "decimal point",
			input:       "*1.5",
			shouldError: true,
		},
		{
			name:        "bracket characters",
			input:       "*[",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeAttribString(tt.input)

			if tt.shouldError {
				if err == nil {
					t.Errorf("expected error but got none for input: %s", tt.input)
				}
				if result != nil {
					t.Errorf("expected nil result on error, but got: %v", result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Errorf("expected result but got nil")
				}
			}
		})
	}
}

func TestDecodeAttribString_ValidCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int
	}{
		{
			name:     "single digit",
			input:    "*1",
			expected: []int{1},
		},
		{
			name:     "multiple digits",
			input:    "*123",
			expected: []int{parseInt36("123")},
		},
		{
			name:     "letter a",
			input:    "*a",
			expected: []int{10}, // 'a' in base36 is 10
		},
		{
			name:     "letter z",
			input:    "*z",
			expected: []int{35}, // 'z' in base36 is 35
		},
		{
			name:     "mixed digits and letters",
			input:    "*1a2b",
			expected: []int{parseInt36("1a2b")},
		},
		{
			name:     "multiple attributes",
			input:    "*1*2*3",
			expected: []int{1, 2, 3},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeAttribString(tt.input)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDecodeAttribString_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldError bool
		description string
	}{
		{
			name:        "malformed regex match",
			input:       "*", // Dies könnte einen unvollständigen Match erzeugen
			shouldError: true,
			description: "should trigger invalid match error",
		},

		{
			name:        "base36 overflow",
			input:       "*zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", // Sehr große base36 Zahl
			shouldError: true,
			description: "should trigger parseInt error",
		},

		{
			name:        "extreme base36",
			input:       "*" + strings.Repeat("z", 20), // 20 mal 'z'
			shouldError: true,
			description: "should trigger parseInt overflow error",
		},

		{
			name:        "invalid base36 after asterisk",
			input:       "*{", // Falls regex-Implementierung inkonsistent ist
			shouldError: true,
			description: "should trigger error in parseInt or match validation",
		},

		{
			name:        "empty after asterisk",
			input:       "*\x00", // Null-Byte könnte regex verwirren
			shouldError: true,
			description: "should trigger regex or parsing error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecodeAttribString(tt.input)

			if tt.shouldError {
				if err == nil {
					t.Errorf("%s: expected error but got none for input: %s", tt.description, tt.input)
				}
				if result != nil {
					t.Errorf("expected nil result on error, but got: %v", result)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDecodeAttribString_ParseIntErrors(t *testing.T) {
	tests := []string{
		"*" + strings.Repeat("z", 15),
		"*" + strings.Repeat("9", 20),
	}

	for i, input := range tests {
		t.Run(fmt.Sprintf("parseInt_error_%d", i), func(t *testing.T) {
			result, err := DecodeAttribString(input)

			// Diese sollten einen parseInt Error auslösen
			if err == nil {
				t.Errorf("expected parseInt error for input: %s", input)
			}
			if result != nil {
				t.Errorf("expected nil result on error")
			}
		})
	}
}

func TestDecodeAttribString_RegexEdgeCases(t *testing.T) {
	edgeCases := []string{
		"\x01*1", // Control character before *
		"*\x7F",  // DEL character after *
		"*\xFF",  // Byte with all bits set
	}

	for i, input := range edgeCases {
		t.Run(fmt.Sprintf("regex_edge_%d", i), func(t *testing.T) {
			result, err := DecodeAttribString(input)

			if err != nil {
				t.Logf("Got expected error for edge case: %v", err)
			} else if result != nil {
				t.Logf("Successfully processed edge case, result: %v", result)
			}
		})
	}
}

// Helper function to parse base36 numbers (similar to parseInt(n, 36) in JS)
func parseInt36(s string) int {
	result, _ := strconv.ParseInt(s, 36, 64)
	return int(result)
}
