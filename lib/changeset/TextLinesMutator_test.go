package changeset

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func applyMutations(mu *TextLinesMutator, arrayOfArrays [][]interface{}) {
	for _, a := range arrayOfArrays {
		if len(a) == 0 {
			continue
		}
		action, ok := a[0].(string)
		if !ok {
			continue
		}

		switch action {
		case "insert":
			if len(a) < 3 {
				continue
			}
			s, _ := a[1].(string)
			n, _ := toInt(a[2])
			_ = mu.Insert(s, n) // Fehler wird hier nicht propagiert

		case "remove":
			if len(a) < 3 {
				continue
			}
			chars, _ := toInt(a[1])
			lines, _ := toInt(a[2])
			mu.Remove(chars, lines)

		case "skip":
			if len(a) < 4 {
				continue
			}
			chars, _ := toInt(a[1])
			lines, _ := toInt(a[2])
			flag, _ := a[3].(bool)
			mu.Skip(chars, lines, flag)

		default:
			// unbekannte Aktion überspringen
		}
	}
}

// toInt konvertiert gängige numerische Typen zu int (robuste Typumwandlung für Tests)
func toInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint:
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	case float32:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}

func mutationsChangeset(oldLen int, arrayOfArrays [][]interface{}) string {
	assem := NewSmartOpAssembler()
	op := NewOp(nil)
	bank := NewStringAssembler()
	oldPos := 0
	newLen := 0
	for _, a := range arrayOfArrays {
		if a[0] == "skip" {
			op.OpCode = "="
			op.Chars = a[1].(int)
			op.Lines = a[2].(int)
			assem.Append(op)
			oldPos += op.Chars
			newLen += op.Chars
		} else if a[0] == "remove" {
			op.OpCode = "-"
			op.Chars = a[1].(int)
			op.Lines = a[2].(int)
			assem.Append(op)
			oldPos += op.Chars
		} else if a[0] == "insert" {
			op.OpCode = "+"
			bank.Append(a[1].(string))
			op.Chars = len((a[1]).(string))
			op.Lines = (a[2]).(int)
			assem.Append(op)
			newLen += op.Chars
		}
	}
	newLen += oldLen - oldPos
	assem.EndDocument()
	return Pack(oldLen, newLen, assem.String(), bank.String())
}

func runMutationTest(t *testing.T, testId int, origLines []string, muts [][]interface{}, correct []string) {
	t.Run(fmt.Sprintf("runMutationTest#%d", testId), func(t *testing.T) {

		lines := copyLines(origLines)
		mu := NewTextLinesMutator(&lines)
		applyMutations(mu, muts)
		mu.Close()
		if !reflect.DeepEqual(lines, correct) {
			t.Errorf("mutator result mismatch:\nexpected: %v\nactual:   %v", correct, lines)
		}

		inText := strings.Join(origLines, "")
		cs := mutationsChangeset(len(inText), muts)

		lines = copyLines(origLines)

		MutateTextLines(cs, &lines)
		if !reflect.DeepEqual(lines, correct) {
			t.Errorf("mutateTextLines result mismatch:\nexpected: %v\nactual:   %v", correct, lines)
		}

		correctText := strings.Join(correct, "")

		outText, err := ApplyToText(cs, inText)
		if err != nil {
			t.Errorf("ApplyToText returned error: %v", err)
		}
		if *outText != correctText {
			t.Errorf("applyToText result mismatch:\nexpected: %q\nactual:   %q", correctText, *outText)
		}
	})
}

func TestMutations(t *testing.T) {
	orig := []string{"apple\n", "banana\n", "cabbage\n", "duffle\n", "eggplant\n"}

	muts := [][]interface{}{
		{"remove", 1, 0, "a"},
		{"insert", "tu", 0},
		{"remove", 1, 0, "p"},
		{"skip", 4, 1, false},
		{"skip", 7, 1, false},
		{"insert", "cream\npie\n", 2},
		{"skip", 2, 0, false},
		{"insert", "bot", 0},
		{"insert", "\n", 1},
		{"insert", "bu", 0},
		{"skip", 3, 0, false},
		{"remove", 3, 1, "ge\n"},
		{"remove", 6, 0, "duffle"},
	}

	correct := []string{
		"tuple\n",
		"banana\n",
		"cream\n",
		"pie\n",
		"cabot\n",
		"bubba\n",
		"eggplant\n",
	}

	runMutationTest(t, 1, orig, muts, correct)
}

func TestMutations2(t *testing.T) {
	orig := []string{"apple\n", "banana\n", "cabbage\n", "duffle\n", "eggplant\n"}
	muts := [][]interface{}{
		{"remove", 1, 0, "a"},
		{"remove", 1, 0, "p"},
		{"insert", "tu", 0},
		{"skip", 11, 2, false},
		{"insert", "cream\npie\n", 2},
		{"skip", 2, 0, false},
		{"insert", "bot", 0},
		{"insert", "\n", 1},
		{"insert", "bu", 0},
		{"skip", 3, 0, false},
		{"remove", 3, 1, "ge\n"},
		{"remove", 6, 0, "duffle"},
	}
	correct := []string{
		"tuple\n",
		"banana\n",
		"cream\n",
		"pie\n",
		"cabot\n",
		"bubba\n",
		"eggplant\n",
	}
	runMutationTest(t, 2, orig, muts, correct)
}

