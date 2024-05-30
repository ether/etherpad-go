package changeset

import "testing"

func TestNewStringAssembler(t *testing.T) {
	sa := NewStringAssembler()
	if sa.str != "" {
		t.Errorf("Expected \"\", got %s", sa.str)
	}
}

func TestStringAssembler_Append(t *testing.T) {
	sa := NewStringAssembler()
	sa.Append("Hello")
	if sa.str != "Hello" {
		t.Errorf("Expected Hello, got %s", sa.str)
	}
}

func TestStringAssembler_String(t *testing.T) {
	sa := NewStringAssembler()
	sa.Append("Hello")
	if sa.String() != "Hello" {
		t.Errorf("Expected Hello, got %s", sa.String())
	}
}

func TestStringAssembler_Clear(t *testing.T) {
	sa := NewStringAssembler()
	sa.Append("Hello")
	sa.Clear()
	if sa.str != "" {
		t.Errorf("Expected \"\", got %s", sa.str)
	}
}
