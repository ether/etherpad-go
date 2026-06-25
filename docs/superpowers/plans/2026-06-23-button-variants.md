# Button Variants + Visible Borders Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the shared `EpButton` web component a complete, conventionally-named variant set (`primary`, `secondary`, `danger`, `ghost`, `icon`, with `default` as an alias of `secondary`), guarantee a visible border, publish `0.0.14`, and make the styled buttons render correctly in the running `etherpad-go` app.

**Architecture:** Two repos. (1) `webcomponents` — a Lit 3 component library; edit `src/EpButton.ts` (CSS variants + reflected `variant` property) and `src/EpTheme.ts` (add `--error-color` token), with Storybook play-function tests run by Vitest in a real Chromium browser. Publish via the existing `workflow_dispatch` OIDC publish workflow. (2) `etherpad-go` — bump the pinned dependency, regenerate the lockfile, and reproduce/fix the integration rendering.

**Tech Stack:** Lit 3, TypeScript, Vitest + Storybook (`@storybook/web-components-vite`, Chromium via Playwright), pnpm workspace, npm Trusted Publishing (OIDC).

## Global Constraints

- webcomponents repo path: `C:\Users\samue\WebstormProjects\webcomponents`. etherpad-go repo path: `C:\Users\samue\GolandProjects\etherpad-go`. These are **separate git repos** — commit in the repo each task touches.
- Target published version: `@samtv12345/etherpad-webcomponents@0.0.14` (bump from `0.0.13`).
- Backward compatibility is REQUIRED: `variant="default"` must keep working (alias of `secondary`); a variant-less `<ep-button>` must stay bordered. No existing usage may break.
- Variant set (exact): `primary` (filled `--primary-color`), `secondary` (outlined neutral), `default` (alias of `secondary`), `danger` (filled `--error-color`, fallback `#d1242f`), `ghost`, `icon`.
- Danger fallback color `#d1242f` == `rgb(209, 36, 47)`. Primary fallback `#64d29b` == `rgb(100, 210, 155)`.
- webcomponents: `pnpm` (CI uses pnpm v9 / Node 22). etherpad-go: `pnpm@11.5.2`, Node 24.
- etherpad-go pins the package via `overrides` AND `minimumReleaseAgeExclude` in `pnpm-workspace.yaml`, plus `package.json` (root) and `ui/package.json`. The local pnpm release-age gate blocks a same-day version unless `minimumReleaseAgeExclude` lists the exact `name@version` (see memory `reference_ci_pnpm_release_age_gate`).
- Publishing is via the `publish.yml` `workflow_dispatch` workflow (input `version`); it sets the version, builds, `npm publish --access public`, commits `release: vX`, tags `vX`, pushes. Do NOT hand-edit `package.json` version — the workflow owns it.
- Storybook play functions ARE the tests (run in Chromium). Import test helpers from `storybook/test`.

---

### Task 1: `secondary` variant + `default` alias (webcomponents)

**Repo:** `C:\Users\samue\WebstormProjects\webcomponents`

**Files:**
- Modify: `src/EpButton.ts` (variant union + CSS selector lists)
- Modify: `stories/EpButton.stories.ts` (args type, argTypes, new `Secondary` story)

**Interfaces:**
- Produces: `<ep-button variant="secondary">` renders the outlined neutral style (1px solid border); `variant="default"` and no-variant remain identical to it.

- [ ] **Step 1: Write the failing test (new Secondary story with computed-style assertions)**

In `stories/EpButton.stories.ts`, first widen the args type and argTypes to the full final set (used by this and Task 2), then add the story:

```typescript
// Replace the EpButtonArgs type:
type EpButtonArgs = {
  variant: 'primary' | 'secondary' | 'default' | 'danger' | 'ghost' | 'icon';
  size: 'small' | 'medium' | 'large';
  disabled: boolean;
};

// In meta.argTypes.variant.options use:
//   ['default', 'secondary', 'primary', 'danger', 'ghost', 'icon']
```

Add this story at the end of the file:

```typescript
export const Secondary: Story = {
  args: { variant: 'secondary' },
  render: (args: EpButtonArgs) => html`
    <ep-button variant="${args.variant}">Secondary</ep-button>
  `,
  play: async ({ canvasElement }) => {
    const { button } = await getButton(canvasElement);
    const cs = getComputedStyle(button);
    // secondary must show a visible border (the borderless base has 'none')
    await expect(cs.borderTopStyle).toBe('solid');
    await expect(cs.borderTopWidth).toBe('1px');
  },
};

export const DefaultIsBordered: Story = {
  args: { variant: 'default' },
  render: (args: EpButtonArgs) => html`
    <ep-button variant="${args.variant}">Default</ep-button>
  `,
  play: async ({ canvasElement }) => {
    const { button } = await getButton(canvasElement);
    await expect(getComputedStyle(button).borderTopStyle).toBe('solid');
  },
};
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /c/Users/samue/WebstormProjects/webcomponents
pnpm exec vitest run --project storybook -t "Secondary"
```
Expected: FAIL — `Secondary` play asserts `borderTopStyle === 'solid'` but `variant="secondary"` matches no CSS rule, so the base button border is `none`. (`DefaultIsBordered` passes — it already has a border.)

