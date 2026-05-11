# UI Redesign — top-level spec

**Status:** approved 2026-05-11. Implementation split across eight milestones; this spec is the umbrella. Each milestone gets its own dated spec under `docs/specs/`.

**Goal.** Replace the entire SPA UI with the new design committed in `ui_design/` (`Tap Brand and UI Spec.md`, `styles.css`, JSX mockups). Preserve all M1–M11 backend behaviour. Add three new pages — Categories management, Feeds management, History — that the new design defines but the current SPA does not have.

---

## 1. Scope

**In scope:**

- Reskin every existing view (Unread, Reader, Saved, Settings, Login, Admin, Category) to the new design.
- Build three new pages: Categories management (`/categories`), Feeds management (`/feeds`), History (`/history`).
- Replace the sidebar + split-pane shell with the single simple-centred `.ts-shell` for desktop, the `.tap.is-mobile` shell for mobile.
- Adopt the design's brand chrome: wordmark, account-avatar + popover, top tabs (desktop), bottom tabs + More sheet (mobile), `ts-foot` status strip.
- Adopt three themes (light, dark, sepia — sepia already in `tokens.css`), Inter Tight / Source Serif 4 / JetBrains Mono fonts, all design tokens.
- Rewrite Login to the `.tl-root` shell with three modes (password, passkey, OTP/2FA-step). Mode-switching UI per `Tap Brand and UI Spec.md` §6.0.

**Out of scope (this rewrite):**

- Magic-link sign-in (requires backend; not in this rewrite).
- Backend API changes other than narrow additions a milestone explicitly justifies (e.g., category reorder column).
- The split-pane / sidebar / reader-rail variants shown in the design's `Tap Desktop.html` canvas — those are the *old* design and are being replaced. Any mention of `.tap-sidebar`, `.reader-rail`, or the three-pane layout in the design files is treated as a historical artefact, not a target.
- New milestones (M12+) like observability hardening — those continue in parallel under their own specs.

---

## 2. Architecture

### 2.1 Shells

One desktop shell. One mobile shell. Both rooted at `web/src/App.svelte`.

**Desktop (`.tap .theme-{light|dark|sepia} .ts-root`):**

```
<AppShell>
  <TopTabs />              // .ts-nav: wordmark + tab list (+ unread count on Unread tab)
  <AccountAvatar />        // .ts-account: fixed top-right; opens <AccountMenu>
  <main class="ts-shell">  // 720px centred column; .ts-shell-reader on /entry/:id
    {route content}
  </main>
  <footer class="ts-foot"> // status strip: pulse dot + "n unread · polled Xm ago · ? for shortcuts"
</AppShell>
```

**Mobile (`.tap.is-mobile`):**

```
<AppShell is-mobile>
  <MobileTopBar />         // wordmark + page title + count; 60px top inset
  <main>                   // full-bleed scroll column
    {route content}
  </main>
  <MobileTabBar />         // .tmnav: 4 fixed tabs + More
  <MobileMoreSheet />      // .tmnav-sheet: identity + History + Settings + Log out
</AppShell>
```

The two shells share `AppShell.svelte`, which branches on a media-query-driven `isMobile` state already wired in the current `App.svelte`.

### 2.2 Routing

`web/src/lib/router.ts` gains three routes and loses one:

| Path | Route name | Desktop tab | Mobile location | Component |
|---|---|---|---|---|
| `/` | `unread` | Unread | bottom tab | `views/Unread.svelte` |
| `/entry/:id` | `reader` | — (back to Unread) | dedicated reader shell | `views/Reader.svelte` |
| `/saved` | `saved` | Saved | bottom tab | `views/Saved.svelte` |
| `/categories` | `categories` | Categories | bottom tab | `views/Categories.svelte` (new content) |
| `/feeds` | `feeds` | Feeds | bottom tab | `views/Feeds.svelte` (new) |
| `/history` | `history` | History | More sheet | `views/History.svelte` (new) |
| `/settings` | `settings` | Settings | More sheet | `views/Settings.svelte` |
| `/admin` | `admin` | Admin (admin role only) | More sheet (admin only) | `views/Admin.svelte` |
| `/sign-in` | `signin` | — | — | `views/Login.svelte` |

