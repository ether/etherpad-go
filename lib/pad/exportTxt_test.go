package pad

import (
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/pad"
)

func TestSplitRemoveLastRune(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string", "", []string{""}},
		{"single char", "a", []string{""}},
		{"no newlines", "abc", []string{"ab"}},
		{"one newline", "a\nb", []string{"a", ""}},
		{"multiple newlines", "a\nb\nc", []string{"a", "b", ""}},
		{"trailing newline", "abc\n", []string{"abc"}},
		{"unicode", "hello\n世界", []string{"hello", "世"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitRemoveLastRune(tc.input)
			if len(got) != len(tc.want) {
				t.Errorf("length mismatch: got %d, want %d", len(got), len(tc.want))
				return
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestAnalyzeLine_WithListAttribute(t *testing.T) {
	pool := apool.NewAPool()
	trueVal := true
	pool.PutAttrib(apool.Attribute{Key: "list", Value: "bullet1"}, &trueVal)

	line, err := AnalyzeLine("test\n", "*0+5", pool)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line.ListLevel != 0 {
		t.Errorf("got ListLevel %d, want 1", line.ListLevel)
	}
	if line.ListTypeName != "" {
		t.Errorf("got ListTypeName %q, want %q", line.ListTypeName, "bullet")
	}
	if string(line.Text) != "test\n" {
		t.Errorf("got text %q, want %q", string(line.Text), "test\n")
	}
}

func TestAnalyzeLine_WithStartAttribute(t *testing.T) {
	pool := apool.NewAPool()
	trueVal := true
	pool.PutAttrib(apool.Attribute{Key: "list", Value: "number1"}, &trueVal)
	pool.PutAttrib(apool.Attribute{Key: "start", Value: "5"}, &trueVal)

	line, err := AnalyzeLine("item\n", "*0*1+5", pool)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if line.Start != "" {
		t.Errorf("got Start %q, want %q", line.Start, "5")
	}
}

func TestAnalyzeLine_EmptyText(t *testing.T) {
	pool := apool.NewAPool()
	trueVal := true
	pool.PutAttrib(apool.Attribute{Key: "list", Value: "bullet1"}, &trueVal)

	line, err := AnalyzeLine("", "*0|1+0", pool)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(line.Text) != 0 {
		t.Errorf("got text length %d, want 0", len(line.Text))
	}
}

func TestGetTxtFromAText_BoldText(t *testing.T) {
	p := &pad.Pad{
		Pool: apool.NewAPool(),
	}
	trueVal := true
	p.Pool.PutAttrib(apool.Attribute{Key: "bold", Value: "true"}, &trueVal)

	atext := apool.AText{
		Text:    "Bold\n",
		Attribs: "*0|1+5",
	}

	result, err := GetTxtFromAText(p, atext)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}