If Chromium is missing, first run `pnpm exec playwright install chromium`, then re-run.

- [ ] **Step 3: Implement — add `secondary` to the bordered selector lists and the variant union**

In `src/EpButton.ts`, change the "Default" CSS block so `secondary` shares it:

```css
    /* Secondary / Default (outlined neutral) */
    :host([variant="secondary"]) button,
    :host([variant="default"]) button,
    :host(:not([variant])) button {
      color: var(--text-color, #485365);
      border: 1px solid var(--middle-color, #d2d2d2);
    }

    :host([variant="secondary"]) button:hover,
    :host([variant="default"]) button:hover,
    :host(:not([variant])) button:hover {
      background: var(--bg-soft-color, #f2f3f4);
    }
```

Update the property declaration (union + default value):

```typescript
  @property({ reflect: true }) variant: 'primary' | 'secondary' | 'default' | 'danger' | 'ghost' | 'icon' = 'secondary';
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
pnpm exec vitest run --project storybook -t "Secondary"
```
Expected: PASS (both `Secondary` and `DefaultIsBordered`).

- [ ] **Step 5: Commit**

```bash
git add src/EpButton.ts stories/EpButton.stories.ts
git commit -m "feat(button): add secondary variant (alias of default), guarantee border"
```

---

### Task 2: `danger` variant + `--error-color` token (webcomponents)

**Repo:** `C:\Users\samue\WebstormProjects\webcomponents`

**Files:**
- Modify: `src/EpTheme.ts` (add `--error-color` to `ThemeTokens` + all four themes)
- Modify: `src/EpButton.ts` (add `danger` CSS rules)
- Modify: `stories/EpButton.stories.ts` (add `Danger` story; extend `AllVariants`)

**Interfaces:**
- Consumes: the variant union from Task 1 (already includes `'danger'`).
- Produces: `<ep-button variant="danger">` renders a filled red button (`--error-color`, fallback `#d1242f`).

- [ ] **Step 1: Write the failing test (Danger story)**

Add to `stories/EpButton.stories.ts`:

```typescript
export const Danger: Story = {
  args: { variant: 'danger' },
  render: (args: EpButtonArgs) => html`
    <ep-button variant="${args.variant}">Delete</ep-button>
  `,
  play: async ({ canvasElement }) => {
    const { button } = await getButton(canvasElement);
    const cs = getComputedStyle(button);
    // danger fills with --error-color; fallback #d1242f == rgb(209, 36, 47)
    await expect(cs.backgroundColor).toBe('rgb(209, 36, 47)');
    await expect(cs.borderTopStyle).toBe('none');
  },
};
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /c/Users/samue/WebstormProjects/webcomponents
pnpm exec vitest run --project storybook -t "Danger"
```
Expected: FAIL — `variant="danger"` matches no rule, so the base button background is `transparent` (`rgba(0, 0, 0, 0)`), not `rgb(209, 36, 47)`.

- [ ] **Step 3a: Add the `--error-color` token to `src/EpTheme.ts`**

In the `ThemeTokens` interface add the field:

```typescript
  '--error-color': string;
```

Add `'--error-color'` to each theme object:

```typescript
// colibris:
'--error-color': '#d1242f',
// colibris-dark:
'--error-color': '#f0626b',
// high-contrast:
'--error-color': '#cc0000',
// warm:
'--error-color': '#b3402f',
```

- [ ] **Step 3b: Add the `danger` CSS rules to `src/EpButton.ts`**

Add after the `primary` rules:

```css
    /* Danger — filled destructive. */
    :host([variant="danger"]) button {
      background: var(--error-color, #d1242f);
      color: var(--bg-color, #ffffff);
      border: none;
      transition: filter 0.15s ease, opacity 0.15s ease;
    }

    :host([variant="danger"]) button:hover {
      filter: brightness(0.94);
    }

    :host([variant="danger"]) button:active {
      filter: brightness(0.88);
    }
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
pnpm exec vitest run --project storybook -t "Danger"
```
Expected: PASS.

- [ ] **Step 5: Extend the `AllVariants` story (visual coverage) and run the whole button suite**

In `stories/EpButton.stories.ts`, update `AllVariants` to include the new variants and fix the count assertion:

```typescript
export const AllVariants: Story = {
  render: () => html`
    <div style="display: flex; gap: 12px; align-items: center;">
      <ep-button variant="secondary">Secondary</ep-button>
      <ep-button variant="primary">Primary</ep-button>
      <ep-button variant="danger">Danger</ep-button>
      <ep-button variant="ghost">Ghost</ep-button>
      <ep-button variant="icon">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
          <path d="M8 4a.5.5 0 01.5.5v3h3a.5.5 0 010 1h-3v3a.5.5 0 01-1 0v-3h-3a.5.5 0 010-1h3v-3A.5.5 0 018 4z"/>
        </svg>
      </ep-button>
    </div>
  `,
  play: async ({ canvasElement }) => {
    const buttons = canvasElement.querySelectorAll('ep-button');
    await expect(buttons.length).toBe(5);
  },
};
```

```bash
pnpm exec vitest run --project storybook -t "EpButton"
pnpm exec tsc --noEmit
```
Expected: all EpButton stories PASS; typecheck clean.

- [ ] **Step 6: Commit**

```bash
git add src/EpButton.ts src/EpTheme.ts stories/EpButton.stories.ts
git commit -m "feat(button): add danger variant and --error-color theme token"
```

---

### Task 3: Publish `0.0.14` (webcomponents)

**Repo:** `C:\Users\samue\WebstormProjects\webcomponents`

**Files:** none edited here (the publish workflow owns the version bump).

**Interfaces:**
- Produces: `@samtv12345/etherpad-webcomponents@0.0.14` live on npmjs, built from the committed variant changes.

- [ ] **Step 1: Push the committed changes to the default branch**

