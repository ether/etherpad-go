package changeset

import (
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
)

func TestNewBuilder(t *testing.T) {
	b := NewBuilder(10)

	if b.oldLen != 10 {
		t.Errorf("expected oldLen 10, got %d", b.oldLen)
	}
	if b.o.OpCode != "" {
		t.Errorf("expected empty OpCode, got %s", b.o.OpCode)
	}
}

func TestBuilder_Keep_WithStringAttribs(t *testing.T) {
	b := NewBuilder(10)
	pool := apool.NewAPool()
	attribStr := "*0"

	b = b.Keep(5, 1, KeepArgs{stringAttribs: &attribStr}, &pool)
	result := b.ToString()

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuilder_Keep_WithApoolAttribs(t *testing.T) {
	b := NewBuilder(10)
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "bold", Value: "true"}, nil)

	attribs := []apool.Attribute{{Key: "bold", Value: "true"}}
	b = b.Keep(5, 1, KeepArgs{apoolAttribs: &attribs}, &pool)
	result := b.ToString()

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuilder_Keep_WithNilAttribs(t *testing.T) {
	b := NewBuilder(10)
	pool := apool.NewAPool()

	b = b.Keep(5, 0, KeepArgs{}, &pool)
	result := b.ToString()

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuilder_KeepText(t *testing.T) {
	b := NewBuilder(10)
	pool := apool.NewAPool()
	text := "hello\nworld"

	b = b.KeepText(text, KeepArgs{}, &pool)
	result := b.ToString()

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuilder_KeepText_WithAttribs(t *testing.T) {
	b := NewBuilder(10)
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "italic", Value: "true"}, nil)

	attribs := []apool.Attribute{{Key: "italic", Value: "true"}}
	b = b.KeepText("test", KeepArgs{apoolAttribs: &attribs}, &pool)
	result := b.ToString()

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuilder_Insert(t *testing.T) {
	b := NewBuilder(10)
	pool := apool.NewAPool()

	b = b.Insert("new text", KeepArgs{}, &pool)
	result := b.ToString()

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuilder_Insert_WithNewlines(t *testing.T) {
	b := NewBuilder(0)
	pool := apool.NewAPool()

	b = b.Insert("line1\nline2\nline3", KeepArgs{}, &pool)
	result := b.ToString()

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuilder_Insert_WithAttribs(t *testing.T) {
	b := NewBuilder(5)
	pool := apool.NewAPool()
	pool.PutAttrib(apool.Attribute{Key: "author", Value: "user1"}, nil)

	attribs := []apool.Attribute{{Key: "author", Value: "user1"}}
	b = b.Insert("inserted", KeepArgs{apoolAttribs: &attribs}, &pool)
	result := b.ToString()

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuilder_Remove(t *testing.T) {
	b := NewBuilder(20)

	b = b.Remove(5, 0)
	result := b.ToString()

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuilder_Remove_WithLines(t *testing.T) {
	b := NewBuilder(20)

	b = b.Remove(10, 2)
	result := b.ToString()

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuilder_ChainedOperations(t *testing.T) {
	b := NewBuilder(10)
	pool := apool.NewAPool()

	b = b.Keep(5, 0, KeepArgs{}, &pool).
		Insert("test", KeepArgs{}, &pool).
		Remove(3, 0).
		Keep(2, 0, KeepArgs{}, &pool)

	result := b.ToString()
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuilder_ToString_EmptyBuilder(t *testing.T) {
	b := NewBuilder(0)
	result := b.ToString()

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuilder_MultipleInserts(t *testing.T) {
	b := NewBuilder(0)
	pool := apool.NewAPool()

	b = b.Insert("first ", KeepArgs{}, &pool).
		Insert("second ", KeepArgs{}, &pool).
		Insert("third", KeepArgs{}, &pool)

	result := b.ToString()
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBuilder_MixedOperations(t *testing.T) {
	b := NewBuilder(15)
	pool := apool.NewAPool()
	attribStr := "*0"

	b = b.Keep(5, 1, KeepArgs{stringAttribs: &attribStr}, &pool).
		Insert("new\n", KeepArgs{}, &pool).
		Remove(3, 0).
		KeepText("kept", KeepArgs{}, &pool).
		Remove(2, 1)

	result := b.ToString()
	if result == "" {
		t.Error("expected non-empty result")
	}
}
