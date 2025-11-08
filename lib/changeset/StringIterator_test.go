package changeset

import (
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
	if si.str != str {
		t.Errorf("Expected %s, got %s", str, si.str)
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
	if si.Take(5) != "Hello" {
		t.Errorf("Expected Hello, got %s", si.Take(5))
	}
	if si.curIndex != 5 {
		t.Errorf("Expected 5, got %d", si.curIndex)
	}
	if si.newLines != 0 {
		t.Errorf("Expected 0, got %d", si.newLines)
	}
}

func TestStringIterator_Peek(t *testing.T) {
	str := "Hello, world!"
	si := NewStringIterator(str)
	if si.Peek(5) != "Hello" {
		t.Errorf("Expected Hello, got %s", si.Peek(5))
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
