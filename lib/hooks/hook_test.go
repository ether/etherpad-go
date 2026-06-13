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
