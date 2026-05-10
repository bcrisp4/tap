# M8 — SPA polish

**Status:** Draft, awaiting review.

## Context

Tap is the self-hosted feed reader described in [`../concept.md`](../concept.md), with the
visual identity and detailed design in [`../../ui_design/`](../../ui_design/). M8 is the
eighth of twelve milestones — see [`../roadmap.md`](../roadmap.md).

M1 shipped the thinnest possible SPA: light theme only, desktop layout only, no keyboard
shortcuts, no animations beyond CSS hover, no mobile breakpoints. Every visual and UX gap
was explicitly deferred to M8. M6 and M7 have since added auth, sessions, TOTP, passkeys,
and per-user isolation; those milestones shipped functional views (Login, Settings/Security,
Admin) that are unstyled relative to the design system. M8 closes all of this in one pass:
it completes the design system implementation and applies it uniformly across every view the
SPA now contains.

The design source of truth is `ui_design/styles.css`, `ui_design/README.md`, and
`ui_design/Tap - Design Spec.md`. M8 is a faithful implementation of a fully-specified
design, not a design-from-scratch exercise.

## Goal

After M8, Tap's SPA matches the `ui_design/` reference at high fidelity across all three
themes (light, dark, sepia) and in system-tracking mode, on both desktop and mobile
viewports. Every view is keyboard-navigable on desktop. Mobile users get gesture-driven
interaction (swipe to read/save, pull-to-refresh) and a bottom tab bar. The accessibility
pass ensures WCAG AA compliance throughout. Font files are self-hosted — no runtime
requests to Google Fonts or any other third party.

## In scope

### Font self-hosting

Remove the `@import url('https://fonts.googleapis.com/...')` from `styles.css`. Replace
with `@fontsource` npm packages:

```
@fontsource-variable/source-serif-4
@fontsource-variable/inter-tight
@fontsource/jetbrains-mono
```

Import each in `web/src/app.css` (or a dedicated `web/src/lib/fonts.css`). Vite bundles
the woff2 files into `web/dist` at build time. Zero runtime third-party requests.

The fontsource packages provide variable-font builds where available (Source Serif 4 and
Inter Tight are variable; JetBrains Mono ships fixed-weight builds at 400 and 500). Subset
selection: latin only, matching the current Google Fonts subset, to keep bundle size
reasonable.

### Theming system

Four preference values stored in `localStorage` under the key `tap.theme`:
`light` | `dark` | `sepia` | `system`.

- On mount, `App.svelte` reads the stored preference (defaulting to `system` on first
  visit) and applies the resolved theme class (`theme-light`, `theme-dark`, or
  `theme-sepia`) to `<html>`.
- When the preference is `system`, a `window.matchMedia('(prefers-color-scheme: dark)')`
  listener re-applies the class if the OS theme changes at runtime. No page reload.
- Theme switches are instant (CSS custom-property swap, no transition) per the design spec.
- The resolved class is reactive: a Svelte 5 `$state` in a dedicated `theme.svelte.ts`
  store; `App.svelte` derives the `<html>` class from it via a `$effect`.

Design tokens are already correct in `ui_design/styles.css`. M8 imports that file
(or its contents) verbatim — no token values are changed.

### Font toggle

Preference stored in `localStorage` under `tap.font`: `serif` | `sans`. Default `serif`.

The active font class (`font-serif` | `font-sans`) is applied to `<html>` alongside the
theme class. The `font-sans` class sets `--serif: var(--sans)` so article content switches
family without touching size or line-height.

This follows the pattern already defined in `styles.css`:
```css
.tap[style*="--sans"] .reader-title,
.tap[style*="--sans"] .reader-p { font-family: var(--sans); }
```
M8 formalises this as a `font-sans` class on `<html>` rather than an inline style override.

### Density toggle

Preference stored in `localStorage` under `tap.density`: `compact` | `default` |
`comfortable`. Default `default`.

The active density class (`density-compact` | `density-comfortable`; `default` adds no
class) is applied to `<html>`. The CSS classes are already defined in `styles.css`:
- `density-compact`: hides `.entry .summary`, tighter padding (`10px` top/bottom),
  junction dot repositioned to match.
- `density-comfortable`: `16px` top/bottom padding on `.entry`.
- `default`: `14px` top/bottom padding (current `styles.css` baseline), summary visible.

Summary visibility is tied to density: compact = no summary. There is no independent
summary toggle.

### Appearance section in Settings

