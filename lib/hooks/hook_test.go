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

func TestHandleMessageContextDropMessage(t *testing.T) {
	h := NewHook()
	h.EnqueueHandleMessageHook(func(ctx *events.HandleMessageContext) {
		if ctx.PadId == "p1" {
			ctx.DropMessage()
		}
	})

	ctx := &events.HandleMessageContext{Message: "m", PadId: "p1", AuthorId: "a1"}
	h.ExecuteHandleMessageHooks(ctx)

	if !ctx.Dropped() {
		t.Fatal("expected message to be dropped")
	}
}

func TestHandleMessageSecurityGrant(t *testing.T) {
	h := NewHook()
	h.EnqueueHandleMessageSecurityHook(func(ctx *events.HandleMessageSecurityContext) {
		ctx.GrantWriteAccess()
	})

	ctx := &events.HandleMessageSecurityContext{PadId: "p1"}
	h.ExecuteHandleMessageSecurityHooks(ctx)

	if !ctx.WriteAccessGranted() {
		t.Fatal("expected write access to be granted")
	}
}

func TestChatNewMessageContextMutateAndDrop(t *testing.T) {
	h := NewHook()
	h.EnqueueChatNewMessageHook(func(ctx *events.ChatNewMessageContext) {
		*ctx.Text = "rewritten"
	})

	text := "original"
	ctx := &events.ChatNewMessageContext{Text: &text, PadId: "p1"}
	h.ExecuteChatNewMessageHooks(ctx)

	if *ctx.Text != "rewritten" {
		t.Fatalf("expected text rewritten, got %q", *ctx.Text)
	}
	if ctx.Dropped() {
		t.Fatal("did not expect drop")
	}
}

func TestClientVarsContextExtra(t *testing.T) {
	h := NewHook()
	h.EnqueueClientVarsHook(func(ctx *events.ClientVarsContext) {
		ctx.Extra["myPlugin"] = 42
	})

	ctx := &events.ClientVarsContext{Extra: map[string]any{}, PadId: "p1"}
	h.ExecuteClientVarsHooks(ctx)

	if ctx.Extra["myPlugin"] != 42 {
		t.Fatalf("expected extra key set, got %v", ctx.Extra["myPlugin"])
	}
}

func TestClientReadyTypedWrapperDelivers(t *testing.T) {
	h := NewHook()
	var gotPad string
	h.EnqueueClientReadyHook(func(ctx *events.ClientReadyContext) {
		gotPad = ctx.PadId
	})

	h.ExecuteClientReadyHooks(&events.ClientReadyContext{PadId: "p1", AuthorId: "a1", Token: "t"})

	if gotPad != "p1" {
		t.Fatalf("expected p1, got %q", gotPad)
	}
}

func TestUserJoinLeaveTypedWrappers(t *testing.T) {
	h := NewHook()
	var joined, left string
	h.EnqueueUserJoinHook(func(ctx *events.UserJoinLeaveContext) { joined = ctx.AuthorId })
	h.EnqueueUserLeaveHook(func(ctx *events.UserJoinLeaveContext) { left = ctx.AuthorId })

	h.ExecuteUserJoinHooks(&events.UserJoinLeaveContext{PadId: "p1", AuthorId: "joiner"})
	h.ExecuteUserLeaveHooks(&events.UserJoinLeaveContext{PadId: "p1", AuthorId: "leaver"})

	if joined != "joiner" || left != "leaver" {
		t.Fatalf("expected joiner/leaver, got %q/%q", joined, left)
	}
}

func TestOnAccessCheckDenyWins(t *testing.T) {
	h := NewHook()
	h.EnqueueOnAccessCheckHook(func(ctx *events.OnAccessCheckContext) {}) // no opinion
	h.EnqueueOnAccessCheckHook(func(ctx *events.OnAccessCheckContext) { ctx.Deny() })
	ctx := &events.OnAccessCheckContext{PadId: "p1", Token: "t.x"}
	h.ExecuteOnAccessCheckHooks(ctx)
	if !ctx.Denied() {
		t.Fatal("expected denied when any callback denies")
	}
}

