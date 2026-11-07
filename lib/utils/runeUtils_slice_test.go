package utils

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func refRuneSlice(s string, start, end int) string {
	r := []rune(s)
	if start < 0 {
		start = 0
	}
	if start > len(r) {
		start = len(r)
	}
	if end < start {
		end = start
	}
	if end > len(r) {
		end = len(r)
	}
	return string(r[start:end])
}

func TestRuneSliceMatchesRuneConversion(t *testing.T) {
	inputs := []string{"", "abc", "aä€😀b\nc", strings.Repeat("abcdefgh", 5) + "ü" + strings.Repeat("xyz", 7), strings.Repeat("😀", 20)}
	for _, s := range inputs {
		n := len([]rune(s))
		for start := -2; start <= n+2; start++ {
			for end := -2; end <= n+2; end++ {
				if got, want := RuneSlice(s, start, end), refRuneSlice(s, start, end); got != want {
					t.Fatalf("RuneSlice(%q, %d, %d) = %q, want %q", s, start, end, got, want)
				}
			}
		}
	}
}

func TestRuneCountMatchesUTF8(t *testing.T) {
	inputs := []string{"", "a", "abcdefgh", "abcdefghi", "aä€😀b\nc", strings.Repeat("abcdefg", 9) + "ü" + strings.Repeat("x", 13),
		"\xff\xfeabc", "abc\x80defghijk", strings.Repeat("😀a", 11), "abcdefg\xe2\x82"}
	for _, s := range inputs {
		if got, want := RuneCount(s), utf8.RuneCountInString(s); got != want {
			t.Fatalf("RuneCount(%q) = %d, want %d", s, got, want)
		}
	}
}

func BenchmarkRuneCount(b *testing.B) {
	s := strings.Repeat("lorem ipsum dolor sit amet ", 40000)
	b.SetBytes(int64(len(s)))
	for i := 0; i < b.N; i++ {
		RuneCount(s)
	}
}

func BenchmarkRuneCountStdlib(b *testing.B) {
	s := strings.Repeat("lorem ipsum dolor sit amet ", 40000)
	b.SetBytes(int64(len(s)))
	for i := 0; i < b.N; i++ {
		utf8.RuneCountInString(s)
	}
}
