# Server Hooks — Phase B (collab/client) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the collab/client server hooks — `handleMessage`, `handleMessageSecurity`, `clientReady`, `clientVars` (with an `Extra` map merged into the payload), `chatNewMessage` — and give the already-fired `userJoin`/`userLeave` hooks typed wrappers.

**Architecture:** Follow the conventions locked in Phase 0+A: contexts in `lib/hooks/events/`, engine objects (`ws` messages, `*Client`) exposed as `any` (cycle: `lib/ws` imports `lib/hooks`); the cycle-safe `*clientVars.ClientVars` is exposed concretely. Decision/mutation via accumulator methods (`DropMessage`/`GrantWriteAccess`) + getters, like `events.PreAuthorizeContext`. New fire sites all live in `lib/ws/PadMessageHandler.go`. The `clientVars` Extra-merge is extracted into a pure, unit-tested helper.

**Tech Stack:** Go, `go test`, existing ws test harness (`lib/test/testutils`, `MockWebSocketConn`, `SessionStore.*ForTest`).

Part of the larger design: `docs/superpowers/specs/2026-06-13-server-hooks-completion-design.md`. Builds on Phase 0+A (PR #293).

---

## File Structure

- `lib/hooks/HookConstants.go` — add 7 hook-name string constants.
- `lib/hooks/events/collab.go` — **new**: the 5 new collab/client context types.
- `lib/hooks/hook.go` — add 14 typed wrapper methods (7 hooks × Enqueue/Execute); `userJoin`/`userLeave` reuse the existing `events.UserJoinLeaveContext`.
- `lib/hooks/hook_test.go` — append accumulator-semantics unit tests.
- `lib/ws/clientvars_merge.go` — **new**: pure `MergeClientVarsExtra` helper.
- `lib/ws/clientvars_merge_test.go` — **new**: unit tests for the merge helper.
- `lib/ws/PadMessageHandler.go` — wire all fire sites.
- `lib/test/ws/pad_message_handler_test.go` — add harness tests for the new fire sites.
- `doc/` — document the new hooks.

---

## Task 1: Hook constants, collab event contexts, typed wrappers

**Files:**
- Modify: `lib/hooks/HookConstants.go`
- Create: `lib/hooks/events/collab.go`
- Modify: `lib/hooks/hook.go`
- Test: `lib/hooks/hook_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `lib/hooks/hook_test.go` (the `events` import already exists in this file from Phase A):

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./lib/hooks/ -run 'HandleMessage|ChatNewMessage|ClientVars|ClientReady|UserJoinLeaveTyped' -v`
Expected: FAIL — compile errors, the `events.*Context` types and `Enqueue*`/`Execute*` methods are undefined.

- [ ] **Step 3: Add the hook-name constants**

Append to `lib/hooks/HookConstants.go`:

```go
const HandleMessageString = "handleMessage"
const HandleMessageSecurityString = "handleMessageSecurity"
const ClientReadyString = "clientReady"
const ClientVarsString = "clientVars"
const ChatNewMessageString = "chatNewMessage"
const UserJoinString = "userJoin"
const UserLeaveString = "userLeave"
```

- [ ] **Step 4: Create the collab event contexts**

Create `lib/hooks/events/collab.go`:

```go
package events

import "github.com/ether/etherpad-go/lib/models/clientVars"

// HandleMessageContext is passed to handleMessage hooks before an incoming
// socket message is dispatched. Message and Client are exposed as `any` to
// avoid the lib/ws -> lib/hooks import cycle; plugins type-assert them
// (Message to a concrete ws message type, Client to *ws.Client). A callback
// may call DropMessage() to stop the message from being processed.
type HandleMessageContext struct {
	Message  any
	Client   any
	PadId    string
	AuthorId string

	dropped bool
}

// DropMessage signals that the message must not be dispatched.
func (c *HandleMessageContext) DropMessage() { c.dropped = true }

// Dropped reports whether any callback dropped the message.
func (c *HandleMessageContext) Dropped() bool { return c.dropped }

// HandleMessageSecurityContext is passed to handleMessageSecurity hooks when a
// write message arrives on a read-only connection. A callback may call
// GrantWriteAccess() to allow this single message through. Message is `any`.
type HandleMessageSecurityContext struct {
	Message  any
	PadId    string
	AuthorId string

	writeGranted bool
}

// GrantWriteAccess allows this single write message despite the read-only connection.
func (c *HandleMessageSecurityContext) GrantWriteAccess() { c.writeGranted = true }

// WriteAccessGranted reports whether a callback granted write access.
func (c *HandleMessageSecurityContext) WriteAccessGranted() bool { return c.writeGranted }

// ClientReadyContext is passed to clientReady hooks once a client has finished
// joining a pad (informational).
type ClientReadyContext struct {
	PadId    string
	AuthorId string
	Token    string
}

// ClientVarsContext is passed to clientVars hooks just before the CLIENT_VARS
// payload is sent. A callback may mutate the typed ClientVars fields and/or add
// arbitrary top-level keys via Extra. On key collision the typed field wins
// (Extra cannot clobber engine-owned keys). Extra is always non-nil when the
// hook runs.
type ClientVarsContext struct {
	ClientVars *clientVars.ClientVars
	Extra      map[string]any
	PadId      string
	AuthorId   string
}

// ChatNewMessageContext is passed to chatNewMessage hooks before a chat message
// is stored and broadcast. Text is mutable (callbacks may set *ctx.Text or
// reassign ctx.Text); a callback may call DropMessage() to suppress the message
// entirely. Message is the chat message exposed as `any`.
type ChatNewMessageContext struct {
	Message  any
	Text     *string
	PadId    string
	AuthorId string

	dropped bool
}

// DropMessage signals that the chat message must not be stored or broadcast.
func (c *ChatNewMessageContext) DropMessage() { c.dropped = true }

// Dropped reports whether any callback dropped the chat message.
func (c *ChatNewMessageContext) Dropped() bool { return c.dropped }
```

NOTE: `lib/hooks/events` importing `lib/models/clientVars` must not introduce a cycle. `lib/models/clientVars` does not import `lib/hooks` (verified). If `go build` reports a cycle (a transitive import you couldn't foresee), STOP and report it — the fallback is to expose `ClientVars` as `any` too, but only do that if the build forces it.

- [ ] **Step 5: Add the typed wrapper methods**

Append to `lib/hooks/hook.go`:

```go
// EnqueueHandleMessageHook registers a callback for the handleMessage hook,
// fired before an incoming socket message is dispatched; a callback may drop it
// (see events.HandleMessageContext).
func (h *Hook) EnqueueHandleMessageHook(cb func(ctx *events.HandleMessageContext)) string {
	return h.EnqueueHook(HandleMessageString, func(ctx any) {
		if c, ok := ctx.(*events.HandleMessageContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteHandleMessageHooks(ctx *events.HandleMessageContext) {
	h.ExecuteHooks(HandleMessageString, ctx)
}

// EnqueueHandleMessageSecurityHook registers a callback for the
// handleMessageSecurity hook, which may grant write access to a read-only
// connection for a single message (see events.HandleMessageSecurityContext).
func (h *Hook) EnqueueHandleMessageSecurityHook(cb func(ctx *events.HandleMessageSecurityContext)) string {
	return h.EnqueueHook(HandleMessageSecurityString, func(ctx any) {
		if c, ok := ctx.(*events.HandleMessageSecurityContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteHandleMessageSecurityHooks(ctx *events.HandleMessageSecurityContext) {
	h.ExecuteHooks(HandleMessageSecurityString, ctx)
}

// EnqueueClientReadyHook registers a callback for the clientReady hook, fired
// once a client has finished joining a pad (see events.ClientReadyContext).
func (h *Hook) EnqueueClientReadyHook(cb func(ctx *events.ClientReadyContext)) string {
	return h.EnqueueHook(ClientReadyString, func(ctx any) {
		if c, ok := ctx.(*events.ClientReadyContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteClientReadyHooks(ctx *events.ClientReadyContext) {
	h.ExecuteHooks(ClientReadyString, ctx)
}

// EnqueueClientVarsHook registers a callback for the clientVars hook, fired just
// before the CLIENT_VARS payload is sent; a callback may mutate typed fields or
// add keys via Extra (see events.ClientVarsContext).
func (h *Hook) EnqueueClientVarsHook(cb func(ctx *events.ClientVarsContext)) string {
	return h.EnqueueHook(ClientVarsString, func(ctx any) {
		if c, ok := ctx.(*events.ClientVarsContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteClientVarsHooks(ctx *events.ClientVarsContext) {
	h.ExecuteHooks(ClientVarsString, ctx)
}

// EnqueueChatNewMessageHook registers a callback for the chatNewMessage hook,
// fired before a chat message is stored and broadcast; a callback may edit the
// text or drop it (see events.ChatNewMessageContext).
func (h *Hook) EnqueueChatNewMessageHook(cb func(ctx *events.ChatNewMessageContext)) string {
	return h.EnqueueHook(ChatNewMessageString, func(ctx any) {
		if c, ok := ctx.(*events.ChatNewMessageContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteChatNewMessageHooks(ctx *events.ChatNewMessageContext) {
	h.ExecuteHooks(ChatNewMessageString, ctx)
}

// EnqueueUserJoinHook registers a callback for the userJoin hook, fired when a
// user finishes joining a pad (see events.UserJoinLeaveContext).
func (h *Hook) EnqueueUserJoinHook(cb func(ctx *events.UserJoinLeaveContext)) string {
	return h.EnqueueHook(UserJoinString, func(ctx any) {
		if c, ok := ctx.(*events.UserJoinLeaveContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteUserJoinHooks(ctx *events.UserJoinLeaveContext) {
	h.ExecuteHooks(UserJoinString, ctx)
}

// EnqueueUserLeaveHook registers a callback for the userLeave hook, fired when a
// user disconnects from a pad (see events.UserJoinLeaveContext).
func (h *Hook) EnqueueUserLeaveHook(cb func(ctx *events.UserJoinLeaveContext)) string {
	return h.EnqueueHook(UserLeaveString, func(ctx any) {
		if c, ok := ctx.(*events.UserJoinLeaveContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecuteUserLeaveHooks(ctx *events.UserJoinLeaveContext) {
	h.ExecuteHooks(UserLeaveString, ctx)
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./lib/hooks/ -v`
Expected: PASS (all hook tests, including the six new ones).

- [ ] **Step 7: Commit**

```bash
git add lib/hooks/HookConstants.go lib/hooks/events/collab.go lib/hooks/hook.go lib/hooks/hook_test.go
git commit -m "feat(hooks): add collab/client hook contexts and typed wrappers"
```

---

## Task 2: clientVars Extra-merge helper

**Files:**
- Create: `lib/ws/clientvars_merge.go`
- Test: `lib/ws/clientvars_merge_test.go`

The CLIENT_VARS payload is built from a typed `*clientVars.ClientVars`, but the `clientVars` hook lets plugins add arbitrary top-level keys via `Extra`. Because `ws.Message.Data` is a typed `ClientVars`, injecting extra keys requires an untyped payload. Extract that into a pure, testable helper.

- [ ] **Step 1: Write the failing test**

Create `lib/ws/clientvars_merge_test.go`:

```go
package ws

import (
	"encoding/json"
	"testing"

	"github.com/ether/etherpad-go/lib/models/clientVars"
)

func TestMergeClientVarsExtra_AddsKey(t *testing.T) {
	cv := &clientVars.ClientVars{}
	out, err := MergeClientVarsExtra(cv, map[string]any{"myPlugin": "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["myPlugin"] != "hi" {
		t.Fatalf("expected myPlugin key, got %v", out["myPlugin"])
	}
}

func TestMergeClientVarsExtra_TypedFieldWins(t *testing.T) {
	cv := &clientVars.ClientVars{}
	// Marshal once to discover a real top-level key the typed struct owns.
	base, _ := json.Marshal(cv)
	var m map[string]any
	_ = json.Unmarshal(base, &m)
	var ownedKey string
	for k := range m {
		ownedKey = k
		break
	}
	if ownedKey == "" {
		t.Skip("ClientVars marshals to no top-level keys")
	}

	out, err := MergeClientVarsExtra(cv, map[string]any{ownedKey: "SHOULD_NOT_OVERRIDE"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[ownedKey] == "SHOULD_NOT_OVERRIDE" {
		t.Fatalf("typed field %q was clobbered by Extra", ownedKey)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./lib/ws/ -run TestMergeClientVarsExtra -v`
Expected: FAIL — `MergeClientVarsExtra` undefined.

- [ ] **Step 3: Implement the helper**

Create `lib/ws/clientvars_merge.go`:

```go
package ws

import (
	"encoding/json"

	"github.com/ether/etherpad-go/lib/models/clientVars"
)

// MergeClientVarsExtra serializes cv to a top-level JSON object and overlays the
// keys from extra that the typed struct does not already own. On collision the
// typed field wins, so plugins cannot clobber engine-owned CLIENT_VARS keys.
func MergeClientVarsExtra(cv *clientVars.ClientVars, extra map[string]any) (map[string]any, error) {
	base, err := json.Marshal(cv)
	if err != nil {
		return nil, err
	}
	var merged map[string]any
	if err := json.Unmarshal(base, &merged); err != nil {
		return nil, err
	}
	for k, v := range extra {
		if _, exists := merged[k]; !exists {
			merged[k] = v
		}
	}
	return merged, nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./lib/ws/ -run TestMergeClientVarsExtra -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add lib/ws/clientvars_merge.go lib/ws/clientvars_merge_test.go
git commit -m "feat(ws): add clientVars Extra-merge helper"
```

---

## Task 3: Wire handleMessage + handleMessageSecurity fire sites

**Files:**
- Modify: `lib/ws/PadMessageHandler.go` (HandleMessage, ~line 530-565)
- Test: `lib/test/ws/pad_message_handler_test.go`

- [ ] **Step 1: Add the events import (if missing)**

Ensure `lib/ws/PadMessageHandler.go` imports `"github.com/ether/etherpad-go/lib/hooks/events"` (it already uses `events.UserJoinLeaveContext`, so the import exists).

- [ ] **Step 2: Fire handleMessage before the dispatch switch**

In `HandleMessage`, immediately before `switch expectedType := message.(type) {` (currently ~line 536, right after the `thisSessionNewRetrieved == nil` guard), insert:

```go
	hmCtx := &events.HandleMessageContext{
		Message:  message,
		Client:   client,
		PadId:    thisSessionNewRetrieved.PadId,
		AuthorId: thisSessionNewRetrieved.Author,
	}
	p.hooks.ExecuteHandleMessageHooks(hmCtx)
	if hmCtx.Dropped() {
		return
	}
```

- [ ] **Step 3: Fire handleMessageSecurity in the read-only write-gate**

In the `case ws.UserChange:` branch, replace the read-only rejection:

```go
			if readonly {
				p.Logger.Warn("write attempt on read-only pad")
				return
			}
```

with:

```go
			if readonly {
				secCtx := &events.HandleMessageSecurityContext{
					Message:  expectedType,
					PadId:    thisSessionNewRetrieved.PadId,
					AuthorId: thisSessionNewRetrieved.Author,
				}
				p.hooks.ExecuteHandleMessageSecurityHooks(secCtx)
				if !secCtx.WriteAccessGranted() {
					p.Logger.Warn("write attempt on read-only pad")
					return
				}
			}
```

- [ ] **Step 4: Write the harness tests**

Append to `lib/test/ws/pad_message_handler_test.go`. First register the two new cases in the `testDb.AddTests(...)` call inside `TestPadMessageHandler_AllMethods` (add two `testutils.TestRunConfig{...}` entries pointing to the functions below). Then add the functions:

```go
func testHandleMessageDropStopsDispatch(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-hm-drop"
	authorId, err := setupPadAndAuthor(t, ds, padId, "DropUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-hm-drop"
	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() { delete(ds.Hub.Clients, client) }()

	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	ds.PadMessageHandler.SessionStore.AddPadReadOnlyIdsForTest(sessionId, padId, "readonly-id", false)

	retrievedPad, err := ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)
	headBefore := retrievedPad.Head

	// Drop every message.
	id := ds.Hooks.EnqueueHandleMessageHook(func(ctx *events.HandleMessageContext) {
		ctx.DropMessage()
	})
	defer ds.Hooks.DequeueHook(hooks.HandleMessageString, id)

	userChange := ws.UserChange{
		Event: "message",
		Data: ws.UserChangeData{
			Component: "pad",
			Type:      "USER_CHANGES",
			Data: ws.UserChangeDataData{
				Apool:     ws.UserChangeDataDataApool{NumToAttrib: map[int][]string{}, NextNum: 0},
				BaseRev:   0,
				Changeset: "Z:1>3+3$abc",
			},
		},
	}
	initStore := ds.ToInitStore()
	ds.PadMessageHandler.HandleMessage(userChange, client, initStore.RetrievedSettings, ds.Logger)
	time.Sleep(100 * time.Millisecond)

	retrievedPad, err = ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)
	assert.Equal(t, headBefore, retrievedPad.Head, "dropped USER_CHANGES must not change the pad")
}

func testHandleMessageSecurityGrantsWriteOnReadonly(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-hms-grant"
	authorId, err := setupPadAndAuthor(t, ds, padId, "GrantUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-hms-grant"
	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() { delete(ds.Hub.Clients, client) }()

	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	ds.PadMessageHandler.SessionStore.AddPadReadOnlyIdsForTest(sessionId, padId, "readonly-id", true)
	ds.PadMessageHandler.SessionStore.SetReadOnlyForTest(sessionId, true)

	retrievedPad, err := ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)
	headBefore := retrievedPad.Head

	// Grant write access for the read-only connection.
	id := ds.Hooks.EnqueueHandleMessageSecurityHook(func(ctx *events.HandleMessageSecurityContext) {
		ctx.GrantWriteAccess()
	})
	defer ds.Hooks.DequeueHook(hooks.HandleMessageSecurityString, id)

	userChange := ws.UserChange{
		Event: "message",
		Data: ws.UserChangeData{
			Component: "pad",
			Type:      "USER_CHANGES",
			Data: ws.UserChangeDataData{
				Apool:     ws.UserChangeDataDataApool{NumToAttrib: map[int][]string{}, NextNum: 0},
				BaseRev:   0,
				Changeset: "Z:1>3+3$abc",
			},
		},
	}
	initStore := ds.ToInitStore()
	ds.PadMessageHandler.HandleMessage(userChange, client, initStore.RetrievedSettings, ds.Logger)
	time.Sleep(200 * time.Millisecond)

	retrievedPad, err = ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)
	assert.Greater(t, retrievedPad.Head, headBefore, "granted write must apply the change despite read-only")
}
```

Add the `hooks` and `events` imports to the test file's import block:

```go
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
```

- [ ] **Step 5: Run the tests**

Run: `go test ./lib/test/ws/ -run TestPadMessageHandler_AllMethods -count=1 -v 2>&1 | grep -E "hm-drop|hms-grant|FAIL|ok"`
Expected: both new subtests PASS. (If `testHandleMessageSecurityGrantsWriteOnReadonly` shows the change did not apply, increase the sleep — the USER_CHANGES path is queued via `padChannels.AddToQueue` and applied asynchronously; the existing readonly-reject test uses 100ms, this one needs the change to actually process, so 200ms is used.)

- [ ] **Step 6: Build and commit**

```bash
go build ./...
git add lib/ws/PadMessageHandler.go lib/test/ws/pad_message_handler_test.go
git commit -m "feat(hooks): wire handleMessage and handleMessageSecurity hooks"
```

---

## Task 4: Wire clientVars hook + Extra-merge into the send path

**Files:**
- Modify: `lib/ws/PadMessageHandler.go` (HandleClientReadyMessage, ~line 1425-1450)

- [ ] **Step 1: Fire the clientVars hook and merge Extra at send**

In `HandleClientReadyMessage`, after the atomic-snapshot assignments (`retrivedClientVars.CollabClientVars.InitialAttributedText.Attribs = atextSnapshot.Attribs`, ~line 1433) and before the session-time block, fire the hook:

```go
		cvCtx := &events.ClientVarsContext{
			ClientVars: retrivedClientVars,
			Extra:      map[string]any{},
			PadId:      thisSession.PadId,
			AuthorId:   thisSession.Author,
		}
		p.hooks.ExecuteClientVarsHooks(cvCtx)
```

Then replace the payload-building block (currently):

```go
		var arr = make([]interface{}, 2)
		arr[0] = "message"
		arr[1] = Message{
			Data: *retrivedClientVars,
			Type: "CLIENT_VARS",
		}
		var encoded, _ = json.Marshal(arr)
```

with:

```go
		var arr = make([]interface{}, 2)
		arr[0] = "message"
		if len(cvCtx.Extra) == 0 {
			arr[1] = Message{
				Data: *retrivedClientVars,
				Type: "CLIENT_VARS",
			}
		} else {
			merged, mergeErr := MergeClientVarsExtra(retrivedClientVars, cvCtx.Extra)
			if mergeErr != nil {
				p.Logger.Warn("Error merging clientVars extras", mergeErr.Error())
				return
			}
			arr[1] = map[string]any{
				"type": "CLIENT_VARS",
				"data": merged,
			}
		}
		var encoded, _ = json.Marshal(arr)
```

(Leave the rest of the function — `thisSession.PadId = retrievedPad.Id`, `client.SafeSend(encoded)`, etc. — unchanged.)

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: success. (The merge logic itself is covered by Task 2's unit tests; this step wires it in. The fire site runs whenever a client becomes ready.)

- [ ] **Step 3: Commit**

```bash
git add lib/ws/PadMessageHandler.go
git commit -m "feat(hooks): wire clientVars hook with Extra-map merge"
```

---

## Task 5: Wire chatNewMessage + migrate userJoin/userLeave + clientReady

**Files:**
- Modify: `lib/ws/PadMessageHandler.go` (SendChatMessageToPadClients ~907; userLeave ~1195; HandleClientReadyMessage userJoin ~1531)
- Test: `lib/test/ws/pad_message_handler_test.go`

- [ ] **Step 1: Fire chatNewMessage at the top of SendChatMessageToPadClients**

At the start of `SendChatMessageToPadClients` (before `var retrievedPad, err = p.padManager.GetPad(...)`, ~line 908), insert:

```go
	var chatAuthorId string
	if chatMessage.AuthorId != nil {
		chatAuthorId = *chatMessage.AuthorId
	}
	text := chatMessage.Text
	cmCtx := &events.ChatNewMessageContext{
		Message:  chatMessage,
		Text:     &text,
		PadId:    session.PadId,
		AuthorId: chatAuthorId,
	}
	p.hooks.ExecuteChatNewMessageHooks(cmCtx)
	if cmCtx.Dropped() {
		return
	}
	chatMessage.Text = *cmCtx.Text
```

(`chatMessage` is a value parameter, so reassigning `chatMessage.Text` affects only this call's copy, which is exactly what the rest of the function uses for storage and broadcast.)

- [ ] **Step 2: Migrate the userLeave fire site**

Replace (currently ~line 1195):

```go
	p.hooks.ExecuteHooks("userLeave", &events.UserJoinLeaveContext{
		PadId:    padId,
		AuthorId: authorId,
		BroadcastChat: func(message map[string]any) {
			p.BroadcastSystemChatToRoom(padId, message)
		},
	})
```

with:

```go
	p.hooks.ExecuteUserLeaveHooks(&events.UserJoinLeaveContext{
		PadId:    padId,
		AuthorId: authorId,
		BroadcastChat: func(message map[string]any) {
			p.BroadcastSystemChatToRoom(padId, message)
		},
	})
```

- [ ] **Step 3: Migrate the userJoin fire site and add clientReady**

Replace the userJoin block at the end of `HandleClientReadyMessage` (currently ~line 1531):

```go
	// Fire userJoin hooks
	p.hooks.ExecuteHooks("userJoin", &events.UserJoinLeaveContext{
		PadId:    thisSession.PadId,
		AuthorId: thisSession.Author,
		BroadcastChat: func(message map[string]any) {
			p.BroadcastSystemChatToRoom(thisSession.PadId, message)
		},
	})
```

with:

```go
	// Fire userJoin hooks
	p.hooks.ExecuteUserJoinHooks(&events.UserJoinLeaveContext{
		PadId:    thisSession.PadId,
		AuthorId: thisSession.Author,
		BroadcastChat: func(message map[string]any) {
			p.BroadcastSystemChatToRoom(thisSession.PadId, message)
		},
	})

	// Fire clientReady hooks now that the client has fully joined the pad.
	var clientReadyToken string
	if thisSession.Auth != nil {
		clientReadyToken = thisSession.Auth.Token
	}
	p.hooks.ExecuteClientReadyHooks(&events.ClientReadyContext{
		PadId:    thisSession.PadId,
		AuthorId: thisSession.Author,
		Token:    clientReadyToken,
	})
```

- [ ] **Step 4: Write chatNewMessage harness tests**

Register two new entries in `TestPadMessageHandler_AllMethods`'s `AddTests`, then add:

```go
func testChatNewMessageRewritesText(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-chat-rewrite"
	authorId, err := setupPadAndAuthor(t, ds, padId, "ChatRewriteUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-chat-rewrite"
	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() { delete(ds.Hub.Clients, client) }()
	wg := startMockWritePump(client, mockConn)

	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	session := ds.PadMessageHandler.SessionStore.GetSessionForTest(sessionId)
	require.NotNil(t, session)

	id := ds.Hooks.EnqueueChatNewMessageHook(func(ctx *events.ChatNewMessageContext) {
		*ctx.Text = "REWRITTEN"
	})
	defer ds.Hooks.DequeueHook(hooks.ChatNewMessageString, id)

	chatTime := time.Now().UnixMilli()
	ds.PadMessageHandler.SendChatMessageToPadClients(session, ws.ChatMessageData{
		Text: "original", Time: &chatTime, AuthorId: &authorId,
	})
	wg.Wait()

	// The stored message must reflect the rewrite.
	retrievedPad, err := ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)
	msgs, err := retrievedPad.GetChatMessages(0, 1)
	require.NoError(t, err)
	require.Len(t, *msgs, 1)
	assert.Equal(t, "REWRITTEN", (*msgs)[0].Text)
}

func testChatNewMessageDropSuppresses(t *testing.T, ds testutils.TestDataStore) {
	padId := "test-pad-chat-drop"
	authorId, err := setupPadAndAuthor(t, ds, padId, "ChatDropUser")
	require.NoError(t, err)

	mockConn := libws.NewActualMockWebSocketconn()
	sessionId := "test-session-chat-drop"
	client := createTestClient(ds.Hub, sessionId, padId, mockConn)
	defer func() { delete(ds.Hub.Clients, client) }()

	ds.PadMessageHandler.SessionStore.InitSessionForTest(sessionId)
	ds.PadMessageHandler.SessionStore.AddHandleClientInformationForTest(sessionId, padId, "test-token")
	ds.PadMessageHandler.SessionStore.SetAuthorForTest(sessionId, authorId)
	ds.PadMessageHandler.SessionStore.SetPadIdForTest(sessionId, padId)
	session := ds.PadMessageHandler.SessionStore.GetSessionForTest(sessionId)
	require.NotNil(t, session)

	id := ds.Hooks.EnqueueChatNewMessageHook(func(ctx *events.ChatNewMessageContext) {
		ctx.DropMessage()
	})
	defer ds.Hooks.DequeueHook(hooks.ChatNewMessageString, id)

	chatTime := time.Now().UnixMilli()
	ds.PadMessageHandler.SendChatMessageToPadClients(session, ws.ChatMessageData{
		Text: "should be dropped", Time: &chatTime, AuthorId: &authorId,
	})
	time.Sleep(50 * time.Millisecond)

	retrievedPad, err := ds.PadManager.GetPad(padId, nil, &authorId)
	require.NoError(t, err)
	msgs, err := retrievedPad.GetChatMessages(0, 1)
	require.NoError(t, err)
	assert.Len(t, *msgs, 0, "dropped chat message must not be stored")
}
```

NOTE: confirm the chat message type returned by `GetChatMessages` exposes a `.Text` field (it is the same `ChatMessageData`/chat model used elsewhere in this test file). If the field name differs, adapt the assertion to the actual field.

- [ ] **Step 5: Run the tests**

Run: `go test ./lib/test/ws/ -run TestPadMessageHandler_AllMethods -count=1 -v 2>&1 | grep -E "chat-rewrite|chat-drop|FAIL|ok"`
Expected: both new subtests PASS.

- [ ] **Step 6: Build and commit**

```bash
go build ./...
git add lib/ws/PadMessageHandler.go lib/test/ws/pad_message_handler_test.go
git commit -m "feat(hooks): wire chatNewMessage + clientReady, typed userJoin/userLeave"
```

---

## Task 6: Documentation + full verification

**Files:**
- Modify/Create: a hooks doc under `doc/`

- [ ] **Step 1: Document the new hooks**

Find the existing hooks documentation (e.g. `doc/` — search for a server-hooks page; if none exists for Go, create `doc/api/hooks_server-side_go.md`). Add an entry for each new hook: name, when it fires, the `events.*Context` type, its fields, the decision/mutation methods, and the "engine objects are `any` — type-assert" convention. Cover: `handleMessage` (DropMessage), `handleMessageSecurity` (GrantWriteAccess), `clientReady`, `clientVars` (mutate typed fields or add via `Extra`; typed field wins on collision), `chatNewMessage` (edit `*Text` / DropMessage), `userJoin`, `userLeave`.

- [ ] **Step 2: Run the full affected test suite**

Run: `go build ./... && go test ./lib/hooks/ ./lib/ws/ ./lib/test/ws/ ./lib/test/api/pad/ -count=1`
Expected: build clean; all PASS. (A failure in `lib/test/plugins/ep_rss` from a Windows MySQL test-lock "Access is denied" is a known, unrelated infra flake — ignore only that one.)

- [ ] **Step 3: Commit**

```bash
git add doc/
git commit -m "docs(hooks): document collab/client server hooks"
```

---

## Self-Review

**Spec coverage (Phase B):**
- `handleMessage` (DropMessage) → Task 1 (context/wrapper) + Task 3 (fire site, before dispatch switch). ✓
- `handleMessageSecurity` (GrantWriteAccess) → Task 1 + Task 3 (read-only write-gate). ✓
- `clientReady` → Task 1 + Task 5 (end of HandleClientReadyMessage). ✓
- `clientVars` (typed mutation + Extra, typed-wins) → Task 1 + Task 2 (helper) + Task 4 (fire + merge). ✓
- `chatNewMessage` (edit text / drop) → Task 1 + Task 5 (SendChatMessageToPadClients top — broader & more testable than the case branch; still "before store & broadcast" per spec). ✓
- `userJoin`/`userLeave` typed wrappers → Task 1 + Task 5 (migrate raw fire sites). ✓
- Engine objects exposed as `any` (Message, Client) → contexts in Task 1. `clientVars.ClientVars` concrete (cycle-safe). ✓
- Deterministic ordering (Phase 0) underpins DropMessage/Extra correctness — relied upon, not re-implemented. ✓

**Placeholder scan:** No TBD/"handle edge cases"/"similar to". Two explicit verify-then-adapt notes (clientVars import cycle in Task 1 Step 4; chat model `.Text` field name in Task 5 Step 4) are real fallbacks with concrete actions, not placeholders.

**Type consistency:** Context type names (`HandleMessageContext`, `HandleMessageSecurityContext`, `ClientReadyContext`, `ClientVarsContext`, `ChatNewMessageContext`) and method names (`Enqueue*Hook`/`Execute*Hooks`, `DropMessage`/`Dropped`, `GrantWriteAccess`/`WriteAccessGranted`) match across Tasks 1, 3, 4, 5 and the tests. Constants (`HandleMessageString`, etc.) match their wrapper usages. `MergeClientVarsExtra(*clientVars.ClientVars, map[string]any) (map[string]any, error)` signature matches Task 2 definition and Task 4 call site.

**Known coverage gap (acceptable):** `clientReady`'s fire site has no dedicated harness test (constructing a full `ws.ClientReady` to drive `HandleClientReadyMessage` is intricate; existing repo tests avoid it). The typed wrapper is unit-tested (Task 1) and the fire site is a trivial 5-line call exercised whenever a client joins. Flagged here rather than hidden.