**Removed:** `/search` (route + view). Replaced by a `/`-triggered `<SearchOverlay>` that filters entries within the current scope (Unread, Saved, or All). Foundations milestone wires the overlay component + key handler with no-op filter behaviour; M2 implements the actual search.

**Removed:** `/categories/:id` per-category-detail view. The current `Category.svelte`'s job — filtering entries to one category — becomes a chip on `/` (Unread) and a filter on `/feeds`.

Desktop top-tabs roster: **Unread · Saved · History · Categories · Feeds · Settings · (Admin if role=admin)**. Mobile bottom-tab roster: **Unread · Saved · Feeds · Categories · More**. (The mobile More sheet contains History, Settings, optionally Admin, and Log out.)

### 2.3 State

The current Svelte 5 stores stay. Additions:

| Store / pref | Where | Purpose |
|---|---|---|
| `prefs.measure` | `lib/preferences.svelte.ts` | `narrow` / `comfortable` / `wide` — article body max-width on `.ts-article` |
| `searchOverlay` | new `lib/searchOverlay.svelte.ts` | `{ open, query, scope }`; opened by `/` keystroke. Foundations ships the store + key handler with an empty overlay component; M2 adds the actual filter UI. |
| `pollStatus` (promoted) | existing `PollerStatus.svelte` state lifted into `lib/store.ts` | renders in `ts-foot` on every page, not just Unread |

No new client-side route state shapes beyond the new route names.

### 2.4 Data flow / backend

No new endpoints required for the foundations milestone or for M2 (Unread+Reader), M3 (Saved), or M8 (History). The existing `/api/v1/entries` with no `unread` flag returns all entries newest-first; History reuses it. Saved already filters via `?saved=1`. Day-band groupings ("Today / Yesterday / This week / Earlier") are computed client-side from `published_at`.

**M5 (Feeds) adds a refresh flag.** Per the Feeds management page's per-row + bulk "Refresh" action, M5 extends `PATCH /api/v1/subscriptions/:id` to accept `{ refresh_now: true }`, which calls `Scheduler.Poke()` to advance the feed's next-poll time. This is the canonical refresh mechanism for the whole SPA — M6's "Refresh all now" in Settings iterates over `/api/v1/subscriptions` and calls this PATCH per feed. No separate `/refresh` endpoint or `POST /api/v1/poll-all`.

**M6 (Settings) adds an account self-delete endpoint.** Brand spec §6.6 row 07 specifies "Delete account (danger, opens confirm dialog)". The existing admin `DELETE /api/v1/admin/users/{id}` is admin-only; M6 adds `DELETE /api/v1/me` for users to delete their own account. Cascade via existing FKs (sessions, passkeys, subscriptions, entries, categories all reference `user_id`). No other backend changes for M6.

**M7 (Admin) needs a small additive backend change.** The existing `/api/v1/status` (admin-gated) returns version, uptime, db health, and poll info, but not instance-wide feeds/entries aggregates. Rather than introduce a new endpoint, M7 extends the existing `statusResponse` struct with `feeds_total`, `feeds_ok`, `feeds_with_errors`, `offending_feeds`, `entries_total`, and `entries_24h` fields. The Admin metric grid (FEEDS / ENTRIES / ERRORS / POLL) then sources every cell from one fetch. Status remains the single instance-level admin endpoint; no new route registration.

