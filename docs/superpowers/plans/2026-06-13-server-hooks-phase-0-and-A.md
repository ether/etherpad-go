# Server Hooks — Phase 0 + Phase A Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the hook system's execution order deterministic, then give the six already-fired pad-lifecycle hooks typed `Enqueue`/`Execute` wrappers with contexts that live in `lib/hooks/events`.

**Architecture:** Phase 0 changes the core `Hook` storage from a nested map (nondeterministic iteration) to per-key registration-ordered slices, keeping the existing `EnqueueHook`/`DequeueHook`/`ExecuteHooks` primitives. Phase A defines `events.*Context` types (engine `*Pad` exposed as `any` to avoid the `models/pad → hooks` import cycle), adds typed wrapper methods on `*hooks.Hook`, migrates the six raw fire sites to the typed API, updates the existing `padDefaultContent` tests, and deletes the now-unused `models/pad` context structs.

**Tech Stack:** Go, `go test`, `github.com/gofiber/utils/v2` (UUID).

This is the foundation increment of the larger design in `docs/superpowers/specs/2026-06-13-server-hooks-completion-design.md`. Phases B (collab/client), C (auth/access), and D (import/export + lifecycle) are separate plans that build on the conventions established here.

---

## File Structure

- `lib/hooks/hook.go` — core registry. Phase 0 changes internal storage; Phase A adds 12 typed wrapper methods.
- `lib/hooks/hook_test.go` — **new** unit tests for ordering, dequeue, and the typed pad-lifecycle wrappers.
- `lib/hooks/events/padLifecycle.go` — **new** file holding the six pad-lifecycle context types.
- `lib/models/pad/Pad.go` — migrate `padDefaultContent`, `padCreate`, `padLoad`, `padUpdate` fire sites.
- `lib/pad/padManager.go` — migrate `padRemove` fire site.
- `lib/api/pad/copyMove.go` — migrate `padCopy` fire site.
- `lib/pad/pad_test.go` — update the six `padDefaultContent` test callbacks to the typed API.
- `lib/models/pad/PadDefaultContent.go` — delete the six now-unused context structs.

---

## Task 1: Deterministic hook ordering (Phase 0)

**Files:**
- Create: `lib/hooks/hook_test.go`
- Modify: `lib/hooks/hook.go`

- [ ] **Step 1: Write the failing tests**

Create `lib/hooks/hook_test.go`:

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./lib/hooks/ -run 'TestExecuteHooksRunsInRegistrationOrder|TestDequeueHookRemovesEntryAndPreservesOrder' -count=5 -v`

Expected: FAIL. The current `map[string]map[string]func` ranges a map, so execution order is nondeterministic — the order assertions fail (run with `-count=5` because a map sometimes happens to iterate in insertion order; at least one of the five runs fails).

- [ ] **Step 3: Replace the storage with registration-ordered slices**

Edit `lib/hooks/hook.go`. Replace the `Hook` type, `NewHook`, `EnqueueHook`, `DequeueHook`, and `ExecuteHooks` (leave every other method untouched — they delegate to these primitives):

```go
type hookEntry struct {
	id string
	fn func(ctx any)
}

type Hook struct {
	hooks map[string][]hookEntry
}

func NewHook() Hook {
	return Hook{
		hooks: make(map[string][]hookEntry),
	}
}

func (h *Hook) EnqueueHook(key string, ctx func(ctx any)) string {
	var uuid = utils.UUID()
	h.hooks[key] = append(h.hooks[key], hookEntry{id: uuid, fn: ctx})
	return uuid
}

func (h *Hook) DequeueHook(key, id string) {
	entries := h.hooks[key]
	for i, e := range entries {
		if e.id == id {
			h.hooks[key] = append(entries[:i], entries[i+1:]...)
			return
		}
	}
}

func (h *Hook) ExecuteHooks(key string, ctx any) {
	for _, e := range h.hooks[key] {
		e.fn(ctx)
	}
}
```

The `github.com/gofiber/utils/v2` import stays (still used by `utils.UUID()`).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./lib/hooks/ -count=5 -v`
Expected: PASS on all runs.

- [ ] **Step 5: Build the whole module**

Run: `go build ./...`
Expected: no output (success).

- [ ] **Step 6: Commit**

```bash
git add lib/hooks/hook.go lib/hooks/hook_test.go
git commit -m "feat(hooks): make hook execution order deterministic"
```

---

## Task 2: Pad-lifecycle event contexts + typed wrappers (Phase A)

**Files:**
- Create: `lib/hooks/events/padLifecycle.go`
- Modify: `lib/hooks/hook.go`
- Test: `lib/hooks/hook_test.go`

