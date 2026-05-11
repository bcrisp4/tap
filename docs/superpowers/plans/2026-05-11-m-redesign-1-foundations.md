# M-Redesign-1 (Foundations) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the SPA's sidebar/split-pane chrome with the single centred `.ts-shell` (desktop) and the `.tap.is-mobile` shell (mobile) — both rooted in `AppShell.svelte`. Rewrite Login to the `.tl-root` shell with three modes (password / passkey / OTP-step), ship the primitive component library and global tokens the rest of the redesign milestones (M2–M8) consume, and leave every existing view minimally restyled but visually consistent with the new chrome.

**Architecture:** All chrome flows through `web/src/components/AppShell.svelte` (theme class, mobile detection, layout). Top-tab navigation (desktop) and bottom-tab + More sheet (mobile) replace the sidebar. CSS is per-component scoped `<style>` blocks; only tokens, fonts, focus ring, scrollbar, reduced-motion guard, and the two sanctioned keyframes (`tap-pulse`, `tf-spin`) live in global stylesheets. Existing views (Unread, Reader, Saved, Settings, Admin) keep their logic and stores; only their wrapping chrome and a small set of controls (`<select>` → `Segmented`, ad-hoc `<button>` → `Button`, ad-hoc dialogs → `Dialog`) change in M1.

**Tech Stack:** Svelte 5 (runes), TypeScript, Vite 6, Vitest 4, `@fontsource/*` (already in deps), CSS custom properties. **Not used:** SvelteKit (this is a plain Svelte 5 SPA embedded in a Go binary). No new runtime deps are introduced in M1.

**Umbrella spec:** `docs/specs/2026-05-11-ui-redesign.md`. Specifically §2 (architecture), §3.2 (component decomposition), §3.3 (deletes), §4 (CSS adoption strategy). Pay particular attention to the M-Redesign-1 row in §5.

**Amendment from team-lead (2026-05-11):** umbrella §3.2 collapses two distinct row chromes (`.entry` and `.ts-saved-row`) into one primitive. **Split them.** This plan delivers **`EntryRow.svelte` only** (junction-dot row, density variants, is-selected/is-read/is-saved states). `SavedRow.svelte` is **NOT** in M1's primitive library — it ships in M3.

---

## In-scope (M1)

- Token extension: every CSS custom property listed in brand spec §2 (type scale, spacing scale, radii, motion durations, font-feature-settings).
- Global stylesheets: `tokens.css` (extended), `global.css` (fonts, focus, scrollbar, reduced-motion, keyframes), one optional `print.css`.
- Font verification (not vendoring — per team-lead correction 2026-05-11): the existing `@fontsource/*` package imports in `main.ts` are the canonical mechanism. Add italic 400 for Source Serif 4. Confirm Vite bundles `.woff2` same-origin into `web/dist` and no Google Fonts CDN reference survives the build. Do **not** create `web/src/assets/fonts/`. Do **not** write hand-rolled `@font-face` blocks.
- New chrome components: `AppShell`, `TopTabs`, `AccountAvatar`, `AccountMenu`, `StatusFoot`, `MobileTopBar`, `MobileTabBar`, `MobileMoreSheet`, `SearchOverlay` (stub).
- Primitive library: `Button`, `Field`, `Segmented`, `Chip`, `Popover`, `Dialog`, `KbdChip`, `OtpInput`, `RecoveryCodesGrid`, `EmptyState`, `EntryRow` (rewritten), `GroupHeading`, `FeedAvatar` (existing — restyled to design spec).
- Login rewrite: `.tl-root` shell, three modes (password, passkey, OTP-step). Magic link is explicitly excluded.
- Routing: drop `/search` and `/categories/:id`; add `/categories`, `/feeds`, `/history` stub routes; lift Admin from a conditional inside `/` to its own routes table entry; honour role for Admin.
- Wire existing views (Unread, Reader, Saved, Settings, Admin) into the new shell with **minimal restyling**, defined as: swap native `<select>` for `Segmented`, raw `<button>` for `Button`, ad-hoc modal markup for `Dialog`, remove sidebar-relative chrome. No deep view redesigns (those are M2–M7).
- Stub views: `/categories`, `/feeds`, `/history` each render an `EmptyState` reading "Coming soon — M{4|5|8}" inside the new shell.
- HotkeysModal restyled to the `.tap-modal` two-column shortcut grid.
- `searchOverlay` store + `/` key handler, with overlay component shipping a no-op filter UI (M2 implements the actual filter).
- Deletes: `Sidebar.svelte`, `TopBar.svelte`, `SystemActions.svelte`, `SystemStatus.svelte`, `TabBar.svelte`, `Search.svelte`, `Category.svelte`, `FeedSettingsModal.svelte`, and all their tests under `__tests__/`.
- Service-worker cache invalidation verification (asset paths change wholesale, so a smoke test with a stale SW is mandatory).

## Out-of-scope (handled in later milestones)

- Magic-link sign-in (umbrella §1: out of scope for the rewrite entirely).
- Deep redesign of any existing view — visual polish per view ships in M2–M7.
- Real search filter behaviour (M2).
- `.ts-article` reader anatomy, day-band groupings, mark-on-scroll, measure control (M2).
- `SavedRow.svelte` (M3).
- Categories management page content (M4).
- Feeds management page content (M5).
- Settings numbered eyebrow sections, TOTP enrol redesign, sessions list (M6).
- Admin metric grid + errors table redesign (M7).
- History page content (M8).
- Category reorder backend (`position` column / endpoint) — decided in M4.
- Tests for the per-view content rewrites (each later milestone owns its own tests).

## Files

### Created

- `web/src/components/AppShell.svelte` — wraps theme/font/mobile classes; branches on `isMobile`; renders TopTabs+AccountAvatar+children+StatusFoot or MobileTopBar+children+MobileTabBar+MobileMoreSheet.
- `web/src/components/TopTabs.svelte` — `.ts-nav` wordmark + tab list (Unread / Saved / History / Categories / Feeds / Settings / Admin*).
- `web/src/components/AccountAvatar.svelte` — `.ts-account` floating top-right 20×20 circle, opens AccountMenu.
- `web/src/components/AccountMenu.svelte` — `.tap-popover` with Account / Theme / Log out rows (Log-out tinted).
- `web/src/components/StatusFoot.svelte` — `.ts-foot` status strip (pulse dot · "n unread · polled Xm ago · ? for shortcuts").
- `web/src/components/MobileTopBar.svelte` — wordmark + page title + count, 60px status-bar inset.
- `web/src/components/MobileTabBar.svelte` — `.tmnav` 5-tab bottom bar (Unread · Saved · Feeds · Categories · More).
- `web/src/components/MobileMoreSheet.svelte` — `.tmnav-sheet` bottom sheet (identity + History + Settings + Admin* + Log out).
- `web/src/components/SearchOverlay.svelte` — `/`-triggered no-op overlay; M2 fills in the filter UI.
- `web/src/components/Button.svelte` — `.ts-btn` (variants: default / primary / accent / danger / quiet; optional icon; size).
- `web/src/components/Field.svelte` — `.ts-field` (label + input + optional description; sans or mono variant; `is-stacked`).
- `web/src/components/Segmented.svelte` — `.ts-segmented` (theme/font/density/measure pickers).
- `web/src/components/Chip.svelte` — `.cat-chip` / `.ts-feeds-chip` / `.ts-feed-err-chip` (pill / tag / error / backoff).
- `web/src/components/Popover.svelte` — scrim + positioned content; reused by AccountMenu, future reassign popovers.
- `web/src/components/Dialog.svelte` — `.ts-dialog` (head + body + foot; `is-wide`; `.ts-dialog-warn` callout).
- `web/src/components/KbdChip.svelte` — `.ts-kbd` inline keyboard key.
- `web/src/components/OtpInput.svelte` — `.ts-otp` six-cell mono input with paste handling.
- `web/src/components/RecoveryCodesGrid.svelte` — `.ts-codes` 2-col grid (used/unused state).
- `web/src/components/EmptyState.svelte` — `.ts-empty` (accent dot + serif title + sans sub + optional CTA).
- `web/src/components/EntryRow.svelte` — **rewritten** (was already at this path; replace contents). `.entry` row with junction dot, density + read/saved states.
- `web/src/components/GroupHeading.svelte` — `.ts-group-heading` day-band heading.
- `web/src/views/Categories.svelte` — stub page (EmptyState reading "Coming soon — M4").
- `web/src/views/Feeds.svelte` — stub page ("Coming soon — M5").
- `web/src/views/History.svelte` — stub page ("Coming soon — M8").
- `web/src/lib/searchOverlay.svelte.ts` — store: `{ open, query, scope: 'unread'|'saved'|'all' }`; opened by `/` keystroke from `App.svelte`'s existing handler.
- `web/src/lib/pollStatus.ts` — promoted from `components/PollerStatus.svelte` state. Exports a readable store consumed by `StatusFoot`.
- `web/src/lib/breakpoints.svelte.ts` — **canonical location for `isMobile`** (per reviewer 2026-05-11). Exports `isMobile: Readable<boolean>` driven by a single `matchMedia('(max-width: 768px)')` listener at module-init time. Replaces the local `$state` currently in `App.svelte`. Consumed by `AppShell` (M1), and by M2/M3/M4/M5/M6 plans that reference `isMobile` — those plans must import from this location, not from `preferences.svelte.ts`.
- `web/src/styles/print.css` — optional, `theme-print` stack fallback (low priority; deliver only if it fits without expanding scope).
- Test files (TDD-driven, see "TDD posture" below):
  - `web/src/components/__tests__/AppShell.test.ts`
  - `web/src/components/__tests__/TopTabs.test.ts`
  - `web/src/components/__tests__/AccountAvatar.test.ts`
  - `web/src/components/__tests__/AccountMenu.test.ts`
  - `web/src/components/__tests__/StatusFoot.test.ts`
  - `web/src/components/__tests__/MobileTabBar.test.ts`
  - `web/src/components/__tests__/MobileMoreSheet.test.ts`
  - `web/src/components/__tests__/SearchOverlay.test.ts`
  - `web/src/components/__tests__/Button.test.ts`
  - `web/src/components/__tests__/Segmented.test.ts`
  - `web/src/components/__tests__/Popover.test.ts`
  - `web/src/components/__tests__/Dialog.test.ts`
  - `web/src/components/__tests__/OtpInput.test.ts`
  - `web/src/components/__tests__/EntryRow.test.ts`
  - `web/src/lib/__tests__/searchOverlay.test.ts`
  - `web/src/lib/__tests__/pollStatus.test.ts`
  - `web/src/lib/__tests__/breakpoints.test.ts`
  - `web/src/components/__tests__/EmptyState.test.ts`

### Modified