M4 (Categories management) needs *one* additive backend change if reorder is in scope: a `position INTEGER` column on `categories` + a `PATCH /api/v1/categories/:id` accepting `{position}` (or `POST /api/v1/categories/reorder` accepting an ordered ID list). The Categories milestone spec decides whether reorder ships or gets dropped. Everything else (inline rename, delete, reassign feed, mark-all-read) is already supported.

M5 (Feeds management) does not require new endpoints. Bulk actions (refresh selected, delete selected, set-category-on-selected) are N×1 calls to existing per-feed endpoints. If a future milestone wants true batch endpoints for performance, that's separate.

---

## 3. Component decomposition

`styles.css` (5,696 lines) is the visual source of truth; the JSX mockups are structural reference. CSS is **fully extracted into scoped Svelte `<style>` blocks per component**; only tokens, font-faces, global reset, focus ring, scrollbar, reduced-motion guard, and the two sanctioned keyframes (`tap-pulse`, `tf-spin`) remain global.

### 3.1 Global stylesheets

| File | Role |
|---|---|
| `web/src/styles/tokens.css` | All design tokens from brand spec §2: theme colour vars, type stack vars, type-scale vars, spacing scale, radii, motion durations |
| `web/src/styles/global.css` | Body reset, `:focus-visible` outline rule, scrollbar, `prefers-reduced-motion` guard, `@keyframes tap-pulse` and `tf-spin`. Font `@font-face` declarations come from the existing `@fontsource-variable/source-serif-4`, `@fontsource-variable/inter-tight`, and `@fontsource/jetbrains-mono` packages already imported in `web/src/main.ts`; they bundle into `web/dist` via Vite and ship same-origin inside the embedded SPA |
| `web/src/styles/print.css` *(optional, low priority)* | `theme-print` stack fallback for browser print |

### 3.2 New / rebuilt Svelte components

Foundations milestone owns the chrome:

| Component | JSX source | Owns |
|---|---|---|
| `components/AppShell.svelte` | `tap-simple.jsx` `TapSimple` root | theme class, font class, isMobile branching, layout |
| `components/TopTabs.svelte` | `tap-simple.jsx` `TopTabs` | wordmark + tab list (`.ts-nav`, `.ts-tabs`) |
| `components/AccountAvatar.svelte` | `tap-simple.jsx` `.ts-account` | fixed top-right circle, opens AccountMenu |
| `components/AccountMenu.svelte` | `tap-components.jsx` `AccountMenu` | `.tap-popover` with Account / Theme / Log out |
| `components/StatusFoot.svelte` | `tap-simple.jsx` `TSStatus` | `.ts-foot` status strip |
| `components/MobileTopBar.svelte` | mobile head in `tap-simple.jsx` | wordmark + page title + count + 60px status-bar inset |
| `components/MobileTabBar.svelte` | `tap-mobile-nav.jsx` | 5-tab bottom bar |
| `components/MobileMoreSheet.svelte` | `tap-mobile-nav.jsx` `.tmnav-sheet` | bottom sheet: identity + History + Settings + (Admin) + Log out |
| `components/SearchOverlay.svelte` | — | `/`-triggered filter overlay |

Foundations milestone also delivers the **primitive component library** (every view uses these):

| Primitive | JSX source | Variants |
|---|---|---|
| `Button.svelte` | `.ts-btn` family | default / primary / accent / danger / quiet, optional icon, size |
| `Field.svelte` | `.ts-field` family | label + input (sans or mono) + description; flat or `.is-stacked` |
| `Segmented.svelte` | `.ts-segmented` | theme / font / density / measure pickers |
| `Chip.svelte` | `.cat-chip`, `.ts-feeds-chip`, `.ts-feed-err-chip` | pill, tag, error, backoff variants |
| `Popover.svelte` | `.tap-popover` | scrim + positioned content; reused by AccountMenu, reassign, sort |
| `Dialog.svelte` | `.ts-dialog` / `.tap-modal` | head + body + foot; `is-wide`; `ts-dialog-warn` callout |
| `KbdChip.svelte` | `.ts-kbd` | inline keyboard key |
| `OtpInput.svelte` | `.ts-otp` | six-cell mono input with paste handling |
| `RecoveryCodesGrid.svelte` | `.ts-codes` | 2-col grid; used/unused state |
| `EmptyState.svelte` | `.ts-empty` | dot + serif title + sans sub + optional CTA |
| `EntryRow.svelte` (rewritten) | `tap-components.jsx` `EntryRow` + `tap-simple.jsx` `TSEntryRow` | `.entry` row with junction dot, density + read/saved states. Consumed by Unread (M2) and History (M8). |
| `GroupHeading.svelte` | `tap-simple.jsx` `TSGroupHeading` | day-band heading inside lists |
| `FeedAvatar.svelte` (existing, restyled) | existing | 14×14 icon or colour square |