- [ ] **Step 1: Write the failing test for the typed wrappers**

Append to `lib/hooks/hook_test.go`:

```go
import "github.com/ether/etherpad-go/lib/hooks/events" // add to the existing import block

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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./lib/hooks/ -run 'TestPadCreateTypedWrapperDeliversContext|TestPadDefaultContentTypedWrapperMutatesContent' -v`
Expected: FAIL — compile error, `events.PadCreateContext` / `EnqueuePadCreateHook` undefined.

- [ ] **Step 3: Create the event context types**

Create `lib/hooks/events/padLifecycle.go`:

```go
package events

// The pad-lifecycle hook contexts. The engine pad object is exposed as `any`
// to avoid the lib/models/pad -> lib/hooks import cycle; plugins type-assert it
// to *pad.Pad (plugins are leaf packages and may import lib/models/pad).

// PadDefaultContentContext is passed to the padDefaultContent hook before a new
// pad's initial revision is written. A callback may replace Content (and Type);
// the caller reads Content back after the hook runs.
type PadDefaultContentContext struct {
	Type     *string
	Content  *string
	Pad      any
	AuthorId *string
	PadId    string
}

// PadLoadContext is passed to the padLoad hook whenever a pad is materialized.
type PadLoadContext struct {
	Pad   any
	PadId string
}

// PadCreateContext is passed to the padCreate hook right after a pad's first
// revision is persisted.
type PadCreateContext struct {
	Pad      any
	PadId    string
	AuthorId string
}

// PadUpdateContext is passed to the padUpdate hook after a revision is appended.
type PadUpdateContext struct {
	Pad       any
	PadId     string
	AuthorId  string
	Revs      int
	Changeset string
}

// PadCopyContext is passed to the padCopy hook after a pad is copied to a new
// destination (copyPad, copyPadWithoutHistory, movePad).
type PadCopyContext struct {
	SrcPad any
	DstPad any
	SrcId  string
	DstId  string
}

// PadRemoveContext is passed to the padRemove hook when a pad is deleted.
type PadRemoveContext struct {
	Pad   any
	PadId string
}
```

- [ ] **Step 4: Add the typed wrapper methods**

Append to `lib/hooks/hook.go` (the `events` package is already imported there):

```go
func (h *Hook) EnqueuePadDefaultContentHook(cb func(ctx *events.PadDefaultContentContext)) string {
	return h.EnqueueHook(PadDefaultContentString, func(ctx any) {
		if c, ok := ctx.(*events.PadDefaultContentContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecutePadDefaultContentHooks(ctx *events.PadDefaultContentContext) {
	h.ExecuteHooks(PadDefaultContentString, ctx)
}

func (h *Hook) EnqueuePadLoadHook(cb func(ctx *events.PadLoadContext)) string {
	return h.EnqueueHook(PadLoadString, func(ctx any) {
		if c, ok := ctx.(*events.PadLoadContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecutePadLoadHooks(ctx *events.PadLoadContext) {
	h.ExecuteHooks(PadLoadString, ctx)
}

func (h *Hook) EnqueuePadCreateHook(cb func(ctx *events.PadCreateContext)) string {
	return h.EnqueueHook(PadCreateString, func(ctx any) {
		if c, ok := ctx.(*events.PadCreateContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecutePadCreateHooks(ctx *events.PadCreateContext) {
	h.ExecuteHooks(PadCreateString, ctx)
}

func (h *Hook) EnqueuePadUpdateHook(cb func(ctx *events.PadUpdateContext)) string {
	return h.EnqueueHook(PadUpdateString, func(ctx any) {
		if c, ok := ctx.(*events.PadUpdateContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecutePadUpdateHooks(ctx *events.PadUpdateContext) {
	h.ExecuteHooks(PadUpdateString, ctx)
}

func (h *Hook) EnqueuePadCopyHook(cb func(ctx *events.PadCopyContext)) string {
	return h.EnqueueHook(PadCopyString, func(ctx any) {
		if c, ok := ctx.(*events.PadCopyContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecutePadCopyHooks(ctx *events.PadCopyContext) {
	h.ExecuteHooks(PadCopyString, ctx)
}

func (h *Hook) EnqueuePadRemoveHook(cb func(ctx *events.PadRemoveContext)) string {
	return h.EnqueueHook(PadRemoveString, func(ctx any) {
		if c, ok := ctx.(*events.PadRemoveContext); ok {
			cb(c)
		}
	})
}

func (h *Hook) ExecutePadRemoveHooks(ctx *events.PadRemoveContext) {
	h.ExecuteHooks(PadRemoveString, ctx)
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./lib/hooks/ -v`
Expected: PASS (all hook tests, including the two new ones).