`Settings.svelte` (registered at `/settings` by M7) gains an Appearance section alongside
the Security section M7 shipped. The section contains three controls:

| Control | Values | Storage key |
|---|---|---|
| Theme | Light / Dark / Sepia / System | `tap.theme` |
| Font | Serif / Sans | `tap.font` |
| Density | Compact / Default / Comfortable | `tap.density` |

The Settings layout uses a sidebar-tabs pattern within the view: a left column of section
links (Appearance, Security; admin users also see a link to the User Management view) and a
right content area. This is a within-view layout, not sub-routes — `/settings` remains the
only route.

### Desktop layout — reader view

M1's reader (`/entry/:id`) hid the sidebar. M8 restores the sidebar in the reader view.
Desktop layout in the reader:

```
┌──────────────┬──────────────────────────────────────────┐
│ Sidebar      │ Reader pane (flex)                       │
│ 240 px       │  sticky header: ‹ UNREAD  MARK · SAVED  │
│              │  reader body max-width 680px centred     │
└──────────────┴──────────────────────────────────────────┘
```

The sidebar is the same component used in the unread view. No list-rail column — content is
front and centre, entry list is not visible when reading.

### Keyboard shortcuts

A single document-level `keydown` listener mounted in `App.svelte`. All bindings are
suppressed when the event target is an `<input>`, `<textarea>`, `<select>`, or any element
with `contenteditable`.

| Key | Action | Context |
|---|---|---|
| `j` / `↓` | Next entry | Unread / Saved / History list |
| `k` / `↑` | Previous entry | Unread / Saved / History list |
| `o` / `Enter` | Open focused entry | List views |
| `m` | Toggle read on focused entry | List views + Reader |
| `s` | Toggle saved on focused entry | List views + Reader |
| `v` | View original (new tab) | Reader |
| `Esc` | Back to list from reader; close open modal | Global |
| `?` | Open hotkeys modal | Global |
| `/` | Focus search input (no-op until M9) | Global |

The active view exposes its handler interface via Svelte context so `App.svelte`'s key
handler can dispatch list-navigation actions (next/prev/open) to whichever list view is
mounted. The handler is typed; M9's search view will implement the `focusSearch()` side of
the context contract.

**`/` keybinding is a known stub in M8.** It is registered and suppressed inside form
controls, but calls `focusSearch()` which is a no-op function until M9 wires it up. This
is documented here so M9 knows to implement the contract, not add a new binding.

#### Hotkeys modal

Triggered by `?`. A `<dialog>`-based modal rendered at the app root via a `$state` boolean
in `App.svelte`. Closed by `Esc` or clicking outside. Uses the `.tap-modal`,
`.tap-modal-head`, `.tap-modal-body`, `.shortcut-row`, `.shortcut-keys`, and `.kbd` classes
already defined in `styles.css`. Two-column grid of shortcut groups matching the design.
Focus is trapped inside the modal while open (standard `<dialog>` focus-trap behaviour).

### Mobile layout

**Breakpoint:** `≤ 768px` switches to the mobile layout. Below this breakpoint:
- Sidebar is hidden.
- Bottom tab bar replaces sidebar navigation.
- Entry row padding adjusts per `styles.css` `.is-mobile` rules.

#### Bottom tab bar

Fixed to the bottom of the viewport. Four tabs: **Unread**, **Saved**, **Search**,
**Settings**. The Search tab is present but shows an empty state until M9 ships.

Spec from `styles.css` and `ui_design/README.md`:
- `grid-template-columns: repeat(4, 1fr)`, `min-height: 48px`.
- `padding: 10px 0 calc(env(safe-area-inset-bottom, 0px) + 14px)`.
- Each tab: 24×24 line icon + 11px Inter Tight label, 5px gap.
- Active tab: Klein junction dot (4px, `--accent`) at top edge + `--ink` colour.
- `border-top: 1px solid var(--rule)`.

#### Mobile top bar

- Unread view top bar: wordmark + `· unread` + count + search icon button.
  `padding-top: 60px` to clear device status bar.
- Reader top bar: 40×40 back button (chevron-left) + reading-progress bar (2px, full-width,
  `--rule` track, `--accent` fill at scroll position) + 40×40 save action.
  Same `padding-top: 60px`.

#### Mobile reader body

Padding `22px 22px 28px`. Typography steps down: title `28px`, body `p` `16/1.65`,
`h2` `20px` — all defined in `styles.css` `.is-mobile` overrides.

