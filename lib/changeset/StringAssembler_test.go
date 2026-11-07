package changeset

import "testing"

func TestNewStringAssembler(t *testing.T) {
	sa := NewStringAssembler()
	if sa.String() != "" {
		t.Errorf("Expected \"\", got %s", sa.String())
	}
}

func TestStringAssembler_Append(t *testing.T) {
	sa := NewStringAssembler()
	sa.Append("Hello")
	if sa.String() != "Hello" {
		t.Errorf("Expected Hello, got %s", sa.String())
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
	if sa.String() != "" {
		t.Errorf("Expected \"\", got %s", sa.String())
	}
}