- `web/src/App.svelte` — replace sidebar/TabBar wrapping with `<AppShell>`; update route table; wire `searchOverlay`.
- `web/src/lib/router.ts` — add `categories` / `feeds` / `history` route names; remove `search` and `category` route names; ensure `unread` / `reader` / `saved` / `settings` / `admin` keep their paths.
- `web/src/lib/router.ts` test (`web/src/lib/__tests__/router.test.ts`) — update for new routes.
- `web/src/lib/preferences.svelte.ts` — (1) add `measure` pref (`narrow` / `comfortable` / `wide`) for `.ts-article` consumption in M2; (2) **migrate `density` vocabulary** from `'compact' | 'default' | 'comfortable'` to the brand-spec canonical `'compact' | 'comfortable' | 'cosy'` with `'comfortable'` as the default. Per team-lead decision 2026-05-11: this is the canonical vocabulary across stores, CSS, EntryRow prop, and Settings. Existing `localStorage` values of `'default'` migrate to `'comfortable'` on first read.
- `web/src/lib/__tests__/preferences.test.ts` — add coverage for new `measure` pref + new `density` vocabulary + `'default' → 'comfortable'` migration.
- `web/src/styles/tokens.css` — add full token set from brand spec §2 (type scale vars, spacing scale, radii, motion durations, font-feature-settings; ensure existing theme colour vars + sepia stay intact).
- `web/src/styles/global.css` — pare down to: body reset, `:focus-visible` outline rule, `.tap`-scoped scrollbar styling, `prefers-reduced-motion` guard, `@keyframes tap-pulse`, `@keyframes tf-spin`, plus the two `font-feature-settings` rules. **No `@font-face` blocks** — those come from the `@fontsource*` packages via `main.ts`. Remove all component-level CSS (every selector currently here that owns a component now lives in that component's scoped block).
- `web/src/components/HotkeysModal.svelte` — restyle to the `.tap-modal` two-column shortcut grid per brand spec §7.
- `web/src/components/__tests__/HotkeysModal.test.ts` — update DOM expectations.
- `web/src/components/FeedAvatar.svelte` — confirm 14×14 default + 3px radius (per design); minor style tweak only.
- (Note: `PollerStatus.svelte` and its test are not "modified" — they are deleted outright. See the Deleted section. The replacement is `lib/pollStatus.ts` + `StatusFoot.svelte`, both created in Groups D and I. No shim, no re-export.)
- `web/src/views/Login.svelte` — full rewrite to `.tl-root` shell with three modes; reuses Field, Button, KbdChip, OtpInput primitives.
- `web/src/views/__tests__/Login.test.ts` — rewrite to cover mode switching, OTP entry, passkey button visibility.
- `web/src/views/Unread.svelte` — remove `<Sidebar>` and `<TopBar>`; wrap content in `<AppShell>` (via App.svelte); replace any native `<select>` density/font UI with `Segmented`. Keep keyboard handlers, mark-all-read, refresh.
- `web/src/views/__tests__/Unread.test.ts` — adjust DOM queries for removed chrome.
- `web/src/views/__tests__/UnreadMarkAll.test.ts` — same as above; verify behaviour preserved.
- `web/src/views/Reader.svelte` — remove sidebar; wrap content in `<AppShell>`; ensure back-row works against new shell. **No** `.ts-article` rebuild here — that's M2.
- `web/src/views/__tests__/Reader.test.ts` — adjust DOM queries.
- `web/src/views/Saved.svelte` — remove sidebar; wrap in `<AppShell>`; tidy any controls to use `Button`. **No** `.ts-saved-row` rebuild — that's M3.
- `web/src/views/__tests__/Saved.test.ts` — adjust DOM queries.
- `web/src/views/Settings.svelte` — remove sidebar; wrap in `<AppShell>`. Swap any native `<select>` for `Segmented` (theme/font/density) and ad-hoc dialogs for `Dialog`. Numbered eyebrow rebuild lives in M6.
- `web/src/views/__tests__/Settings.test.ts` — adjust DOM queries; preserve security/TOTP coverage.
- `web/src/views/Admin.svelte` — remove sidebar; wrap in `<AppShell>`. Metric grid lives in M7.
- `web/src/lib/keyboard.ts` — extend `KeyboardContext` with `onSearchOpen` for `/` (already exists in App.svelte; centralise it). Add `G U / G S / G F / G C / G ,` go-to-route sequences as a stretch goal **only if** they fit cleanly without a refactor; otherwise defer to M2. Default: leave for M2.
- `web/src/lib/__tests__/keyboard.test.ts` — add coverage for `/` only (sequences deferred).
- `web/package.json` — confirm `@fontsource-variable/source-serif-4`, `@fontsource-variable/inter-tight`, `@fontsource/jetbrains-mono` are present (they are; verify versions in lockfile and update if missing weights).
- `web/vite.config.ts` — no change expected; verify PWA manifest icons and asset paths still resolve after the SPA reshuffle.

### Deleted (per umbrella §3.3)

- `web/src/components/Sidebar.svelte`
- `web/src/components/TopBar.svelte`
- `web/src/components/SystemActions.svelte`
- `web/src/components/SystemStatus.svelte`
- `web/src/components/TabBar.svelte`
- `web/src/views/Search.svelte`
- `web/src/views/Category.svelte`
- `web/src/components/FeedSettingsModal.svelte`
- `web/src/components/__tests__/Sidebar.test.ts`
- `web/src/components/__tests__/TopBar.test.ts`
- `web/src/components/__tests__/SystemActions.test.ts`
- `web/src/components/__tests__/TabBar.test.ts`
- `web/src/components/__tests__/FeedSettingsModal.test.ts`

`web/src/components/HotkeysModal.svelte` is **kept** (restyled). `web/src/components/PollerStatus.svelte` is **deleted** in favour of `lib/pollStatus.ts` + `StatusFoot`. `web/src/views/Login.svelte` is **kept** (rewritten). `web/src/views/Settings.svelte`, `Admin.svelte`, `Unread.svelte`, `Reader.svelte`, `Saved.svelte` are **kept** (wrapped in new shell with minimal control swaps).

---

## TDD posture per task

Per `CLAUDE.md`: TDD non-negotiable on branches, state, error handling; pure CSS/markup is exempt. Posture below maps to the task groups in the next section.

**TDD-required (red → green → refactor):**

- `AccountMenu` open/close + click-outside dismissal.
- `Popover` open/close + click-outside dismissal + Esc dismissal.
- `Dialog` focus trap + Esc + scrim click + close button.
- `Segmented` selection update fires `onChange` once.
- `OtpInput` paste of 6 digits distributes across cells; arrow-key cell navigation; Backspace empties + moves left.
- `SearchOverlay` opens on `/`, closes on Esc, debounces query updates (debounce stub, no real filter).
- `MobileMoreSheet` open/close, item dispatch, Log out path.
- `MobileTabBar` active-tab derivation when route is `history` or `settings` (collapses to More).
- `EntryRow` `is-read` tone-down; `is-saved` indicator; click navigates to `/entry/:id`; density-variant CSS class.
- `searchOverlay` store: open/close, query update, scope change.
- `pollStatus` store: initial poll, interval setup, error handling (network failure leaves prior value alone).
- `breakpoints.isMobile` store: initial value from `matchMedia('(max-width: 768px)')`; updates on `change` event; subscribers receive new value.
- `EmptyState` discriminator: renders string `sub` directly; renders Snippet `sub` via `{@render}`.
- `Login` mode transitions (`password` → `otp` after `auth.login` returns `totp_required`); passkey button shown only when `'credentials' in navigator`; recovery-code toggle inside OTP mode.
- Router parse/format for `/categories` / `/feeds` / `/history` / `/sign-in`; absence of `/search` and `/categories/:id`.
- App.svelte auth-redirect contract: unauthenticated on a non-signin route → push `/sign-in`; authenticated on `/sign-in` → push `/`.
- `preferences.measure` round-trip via `localStorage` (mirror existing pref tests).
- HotkeysModal `?` opens; Esc closes; click-outside closes.

**Exempt (CSS/markup-only or pure shell layout):**

- `AppShell` (no state beyond consuming `isMobile` from `lib/breakpoints.svelte.ts`; store has its own test).
- `TopTabs` (renders `route` store; existing route tests cover navigation).
- `StatusFoot` (consumes `pollStatus` store; store tests cover behaviour).
- `MobileTopBar` (pure markup).
- `Button`, `KbdChip`, `Chip`, `EmptyState`, `GroupHeading`, `RecoveryCodesGrid`, `Field` (pure markup unless a behaviour is added).

**Pure scaffolding (no tests required):**

- Token additions in `tokens.css`.
- Global stylesheet pruning in `global.css`.
- Font-face declarations.
- `print.css` (if shipped).

---

## Skills and tools for implementers

Invoke these as you work — they capture conventions you'd otherwise miss. Skills marked **MANDATORY** are required by `CLAUDE.md` or by this plan.

- `superpowers:writing-plans` — already used to author this plan. **Do not invoke again during execution.**
- `superpowers:test-driven-development` — **MANDATORY** before every TDD task in the section above. Drives the red-green-refactor loop.
- `superpowers:verification-before-completion` — **MANDATORY** before claiming any task complete. Run the verification commands listed under "Verification" below and confirm they pass before marking checkboxes done.
- `svelte-runes` — invoke before touching reactive state in `AppShell.svelte`, `SearchOverlay.svelte`, `OtpInput.svelte`, `Dialog.svelte`, `Login.svelte`, and any new store. Locks in `$state` / `$derived` / `$effect` usage so you don't accidentally write Svelte 4 patterns.
- `svelte-styling` — invoke before writing the first scoped `<style>` block. Critical for `:global(html.theme-dark)` overrides inside scoped components; without it implementers will fight Svelte's CSS scoping and either produce dead rules or leak selectors to the global scope.
- `svelte-components` — invoke before designing `Popover.svelte` and `Dialog.svelte` (they're the only components in M1 that follow the headless-primitive pattern; everything else is a simple presentational component).
- `svelte-template-directives` — invoke when wiring focus trap in `Dialog.svelte` or auto-focus in `OtpInput.svelte` (`{@attach focus()}` replaces use-actions).
- `tdd` — same as `superpowers:test-driven-development` but the project-local variant; either is fine.
- `frontend-design:frontend-design` — invoke once at the start of the primitive library task block. Sets the bar for the visual fidelity expected against `ui_design/styles.css`.
- **NOT applicable, do not invoke:** `sveltekit-structure`, `sveltekit-data-flow`, `sveltekit-remote-functions`. The Tap web SPA is plain Svelte 5 embedded in a Go binary — there is no SvelteKit, no SSR, no `+page.ts` files. Implementers who pattern-match to SvelteKit will waste hours.
- **MCP (optional, on demand):** `mcp__plugin_context7_context7__resolve-library-id` then `query-docs` for current Svelte 5, Vite 6, Vitest 4, or `@fontsource` docs if a behaviour question comes up. Do not preemptively fetch; only on demand.
- **MCP (mandatory for manual smoke):** `mcp__plugin_playwright_playwright__*` — used in the final "Verification" task to exercise the dev server end-to-end. Specifically `browser_navigate`, `browser_snapshot`, `browser_press_key`, `browser_take_screenshot` against `http://localhost:5173` after `make dev` is running.

---

## Step-by-step tasks

Tasks are grouped by sub-area. Within each group, complete in order. Tasks across groups can overlap if they don't share files; default to sequential within a single session.

### Group A — Tokens

**Files:** `web/src/styles/tokens.css`

Brand spec §2 has the authoritative list; cross-reference it as you go.

- [ ] **A1. Add type scale custom properties** to `:root` in `tokens.css`. The current file only sets font-family stacks. Append:

```css
:root {
  /* ── Type scale (brand spec §2.2) ── */
  --fs-article-title:        38px;
  --fs-article-title-mobile: 28px;
  --fs-page-title:           38px;
  --fs-section-title:        28px;
  --fs-article-h2:           22px;
  --fs-lede:                 19px;
  --fs-entry-title:          19px;   /* 17–19 per spec; use 19 for desktop */
  --fs-entry-title-mobile:   16px;
  --fs-entry-summary:        14.5px;
  --fs-article-body:         17px;
  --fs-settings-label:       17px;
  --fs-ui-body:              13px;
  --fs-button:               12.5px;
  --fs-tab-nav:              11px;
  --fs-eyebrow:              10px;
  --fs-section-eyebrow:      10px;
  --fs-meta:                 10.5px;
  --fs-tiny-tag:             9.5px;
  --fs-kbd:                  10.5px;

  /* ── Line heights ── */
  --lh-article-title: 1.12;
  --lh-page-title:    1.05;
  --lh-h2:            1.25;
  --lh-lede:          1.55;
  --lh-entry-title:   1.3;
  --lh-entry-summary: 1.55;
  --lh-article-body:  1.7;
  --lh-ui-body:       1.4;

  /* ── Tracking ── */
  --tr-display:       -0.02em;
  --tr-entry-title:   -0.005em;
  --tr-h2:            -0.01em;
  --tr-tab-nav:        0.10em;
  --tr-eyebrow:        0.14em;
  --tr-section-eyebrow:0.16em;
  --tr-meta:           0.02em;
  --tr-tiny-tag:       0.08em;

  /* ── Spacing scale (4-pt grid, brand spec §2.3) ── */
  --sp-1:  4px;  --sp-2: 6px;  --sp-3: 8px;  --sp-4: 10px;
  --sp-5: 12px;  --sp-6: 14px; --sp-7: 16px; --sp-8: 18px;
  --sp-9: 20px;  --sp-10: 22px; --sp-11: 24px; --sp-12: 28px;
  --sp-13: 32px; --sp-14: 36px; --sp-15: 40px; --sp-16: 44px;
  --sp-17: 48px; --sp-18: 56px; --sp-19: 64px; --sp-20: 80px;

  /* ── Measure (article body) ── */
  --measure-narrow:      580px;
  --measure-comfortable: 680px;
  --measure-wide:        760px;

  /* ── Radii ── */
  --radius-chip:   2px;
  --radius-input:  4px;   /* default */
  --radius-dialog: 6px;
  --radius-pill:   100px;

  /* ── Motion ── */
  --dur-fast:   100ms;
  --dur-base:   120ms;
  --ease-base:  ease;

  /* ── Shadows (only on dialogs/popovers — see brand spec §2.4) ── */
  --shadow-dialog:  0 24px 60px rgba(0, 0, 0, 0.32);
  --shadow-popover: 0 10px 30px rgba(0, 0, 0, 0.18);

  /* ── Semi-semantic state colours (state ONLY; brand spec §2.1) ── */
  --color-error-light:   #c43a3a;
  --color-error-dark:    #ec7a7a;
  --color-warning-light: #c97a1a;
  --color-warning-dark:  #f0a655;
}
```

- [ ] **A2. Confirm theme blocks intact.** Read `tokens.css` and verify `.theme-light` defaults, `html.theme-dark`, `html.theme-sepia` still hold the colour vars listed in brand spec §2.1. If a value drifted (e.g. `--accent` got moved), restore it.

- [ ] **A3. Remove the existing `.entry`-related density rules** in `tokens.css` lines 52–57 (the `.density-compact`, `.density-comfortable` blocks). They will be re-implemented in `EntryRow.svelte`'s scoped style. Leave the `html.font-sans { --serif: var(--sans); }` rule — that's a global font-toggle hook still used by AppShell.

- [ ] **A4. Commit.**

```bash
git add web/src/styles/tokens.css
git commit -m "M1-A: extend tokens.css with full design system tokens"
```

### Group B — Fonts (verify, do not vendor)

**Files:** `web/src/main.ts`, `web/src/styles/global.css`.

Per team-lead correction 2026-05-11 (amending umbrella §3.1 + §7 risk 6): the codebase already uses `@fontsource*` packages. Vite bundles their `.woff2` files into `web/dist`, which is `go:embed`ed into the binary. Same-origin is already true. **Do NOT** vendor anything into `web/src/assets/fonts/`. **Do NOT** write hand-rolled `@font-face` blocks — the fontsource packages emit them via their imported CSS.

Group B's job: confirm the existing imports cover every weight/style the brand spec uses, add font-feature-settings, verify in dev.

- [ ] **B1. Confirm `main.ts` imports.** Read `web/src/main.ts`. The four imports must be present:

```ts
import '@fontsource-variable/source-serif-4';
import '@fontsource-variable/inter-tight';
import '@fontsource/jetbrains-mono/400.css';
import '@fontsource/jetbrains-mono/500.css';
```

- [ ] **B2. Add italic 400 for Source Serif 4.** Brand spec §2.2 calls for italic 400 (article lede, `tap-login.css` quote body). The variable package's default `*.css` does not include italic — add:

```ts
import '@fontsource-variable/source-serif-4/wght-italic.css';
```

- [ ] **B3. Confirm Source Serif 4 optical-size range.** Brand spec §2.2 references "8..60 optical sizes". The `@fontsource-variable/source-serif-4` package ships a variable font with both `wght` and `opsz` axes — confirm by inspecting the imported CSS after the next `pnpm install`:

```bash
grep -r "opsz\|font-variation-settings" node_modules/@fontsource-variable/source-serif-4/ | head
```

Expected: at least one match showing the `opsz` axis. If absent (unlikely), file a follow-up — the article reader (M2) would degrade gracefully to the default optical size; nothing in M1 itself depends on `opsz`.

- [ ] **B4. Add `font-feature-settings`.** Append to `global.css` (the body rule from Group C2 already includes the body version; add the mono rule too). Group C handles the final shape of `global.css` — this step just records the requirement:

```css
body { font-feature-settings: "kern", "liga", "onum"; }
:where([class*="mono"], code, kbd, .ts-kbd, .ts-rt, .ts-tab-count) {
  font-feature-settings: "tnum", "zero";
}
```

If Group C runs before Group B (the order is flexible), confirm those two rules are in `global.css` after Group C ships. Otherwise add them here.

- [ ] **B5. Build the SPA.** Verify no external CDN fetch:

```bash
pnpm --dir web build
grep -r "fonts.googleapis\|fonts.gstatic" web/dist/ || echo "OK: no Google Fonts references"
ls web/dist/assets/ | grep -E '(inter-tight|source-serif|jetbrains)' | head
```

Expected: "OK" from the grep + at least 6 `.woff2` files in the `ls`.

- [ ] **B6. Visual smoke** (deferred to Group P2 — recorded here as a reminder). When running `make dev`, manually confirm:
  - Article body / titles use Source Serif 4 (serif, slightly modulated, ink #1a1a1a).
  - Top tabs / button labels / Inter Tight (sans, tight tracking).
  - `.ts-foot` status line / `.ts-kbd` / timestamps use JetBrains Mono (mono, tabular figures).
  - Italic renders in the article lede (M2 wires this; in M1 the only italic usage is `Uncategorised` in brand spec §6.4 mock — not yet shipped, so M1 has no visible italic surface. The italic import still needs to be in place so M2 can use it without another rebuild.)

- [ ] **B7. Commit.**

```bash
git add web/src/main.ts
git commit -m "M1-B: import Source Serif 4 italic; verify same-origin font bundle"
```

### Group C — Global stylesheet pruning

**Files:** `web/src/styles/global.css`.

The existing `global.css` contains a lot of component-level CSS (`.entry`, `.tap-topbar`, `.tap-sidebar`, `.reader-pane`, `.tap-modal`, `.tap-popover`, etc.) that will now live in scoped Svelte `<style>` blocks. Pare `global.css` down to: body reset, `:focus-visible`, `.tap`-scoped scrollbar, `prefers-reduced-motion`, `@keyframes tap-pulse`, `@keyframes tf-spin`, the two `font-feature-settings` rules from B4. **Font-faces come from `@fontsource*` packages via `main.ts` — do not declare them here.**

- [ ] **C1. Save a backup snapshot** (in your head — git is the actual safety net). The rewrite is destructive; the old rules will be reborn inside scoped component styles in later groups.

- [ ] **C2. Replace `global.css` contents.** Use Write to overwrite the file. The new contents:

```css
/* ──────────────────────────────────────────────────────────────
   Tap — global stylesheet
   Only tokens, reset, focus ring, scrollbar, reduced-motion, and
   sanctioned keyframes live here. Component CSS belongs in each
   component's scoped <style> block. Brand spec §2 + umbrella §3.1.
   ────────────────────────────────────────────────────────────── */

/* Fonts are imported via @fontsource* packages in main.ts.
   They emit @font-face declarations with same-origin URLs that Vite
   bundles into web/dist/assets/. Do NOT add Google Fonts <link> tags
   or @import url(...) — the binary must work offline (brand spec §1.3). */

*,
*::before,
*::after {
  box-sizing: border-box;
}

html, body {
  margin: 0;
  padding: 0;
  background: var(--bg);
  color: var(--ink);
  font-family: var(--sans);
  font-size: 14px;
  -webkit-font-smoothing: antialiased;
  text-rendering: optimizeLegibility;
  font-feature-settings: "kern", "liga", "onum";
}

button {
  font-family: inherit;
  font-size: inherit;
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  color: inherit;
}

a {
  color: var(--accent);
  text-decoration: none;
}

:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

:where([class*="mono"], code, kbd, .ts-kbd, .ts-rt, .ts-tab-count) {
  font-feature-settings: "tnum", "zero";
}

/* Scrollbar — only inside .tap; OS default elsewhere */
.tap *::-webkit-scrollbar { width: 8px; height: 8px; }
.tap *::-webkit-scrollbar-thumb { background: var(--ink-4); border-radius: 4px; }
.tap *::-webkit-scrollbar-track { background: transparent; }

/* Sanctioned keyframes (brand spec §2.5) */
@keyframes tap-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50%      { opacity: 0.35; transform: scale(0.8); }
}
@keyframes tf-spin { to { transform: rotate(360deg); } }

/* Reduced motion */
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    transition-duration: 0ms !important;
    animation-duration: 0ms !important;
    animation-iteration-count: 1 !important;
  }
}
```

- [ ] **C3. Smoke build to confirm no missing global rule breaks the page.** Run:

```bash
pnpm --dir web run check
```

Expected: passes. Type-checker doesn't know about CSS class collisions, so this only proves TS compiles. Defer the visual smoke to Group P.

- [ ] **C4. Commit.**

```bash
git add web/src/styles/global.css
git commit -m "M1-C: prune global.css to reset/focus/scrollbar/keyframes only"
```

### Group D — Stores: `searchOverlay`, `pollStatus`, `breakpoints`

**Files:** `web/src/lib/searchOverlay.svelte.ts`, `web/src/lib/pollStatus.ts`, `web/src/lib/breakpoints.svelte.ts`, plus matching tests.

These three stores back `SearchOverlay.svelte`, `StatusFoot.svelte`, and `AppShell.svelte` respectively. Invoke `svelte-runes` skill before writing — `.svelte.ts` files use runes outside components, same pattern as `preferences.svelte.ts`. `breakpoints.svelte.ts` is the canonical location for `isMobile` per reviewer 2026-05-11 — it replaces the existing local `$state` in `App.svelte` and is consumed by every downstream redesign plan (M2/M3/M4/M5/M6).

- [ ] **D1. Write the failing `searchOverlay` test.**

`web/src/lib/__tests__/searchOverlay.test.ts`:

```ts
import { describe, it, expect, beforeEach } from 'vitest';
import { searchOverlay } from '../searchOverlay.svelte';

describe('searchOverlay store', () => {
  beforeEach(() => {
    searchOverlay.close();
    searchOverlay.setQuery('');
    searchOverlay.setScope('unread');
  });

  it('opens and closes', () => {
    expect(searchOverlay.open).toBe(false);
    searchOverlay.openOverlay();
    expect(searchOverlay.open).toBe(true);
    searchOverlay.close();
    expect(searchOverlay.open).toBe(false);
  });

  it('tracks query', () => {
    searchOverlay.setQuery('hello');
    expect(searchOverlay.query).toBe('hello');
  });

  it('tracks scope', () => {
    searchOverlay.setScope('saved');
    expect(searchOverlay.scope).toBe('saved');
  });

  it('resets query when closed', () => {
    searchOverlay.openOverlay();
    searchOverlay.setQuery('hello');
    searchOverlay.close();
    expect(searchOverlay.query).toBe('');
  });
});
```

- [ ] **D2. Run test, confirm it fails.**

```bash
pnpm --dir web test -- src/lib/__tests__/searchOverlay.test.ts
```

Expected: fails with "Cannot find module '../searchOverlay.svelte'".

- [ ] **D3. Implement `searchOverlay.svelte.ts`.**

```ts
// .svelte.ts enables Svelte runes outside components.
type Scope = 'unread' | 'saved' | 'all';

function make() {
  let open = $state(false);
  let query = $state('');
  let scope = $state<Scope>('unread');
  return {
    get open() { return open; },
    get query() { return query; },
    get scope() { return scope; },
    openOverlay() { open = true; },
    close() { open = false; query = ''; },
    setQuery(v: string) { query = v; },
    setScope(v: Scope) { scope = v; },
  };
}

export const searchOverlay = make();
```

- [ ] **D4. Run test, confirm green.**

```bash
pnpm --dir web test -- src/lib/__tests__/searchOverlay.test.ts
```

Expected: 4 passing.

- [ ] **D5. Write the failing `pollStatus` test.**

`web/src/lib/__tests__/pollStatus.test.ts`:

```ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { pollStatus, startPollStatus, stopPollStatus } from '../pollStatus';

describe('pollStatus store', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal('fetch', vi.fn());
  });
  afterEach(() => {
    stopPollStatus();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('reports waking-up initially', () => {
    let snapshot: unknown;
    const off = pollStatus.subscribe(s => { snapshot = s; });
    expect(snapshot).toMatchObject({ active: null });
    off();
  });

  it('updates active count after a successful poll', async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ polls_active: 3 }),
    });
    startPollStatus();
    await vi.advanceTimersByTimeAsync(0);
    let snapshot: { active: number | null } | null = null;
    const off = pollStatus.subscribe(s => { snapshot = s as typeof snapshot; });
    expect(snapshot?.active).toBe(3);
    off();
  });

  it('leaves prior value alone on network error', async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce({ ok: true, json: async () => ({ polls_active: 2 }) })
      .mockRejectedValueOnce(new Error('offline'));
    startPollStatus();
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(30000);
    let snapshot: { active: number | null } | null = null;
    const off = pollStatus.subscribe(s => { snapshot = s as typeof snapshot; });
    expect(snapshot?.active).toBe(2);
    off();
  });
});
```

- [ ] **D6. Run test, confirm it fails.**

```bash
pnpm --dir web test -- src/lib/__tests__/pollStatus.test.ts
```

Expected: fails with "Cannot find module '../pollStatus'".

- [ ] **D7. Implement `pollStatus.ts`.**

```ts
import { writable, type Readable } from 'svelte/store';

type Status = { active: number | null };

const internal = writable<Status>({ active: null });

let timer: ReturnType<typeof setTimeout> | undefined;
let running = false;

async function tick() {
  try {
    const r = await fetch('/healthz');
    if (r.ok) {
      const body = await r.json() as { polls_active?: number };
      internal.set({ active: body.polls_active ?? 0 });
    }
  } catch { /* leave value alone */ }
  if (running) {
    timer = setTimeout(tick, 30000);
  }
}

export function startPollStatus() {
  if (running) return;
  running = true;
  void tick();
}

export function stopPollStatus() {
  running = false;
  if (timer) clearTimeout(timer);
  timer = undefined;
}

export const pollStatus: Readable<Status> = { subscribe: internal.subscribe };
```

- [ ] **D8. Run test, confirm green.**

```bash
pnpm --dir web test -- src/lib/__tests__/pollStatus.test.ts
```

Expected: 3 passing.

- [ ] **D9. Write the failing `breakpoints.isMobile` test.** `web/src/lib/__tests__/breakpoints.test.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';

describe('breakpoints.isMobile store', () => {
  let listeners: Array<(e: MediaQueryListEvent) => void>;
  let mql: { matches: boolean; addEventListener: ReturnType<typeof vi.fn>; removeEventListener: ReturnType<typeof vi.fn>; media: string };

  beforeEach(() => {
    listeners = [];
    mql = {
      matches: false,
      media: '(max-width: 768px)',
      addEventListener: vi.fn((_evt: string, fn: (e: MediaQueryListEvent) => void) => listeners.push(fn)),
      removeEventListener: vi.fn(),
    };
    vi.stubGlobal('matchMedia', vi.fn(() => mql));
  });

  it('initial value reflects matchMedia.matches', async () => {
    mql.matches = true;
    const { isMobile } = await import('../breakpoints.svelte?fresh-initial-true' as string);
    expect(get(isMobile)).toBe(true);
  });

  it('initial value false when not mobile', async () => {
    mql.matches = false;
    const { isMobile } = await import('../breakpoints.svelte?fresh-initial-false' as string);
    expect(get(isMobile)).toBe(false);
  });

  it('updates when the media query fires change', async () => {
    mql.matches = false;
    const { isMobile } = await import('../breakpoints.svelte?fresh-change' as string);
    expect(get(isMobile)).toBe(false);
    listeners.forEach(fn => fn({ matches: true } as MediaQueryListEvent));
    expect(get(isMobile)).toBe(true);
  });
});
```

- [ ] **D10. Run, confirm fail.** `pnpm --dir web test -- src/lib/__tests__/breakpoints.test.ts`. Expected: module not found.

- [ ] **D11. Implement `breakpoints.svelte.ts`.**

```ts
import { writable, type Readable } from 'svelte/store';

const MOBILE_QUERY = '(max-width: 768px)';

function makeIsMobile(): Readable<boolean> {
  // SSR guard: if matchMedia is undefined (Node/jsdom without stub), default to false.
  const supportsMQ = typeof window !== 'undefined' && typeof window.matchMedia === 'function';
  const initial = supportsMQ ? window.matchMedia(MOBILE_QUERY).matches : false;
  const { subscribe, set } = writable<boolean>(initial);

  if (supportsMQ) {
    const mql = window.matchMedia(MOBILE_QUERY);
    mql.addEventListener('change', (e: MediaQueryListEvent) => set(e.matches));
  }

  return { subscribe };
}

export const isMobile = makeIsMobile();
```

**Why `writable` not runes:** Svelte runes (`$state`) are scoped to component / `.svelte.ts` modules but they don't satisfy the `Readable<T>` interface that downstream plans (M2/M3/M4/M5/M6) consume via `$isMobile`. A plain `writable` + sealed `subscribe` export gives them the `$store` shape they expect. The filename `breakpoints.svelte.ts` is kept for consistency with the existing `preferences.svelte.ts` (mixed-mode is fine — Vite handles both).

- [ ] **D12. Run, confirm green.** `pnpm --dir web test -- src/lib/__tests__/breakpoints.test.ts`. Expected: 3 passing.

- [ ] **D13. Commit.**

```bash
git add web/src/lib/searchOverlay.svelte.ts web/src/lib/pollStatus.ts web/src/lib/breakpoints.svelte.ts web/src/lib/__tests__/searchOverlay.test.ts web/src/lib/__tests__/pollStatus.test.ts web/src/lib/__tests__/breakpoints.test.ts
git commit -m "M1-D: add searchOverlay, pollStatus, and breakpoints stores (TDD)"
```

### Group E — Preferences (density migration + measure)

**Files:** `web/src/lib/preferences.svelte.ts`, `web/src/lib/__tests__/preferences.test.ts`.

Two changes in one group:

1. **Density vocabulary migration** (per team-lead decision 2026-05-11): the existing `'compact' | 'default' | 'comfortable'` enum becomes the canonical brand-spec `'compact' | 'comfortable' | 'cosy'`. Default flips from `'default'` to `'comfortable'`. Legacy `'default'` values stored in `localStorage` migrate to `'comfortable'` on first read.
2. **New `measure` pref** for article body width (`'narrow' | 'comfortable' | 'wide'`).

- [ ] **E1. Write failing tests for both changes.** Append to `preferences.test.ts`:

```ts
import { describe, it, expect, beforeEach } from 'vitest';

describe('density pref (canonical vocabulary)', () => {
  beforeEach(() => { localStorage.clear(); });

  it('defaults to comfortable when localStorage empty', async () => {
    // Re-import to get a fresh module-level binding after clearing storage.
    const { density } = await import('../preferences.svelte?fresh-density-default' as string);
    expect(density.value).toBe('comfortable');
  });

  it('migrates legacy "default" value to "comfortable"', async () => {
    localStorage.setItem('tap.density', 'default');
    const { density } = await import('../preferences.svelte?fresh-density-migrate' as string);
    expect(density.value).toBe('comfortable');
    // Migration persists the new value so it doesn't fire again.
    expect(localStorage.getItem('tap.density')).toBe('comfortable');
  });

  it('accepts cosy', async () => {
    const { density } = await import('../preferences.svelte?fresh-density-cosy' as string);
    density.value = 'cosy';
    expect(density.value).toBe('cosy');
    expect(localStorage.getItem('tap.density')).toBe('cosy');
  });
});

describe('measure pref', () => {
  beforeEach(() => { localStorage.clear(); });

  it('defaults to comfortable', async () => {
    const { measure } = await import('../preferences.svelte?fresh-measure' as string);
    expect(measure.value).toBe('comfortable');
  });

  it('persists to localStorage', async () => {
    const { measure } = await import('../preferences.svelte?fresh-measure-persist' as string);
    measure.value = 'wide';
    expect(localStorage.getItem('tap.measure')).toBe('wide');
  });
});
```

**Note on the `?fresh-*` query strings:** Vitest's module cache otherwise pins the first read of `localStorage`, so the migration test would never see the legacy value. The query string forces a fresh evaluation. If this proves brittle in the harness, fall back to `vi.resetModules()` + dynamic `import('../preferences.svelte')` inside each `beforeEach`.

- [ ] **E2. Run, confirm fail.** `pnpm --dir web test -- src/lib/__tests__/preferences.test.ts`. Expected: density migration test fails (legacy `'default'` not migrated); `cosy` rejected by the existing enum; `measure` not exported.

- [ ] **E3. Rewrite `preferences.svelte.ts`.** Replace the file's contents. **Load-bearing detail:** the `migrateDensity()` IIFE must execute *before* `export const density = makePref(...)`. The snippet below is correctly ordered (IIFE on line 23 of the snippet, density export on line 31). Do not reorder. If you split the file later (e.g., per-pref modules), make sure migration runs before the pref reads `localStorage`.

```ts
// .svelte.ts enables Svelte runes ($state, $derived) outside components.
type Theme = 'light' | 'dark' | 'sepia' | 'system';
type Font = 'serif' | 'sans';
type Density = 'compact' | 'comfortable' | 'cosy';
type Measure = 'narrow' | 'comfortable' | 'wide';

const THEMES: Theme[] = ['light', 'dark', 'sepia', 'system'];
const FONTS: Font[] = ['serif', 'sans'];
const DENSITIES: Density[] = ['compact', 'comfortable', 'cosy'];
const MEASURES: Measure[] = ['narrow', 'comfortable', 'wide'];

const mq = typeof window !== 'undefined'
  ? window.matchMedia('(prefers-color-scheme: dark)')
  : null;

let prefersDark = $state(mq?.matches ?? false);
if (mq) {
  mq.addEventListener('change', (e) => { prefersDark = e.matches; });
}

function makeTheme() {
  const raw = localStorage.getItem('tap.theme');
  let stored = $state<Theme>(THEMES.includes(raw as Theme) ? (raw as Theme) : 'system');
  const resolved = $derived<'light' | 'dark' | 'sepia'>(
    stored === 'system' ? (prefersDark ? 'dark' : 'light') : stored,
  );
  return {
    get stored() { return stored; },
    set stored(v: Theme) { stored = v; localStorage.setItem('tap.theme', v); },
    get resolved() { return resolved; },
  };
}

function makePref<T extends string>(key: string, def: T, allowed: T[]) {
  const raw = localStorage.getItem(key);
  let value = $state<T>(allowed.includes(raw as T) ? (raw as T) : def);
  return {
    get value() { return value; },
    set value(v: T) { value = v; localStorage.setItem(key, v); },
  };
}

// One-off migration: legacy density value "default" → "comfortable".
// Run before makePref reads, so the legacy value is replaced *before* the
// allowed-list filter would drop it back to the default.
(function migrateDensity() {
  if (typeof localStorage === 'undefined') return;
  const raw = localStorage.getItem('tap.density');
  if (raw === 'default') {
    localStorage.setItem('tap.density', 'comfortable');
  }
})();

export const theme = makeTheme();
export const font = makePref<Font>('tap.font', 'serif', FONTS);
export const density = makePref<Density>('tap.density', 'comfortable', DENSITIES);
export const measure = makePref<Measure>('tap.measure', 'comfortable', MEASURES);
```

- [ ] **E4. Update any existing test in `preferences.test.ts` that referenced the old `'default'` density.** Grep:

```bash
grep -n "density.*default\|'default'" web/src/lib/__tests__/preferences.test.ts
```

For each match, replace `'default'` with `'comfortable'`, except for tests asserting the migration path (which must keep the old value to drive the migration).

- [ ] **E5. Run, confirm green.** `pnpm --dir web test -- src/lib/__tests__/preferences.test.ts`. Expected: all density tests + 2 measure tests + any prior passing test still passes.

- [ ] **E6. Update `App.svelte`'s density class application.** The current code adds `density-compact` / `density-comfortable` classes to `html`. After E3, `density.value` can now also be `'cosy'`. Verify (Group O will rewrite App.svelte; ensure the rewrite there reads as):

```ts
$effect(() => {
  const html = document.documentElement;
  html.classList.remove('density-compact', 'density-comfortable', 'density-cosy');
  html.classList.add(`density-${density.value}`);
});
```

(Note: the rewrite in Group O already drops the old `if (density.value !== 'default')` guard. Confirm it always applies a class.)

- [ ] **E7. Add the density CSS rules to `EntryRow.svelte`.** The rules now live with the row, not the global stylesheet. Confirm the `EntryRow.svelte` scoped block (Group H5c) defines:
  - `.entry.density-compact` — hides summary, 10px padding, junction at `top: 17px` (already in the H5c snippet).
  - `.entry.density-comfortable` — default shape, 16px padding, summary clamped to 2 lines (this IS the default for `.entry`; no extra rule needed beyond the base `.entry`).
  - `.entry.density-cosy` — same as compact for padding/junction but **keeps the summary visible clamped to 1 line** (per brand spec §4.4).

If Group H5c's CSS doesn't yet show `.density-cosy { padding-top: 10px; padding-bottom: 10px; } .density-cosy .summary { -webkit-line-clamp: 1; display: -webkit-box; }`, add it before committing H5.

- [ ] **E8. Commit.**

```bash
git add web/src/lib/preferences.svelte.ts web/src/lib/__tests__/preferences.test.ts
git commit -m "M1-E: migrate density to compact/comfortable/cosy; add measure pref"
```

### Group F — Router routes

**Files:** `web/src/lib/router.ts`, `web/src/lib/__tests__/router.test.ts`.

- [ ] **F1. Read the current router test** to see the existing assertion style. Then write the failing test for the new route table. Replace the contents of `router.test.ts` with:

```ts
import { describe, it, expect, beforeEach } from 'vitest';
import { route, navigate } from '../router';
import { get } from 'svelte/store';

describe('router', () => {
  beforeEach(() => { window.history.replaceState({}, '', '/'); });

  it('parses /', () => {
    navigate('/');
    expect(get(route)).toEqual({ name: 'unread' });
  });

  it('parses /entry/123', () => {
    navigate('/entry/123');
    expect(get(route)).toEqual({ name: 'reader', params: { id: 123 } });
  });

  it('parses /saved', () => {
    navigate('/saved');
    expect(get(route)).toEqual({ name: 'saved' });
  });

  it('parses /categories', () => {
    navigate('/categories');
    expect(get(route)).toEqual({ name: 'categories' });
  });

  it('parses /feeds', () => {
    navigate('/feeds');
    expect(get(route)).toEqual({ name: 'feeds' });
  });

  it('parses /history', () => {
    navigate('/history');
    expect(get(route)).toEqual({ name: 'history' });
  });

  it('parses /settings', () => {
    navigate('/settings');
    expect(get(route)).toEqual({ name: 'settings' });
  });

  it('parses /admin', () => {
    navigate('/admin');
    expect(get(route)).toEqual({ name: 'admin' });
  });

  it('parses /sign-in', () => {
    navigate('/sign-in');
    expect(get(route)).toEqual({ name: 'signin' });
  });

  it('falls back to unread for /search (deleted)', () => {
    navigate('/search');
    expect(get(route)).toEqual({ name: 'unread' });
  });

  it('falls back to unread for /categories/1 (deleted per-category route)', () => {
    navigate('/categories/1');
    expect(get(route)).toEqual({ name: 'unread' });
  });
});
```

- [ ] **F2. Run test, confirm fail on `/categories`, `/feeds`, `/history`.**

```bash
pnpm --dir web test -- src/lib/__tests__/router.test.ts
```

Expected: 3 failures on new routes; the `/search` and `/categories/1` cases should also fail (still pointing at old route names).

- [ ] **F3. Rewrite `router.ts`.**

```ts
import { writable, type Readable } from 'svelte/store';

type RouteState =
  | { name: 'unread' }
  | { name: 'reader'; params: { id: number } }
  | { name: 'saved' }
  | { name: 'categories' }
  | { name: 'feeds' }
  | { name: 'history' }
  | { name: 'settings' }
  | { name: 'admin' }
  | { name: 'signin' };

function parse(pathname: string): RouteState {
  const m = pathname.match(/^\/entry\/(\d+)$/);
  if (m) return { name: 'reader', params: { id: Number(m[1]) } };
  if (pathname === '/saved')      return { name: 'saved' };
  if (pathname === '/categories') return { name: 'categories' };
  if (pathname === '/feeds')      return { name: 'feeds' };
  if (pathname === '/history')    return { name: 'history' };
  if (pathname === '/settings')   return { name: 'settings' };
  if (pathname === '/admin')      return { name: 'admin' };
  if (pathname === '/sign-in')    return { name: 'signin' };
  return { name: 'unread' };
}

const internal = writable<RouteState>(parse(window.location.pathname));
window.addEventListener('popstate', () => internal.set(parse(window.location.pathname)));

export const route: Readable<RouteState> = { subscribe: internal.subscribe };

export function navigate(to: string) {
  if (window.location.pathname === to) return;
  window.history.pushState({}, '', to);
  internal.set(parse(to));
}
```

- [ ] **F4. Run test, confirm green.**

```bash
pnpm --dir web test -- src/lib/__tests__/router.test.ts
```

Expected: 11 passing.

- [ ] **F5. Commit.**

```bash
git add web/src/lib/router.ts web/src/lib/__tests__/router.test.ts
git commit -m "M1-F: rewrite router with categories/feeds/history/sign-in; drop search/category"
```

### Group G — Pure-markup primitives (no behaviour)

These are presentational; per "TDD posture" they don't need TDD. Build them in one focused session: write the component, eyeball it against the design canvas in a Vite dev server, move on. Each one ships with its scoped `<style>` block translated from the matching selectors in `ui_design/styles.css`.

**Files:** as listed.

- [ ] **G1. Invoke `svelte-styling` skill.** Do this once before writing the first primitive.

- [ ] **G2. `Button.svelte`** (`web/src/components/Button.svelte`).

```svelte
<script lang="ts">
  type Variant = 'default' | 'primary' | 'accent' | 'danger' | 'quiet';
  type Size = 'sm' | 'md';
  type Props = {
    variant?: Variant;
    size?: Size;
    type?: 'button' | 'submit';
    disabled?: boolean;
    title?: string;
    onclick?: (e: MouseEvent) => void;
    children: import('svelte').Snippet;
  };
  let { variant = 'default', size = 'md', type = 'button', disabled = false, title, onclick, children }: Props = $props();
</script>

<button
  {type}
  {title}
  {disabled}
  class="btn variant-{variant} size-{size}"
  {onclick}
>
  {@render children()}
</button>

<style>
  .btn {
    display: inline-flex; align-items: center; gap: 7px;
    font-family: var(--sans);
    font-size: var(--fs-button, 12.5px);
    font-weight: 500;
    padding: 8px 14px;
    border-radius: var(--radius-input, 4px);
    border: 1px solid var(--rule);
    background: var(--bg);
    color: var(--ink);
    cursor: pointer;
    white-space: nowrap;
    transition: background var(--dur-fast, 100ms) ease, border-color var(--dur-fast, 100ms) ease;
  }
  .btn:hover { background: var(--bg-soft); border-color: var(--ink-4); }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn.variant-primary { background: var(--ink); color: var(--bg); border-color: var(--ink); }
  .btn.variant-primary:hover { opacity: 0.88; }
  .btn.variant-accent { color: var(--accent); border-color: var(--accent); background: transparent; }
  .btn.variant-accent:hover { background: var(--accent-soft); }
  .btn.variant-danger { color: var(--color-error-light, #c43a3a); border-color: rgba(196, 58, 58, 0.4); background: transparent; }
  :global(html.theme-dark) .btn.variant-danger { color: var(--color-error-dark, #ec7a7a); border-color: rgba(236, 122, 122, 0.4); }
  .btn.variant-danger:hover { background: rgba(196, 58, 58, 0.06); }
  .btn.variant-quiet { border-color: transparent; background: transparent; color: var(--ink-2); padding: 6px 8px; }
  .btn.variant-quiet:hover { color: var(--ink); background: var(--bg-soft); }
  .btn.size-sm { padding: 6px 10px; font-size: 11px; }
</style>
```

- [ ] **G3. `KbdChip.svelte`** (`web/src/components/KbdChip.svelte`).

```svelte
<script lang="ts">
  type Props = { children: import('svelte').Snippet };
  let { children }: Props = $props();
</script>

<kbd class="kbd">{@render children()}</kbd>

<style>
  .kbd {
    display: inline-block;
    font-family: var(--mono);
    font-size: var(--fs-kbd, 10.5px);
    padding: 1px 5px;
    border: 1px solid var(--rule);
    border-bottom-width: 2px;
    border-radius: 3px;
    background: var(--surface);
    color: var(--ink-2);
    line-height: 1.2;
    vertical-align: 1px;
  }
</style>
```

- [ ] **G4. `EmptyState.svelte`** (`web/src/components/EmptyState.svelte`).

**Contract (cross-plan, per team-lead decision 2026-05-11):** prop name is **`subtitle`** and accepts **`string | Snippet`**. Strings render as plain text inside `<p class="ts-empty-sub">`; Snippets render via `{@render subtitle()}` so callers can slot inline elements (e.g. M3's `Press <KbdChip>S</KbdChip> on any entry to save it`). M3/M5/M6/M8 plans align on this prop name and union type. `cta` is a structured `{ label, onClick }` object rather than a Snippet — covers the only call-site shape used across the redesign without a second discriminator. The `dot` prop is `'accent' | 'ink-4'` per brand spec §4.16: "no results" empty states should use `ink-4`.

```svelte
<script lang="ts">
  import type { Snippet } from 'svelte';
  type Props = {
    title: string;
    subtitle?: string | Snippet;
    cta?: { label: string; onClick: () => void };
    dot?: 'accent' | 'ink-4';
  };
  let { title, subtitle, cta, dot = 'accent' }: Props = $props();
</script>

<div class="ts-empty">
  <span class="ts-empty-dot" data-tone={dot} aria-hidden="true"></span>
  <h2 class="ts-empty-title">{title}</h2>
  {#if typeof subtitle === 'string'}
    <p class="ts-empty-sub">{subtitle}</p>
  {:else if subtitle}
    <p class="ts-empty-sub">{@render subtitle()}</p>
  {/if}
  {#if cta}
    <button type="button" class="ts-btn is-primary" onclick={cta.onClick}>{cta.label}</button>
  {/if}
</div>

<style>
  .ts-empty { padding: 80px 24px; text-align: center; color: var(--ink-3); }
  .ts-empty-dot {
    display: inline-block;
    width: 8px; height: 8px; border-radius: 50%;
    background: var(--accent);
    margin: 0 auto 18px;
  }
  .ts-empty-dot[data-tone="ink-4"] { background: var(--ink-4); }
  .ts-empty-title {
    font-family: var(--serif); font-size: 20px; font-weight: 500;
    color: var(--ink-2);
    margin: 0 0 8px;
    letter-spacing: -0.01em;
  }
  .ts-empty-sub {
    font-family: var(--sans); font-size: 13px; color: var(--ink-3);
    max-width: 360px; margin: 0 auto;
    line-height: 1.5;
  }
  .ts-empty .ts-btn { margin-top: 18px; }
</style>
```

**Test note:** add a unit test asserting both `subtitle` shapes and the `cta` callback. `web/src/components/__tests__/EmptyState.test.ts`:

```ts
import { render } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import EmptyState from '../EmptyState.svelte';
import { createRawSnippet } from 'svelte';

describe('EmptyState', () => {
  it('renders string subtitle', () => {
    const { getByText } = render(EmptyState, { title: 'Nothing yet', subtitle: 'Come back later.' });
    expect(getByText('Come back later.')).toBeTruthy();
  });

  it('renders Snippet subtitle (allows inline interactive markup)', () => {
    const subtitle = createRawSnippet(() => ({ render: () => '<span>Press <kbd>S</kbd></span>' }));
    const { getByText } = render(EmptyState, { title: 'Empty', subtitle });
    expect(getByText(/Press/)).toBeTruthy();
  });

  it('renders cta button and fires onClick', async () => {
    const onClick = vi.fn();
    const { getByRole } = render(EmptyState, {
      title: 'No feeds', subtitle: 'Add one to get started.',
      cta: { label: 'Add feed', onClick },
    });
    (getByRole('button', { name: /add feed/i }) as HTMLButtonElement).click();
    expect(onClick).toHaveBeenCalledOnce();
  });
});
```

This bumps EmptyState into the "exempt-but-has-one-test" bucket because the `subtitle` type discriminator is behavioural.

- [ ] **G5. `Chip.svelte`** (`web/src/components/Chip.svelte`). Variants: pill / tag / error / backoff.

```svelte
<script lang="ts">
  type Variant = 'pill' | 'tag' | 'error' | 'backoff';
  type Props = {
    variant?: Variant;
    active?: boolean;
    count?: number;
    onclick?: () => void;
    children: import('svelte').Snippet;
  };
  let { variant = 'pill', active = false, count, onclick, children }: Props = $props();
</script>

<button
  type="button"
  class="chip variant-{variant}"
  class:is-active={active}
  {onclick}
>
  {@render children()}
  {#if count !== undefined}<span class="ct">{count}</span>{/if}
</button>

<style>
  .chip {
    display: inline-flex; align-items: baseline; gap: 8px;
    padding: 5px 10px 5px 12px;
    border: 1px solid var(--rule);
    border-radius: var(--radius-pill, 100px);
    background: var(--bg);
    color: var(--ink-2);
    font-family: var(--sans);
    font-size: 12px; font-weight: 500;
    cursor: pointer;
    transition: all var(--dur-base, 120ms) ease;
  }
  .chip:hover { color: var(--ink); border-color: var(--ink-4); }
  .chip.is-active { background: var(--ink); color: var(--bg); border-color: var(--ink); }
  .chip .ct { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); letter-spacing: 0; }
  .chip.is-active .ct { color: var(--bg); opacity: 0.65; }
  .variant-tag { background: var(--bg-soft); border-color: transparent; padding: 3px 8px; font-size: 11.5px; }
  .variant-error { font-family: var(--mono); font-size: 9.5px; padding: 2px 8px; color: var(--color-error-light, #c43a3a); border-color: rgba(196, 58, 58, 0.4); text-transform: uppercase; letter-spacing: 0.08em; }
  :global(html.theme-dark) .variant-error { color: var(--color-error-dark, #ec7a7a); border-color: rgba(236, 122, 122, 0.4); }
  .variant-backoff { font-family: var(--mono); font-size: 9.5px; padding: 2px 8px; color: var(--ink-3); border-style: dashed; border-color: var(--ink-4); text-transform: uppercase; letter-spacing: 0.08em; }
</style>
```

- [ ] **G6. `Field.svelte`** (`web/src/components/Field.svelte`).

```svelte
<script lang="ts">
  type Props = {
    label: string;
    value: string;
    type?: 'text' | 'email' | 'password' | 'number';
    placeholder?: string;
    mono?: boolean;
    description?: string;
    trailing?: import('svelte').Snippet;
    autocomplete?: string;
    autofocus?: boolean;
    required?: boolean;
    disabled?: boolean;
    inputmode?: string;
    pattern?: string;
    maxlength?: number;
    name?: string;
    onInput?: (v: string) => void;
  };
  let {
    label, value = $bindable(''), type = 'text', placeholder, mono = false,
    description, trailing, autocomplete, autofocus = false, required = false,
    disabled = false, inputmode, pattern, maxlength, name, onInput,
  }: Props = $props();
</script>

<label class="field" class:is-mono={mono}>
  <span class="field-label">
    <span>{label}</span>
    {#if trailing}<span class="trailing">{@render trailing()}</span>{/if}
  </span>
  <input
    {type}
    {placeholder}
    {autocomplete}
    {autofocus}
    {required}
    {disabled}
    {inputmode}
    {pattern}
    {maxlength}
    {name}
    class="input"
    bind:value
    oninput={(e) => onInput?.((e.currentTarget as HTMLInputElement).value)}
  />
  {#if description}<span class="desc">{description}</span>{/if}
</label>

<style>
  .field { display: flex; flex-direction: column; gap: 8px; }
  .field-label {
    display: flex; justify-content: space-between; align-items: baseline;
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.12em; text-transform: uppercase;
    color: var(--ink-3);
  }
  .trailing { font-family: var(--mono); font-size: 10px; letter-spacing: 0.14em; }
  .input {
    font-family: var(--sans);
    font-size: 14px;
    padding: 9px 12px;
    border: 1px solid var(--rule);
    background: var(--bg);
    color: var(--ink);
    border-radius: var(--radius-input, 4px);
    width: 100%;
  }
  .field.is-mono .input { font-family: var(--mono); font-size: 13px; }
  .input:focus { outline: 2px solid var(--accent); outline-offset: 1px; border-color: var(--accent); }
  .desc { font-family: var(--sans); font-size: 11.5px; color: var(--ink-3); }
</style>
```

- [ ] **G7. `RecoveryCodesGrid.svelte`** (`web/src/components/RecoveryCodesGrid.svelte`).

```svelte
<script lang="ts">
  type Code = { code: string; used?: boolean };
  type Props = { codes: Code[] };
  let { codes }: Props = $props();
</script>

<div class="codes">
  {#each codes as c, i (c.code)}
    <span class="code" class:is-used={c.used}>
      <span class="n">{i + 1}</span>
      <span>{c.code}</span>
    </span>
  {/each}
</div>

<style>
  .codes {
    display: grid; grid-template-columns: 1fr 1fr; gap: 6px 18px;
    padding: 14px 18px;
    background: var(--bg-soft);
    border: 1px solid var(--rule);
    border-radius: var(--radius-input, 4px);
  }
  .code {
    font-family: var(--mono); font-size: 13px; color: var(--ink);
    letter-spacing: 0.06em; padding: 4px 0;
    display: flex; align-items: center; gap: 10px;
  }
  .code .n { color: var(--ink-4); font-size: 10px; width: 14px; text-align: right; }
  .code.is-used { color: var(--ink-4); text-decoration: line-through; }
</style>
```

- [ ] **G8. `GroupHeading.svelte`** (`web/src/components/GroupHeading.svelte`).

```svelte
<script lang="ts">
  type Props = { label: string; count?: number };
  let { label, count }: Props = $props();
</script>

<div class="heading">
  <span class="label">{label}</span>
  <span class="rule" aria-hidden="true"></span>
  {#if count !== undefined}<span class="count">{count}</span>{/if}
</div>

<style>
  .heading {
    display: flex; align-items: center; gap: 12px;
    padding: 28px 2px 10px;
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.14em; text-transform: uppercase;
    color: var(--ink-3);
  }
  .heading:first-child { padding-top: 18px; }
  .label { flex-shrink: 0; }
  .rule { flex: 1; height: 1px; background: var(--rule); }
  .count { font-family: var(--mono); font-size: 10px; color: var(--ink-3); letter-spacing: 0.04em; }
</style>
```

- [ ] **G9. Update `FeedAvatar.svelte`.** Read the existing file. Confirm default `size` is 14, default `radius` is 3 (per design). If different, set the prop defaults to those values.

- [ ] **G10. Commit.**

```bash
git add web/src/components/Button.svelte web/src/components/KbdChip.svelte web/src/components/EmptyState.svelte web/src/components/Chip.svelte web/src/components/Field.svelte web/src/components/RecoveryCodesGrid.svelte web/src/components/GroupHeading.svelte web/src/components/FeedAvatar.svelte
git commit -m "M1-G: pure-markup primitives (Button, KbdChip, EmptyState, Chip, Field, RecoveryCodesGrid, GroupHeading)"
```

### Group H — Behavioural primitives (TDD)

Each item: write failing test → run → minimal impl → run → commit.

**Invoke `superpowers:test-driven-development` and `svelte-template-directives` skills before starting.**

#### H1. Segmented

- [ ] **H1a. Write failing test.** `web/src/components/__tests__/Segmented.test.ts`:

```ts
import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import Segmented from '../Segmented.svelte';

describe('Segmented', () => {
  it('renders options with active marker', () => {
    const { getByRole } = render(Segmented, {
      options: [
        { value: 'light', label: 'Light' },
        { value: 'dark', label: 'Dark' },
      ],
      value: 'dark',
      onChange: () => {},
    });
    const dark = getByRole('button', { name: /dark/i });
    expect(dark.className).toMatch(/is-active/);
  });

  it('fires onChange on click', async () => {
    const onChange = vi.fn();
    const { getByRole } = render(Segmented, {
      options: [
        { value: 'light', label: 'Light' },
        { value: 'dark', label: 'Dark' },
      ],
      value: 'light',
      onChange,
    });
    await fireEvent.click(getByRole('button', { name: /dark/i }));
    expect(onChange).toHaveBeenCalledWith('dark');
    expect(onChange).toHaveBeenCalledTimes(1);
  });
});
```

- [ ] **H1b. Run, confirm fail.** `pnpm --dir web test -- src/components/__tests__/Segmented.test.ts`. Expected: cannot find module.

- [ ] **H1c. Implement `Segmented.svelte`.**

```svelte
<script lang="ts" generics="T extends string">
  type Option = { value: T; label: string; preview?: import('svelte').Snippet };
  type Props = {
    options: Option[];
    value: T;
    onChange: (v: T) => void;
    ariaLabel?: string;
  };
  let { options, value, onChange, ariaLabel }: Props = $props();
</script>

<div class="segmented" role="radiogroup" aria-label={ariaLabel}>
  {#each options as o (o.value)}
    <button
      type="button"
      role="radio"
      aria-checked={value === o.value}
      class="btn"
      class:is-active={value === o.value}
      onclick={() => onChange(o.value)}
    >
      {#if o.preview}{@render o.preview()}{/if}
      <span>{o.label}</span>
    </button>
  {/each}
</div>

<style>
  .segmented {
    display: inline-flex; align-items: stretch;
    background: var(--bg-soft);
    border: 1px solid var(--rule);
    border-radius: 999px;
    padding: 3px; gap: 2px;
  }
  .btn {
    display: inline-flex; align-items: center; gap: 7px;
    padding: 7px 14px;
    border: 0; background: transparent;
    border-radius: 999px;
    font-family: var(--sans); font-size: 12.5px; font-weight: 500;
    color: var(--ink-2); cursor: pointer;
    white-space: nowrap;
    transition: background var(--dur-fast, 100ms) ease, color var(--dur-fast, 100ms) ease;
  }
  .btn:hover { color: var(--ink); }
  .btn.is-active {
    background: var(--bg); color: var(--ink);
    box-shadow: 0 1px 2px rgba(0,0,0,0.06), 0 0 0 1px var(--rule);
  }
</style>
```

- [ ] **H1d. Run, confirm green. Commit.**

```bash
git add web/src/components/Segmented.svelte web/src/components/__tests__/Segmented.test.ts
git commit -m "M1-H1: Segmented primitive (TDD)"
```

#### H2. Popover

- [ ] **H2a. Write failing test.** `web/src/components/__tests__/Popover.test.ts`:

```ts
import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import Popover from '../Popover.svelte';
import { createRawSnippet } from 'svelte';

const noop = createRawSnippet(() => ({ render: () => '<div>hello</div>' }));

describe('Popover', () => {
  it('renders children when open', () => {
    const { getByText } = render(Popover, { open: true, onClose: () => {}, children: noop });
    expect(getByText('hello')).toBeTruthy();
  });

  it('does not render when closed', () => {
    const { queryByText } = render(Popover, { open: false, onClose: () => {}, children: noop });
    expect(queryByText('hello')).toBeNull();
  });

  it('calls onClose on scrim click', async () => {
    const onClose = vi.fn();
    const { container } = render(Popover, { open: true, onClose, children: noop });
    const scrim = container.querySelector('.scrim') as HTMLElement;
    await fireEvent.click(scrim);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('calls onClose on Escape', async () => {
    const onClose = vi.fn();
    render(Popover, { open: true, onClose, children: noop });
    await fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledOnce();
  });
});
```

- [ ] **H2b. Run, confirm fail.**

- [ ] **H2c. Implement `Popover.svelte`.**

```svelte
<script lang="ts">
  type Props = {
    open: boolean;
    onClose: () => void;
    anchor?: 'top-right' | 'bottom-left' | 'bottom-right' | 'free';
    children: import('svelte').Snippet;
  };
  let { open, onClose, anchor = 'free', children }: Props = $props();

  $effect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  });
</script>

{#if open}
  <div class="scrim" onclick={onClose} role="presentation"></div>
  <div class="popover anchor-{anchor}" role="menu" onclick={(e) => e.stopPropagation()}>
    {@render children()}
  </div>
{/if}

<style>
  .scrim { position: fixed; inset: 0; z-index: 150; }
  .popover {
    position: absolute; z-index: 151;
    background: var(--bg);
    border: 1px solid var(--rule);
    border-radius: 6px;
    box-shadow: var(--shadow-popover, 0 10px 30px rgba(0,0,0,0.18));
    padding: 4px;
    font-family: var(--sans);
    min-width: 200px;
  }
  .anchor-top-right { right: 18px; top: 56px; }
  .anchor-bottom-left { left: 14px; bottom: 56px; }
  .anchor-bottom-right { right: 18px; bottom: 56px; }
</style>
```

- [ ] **H2d. Run, confirm green. Commit.**

```bash
git add web/src/components/Popover.svelte web/src/components/__tests__/Popover.test.ts
git commit -m "M1-H2: Popover primitive with click-outside + Esc (TDD)"
```

#### H3. Dialog

- [ ] **H3a. Write failing test.** `web/src/components/__tests__/Dialog.test.ts`:

```ts
import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import Dialog from '../Dialog.svelte';
import { createRawSnippet } from 'svelte';

const body = createRawSnippet(() => ({ render: () => '<p>body</p>' }));

describe('Dialog', () => {
  it('renders title and body when open', () => {
    const { getByText } = render(Dialog, {
      open: true, onClose: () => {}, title: 'Confirm', children: body,
    });
    expect(getByText('Confirm')).toBeTruthy();
    expect(getByText('body')).toBeTruthy();
  });

  it('closes on Escape', async () => {
    const onClose = vi.fn();
    render(Dialog, { open: true, onClose, title: 'X', children: body });
    await fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('closes on scrim click', async () => {
    const onClose = vi.fn();
    const { container } = render(Dialog, { open: true, onClose, title: 'X', children: body });
    await fireEvent.click(container.querySelector('.scrim') as HTMLElement);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('closes on close button', async () => {
    const onClose = vi.fn();
    const { getByLabelText } = render(Dialog, { open: true, onClose, title: 'X', children: body });
    await fireEvent.click(getByLabelText('Close'));
    expect(onClose).toHaveBeenCalledOnce();
  });
});
```

- [ ] **H3b. Run, confirm fail.**

- [ ] **H3c. Implement `Dialog.svelte`.**

```svelte
<script lang="ts">
  type Props = {
    open: boolean;
    onClose: () => void;
    title: string;
    wide?: boolean;
    foot?: import('svelte').Snippet;
    children: import('svelte').Snippet;
  };
  let { open, onClose, title, wide = false, foot, children }: Props = $props();

  $effect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose(); };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  });
</script>

{#if open}
  <div class="scrim" onclick={onClose} role="presentation"></div>
  <div
    class="dialog"
    class:is-wide={wide}
    role="dialog"
    aria-modal="true"
    aria-label={title}
    onclick={(e) => e.stopPropagation()}
  >
    <div class="head">
      <span class="title">{title}</span>
      <button class="close" onclick={onClose} aria-label="Close" type="button">
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"><path d="M3.5 3.5l9 9M12.5 3.5l-9 9"/></svg>
      </button>
    </div>
    <div class="body">{@render children()}</div>
    {#if foot}<div class="foot">{@render foot()}</div>{/if}
  </div>
{/if}

<style>
  .scrim {
    position: fixed; inset: 0; z-index: 200;
    background: rgba(0,0,0,0.42);
    backdrop-filter: blur(2px);
  }
  :global(html.theme-dark) .scrim { background: rgba(0,0,0,0.6); }
  .dialog {
    position: fixed;
    left: 50%; top: 64px;
    transform: translateX(-50%);
    width: min(520px, calc(100% - 32px));
    max-height: calc(100vh - 96px);
    overflow: auto;
    z-index: 201;
    background: var(--bg);
    border: 1px solid var(--rule);
    border-radius: var(--radius-dialog, 6px);
    box-shadow: var(--shadow-dialog, 0 24px 60px rgba(0,0,0,0.32));
  }
  .dialog.is-wide { width: min(640px, 92vw); }
  .head {
    display: flex; align-items: center; justify-content: space-between;
    padding: 16px 20px 14px;
    border-bottom: 1px solid var(--rule);
  }
  .title { font-family: var(--serif); font-size: 18px; font-weight: 600; color: var(--ink); letter-spacing: -0.01em; }
  .close {
    width: 28px; height: 28px;
    display: inline-flex; align-items: center; justify-content: center;
    border: 0; background: transparent;
    color: var(--ink-3); cursor: pointer; border-radius: 4px;
  }
  .close:hover { color: var(--ink); background: var(--bg-soft); }
  .body { padding: 18px 20px; }
  .foot {
    display: flex; justify-content: flex-end; gap: 8px;
    padding: 14px 20px;
    border-top: 1px solid var(--rule);
    background: var(--bg-soft);
  }
</style>
```

- [ ] **H3d. Run, confirm green. Commit.**

```bash
git add web/src/components/Dialog.svelte web/src/components/__tests__/Dialog.test.ts
git commit -m "M1-H3: Dialog primitive with Esc/scrim/close (TDD)"
```

#### H4. OtpInput

- [ ] **H4a. Write failing test.** `web/src/components/__tests__/OtpInput.test.ts`:

```ts
import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import OtpInput from '../OtpInput.svelte';

describe('OtpInput', () => {
  it('renders six cells', () => {
    const { container } = render(OtpInput, { value: '', onChange: () => {} });
    expect(container.querySelectorAll('input').length).toBe(6);
  });

  it('fires onChange with full code on paste', async () => {
    const onChange = vi.fn();
    const { container } = render(OtpInput, { value: '', onChange });
    const first = container.querySelector('input') as HTMLInputElement;
    const evt = new ClipboardEvent('paste', {
      clipboardData: new DataTransfer(),
    });
    // jsdom's DataTransfer does not support setData via constructor; mock manually:
    Object.defineProperty(evt, 'clipboardData', {
      value: { getData: () => '123456' },
    });
    first.dispatchEvent(evt);
    expect(onChange).toHaveBeenCalledWith('123456');
  });

  it('advances focus after typing a digit', async () => {
    const { container } = render(OtpInput, { value: '', onChange: () => {} });
    const inputs = container.querySelectorAll('input');
    (inputs[0] as HTMLInputElement).focus();
    await fireEvent.input(inputs[0], { target: { value: '7' } });
    expect(document.activeElement).toBe(inputs[1]);
  });

  it('moves focus left on Backspace in empty cell', async () => {
    const { container } = render(OtpInput, { value: '1', onChange: () => {} });
    const inputs = container.querySelectorAll('input');
    (inputs[1] as HTMLInputElement).focus();
    await fireEvent.keyDown(inputs[1], { key: 'Backspace' });
    expect(document.activeElement).toBe(inputs[0]);
  });
});
```

- [ ] **H4b. Run, confirm fail.**

- [ ] **H4c. Implement `OtpInput.svelte`.**

```svelte
<script lang="ts">
  type Props = {
    value: string;
    onChange: (v: string) => void;
    disabled?: boolean;
  };
  let { value, onChange, disabled = false }: Props = $props();

  let refs: (HTMLInputElement | null)[] = $state(Array.from({ length: 6 }, () => null));

  function setCell(i: number, ch: string) {
    const digits = value.padEnd(6, ' ').split('');
    digits[i] = ch || ' ';
    const next = digits.join('').replace(/ /g, '');
    onChange(next.slice(0, 6));
  }

  function onInput(i: number, e: Event) {
    const v = (e.currentTarget as HTMLInputElement).value;
    const ch = v.replace(/\D/g, '').slice(-1);
    setCell(i, ch);
    if (ch && i < 5) refs[i + 1]?.focus();
  }

  function onKey(i: number, e: KeyboardEvent) {
    if (e.key === 'Backspace' && !(e.currentTarget as HTMLInputElement).value && i > 0) {
      refs[i - 1]?.focus();
    } else if (e.key === 'ArrowLeft' && i > 0) refs[i - 1]?.focus();
    else if (e.key === 'ArrowRight' && i < 5) refs[i + 1]?.focus();
  }

  function onPaste(e: ClipboardEvent) {
    e.preventDefault();
    const text = (e.clipboardData?.getData('text') ?? '').replace(/\D/g, '').slice(0, 6);
    if (!text) return;
    onChange(text);
    refs[Math.min(text.length, 5)]?.focus();
  }
</script>

<div class="otp" role="group" aria-label="6-digit code">
  {#each Array(6) as _, i}
    <input
      bind:this={refs[i]}
      class="cell"
      inputmode="numeric"
      pattern="[0-9]*"
      maxlength="1"
      value={value[i] ?? ''}
      {disabled}
      oninput={(e) => onInput(i, e)}
      onkeydown={(e) => onKey(i, e)}
      onpaste={(e) => onPaste(e)}
    />
  {/each}
</div>

<style>
  .otp { display: inline-flex; gap: 6px; }
  .cell {
    width: 36px; height: 44px;
    border: 1px solid var(--rule);
    background: var(--bg);
    border-radius: 4px;
    text-align: center;
    font-family: var(--mono);
    font-size: 17px;
    color: var(--ink);
    font-weight: 500;
    font-variant-numeric: tabular-nums;
  }
  .cell:focus {
    outline: none;
    border-color: var(--accent);
    box-shadow: 0 0 0 2px var(--accent-soft);
  }
</style>
```

- [ ] **H4d. Run, confirm green. Commit.**

```bash
git add web/src/components/OtpInput.svelte web/src/components/__tests__/OtpInput.test.ts
git commit -m "M1-H4: OtpInput six-cell with paste/arrows/Backspace (TDD)"
```

#### H5. EntryRow (rewritten)

The existing `EntryRow.svelte` has tests baked into the views; here we add a dedicated unit test.

- [ ] **H5a. Write failing test.** `web/src/components/__tests__/EntryRow.test.ts`:

```ts
import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import EntryRow from '../EntryRow.svelte';
import type { EntryListItem, Subscription } from '../../lib/types';

const baseEntry: EntryListItem = {
  id: 1, subscription_id: 1, title: 'Hello', author: 'Ada',
  url: 'https://x', published_at: Math.floor(Date.now() / 1000) - 600,
  fetched_at: 0, read: false, saved: false, extract_failed: false,
};
const baseFeed: Subscription = {
  id: 1, title: 'Site', feed_url: 'https://x/feed', next_poll_at: 0,
  error_count: 0, created_at: 0, extract: false, extract_selector: '',
  has_cookie: false, has_basic_auth: false, category_id: null,
};

describe('EntryRow', () => {
  it('renders unread by default', () => {
    const { container } = render(EntryRow, { entry: baseEntry, feed: baseFeed });
    expect(container.querySelector('.entry')?.className).not.toMatch(/is-read/);
  });

  it('applies is-read when entry.read', () => {
    const { container } = render(EntryRow, { entry: { ...baseEntry, read: true }, feed: baseFeed });
    expect(container.querySelector('.entry')?.className).toMatch(/is-read/);
  });

  it('applies is-saved and shows mark when entry.saved', () => {
    const { container, getByText } = render(EntryRow, {
      entry: { ...baseEntry, saved: true }, feed: baseFeed,
    });
    expect(container.querySelector('.entry')?.className).toMatch(/is-saved/);
    expect(getByText(/saved/i)).toBeTruthy();
  });

  it('navigates on click', async () => {
    const { container } = render(EntryRow, { entry: baseEntry, feed: baseFeed });
    const pushState = vi.spyOn(window.history, 'pushState');
    await fireEvent.click(container.querySelector('.entry') as HTMLElement);
    expect(pushState).toHaveBeenCalled();
    pushState.mockRestore();
  });

  it('applies density class', () => {
    const { container } = render(EntryRow, { entry: baseEntry, feed: baseFeed, density: 'compact' });
    expect(container.querySelector('.entry')?.className).toMatch(/density-compact/);
  });
});
```

- [ ] **H5b. Run, confirm at least the density and is-saved cases fail.**

- [ ] **H5c. Rewrite `EntryRow.svelte`.** Replace the current contents with:

```svelte
<script lang="ts">
  import FeedAvatar from './FeedAvatar.svelte';
  import type { EntryListItem, Subscription } from '../lib/types';
  import { navigate } from '../lib/router';
  import { swipe } from '../lib/swipe';

  type Density = 'compact' | 'comfortable' | 'cosy';
  type Props = {
    entry: EntryListItem;
    feed: Subscription | undefined;
    isSelected?: boolean;
    density?: Density;
    onToggleRead?: () => void;
    onToggleSaved?: () => void;
  };
  let {
    entry, feed, isSelected = false, density = 'comfortable',
    onToggleRead, onToggleSaved,
  }: Props = $props();

  function ago(ts: number): string {
    const sec = Math.max(1, Math.floor(Date.now() / 1000) - ts);
    if (sec < 60) return `${sec}s`;
    if (sec < 3600) return `${Math.floor(sec / 60)}m`;
    if (sec < 86400) return `${Math.floor(sec / 3600)}h`;
    return `${Math.floor(sec / 86400)}d`;
  }
</script>

<button
  class="entry density-{density}"
  class:is-read={entry.read}
  class:is-selected={isSelected}
  class:is-saved={entry.saved}
  aria-label="{entry.title}{entry.read ? ' (read)' : ''}"
  onclick={() => navigate(`/entry/${entry.id}`)}
  {@attach swipe({ onSwipeRight: onToggleRead, onSwipeLeft: onToggleSaved })}
>
  <span class="junction" aria-hidden="true"></span>
  {#if entry.saved}<span class="saved-mark" aria-label="Saved">SAVED</span>{/if}
  <h3 class="title">{entry.title}</h3>
  <div class="meta">
    {#if feed}
      <FeedAvatar feedURL={feed.feed_url} size={10} radius={2} />
      <span class="source">{feed.title}</span>
      <span class="sep" aria-hidden="true"></span>
    {/if}
    <span class="ago">{ago(entry.published_at)} ago</span>
  </div>
  {#if entry.author && density !== 'compact'}
    <p class="summary">{entry.author}</p>
  {/if}
</button>

<style>
  .entry {
    position: relative;
    padding: 16px 24px 16px 40px;
    border-bottom: 1px solid var(--rule);
    cursor: pointer;
    transition: background var(--dur-fast, 100ms) ease;
    width: 100%;
    text-align: left;
    background: transparent;
    border-left: 0; border-right: 0; border-top: 0;
    display: block;
  }
  .entry:hover { background: var(--bg-soft); }
  .entry.is-selected { background: var(--accent-soft); }
  .entry.is-read .title { color: var(--ink-3); font-weight: 400; }
  .entry.is-read .meta { color: var(--ink-3); }
  /* Brand spec §4.4: comfortable is the default (16px pad, summary 2 lines). */
  .entry.density-compact { padding-top: 10px; padding-bottom: 10px; }
  .entry.density-compact .summary { display: none; }
  .entry.density-compact .junction { top: 17px; }
  .entry.density-cosy { padding-top: 10px; padding-bottom: 10px; }
  .entry.density-cosy .junction { top: 17px; }
  .entry.density-cosy .summary { -webkit-line-clamp: 1; }

  .junction {
    position: absolute;
    left: 22px; top: 22px;
    width: 6px; height: 6px;
    border-radius: 50%;
    background: var(--accent);
    transition: transform 200ms ease, background 200ms ease;
  }
  .entry.is-read .junction { background: transparent; border: 1px solid var(--ink-4); }

  .saved-mark {
    position: absolute;
    right: 22px; top: 18px;
    color: var(--accent);
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.04em;
  }

  .title {
    font-family: var(--serif);
    font-size: var(--fs-entry-title, 19px);
    line-height: var(--lh-entry-title, 1.3);
    font-weight: 500;
    color: var(--ink);
    margin: 0 0 5px;
    text-wrap: pretty;
    letter-spacing: var(--tr-entry-title, -0.005em);
  }

  .meta {
    font-family: var(--sans);
    font-size: 12px;
    color: var(--ink-2);
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 4px;
    flex-wrap: wrap;
  }
  .meta .source { color: var(--ink); font-weight: 500; }
  .meta .sep {
    display: inline-block;
    width: 3px; height: 3px;
    background: var(--ink-4);
    border-radius: 50%;
  }
  .meta .ago { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); }

  .summary {
    font-family: var(--serif);
    font-size: var(--fs-entry-summary, 14.5px);
    line-height: var(--lh-entry-summary, 1.55);
    color: var(--ink-2);
    margin: 6px 0 0;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    text-wrap: pretty;
  }

  :global(.is-mobile) .entry { padding-left: 36px; padding-right: 18px; }
  :global(.is-mobile) .entry .junction { left: 18px; top: 22px; }
  :global(.is-mobile) .entry .title { font-size: var(--fs-entry-title-mobile, 16px); }
</style>
```

- [ ] **H5d. Run, confirm green. Commit.**

```bash
git add web/src/components/EntryRow.svelte web/src/components/__tests__/EntryRow.test.ts
git commit -m "M1-H5: rewrite EntryRow with density variants + tests"
```

### Group I — Shell components

These wire chrome to stores; they're presentational composers that mostly delegate behaviour to `route`, `auth`, `pollStatus`, `searchOverlay`. Tests cover only the cases listed in "TDD posture".

**Invoke `svelte-runes` skill before starting.**

#### I1. TopTabs

- [ ] **I1a. Write the failing test.** `web/src/components/__tests__/TopTabs.test.ts`:

```ts
import { render } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import TopTabs from '../TopTabs.svelte';
import { navigate } from '../../lib/router';

describe('TopTabs', () => {
  it('renders all visible tabs', () => {
    const { getByText } = render(TopTabs, { unreadCount: 0, isAdmin: false });
    ['Unread', 'Saved', 'History', 'Categories', 'Feeds', 'Settings'].forEach(label => {
      expect(getByText(label)).toBeTruthy();
    });
  });

  it('shows Admin tab when isAdmin', () => {
    const { getByText } = render(TopTabs, { unreadCount: 0, isAdmin: true });
    expect(getByText('Admin')).toBeTruthy();
  });

  it('renders unread count beside Unread tab', () => {
    const { getByText } = render(TopTabs, { unreadCount: 12, isAdmin: false });
    expect(getByText('12')).toBeTruthy();
  });

  it('marks the active tab', () => {
    navigate('/saved');
    const { getByText } = render(TopTabs, { unreadCount: 0, isAdmin: false });
    const tab = getByText('Saved').closest('button');
    expect(tab?.className).toMatch(/is-active/);
  });
});
```

- [ ] **I1b. Run, confirm fail.**

- [ ] **I1c. Implement `TopTabs.svelte`.**

```svelte
<script lang="ts">
  import { route, navigate } from '../lib/router';
  type Props = { unreadCount: number; isAdmin: boolean };
  let { unreadCount, isAdmin }: Props = $props();

  const tabs = $derived([
    { name: 'unread',     label: 'Unread',     path: '/' },
    { name: 'saved',      label: 'Saved',      path: '/saved' },
    { name: 'history',    label: 'History',    path: '/history' },
    { name: 'categories', label: 'Categories', path: '/categories' },
    { name: 'feeds',      label: 'Feeds',      path: '/feeds' },
    { name: 'settings',   label: 'Settings',   path: '/settings' },
    ...(isAdmin ? [{ name: 'admin', label: 'Admin', path: '/admin' }] : []),
  ]);

  const activeName = $derived($route.name === 'reader' ? 'unread' : $route.name);
</script>

<nav class="nav" aria-label="Primary">
  <a class="wordmark" href="/" onclick={(e) => { e.preventDefault(); navigate('/'); }}>
    tap<span class="dot" aria-hidden="true"></span>
  </a>
  <ul class="tabs">
    {#each tabs as t (t.name)}
      <li>
        <button
          type="button"
          class="tab"
          class:is-active={activeName === t.name}
          onclick={() => navigate(t.path)}
        >
          <span>{t.label}</span>
          {#if t.name === 'unread' && unreadCount > 0}
            <span class="count">{unreadCount}</span>
          {/if}
        </button>
      </li>
    {/each}
  </ul>
</nav>

<style>
  .nav {
    display: flex; align-items: baseline; gap: 32px;
    padding: 0 2px 16px;
    border-bottom: 1px solid var(--rule);
    position: sticky; top: 0;
    background: var(--bg);
    z-index: 4;
  }
  .wordmark {
    font-family: var(--sans);
    font-weight: 600; font-size: 18px;
    letter-spacing: var(--tr-display, -0.02em);
    color: var(--ink);
    text-decoration: none;
    display: inline-flex; align-items: baseline; gap: 1px;
    padding: 6px 0;
    flex-shrink: 0;
  }
  .dot {
    display: inline-block;
    width: 5px; height: 5px; border-radius: 50%;
    background: var(--accent);
    transform: translateY(-1px);
    margin-left: 1px;
  }
  .tabs { list-style: none; margin: 0; padding: 0; display: flex; align-items: baseline; flex: 1; flex-wrap: wrap; }
  .tabs > li { display: inline-flex; align-items: baseline; }
  .tabs > li + li::before {
    content: "/";
    font-family: var(--mono);
    font-size: 12px;
    color: var(--ink-4);
    padding: 0 6px;
    user-select: none;
  }
  .tab {
    position: relative;
    display: inline-flex; align-items: baseline; gap: 8px;
    padding: 8px 12px;
    font-family: var(--mono);
    font-size: var(--fs-tab-nav, 11px);
    font-weight: 400;
    letter-spacing: var(--tr-tab-nav, 0.10em);
    text-transform: uppercase;
    color: var(--ink-3);
    background: transparent;
    border: 0; cursor: pointer;
    transition: color var(--dur-fast, 100ms) ease;
  }
  .tab:hover { color: var(--ink-2); }
  .tab.is-active { color: var(--ink); font-weight: 500; }
  .tab.is-active::after {
    content: "";
    position: absolute;
    left: 50%; transform: translateX(-50%);
    bottom: -18px;
    width: 20px; height: 2px;
    background: var(--ink);
  }
  .count {
    font-family: var(--mono);
    font-size: 10.5px;
    color: var(--ink-4);
    letter-spacing: 0;
    font-weight: 400;
  }
  .tab.is-active .count { color: var(--accent); }
</style>
```

- [ ] **I1d. Run, confirm green. Commit.**

```bash
git add web/src/components/TopTabs.svelte web/src/components/__tests__/TopTabs.test.ts
git commit -m "M1-I1: TopTabs with route-aware active state"
```

#### I2. AccountAvatar + AccountMenu

- [ ] **I2a. Write the failing AccountMenu test.** `web/src/components/__tests__/AccountMenu.test.ts`:

```ts
import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import AccountMenu from '../AccountMenu.svelte';

describe('AccountMenu', () => {
  const user = { id: 1, username: 'ada', role: 'user' as const, has_totp: false, passkey_count: 0 };

  it('shows when open', () => {
    const { getByText } = render(AccountMenu, { open: true, user, onClose: () => {}, onLogout: () => {} });
    expect(getByText('ada')).toBeTruthy();
    expect(getByText('Log out')).toBeTruthy();
  });

  it('hidden when closed', () => {
    const { queryByText } = render(AccountMenu, { open: false, user, onClose: () => {}, onLogout: () => {} });
    expect(queryByText('Log out')).toBeNull();
  });

  it('fires onLogout', async () => {
    const onLogout = vi.fn();
    const { getByText } = render(AccountMenu, { open: true, user, onClose: () => {}, onLogout });
    await fireEvent.click(getByText('Log out'));
    expect(onLogout).toHaveBeenCalledOnce();
  });
});
```

- [ ] **I2b. Run, confirm fail.**

- [ ] **I2c. Implement `AccountMenu.svelte`.**

```svelte
<script lang="ts">
  import Popover from './Popover.svelte';
  import type { User } from '../lib/types';
  import { navigate } from '../lib/router';
  type Props = { open: boolean; user: User; onClose: () => void; onLogout: () => void };
  let { open, user, onClose, onLogout }: Props = $props();

  function go(path: string) { onClose(); navigate(path); }
</script>

<Popover {open} {onClose} anchor="top-right">
  <div class="head">
    <div class="name">{user.username}</div>
    <div class="email">user #{user.id}</div>
  </div>
  <button class="item" type="button" onclick={() => go('/settings')}>
    <span class="ico" aria-hidden="true">
      <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><circle cx="8" cy="6" r="2.5"/><path d="M3.5 13.5a4.5 4.5 0 0 1 9 0"/></svg>
    </span>
    <span>Account settings</span>
  </button>
  <button class="item" type="button" onclick={() => go('/settings')}>
    <span class="ico" aria-hidden="true"></span>
    <span>Switch theme</span>
  </button>
  <div class="sep" aria-hidden="true"></div>
  <button class="item logout" type="button" onclick={() => { onClose(); onLogout(); }}>
    <span class="ico" aria-hidden="true">
      <svg width="13" height="13" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><path d="M9.5 3.5H4.5a1 1 0 0 0-1 1v7a1 1 0 0 0 1 1h5"/><path d="m11 5.5 2.5 2.5L11 10.5"/><path d="M7 8h6.5"/></svg>
    </span>
    <span>Log out</span>
  </button>
</Popover>

<style>
  .head { padding: 10px 12px 12px; border-bottom: 1px solid var(--rule); margin-bottom: 4px; }
  .name { font-family: var(--sans); font-size: 13px; font-weight: 600; color: var(--ink); line-height: 1.2; }
  .email { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); margin-top: 3px; line-height: 1.2; }
  .item {
    display: flex; align-items: center; gap: 10px;
    width: 100%; padding: 6px 10px;
    background: transparent; border: 0;
    color: var(--ink-2);
    font-family: var(--mono); font-size: 11.5px;
    letter-spacing: 0.01em; cursor: pointer;
    border-radius: 4px;
    text-align: left;
  }
  .item:hover { background: rgba(0,0,0,0.06); color: var(--ink); }
  .ico { width: 16px; display: inline-flex; align-items: center; justify-content: center; color: var(--ink-3); }
  .sep { height: 1px; background: var(--rule); margin: 4px 0; }
  .logout { color: #b3402c; }
  .logout:hover { background: rgba(179, 64, 44, 0.08); color: #b3402c; }
  :global(html.theme-dark) .logout { color: #e9846f; }
  :global(html.theme-dark) .logout:hover { background: rgba(233, 132, 111, 0.10); color: #e9846f; }
</style>
```

- [ ] **I2d. Implement `AccountAvatar.svelte`** (pure markup; no TDD).

```svelte
<script lang="ts">
  import AccountMenu from './AccountMenu.svelte';
  import { auth } from '../lib/auth';

  let open = $state(false);
  const initial = $derived(($auth.user?.username ?? '?')[0].toUpperCase());

  async function logout() {
    await auth.logout();
    window.location.assign('/');
  }
</script>

{#if $auth.user}
  <button
    type="button"
    class="avatar-btn"
    class:is-open={open}
    aria-haspopup="menu"
    aria-expanded={open}
    aria-label="Account: {$auth.user.username}"
    title={$auth.user.username}
    onclick={() => open = true}
  >
    <span class="avatar" aria-hidden="true">{initial}</span>
  </button>
  <AccountMenu open={open} user={$auth.user} onClose={() => open = false} onLogout={logout} />
{/if}

<style>
  .avatar-btn {
    position: absolute;
    top: 30px; right: 22px;
    z-index: 6;
    display: inline-flex; align-items: center; justify-content: center;
    background: transparent; border: 0;
    padding: 3px;
    border-radius: 999px;
    cursor: pointer;
    transition: background var(--dur-base, 120ms) ease;
  }
  .avatar-btn:hover { background: rgba(0,0,0,0.06); }
  :global(html.theme-dark) .avatar-btn:hover { background: rgba(255,255,255,0.07); }
  .avatar-btn.is-open { background: rgba(0,0,0,0.08); }
  :global(html.theme-dark) .avatar-btn.is-open { background: rgba(255,255,255,0.10); }
  .avatar {
    width: 20px; height: 20px;
    border-radius: 999px;
    background: var(--ink-4);
    color: var(--ink);
    display: inline-flex; align-items: center; justify-content: center;
    font-family: var(--mono);
    font-size: 9.5px; font-weight: 600;
    line-height: 1;
  }
  :global(html.theme-dark) .avatar { background: rgba(255,255,255,0.14); color: var(--ink); }
</style>
```

- [ ] **I2e. Run AccountMenu tests, confirm green. Commit.**

```bash
git add web/src/components/AccountMenu.svelte web/src/components/AccountAvatar.svelte web/src/components/__tests__/AccountMenu.test.ts
git commit -m "M1-I2: AccountAvatar + AccountMenu with logout (TDD on menu)"
```

#### I3. StatusFoot

- [ ] **I3a. Implement `StatusFoot.svelte`** (no TDD — `pollStatus` store tests cover behaviour).

```svelte
<script lang="ts">
  import { pollStatus, startPollStatus, stopPollStatus } from '../lib/pollStatus';
  import { entries } from '../lib/store';
  import { onMount, onDestroy } from 'svelte';

  onMount(() => startPollStatus());
  onDestroy(() => stopPollStatus());

  const unread = $derived($entries.items.filter(e => !e.read).length);
</script>

<footer class="foot" role="status" aria-live="polite">
  <span class="dot" aria-hidden="true"></span>
  <span>{unread === 0 ? 'all caught up' : `${unread} unread`}</span>
  <span class="sep" aria-hidden="true">·</span>
  {#if $pollStatus.active === null}
    <span>polling…</span>
  {:else if $pollStatus.active === 0}
    <span>idle</span>
  {:else}
    <span>{$pollStatus.active} polling</span>
  {/if}
  <span class="sep" aria-hidden="true">·</span>
  <span>press <kbd>?</kbd> for shortcuts</span>
</footer>

<style>
  .foot {
    display: flex; align-items: center; gap: 8px;
    padding: 24px 2px 0;
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.06em;
    color: var(--ink-3);
  }
  .dot {
    width: 5px; height: 5px;
    border-radius: 50%;
    background: var(--accent);
    animation: tap-pulse 2.4s ease-in-out infinite;
    flex-shrink: 0;
  }
  .sep { color: var(--ink-4); }
  kbd {
    font-family: var(--mono);
    font-size: 10px;
    padding: 1px 5px;
    border: 1px solid var(--rule);
    border-bottom-width: 2px;
    border-radius: 3px;
    background: var(--surface);
    color: var(--ink-2);
  }
</style>
```

- [ ] **I3b. Commit.**

```bash
git add web/src/components/StatusFoot.svelte
git commit -m "M1-I3: StatusFoot consuming pollStatus + entries stores"
```

#### I4. MobileTopBar + MobileTabBar + MobileMoreSheet

- [ ] **I4a. Write failing MobileTabBar test.** `web/src/components/__tests__/MobileTabBar.test.ts`:

```ts
import { render } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import MobileTabBar from '../MobileTabBar.svelte';
import { navigate } from '../../lib/router';

describe('MobileTabBar', () => {
  it('renders five primary tabs', () => {
    const { getByLabelText } = render(MobileTabBar, { unreadCount: 0 });
    ['Unread', 'Saved', 'Feeds', 'Categories', 'More'].forEach(t =>
      expect(getByLabelText(t)).toBeTruthy(),
    );
  });

  it('marks More active when route is history', () => {
    navigate('/history');
    const { getByLabelText } = render(MobileTabBar, { unreadCount: 0 });
    expect(getByLabelText('More').className).toMatch(/is-active/);
  });

  it('marks More active when route is settings', () => {
    navigate('/settings');
    const { getByLabelText } = render(MobileTabBar, { unreadCount: 0 });
    expect(getByLabelText('More').className).toMatch(/is-active/);
  });

  it('renders unread badge when count > 0', () => {
    navigate('/');
    const { getByText } = render(MobileTabBar, { unreadCount: 7 });
    expect(getByText('7')).toBeTruthy();
  });
});
```

- [ ] **I4b. Run, confirm fail.**

- [ ] **I4c. Implement `MobileTabBar.svelte`.**

```svelte
<script lang="ts">
  import MobileMoreSheet from './MobileMoreSheet.svelte';
  import { route, navigate } from '../lib/router';
  type Props = { unreadCount: number };
  let { unreadCount }: Props = $props();

  let moreOpen = $state(false);
  const activeName = $derived(
    $route.name === 'history' || $route.name === 'settings' || $route.name === 'admin' ? 'more' :
    $route.name === 'reader' ? 'unread' : $route.name,
  );

  const tabs = [
    { name: 'unread',     label: 'Unread',     path: '/' },
    { name: 'saved',      label: 'Saved',      path: '/saved' },
    { name: 'feeds',      label: 'Feeds',      path: '/feeds' },
    { name: 'categories', label: 'Categories', path: '/categories' },
    { name: 'more',       label: 'More',       path: null },
  ];

  function go(t: typeof tabs[number]) {
    if (t.name === 'more') { moreOpen = true; return; }
    moreOpen = false;
    if (t.path) navigate(t.path);
  }
</script>

<nav class="tmnav" aria-label="Primary">
  {#each tabs as t (t.name)}
    <button
      type="button"
      aria-label={t.label}
      aria-current={activeName === t.name ? 'page' : undefined}
      class="tmnav-tab"
      class:is-active={activeName === t.name}
      onclick={() => go(t)}
    >
      <span class="lbl">{t.label}</span>
      {#if t.name === 'unread' && unreadCount > 0}
        <span class="badge" aria-hidden="true">{unreadCount > 99 ? '99+' : unreadCount}</span>
      {/if}
    </button>
  {/each}
</nav>

<MobileMoreSheet open={moreOpen} onClose={() => moreOpen = false} />

<style>
  .tmnav {
    display: grid; grid-template-columns: repeat(5, 1fr);
    border-top: 1px solid var(--rule);
    background: var(--bg);
    padding: 8px 0 calc(env(safe-area-inset-bottom, 0px) + 12px);
    position: fixed; bottom: 0; left: 0; right: 0;
    z-index: 10;
  }
  .tmnav-tab {
    display: flex; flex-direction: column; align-items: center; gap: 4px;
    padding: 6px 0 4px;
    font-family: var(--sans); font-size: 11px; font-weight: 500;
    color: var(--ink-3);
    min-height: 44px;
    position: relative;
    background: transparent; border: 0; cursor: pointer;
  }
  .tmnav-tab.is-active { color: var(--ink); }
  .tmnav-tab.is-active::before {
    content: "";
    position: absolute;
    top: 0; left: 50%; transform: translateX(-50%);
    width: 4px; height: 4px; border-radius: 50%;
    background: var(--accent);
  }
  .badge {
    font-family: var(--mono);
    font-size: 9.5px;
    color: var(--ink-3);
  }
</style>
```

- [ ] **I4d. Write failing MobileMoreSheet test.** `web/src/components/__tests__/MobileMoreSheet.test.ts`:

```ts
import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import MobileMoreSheet from '../MobileMoreSheet.svelte';

describe('MobileMoreSheet', () => {
  it('hidden when closed', () => {
    const { queryByText } = render(MobileMoreSheet, { open: false, onClose: () => {} });
    expect(queryByText('History')).toBeNull();
  });

  it('renders History and Settings items when open', () => {
    const { getByText } = render(MobileMoreSheet, { open: true, onClose: () => {} });
    expect(getByText('History')).toBeTruthy();
    expect(getByText('Settings')).toBeTruthy();
  });

  it('closes on backdrop click', async () => {
    const onClose = vi.fn();
    const { container } = render(MobileMoreSheet, { open: true, onClose });
    await fireEvent.click(container.querySelector('.tmnav-sheet-backdrop') as HTMLElement);
    expect(onClose).toHaveBeenCalledOnce();
  });
});
```

- [ ] **I4e. Run, confirm fail.**

- [ ] **I4f. Implement `MobileMoreSheet.svelte`.**

```svelte
<script lang="ts">
  import { auth } from '../lib/auth';
  import { navigate } from '../lib/router';
  type Props = { open: boolean; onClose: () => void };
  let { open, onClose }: Props = $props();

  const items = $derived([
    { id: 'history',  label: 'History',  desc: "Everything you've read.", path: '/history' },
    { id: 'settings', label: 'Settings', desc: 'Theme, account, security.', path: '/settings' },
    ...($auth.user?.role === 'admin'
      ? [{ id: 'admin', label: 'Admin', desc: 'Operator surface.', path: '/admin' }]
      : []),
  ]);

  function pick(path: string) { onClose(); navigate(path); }
  async function logout() { onClose(); await auth.logout(); window.location.assign('/'); }
</script>

{#if open}
  <div class="tmnav-sheet-backdrop" onclick={onClose} role="presentation"></div>
  <div class="tmnav-sheet" role="dialog" aria-label="More">
    <div class="handle" aria-hidden="true"></div>
    {#if $auth.user}
      <div class="identity">
        <span class="avatar" aria-hidden="true">{($auth.user.username[0] ?? '?').toUpperCase()}</span>
        <span class="who">
          <span class="name">{$auth.user.username}</span>
          <span class="email">user #{$auth.user.id}</span>
        </span>
      </div>
    {/if}
    <div class="list">
      {#each items as it (it.id)}
        <button type="button" class="item" onclick={() => pick(it.path)}>
          <span class="ico"></span>
          <span class="body">
            <span class="name">{it.label}</span>
            <span class="desc">{it.desc}</span>
          </span>
          <span class="chev" aria-hidden="true">›</span>
        </button>
      {/each}
      <div class="sep" aria-hidden="true"></div>
      <button type="button" class="item is-logout" onclick={logout}>
        <span class="ico"></span>
        <span class="body">
          <span class="name">Log out</span>
          <span class="desc">Sign out of {$auth.user?.username ?? ''}</span>
        </span>
      </button>
    </div>
    <div class="foot">
      <button type="button" class="cancel" onclick={onClose}>Close</button>
    </div>
  </div>
{/if}

<style>
  .tmnav-sheet-backdrop {
    position: fixed; inset: 0;
    background: rgba(0,0,0,0.32);
    z-index: 90;
  }
  .tmnav-sheet {
    position: fixed; left: 0; right: 0; bottom: 0;
    z-index: 91;
    background: var(--bg);
    border-radius: 18px 18px 0 0;
    border-top: 1px solid var(--rule);
    padding: 14px 18px calc(env(safe-area-inset-bottom, 0px) + 16px);
    max-height: 80vh; overflow: auto;
  }
  .handle { width: 36px; height: 4px; background: var(--ink-4); border-radius: 4px; margin: 0 auto 14px; }
  .identity { display: flex; align-items: center; gap: 12px; padding: 0 4px 14px; border-bottom: 1px solid var(--rule); }
  .avatar {
    width: 36px; height: 36px; border-radius: 4px;
    background: var(--accent); color: #fff;
    display: inline-flex; align-items: center; justify-content: center;
    font-family: var(--mono); font-size: 15px; font-weight: 600;
  }
  .who { display: flex; flex-direction: column; gap: 2px; }
  .who .name { font-family: var(--sans); font-size: 15px; font-weight: 600; color: var(--ink); }
  .who .email { font-family: var(--mono); font-size: 11px; color: var(--ink-3); }
  .list { padding-top: 8px; }
  .item {
    display: grid; grid-template-columns: 36px 1fr 18px;
    align-items: center; gap: 12px;
    width: 100%;
    padding: 12px 6px;
    background: transparent; border: 0;
    text-align: left; cursor: pointer;
  }
  .item:active { background: var(--bg-soft); }
  .item .ico { width: 36px; height: 36px; }
  .item .body { display: flex; flex-direction: column; gap: 2px; }
  .item .name { font-family: var(--sans); font-size: 15px; font-weight: 600; color: var(--ink); }
  .item .desc { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); }
  .chev { color: var(--ink-4); font-size: 18px; }
  .sep { height: 1px; background: var(--rule); margin: 6px 0; }
  .is-logout .name { color: #b3402c; }
  .is-logout .desc { color: #b3402c; opacity: 0.85; }
  :global(html.theme-dark) .is-logout .name { color: #e9846f; }
  :global(html.theme-dark) .is-logout .desc { color: #e9846f; }
  .foot { display: flex; justify-content: center; padding: 12px 0 4px; }
  .cancel {
    font-family: var(--mono); font-size: 11px;
    color: var(--ink-3);
    padding: 8px 12px; background: transparent; border: 0; cursor: pointer;
    text-transform: uppercase; letter-spacing: 0.12em;
  }
  .cancel:active { background: var(--bg-soft); }
</style>
```

- [ ] **I4g. Implement `MobileTopBar.svelte`** (pure markup; consumes `route` + counts).

```svelte
<script lang="ts">
  import { route } from '../lib/router';
  type Props = { title: string; count?: number; countLabel?: string };
  let { title, count, countLabel }: Props = $props();
</script>

<header class="mhead">
  <a class="wordmark" href="/" aria-label="Tap home">tap<span class="dot" aria-hidden="true"></span></a>
  <h1 class="title">{title}</h1>
  {#if count !== undefined}
    <span class="count"><b>{count}</b>{#if countLabel}<span> {countLabel}</span>{/if}</span>
  {/if}
</header>

<style>
  .mhead {
    display: flex; align-items: baseline; gap: 12px;
    padding: 60px 18px 10px;
    border-bottom: 1px solid var(--rule);
    background: var(--bg);
  }
  .wordmark {
    font-family: var(--sans);
    font-weight: 600; font-size: 18px;
    letter-spacing: var(--tr-display, -0.02em);
    color: var(--ink); text-decoration: none;
    display: inline-flex; align-items: baseline; gap: 1px;
  }
  .dot { width: 5px; height: 5px; border-radius: 50%; background: var(--accent); transform: translateY(-1px); margin-left: 1px; display: inline-block; }
  .title {
    font-family: var(--sans); font-size: 16px; font-weight: 600;
    color: var(--ink);
    margin: 0;
    flex: 1;
  }
  .count { font-family: var(--mono); font-size: 11px; color: var(--ink-3); }
  .count b { color: var(--ink); font-weight: 600; }
</style>
```

- [ ] **I4h. Run all I4 tests, confirm green. Commit.**

```bash
git add web/src/components/MobileTopBar.svelte web/src/components/MobileTabBar.svelte web/src/components/MobileMoreSheet.svelte web/src/components/__tests__/MobileTabBar.test.ts web/src/components/__tests__/MobileMoreSheet.test.ts
git commit -m "M1-I4: mobile chrome (TopBar / TabBar / MoreSheet)"
```

#### I5. SearchOverlay (no-op M1)

- [ ] **I5a. Write failing test.** `web/src/components/__tests__/SearchOverlay.test.ts`:

```ts
import { render } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import SearchOverlay from '../SearchOverlay.svelte';
import { searchOverlay } from '../../lib/searchOverlay.svelte';

describe('SearchOverlay', () => {
  it('hidden when store closed', () => {
    searchOverlay.close();
    const { container } = render(SearchOverlay);
    expect(container.querySelector('.search-overlay')).toBeNull();
  });

  it('renders when store open', () => {
    searchOverlay.openOverlay();
    const { container } = render(SearchOverlay);
    expect(container.querySelector('.search-overlay')).not.toBeNull();
    searchOverlay.close();
  });
});
```

- [ ] **I5b. Run, confirm fail.**

- [ ] **I5c. Implement `SearchOverlay.svelte`.**

```svelte
<script lang="ts">
  import { searchOverlay } from '../lib/searchOverlay.svelte';
</script>

{#if searchOverlay.open}
  <div class="search-overlay" role="dialog" aria-label="Search">
    <div class="scrim" onclick={() => searchOverlay.close()} role="presentation"></div>
    <div class="panel">
      <input
        class="input"
        type="search"
        autofocus
        placeholder="Search (M2 will implement)"
        value={searchOverlay.query}
        oninput={(e) => searchOverlay.setQuery((e.currentTarget as HTMLInputElement).value)}
      />
      <p class="hint">M2 will land the actual filter behaviour.</p>
    </div>
  </div>
{/if}

<style>
  .search-overlay { position: fixed; inset: 0; z-index: 220; display: flex; align-items: flex-start; justify-content: center; padding-top: 64px; }
  .scrim { position: absolute; inset: 0; background: rgba(0,0,0,0.42); backdrop-filter: blur(2px); }
  .panel {
    position: relative;
    background: var(--bg);
    border: 1px solid var(--rule);
    border-radius: 6px;
    padding: 14px 16px;
    width: min(520px, calc(100% - 32px));
    box-shadow: var(--shadow-dialog, 0 24px 60px rgba(0,0,0,0.32));
  }
  .input {
    width: 100%;
    font-family: var(--sans); font-size: 14px;
    padding: 9px 12px;
    border: 1px solid var(--rule);
    border-radius: 4px;
    background: var(--bg-soft);
    color: var(--ink);
  }
  .input:focus { outline: 2px solid var(--accent); outline-offset: 1px; border-color: var(--accent); }
  .hint { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); margin: 10px 4px 0; letter-spacing: 0.04em; }
</style>
```

- [ ] **I5d. Run, confirm green. Commit.**

```bash
git add web/src/components/SearchOverlay.svelte web/src/components/__tests__/SearchOverlay.test.ts
git commit -m "M1-I5: SearchOverlay stub (filter UI lands in M2)"
```

#### I6. AppShell

- [ ] **I6a. Write failing test.** `web/src/components/__tests__/AppShell.test.ts`:

```ts
import { render } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import AppShell from '../AppShell.svelte';
import { createRawSnippet } from 'svelte';

const slot = createRawSnippet(() => ({ render: () => '<div>slot</div>' }));

describe('AppShell', () => {
  it('renders children', () => {
    const { getByText } = render(AppShell, { children: slot });
    expect(getByText('slot')).toBeTruthy();
  });

  it('applies tap theme class to root', () => {
    const { container } = render(AppShell, { children: slot });
    expect(container.querySelector('.tap')).not.toBeNull();
  });
});
```

- [ ] **I6b. Run, confirm fail.**

- [ ] **I6c. Implement `AppShell.svelte`.**

`AppShell.svelte` is a thin layout wrapper: it (a) applies the `.tap.ts-root` (+ `.is-mobile`, `+ .font-sans`) class chain on its root, (b) consumes `isMobile` from `lib/breakpoints.svelte.ts` (NOT a local `$state`), (c) branches the children render between the desktop (`<TopTabs> + <AccountAvatar> + main + <StatusFoot>`) and mobile (`<MobileTopBar> + main + <MobileTabBar>`) trees, (d) computes `pageTitle`, `unread`, and `isAdmin` derivations consumed by the chrome. No further state or behaviour — every interactive piece is delegated to a child component.

```svelte
<script lang="ts">
  import TopTabs from './TopTabs.svelte';
  import AccountAvatar from './AccountAvatar.svelte';
  import StatusFoot from './StatusFoot.svelte';
  import MobileTopBar from './MobileTopBar.svelte';
  import MobileTabBar from './MobileTabBar.svelte';
  import { auth } from '../lib/auth';
  import { route } from '../lib/router';
  import { entries } from '../lib/store';
  import { font } from '../lib/preferences.svelte';
  import { isMobile } from '../lib/breakpoints.svelte';

  type Props = { children: import('svelte').Snippet };
  let { children }: Props = $props();

  const unread = $derived($entries.items.filter(e => !e.read).length);
  const isAdmin = $derived($auth.user?.role === 'admin');

  const pageTitle = $derived(({
    unread: 'Unread', saved: 'Saved', history: 'History',
    categories: 'Categories', feeds: 'Feeds', settings: 'Settings',
    admin: 'Admin', reader: 'Reader',
  } as Record<string, string>)[$route.name] ?? '');
</script>

<div
  class="tap ts-root"
  class:is-mobile={$isMobile}
  class:font-sans={font.value === 'sans'}
>
  {#if $isMobile}
    <MobileTopBar title={pageTitle} count={$route.name === 'unread' ? unread : undefined} countLabel="unread" />
    <main class="mbody">{@render children()}</main>
    <MobileTabBar unreadCount={unread} />
  {:else}
    <div class="shell" class:shell-reader={$route.name === 'reader'}>
      <TopTabs unreadCount={unread} isAdmin={isAdmin} />
      <AccountAvatar />
      <main class="main">{@render children()}</main>
      {#if $route.name !== 'reader'}<StatusFoot />{/if}
    </div>
  {/if}
</div>

<style>
  .tap {
    background: var(--bg);
    color: var(--ink);
    min-height: 100vh;
    position: relative;
    font-family: var(--sans);
  }
  .shell {
    max-width: 720px;
    margin: 0 auto;
    padding: 36px 24px 80px;
    min-height: 100vh;
    display: flex;
    flex-direction: column;
  }
  .shell-reader { max-width: 720px; padding-bottom: 120px; }
  .main { flex: 1; padding-top: 4px; }
  .mbody { padding: 0; }
  .is-mobile { padding-bottom: calc(56px + env(safe-area-inset-bottom, 0px)); }
</style>
```

- [ ] **I6d. Run, confirm green. Commit.**

```bash
git add web/src/components/AppShell.svelte web/src/components/__tests__/AppShell.test.ts
git commit -m "M1-I6: AppShell branching on desktop/mobile"
```

### Group J — Login rewrite

The existing `web/src/views/Login.svelte` uses a flat form. Rewrite to `.tl-root` shell with three modes (password, passkey, OTP-step). Magic link is out of scope; do not add it.

**Reuses:** `Field`, `Button`, `KbdChip`, `OtpInput`.

- [ ] **J1. Read the current Login implementation and its test.** Then write the new failing test. `web/src/views/__tests__/Login.test.ts` — replace contents:

```ts
import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import Login from '../Login.svelte';
import { auth } from '../../lib/auth';

describe('Login (rewritten)', () => {
  beforeEach(() => {
    vi.spyOn(auth, 'login').mockReset();
    vi.spyOn(auth, 'loginWithTOTP').mockReset();
  });

  it('renders password mode by default', () => {
    const { getByText, getByLabelText } = render(Login);
    expect(getByText('Sign in')).toBeTruthy();
    expect(getByLabelText(/email/i)).toBeTruthy();
    expect(getByLabelText(/password/i)).toBeTruthy();
  });

  it('shows passkey button when credentials API is available', () => {
    Object.defineProperty(window.navigator, 'credentials', {
      configurable: true,
      value: { get: vi.fn() },
    });
    const { getByText } = render(Login);
    expect(getByText(/use a passkey/i)).toBeTruthy();
  });

  it('transitions to OTP mode when login returns totp_required', async () => {
    vi.spyOn(auth, 'login').mockResolvedValueOnce({
      totp_required: true, pending_token: 'pt',
    } as unknown as ReturnType<typeof auth.login> extends Promise<infer R> ? R : never);
    const { getByText, getByLabelText, findByText } = render(Login);
    await fireEvent.input(getByLabelText(/email/i), { target: { value: 'ada@x' } });
    await fireEvent.input(getByLabelText(/password/i), { target: { value: 'pw' } });
    await fireEvent.click(getByText(/continue/i));
    expect(await findByText(/verification code/i)).toBeTruthy();
  });

  it('error chip appears when auth.login throws', async () => {
    vi.spyOn(auth, 'login').mockRejectedValueOnce(new Error('Invalid'));
    const { getByText, getByLabelText, findByRole } = render(Login);
    await fireEvent.input(getByLabelText(/email/i), { target: { value: 'ada@x' } });
    await fireEvent.input(getByLabelText(/password/i), { target: { value: 'pw' } });
    await fireEvent.click(getByText(/continue/i));
    expect(await findByRole('alert')).toBeTruthy();
  });

  it('OTP mode can toggle to recovery code', async () => {
    vi.spyOn(auth, 'login').mockResolvedValueOnce({
      totp_required: true, pending_token: 'pt',
    } as unknown as ReturnType<typeof auth.login> extends Promise<infer R> ? R : never);
    const { getByText, getByLabelText } = render(Login);
    await fireEvent.input(getByLabelText(/email/i), { target: { value: 'ada@x' } });
    await fireEvent.input(getByLabelText(/password/i), { target: { value: 'pw' } });
    await fireEvent.click(getByText(/continue/i));
    await fireEvent.click(getByText(/use a recovery code/i));
    expect(getByLabelText(/recovery code/i)).toBeTruthy();
  });
});
```

- [ ] **J2. Run, confirm fail.**

```bash
pnpm --dir web test -- src/views/__tests__/Login.test.ts
```

- [ ] **J3. Rewrite `web/src/views/Login.svelte`.** Full replacement:

```svelte
<!--
  Login is mounted by App.svelte under two conditions:
   (1) URL is /sign-in (route name 'signin'), OR
   (2) $auth.user == null on any other URL (state-aware fallback).
  App.svelte's redirect $effect keeps the two in sync: unauthenticated
  users are pushed to /sign-in; authenticated users on /sign-in are
  pushed to /. Magic-link mode is intentionally out of scope (umbrella §1).
-->
<script lang="ts">
  import Button from '../components/Button.svelte';
  import Field from '../components/Field.svelte';
  import KbdChip from '../components/KbdChip.svelte';
  import OtpInput from '../components/OtpInput.svelte';
  import { auth, ERR_UNAUTHORIZED } from '../lib/auth';

  type Mode = 'password' | 'passkey' | 'otp';

  let mode = $state<Mode>('password');
  let username = $state('');
  let password = $state('');
  let pendingToken = $state('');
  let otp = $state('');
  let useRecovery = $state(false);
  let recovery = $state('');
  let error = $state('');
  let busy = $state(false);

  const passkeyAvailable = typeof window !== 'undefined' && 'credentials' in navigator;

  async function submit(e: Event) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      if (mode === 'otp') {
        await auth.loginWithTOTP(
          pendingToken,
          useRecovery ? undefined : otp,
          useRecovery ? recovery : undefined,
        );
      } else {
        const result = await auth.login(username, password) as { totp_required?: boolean; pending_token?: string };
        if (result?.totp_required && result.pending_token) {
          pendingToken = result.pending_token;
          mode = 'otp';
        }
      }
    } catch (err) {
      error = err instanceof Error && err.message !== ERR_UNAUTHORIZED
        ? err.message
        : 'Invalid email or password.';
    } finally {
      busy = false;
    }
  }

  async function signInWithPasskey() {
    error = '';
    busy = true;
    try {
      const { session_id, options } = await auth.beginPasskeyLogin();
      const credential = await navigator.credentials.get({
        publicKey: parseRequestOptions(options as PublicKeyCredentialRequestOptionsJSON),
      });
      if (!credential) throw new Error('No credential returned');
      await auth.finishPasskeyLogin(session_id, serializeAssertion(credential as PublicKeyCredential));
    } catch (err) {
      error = err instanceof Error ? err.message : 'Passkey login failed.';
    } finally {
      busy = false;
    }
  }

  interface PublicKeyCredentialRequestOptionsJSON {
    challenge: string;
    rpId?: string;
    allowCredentials?: Array<{ id: string; type: string; transports?: string[] }>;
    userVerification?: string;
    timeout?: number;
  }
  function b64urlToBytes(b64: string): Uint8Array {
    const pad = b64.length % 4 === 0 ? '' : '='.repeat(4 - (b64.length % 4));
    const std = (b64 + pad).replace(/-/g, '+').replace(/_/g, '/');
    return Uint8Array.from(atob(std), c => c.charCodeAt(0));
  }
  function bytesToB64url(buf: ArrayBuffer): string {
    return btoa(String.fromCharCode(...new Uint8Array(buf)))
      .replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
  }
  function parseRequestOptions(opts: PublicKeyCredentialRequestOptionsJSON): PublicKeyCredentialRequestOptions {
    return {
      challenge: b64urlToBytes(opts.challenge).buffer as ArrayBuffer,
      rpId: opts.rpId,
      allowCredentials: opts.allowCredentials?.map(c => ({
        id: b64urlToBytes(c.id).buffer as ArrayBuffer,
        type: c.type as PublicKeyCredentialType,
        transports: c.transports as AuthenticatorTransport[] | undefined,
      })),
      userVerification: opts.userVerification as UserVerificationRequirement | undefined,
      timeout: opts.timeout,
    };
  }
  function serializeAssertion(cred: PublicKeyCredential) {
    const resp = cred.response as AuthenticatorAssertionResponse;
    return {
      id: cred.id,
      rawId: bytesToB64url(cred.rawId),
      type: cred.type,
      response: {
        authenticatorData: bytesToB64url(resp.authenticatorData),
        clientDataJSON: bytesToB64url(resp.clientDataJSON),
        signature: bytesToB64url(resp.signature),
        userHandle: resp.userHandle ? bytesToB64url(resp.userHandle) : null,
      },
    };
  }
</script>

<div class="tl-root">
  <header class="tl-header">
    <a class="wordmark" href="/" aria-label="Tap home">tap<span class="dot" aria-hidden="true"></span></a>
  </header>
  <main class="tl-main">
    <div class="tl-col">
      <form class="tl-form" onsubmit={submit}>
        <h1 class="tl-title">
          {mode === 'otp' ? 'Verification code' : 'Sign in'}
        </h1>

        {#if error}
          <div class="tl-error" role="alert">
            <svg class="tl-error-ico" width="13" height="13" viewBox="0 0 16 16" fill="none" aria-hidden="true">
              <path d="M8 2 1.5 13.5h13L8 2Z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/>
              <path d="M8 6.5v3.5" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
              <circle cx="8" cy="11.7" r="0.7" fill="currentColor"/>
            </svg>
            <span>{error}</span>
          </div>
        {/if}

        {#if mode === 'password'}
          <Field
            label="EMAIL"
            bind:value={username}
            type="email"
            placeholder="you@domain.com"
            mono
            autofocus
            autocomplete="username"
            required
          />
          <Field
            label="PASSWORD"
            bind:value={password}
            type="password"
            autocomplete="current-password"
            required
          />
        {:else if mode === 'otp'}
          {#if !useRecovery}
            <label class="otp-row">
              <span class="otp-label">CODE</span>
              <OtpInput value={otp} onChange={(v) => otp = v} disabled={busy} />
            </label>
          {:else}
            <Field
              label="RECOVERY CODE"
              bind:value={recovery}
              type="text"
              mono
              autofocus
              required
            />
          {/if}
        {/if}

        <div class="tl-actions">
          <Button type="submit" variant="primary" disabled={busy}>
            <span>{mode === 'otp' ? 'Verify and continue' : 'Continue'}</span>
            <span aria-hidden="true">→</span>
            <KbdChip>Enter</KbdChip>
          </Button>
          {#if mode === 'password' && passkeyAvailable}
            <Button variant="quiet" disabled={busy} onclick={signInWithPasskey}>
              Use a passkey
            </Button>
          {:else if mode === 'otp'}
            <Button variant="quiet" onclick={() => { useRecovery = !useRecovery; otp = ''; recovery = ''; }}>
              {useRecovery ? 'Use authenticator code' : 'Use a recovery code'}
            </Button>
          {/if}
        </div>
      </form>
    </div>
  </main>
</div>

<style>
  .tl-root {
    background: var(--bg); color: var(--ink);
    font-family: var(--serif);
    min-height: 100vh;
    display: flex; flex-direction: column;
  }
  .tl-header { padding: 22px 32px; }
  .wordmark { font-family: var(--sans); font-weight: 600; font-size: 20px; letter-spacing: -0.02em; color: var(--ink); text-decoration: none; display: inline-flex; align-items: baseline; gap: 1px; }
  .dot { display: inline-block; width: 5px; height: 5px; border-radius: 50%; background: var(--accent); transform: translateY(-1px); margin-left: 1px; }
  .tl-main { flex: 1; display: flex; align-items: center; justify-content: center; padding: 24px; }
  .tl-col { width: 100%; max-width: 380px; }
  .tl-form { display: flex; flex-direction: column; gap: 14px; }
  .tl-title { font-family: var(--serif); font-weight: 600; font-size: 34px; line-height: 1.05; letter-spacing: -0.02em; margin: 0 0 4px; color: var(--ink); text-wrap: balance; }
  .tl-error {
    display: flex; gap: 10px; align-items: flex-start;
    background: rgba(196, 58, 58, 0.06);
    border: 1px solid rgba(196, 58, 58, 0.28);
    border-left-width: 2px; border-left-color: #c43a3a;
    border-radius: 4px;
    padding: 10px 12px;
    color: #c43a3a;
    font-family: var(--sans); font-size: 13px; line-height: 1.45;
  }
  :global(html.theme-dark) .tl-error {
    background: rgba(236, 122, 122, 0.06);
    border-color: rgba(236, 122, 122, 0.32);
    border-left-color: #ec7a7a;
    color: #ec7a7a;
  }
  .tl-error-ico { margin-top: 1px; flex-shrink: 0; }
  .tl-actions { display: flex; flex-direction: column; gap: 8px; margin-top: 14px; }
  .otp-row { display: flex; flex-direction: column; gap: 8px; }
  .otp-label { font-family: var(--mono); font-size: 10px; letter-spacing: 0.14em; text-transform: uppercase; color: var(--ink-3); }
</style>
```

- [ ] **J4. Run Login tests, confirm green.**

```bash
pnpm --dir web test -- src/views/__tests__/Login.test.ts
```

- [ ] **J5. Commit.**

```bash
git add web/src/views/Login.svelte web/src/views/__tests__/Login.test.ts
git commit -m "M1-J: rewrite Login to .tl-root with password/passkey/OTP modes"
```

### Group K — Wire existing views into AppShell

Each existing view (Unread, Reader, Saved, Settings, Admin) currently renders `<Sidebar>` and `<TopBar>` directly. The new pattern is: `App.svelte` wraps everything in `<AppShell>`; each view renders only its body content.

The "minimal restyling" rule: replace native `<select>` with `Segmented`, raw `<button>` with `Button`, ad-hoc modal markup with `Dialog`. No deep rebuild — that's M2–M7's job.

- [ ] **K1. Update `Unread.svelte`.** Remove the `<Sidebar>` and `<TopBar>` wrappers. Keep:
  - `entries.load(true)` + `subscriptions.load()` on mount.
  - The keyboard dispatch wiring (lines 24–55 of the current file).
  - The body list rendering (EntryRow loop).
  - `markAllRead` and `doRefresh` handlers — surface as Button controls in a small toolbar above the list.

Skeleton (read the existing file first, preserve handlers):

```svelte
<script lang="ts">
  import { onMount, onDestroy, getContext } from 'svelte';
  import EntryRow from '../components/EntryRow.svelte';
  import Button from '../components/Button.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import { entries, subscriptions } from '../lib/store';
  import { navigate } from '../lib/router';

  let selectedId = $state<number | null>(null);
  // ... (preserve dispatch wiring from the original file)

  function feedFor(subId: number) {
    return $subscriptions.find(s => s.id === subId);
  }

  async function refresh() { await entries.load(true); }
  async function markAllRead() {
    const ids = $entries.items.map(e => e.id);
    await Promise.allSettled(ids.map(id => entries.toggleRead(id, true)));
  }
</script>

<div class="actions">
  <Button variant="quiet" onclick={refresh}>Refresh</Button>
  <Button variant="quiet" onclick={markAllRead}>Mark all read</Button>
</div>

{#if $entries.loading}
  <p class="loading">Loading…</p>
{:else if $entries.items.length === 0}
  <EmptyState title="Inbox zero." sub="Tap polls every minute. Come back later." />
{:else}
  <div class="list">
    {#each $entries.items as e (e.id)}
      <EntryRow entry={e} feed={feedFor(e.subscription_id)} isSelected={selectedId === e.id} />
    {/each}
  </div>
{/if}

<style>
  .actions { display: flex; gap: 8px; justify-content: flex-end; margin: 8px 0 16px; }
  .loading { font-family: var(--mono); font-size: 11px; color: var(--ink-3); text-align: center; padding: 32px; }
  .list { display: flex; flex-direction: column; }
</style>
```

Update `web/src/views/__tests__/Unread.test.ts` and `UnreadMarkAll.test.ts` to query for the new DOM (no `<Sidebar>`).

- [ ] **K2. Run Unread tests.**

```bash
pnpm --dir web test -- src/views/__tests__/Unread.test.ts src/views/__tests__/UnreadMarkAll.test.ts
```

Fix any selectors that drift. Commit when green:

```bash
git add web/src/views/Unread.svelte web/src/views/__tests__/Unread.test.ts web/src/views/__tests__/UnreadMarkAll.test.ts
git commit -m "M1-K1: wire Unread into AppShell, swap controls for Button"
```

- [ ] **K3. Update `Reader.svelte`.** Remove sidebar/topbar wrappers. Wrap content in a simple back-row + article body. **No** `.ts-article` rebuild — keep the existing markup, just live inside the new shell. Replace native back buttons with `<Button variant="quiet">`. Update the matching test for the removed `<Sidebar>`.

- [ ] **K4. Run Reader tests, fix drift, commit.**

```bash
pnpm --dir web test -- src/views/__tests__/Reader.test.ts
git add web/src/views/Reader.svelte web/src/views/__tests__/Reader.test.ts
git commit -m "M1-K3: wire Reader into AppShell, minimal control swap"
```

- [ ] **K5. Update `Saved.svelte`.** Remove sidebar/topbar wrappers. Use existing list rendering. EmptyState when no saved entries: title "Nothing saved yet", sub "Press S on any entry to keep it here." (per brand spec §6.3). Update its test.

- [ ] **K6. Run Saved tests, fix drift, commit.**

```bash
pnpm --dir web test -- src/views/__tests__/Saved.test.ts
git add web/src/views/Saved.svelte web/src/views/__tests__/Saved.test.ts
git commit -m "M1-K5: wire Saved into AppShell, EmptyState on empty"
```

- [ ] **K7. Update `Settings.svelte`.** Remove sidebar wrapper. Control swaps (M6 will do the full numbered-eyebrow rebuild — M1 only swaps controls to the new primitives so the page doesn't visually clash with the new shell):
  - Native theme `<select>` → `<Segmented options={[{value:'light',label:'Light'},{value:'dark',label:'Dark'},{value:'sepia',label:'Sepia'},{value:'system',label:'System'}]} value={theme.stored} onChange={(v) => theme.stored = v}>`.
  - Native font `<select>` → `<Segmented options={[{value:'serif',label:'Serif'},{value:'sans',label:'Sans'}]} value={font.value} onChange={(v) => font.value = v}>`.
  - Native density `<select>` → `<Segmented options={[{value:'compact',label:'Compact'},{value:'comfortable',label:'Comfortable'},{value:'cosy',label:'Cosy'}]} value={density.value} onChange={(v) => density.value = v}>`. **Note the new vocabulary** — `'compact' | 'comfortable' | 'cosy'`, defaulting to `'comfortable'`. Any existing test assertion that selects an `<option value="default">` or asserts `density.value === 'default'` must be rewritten; legacy stored values migrate automatically (Group E).
  - Any ad-hoc dialog → `<Dialog>`.
  - Add a `measure` control as well (`<Segmented options={[{value:'narrow',label:'Narrow'},{value:'comfortable',label:'Comfortable'},{value:'wide',label:'Wide'}]} value={measure.value} onChange={(v) => measure.value = v}>`) so the M2 article-rebuild can consume it — keep it visually grouped with density; M6 will rearrange these into the §4.7 segmented row.

Update `Settings.test.ts` to:
- Query Segmented buttons (role="radio") instead of native `<select>` options.
- Assert `density.value` round-trips through clicks on `'Compact'` / `'Comfortable'` / `'Cosy'`.
- Drop any assertion that uses the legacy `'default'` density.

Settings.test.ts is also the canonical place to verify the Segmented-store-write contract (Risk §3). One click → one store mutation.

- [ ] **K8. Run Settings tests, fix drift, commit.**

```bash
pnpm --dir web test -- src/views/__tests__/Settings.test.ts
git add web/src/views/Settings.svelte web/src/views/__tests__/Settings.test.ts
git commit -m "M1-K7: wire Settings into AppShell, swap selects for Segmented"
```

- [ ] **K9. Update `Admin.svelte`.** Remove sidebar wrapper. Leave the metric markup as-is; M7 owns the redesign. Confirm tests still pass (Admin has no dedicated test file in this codebase — check `views/__tests__/` and add a minimal smoke test if missing). Commit.

```bash
git add web/src/views/Admin.svelte
git commit -m "M1-K9: wire Admin into AppShell (chrome only; M7 redesigns body)"
```

### Group L — Deletes

Per umbrella §3.3. Delete components and tests no longer used.

- [ ] **L1. Confirm no remaining imports.** Run:

```bash
grep -rn "components/Sidebar\|components/TopBar\|components/SystemActions\|components/SystemStatus\|components/TabBar\|views/Search\|views/Category\|components/FeedSettingsModal\|components/PollerStatus" web/src/
```

Expected: only matches inside files being deleted, or stale imports in `App.svelte` (which Group O removes). If any other view still imports a deleted file, fix it before proceeding — the build will fail otherwise.

- [ ] **L2. Delete files.**

```bash
git rm web/src/components/Sidebar.svelte
git rm web/src/components/TopBar.svelte
git rm web/src/components/SystemActions.svelte
git rm web/src/components/SystemStatus.svelte
git rm web/src/components/TabBar.svelte
git rm web/src/components/FeedSettingsModal.svelte
git rm web/src/components/PollerStatus.svelte
git rm web/src/views/Search.svelte
git rm web/src/views/Category.svelte
git rm web/src/components/__tests__/Sidebar.test.ts
git rm web/src/components/__tests__/TopBar.test.ts
git rm web/src/components/__tests__/SystemActions.test.ts
git rm web/src/components/__tests__/TabBar.test.ts
git rm web/src/components/__tests__/FeedSettingsModal.test.ts
git rm web/src/components/__tests__/PollerStatus.test.ts
```

- [ ] **L3. Run check.**

```bash
pnpm --dir web run check
```

Expected: passes. If TypeScript still complains about a missing import, fix the offender (likely `App.svelte` — addressed in Group O).

- [ ] **L4. Commit.**

```bash
git commit -m "M1-L: delete sidebar/topbar/system/tabbar/search/category/feedsettings components and tests"
```

### Group M — HotkeysModal restyle

- [ ] **M1. Update `HotkeysModal.test.ts`** to assert the new structure: title "Keyboard shortcuts", three groups (Navigation / Actions / App), `.shortcut-row` rows, `.kbd` chips.

- [ ] **M2. Rewrite `HotkeysModal.svelte`** as a two-column grid per brand spec §7. Use the project's `<dialog>` element and tokens. Match the JSX mockup in `ui_design/tap-components.jsx` `ShortcutsModal`.

```svelte
<script lang="ts">
  type Props = { open: boolean; onClose: () => void };
  let { open, onClose }: Props = $props();
  let dialog = $state<HTMLDialogElement | null>(null);

  $effect(() => {
    if (!dialog) return;
    if (open) dialog.showModal();
    else dialog.close();
  });

  const groups = [
    {
      title: 'Navigation',
      rows: [
        { keys: ['j'], desc: 'Next entry' },
        { keys: ['k'], desc: 'Previous entry' },
        { keys: ['o', '↵'], desc: 'Open entry' },
        { keys: ['Esc'], desc: 'Back to list' },
      ],
    },
    {
      title: 'Actions',
      rows: [
        { keys: ['m'], desc: 'Toggle read' },
        { keys: ['s'], desc: 'Toggle saved' },
        { keys: ['v'], desc: 'View original' },
        { keys: ['r'], desc: 'Refresh' },
      ],
    },
    {
      title: 'App',
      rows: [
        { keys: ['/'], desc: 'Search' },
        { keys: ['?'], desc: 'This help' },
      ],
    },
  ];
</script>

<dialog bind:this={dialog} class="modal" aria-labelledby="hk-title" onclose={onClose}>
  <div class="head">
    <span id="hk-title" class="eyebrow">Keyboard shortcuts</span>
    <button class="close" type="button" onclick={onClose} aria-label="Close">
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"><path d="M3.5 3.5l9 9M12.5 3.5l-9 9"/></svg>
    </button>
  </div>
  <div class="body">
    {#each groups as g (g.title)}
      <div class="group">
        <div class="group-title">{g.title}</div>
        {#each g.rows as r, i (i)}
          <div class="row">
            <span class="keys">
              {#each r.keys as k, ki (ki)}
                {#if ki > 0}<span class="plus">then</span>{/if}
                <kbd class="kbd">{k}</kbd>
              {/each}
            </span>
            <span class="desc">{r.desc}</span>
          </div>
        {/each}
      </div>
    {/each}
  </div>
</dialog>

<style>
  .modal {
    background: var(--bg); color: var(--ink);
    border: 1px solid var(--rule);
    border-radius: var(--radius-dialog, 6px);
    width: min(560px, 100%);
    max-height: 80%;
    box-shadow: 0 20px 60px rgba(0,0,0,0.25);
    padding: 0;
  }
  .modal::backdrop { background: rgba(0,0,0,0.32); backdrop-filter: blur(2px); }
  .head { display: flex; align-items: center; justify-content: space-between; padding: 14px 18px; border-bottom: 1px solid var(--rule); }
  .eyebrow { font-family: var(--mono); font-size: 10px; letter-spacing: 0.12em; text-transform: uppercase; color: var(--ink-3); }
  .close { width: 26px; height: 26px; border: 0; background: transparent; color: var(--ink-3); cursor: pointer; border-radius: 4px; display: inline-flex; align-items: center; justify-content: center; }
  .close:hover { background: rgba(0,0,0,0.05); color: var(--ink); }
  .body { padding: 18px 22px 22px; display: grid; grid-template-columns: 1fr 1fr; gap: 22px 32px; }
  .group-title { font-family: var(--mono); font-size: 10px; letter-spacing: 0.12em; text-transform: uppercase; color: var(--ink-3); margin-bottom: 8px; }
  .row { display: flex; align-items: center; justify-content: space-between; padding: 5px 0; font-size: 13px; }
  .keys { display: inline-flex; align-items: center; gap: 6px; }
  .plus { font-family: var(--mono); font-size: 10px; color: var(--ink-3); }
  .desc { color: var(--ink-2); }
  .kbd { font-family: var(--mono); font-size: 11px; padding: 2px 6px; border: 1px solid var(--rule); border-bottom-width: 2px; border-radius: 4px; background: var(--bg-soft); color: var(--ink-2); min-width: 16px; text-align: center; }
</style>
```

- [ ] **M3. Run HotkeysModal test, fix drift, commit.**

```bash
pnpm --dir web test -- src/components/__tests__/HotkeysModal.test.ts
git add web/src/components/HotkeysModal.svelte web/src/components/__tests__/HotkeysModal.test.ts
git commit -m "M1-M: restyle HotkeysModal to .tap-modal two-column grid"
```

### Group N — Stub views

Three placeholder views so navigation works end-to-end.

- [ ] **N1. Create `web/src/views/Categories.svelte`.**

```svelte
<script lang="ts">
  import EmptyState from '../components/EmptyState.svelte';
</script>

<EmptyState
  title="Categories"
  sub="Coming soon — M4. Group feeds into named clusters."
/>
```

- [ ] **N2. Create `web/src/views/Feeds.svelte`.**

```svelte
<script lang="ts">
  import EmptyState from '../components/EmptyState.svelte';
</script>

<EmptyState
  title="Feeds"
  sub="Coming soon — M5. Manage your subscriptions."
/>
```

- [ ] **N3. Create `web/src/views/History.svelte`.**

```svelte
<script lang="ts">
  import EmptyState from '../components/EmptyState.svelte';
</script>

<EmptyState
  title="History"
  sub="Coming soon — M8. Everything you've read, newest first."
/>
```

- [ ] **N4. Commit.**

```bash
git add web/src/views/Categories.svelte web/src/views/Feeds.svelte web/src/views/History.svelte
git commit -m "M1-N: stub views for /categories, /feeds, /history"
```

### Group O — App.svelte rewrite

Wire everything together: replace the old route table with the new one, wrap content in `AppShell`, hook `/` to `searchOverlay`, mount `SearchOverlay`.

- [ ] **O1. Read `App.svelte` and `App.test.ts`.** Update `App.test.ts` to reflect the new mount: no `<TabBar>`, no top-level `<Sidebar>`, single AppShell wrapping current route content. Add two new test cases for the auth-redirect contract (item 4 of reviewer's round 1):
  1. **Unauthenticated user lands on `/` → redirected to `/sign-in`.** Mock `auth.bootstrap()` to resolve with `user: null`; assert `window.location.pathname === '/sign-in'` after the `$effect` flushes (use `tick()` from svelte).
  2. **Authenticated user visits `/sign-in` → redirected to `/`.** Mock `auth` store with a non-null `user`; `navigate('/sign-in')`; assert pathname becomes `/`.

Both tests live in `web/src/views/__tests__/App.test.ts`. Verify the existing `__mocks__/pwa-register-svelte.ts` mock still resolves the `virtual:pwa-register/svelte` import (it does; do not modify).

- [ ] **O2. Rewrite `App.svelte`.**

```svelte
<script lang="ts">
  import { onMount, setContext } from 'svelte';
  import { get } from 'svelte/store';
  import { route, navigate } from './lib/router';
  import { auth } from './lib/auth';
  import { offlineQueue } from './lib/offlineQueue';
  import { warmCache } from './lib/warmCache';
  import { theme, font, density } from './lib/preferences.svelte';
  import { buildHandler } from './lib/keyboard';
  import { searchOverlay } from './lib/searchOverlay.svelte';
  import { useRegisterSW } from 'virtual:pwa-register/svelte';

  import AppShell from './components/AppShell.svelte';
  import HotkeysModal from './components/HotkeysModal.svelte';
  import SearchOverlay from './components/SearchOverlay.svelte';

  import Login from './views/Login.svelte';
  import Unread from './views/Unread.svelte';
  import Reader from './views/Reader.svelte';
  import Saved from './views/Saved.svelte';
  import Categories from './views/Categories.svelte';
  import Feeds from './views/Feeds.svelte';
  import History from './views/History.svelte';
  import Settings from './views/Settings.svelte';
  import Admin from './views/Admin.svelte';

  const { needRefresh, updateServiceWorker } = useRegisterSW();
  let hotkeysOpen = $state(false);

  const dispatch = $state({
    onNext: () => {}, onPrev: () => {}, onOpen: () => {},
    onToggleRead: () => {}, onToggleSaved: () => {}, onViewOriginal: () => {},
  });
  setContext('keyDispatch', dispatch);

  const keyHandler = buildHandler({
    get onNext() { return dispatch.onNext; },
    get onPrev() { return dispatch.onPrev; },
    get onOpen() { return dispatch.onOpen; },
    get onToggleRead() { return dispatch.onToggleRead; },
    get onToggleSaved() { return dispatch.onToggleSaved; },
    get onViewOriginal() { return dispatch.onViewOriginal; },
    onEscape: () => {
      if (searchOverlay.open) { searchOverlay.close(); return; }
      if (hotkeysOpen) { hotkeysOpen = false; return; }
      if ($route.name === 'reader') navigate('/');
    },
    setModalOpen: (open: boolean) => { hotkeysOpen = open; },
  });

  onMount(() => {
    void auth.bootstrap().then(() => {
      const user = get(auth).user;
      if (user) {
        void offlineQueue.drain(user.id);
        setTimeout(() => { void warmCache(user.id); }, 2000);
      }
    });
    const handleOnline = async () => {
      const user = get(auth).user;
      if (user) {
        await offlineQueue.drain(user.id);
        void warmCache(user.id);
      }
    };
    window.addEventListener('online', handleOnline);
    return () => window.removeEventListener('online', handleOnline);
  });

  $effect(() => {
    const html = document.documentElement;
    html.classList.remove('theme-light', 'theme-dark', 'theme-sepia');
    html.classList.add(`theme-${theme.resolved}`);
  });

  $effect(() => {
    document.documentElement.classList.toggle('font-sans', font.value === 'sans');
  });

  $effect(() => {
    const html = document.documentElement;
    html.classList.remove('density-compact', 'density-comfortable', 'density-cosy');
    html.classList.add(`density-${density.value}`);
  });

  // Auth-route redirect contract (umbrella §2.2 + reviewer 2026-05-11 round 1 item 4):
  //   - Unauthenticated and NOT already on /sign-in → navigate('/sign-in').
  //   - Authenticated and ON /sign-in → navigate('/').
  // Single $effect so both checks fire on auth bootstrap and on route change.
  $effect(() => {
    if (!$auth.bootstrapped) return;
    if ($auth.user == null && $route.name !== 'signin') {
      navigate('/sign-in');
    } else if ($auth.user != null && $route.name === 'signin') {
      navigate('/');
    }
  });
</script>

<svelte:window onkeydown={(e) => {
  if (e.key === '/' && !hotkeysOpen) {
    const tag = (e.target as HTMLElement)?.tagName?.toLowerCase();
    if (tag !== 'input' && tag !== 'textarea' && tag !== 'select') {
      e.preventDefault();
      searchOverlay.openOverlay();
      return;
    }
  }
  keyHandler(e);
}} />

{#if $needRefresh}
  <div class="sw-update-banner">
    Update available —
    <button onclick={() => updateServiceWorker(true)}>Reload</button>
  </div>
{/if}

<HotkeysModal open={hotkeysOpen} onClose={() => hotkeysOpen = false} />
<SearchOverlay />

{#if !$auth.bootstrapped}
  <!-- empty during bootstrap window -->
{:else if $route.name === 'signin' || $auth.user == null}
  <Login />
{:else}
  <AppShell>
    {#if $route.name === 'reader'}
      <Reader id={$route.params.id} />
    {:else if $route.name === 'saved'}
      <Saved />
    {:else if $route.name === 'categories'}
      <Categories />
    {:else if $route.name === 'feeds'}
      <Feeds />
    {:else if $route.name === 'history'}
      <History />
    {:else if $route.name === 'settings'}
      <Settings />
    {:else if $route.name === 'admin'}
      {#if $auth.user.role === 'admin'}<Admin />{:else}<p>Access denied.</p>{/if}
    {:else}
      <Unread />
    {/if}
  </AppShell>
{/if}

<style>
  .sw-update-banner {
    position: fixed;
    top: 0; left: 0; right: 0;
    z-index: 9999;
    background: var(--accent);
    color: #fff;
    padding: 8px 16px;
    font-size: 13px;
    display: flex; align-items: center; gap: 16px;
  }
  .sw-update-banner button {
    background: rgba(255,255,255,0.2);
    border: 1px solid rgba(255,255,255,0.5);
    color: #fff;
    padding: 4px 12px;
    border-radius: 4px;
    cursor: pointer;
  }
</style>
```

- [ ] **O3. Run all tests.**

```bash
pnpm --dir web test
```

Expected: all pass. Fix any drift introduced by the rewrite (most likely `App.test.ts` and view tests that still reference removed chrome).

- [ ] **O4. Run svelte-check.**

```bash
pnpm --dir web run check
```

Expected: 0 errors.

- [ ] **O5. Commit.**

```bash
git add web/src/App.svelte web/src/views/__tests__/App.test.ts
git commit -m "M1-O: rewrite App.svelte to wrap routes in AppShell + new route table"
```

### Group P — Verification (manual + automated)

**Invoke `superpowers:verification-before-completion` skill before starting this group.** Every box in this section must be ticked before opening the PR — no exceptions.

#### P1. Full test suite

- [ ] **P1a. Web tests.**

```bash
pnpm --dir web test
```

Expected: green, no failures, no `.only`-skipped suites.

- [ ] **P1b. Type-check.**

```bash
pnpm --dir web run check
```

Expected: 0 errors, 0 warnings related to the new files.

- [ ] **P1c. Go tests.**

```bash
make test
```

Expected: green. The Go side is untouched, but `make test` also rebuilds `web/dist` so this proves the SPA still bundles.

#### P2. Dev-mode smoke

- [ ] **P2a. Start dev server.** In a separate terminal:

```bash
make dev
```

Wait for "Local: http://localhost:5173".

- [ ] **P2b. Use Playwright MCP** (`mcp__plugin_playwright_playwright__*`) to drive the browser. Sequence:

```
browser_navigate http://localhost:5173/
browser_snapshot   # confirms .ts-shell, .ts-nav, .ts-account, .ts-foot present
browser_press_key Tab x N (visit each tab)
browser_navigate http://localhost:5173/categories  # EmptyState "Coming soon — M4"
browser_navigate http://localhost:5173/feeds       # EmptyState "Coming soon — M5"
browser_navigate http://localhost:5173/history     # EmptyState "Coming soon — M8"
browser_press_key /                                 # SearchOverlay opens
browser_press_key Escape                            # SearchOverlay closes
browser_press_key ?                                 # HotkeysModal opens
browser_press_key Escape                            # HotkeysModal closes
```

- [ ] **P2c. Visit `/sign-in`** if not signed in. Take a `browser_take_screenshot` and verify:
  - Wordmark top-left only.
  - 380px centred column.
  - Email field is mono.
  - Password field is sans.
  - "Use a passkey" quiet button visible.
  - No "Create account" link, no footer.

- [ ] **P2d. Mobile viewport.** `browser_resize 390 844`. Verify:
  - `MobileTopBar` at top with 60px inset.
  - Bottom tab bar `.tmnav` with 5 tabs (Unread/Saved/Feeds/Categories/More).
  - Tapping More opens `MobileMoreSheet` with History/Settings items.
  - Sidebar is NOT rendered.

- [ ] **P2e. Theme cycle.** Switch theme via Settings (segmented). Verify dark and sepia both render without colour clashes.

#### P3. Build + offline check

- [ ] **P3a. Production build.**

```bash
make build
```

Expected: `bin/tap` produced, `web/dist/` populated. No build warnings about missing assets.

- [ ] **P3b. Inspect `web/dist/index.html`.** Confirm:
  - No `<link>` to Google Fonts.
  - No `@import url(https://...)` in any CSS file under `web/dist/assets/`. Run:

```bash
grep -r "fonts.googleapis\|fonts.gstatic" web/dist/ || echo "OK: no Google Fonts references"
```

Expected: "OK".

- [ ] **P3c. Run the binary against a fresh DB.**

```bash
bin/tap --db /tmp/m1-smoke.db
```

In a separate terminal, `curl -s http://127.0.0.1:8080/healthz | head` — confirms the binary serves and the embedded SPA loads.

#### P4. Service worker cache invalidation

The asset paths change wholesale in this milestone. The PWA registration is `registerType: 'prompt'` (see `vite.config.ts`), which surfaces a "Reload" banner via `useRegisterSW` when a new build is detected.

- [ ] **P4a. Simulate an upgrade in a logged-in browser.**
  - In Playwright, navigate to `http://localhost:5173/`, sign in.
  - Stop and rebuild (`pnpm --dir web build` in another terminal — or rely on Vite HMR).
  - `browser_evaluate` to force a `navigator.serviceWorker.register` reload simulation:

```js
() => navigator.serviceWorker.getRegistrations().then(rs => Promise.all(rs.map(r => r.update())))
```

  - Refresh once. Confirm the `Update available — Reload` banner appears, click it, and the page reloads cleanly without stale chrome.

- [ ] **P4b. If P4a is impractical in the harness, document the manual verification step** in the PR description: "Tested SW cache invalidation by [...]". Do not skip silently — flag to the reviewer.

#### P5. Final acceptance pass

- [ ] **P5a. Read every "Acceptance criteria" checkbox** in the next section. Tick each only after manual confirmation.

- [ ] **P5b. Re-run `make test` one more time** after acceptance verification, in case manual smoke touched files.

- [ ] **P5c. Open the PR.** Use the title and body from the team-lead's instructions.

---

## Acceptance criteria

Each criterion ties to brand spec / umbrella spec / JSX mockup / `styles.css` selector. Tick once you've verified visually + behaviourally.

### Shell (umbrella §2.1, §3.2)

- [ ] `.tap.ts-root` is the only top-level class applied on authenticated routes (`AppShell.svelte`).
- [ ] Desktop shell `.ts-shell` is 720px wide, centred, with 36px top / 24px side padding (styles.css:1444–1451).
- [ ] `.ts-nav` is sticky-top, hairline-bottom (styles.css:1454–1463).
- [ ] `.ts-account` floats absolute top-right of `.ts-root`, outside the centred column (styles.css:944–963; brand spec §4.1 shell B).
- [ ] `.ts-tabs` lays the tab list with `/` separators between siblings (styles.css:1500–1507).
- [ ] Active tab has a 2px underline 18px below the baseline (styles.css:1531–1540).
- [ ] Unread tab shows a count when `unreadCount > 0` (styles.css:1541–1548; `tap-simple.jsx` `TopTabs`).
- [ ] `.ts-foot` status strip renders pulsing dot + "n unread · idle/polling · ? for shortcuts" on every non-reader route (styles.css:1706–1726).
- [ ] Mobile shell shows `MobileTopBar` (60px status-bar inset, 16/600 title, mono count) and `.tmnav` (5-tab bottom bar) per `tap-mobile-nav.jsx`.
- [ ] `.tmnav-sheet` opens from the More tab with bottom-sheet animation, identity row, History/Settings items, and tinted Log out row.

### Login (brand spec §6.0)

- [ ] `.tl-root` shell: wordmark top-left, 380px centred column, no card chrome, no footer.
- [ ] `.tl-title` is serif 34px / 600 / -0.02em (brand spec §6.0 anatomy).
- [ ] Email field uses mono font (matches brand spec "URLs and identities sit in mono").
- [ ] Password field uses sans default `.ts-field`.
- [ ] Primary button is full-width 42px, `<Button variant="primary">`, with trailing arrow and `Enter` `KbdChip`.
- [ ] Passkey quiet button only renders when `'credentials' in navigator`.
- [ ] OTP mode renders `OtpInput` (six 36×44 cells, mono 17, accent focus ring).
- [ ] OTP mode "Use a recovery code" button toggles into a single text Field; back-toggle returns to OTP.
- [ ] Error state renders `.tl-error` callout (10px left border, accent red, sans 13 body) per `tap-login.css` lines 99–128.
- [ ] No "Create account" link, no footer, no eyebrow on default screen (brand spec §6.0 Don'ts).
- [ ] Mobile variant has 50px status-bar inset, 28px title, 48px primary button (tap-login.css lines 381–407).

### Primitives (umbrella §3.2)

- [ ] `Button` covers default / primary / accent / danger / quiet variants (styles.css:2219–2255).
- [ ] `Field` exposes label + input + optional description + trailing snippet; sans default, mono via prop.
- [ ] `Segmented` renders a pill with hairline border, active button gets shadow + ring (styles.css:2158–2216).
- [ ] `Chip` covers pill / tag / error / backoff variants (styles.css:1037–1062 for `.cat-chip`; 3611+ for `.ts-feeds-chip`).
- [ ] `Popover` shows fixed scrim + positioned content, dismisses on click-outside and Esc.
- [ ] `Dialog` renders head + body + foot with 16/14/20 padding, serif title, Esc/scrim close (styles.css:2581–2640).
- [ ] `KbdChip` is mono 10.5px with hairline + 2px bottom border (styles.css:1691–1703).
- [ ] `OtpInput` distributes pasted 6-digit codes; arrow keys + Backspace navigate cells (styles.css:2488–2507).
- [ ] `RecoveryCodesGrid` renders 2-col grid with leading numeral and line-through on used codes (styles.css:2544–2567).
- [ ] `EmptyState` renders accent dot + serif title + sans sub + optional CTA (styles.css:1664–1690).
- [ ] `EntryRow` shows junction dot (6×6 accent or hollow ring), serif 19/500 title, sans 12 meta, mono 10.5 timestamps, summary clamped to 2 lines (styles.css:1587–1661).
- [ ] `EntryRow` `is-read` tones title to weight 400 / `--ink-3` (no strike-through).
- [ ] `EntryRow` `is-saved` shows the "SAVED" mark in mono accent.
- [ ] `EntryRow` density variants: `compact` hides summary + tightens padding; `cosy` keeps summary clamped to 1 line; `comfortable` default.
- [ ] `GroupHeading` shows mono uppercase label + hairline rule + right-aligned count (styles.css:1557–1580).

### Tokens & fonts (brand spec §2)

- [ ] `tokens.css` has every property listed in Group A1.
- [ ] `:focus-visible` outline is `2px solid var(--accent)` (brand spec §2.6).
- [ ] `@keyframes tap-pulse` and `tf-spin` live in `global.css`.
- [ ] `prefers-reduced-motion` disables both keyframes.
- [ ] No Google Fonts CDN reference in `web/dist/` after build (P3b grep).
- [ ] No `@font-face` block in any file under `web/src/styles/` (fonts come from `@fontsource*` in `main.ts`; verify with `grep "@font-face" web/src/styles/` → empty).
- [ ] No directory `web/src/assets/fonts/` exists (verify with `test ! -d web/src/assets/fonts`).
- [ ] `main.ts` imports the four `@fontsource*` packages **plus** the Source Serif 4 italic CSS (B1, B2).
- [ ] `font-feature-settings: "kern", "liga", "onum"` on body; `"tnum", "zero"` on mono surfaces.
- [ ] `make dev` visual check: Source Serif 4 renders for article body, Inter Tight for UI chrome, JetBrains Mono for timestamps/kbd (B6).

### Routes (umbrella §2.2)

- [ ] `/`, `/entry/:id`, `/saved`, `/categories`, `/feeds`, `/history`, `/settings`, `/admin`, `/sign-in` all resolve.
- [ ] `/search` returns to `/` (route deleted).
- [ ] `/categories/1` returns to `/` (per-category-detail deleted).
- [ ] Admin route renders Admin only when `auth.user.role === 'admin'`.
- [ ] Unauthenticated user visiting `/` is pushed to `/sign-in`.
- [ ] Authenticated user visiting `/sign-in` is pushed to `/`.

### Deletes (umbrella §3.3)

- [ ] `web/src/components/Sidebar.svelte`, `TopBar.svelte`, `SystemActions.svelte`, `SystemStatus.svelte`, `TabBar.svelte`, `FeedSettingsModal.svelte`, `PollerStatus.svelte` no longer exist.
- [ ] `web/src/views/Search.svelte`, `Category.svelte` no longer exist.
- [ ] Their `__tests__/*` siblings no longer exist.
- [ ] No remaining import of any deleted file across `web/src/`.

### Tests (umbrella §6 / CLAUDE.md TDD posture)

- [ ] Every behaviour listed in "TDD posture" has a passing test.
- [ ] No CSS-only / shell-only component has tests (per "Exempt" list).
- [ ] All deleted-component tests are also deleted.
- [ ] `pnpm --dir web run check` passes with 0 errors.

### Service worker (umbrella §7 risks)

- [ ] `Update available — Reload` banner surfaces after a build.
- [ ] After reload, the new shell is in place; no stale Sidebar/TopBar visible.

---

## Verification commands

Run these in order before opening the PR. All must pass.

```bash
# 1. Unit tests
pnpm --dir web test

# 2. Type-check
pnpm --dir web run check

# 3. Frontend production build
pnpm --dir web build

# 4. Verify no Google Fonts reference in built assets
grep -r "fonts.googleapis\|fonts.gstatic" web/dist/ || echo "OK: no Google Fonts references"

# 5. Go tests + binary build (also rebuilds web/dist)
make test
make build

# 6. Manual smoke: start dev server
make dev
# → in browser: visit http://localhost:5173, exercise all tabs, themes, mobile viewport
```

If step 6 finds a regression, fix it before continuing; do not file an issue and move on.

---

## Risks

1. **Font hosting (RESOLVED 2026-05-11).** Team-lead correction: the codebase already uses `@fontsource*` packages and Vite bundles `.woff2` into `web/dist` (which is `go:embed`-ed). Same-origin is already guaranteed. No vendoring, no hand-rolled `@font-face` blocks. Group B's job is verification (grep for `fonts.googleapis`/`fonts.gstatic` in `web/dist` → must come back empty). **Residual risk:** a future `@fontsource*` release could in theory swap to a CDN-fetching CSS; the grep in B5/P3b is the canary. **Backout** (only if needed someday): manually copy `.woff2` into `web/src/assets/fonts/` and write `@font-face` blocks in `global.css`. Not in M1's scope.

2. **Service-worker cache invalidation.** Asset paths change wholesale in M1; every previous build's SW cache becomes stale. The PWA registration is `registerType: 'prompt'`, which surfaces a "Reload" banner via `useRegisterSW`. **Risk:** the banner is dismissable; a user who ignores it sees the old shell. Mitigation: P4 manual test confirms the banner appears and reloading lands on the new build. Implementers must NOT bump the SW strategy to `autoUpdate` in M1 — that's a separate decision documented elsewhere.

3. **`<select>` → `Segmented` store migration.** Settings currently writes to `theme.stored` / `font.value` / `density.value` via native `<select>` events. The new `Segmented` component takes an `onChange` callback. **Risk:** if a Segmented `onChange` is wired to a function that doesn't update the underlying store, the control becomes read-only without the user noticing. Mitigation: tests in `Settings.test.ts` must click each Segmented option and assert the corresponding `theme.stored` / `font.value` / `density.value` mutated.

4. **Two distinct sources of truth for the design.** `ui_design/styles.css` and `ui_design/*.jsx` sometimes disagree (e.g., the JSX mockups still reference a `.tap-sidebar` that the umbrella spec deletes). The umbrella spec §1 calls this out as historical artefact. **Mitigation:** when in doubt, trust `Tap Brand and UI Spec.md` > `styles.css` (matching selector) > JSX mockup (structural reference). The JSX is for "what does it look like assembled", not "what should ship".

5. **PWA manifest icon paths.** `vite.config.ts` references `/icons/icon-192.png` and `/icons/icon-512.png`. These files live outside `web/src` (typically `web/public/icons/`). Verify they still resolve after the M1 reshuffle. **Mitigation:** P2 smoke (Playwright `browser_navigate` will fail loud if `manifest.webmanifest` 404s).

6. **`offline-first WebAuthn` and OTP entry.** The login rewrite preserves all auth flows from M7 (passkeys, TOTP, recovery codes). **Risk:** mode transitions (`password` → `otp`) rely on `auth.login` returning the right shape. The shape is contracted in `lib/types.ts` `LoginResponse`. If the backend's response shape ever drifts, the form silently hangs in the loading state. Mitigation: the new test in J1 explicitly mocks `auth.login` and asserts the mode flips.

7. **`useRegisterSW` is a TypeScript-typed virtual import.** Type-checking can flake if the `vite-plugin-pwa` types lag the Svelte 5 plugin. **Mitigation:** if `pnpm --dir web run check` errors specifically on `virtual:pwa-register/svelte`, check `web/src/__mocks__/pwa-register-svelte.ts` — it provides the test-time stub. The runtime path is fine.

8. **Density preference vocabulary migration (RESOLVED 2026-05-11).** Existing `density` pref values were `'compact' | 'default' | 'comfortable'`. Brand spec §4.4 names the three densities `Compact`, `Comfortable` (default), `Cosy`. **Team-lead decision 2026-05-11:** the brand-spec vocabulary is canonical — `'compact' | 'comfortable' | 'cosy'`, default `'comfortable'`. Group E migrates `localStorage` `'default'` → `'comfortable'` on first read; `preferences.svelte.ts` declares only the three new values; `EntryRow.svelte` accepts only the three new values; Settings (K7) uses the three new values; App.svelte (O2) applies whichever class is current. Planner-m2 has a `grep EntryRow.svelte .ts-entry density` check that fails fast if M1 ships the wrong vocabulary — that check passes here. **No residual risk** for new installs; legacy users see one-time `localStorage` rewrite on first load after upgrade.

9. **AccountAvatar styling assumes parent has `position: relative`.** `.avatar-btn` is `position: absolute` against the nearest positioned ancestor. `.tap` in `AppShell` is `position: relative` — verify this is set, or the avatar will float to the document root. **Mitigation:** included in `AppShell` style block.

---

## Open questions for the umbrella spec

Per the task instructions, do NOT edit `docs/specs/2026-05-11-ui-redesign.md`. Inconsistencies noted here for the reviewer:

1. **Primitive list (RESOLVED 2026-05-11 by team-lead):** umbrella §3.2 listed `EntryRow.svelte` as a single primitive; team-lead amendment defers `SavedRow.svelte` to M3, keeps `EntryRow` in M1. This plan follows the amendment.

2. **`pollStatus` location (RESOLVED 2026-05-11 by umbrella amendment):** umbrella §2.3 now reads "lifted into a new `lib/pollStatus.ts`", matching this plan. No further action.

3. **`/categories/:id` removal vs. M4 future state.** Umbrella §2.2 says `/categories/:id` is replaced by a chip on Unread and a filter on Feeds. The chip lives in M2/M4. Until M4 ships, users have no way to filter by category. Acceptable per the umbrella spec, but worth confirming.

4. **Mobile More sheet for Admin.** Brand spec §4.10 / §6.8 do not mention Admin in the More sheet. Umbrella §2.2 says "More sheet contains History, Settings, optionally Admin, and Log out." This plan includes Admin in the More sheet when the user has admin role. Recommend the brand spec annotate this.

5. **`/sign-in` route (RESOLVED 2026-05-11 by team-lead + reviewer):** umbrella §2.2 lists `/sign-in` (route name `signin`); M1 wires it. App.svelte enforces a two-way redirect: unauthenticated and NOT on `/sign-in` → push `/sign-in`; authenticated and on `/sign-in` → push `/`. Login.svelte is mounted by either URL match or `$auth.user == null` (deep-link + state-aware). See Group F and Group O.

6. **Magic-link mode.** Brand spec §6.0 lists four modes (password / passkey / magic / otp); umbrella §1 explicitly excludes magic. This plan delivers three modes. No issue, just confirming.

7. **`searchOverlay` scope picker.** Brand spec §7 says `/` "focuses search" — scope is implicit. The store carries a `scope` field so M2 can implement scope toggling. M1 ships the field but the overlay UI doesn't expose it. Not a defect.

8. **EmptyState contract (RESOLVED 2026-05-11 by team-lead):** prop is `subtitle: string | Snippet`, `cta` is `{ label, onClick }` (not Snippet), `dot` is `'accent' | 'ink-4'`. M3/M5/M6/M8 plans align. See Group G4.

9. **`isMobile` shared location (RESOLVED 2026-05-11 by team-lead + reviewer):** new file `web/src/lib/breakpoints.svelte.ts` exports `isMobile: Readable<boolean>` from a single module-init `matchMedia('(max-width: 768px)')` listener. M2/M3/M4/M5/M6 all import from this path. See Group D (D9–D13) and AppShell (Group I6c).

---

## Self-review

After authoring this plan, I checked it against the spec with fresh eyes:

- **Spec coverage:** every bullet in umbrella §5 M-Redesign-1 row maps to a Group A–O task. Deletes from §3.3 map to Group L. Stub views map to Group N. HotkeysModal restyle maps to Group M. Login rewrite maps to Group J. Service-worker risk maps to verification P4.
- **Placeholder scan:** no "TBD", "implement later", or generic "handle edge cases" — every code block is complete.
- **Type consistency:** `Segmented`'s `onChange` signature matches its usage in Settings. `Field`'s `bind:value` matches its usage in Login. `EntryRow`'s `density` prop accepts `'compact' | 'comfortable' | 'cosy'` everywhere — store, CSS class names, prop default, Settings Segmented options. Group E migrates legacy `'default'` to `'comfortable'`.
- **Two team-lead amendments baked in:** (1) EntryRow primitive in M1, SavedRow deferred to M3 (header §Amendment). (2) Density vocabulary canonical per brand spec — see Group E and Risks §8.

---

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-11-m-redesign-1-foundations.md`.

Recommended execution: **subagent-driven**. The plan is large (15 groups, ~80 steps); a fresh subagent per group with the team-lead reviewing between groups keeps each session's context tight and lets TDD remain disciplined.

