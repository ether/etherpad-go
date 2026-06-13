package hooks

import (
	"testing"

	"github.com/ether/etherpad-go/lib/hooks/events"
)

func TestExecuteHooksRunsInRegistrationOrder(t *testing.T) {
	h := NewHook()
	var order []string
	h.EnqueueHook("k", func(ctx any) { order = append(order, "a") })
	h.EnqueueHook("k", func(ctx any) { order = append(order, "b") })
	h.EnqueueHook("k", func(ctx any) { order = append(order, "c") })

	h.ExecuteHooks("k", nil)

	if len(order) != 3 || order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Fatalf("expected registration order [a b c], got %v", order)
	}
}

func TestDequeueHookRemovesEntryAndPreservesOrder(t *testing.T) {
	h := NewHook()
	var order []string
	h.EnqueueHook("k", func(ctx any) { order = append(order, "a") })
	id := h.EnqueueHook("k", func(ctx any) { order = append(order, "b") })
	h.EnqueueHook("k", func(ctx any) { order = append(order, "c") })

	h.DequeueHook("k", id)
	h.ExecuteHooks("k", nil)

	if len(order) != 2 || order[0] != "a" || order[1] != "c" {
		t.Fatalf("expected [a c] after dequeue, got %v", order)
	}
}

func TestDequeueHookRemovesFirstElement(t *testing.T) {
	h := NewHook()
	var order []string
	id := h.EnqueueHook("k", func(ctx any) { order = append(order, "a") })
	h.EnqueueHook("k", func(ctx any) { order = append(order, "b") })
	h.EnqueueHook("k", func(ctx any) { order = append(order, "c") })

	h.DequeueHook("k", id)
	h.ExecuteHooks("k", nil)

	if len(order) != 2 || order[0] != "b" || order[1] != "c" {
		t.Fatalf("expected [b c] after removing first, got %v", order)
	}
}

func TestDequeueHookRemovesLastElement(t *testing.T) {
	h := NewHook()
	var order []string
	h.EnqueueHook("k", func(ctx any) { order = append(order, "a") })
	h.EnqueueHook("k", func(ctx any) { order = append(order, "b") })
	id := h.EnqueueHook("k", func(ctx any) { order = append(order, "c") })

	h.DequeueHook("k", id)
	h.ExecuteHooks("k", nil)

	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Fatalf("expected [a b] after removing last, got %v", order)
	}
}

func TestDequeueHookUnknownIdIsNoOp(t *testing.T) {
	h := NewHook()
	var order []string
	h.EnqueueHook("k", func(ctx any) { order = append(order, "a") })
	h.EnqueueHook("k", func(ctx any) { order = append(order, "b") })

	h.DequeueHook("k", "does-not-exist")
	h.ExecuteHooks("k", nil)

	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Fatalf("expected [a b] unchanged after unknown-id dequeue, got %v", order)
	}
}

func TestPadCreateTypedWrapperDeliversContext(t *testing.T) {
	h := NewHook()
	var gotPadId, gotAuthor string
	h.EnqueuePadCreateHook(func(ctx *events.PadCreateContext) {
		gotPadId = ctx.PadId
		gotAuthor = ctx.AuthorId
	})

	h.ExecutePadCreateHooks(&events.PadCreateContext{PadId: "p1", AuthorId: "a1"})

	if gotPadId != "p1" || gotAuthor != "a1" {
		t.Fatalf("expected (p1,a1), got (%s,%s)", gotPadId, gotAuthor)
	}
}

func TestPadDefaultContentTypedWrapperMutatesContent(t *testing.T) {
	h := NewHook()
	h.EnqueuePadDefaultContentHook(func(ctx *events.PadDefaultContentContext) {
		want := "hello"
		ctx.Content = &want
	})

	orig := "original"
	ctx := &events.PadDefaultContentContext{Content: &orig}
	h.ExecutePadDefaultContentHooks(ctx)

	if ctx.Content == nil || *ctx.Content != "hello" {
		t.Fatalf("expected content mutated to 'hello', got %v", ctx.Content)
	}
}