```bash
cd /c/Users/samue/WebstormProjects/webcomponents
git push origin HEAD
```
(The `publish.yml` workflow builds from the repo's checked-out default branch, so the variant commits must be pushed first.)

- [ ] **Step 2: Trigger the publish workflow for 0.0.14**

```bash
gh workflow run publish.yml -f version=0.0.14 --repo <owner>/<webcomponents-repo>
```
Determine `<owner>/<repo>` with `gh repo view --json nameWithOwner -q .nameWithOwner` run inside the webcomponents repo.

- [ ] **Step 3: Verify the run succeeded and the version is live**

```bash
gh run list --workflow=publish.yml --limit 1
# after it completes:
npm view @samtv12345/etherpad-webcomponents version
```
Expected: `0.0.14`. The workflow also creates tag `v0.0.14` and a `release: v0.0.14` commit.

---

### Task 4: Bump etherpad-go to `0.0.14`

**Repo:** `C:\Users\samue\GolandProjects\etherpad-go`

**Files:**
- Modify: `pnpm-workspace.yaml` (`overrides` + `minimumReleaseAgeExclude`)
- Modify: `package.json` (root dependency)
- Modify: `ui/package.json` (dependency)
- Modify: `pnpm-lock.yaml` (regenerated)

**Interfaces:**
- Consumes: published `0.0.14` from Task 3 (must be live on npm before regenerating the lockfile).

- [ ] **Step 1: Update the pinned version in all three manifests + the release-age exclude**

In `pnpm-workspace.yaml`:

```yaml
overrides:
  "@samtv12345/etherpad-webcomponents": 0.0.14

minimumReleaseAgeExclude:
  - "@samtv12345/etherpad-webcomponents@0.0.14"
```

In root `package.json` set `"@samtv12345/etherpad-webcomponents": "^0.0.14"`.
In `ui/package.json` set `"@samtv12345/etherpad-webcomponents": "^0.0.14"`.

- [ ] **Step 2: Regenerate the lockfile**

```bash
cd /c/Users/samue/GolandProjects/etherpad-go
pnpm install --lockfile-only
```
Expected: `pnpm-lock.yaml` now resolves `@samtv12345/etherpad-webcomponents@0.0.14`. Verify:

```bash
grep -n "etherpad-webcomponents@0.0.14" pnpm-lock.yaml | head
```
Expected: at least one match; no remaining `@0.0.13` for this package.

- [ ] **Step 3: Verify a frozen install succeeds (CI parity)**

```bash
pnpm install --frozen-lockfile
```
Expected: completes without `ERR_PNPM_OUTDATED_LOCKFILE` or `ERR_PNPM_MINIMUM_RELEASE_AGE_VIOLATION`.

- [ ] **Step 4: Commit**

```bash
git add pnpm-workspace.yaml package.json ui/package.json pnpm-lock.yaml
git commit -m "chore(deps): bump @samtv12345/etherpad-webcomponents to 0.0.14"
```

---

### Task 5: Reproduce and fix the "no border" rendering in etherpad-go

**Repo:** `C:\Users\samue\GolandProjects\etherpad-go`

**Files:**
- Inspect: `assets/welcome/main.templ` (uses `<ep-button variant="primary">` / `variant="default">`)
- Inspect: `ui/src/welcome.ts`, `ui/src/welcome.entry.ts` (registers `EpButton.js`)
- Modify (one of, depending on the confirmed cause): the welcome template/entry to ensure the bundle loads and/or wrap rendered web components in `<ep-theme name="colibris">`.

**Interfaces:**
- Consumes: `0.0.14` from Task 4 (the secondary/border guarantee).
- Produces: Welcome page renders a green `primary` button and a bordered `secondary`/`default` button; no unstyled/borderless `ep-button`.

- [ ] **Step 1: Reproduce in the running app**

Use the `run` skill (or the project's documented dev start) to launch etherpad-go and open the Welcome page. Confirm the symptom (borderless/unstyled buttons) and capture which it is:
- Open devtools, inspect an `<ep-button>`. If it has **no `#shadow-root`**, the custom element never upgraded → the registering bundle isn't loaded (cause A).
- If it HAS a shadow root with the inner `<button>` but no border, tokens/variant are the issue (cause B).

- [ ] **Step 2: Apply the matching fix**

Cause A (element not upgraded) — ensure the Welcome HTML loads the JS entry that imports `EpButton.js`. Confirm `ui/src/welcome.entry.ts` imports `./welcome.ts` (which does `import '@samtv12345/etherpad-webcomponents/EpButton.js'`) and that `assets/welcome/main.templ` includes the built welcome bundle `<script type="module">`. If the script tag is missing or points at the wrong asset, fix it so the bundle loads.

Cause B (tokens) — wrap the welcome button container in the theme element so tokens are defined:

```html
<ep-theme name="colibris">
  ... existing <ep-button> markup ...
</ep-theme>
```
and ensure `EpTheme` is registered (add `import '@samtv12345/etherpad-webcomponents/EpTheme.js'` to `ui/src/welcome.ts` — verify the package exposes that subpath export; otherwise import `{ EpTheme }` from the package root). Note: even without a theme the new `0.0.14` secondary/default border uses a hardcoded fallback, so cause A is the more likely real fix.

- [ ] **Step 3: (Optional) switch to canonical variant naming**

In `assets/welcome/main.templ`, change `<ep-button variant="default" id="newSheet">` to `variant="secondary"`. Re-run `templ generate` if the project compiles templ:

```bash
templ generate
```

- [ ] **Step 4: Verify in the running app**

Reload the Welcome page. Expected: "New pad"/OK buttons are green (primary); the spreadsheet button shows a visible border (secondary/default). No unstyled buttons remain.

- [ ] **Step 5: Commit**

```bash
git add assets/welcome/main.templ assets/welcome/main_templ.go ui/src/welcome.ts
git commit -m "fix(welcome): render styled ep-buttons (load bundle / theme) and use secondary variant"
```

---

## Self-Review

**Spec coverage:**
- Variant set (primary/secondary/default-alias/danger/ghost/icon) → Tasks 1, 2. ✓
- Visible border guaranteed via fallback → Task 1 (border on secondary/default/no-variant). ✓
- `--error-color` token in all 4 themes → Task 2 Step 3a. ✓
- Storybook stories + tests per variant → Tasks 1, 2 (Secondary, DefaultIsBordered, Danger, AllVariants). ✓
- Publish 0.0.14 via OIDC workflow → Task 3. ✓
- etherpad-go override/lockfile/minimumReleaseAgeExclude bump → Task 4. ✓
- Reproduce & fix "no border" rendering → Task 5. ✓
- Optional default→secondary in welcome → Task 5 Step 3. ✓

**Placeholder scan:** No TBD/TODO; every code step shows the code. The one open branch (Task 5 cause A vs B) is an explicit reproduce-then-choose with both fixes written out — not a placeholder. `<owner>/<repo>` in Task 3 has an explicit command to resolve it.

**Type consistency:** The `variant` union `'primary' | 'secondary' | 'default' | 'danger' | 'ghost' | 'icon'` is defined once in Task 1 (EpButton.ts property and the stories `EpButtonArgs`) and reused in Task 2. Story helper `getButton`, `expect` (from `storybook/test`), and `html` are the file's existing imports. Computed-style values (`rgb(209, 36, 47)`, `'solid'`, `'1px'`, `'none'`) are consistent across tasks.

## Notes / sequencing

- Hard ordering: Task 1 → 2 → 3 (publish) → 4 (etherpad-go lockfile needs 0.0.14 live) → 5.
- Task 3 depends on an external CI run (npm publish via OIDC) — Task 4 cannot complete its lockfile step until `npm view ... version` shows `0.0.14`.
- Tasks 1–3 are in the `webcomponents` repo; Tasks 4–5 in `etherpad-go`. Commit in the correct repo.