func TestMutations3(t *testing.T) {
	orig := []string{"apple\n", "banana\n", "cabbage\n", "duffle\n", "eggplant\n"}
	muts := [][]interface{}{
		{"remove", 6, 1, "apple\n"},
		{"skip", 15, 2, false},
		{"skip", 6, 0, false},
		{"remove", 1, 1, "\n"},
		{"remove", 8, 0, "eggplant"},
		{"skip", 1, 1, false},
	}
	correct := []string{"banana\n", "cabbage\n", "duffle\n"}
	runMutationTest(t, 3, orig, muts, correct)
}

func TestMutations4(t *testing.T) {
	orig := []string{"15\n"}
	muts := [][]interface{}{
		{"skip", 1, 0, false},
		{"insert", "\n2\n3\n4\n", 4},
		{"skip", 2, 1, false},
	}
	correct := []string{"1\n", "2\n", "3\n", "4\n", "5\n"}
	runMutationTest(t, 4, orig, muts, correct)
}

func TestMutations5(t *testing.T) {
	orig := []string{"1\n", "2\n", "3\n", "4\n", "5\n"}
	muts := [][]interface{}{
		{"skip", 1, 0, false},
		{"remove", 7, 4, "\n2\n3\n4\n"},
		{"skip", 2, 1, false},
	}
	correct := []string{"15\n"}
	runMutationTest(t, 5, orig, muts, correct)
}

func TestMutations6(t *testing.T) {
	orig := []string{"123\n", "abc\n", "def\n", "ghi\n", "xyz\n"}
	muts := [][]interface{}{
		{"insert", "0", 0},
		{"skip", 4, 1, false},
		{"skip", 4, 1, false},
		{"remove", 8, 2, "def\nghi\n"},
		{"skip", 4, 1, false},
	}
	correct := []string{"0123\n", "abc\n", "xyz\n"}
	runMutationTest(t, 6, orig, muts, correct)
}

func TestMutations7(t *testing.T) {
	orig := []string{"apple\n", "banana\n", "cabbage\n", "duffle\n", "eggplant\n"}
	muts := [][]interface{}{
		{"remove", 6, 1, "apple\n"},
		{"skip", 15, 2, true},
		{"skip", 6, 0, true},
		{"remove", 1, 1, "\n"},
		{"remove", 8, 0, "eggplant"},
		{"skip", 1, 1, true},
	}
	correct := []string{"banana\n", "cabbage\n", "duffle\n"}
	runMutationTest(t, 7, orig, muts, correct)
}

func TestMutatorHasMore(t *testing.T) {
	lines := []string{"1\n", "2\n", "3\n", "4\n"}
	var mu *TextLinesMutator

	// Test case 1: Skip all lines
	copiedLines := copyLines(lines)
	mu = NewTextLinesMutator(&copiedLines)
	if !mu.HasMore() {
		t.Error("Expected HasMore to be true initially")
	}
	mu.Skip(8, 4, false)
	if mu.HasMore() {
		t.Error("Expected HasMore to be false after skipping all lines")
	}
	mu.Close()
	if mu.HasMore() {
		t.Error("Expected HasMore to be false after close")
	}

	// Test case 2: Remove and skip operations
	// still 1,2,3,4
	copiedLines = copyLines(lines)
	mu = NewTextLinesMutator(&copiedLines)
	if !mu.HasMore() {
		t.Error("Expected HasMore to be true initially")
	}
	mu.Remove(2, 1)
	if !mu.HasMore() {
		t.Error("Expected HasMore to be true after first remove")
	}
	mu.Skip(2, 1, false)
	if !mu.HasMore() {
		t.Error("Expected HasMore to be true after first skip")
	}
	mu.Skip(2, 1, false)
	if !mu.HasMore() {
		t.Error("Expected HasMore to be true after second skip")
	}
	mu.Skip(2, 1, false)
	if mu.HasMore() {
		t.Error("Expected HasMore to be false after third skip")
	}
	err := mu.Insert("5\n", 1)
	if err != nil {
		t.Errorf("Insert failed: %v", err)
	}
	if mu.HasMore() {
		t.Error("Expected HasMore to be false after insert")
	}
	mu.Close()
	if mu.HasMore() {
		t.Error("Expected HasMore to be false after close")
	}

	// Test case 3: Multiple removes and insert
	// Lines are now 2,3,4,5 from previous test
	resultLines := mu.GetLines()
	copiedLines = copyLines(resultLines)
	mu = NewTextLinesMutator(&copiedLines)
	if !mu.HasMore() {
		t.Error("Expected HasMore to be true initially")
	}
	mu.Remove(6, 3)
	if !mu.HasMore() {
		t.Error("Expected HasMore to be true after first remove")
	}
	mu.Remove(2, 1)
	if mu.HasMore() {
		t.Error("Expected HasMore to be false after second remove")
	}
	err = mu.Insert("hello\n", 1)
	if err != nil {
		t.Errorf("Insert failed: %v", err)
	}
	if mu.HasMore() {
		t.Error("Expected HasMore to be false after insert")
	}
	mu.Close()
	if mu.HasMore() {
		t.Error("Expected HasMore to be false after close")
	}
}

// copyLines creates a copy of the lines slice to ensure test isolation
func copyLines(lines []string) []string {
	result := make([]string, len(lines))
	copy(result, lines)
	return result
}