#### Mobile reader bottom action bar

Three-column grid: **Mark unread / Saved / Original**. `min-height: 52px` per button.
`padding: 8px 8px calc(env(safe-area-inset-bottom, 0px) + 12px)`.
Active Saved button: Klein colour + filled bookmark glyph.

### Swipe gestures

A reusable `useSwipe` action (Svelte `{@attach}` directive) that:
1. Listens to `touchstart` / `touchmove` / `touchend`.
2. Requires ≥ 40px horizontal travel AND angle from horizontal < 30° before firing.
3. Emits `swipeleft` or `swiperight` custom events.

**Unread list rows:** `swiperight` → toggle read; `swipeleft` → toggle saved. A brief
visual affordance (row slides slightly in the swipe direction, snaps back) confirms the
gesture. The mutation is optimistic, matching the existing mark-read behaviour.

**Reader view:** `swiperight` → previous entry; `swipeleft` → next entry. Navigates via
the existing router. The gesture target is the reader body container, not individual
elements.

The angle gate is the only defence against accidental trigger during vertical scrolling.
40px / 30° are implementation defaults; no configuration exposed to users.

### Pull-to-refresh

On mobile only (hidden on desktop). Applied to the unread list container.

Behaviour:
1. User pulls down past a 60px threshold while scrolled to the top.
2. A loading indicator appears (Klein pulse dot, same animation as the poll strip).
3. `POST /api/v1/subscriptions/poke` (or equivalent refresh trigger) fires.
4. Indicator dismissed on response.

The pull gesture is vertical; no angle gate needed. The 60px threshold is implementation
judgment. Pull-to-refresh is suppressed if a refresh is already in flight.

Infinite scroll is **deferred to M9**.

### Animations

Per the design spec: minimal and functional, never decorative.

| Element | Animation |
|---|---|
| Hover / active state changes | `transition: background 120ms ease` (already in `styles.css`) |
| Junction dot pulse (poll strip) | `tap-pulse` keyframe already in `styles.css` (2.4s ease-in-out, opacity 1→0.35, scale 1→0.8) |
| Theme switch | Instant — no transition |
| Modal open/close | `opacity` fade, 150ms ease |
| Swipe affordance | `transform: translateX` snap-back, 120ms ease |
| Pull-to-refresh indicator appear/dismiss | `opacity` fade, 120ms ease |

No decorative motion. No scroll-triggered animations.

### Accessibility pass

Applied across all views, not just new ones.

- **Focus rings:** `outline: 2px solid var(--accent); outline-offset: 2px` on
  `:focus-visible` — already in `styles.css`, verified on every interactive element.
- **Semantic HTML:** `<nav>`, `<main>`, `<header>`, `<button>` (not `<div>` with
  `onclick`), `<dialog>` for modals, `<ul>`/`<li>` for lists.
- **ARIA labels:** icon-only buttons get `aria-label`. The hotkeys modal gets
  `aria-labelledby`. The tab bar tabs get `aria-current="page"` on the active tab.
- **Colour contrast:** Klein Blue `#002FA7` on `#fafaf7` (light) passes AA at all text
  sizes. Dark theme `#5a7fdc` on `#0d0d0e` — verify AA at implementation time via
  browser DevTools or a contrast checker; adjust the dark accent token if it falls short.
- **Keyboard nav:** every interactive element reachable by `Tab` in logical DOM order.
  Modal focus trap via `<dialog>` native behaviour. Swipe gestures have keyboard equivalents
  (`m` / `s`) — no gesture-only interactions.
- **`prefers-reduced-motion`:** if set, all `transition` and `animation` durations collapse
  to `0ms` via a `@media (prefers-reduced-motion: reduce)` block.
- **Screen reader:** entry list uses `role="list"`, entries `role="listitem"`. Unread
  indicator dot is `aria-hidden="true"` (decorative). Read/saved state is communicated via
  `aria-label` on the toggle buttons (e.g. `aria-label="Mark as read"`).

### Styling pass on M7 views

M7 shipped functional but unstyled views: the TOTP second step in `Login.svelte`, the
Security section of `Settings.svelte`, and `Admin.svelte`. M8 applies the full design
system to these:

- **Login TOTP step:** 6-digit code input styled as a mono input group matching the
  design's form patterns. "Use a recovery code instead" toggle as a secondary text link.