- [ ] **Step 6: Commit**

```bash
git add lib/hooks/events/padLifecycle.go lib/hooks/hook.go lib/hooks/hook_test.go
git commit -m "feat(hooks): add typed wrappers for pad-lifecycle hooks"
```

---

## Task 3: Migrate `padDefaultContent`/`padCreate`/`padLoad` fire sites + update pad tests

**Files:**
- Modify: `lib/models/pad/Pad.go:367-404`
- Modify: `lib/pad/pad_test.go:67-168`

The typed `Execute*` wrappers deliver `*events.*Context`. The raw fire sites currently send `models/pad` structs, so a typed registration would never match. Migrate the fire sites and the only existing consumer (`pad_test.go`) together so the build and tests stay green.

- [ ] **Step 1: Migrate the three fire sites in `Pad.go`**

In `lib/models/pad/Pad.go`, add the events import to the existing import block:

```go
"github.com/ether/etherpad-go/lib/hooks/events"
```

Replace the `padDefaultContent` block (currently lines ~367-375):

```go
			var context = events.PadDefaultContentContext{
				AuthorId: author,
				Type:     &padDefaultText,
				Content:  text,
				Pad:      p,
				PadId:    p.Id,
			}
			p.hook.ExecutePadDefaultContentHooks(&context)
			text = context.Content
```

Replace the `padCreate` block (currently lines ~394-398):

```go
		p.hook.ExecutePadCreateHooks(&events.PadCreateContext{
			Pad:      p,
			PadId:    p.Id,
			AuthorId: createAuthor,
		})
```

Replace the `padLoad` block (currently lines ~401-404):

```go
	p.hook.ExecutePadLoadHooks(&events.PadLoadContext{
		Pad:   p,
		PadId: p.Id,
	})
```

- [ ] **Step 2: Update the six `padDefaultContent` callbacks in `pad_test.go`**

In `lib/pad/pad_test.go`, change the import of `lib/models/pad` use to `lib/hooks/events`. Add to the import block (keep `lib/models/pad` only if still referenced elsewhere in the file — if it becomes unused, remove it):

```go
"github.com/ether/etherpad-go/lib/hooks/events"
```

Replace each `padDefaultContent` registration to use the typed wrapper. The six replacements:

`TestUseProvidedContent` (lines ~67-73):

```go
	createdHooks.EnqueuePadDefaultContentHook(func(content *events.PadDefaultContentContext) {
		var emptyString = ""
		content.Content = &emptyString
		content.Content = &want
	})
```

`TestRunWhenAPadIsCreated` (lines ~101-103):

```go
	hook.EnqueuePadDefaultContentHook(func(ctx *events.PadDefaultContentContext) {
		called = true
	})
```

`TestNotCalledWithSpecificText` (lines ~114-116):

```go
	hook.EnqueuePadDefaultContentHook(func(ctx *events.PadDefaultContentContext) {
		called = true
	})
```

`TestDefaultsToSettingsPadText` (lines ~128-136):

```go
	hook.EnqueuePadDefaultContentHook(func(ctx *events.PadDefaultContentContext) {
		if *ctx.Type != "text" {
			t.Error("wrong type")
		}
		if *ctx.Content != settings.Displayed.DefaultPadText {
			t.Error("Default pad text should be settings pad text")
		}
	})
```

`TestPassesEmptyAuthorIdIfNotProvided` (lines ~144-146):

```go
	hook.EnqueuePadDefaultContentHook(func(ctx *events.PadDefaultContentContext) {
		authorId = ctx.AuthorId
	})
```

`TestPassesAuthorIdIfProvided` (lines ~158-160):

```go
	hook.EnqueuePadDefaultContentHook(func(ctx *events.PadDefaultContentContext) {
		authorId = *ctx.AuthorId
	})
```

- [ ] **Step 3: Run the pad tests to verify they pass**

Run: `go test ./lib/pad/ -v`
Expected: PASS. (If the compiler reports `lib/models/pad` imported and not used in `pad_test.go`, delete that import line and re-run.)

- [ ] **Step 4: Build the whole module**

Run: `go build ./...`
Expected: no output (success).

- [ ] **Step 5: Commit**

```bash
git add lib/models/pad/Pad.go lib/pad/pad_test.go
git commit -m "refactor(hooks): fire padDefaultContent/padCreate/padLoad via typed wrappers"
```

---

## Task 4: Migrate `padUpdate`, `padRemove`, `padCopy` fire sites

**Files:**
- Modify: `lib/models/pad/Pad.go:622-628`
- Modify: `lib/pad/padManager.go:142-145`
- Modify: `lib/api/pad/copyMove.go:119-124`

