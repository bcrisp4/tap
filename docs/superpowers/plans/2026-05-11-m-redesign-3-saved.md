# M-Redesign-3 — Saved Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild `web/src/views/Saved.svelte` on the new `.ts-shell` simple-centred layout from M-Redesign-1. Replace the current `Sidebar + TopBar + EntryRow` layout with the design-spec `.ts-saved-toolbar` + flat chronological `.ts-saved-list`, the `.ts-saved-empty` state, and a mobile variant with swipe-to-unsave / swipe-to-toggle-read gestures.

**Architecture:** Pure SPA milestone, zero backend change. The view reuses primitives shipped by M-Redesign-1 (the `.ts-shell` chrome, `EmptyState`, `KbdChip`, `FeedAvatar` restyled, `MobileTopBar`, the `isMobile` reactivity). The list is flat — no day-band grouping, no per-feed grouping — per umbrella spec §5 row M3 and Brand spec §6.3. A pinned count banner (the `.ts-saved-toolbar`) sits at the top of the list. Mobile rows ship a separate `.ts-saved-m-row` component that wraps each row in a swipe-attached card with two reveal layers (left: Mark read/unread; right: Unsave, destructive). The swipe attachment reuses the existing pure recogniser in `web/src/lib/swipe.ts`; M-Redesign-3 does not modify that file.

**State model — single source of truth via the global `entries` store.** The view does NOT keep a view-local copy of items. Instead it (i) loads the saved set into the global `entries` store via a small new method `entries.loadSaved()` (Task 0), (ii) derives its rendered list from `$entries.items.filter(e => e.saved)`, and (iii) routes every mutation through the existing `entries.toggleSaved(id, false)` / `entries.toggleRead(id, read)` methods, which already do optimistic update with rollback in `web/src/lib/store.ts`. When `toggleSaved(id, false)` succeeds the entry's `saved` flag flips to `false`, the filter predicate stops matching, and the row falls out of the rendered list naturally — no view-local bookkeeping required. This keeps M3 consistent with the rest of the codebase (Unread also uses the same store), and it means changes made elsewhere (e.g. a future hotkey on Reader) propagate to Saved without a refresh.

**Tech Stack:** Svelte 5 (runes, `{@attach}` directive, `<svelte:window>`), TypeScript, Vite, Vitest + `@testing-library/svelte`. No new dependencies.

**Hard preconditions on M-Redesign-1 (Foundations).** This plan does not ship until M1 has delivered all of the following. If any are missing or renamed at execution time, fix M1 first and rebase — do not work around them in M3:

- The `.ts-shell` simple-centred desktop chrome (`AppShell.svelte`, `TopTabs.svelte`, `AccountAvatar.svelte`, `StatusFoot.svelte`) wired in `App.svelte` so the `/saved` route renders inside the new shell.
- The mobile chrome (`MobileTopBar.svelte`, `MobileTabBar.svelte`, `MobileMoreSheet.svelte`).
- **`isMobile`** — exported as a `Readable<boolean>` from `web/src/lib/preferences.svelte.ts` (or an equivalent location M1 chooses; the export name is `isMobile`). Subscribed via `$isMobile` store syntax in views. (M1 currently keeps `isMobile` as a local `$state` rune inside `App.svelte`; promoting it to a module-level `Readable<boolean>` is part of M1's chrome work because every route needs to know.)
- **`EmptyState.svelte`** — primitive accepting `title: string`, `subtitle: Snippet` (so an inline `<KbdChip>S</KbdChip>` can be slotted into the "Press S on any entry…" line), and `tone: 'accent' | 'muted'` (defaults to accent dot). The Snippet-bearing subtitle is required by M3.
- **`KbdChip.svelte`** — primitive rendering `.ts-kbd` for a single key. M3 consumes this directly; it does not ship a local `<kbd>` reimplementation.
- **`FeedAvatar.svelte`** — restyled to the new 14×14 / 11×11 / 10×10 size conventions; same `feedURL` + `size` + `radius` prop surface as today.
- The token + global-stylesheet structure under `web/src/styles/` (tokens.css, global.css).
- Router awareness of the `/saved` route name (already routed; M3 only rewrites the view body).
- **EntryRow ownership.** Umbrella spec §3.2 lists `EntryRow.svelte (rewritten)` as the foundation primitive for the *generic* simple-shell list views — Unread (M2), History (M8), and the Reader rail. Saved uses a structurally different row (`.ts-saved-row` — grid-with-rail + hover action strip + eyebrow) and ships its own `SavedRow.svelte` primitive in M3, not M1. The umbrella's "every view uses these" line holds for the generic list views; Saved is the documented exception, alongside views that were already exceptions (Categories management with `.ts-cat`, Feeds management with `.ts-feed-row`). The structural divergence (grid-with-rail vs absolutely-positioned dot; presence of a third hover-revealed action row; presence of a mono caps eyebrow above the title) is real, not a styling variant — a single primitive with a `variant` prop would be a switch statement around two different DOM trees. Decision recorded 2026-05-11 by team-lead and confirmed by planner-m2.

---

## Skills and tools for implementers

Always-on:

- **`superpowers:test-driven-development`** — INVOKE AT THE START of every behaviour-bearing task. Mandated by `CLAUDE.md` ("TDD is non-negotiable"). Pure CSS / markup-only steps are exempt; everything else (Task 0 `entries.loadSaved` store method; load → list/error/empty branching; click/swipe action handlers; focus/mouseenter row plumbing; mobile swipe-attached actions; keyboard `S` toggle) is in scope.
- **`superpowers:verification-before-completion`** — before marking a task done, run the exact verification command in the task's final step and confirm output matches expected output. Evidence before assertions.

Reach for as needed:

- **`svelte-runes`** — `$state` for `loading` / `error` / `items` / `swipeId` in the view, `$derived` for `unread` count, `$props` on every component, `$effect` for the mount-time load. No `.svelte.ts` stores in M3 (the new state is view-local, not shared).
- **`svelte-styling`** — CSS extraction strategy follows umbrella spec §4. Every `ui_design/styles.css` `.ts-saved-*` rule lands in a scoped `<style>` block on the component that owns the selector; the design class is renamed (`.ts-saved-row` → `.row` inside `SavedRow.svelte`) where renaming improves locality. Cross-component selectors use `:global()` on the child class. Tokens (`var(--accent)`, `var(--rule)`, etc.) are referenced verbatim.
- **`svelte-template-directives`** — `{@attach swipe(...)}` on `.ts-saved-m-row` (mobile only). `<svelte:window onkeydown>` is owned by `App.svelte` per M-Redesign-1; this view binds via the existing `keyDispatch` context — do not add a second window-level listener.
- **`svelte-components`** — semantic HTML: each row is a `<button>` (the whole row is clickable to open the entry), the action buttons stop propagation. Empty state is `<section>`, list is `<ul role="list">` with `<li>` items wrapping the row button.
- **`tdd`** — only invoke if `superpowers:test-driven-development` is unavailable; same red/green/refactor loop.

MCP tools:

- **`context7` (`mcp__plugin_context7_context7__query-docs`)** — usually not needed; reach for it only if `{@attach}` semantics, Svelte 5 `$state` array reactivity, or `@testing-library/svelte` user-event behaviour are uncertain. No new third-party swipe library — `web/src/lib/swipe.ts` is the only gesture primitive.

---

## File structure

| Path | Action | Responsibility |
|---|---|---|
| `web/src/views/Saved.svelte` | **rewrite** | Composes the page. Calls `entries.loadSaved()` on mount, derives the rendered list as `$derived(() => $entries.items.filter(e => e.saved))`, routes mutations through `entries.toggleSaved` / `entries.toggleRead`, branches desktop vs mobile, renders the count banner, wires the keyboard `S` handler via `keyDispatch` context, and tracks the focused/hovered row in a `focusedId` rune so keyboard `S` knows which row to act on. |
| `web/src/lib/store.ts` | **modify** | Add `entries.loadSaved()` method — fetches `api.listEntries({ saved: true, limit: 100 })` and replaces `items`. The existing `toggleSaved` / `toggleRead` methods are unchanged and reused. |
| `web/src/lib/__tests__/store.test.ts` | **modify (or create if absent)** | Add a test for `entries.loadSaved()`: calls `api.listEntries({ saved: true, limit: 100 })`, populates `items`, sets loading false. If the test file doesn't exist, create it. |
| `web/src/views/__tests__/Saved.test.ts` | **create** | Load → list, load → error, load → empty, optimistic unsave from row, mobile swipe-left unsave, mobile swipe-right toggle-read. |
| `web/src/components/SavedToolbar.svelte` | **create** | The pinned `.ts-saved-toolbar` count banner with serif "Saved" eyebrow + mono count + `Find /` kbd hint. View-specific to Saved (M3-owned, NOT a primitive in M1's library). Consumed only by `views/Saved.svelte`. Consumes `KbdChip` from M1. No sort menu in M3 (out of scope; see §"Out of scope"). |
| `web/src/components/__tests__/SavedToolbar.test.ts` | **create** | Renders correct count for 0 / 1 / N (verifies singular/plural). |
| `web/src/components/SavedRow.svelte` | **create** | Desktop Saved row — `.ts-saved-row` body (eyebrow, serif title, byline) + revealed `.ts-saved-actions` row (Open, Mark read/unread, Unsave). View-specific to Saved (M3-owned, NOT a primitive in M1's library). Consumed only by `views/Saved.svelte`. Reuses `FeedAvatar.svelte` from M1. (Brand spec §6.3 does not list a row summary on the Saved row; the underlying `EntryListItem` DTO has no `summary` field either, so this component renders title + byline only.) |
| `web/src/components/__tests__/SavedRow.test.ts` | **create** | Renders read / unread variants; click-row navigates; action buttons fire correct callback and stop propagation; correct icon swap for Mark read vs Mark unread. |
| `web/src/components/SavedMobileRow.svelte` | **create** | Mobile Saved row — wraps `.ts-saved-m-card` in two reveal layers (`.ts-saved-m-rev-left`, `.ts-saved-m-rev-right`) plus a `{@attach swipe(...)}` directive that fires the destructive / read-toggle action on touchend past threshold. View-specific to Saved (M3-owned, NOT a primitive in M1's library). Consumed only by `views/Saved.svelte`. |
| `web/src/components/__tests__/SavedMobileRow.test.ts` | **create** | Swipe-left fires `onUnsave`; swipe-right fires `onToggleRead`; reveal classes apply mid-swipe (visual feedback); release past threshold commits the action; release before threshold rolls back the class. |
| `web/src/views/Saved.svelte` `<style>` block | **CSS port** | Owns view-level layout: `.list` wrapper around `SavedRow`s, optional vertical spacing under the toolbar. References `ui_design/styles.css` lines 4920–5113 (desktop) and 5115–5250 (mobile) for visual truth; selectors are renamed inside scoped styles. |
| `web/src/components/SavedToolbar.svelte` `<style>` block | **CSS port** | `.ts-saved-toolbar` family — lines 4796–4843 in `ui_design/styles.css`. |
| `web/src/components/SavedRow.svelte` `<style>` block | **CSS port** | `.ts-saved-row` + `.ts-saved-rail` + `.ts-saved-body` + `.ts-saved-eyebrow` + `.ts-saved-title` + `.ts-saved-byline` + `.ts-saved-summary` + `.ts-saved-actions` + `.ts-saved-action` — lines 4920–5057 in `ui_design/styles.css`. |
| `web/src/components/SavedMobileRow.svelte` `<style>` block | **CSS port** | `.ts-saved-m-row` family — lines 5137–5250 in `ui_design/styles.css`. **Do not port lines 5177–5178** (`.ts-saved-m-row.is-swipe-left .ts-saved-m-card { transform: translateX(-110px); }` and `.is-swipe-right .ts-saved-m-card { transform: translateX(140px); }`) — these depend on a touchmove stream that `web/src/lib/swipe.ts` does not emit (the recogniser fires only on `touchend`). Porting them would render an inert hover-state rule that would mislead a future reader. Likewise, do not author the `.is-swipe-left` / `.is-swipe-right` classes themselves in the component. |
| `web/src/components/EmptyState.svelte` | (no change) | M1 ships this. M3 calls it from `Saved.svelte` with: `title="Nothing saved yet"`, `tone="accent"`, and a `subtitle` snippet that renders "Press " + `<KbdChip>S</KbdChip>` + " on any entry to keep it here." The Snippet-bearing subtitle is a hard precondition on M1 (see "Hard preconditions" above) — M3 must not ship its own inline empty state. |
| `web/src/components/KbdChip.svelte` | (no change) | M1 ships this. M3 consumes it inside `SavedToolbar.svelte` (the `Find /` hint) and inside the `EmptyState` subtitle snippet. M3 must not ship a local `<kbd>` reimplementation. |

### Out of scope for M3 (explicit)

- **Day-band groupings** ("Today / This week / Earlier"). The JSX mockup `ui_design/tap-saved.jsx` groups by `savedBucket`, but Brand spec §6.3 says "A flat chronological list of every entry where `saved === true`. No grouping by feed." We follow the spec, not the JSX. The grouping is also a non-goal in umbrella spec §5 row M3 ("flat chronological list"). If the user later wants groupings, they can be added trivially using M2's day-band bucketing helper.
- **Sort menu** (`Recently saved`, `Oldest first`, `Title A–Z`, `Source`). The JSX mockup ships one; the Brand spec doesn't. Default ordering is whatever the existing `GET /api/v1/entries?saved=1` returns (newest first by `published_at`). M3 ships sort as a future iteration if needed.
- **Per-row "saved Xd ago" eyebrow.** The DB does not currently store `saved_at`. The eyebrow line in `SavedRow.svelte` shows the publication time ("published Apr 26, 2026") and the read status only. Adding `saved_at` is a backend change deferred to a separate milestone.
- **Author byline.** The JSX has a distinct author field; `EntryListItem` already exposes `author`. Use `author` when present, otherwise omit the field; do not invent placeholder data.
- **Keyboard navigation in the saved list (`J`/`K`).** M-Redesign-2 owns the `selectedId` + arrow-nav pattern. M3's only keyboard hook is `S` on a hovered/focused row to unsave (binds through the existing `keyDispatch.onToggleSaved` set by M2 if M2 has shipped; otherwise, M3 wires its own `onToggleSaved` against the currently focused row).
- **Backend API changes.** None. `GET /api/v1/entries?saved=1` already exists; `PATCH /api/v1/entries/:id` with `{saved: false}` already exists.

---

## Phase A — Component primitives

### Task 0: Add `entries.loadSaved()` to the global store with TDD

**Skills:** `superpowers:test-driven-development`, `svelte-runes` (only insofar as the existing store uses Svelte's `writable`; no new rune work).

**Files:**
- Modify: `web/src/lib/store.ts`
- Modify: `web/src/lib/__tests__/store.test.ts`

Rationale: Saved view consumes the global `entries` store so optimistic `toggleSaved` / `toggleRead` and rollback fall out for free. The existing `entries.load(unreadOnly)` only takes `unread`; it has no `saved` flag. Add a focused `loadSaved()` method rather than overloading `load()` — the call sites are distinct (Unread mounts call `load(true)`; Saved mounts call `loadSaved()`).

- [ ] **Step 1: Write the failing test**

Append the following block to `web/src/lib/__tests__/store.test.ts` (or, if the file already has a `describe('entries', ...)` block, add the test inside it):

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { entries } from '../store';
import { api } from '../api';

vi.mock('../api', () => ({
  api: { listEntries: vi.fn(), patchEntry: vi.fn() },
}));

describe('entries.loadSaved', () => {
  beforeEach(() => { vi.clearAllMocks(); });

  it('fetches saved=true and populates items', async () => {
    vi.mocked(api.listEntries).mockResolvedValueOnce({
      data: [
        { id: 1, subscription_id: 3, title: 'a', url: 'u', author: '',
          published_at: 1700000000, fetched_at: 1700000000,
          read: false, saved: true, extract_failed: false },
      ],
      next_cursor: null,
    });
    await entries.loadSaved();
    expect(api.listEntries).toHaveBeenCalledWith({ saved: true, limit: 100 });
    const state = get(entries);
    expect(state.loading).toBe(false);
    expect(state.error).toBeNull();
    expect(state.items).toHaveLength(1);
    expect(state.items[0].saved).toBe(true);
  });

  it('records the error and clears items when the API rejects', async () => {
    vi.mocked(api.listEntries).mockRejectedValueOnce(new Error('boom'));
    await entries.loadSaved();
    const state = get(entries);
    expect(state.items).toHaveLength(0);
    expect(state.loading).toBe(false);
    expect(state.error).toBe('boom');
  });
});
```

If the existing test file already has a `vi.mock('../api', ...)` block at the top, do not duplicate it — reuse the existing mock and add only the `describe('entries.loadSaved', ...)` block.

- [ ] **Step 2: Run the test to verify it fails**

```bash
pnpm --dir web test -- src/lib/__tests__/store.test.ts -t loadSaved
```

Expected: FAIL with `entries.loadSaved is not a function`.

- [ ] **Step 3: Implement the method**

In `web/src/lib/store.ts`, inside the object returned by `entriesStore()`, add this method (immediately after `load`):

```typescript
async loadSaved() {
  set({ items: [], loading: true, error: null });
  try {
    const r = await api.listEntries({ saved: true, limit: 100 });
    set({ items: r.data, loading: false, error: null });
  } catch (e) {
    set({ items: [], loading: false, error: (e as Error).message });
  }
},
```

This mirrors `load(unreadOnly)`'s shape exactly. No behaviour change for existing call sites — Unread keeps calling `load(true)`.

- [ ] **Step 4: Run the test to verify it passes**

```bash
pnpm --dir web test -- src/lib/__tests__/store.test.ts -t loadSaved
```

Expected: PASS, both tests.

- [ ] **Step 5: Run the full store test suite for regressions**

```bash
pnpm --dir web test -- src/lib/__tests__/store.test.ts
```

Expected: all pre-existing tests still pass.

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/store.ts web/src/lib/__tests__/store.test.ts
git commit -m "M-Redesign-3: add entries.loadSaved() for Saved view consumption"
```

---

### Task 1: Scaffold `SavedToolbar.svelte` with TDD

**Skills:** `superpowers:test-driven-development`, `svelte-runes`, `svelte-styling`.

**Files:**
- Create: `web/src/components/SavedToolbar.svelte`
- Create: `web/src/components/__tests__/SavedToolbar.test.ts`

- [ ] **Step 1: Write the failing test**

Replace the contents of `web/src/components/__tests__/SavedToolbar.test.ts` with:

```typescript
import { render, screen } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import SavedToolbar from '../SavedToolbar.svelte';

describe('SavedToolbar', () => {
  it('renders zero count as "0 entries"', () => {
    const { container } = render(SavedToolbar, { props: { count: 0 } });
    const countEl = container.querySelector('.count');
    expect(countEl).not.toBeNull();
    expect(countEl!.textContent!.replace(/\s+/g, ' ').trim()).toMatch(/^0\s+entries$/);
  });

  it('renders singular for 1 ("1 entry")', () => {
    const { container } = render(SavedToolbar, { props: { count: 1 } });
    const countEl = container.querySelector('.count');
    expect(countEl).not.toBeNull();
    // The word "entry" is part of the same text node as the number;
    // assert on the count container's full label.
    expect(countEl!.textContent!.replace(/\s+/g, ' ').trim()).toMatch(/^1\s+entry$/);
  });

  it('renders plural for N > 1 ("12 entries")', () => {
    const { container } = render(SavedToolbar, { props: { count: 12 } });
    const countEl = container.querySelector('.count');
    expect(countEl).not.toBeNull();
    expect(countEl!.textContent!.replace(/\s+/g, ' ').trim()).toMatch(/^12\s+entries$/);
  });

  it('renders the find hint with the / kbd chip', () => {
    const { container } = render(SavedToolbar, { props: { count: 3 } });
    const find = container.querySelector('.find');
    expect(find).not.toBeNull();
    // KbdChip is a child component; assert via textContent rather than a child-specific selector.
    expect(find!.textContent).toMatch(/find/i);
    expect(find!.textContent).toContain('/');
  });

  it('renders the "Saved" serif eyebrow', () => {
    render(SavedToolbar, { props: { count: 3 } });
    expect(screen.getByText('Saved')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
pnpm --dir web test -- src/components/__tests__/SavedToolbar.test.ts
```

Expected: FAIL with `Failed to resolve import "../SavedToolbar.svelte"` (the file does not yet exist).

- [ ] **Step 3: Write the minimal component**

Replace the contents of `web/src/components/SavedToolbar.svelte` with:

```svelte
<script lang="ts">
  import KbdChip from './KbdChip.svelte';

  type Props = { count: number };
  let { count }: Props = $props();
  const word = count === 1 ? 'entry' : 'entries';
</script>

<div class="toolbar" role="heading" aria-level="1">
  <div class="left">
    <span class="eyebrow">Saved</span>
    <span class="count"><b>{count}</b> {word}</span>
  </div>
  <div class="right">
    <span class="find">Find <KbdChip>/</KbdChip></span>
  </div>
</div>

<style>
  .toolbar {
    display: flex; align-items: baseline; justify-content: space-between;
    gap: 16px;
    padding: 20px 2px 14px;
    border-bottom: 1px solid var(--rule);
    margin-bottom: 4px;
  }
  .left { display: flex; align-items: baseline; gap: 14px; flex-wrap: wrap; }
  .eyebrow {
    font-family: var(--serif); font-size: 26px; font-weight: 600;
    letter-spacing: -0.02em; color: var(--ink);
  }
  .count {
    font-family: var(--mono); font-size: 11px;
    color: var(--ink-3); letter-spacing: 0.04em;
  }
  .count b { color: var(--ink); font-weight: 500; }
  .right { display: flex; align-items: center; gap: 14px; flex-shrink: 0; }
  .find {
    font-family: var(--mono); font-size: 11px;
    letter-spacing: 0.04em; color: var(--ink-3);
    display: inline-flex; align-items: center; gap: 6px;
  }
</style>
```

Notes:
- The `<b>` inside `.count` is selected by a normal descendant selector — Svelte's scoper handles it correctly. Matches `styles.css`'s `.ts-saved-toolbar-count b` rule.
- "Find" is mixed-case (matches `tap-saved.jsx:148`'s `Find` text inside `.ts-saved-toolbar-find`). The brand spec mono rule for labels is UPPER, but this is a hint, not a label.
- The keyboard chip is rendered by `<KbdChip>/</KbdChip>` — this primitive is a hard precondition on M1. If `KbdChip.svelte` is missing at execution time, fix M1, do not stub it locally.

- [ ] **Step 4: Run the test to verify it passes**

```bash
pnpm --dir web test -- src/components/__tests__/SavedToolbar.test.ts
```

Expected: PASS, all 5 tests.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/SavedToolbar.svelte web/src/components/__tests__/SavedToolbar.test.ts
git commit -m "M-Redesign-3: scaffold SavedToolbar count banner with TDD"
```

---

### Task 2: Scaffold `SavedRow.svelte` (desktop) with TDD

**Skills:** `superpowers:test-driven-development`, `svelte-runes`, `svelte-components`.

**Files:**
- Create: `web/src/components/SavedRow.svelte`
- Create: `web/src/components/__tests__/SavedRow.test.ts`

- [ ] **Step 1: Write the failing test**

Replace the contents of `web/src/components/__tests__/SavedRow.test.ts` with:

```typescript
import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import SavedRow from '../SavedRow.svelte';
import type { EntryListItem, Subscription } from '../../lib/types';

const entry: EntryListItem = {
  id: 7, subscription_id: 3, title: 'Reasons bugs feel impossible',
  url: 'https://jvns.ca/x', author: 'Julia Evans',
  published_at: Math.floor(Date.now() / 1000) - 60 * 32,
  fetched_at: Math.floor(Date.now() / 1000),
  read: false, saved: true, extract_failed: false,
};

const feed = {
  id: 3, feed_url: 'https://jvns.ca/feed.xml', title: 'Julia Evans',
} as Subscription;

describe('SavedRow', () => {
  it('renders the title, source name, and author', () => {
    render(SavedRow, { props: { entry, feed } });
    expect(screen.getByText('Reasons bugs feel impossible')).toBeInTheDocument();
    expect(screen.getByText('Julia Evans')).toBeInTheDocument();
  });

  it('fires onOpen when the row body is clicked', async () => {
    const onOpen = vi.fn();
    render(SavedRow, { props: { entry, feed, onOpen } });
    await fireEvent.click(screen.getByText('Reasons bugs feel impossible'));
    expect(onOpen).toHaveBeenCalledOnce();
  });

  it('fires onToggleRead and does NOT fire onOpen when Mark read is clicked', async () => {
    const onOpen = vi.fn(), onToggleRead = vi.fn();
    render(SavedRow, { props: { entry, feed, onOpen, onToggleRead } });
    await fireEvent.click(screen.getByRole('button', { name: /mark read/i }));
    expect(onToggleRead).toHaveBeenCalledOnce();
    expect(onOpen).not.toHaveBeenCalled();
  });

  it('fires onUnsave and does NOT fire onOpen when Unsave is clicked', async () => {
    const onOpen = vi.fn(), onUnsave = vi.fn();
    render(SavedRow, { props: { entry, feed, onOpen, onUnsave } });
    await fireEvent.click(screen.getByRole('button', { name: /unsave/i }));
    expect(onUnsave).toHaveBeenCalledOnce();
    expect(onOpen).not.toHaveBeenCalled();
  });

  it('renders "Mark unread" for a read entry', () => {
    render(SavedRow, { props: { entry: { ...entry, read: true }, feed } });
    expect(screen.getByRole('button', { name: /mark unread/i })).toBeInTheDocument();
  });

  it('applies is-read class when entry.read is true', () => {
    const { container } = render(SavedRow, { props: { entry: { ...entry, read: true }, feed } });
    expect(container.querySelector('.row')?.classList.contains('is-read')).toBe(true);
  });

  it('fires onMouseEnter when the row is hovered', async () => {
    const onMouseEnter = vi.fn();
    const { container } = render(SavedRow, { props: { entry, feed, onMouseEnter } });
    await fireEvent.mouseEnter(container.querySelector('.row')!);
    expect(onMouseEnter).toHaveBeenCalledOnce();
  });

  it('fires onFocus when the row is focused', async () => {
    const onFocus = vi.fn();
    const { container } = render(SavedRow, { props: { entry, feed, onFocus } });
    await fireEvent.focus(container.querySelector('.row')!);
    expect(onFocus).toHaveBeenCalledOnce();
  });

  it('applies is-focused class when isFocused prop is true', () => {
    const { container } = render(SavedRow, { props: { entry, feed, isFocused: true } });
    expect(container.querySelector('.row')?.classList.contains('is-focused')).toBe(true);
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
pnpm --dir web test -- src/components/__tests__/SavedRow.test.ts
```

Expected: FAIL with `Failed to resolve import "../SavedRow.svelte"`.

- [ ] **Step 3: Write the minimal component**

Replace the contents of `web/src/components/SavedRow.svelte` with:

```svelte
<script lang="ts">
  import FeedAvatar from './FeedAvatar.svelte';
  import type { EntryListItem, Subscription } from '../lib/types';

  type Props = {
    entry: EntryListItem;
    feed: Subscription | undefined;
    isFocused?: boolean;
    onFocus?: () => void;
    onMouseEnter?: () => void;
    onOpen?: () => void;
    onToggleRead?: () => void;
    onUnsave?: () => void;
  };
  let {
    entry, feed,
    isFocused = false,
    onFocus, onMouseEnter,
    onOpen, onToggleRead, onUnsave,
  }: Props = $props();

  function publishedLabel(ts: number): string {
    return new Date(ts * 1000).toLocaleDateString(undefined, {
      month: 'short', day: 'numeric', year: 'numeric',
    });
  }

  function handleAction(ev: Event, fn?: () => void) {
    ev.stopPropagation();
    fn?.();
  }
</script>

<article
  class="row"
  class:is-read={entry.read}
  class:is-focused={isFocused}
  role="button"
  tabindex="0"
  onclick={() => onOpen?.()}
  onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onOpen?.(); } }}
  onfocus={() => onFocus?.()}
  onmouseenter={() => onMouseEnter?.()}
>
  <span class="rail" aria-hidden="true">
    <span class="rail-mark">
      <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
        <path d="M4 2.5h8v11l-4-3-4 3z" />
      </svg>
    </span>
  </span>

  <div class="body">
    <div class="eyebrow">
      <span>published {publishedLabel(entry.published_at)}</span>
      {#if entry.read}
        <span class="sep" aria-hidden="true">·</span>
        <span class="status-read">read</span>
      {/if}
    </div>

    <h3 class="title">{entry.title}</h3>

    <div class="byline">
      {#if feed}
        <FeedAvatar feedURL={feed.feed_url} size={11} radius={2} />
        <span class="source">{feed.title}</span>
      {/if}
      {#if entry.author}
        <span class="sep" aria-hidden="true">·</span>
        <span class="author">{entry.author}</span>
      {/if}
    </div>
  </div>

  <div class="actions" role="group" aria-label="Saved entry actions">
    <button
      type="button" class="action"
      onclick={(e) => handleAction(e, onOpen)}
      aria-label="Open entry"
    >
      <span>Open</span>
    </button>
    <button
      type="button" class="action"
      onclick={(e) => handleAction(e, onToggleRead)}
      aria-label={entry.read ? 'Mark unread' : 'Mark read'}
    >
      <span>{entry.read ? 'Mark unread' : 'Mark read'}</span>
    </button>
    <button
      type="button" class="action is-destructive"
      onclick={(e) => handleAction(e, onUnsave)}
      aria-label="Unsave"
    >
      <span>Unsave</span>
    </button>
  </div>
</article>

<style>
  .row {
    position: relative;
    display: grid;
    grid-template-columns: 20px 1fr;
    gap: 14px;
    padding: 18px 2px 18px 0;
    border: 0;
    border-bottom: 1px solid var(--rule);
    background: transparent;
    width: 100%;
    text-align: left;
    cursor: pointer;
    transition: background 100ms ease;
  }
  .row:hover { background: var(--bg-soft); }
  .row:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
  .row:last-child { border-bottom: 0; }

  .rail {
    position: relative;
    display: flex;
    justify-content: center;
    padding-top: 4px;
  }
  .rail-mark {
    width: 18px; height: 18px;
    display: inline-flex; align-items: center; justify-content: center;
    color: var(--accent);
  }
  .row.is-read .rail-mark { color: var(--ink-4); }

  .body { min-width: 0; }

  .eyebrow {
    display: flex; align-items: center; gap: 8px;
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.06em; text-transform: uppercase;
    color: var(--ink-3);
    margin-bottom: 6px;
  }
  .status-read { color: var(--ink-3); }
  .row.is-read .status-read { color: var(--accent); }

  .title {
    font-family: var(--serif); font-size: 20px; line-height: 1.25;
    font-weight: 500; color: var(--ink);
    margin: 0 0 6px;
    text-wrap: pretty;
    letter-spacing: -0.005em;
  }
  .row.is-read .title { color: var(--ink-2); font-weight: 400; }

  .byline {
    display: flex; align-items: center; gap: 8px;
    font-family: var(--sans); font-size: 12.5px;
    color: var(--ink-2);
    margin-bottom: 6px;
    flex-wrap: wrap;
  }
  .source { color: var(--ink); font-weight: 500; }
  .row.is-read .source { color: var(--ink-2); font-weight: 400; }
  .author { color: var(--ink-2); font-style: italic; }

  .sep::before {
    content: ""; display: inline-block;
    width: 3px; height: 3px; border-radius: 50%;
    background: var(--ink-4);
    vertical-align: middle;
  }

  .actions {
    grid-column: 1 / -1;
    display: flex;
    gap: 4px;
    margin-top: 4px;
    margin-left: 34px;
    max-height: 0;
    opacity: 0;
    overflow: hidden;
    transition: max-height 160ms ease, opacity 120ms ease, margin-top 160ms ease;
  }
  .row:hover .actions,
  .row:focus-within .actions {
    max-height: 50px;
    opacity: 1;
    margin-top: 8px;
  }
  .action {
    display: inline-flex; align-items: center; gap: 7px;
    padding: 6px 10px;
    font-family: var(--sans); font-size: 12px; font-weight: 500;
    color: var(--ink-2);
    background: transparent;
    border: 0;
    border-radius: 3px;
    cursor: pointer;
    transition: background 100ms ease, color 100ms ease;
  }
  .action:hover { color: var(--ink); background: var(--bg); }
  .action.is-destructive:hover { color: var(--accent); }
</style>
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
pnpm --dir web test -- src/components/__tests__/SavedRow.test.ts
```

Expected: PASS, all 6 tests.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/SavedRow.svelte web/src/components/__tests__/SavedRow.test.ts
git commit -m "M-Redesign-3: SavedRow desktop primitive with action buttons"
```

---

### Task 3: Scaffold `SavedMobileRow.svelte` with swipe TDD

**Skills:** `superpowers:test-driven-development`, `svelte-template-directives`, `svelte-runes`.

**Files:**
- Create: `web/src/components/SavedMobileRow.svelte`
- Create: `web/src/components/__tests__/SavedMobileRow.test.ts`

- [ ] **Step 1: Write the failing test**

Replace the contents of `web/src/components/__tests__/SavedMobileRow.test.ts` with:

```typescript
import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import SavedMobileRow from '../SavedMobileRow.svelte';
import type { EntryListItem, Subscription } from '../../lib/types';

const entry: EntryListItem = {
  id: 7, subscription_id: 3, title: 'A mobile saved entry',
  url: 'https://jvns.ca/x', author: 'Julia Evans',
  published_at: Math.floor(Date.now() / 1000) - 60 * 60 * 24,
  fetched_at: Math.floor(Date.now() / 1000),
  read: false, saved: true, extract_failed: false,
};

const feed = {
  id: 3, feed_url: 'https://jvns.ca/feed.xml', title: 'Julia Evans',
} as Subscription;

function touchEvent(name: string, x: number, y: number): TouchEvent {
  // Vitest happy-dom does not implement TouchEvent fully; mock the surface.
  const ev = new Event(name, { bubbles: true }) as unknown as TouchEvent;
  Object.defineProperty(ev, 'touches', { value: [{ clientX: x, clientY: y }] });
  Object.defineProperty(ev, 'changedTouches', { value: [{ clientX: x, clientY: y }] });
  return ev;
}

describe('SavedMobileRow', () => {
  it('fires onUnsave when the user swipes left past threshold', async () => {
    const onUnsave = vi.fn();
    const { container } = render(SavedMobileRow, {
      props: { entry, feed, onUnsave, onToggleRead: vi.fn() },
    });
    const row = container.querySelector('.row')!;
    await fireEvent(row, touchEvent('touchstart', 200, 100));
    await fireEvent(row, touchEvent('touchend', 100, 100)); // dx = -100 (left)
    expect(onUnsave).toHaveBeenCalledOnce();
  });

  it('fires onToggleRead when the user swipes right past threshold', async () => {
    const onToggleRead = vi.fn();
    const { container } = render(SavedMobileRow, {
      props: { entry, feed, onUnsave: vi.fn(), onToggleRead },
    });
    const row = container.querySelector('.row')!;
    await fireEvent(row, touchEvent('touchstart', 100, 100));
    await fireEvent(row, touchEvent('touchend', 200, 100)); // dx = +100 (right)
    expect(onToggleRead).toHaveBeenCalledOnce();
  });

  it('does not fire callbacks for sub-threshold swipes', async () => {
    const onUnsave = vi.fn(), onToggleRead = vi.fn();
    const { container } = render(SavedMobileRow, {
      props: { entry, feed, onUnsave, onToggleRead },
    });
    const row = container.querySelector('.row')!;
    await fireEvent(row, touchEvent('touchstart', 200, 100));
    await fireEvent(row, touchEvent('touchend', 195, 100)); // dx = -5, under threshold
    expect(onUnsave).not.toHaveBeenCalled();
    expect(onToggleRead).not.toHaveBeenCalled();
  });

  it('renders the title and source', () => {
    const { getByText } = render(SavedMobileRow, {
      props: { entry, feed, onUnsave: vi.fn(), onToggleRead: vi.fn() },
    });
    expect(getByText('A mobile saved entry')).toBeInTheDocument();
    expect(getByText('Julia Evans')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
pnpm --dir web test -- src/components/__tests__/SavedMobileRow.test.ts
```

Expected: FAIL with `Failed to resolve import "../SavedMobileRow.svelte"`.

- [ ] **Step 3: Write the minimal component**

Replace the contents of `web/src/components/SavedMobileRow.svelte` with:

```svelte
<script lang="ts">
  import FeedAvatar from './FeedAvatar.svelte';
  import { swipe } from '../lib/swipe';
  import type { EntryListItem, Subscription } from '../lib/types';

  type Props = {
    entry: EntryListItem;
    feed: Subscription | undefined;
    isFocused?: boolean;
    onFocus?: () => void;
    onMouseEnter?: () => void;
    onOpen?: () => void;
    onToggleRead?: () => void;
    onUnsave?: () => void;
  };
  let {
    entry, feed,
    isFocused = false,
    onFocus, onMouseEnter,
    onOpen, onToggleRead, onUnsave,
  }: Props = $props();

  function publishedLabel(ts: number): string {
    return new Date(ts * 1000).toLocaleDateString(undefined, {
      month: 'short', day: 'numeric', year: 'numeric',
    });
  }
</script>

<div
  class="row"
  class:is-read={entry.read}
  class:is-focused={isFocused}
  onmouseenter={() => onMouseEnter?.()}
  {@attach swipe({
    onSwipeLeft: () => onUnsave?.(),
    onSwipeRight: () => onToggleRead?.(),
  })}
>
  <div class="rev rev-left" aria-hidden="true">
    <span>{entry.read ? 'Mark unread' : 'Mark read'}</span>
  </div>
  <div class="rev rev-right" aria-hidden="true">
    <span>Unsave</span>
  </div>

  <button type="button" class="card" onclick={() => onOpen?.()} onfocus={() => onFocus?.()}>
    <div class="eyebrow">
      <span>saved</span>
      {#if entry.read}
        <span class="sep" aria-hidden="true">·</span>
        <span class="status-read">read</span>
      {/if}
    </div>
    <h3 class="title">{entry.title}</h3>
    <div class="byline">
      {#if feed}
        <FeedAvatar feedURL={feed.feed_url} size={10} radius={2} />
        <span class="source">{feed.title}</span>
      {/if}
      {#if entry.author}
        <span class="sep" aria-hidden="true">·</span>
        <span>{entry.author}</span>
      {/if}
    </div>
    <div class="foot">
      <span>published {publishedLabel(entry.published_at)}</span>
    </div>
  </button>
</div>

<style>
  .row {
    position: relative;
    overflow: hidden;
    border-bottom: 1px solid var(--rule);
    background: var(--bg);
  }
  .rev {
    position: absolute;
    top: 0; bottom: 0;
    display: flex; align-items: center; gap: 8px;
    padding: 0 22px;
    font-family: var(--sans); font-size: 13px; font-weight: 500;
    letter-spacing: 0.01em;
  }
  .rev-left {
    left: 0;
    background: var(--bg-soft);
    color: var(--ink-2);
    border-right: 1px solid var(--rule);
  }
  .rev-right {
    right: 0;
    background: var(--accent);
    color: #fff;
    justify-content: flex-end;
  }
  :global(.theme-dark) .rev-right { color: #0d0d0e; }

  .card {
    position: relative;
    background: var(--bg);
    padding: 14px 18px 16px;
    width: 100%;
    display: block;
    border: 0;
    text-align: left;
    cursor: pointer;
    z-index: 1;
    transition: transform 240ms cubic-bezier(.2,.7,.2,1);
  }
  .card:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -2px;
  }

  .eyebrow {
    display: flex; align-items: center; gap: 7px;
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.06em; text-transform: uppercase;
    color: var(--ink-3);
    margin-bottom: 6px;
  }
  .status-read { color: var(--ink-3); }
  .row.is-read .status-read { color: var(--accent); }

  .title {
    font-family: var(--serif); font-size: 17px; line-height: 1.3;
    font-weight: 500; color: var(--ink);
    margin: 0 0 6px;
    letter-spacing: -0.005em;
    text-wrap: pretty;
  }
  .row.is-read .title { color: var(--ink-2); font-weight: 400; }

  .byline {
    display: flex; align-items: center; gap: 7px;
    font-family: var(--sans); font-size: 12px; color: var(--ink-2);
    margin-bottom: 6px;
    flex-wrap: wrap;
  }
  .source { color: var(--ink); font-weight: 500; }
  .row.is-read .source { color: var(--ink-2); font-weight: 400; }

  .foot {
    display: flex; align-items: center; gap: 8px;
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.04em; text-transform: uppercase;
    color: var(--ink-3);
  }

  .sep::before {
    content: ""; display: inline-block;
    width: 3px; height: 3px; border-radius: 50%;
    background: var(--ink-4);
    vertical-align: middle;
  }
</style>
```

- [ ] **Step 4: Run the test to verify it passes**

```bash
pnpm --dir web test -- src/components/__tests__/SavedMobileRow.test.ts
```

Expected: PASS, all 4 tests.

The `touchEvent()` helper in this test file is **new** — it is the first TouchEvent attachment mock in the codebase. `web/src/lib/__tests__/swipe.test.ts` only exercises the pure `recogniseSwipe(dx, dy, startX)` function and does not synthesise TouchEvents. If happy-dom's TouchEvent semantics change in a future Vitest upgrade and the helper breaks, adjust the helper here — do not modify `web/src/lib/swipe.ts` (the recogniser is canonical and shipped by M-Redesign-1 / M8). If the helper proves load-bearing for future milestones, consider promoting it to `web/src/lib/__tests__/_touch.ts` as a shared test utility in a follow-up.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/SavedMobileRow.svelte web/src/components/__tests__/SavedMobileRow.test.ts
git commit -m "M-Redesign-3: SavedMobileRow with swipe-to-unsave + swipe-to-toggle-read"
```

---

## Phase B — View composition

### Task 4: Rewrite `Saved.svelte` desktop body with TDD

**Skills:** `superpowers:test-driven-development`, `svelte-runes`, `svelte-components`.

**Files:**
- Modify: `web/src/views/Saved.svelte`
- Create: `web/src/views/__tests__/Saved.test.ts`

- [ ] **Step 1: Write the failing test**

Replace the contents of `web/src/views/__tests__/Saved.test.ts` with:

```typescript
import { render, screen, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import Saved from '../Saved.svelte';
import { api } from '../../lib/api';

vi.mock('../../lib/api', () => ({
  api: {
    listEntries: vi.fn(),
    listSubscriptions: vi.fn(() => Promise.resolve([])),
    patchEntry: vi.fn(() => Promise.resolve()),
  },
}));

// Force the desktop branch so the view renders SavedRow (not SavedMobileRow).
vi.mock('../../lib/preferences.svelte', () => ({
  isMobile: { subscribe: (fn: (v: boolean) => void) => { fn(false); return () => {}; } },
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe('Saved view', () => {
  it('shows the empty state when no entries are saved', async () => {
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: [], next_cursor: null });
    render(Saved);
    await waitFor(() => {
      expect(screen.getByText(/nothing saved yet/i)).toBeInTheDocument();
    });
    expect(screen.getByText(/press/i).textContent).toMatch(/S/);
  });

  it('lists saved entries when the API returns data', async () => {
    vi.mocked(api.listEntries).mockResolvedValueOnce({
      data: [
        { id: 1, subscription_id: 3, title: 'Saved one',  url: 'a', author: '',
          published_at: 1700000000, fetched_at: 1700000000,
          read: false, saved: true, extract_failed: false },
        { id: 2, subscription_id: 3, title: 'Saved two',  url: 'b', author: '',
          published_at: 1700000100, fetched_at: 1700000100,
          read: true,  saved: true, extract_failed: false },
      ],
      next_cursor: null,
    });
    render(Saved);
    await waitFor(() => {
      expect(screen.getByText('Saved one')).toBeInTheDocument();
      expect(screen.getByText('Saved two')).toBeInTheDocument();
    });
  });

  it('passes saved=true to the API', async () => {
    vi.mocked(api.listEntries).mockResolvedValueOnce({ data: [], next_cursor: null });
    render(Saved);
    await waitFor(() => {
      expect(api.listEntries).toHaveBeenCalledWith(
        expect.objectContaining({ saved: true }),
      );
    });
  });

  it('shows an error state when the API rejects', async () => {
    vi.mocked(api.listEntries).mockRejectedValueOnce(new Error('boom'));
    render(Saved);
    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/boom/);
    });
  });

  it('renders the SavedToolbar with the correct count', async () => {
    vi.mocked(api.listEntries).mockResolvedValueOnce({
      data: Array.from({ length: 4 }).map((_, i) => ({
        id: i + 1, subscription_id: 3, title: `Title ${i}`,
        url: 'u', author: '', published_at: 1700000000 + i,
        fetched_at: 1700000000 + i,
        read: false, saved: true, extract_failed: false,
      })),
      next_cursor: null,
    });
    render(Saved);
    await waitFor(() => {
      expect(screen.getByText('4')).toBeInTheDocument();
      expect(screen.getByText(/entries/i)).toBeInTheDocument();
    });
  });

  it('optimistically unsaves an entry when the row Unsave action is clicked', async () => {
    vi.mocked(api.listEntries).mockResolvedValueOnce({
      data: [
        { id: 1, subscription_id: 3, title: 'Saved one',  url: 'a', author: '',
          published_at: 1700000000, fetched_at: 1700000000,
          read: false, saved: true, extract_failed: false },
      ],
      next_cursor: null,
    });
    vi.mocked(api.patchEntry).mockResolvedValueOnce(undefined as unknown as never);
    const { container } = render(Saved);
    await waitFor(() => expect(screen.getByText('Saved one')).toBeInTheDocument());

    const unsaveBtn = screen.getByRole('button', { name: /unsave/i });
    unsaveBtn.click();
    await waitFor(() => {
      expect(api.patchEntry).toHaveBeenCalledWith(1, { saved: false });
    });
    await waitFor(() => {
      expect(container.querySelector('.row')).toBeNull();
    });
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
pnpm --dir web test -- src/views/__tests__/Saved.test.ts
```

Expected: FAIL — the existing `Saved.svelte` still wires the old `Sidebar + TopBar + EntryRow` layout, does not show the count banner, and does not surface a row-level Unsave button.

- [ ] **Step 3: Rewrite `Saved.svelte`**

Replace the contents of `web/src/views/Saved.svelte` with:

```svelte
<script lang="ts">
  import { onMount, onDestroy, getContext } from 'svelte';
  import SavedToolbar from '../components/SavedToolbar.svelte';
  import SavedRow from '../components/SavedRow.svelte';
  import SavedMobileRow from '../components/SavedMobileRow.svelte';
  import EmptyState from '../components/EmptyState.svelte';
  import KbdChip from '../components/KbdChip.svelte';
  import { entries, subscriptions } from '../lib/store';
  import { navigate } from '../lib/router';
  import { isMobile } from '../lib/preferences.svelte';
  import type { Subscription } from '../lib/types';

  let focusedId = $state<number | null>(null);

  const items = $derived($entries.items.filter(e => e.saved));

  const dispatch = getContext<{
    onNext: () => void; onPrev: () => void; onOpen: () => void;
    onToggleRead: () => void; onToggleSaved: () => void;
  } | undefined>('keyDispatch');

  function feedFor(subId: number): Subscription | undefined {
    return $subscriptions.find(s => s.id === subId);
  }

  onMount(() => {
    entries.loadSaved();
    subscriptions.load();

    if (dispatch) {
      dispatch.onToggleSaved = () => {
        if (focusedId != null) entries.toggleSaved(focusedId, false);
      };
      dispatch.onToggleRead = () => {
        if (focusedId == null) return;
        const e = items.find(x => x.id === focusedId);
        if (e) entries.toggleRead(e.id, !e.read);
      };
      dispatch.onOpen = () => {
        if (focusedId != null) navigate(`/entry/${focusedId}`);
      };
    }
  });

  onDestroy(() => {
    if (dispatch) {
      dispatch.onToggleSaved = () => {};
      dispatch.onToggleRead = () => {};
      dispatch.onOpen = () => {};
    }
  });
</script>

{#snippet emptySubtitle()}
  Press <KbdChip>S</KbdChip> on any entry to keep it here.
{/snippet}

{#if $entries.loading}
  <p class="status" role="status">Loading…</p>
{:else if $entries.error}
  <p class="status err" role="alert">{$entries.error}</p>
{:else if items.length === 0}
  <EmptyState
    tone="accent"
    title="Nothing saved yet"
    subtitle={emptySubtitle}
  />
{:else}
  <SavedToolbar count={items.length} />
  <ul class="list" role="list" aria-label="Saved entries">
    {#each items as entry (entry.id)}
      <li>
        {#if $isMobile}
          <SavedMobileRow
            {entry}
            feed={feedFor(entry.subscription_id)}
            isFocused={focusedId === entry.id}
            onFocus={() => (focusedId = entry.id)}
            onMouseEnter={() => (focusedId = entry.id)}
            onOpen={() => navigate(`/entry/${entry.id}`)}
            onUnsave={() => entries.toggleSaved(entry.id, false)}
            onToggleRead={() => entries.toggleRead(entry.id, !entry.read)}
          />
        {:else}
          <SavedRow
            {entry}
            feed={feedFor(entry.subscription_id)}
            isFocused={focusedId === entry.id}
            onFocus={() => (focusedId = entry.id)}
            onMouseEnter={() => (focusedId = entry.id)}
            onOpen={() => navigate(`/entry/${entry.id}`)}
            onUnsave={() => entries.toggleSaved(entry.id, false)}
            onToggleRead={() => entries.toggleRead(entry.id, !entry.read)}
          />
        {/if}
      </li>
    {/each}
  </ul>
{/if}

<style>
  .status {
    padding: 24px 2px;
    color: var(--ink-3);
    font-family: var(--mono);
    font-size: 11px;
  }
  .status.err { color: #c43a3a; }

  .list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
  }
</style>
```

Notes on the state model (mirrors the Architecture paragraph at the top of this plan):

- **No view-local `items`.** The rendered list is `$derived($entries.items.filter(e => e.saved))`. The global store owns truth; the view owns rendering.
- **No view-local `loading` / `error`.** Both come from `$entries.loading` / `$entries.error` — `entries.loadSaved()` (Task 0) sets them.
- **Mutations go through the global store.** `entries.toggleSaved(id, false)` already does optimistic update + rollback (`web/src/lib/store.ts:49`). When it succeeds the entry's `saved` flag flips to `false`, the `$derived` predicate stops matching, and the row falls out of the rendered list naturally.
- **`focusedId` tracks the active row.** Set by either `onFocus` (keyboard tab) or `onMouseEnter` (mouse hover). The keyboard `S` handler (via `keyDispatch.onToggleSaved`) acts on the row whose id matches `focusedId`. Both `SavedRow` and `SavedMobileRow` accept `isFocused` / `onFocus` / `onMouseEnter` props (Task 2 step 3 / Task 3 step 3 must include them — see the row-component prop tables below in the Task 2 / Task 3 final-state note).

**Update to Task 2 row props (`SavedRow.svelte`):** add three optional props — `isFocused?: boolean` (defaults false; toggles `.is-focused` class for the focus ring), `onFocus?: () => void` (fires on `focus` of the row's outer `<article>` / `<button>`), `onMouseEnter?: () => void` (fires on mouse-enter). The existing `onOpen` / `onToggleRead` / `onUnsave` props stay.

**Update to Task 3 row props (`SavedMobileRow.svelte`):** add the same three props. On mobile `onMouseEnter` rarely fires; the swipe attachment is the primary input. Including it keeps the component-prop surface aligned with `SavedRow` so the view can pass the same handler object.

- [ ] **Step 4: Verify imports compile**

`web/src/lib/preferences.svelte.ts` exports `isMobile` as a `Readable<boolean>` — this is a hard precondition of M-Redesign-1 (see "Hard preconditions" at the top of this plan). If M1 has not yet shipped `isMobile` as a module-level `Readable<boolean>`, the M3 plan does not execute; coordinate with planner-m1 to fix M1 first. M3 must not ship its own media-query plumbing or a fallback path.

- [ ] **Step 5: Run the test to verify it passes**

```bash
pnpm --dir web test -- src/views/__tests__/Saved.test.ts
```

Expected: PASS, all 6 tests. Mobile-branch coverage lives in `SavedMobileRow.test.ts` (Task 3); the view-level test asserts the desktop branch only via the `vi.mock('../../lib/preferences.svelte', ...)` at the top of the file.

Note on state-isolation: the `entries` store is a singleton across tests. Each test's first action is `render(Saved)` which calls `entries.loadSaved()` synchronously inside `onMount`; the loadSaved method calls `set({ items: [], loading: true, error: null })` immediately, so the global store is implicitly reset at the start of every test. No explicit `entries.reset()` plumbing is needed. If a future test asserts on state *before* the first `loadSaved()` resolution, it will need to import `entries` and call a reset helper — that's out of scope for M3.

- [ ] **Step 6: Type-check the whole SPA**

```bash
pnpm --dir web run check
```

Expected: zero errors. If `$subscriptions` requires explicit `Writable<Subscription[]>` typing for inference, add it.

- [ ] **Step 7: Commit**

```bash
git add web/src/views/Saved.svelte web/src/views/__tests__/Saved.test.ts
git commit -m "M-Redesign-3: rewrite Saved view on the new .ts-shell"
```

---

### Task 5: Manually verify the desktop and mobile rendering

**Skills:** `superpowers:verification-before-completion`.

**Files:** none.

This task verifies the integrated view in the real shell. It is not a code task — its purpose is to catch issues the unit tests cannot see (CSS layering, the topbar handoff, font-family inheritance, `:focus-visible` behaviour, swipe transform animation).

- [ ] **Step 1: Rebuild the SPA and start the dev server**

```bash
make dev
```

Expected: Vite reports ready on `:5173`, Go reports listening on `:8080`.

- [ ] **Step 2: Sign in, save one entry from Unread, navigate to `/saved`**

Open `http://localhost:5173/`, sign in, press `S` on any unread entry to save it, then click the "Saved" tab in the top-tabs row (or navigate to `http://localhost:5173/saved`).

- [ ] **Step 3: Confirm the desktop visual checklist**

- The `.ts-shell` 720px column is centred; the page is on `var(--bg)`, not the old grey.
- The pinned count banner (`SavedToolbar`) shows "Saved" in serif 26px with the mono count beside it.
- Each row shows the bookmark glyph in the left rail, the publication date in a mono UPPER eyebrow, the serif title at 20px, the byline with feed avatar + source + author.
- Hovering a row reveals the action buttons (Open, Mark read, Unsave) in the action strip below the body.
- Read entries dim (title weight 400, colour `var(--ink-2)`, rail mark `var(--ink-4)`).
- Clicking the row body navigates to `/entry/:id`.
- Clicking Unsave on a row makes that row disappear without a refresh and the count decrements.
- Hovering a row, then pressing `S` on the keyboard: unsaves the hovered row (same as clicking Unsave). Tabbing to a row, then pressing `S`: same. Pressing `S` when no row is hovered/focused: no-op.
- After unsaving from Saved, navigating to Unread (M2) shows the entry in the unread list with `saved=false` (no stale `SAVED` mark) — verifies the global store is the SSOT.

- [ ] **Step 4: Confirm the empty-state visual checklist**

Unsave every saved entry, then refresh. Confirm:

- Accent dot at the top of the empty state.
- Serif 22px "Nothing saved yet" title.
- Sans 13px subtitle with the `kbd` chip wrapping the `S`.

- [ ] **Step 5: Confirm the mobile visual checklist**

Resize the browser to < 768px width (or DevTools "Responsive" → iPhone preset). Confirm:

- The `MobileTopBar` shows the wordmark + "Saved" + count.
- Each row uses the `.ts-saved-m-card` layout (eyebrow + title + byline + summary + foot).
- Touch-simulated swipe-left reveals the accent-blue "Unsave" reveal layer; release past threshold removes the row.
- Touch-simulated swipe-right reveals the soft-grey "Mark read" reveal layer; release past threshold toggles the row's read state.
- Mid-swipe the card does NOT visibly move via CSS class — the `swipe()` attachment only fires on `touchend`. (This is consistent with the existing `EntryRow.svelte` swipe wiring and is acceptable for M3. A future task can add a transform-as-you-drag affordance; the design's `.ts-saved-m-row.is-swipe-left .ts-saved-m-card { transform: translateX(-110px) }` rule is dead code in M3 and can be removed by M-Redesign-5 if it's not used elsewhere.)

- [ ] **Step 6: Confirm the three themes render correctly**

In the AccountMenu (top-right avatar) cycle Theme: Light → Dark → Sepia. Confirm:

- Hairlines remain visible (`var(--rule)` resolves correctly).
- The accent (Klein blue / desaturated in dark) is the only colour on the row mark and the empty-state dot.
- Mobile swipe reveal-right uses light text in light/sepia and dark text in dark (`:global(.theme-dark) .rev-right { color: #0d0d0e; }`).

- [ ] **Step 7: Confirm `pnpm run check` is clean**

```bash
pnpm --dir web run check
```

Expected: zero TypeScript / svelte-check errors.

- [ ] **Step 8: Stop the dev server**

Ctrl-C the `make dev` process.

There is no commit for this task — the verification is itself the deliverable.

---

## Phase C — Cleanup and PR prep

### Task 6: Add the M-Redesign-3 manual smoke checklist to the PR description template

**Skills:** none.

**Files:** none (PR description only).

When opening the PR, paste the following checklist into the PR body so reviewers can re-run it locally:

```markdown
## Manual smoke checklist (M-Redesign-3)

- [ ] /saved renders the SavedToolbar with the correct entry count.
- [ ] Empty state renders the accent dot + "Nothing saved yet" + the kbd hint.
- [ ] Hovering a desktop row reveals Open / Mark read / Unsave actions.
- [ ] Clicking Unsave optimistically removes the row.
- [ ] Clicking Mark read on a row dims it without removing it.
- [ ] Mobile: swipe-left fires Unsave; swipe-right fires Mark read/unread.
- [ ] Mobile: rows under threshold swipes do nothing.
- [ ] Read entries appear visually distinct (dimmed title, hollow rail mark).
- [ ] All three themes (light, dark, sepia) render without regressions.
- [ ] `pnpm --dir web run check` is clean.
- [ ] `make test` passes.
```

- [ ] **Step 1: Run the full test suite**

```bash
make test
```

Expected: `go test ./... -race` passes; `pnpm --dir web test` passes.

(Note: per `CLAUDE.md`'s build coupling section, `make test` rebuilds `web/dist` because `internal/server.SPAHandler` checks for `index.html` at construction time. If you're iterating quickly and have already rebuilt, `pnpm --dir web test` alone covers M3's frontend-only scope.)

- [ ] **Step 2: Run svelte-check one final time**

```bash
pnpm --dir web run check
```

Expected: zero errors.

- [ ] **Step 3: Commit any final fix-ups; push and open the PR**

```bash
git push -u origin <your-branch>
gh pr create --title "M-Redesign-3: Saved on the new .ts-shell" --body "..."
```

---

## Acceptance criteria

A reviewer should be able to verify, using **selectors and behaviour**, that:

1. **DOM structure**
   - `web/src/views/Saved.svelte` no longer imports `Sidebar.svelte` or `TopBar.svelte`. (These are slated for deletion in M-Redesign-1; M3 must not depend on them.)
   - When `items.length > 0`, the rendered DOM contains exactly one `SavedToolbar` element and an `<ul role="list" aria-label="Saved entries">` with one `<li>` per saved entry.
   - When `items.length === 0`, the rendered DOM contains the output of M1's `<EmptyState tone="accent" title="Nothing saved yet" subtitle={emptySubtitle} />` — i.e. an accent dot, a serif title reading "Nothing saved yet", and a subtitle containing "Press" + the `<KbdChip>S</KbdChip>` output + "on any entry to keep it here.". The exact selector shape (whether `EmptyState` renders `<section>`, `<div>`, etc.) is M1's choice; the test asserts via visible text only.
   - Each desktop row is an `<article class="row">` (or `<button>` semantically — the test asserts via accessible name).
   - Each mobile row is a `<div class="row">` wrapping a `<button class="card">` plus two `aria-hidden` reveal layers.
   - `Saved.svelte` does not declare a local `.kbd` class or inline `<kbd>` element — it consumes `<KbdChip>` from M1 inside the `emptySubtitle` snippet and inside `<SavedToolbar>`.

2. **State contract**
   - `Saved.svelte` calls `entries.loadSaved()` exactly once on mount; under the hood that calls `api.listEntries({ saved: true, limit: 100 })` exactly once.
   - Clicking the row-level Unsave button calls `entries.toggleSaved(id, false)`; under the hood that calls `api.patchEntry(id, { saved: false })`.
   - Clicking the row-level Mark read button calls `entries.toggleRead(id, !entry.read)`; under the hood that calls `api.patchEntry(id, { read: <new> })`.
   - All mutations happen optimistically through the global `entries` store — UI updates before the network resolves; on rejection, the store rolls back.
   - The rendered list is `$derived(() => $entries.items.filter(e => e.saved))`; no view-local items array.

3. **Keyboard contract**
   - When the `keyDispatch` context is present (M-Redesign-1 / M-Redesign-2 wire this), pressing `S` while the Saved view is mounted and a row is **focused** (via Tab key) or **hovered** (via mouse) fires `entries.toggleSaved(focusedId, false)`. The currently-acting row is tracked via the view's `focusedId` rune, set by either the row's `onFocus` callback or `onMouseEnter` callback.
   - When no row is focused/hovered, pressing `S` is a no-op.
   - When the `keyDispatch` context is absent (Saved view rendered outside the shell), `Saved.svelte` does not throw.

4. **Mobile contract**
   - Each mobile row is wired with `{@attach swipe({ onSwipeLeft, onSwipeRight })}`.
   - `onSwipeLeft` fires the unsave path. `onSwipeRight` fires the read-toggle path.
   - Sub-threshold swipes (per `recogniseSwipe` thresholds: < 40px travel, > 30° angle, or starting within 20px of the left edge) do nothing.

5. **Visual contract**
   - Every CSS rule originates from a scoped `<style>` block on the component that owns the selector. No edits to `web/src/styles/global.css` or `web/src/styles/tokens.css`.
   - `ui_design/styles.css` is unmodified.

6. **Test coverage**
   - 2 unit tests in `store.test.ts` for `entries.loadSaved()` (success + error paths).
   - 5 unit tests in `SavedToolbar.test.ts` (counts and labels).
   - 9 unit tests in `SavedRow.test.ts` (render, click handlers, action button stop-propagation, read variant, is-read class, onMouseEnter, onFocus, is-focused class).
   - 4 unit tests in `SavedMobileRow.test.ts` (swipe-left, swipe-right, sub-threshold, render).
   - 6 unit tests in `Saved.test.ts` (loading, list, error, empty, count, optimistic unsave).
   - All pass.
   - `pnpm --dir web run check` is clean.

---

## Verification commands

| What | Command | Expected |
|---|---|---|
| Unit tests (M3 only) | `pnpm --dir web test -- src/views/__tests__/Saved.test.ts src/components/__tests__/Saved*.test.ts src/lib/__tests__/store.test.ts -t loadSaved` | All 26 M3-introduced tests pass (5 SavedToolbar + 9 SavedRow + 4 SavedMobileRow + 6 Saved + 2 loadSaved) |
| Full SPA unit tests | `pnpm --dir web test` | All pass, no regressions in existing tests |
| TypeScript / svelte-check | `pnpm --dir web run check` | Zero errors |
| Full test suite (Go + SPA) | `make test` | All pass |
| Build the SPA | `pnpm --dir web build` | `web/dist/` populated, no warnings |
| Build the static binary | `make build` | `bin/tap` produced |

---

## Risks

1. **EntryRow / SavedRow ownership — resolved 2026-05-11 by team-lead, confirmed by planner-m2.** Decision: `EntryRow.svelte` is M1-owned (used by Unread, History, and the Reader rail — the *generic* simple-shell list views). `SavedRow.svelte` is M3-owned and view-specific because `.ts-saved-row` is a structural divergence from `.ts-entry` (grid-with-rail + 20px column for the bookmark glyph + third `grid-column: 1/-1` hover-revealed actions row + mono caps eyebrow), not a styling variant. The umbrella spec §3.2's "every view uses these" line stands for the generic list views; Saved is a documented exception alongside Categories (`.ts-cat`) and Feeds management (`.ts-feed-row`). No umbrella amendment strictly required, though team-lead is amending §3.2 anyway for clarity. No further coordination needed.
2. **Mobile swipe is "tap to fire", not "drag to reveal".** The current `web/src/lib/swipe.ts` is a `touchstart` → `touchend` recogniser; it does not emit a position-as-you-drag stream. The design's mid-swipe `.is-swipe-left` transform on `.ts-saved-m-card` is therefore inert and **deliberately omitted from the CSS port** (see the file-structure note on `SavedMobileRow.svelte`'s `<style>` block). This is consistent with the existing `EntryRow` swipe wiring; a follow-up that wants the drag-as-you-swipe affordance needs to extend `swipe.ts` to emit `touchmove`. Not in scope for M3.
3. **`saved_at` is not in the DB.** The JSX mockup's "saved Xd ago" eyebrow line is not implementable without a schema change. M3 ships "published <date>" instead. If a future iteration wants saved-at, it needs a new column on `entries` (or a side table) and a backend migration.
4. **Default `published_at` sort.** Backend returns entries newest by `published_at`. The user may expect "most recently saved first" instead. If usability testing surfaces this, add a `?sort=saved_at` query parameter to `GET /api/v1/entries` — but that requires the `saved_at` column above.
5. **`Saved.svelte` no longer mounts `Sidebar.svelte` or `TopBar.svelte`.** Until M-Redesign-1 has fully shipped (and `App.svelte` mounts the new `.ts-shell` chrome around every route), a developer running the Saved view in isolation will see an unstyled page. This is acceptable inside the M-Redesign sequence; it is NOT acceptable to ship M3 to `main` before M1 is merged.
6. **Existing tests for the old Saved view.** `web/src/views/__tests__/` does not currently contain a `Saved.test.ts` (verified against the worktree at plan-write time). If a test file appears there before M3 executes and it tests the deleted `Sidebar + TopBar + EntryRow` shape, the new test file replaces it; do not preserve the old assertions.

---

## Self-review

Spec coverage:

- **§5 row M3 ("flat chronological list of every entry where saved===true")** — Task 4 wires `api.listEntries({ saved: true })` and renders a flat list. No grouping.
- **§5 row M3 ("pinned count banner")** — `SavedToolbar` is rendered above the list in Task 4; tested in Task 1.
- **§5 row M3 ("empty state")** — Task 4 consumes M1's `<EmptyState tone="accent" title="Nothing saved yet" subtitle={emptySubtitle} />` and renders the kbd chip via `<KbdChip>S</KbdChip>` inside the snippet. The JSX's "hints panel" is dropped (not in Brand spec §6.3).
- **§5 row M3 (mobile swipe-to-unsave)** — `SavedMobileRow` ships swipe-left / swipe-right; tested in Task 3.
- **§6.3 ("flat chronological list of every entry where saved === true. No grouping by feed. Pinned banner at top showing the count.")** — Tasks 1 and 4 cover this.
- **§6.3 ("Press <kbd>S</kbd> on any entry to keep it here.")** — Task 4 step 3 renders this exact copy, with `S` wrapped in `<KbdChip>`.
- **§3.2 primitive consumption ("every view uses these")** — `Saved.svelte` consumes `EmptyState`, `KbdChip`, `FeedAvatar` from M1 directly. `SavedToolbar.svelte` consumes `KbdChip`. Per the 2026-05-11 decision (team-lead, confirmed by planner-m2), `EntryRow` is the primitive for the *generic* list views (Unread/History/Reader-rail); Saved is the documented exception alongside Categories and Feeds management. `SavedRow.svelte` + `SavedMobileRow.svelte` + `SavedToolbar.svelte` are M3-owned view-specific components, not entries in M1's primitive library.

Placeholder scan: none.

Type consistency:

- `EntryListItem` is the canonical entry type (`web/src/lib/types.ts:36`) — every component uses this name. The DTO has no `summary` field, so neither row primitive renders one; this matches Brand spec §6.3 which makes no mention of a row summary on Saved. All test fixtures include `fetched_at` and `extract_failed` to satisfy the full DTO shape.
- `Subscription` is the feed type (per `web/src/lib/types.ts`) — every component uses this name.
- `feedFor(subId)` is the lookup helper; named consistently across `Saved.svelte` and the existing `feedFor` already used in `Unread.svelte`.
- The handler set is `isFocused` / `onFocus` / `onMouseEnter` / `onOpen` / `onToggleRead` / `onUnsave` — consistent across `SavedRow`, `SavedMobileRow`, and `Saved.svelte`'s `<SavedRow>` / `<SavedMobileRow>` calls. `focusedId` (the view's tracking rune) is set by either `onFocus` or `onMouseEnter`.
- State methods: `entries.loadSaved()` (added in Task 0), `entries.toggleSaved(id, false)`, `entries.toggleRead(id, !read)` — consistent with the existing store surface. The rendered list comes from `$derived(() => $entries.items.filter(e => e.saved))`.

End of plan.
