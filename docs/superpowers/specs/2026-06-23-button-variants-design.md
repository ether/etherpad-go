# Button variants + visible borders — design

Date: 2026-06-23
Status: approved (design), pending implementation plan

## Problem

In the running `etherpad-go` app, `<ep-button>` elements (e.g. the Welcome page
"New pad" / "New spreadsheet" buttons) render without a visible border / look
unstyled. The user also wants a complete, conventionally-named variant set
("primary, secondary, etc.") on the shared button component.

Investigation findings (verified):

- The button lives only in the **webcomponents** repo at
  `C:\Users\samue\WebstormProjects\webcomponents` → `src/EpButton.ts`
  (Lit 3 component, `<ep-button>`).
- The **consumed `0.0.13`** package already ships variants `default`, `primary`,
  `ghost`, `icon`, and `default` already carries `border: 1px solid
  var(--middle-color, #d2d2d2)`. So the package is **not** stale.
- There is **no `secondary` variant**. PR #306 showed `variant="secondary"`
  renders unstyled (no matching CSS rule → falls through to the borderless base
  `button`). This is the most likely "no border" trigger where a non-existing
  variant name is used.
- `assets/welcome/main.templ` uses real `<ep-button variant="primary">` and
  `variant="default">`; `ui/src/welcome.ts` imports `EpButton.js`. There is **no
  `<ep-theme>` wrapper** anywhere in etherpad-go, and `ep-button` is not used
  elsewhere in the app.
- The webcomponents repo publishes via npm **Trusted Publishing (OIDC)**,
  tag-based CI (latest commit `f8a7f0a`). Current version: `0.0.13`.
- etherpad-go pins the package via `overrides` + `minimumReleaseAgeExclude` in
  `pnpm-workspace.yaml` (and root/ui `package.json`).

## Goal

1. A complete, conventionally-named variant set on `EpButton`, with a real
   `secondary` variant, robust visible borders, and backward compatibility.
2. The styled buttons actually render correctly in the running etherpad-go app
   (reproduce and fix the "no border" symptom).

Non-goals: redesigning other components; changing the theme system beyond adding
one token; converting non-`ep-button` buttons (admin React, plain `<button>`) to
`ep-button`.

## Variant set (approved)

| variant     | style                                                            |
|-------------|------------------------------------------------------------------|
| `primary`   | filled accent (`--primary-color`), white text. Unchanged.        |
| `secondary` | outlined neutral — today's `default` look (border + neutral text).|
| `default`   | **alias of `secondary`** (kept for backward compatibility).      |
| `danger`    | filled destructive red (`--error-color`, fallback `#d1242f`).    |
| `ghost`     | text-only, hover background. Unchanged.                          |
| `icon`      | square, transparent, hover background. Unchanged.                |

## Design

### Component (`webcomponents/src/EpButton.ts`)

- Widen the `variant` union to
  `'primary' | 'secondary' | 'default' | 'danger' | 'ghost' | 'icon'`
  (reflected property, default value `'secondary'`).
- CSS: make the `secondary`/`default` rules share one selector list so both get
  `color: var(--text-color, #485365)` and
  `border: 1px solid var(--middle-color, #d2d2d2)`. The hardcoded fallbacks
  guarantee a **visible border even without `<ep-theme>`**.
- Add `danger` rules: `background: var(--error-color, #d1242f)`,
  `color: var(--bg-color, #ffffff)`, `border: none`, brightness hover/active
  like `primary`.
- Keep `:host(:not([variant]))` mapping to the secondary/outlined look so a
  variant-less button is always bordered.

### Theme token (`webcomponents/src/EpTheme.ts`)

- Add `--error-color` to `ThemeTokens` and to all four themes (colibris,
  colibris-dark, high-contrast, warm). Use a red consistent with `EpInput`'s
  `#d1242f` for light/colibris; pick contrast-appropriate reds for the others.

### Tests / stories (webcomponents)

- Storybook: a story per variant (primary/secondary/default/danger/ghost/icon)
  for visual review.
- Vitest unit tests: for each variant assert the key computed style —
  `secondary`/`default` have a non-`none` border; `primary`/`danger` have the
  expected background; `default` resolves to the same border as `secondary`.

### Publish

- Bump `0.0.13 → 0.0.14`. Build (`tsc`). Release through the existing OIDC
  Trusted-Publishing CI (create the version tag per that workflow's convention).

### etherpad-go integration

- Bump `@samtv12345/etherpad-webcomponents` `0.0.13 → 0.0.14` in
  `pnpm-workspace.yaml` `overrides`, root `package.json`, and `ui/package.json`;
  regenerate `pnpm-lock.yaml`; update the `minimumReleaseAgeExclude` entry to
  `@samtv12345/etherpad-webcomponents@0.0.14` (the local pnpm release-age gate
  otherwise blocks a same-day version — see
  `reference_ci_pnpm_release_age_gate`).
- **Reproduce and fix the "no border" rendering** in the running app: confirm
  whether the cause is (a) the `ep-button` custom element not upgrading because
  the Welcome bundle that registers it isn't loaded, or (b) missing theme
  tokens. The fallbacks make (a) the more likely cause. Fix accordingly —
  ensure the Welcome page loads the bundle that imports `EpButton.js`, and/or
  wrap the rendered components in `<ep-theme>` so tokens are defined. Verify
  primary renders green and secondary/default shows a border.
- Optional: switch `variant="default"` → `variant="secondary"` in
  `assets/welcome/main.templ` for canonical naming (the alias keeps either
  working).

## Data flow / compatibility

`<ep-button>` is consumed by setting the `variant` attribute; the component reads
CSS custom properties with hardcoded fallbacks, so it is self-sufficient without
a theme. Adding `secondary`/`danger` and aliasing `default` is **backward
compatible** — no existing usage breaks.

## Risks

- Publish flow depends on the OIDC tag-based CI in the webcomponents repo.
- The etherpad-go CI release-age gate can reject a same-day `0.0.14` unless
  `minimumReleaseAgeExclude` is updated.
- The render bug may be an integration issue independent of the variant work —
  hence the explicit reproduce-and-fix step rather than assuming the variant
  addition fixes the visible symptom.

## Verification

- webcomponents: Vitest unit tests + Storybook visual check pass for all variants.
- etherpad-go: Welcome page in the running app shows a bordered secondary/default
  button and a green primary button; no unstyled/borderless buttons.