- **Settings / Security section:** session table uses the design's table pattern (rule
  borders, mono meta, `--ink-3` secondary text). TOTP and passkey flows use modal chrome
  from `.tap-modal`. QR code modal centred.
- **Admin view:** user list table using the same table pattern. Role, disabled, and
  has-TOTP state shown as small mono badges. Destructive actions (Delete, Disable) use a
  warning colour (`#c97a1a` on light, `#f0a655` on dark — already defined as the feed
  warning colour in `styles.css`).

No new API surfaces or routes. Styling pass only.

## Out of scope (deferred)

| Concern | Lands in |
|---|---|
| Infinite scroll (cursor-driven scroll-driven entry loading) | M9 |
| Search view implementation | M9 |
| OPML import/export UI | M9 |
| Categories UI | M9 |
| Service worker, offline shell, mutation queue | M10 |
| PWA manifest, `theme_color`, maskable icons | M10 (must agree with M8 theme tokens — coordinate at M10 spec time) |
| Per-user iframe-host allowlist settings UI | Post-M9 (deferred items) |
| SVG support in media proxy | Post-M9 (deferred items) |

## Risks and open questions

**M8 ↔ M9 — `/` keybinding stub.** M8 registers the `/` binding but it calls a no-op
`focusSearch()`. M9 must export a `focusSearch()` function via a named export from
`web/src/lib/search.svelte.ts` (or equivalent search store) that M8's handler imports and
calls. The interface is: a zero-argument function that focuses the search input and is a
no-op when the search input is not mounted. M9 owns the implementation; M8 owns the call
site.
If M9 changes the contract, M9 must update `App.svelte`.

**M8 ↔ M10 — `theme_color` in PWA manifest.** M10 ships the PWA manifest. The
`theme_color` field must agree with one of M8's themes. Since theme is per-device
(`localStorage`), the manifest cannot bake in one colour statically without it being wrong
half the time. M10 must resolve this dynamically (e.g. a `<meta name="theme-color">`
updated by the same `$effect` that sets the `<html>` class). M8 ships the `$effect` for
the class; M10 extends it to also update the meta tag. Document this in M10's spec.

**M8 ↔ M10 — auto-mark-read trigger semantics.** Concept §6.13 says auto-mark-read fires
once per entry per page lifetime on reader open. M8 owns the keyboard/swipe surface; M10
owns the offline mutation enqueue. Both must agree: the trigger is `onMount` of the reader
component, fires the PATCH immediately (M8), and in M10 that PATCH goes through the
mutation queue when offline. No change to the trigger itself — M10 wraps the transport,
not the trigger.

**Colour contrast — dark theme accent.** `#5a7fdc` on `#0d0d0e` must be checked at
implementation time. The design notes it as "desaturated Klein for AA contrast" but does
not give a measured ratio. If it fails AA at small text sizes, the token value should be
lightened slightly. The fix is a one-token change in `styles.css`.

**`fontsource` bundle size.** Source Serif 4 variable + Inter Tight variable + JetBrains
Mono fixed will add roughly 400–600 KB of woff2 to `web/dist` (latin subset, compressed).
This is acceptable for a self-hosted app. Verify with `pnpm --dir web build` and check the
bundle report. If the size is unexpectedly large, subset more aggressively via fontsource's
unicode-range options.

**`<dialog>` support.** Native `<dialog>` has full support in all modern browsers. Tap has
no stated legacy browser requirement, so this is safe. If a polyfill is ever needed,
`dialog-polyfill` is the standard choice.

**Swipe conflict with browser back gesture.** On iOS Safari, a right-edge swipe navigates
back. The `useSwipe` action starts tracking on `touchstart` anywhere in the element; if the
touch begins near the screen edge, it may conflict with the system gesture. Mitigate by
checking `touchstart.touches[0].clientX > 20` before starting tracking — a common
convention for this class of problem.

## Tests and methodology

M8 is built test-first per the project's non-negotiable TDD policy. Pure CSS and design
tokens are exempt. Everything with state, branches, or side-effects is in scope.

### Vitest component tests (in scope for TDD)

**Theme store (`web/src/lib/theme.svelte.ts`):**
- Reads `localStorage` on init; defaults to `system` when absent.
- `system` resolves to `dark` when `matchMedia` returns `dark`, `light` otherwise.
- Writing a preference persists to `localStorage`.
- `matchMedia` change event re-resolves when preference is `system`.
- Does not re-resolve when preference is an explicit value.

