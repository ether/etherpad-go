package changeset

import (
	"slices"
	"testing"
	"unicode/utf8"
)

func TestNewStringIterator(t *testing.T) {
	str := "Hello, world!"
	si := NewStringIterator(str)
	if si.curIndex != 0 {
		t.Errorf("Expected 0, got %d", si.curIndex)
	}
	if si.newLines != 0 {
		t.Errorf("Expected 0, got %d", si.newLines)
	}

	convertedStr := []rune(str)

	if !slices.Equal(si.str, convertedStr) {
		t.Errorf("Expected %s, got %s", str, string(si.str))
	}
}

func TestStringIterator_Remaining(t *testing.T) {
	str := "Hello, world!"
	si := NewStringIterator(str)
	if si.Remaining() != utf8.RuneCountInString(str) {
		t.Errorf("Expected %d, got %d", utf8.RuneCountInString(str), si.Remaining())
	}
}

func TestStringIterator_AssertRemaining(t *testing.T) {
	str := "Hello, world!"
	si := NewStringIterator(str)
	if si.AssertRemaining(utf8.RuneCountInString(str)) != nil {
		t.Errorf("Expected nil, got error")
	}
	if si.AssertRemaining(utf8.RuneCountInString(str)+1) == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestStringIterator_Take(t *testing.T) {
	str := "Hello, world!"
	si := NewStringIterator(str)
	takenFromSi, err := si.Take(5)
	if err != nil {
		t.Errorf("Expected Hello, got error: %v", err)
	}
	if *takenFromSi != "Hello" {
		t.Errorf("Expected Hello, got %s", *takenFromSi)
	}
	if si.curIndex != 5 {
		t.Errorf("Expected 5, got %d", si.curIndex)
	}
	if si.newLines != 0 {
		t.Errorf("Expected 0, got %d", si.newLines)
	}
}

func TestStringIterator_takeWithTooMany(t *testing.T) {
	str := "Hello, world!"
	si := NewStringIterator(str)
	_, err := si.Take(500)
	if err == nil {
		t.Errorf("Should error when taking too many characters")
	}
}

func TestStringIterator_takeWith0Newline(t *testing.T) {
	str := "Hello, world!\nThis is a test."
	si := NewStringIterator(str)
	_, err := si.Take(15)

	if err != nil {
		t.Errorf("Should error as we take more than first line, got error: %v", err)
	}
	if si.newLines != 0 {
		t.Errorf("Expected 0, got %d", si.newLines)
	}
}

func TestStringIterator_takeWith1Newline(t *testing.T) {
	str := "Hello, world!\nThis is a test."
	si := NewStringIterator(str)
	_, err := si.Take(2)

	if err != nil {
		t.Errorf("Should work as we take only 2 characters, got error: %v", err)
	}
	if si.newLines != 1 {
		t.Errorf("Expected 1, got %d", si.newLines)
	}
}

func TestStringIterator_peekWithTooMany(t *testing.T) {
	str := "Hello, world!"
	si := NewStringIterator(str)
	_, err := si.Peek(500)
	if err == nil {
		t.Errorf("Should error when peeking too many characters")
	}
}

func TestStringIterator_Peek(t *testing.T) {
	str := "Hello, world!"
	si := NewStringIterator(str)
	seekedSi, err := si.Peek(5)
	if err != nil {
		t.Errorf("Expected Hello, got error: %v", err)
	}
	if *seekedSi != "Hello" {
		t.Errorf("Expected Hello, got %s", *seekedSi)
	}
	if si.curIndex != 0 {
		t.Errorf("Expected 0, got %d", si.curIndex)
	}
	if si.newLines != 0 {
		t.Errorf("Expected 0, got %d", si.newLines)
	}
}

func TestStringIterator_Skip(t *testing.T) {
	str := "Hello, world!"
	si := NewStringIterator(str)
	if si.Skip(5) != nil {
		t.Errorf("Expected nil, got error")
	}
	if si.curIndex != 5 {
		t.Errorf("Expected 5, got %d", si.curIndex)
	}
	if si.newLines != 0 {
		t.Errorf("Expected 0, got %d", si.newLines)
	}
}

func TestStringIterator_Skip_Error(t *testing.T) {
	str := "Hello, world!"
	si := NewStringIterator(str)
	if si.Skip(utf8.RuneCountInString(str)+1) == nil {
		t.Errorf("Expected error, got nil")
	}
}