**Saved view uses a separate row component, not `EntryRow`.** The design's `.ts-saved-row` family in `styles.css` (lines 4924–5057) is visually distinct from `.entry`: bookmark rail-mark instead of junction dot, hover-reveal `.ts-saved-actions` strip (Open / Mark read / Unsave), date eyebrow ("published Apr 26, 2026") instead of relative time, dedicated is-read styling. Rather than overload `EntryRow` with a `variant` prop for one consumer, the Saved view ships its own `SavedRow.svelte` as a view-specific component built in **M3**, not in M1's primitive library. `EntryRow` remains the primitive for Unread (M2) and History (M8); History reuses `EntryRow` with `is-read` styling plus `GroupHeading`, with no dedicated `HistoryRow`.

**Ownership principle for domain-specific composites.** Components that compose M1 primitives into a domain-specific shape (e.g., `CategoryReassignPopover.svelte` = M1's `Popover` + a category list with "Uncategorised") are owned by the **first milestone that needs them** and imported by subsequent consumers. Example: M4 owns `CategoryReassignPopover.svelte` (used by the Categories management page); M5 imports the same component for per-row and bulk category change on the Feeds page. The component lives at `web/src/components/CategoryReassignPopover.svelte` (flat under `components/`, not under a per-milestone subdir) so cross-milestone imports stay clean.

Per-view composites live in `views/*.svelte` and own only layout for that view.

### 3.3 Deletes

Foundations milestone removes:

- `web/src/components/Sidebar.svelte`
- `web/src/components/TopBar.svelte` (replaced by `<AccountAvatar>` + `<StatusFoot>` + view-local headers)
- `web/src/components/SystemActions.svelte` (folded into `AccountMenu`)
- `web/src/components/SystemStatus.svelte` (folded into Admin metric grid in M7)
- `web/src/components/TabBar.svelte` (replaced by `MobileTabBar.svelte`)
- `web/src/views/Search.svelte` (replaced by SearchOverlay)
- `web/src/views/Category.svelte` (replaced by `Categories.svelte` management page + Unread filter)
- `web/src/components/FeedSettingsModal.svelte` (its job moves into the Feeds management page row actions in M5)
- `web/src/components/HotkeysModal.svelte` is **kept** but restyled to the `.tap-modal` two-column shortcut grid in foundations.

---

## 4. CSS adoption strategy

`styles.css` stays in `ui_design/` as a read-only reference. **No file in `web/` will be a copy of it.** For each Svelte component:

1. Identify the design selector(s) that component owns (`.ts-btn`, `.ts-segmented`, etc.).
2. Translate those rules into the component's scoped `<style>` block, preserving:
   - Token usage (`var(--accent)`, `var(--rule)`, etc.) verbatim.
   - Selector relationships (e.g., `.ts-btn.is-primary` becomes `.btn.is-primary` inside the component, with `is-primary` a class prop).
   - Theme-conditional rules (`html.theme-dark .ts-foo` becomes `:global(html.theme-dark) .foo` inside the component's scoped block).
3. For cross-component selectors (e.g., `.ts-shell .ts-list`), the parent owns the cross-cutting rule with `:global(...)` on the child class. Keep this sparing.

The brand spec's selectors are not law in `web/`; the design intent (visual, spacing, motion) is. Renaming `.ts-btn` → `.btn` inside scoped components is fine and expected, as long as the rendered output matches the design canvas.

---

## 5. Milestone breakdown

Eight milestones, each its own dated spec, each its own PR or PR-cluster. TDD posture per `CLAUDE.md` — pure CSS/markup is exempt; anything with state, branches, or error handling has tests.

| # | Spec date | Scope | Notable risks |
|---|---|---|---|
| 1 | M-Redesign-1 (Foundations) | Token expansion, global stylesheets, new shell components, primitive library, Login rewritten to `.tl-root` (password / passkey / OTP-step), all existing views wired into new shell with minimal restyling, Sidebar/TopBar/SystemActions/SystemStatus/TabBar/Search/Category deleted, **stub views created for `/categories`, `/feeds`, `/history`** (each renders an EmptyState reading "Coming soon — Mn" inside the new shell), HotkeysModal restyled. Fonts are already wired via `@fontsource` packages in `main.ts`; foundations only verifies bundling is correct | Largest milestone; mid-transition pages look plain but consistent |
| 2 | M-Redesign-2 (Unread + Reader) | `.entry` row complete (junction dot, density variants, saved mark), day-band groupings, `.ts-article` with reader-rule/lede/end + action row, measure control, mark-on-scroll preference, search overlay implementation | Lose the sibling list / reader rail — replaced by ←Back to Unread and (optional) keyboard `J`/`K` for prev/next |
| 3 | M-Redesign-3 (Saved) | `.ts-shell` Saved list, EmptyState ("Press S on any entry…") | Small milestone |
| 4 | M-Redesign-4 (Categories management) | New `/categories` page: `.ts-cats` cards, inline rename, reassign-feed popover, mark-all-read, Uncategorised pseudo-cat, empty state with example chips | Reorder up/down needs a backend `position` column + endpoint, or gets dropped. Spec call. |
| 5 | M-Redesign-5 (Feeds management) | New `/feeds` page: search+add toolbar, filter chips (All/Errors/Unread/Stale), sort dropdown, bulk action bar, per-feed row, expanded health panel, add-feed dialog with discovery list, import OPML dialog, export OPML | Largest non-foundations milestone. Bulk ops N×1 to start. |
| 6 | M-Redesign-6 (Settings) | Numbered eyebrow sections (`01 · APPEARANCE` … `07 · DATA`), segmented controls (theme/font/density/measure), reading toggles, sync settings, account, security (TOTP enrol + passkey list + recovery codes via new `Dialog`), sessions, data export/import/delete | TOTP / passkey flows already work; this is a chrome rebuild |
| 7 | M-Redesign-7 (Admin) | Metric grid (FEEDS / ENTRIES / ERRORS / POLL), errors table, existing user-management surface restyled into new design language | Small additive backend change: extend `/api/v1/status` `statusResponse` struct with `feeds_total`, `feeds_ok`, `feeds_with_errors`, `offending_feeds`, `entries_total`, `entries_24h` fields. Status remains the single instance-level admin endpoint. Recent-errors list is already on `/api/v1/status`. |
| 8 | M-Redesign-8 (History) | New `/history` page: flat chronological list of all entries received (`/api/v1/entries` with no unread filter — returns everything newest-first), day-band groupings | Smallest milestone; pure frontend |

Mobile parity is **not** a separate milestone — each milestone ships desktop + mobile + all three themes for the page(s) it owns.

Each milestone spec must cover: new components, deleted components, route changes (if any), test posture (which tests are TDD-required vs scaffolding), manual smoke checklist, and any backend addition with its own migration / endpoint test.

---

## 6. Testing posture

Per `CLAUDE.md`: TDD non-negotiable for branches, state, error handling; pure CSS/markup exempt.

- **TDD-required**: keyboard handlers, store updates, debouncing (SearchOverlay), popover open/close + click-outside dismissal, Dialog focus trap + Esc, Segmented selection updates, OtpInput paste + tab navigation, MobileMoreSheet open/close, day-band bucketing logic, sign-in mode transitions, every view's load / error / empty / loading / optimistic-write paths.
- **Exempt**: shell layout components (`AppShell`, `TopTabs`, `AccountAvatar`, mobile bars when they have no behaviour), CSS-only primitives (`KbdChip`, `EmptyState` if pure markup, `Chip` if no interactive state).
- Existing tests in `web/src/components/__tests__/` and `web/src/lib/__tests__/` carry forward and get updated as components are rewritten. Test files for deleted components are removed in the milestone that deletes them.
- `pnpm --dir web run check` (svelte-check) is part of every milestone's PR checklist.

---

## 7. Risks & trade-offs

1. **Mid-transition cosmetic inconsistency.** Approach A means M2–M8 sees pages with new chrome but old content styling. Mitigation: Foundations milestone gives each old view a CSS quick-pass — replace native `<select>` with `Segmented`, use the new Button component, apply tokens — so colours and typography don't clash with the new chrome. Not a polish pass, just a "looks like it belongs" pass.
2. **CSS extraction effort is large.** 5,696 lines into scoped Svelte components is real work. Each milestone plan must list a "CSS port" task per component it rebuilds. The `ui_design/styles.css` file stays in the repo as the source of truth; do not delete it after the rewrite, and do not import it from `web/`.
3. **Category reorder needs backend work.** The design's Categories management page shows reorder-up / reorder-down on every card. The current `categories` table has no `position` column. M4's spec decides: ship reorder (add column + endpoint + migration) or drop it from M4 and revisit. Lean: ship it, since it's a small addition and the design implies it's load-bearing.
4. **Bulk feed operations.** The Feeds management page shows bulk refresh / delete / set-category. M5 ships these as N×1 calls to existing per-feed endpoints. Performance is fine at the scale of "the user's own feeds." If bulk endpoints become necessary later, that's separate.
5. **Search regression.** Removing `/search` and replacing with an overlay is a UX change. The overlay must support the same affordances (debounced search, results list, click-through to reader). If the overlay proves worse in practice, the dedicated route can be re-added at low cost.
6. **Font hosting.** The Tap binary is meant to work offline / on a self-hosted server with no third-party fetches. The three font families (Source Serif 4, Inter Tight, JetBrains Mono) are already bundled via `@fontsource-variable/source-serif-4`, `@fontsource-variable/inter-tight`, and `@fontsource/jetbrains-mono` packages, imported in `web/src/main.ts`. Vite bundles them into `web/dist`, which is `go:embed`-ed into the static binary. Same-origin guarantee already holds — foundations milestone only confirms the three families load and apply correctly; no hand-rolled `@font-face` blocks needed.
7. **Service worker cache invalidation.** Foundations milestone changes nearly every asset path. The existing PWA registration prompts the user to reload on `needRefresh`; that should be enough, but the foundations milestone should manually verify a stale SW doesn't pin the old shell.

---

## 8. References

- Design source of truth: `ui_design/Tap Brand and UI Spec.md` (read first), `ui_design/styles.css` (visual source), `ui_design/*.jsx` (structural reference, do not port verbatim).
- Current SPA: `web/src/`, particularly `lib/router.ts`, `lib/store.ts`, `lib/preferences.svelte.ts`, `App.svelte`.
- Project conventions: `CLAUDE.md`, `docs/concept.md`, `docs/roadmap.md`.
- Prior milestone specs: `docs/specs/2026-05-08-m1-walking-skeleton.md` through `docs/specs/2026-05-10-m11-archival-tombstones.md`.