**Font store (`web/src/lib/preferences.svelte.ts` or similar):**
- Reads `tap.font` from `localStorage`; defaults to `serif`.
- Writing persists to `localStorage`.

**Density store:**
- Reads `tap.density` from `localStorage`; defaults to `default`.
- Writing persists to `localStorage`.

**Keyboard handler (`web/src/lib/keyboard.ts`):**
- Each binding fires the correct action.
- All bindings are suppressed when `event.target` is `INPUT`, `TEXTAREA`, `SELECT`, or
  `contenteditable`.
- `?` sets hotkeys-modal-open state to `true`.
- `Esc` closes the hotkeys modal; `Esc` in the reader navigates back to list.
- `/` calls `focusSearch()` (test with a spy; confirm it is called and does not throw).

**Swipe recogniser (`web/src/lib/swipe.ts`):**
- Horizontal travel ≥ 40px + angle < 30° fires `swipeleft` or `swiperight`.
- Travel < 40px fires nothing.
- Angle ≥ 30° fires nothing (vertical-scroll guard).
- Right-edge guard: touch starting at `clientX ≤ 20` fires nothing.
- `swiperight` in reader context calls prev-entry; `swipeleft` calls next-entry.
- `swiperight` on list row calls toggle-read; `swipeleft` calls toggle-saved.

**Pull-to-refresh (`web/src/lib/pulltorefresh.ts`):**
- Pull ≥ 60px while at scroll top fires the refresh callback.
- Pull < 60px fires nothing.
- Second pull while refresh in-flight fires nothing.
- Pull on desktop (no touch events) fires nothing.

### Manual / Playwright visual checks (not TDD, but required for definition of done)

- Visual regression: Unread and Reader views rendered in all three themes at 1280px
  viewport. Compare against `ui_design/` reference artboards.
- Mobile layout: Unread and Reader at 390px viewport (iPhone 14 size). Tab bar present,
  sidebar absent.
- Focus ring visibility: `Tab` through every interactive element in the Unread and Reader
  views; focus ring must be visible in all three themes.
- Colour contrast: Klein Blue on each theme background passes WCAG AA (4.5:1 for normal
  text, 3:1 for large text / UI components). Spot-check dark theme accent `#5a7fdc`.
- `prefers-reduced-motion`: with the media query forced on, no transitions or animations
  play.
- Keyboard navigation: full Unread → Reader → back flow using keyboard only (no mouse).
- Swipe gestures: on a real device or browser DevTools touch simulation, swipe right on a
  list row marks read; swipe left saves; swipe in reader navigates.
- Pull-to-refresh: pull down on mobile unread list triggers the refresh indicator.

### Exempt from TDD

- Design token values in `styles.css`.
- `@font-face` / fontsource import declarations.
- Static layout CSS.
- SVG icon markup.

## Definition of done

1. `pnpm --dir web build` succeeds with no TypeScript errors (`svelte-check` clean).
2. `pnpm --dir web test` passes — all Vitest tests green.
3. `make test` passes — Go test suite unaffected.
4. No `fonts.googleapis.com` requests in the browser Network tab on a cold load.
5. Theme switches (including system-tracking) work without page reload across all three
   themes.
6. Density and font toggles persist across page reload.
7. All keyboard shortcuts work on desktop; suppressed inside form inputs.
8. Hotkeys modal (`?`) opens, shows correct bindings, closes on `Esc` and outside click.
9. On a 390px viewport: sidebar hidden, tab bar visible, swipe gestures functional,
   pull-to-refresh functional.
10. All interactive elements have visible focus rings in all three themes.
11. Colour contrast AA check passes (browser DevTools or axe).
12. `prefers-reduced-motion` collapses all transitions and animations.
13. M7 views (Login TOTP step, Settings/Security, Admin) render correctly within the design
    system.
14. `make build` produces a single static binary that includes the updated SPA assets.

## What this milestone deliberately does not prove

- That search works. (`/` is a registered but no-op binding until M9.)
- That the app works offline. (Service worker and mutation queue are M10.)
- That the PWA manifest is correct. (M10 owns `theme_color` and manifest; M8 provides the
  theme token and the `<meta name="theme-color">` hook but does not write the manifest.)
- That categories, OPML, or feed discovery work. (M9.)
- That infinite scroll works. (M9.)
- That per-user appearance preferences are synced across devices. (Preferences are
  `localStorage`-only, per-device. Server-side sync is not in scope for any current
  milestone.)