func TestGetAuthorIdFirstMatchWins(t *testing.T) {
	h := NewHook()
	h.EnqueueGetAuthorIdHook(func(ctx *events.GetAuthorIdContext) { ctx.SetAuthorId("a.first") })
	h.EnqueueGetAuthorIdHook(func(ctx *events.GetAuthorIdContext) { ctx.SetAuthorId("a.second") })
	ctx := &events.GetAuthorIdContext{Token: "t.x"}
	h.ExecuteGetAuthorIdHooks(ctx)
	if ctx.AuthorId() != "a.first" {
		t.Fatalf("expected first author id to win, got %q", ctx.AuthorId())
	}
}

func TestAuthenticateFirstAnswerWins(t *testing.T) {
	h := NewHook()
	h.EnqueueAuthenticateHook(func(ctx *events.AuthenticateContext) { ctx.Authenticate("alice") })
	h.EnqueueAuthenticateHook(func(ctx *events.AuthenticateContext) { ctx.Reject() })
	ctx := &events.AuthenticateContext{InputUsername: "alice"}
	h.ExecuteAuthenticateHooks(ctx)
	if !ctx.Answered() || ctx.Rejected() || ctx.Username() != "alice" {
		t.Fatalf("expected first answer (authenticate alice) to win; answered=%v rejected=%v user=%q", ctx.Answered(), ctx.Rejected(), ctx.Username())
	}
}

func TestAuthorizeDenyWinsOverGrant(t *testing.T) {
	h := NewHook()
	h.EnqueueAuthorizeHook(func(ctx *events.AuthorizeContext) { ctx.Grant("readOnly") })
	h.EnqueueAuthorizeHook(func(ctx *events.AuthorizeContext) { ctx.Deny() })
	ctx := &events.AuthorizeContext{Path: "/p/x"}
	h.ExecuteAuthorizeHooks(ctx)
	if ctx.Decision() != events.AuthorizeDeny {
		t.Fatalf("expected deny to win, got %v", ctx.Decision())
	}
}

func TestAuthorizeFirstGrantLevel(t *testing.T) {
	h := NewHook()
	h.EnqueueAuthorizeHook(func(ctx *events.AuthorizeContext) { ctx.Grant("modify") })
	h.EnqueueAuthorizeHook(func(ctx *events.AuthorizeContext) { ctx.Grant("create") })
	ctx := &events.AuthorizeContext{}
	h.ExecuteAuthorizeHooks(ctx)
	if ctx.Decision() != events.AuthorizeGrant || ctx.Level() != "modify" {
		t.Fatalf("expected first grant 'modify', got decision=%v level=%q", ctx.Decision(), ctx.Level())
	}
}

func TestAuthnFailureRespond(t *testing.T) {
	h := NewHook()
	h.EnqueueAuthnFailureHook(func(ctx *events.AuthnFailureContext) {
		ctx.SetHeader("Location", "/login")
		ctx.Respond(302, "")
	})
	ctx := &events.AuthnFailureContext{Path: "/p/x"}
	h.ExecuteAuthnFailureHooks(ctx)
	if !ctx.Handled() || ctx.Status() != 302 || ctx.Headers()["Location"] != "/login" {
		t.Fatalf("expected handled 302 redirect, got handled=%v status=%d", ctx.Handled(), ctx.Status())
	}
}

func TestAuthzFailureRespond(t *testing.T) {
	h := NewHook()
	h.EnqueueAuthzFailureHook(func(ctx *events.AuthzFailureContext) { ctx.Respond(200, "upgrade") })
	ctx := &events.AuthzFailureContext{Path: "/p/x"}
	h.ExecuteAuthzFailureHooks(ctx)
	if !ctx.Handled() || ctx.Status() != 200 || ctx.Body() != "upgrade" {
		t.Fatalf("expected handled 200 body, got handled=%v status=%d body=%q", ctx.Handled(), ctx.Status(), ctx.Body())
	}
}