These three hooks have no existing consumers, so this is a straight swap.

- [ ] **Step 1: Migrate `padUpdate` in `Pad.go`**

Replace the block at lines ~622-628:

```go
		p.hook.ExecutePadUpdateHooks(&events.PadUpdateContext{
			Pad:       p,
			PadId:     p.Id,
			AuthorId:  updateAuthor,
			Revs:      newRev,
			Changeset: cs,
		})
```

(The `events` import was added in Task 3.)

- [ ] **Step 2: Migrate `padRemove` in `padManager.go`**

Add to the import block in `lib/pad/padManager.go`:

```go
"github.com/ether/etherpad-go/lib/hooks/events"
```

Replace the block at lines ~142-145:

```go
	m.hook.ExecutePadRemoveHooks(&events.PadRemoveContext{
		Pad:   removedPad,
		PadId: padID,
	})
```

(If `lib/models/pad` — imported as `pad` — is now unused in this file, remove that import.)

- [ ] **Step 3: Migrate `padCopy` in `copyMove.go`**

Add to the import block in `lib/api/pad/copyMove.go`:

```go
"github.com/ether/etherpad-go/lib/hooks/events"
```

Replace the block at lines ~119-124:

```go
	initStore.Hooks.ExecutePadCopyHooks(&events.PadCopyContext{
		SrcPad: srcPad,
		DstPad: dstPad,
		SrcId:  srcPad.Id,
		DstId:  dstId,
	})
```

- [ ] **Step 4: Build and run the affected package tests**

Run: `go build ./... && go test ./lib/pad/ ./lib/api/pad/ ./lib/models/pad/`
Expected: build succeeds; tests PASS.

- [ ] **Step 5: Commit**

```bash
git add lib/models/pad/Pad.go lib/pad/padManager.go lib/api/pad/copyMove.go
git commit -m "refactor(hooks): fire padUpdate/padRemove/padCopy via typed wrappers"
```

---

## Task 5: Delete the unused `models/pad` context structs

**Files:**
- Modify: `lib/models/pad/PadDefaultContent.go`

All six structs (`DefaultContent`, `Load`, `Update`, `Create`, `Copy`, `Remove`) are now unreferenced.

- [ ] **Step 1: Confirm there are no remaining references**

Run: `git grep -nE 'pad\.(DefaultContent|Load|Update|Create|Copy|Remove)\b|[^.]\b(DefaultContent|Load|Update|Create|Copy|Remove)\{' -- lib/models/pad lib/pad lib/api`
Expected: no matches that refer to these struct types (the string constants in `HookConstants.go` are unaffected — they are `PadCreateString` etc., not the struct names).

- [ ] **Step 2: Delete the struct definitions**

Delete the file `lib/models/pad/PadDefaultContent.go` entirely (it contained only these six type declarations).

Run: `rm lib/models/pad/PadDefaultContent.go`

- [ ] **Step 3: Build and run the full test suite**

Run: `go build ./... && go test ./lib/...`
Expected: build succeeds; all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add -A lib/models/pad/PadDefaultContent.go
git commit -m "refactor(hooks): drop unused models/pad lifecycle context structs"
```

---

## Self-Review

**Spec coverage (Phase 0 + A scope only):**
- Deterministic ordering (spec §1) → Task 1. ✓
- Typed wrappers + `events` contexts for `padDefaultContent`, `padCreate`, `padLoad`, `padUpdate`, `padCopy`, `padRemove` (spec §2, Category A) → Tasks 2-5. ✓
- `userJoin`/`userLeave` typed wrappers (also Category A): these have `events.UserJoinLeaveContext` and are fired via raw keys in `lib/ws`; they are deferred to the **Phase B plan** because their fire sites live in `lib/ws` alongside the Phase B collab work. Noted here so it is not lost. (No code in this plan depends on them.)
- Cycle rule "expose engine objects as `any`" (spec conventions) → applied in `padLifecycle.go`. ✓
- Phases B, C, D → separate plans (out of scope here, by design).

**Placeholder scan:** No "TBD"/"TODO"/"handle edge cases"/"similar to" placeholders; every code step shows full code. ✓

**Type consistency:** Context type names (`PadDefaultContentContext`, `PadLoadContext`, `PadCreateContext`, `PadUpdateContext`, `PadCopyContext`, `PadRemoveContext`) and method names (`Enqueue*Hook`/`Execute*Hooks`) are identical across Tasks 2-4 and the tests. Field names (`Pad`, `PadId`, `AuthorId`, `Type`, `Content`, `Revs`, `Changeset`, `SrcPad`, `DstPad`, `SrcId`, `DstId`) match the fire-site assignments and the original `models/pad` structs they replace. ✓
