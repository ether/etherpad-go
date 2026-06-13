package hooks

import "testing"

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
