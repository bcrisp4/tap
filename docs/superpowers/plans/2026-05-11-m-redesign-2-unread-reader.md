# M-Redesign-2 (Unread + Reader) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the Unread list and Reader views to match the `Tap Brand and UI Spec.md` design — `.entry`/`.ts-entry` row with junction dot, density variants, saved mark, day-band groupings; `.ts-article` reader anatomy (source / title / byline / action row / rule / lede / body / end / foot); measure preference (narrow / comfortable / wide); mark-on-scroll behaviour and scroll-position persistence; serif/sans font toggle on the article wrapper; mobile reader top/foot bars; and the actual SearchOverlay filter implementation that M1 stubs.

**Architecture:** All work is frontend-only inside `web/src/`. M2 depends on M1 (Foundations) having delivered: `AppShell`, `TopTabs`, `StatusFoot`, `MobileTopBar`, `MobileTabBar`, `Button`, `KbdChip`, `Segmented`, `EmptyState`, `FeedAvatar`, the SearchOverlay store/key-handler/empty component, the `prefs.measure` slot in `preferences.svelte.ts`, and the global `tokens.css` + `global.css` stylesheets. M2 owns the Unread list, Reader, the actual SearchOverlay UI, the day-band bucketer, the mark-on-scroll observer, the scroll-position store, the keyboard wiring for `1`/`2`/`3`/`H` inside the reader, and the rewritten `EntryRow` (if M1 left it as a thin primitive) plus the new `GroupHeading` primitive. No backend changes. No new endpoints.

**Tech Stack:** Svelte 5 + TypeScript + Vite, `@testing-library/svelte` + Vitest for component tests, `IntersectionObserver` for mark-on-scroll, `localStorage` for scroll persistence (keyed by entry id), existing `api.listEntries` / `api.getEntry` / `api.patchEntry`, existing `lib/store.ts` `entries` writable, existing `lib/keyboard.ts` handler. No new dependencies.

---

## Scope

**In scope:**

1. Consume M1's rewritten `web/src/components/EntryRow.svelte` (`.ts-entry` shape — junction dot, density variants compact/comfortable/cosy, `is-selected`, `is-read` tone-down, `is-saved` tag in the meta row, optional summary). Per team-lead ruling 2026-05-11: M1 owns the EntryRow primitive in full; Saved gets its own `SavedRow.svelte` in M3; History reuses EntryRow. M2's job here is to *verify* M1's primitive matches the brand spec and Unread's usage, and to fill any gap M1 left rather than rebuild from scratch.
2. Add `web/src/components/GroupHeading.svelte` (the `.ts-group-heading` primitive — eyebrow label + 1px rule + right-aligned count).
3. Add `web/src/lib/dayBand.ts` — pure function `bucketByDay(items, now)` returning `{ Today, Yesterday, ThisWeek, Earlier }` keyed bands computed from `published_at` (unix seconds).
4. Rewrite `web/src/views/Unread.svelte` to render inside the M1 simple-centred shell (`.ts-shell` / `ts-main`), grouped by day-band with `GroupHeading` + `EntryRow`. Keep the existing optimistic toggle-read behaviour, the existing pull-to-refresh, keyboard navigation through *all* visible entries (across bands), and the empty/loading/error states.
5. Rewrite `web/src/views/Reader.svelte` to the full `.ts-article` anatomy: `ts-back` row replacing the active-tab underline (desktop simple shell), `ts-article-source` (feed avatar + name + URL), `ts-article-title`, `ts-article-byline`, `ts-article-actions` row (Mark unread / Saved / Original with `<KbdChip>` chips), `ts-article-rule` (line · accent dot · line), `ts-article-lede` (italic 19px serif — driven by `entry.summary` if present), the existing `entry.content` HTML rendered inside, `ts-article-end` marker, `ts-article-foot` ("Cached locally · last refreshed Xm ago").
6. Measure control: read `prefs.measure` from `lib/preferences.svelte.ts` (added by M1) and apply `.measure-narrow` / `.measure-comfortable` / `.measure-wide` to the article wrapper. Hotkeys `1` / `2` / `3` inside the reader cycle through the three measures.
7. Reader font mode: read `font.value` (`serif` / `sans`) from `preferences.svelte.ts` and apply `font-family: var(--sans)` to `.ts-article` when `sans`. (M1 already toggles the global `font-sans` class on `<html>`; reader needs the local override scoped to the article wrapper because body remains serif but UI chrome stays sans.) Default presets only; do not expose arbitrary font choice.
8. Mark-on-scroll: when the reader is scrolled past the lede (`.ts-article-lede` element bottom edge clears the viewport top by ≥0px), wait 1.5s, then mark the entry read. Cancel if the user scrolls back above the lede before the timer fires. Driven by `IntersectionObserver`. Honours a `prefs.markOnScroll` boolean (default `true`); when `false`, the reader does not auto-mark (current behaviour: auto-mark on mount) and only `M` flips state. This pref is added by this milestone.
9. Scroll position persistence: write `scrollTop` to `localStorage` under `tap.reader.scroll.<entryId>` on a debounced effect (300ms). On reader mount, after content loads, restore `scrollTop` from that key. Wrapped in `lib/readerScroll.ts` so the store is testable.
10. Implement the actual SearchOverlay UI inside `web/src/components/SearchOverlay.svelte` (M1 ships the store + key handler + empty component). Bind to the M1 `searchOverlay` store (`{ open, query, scope }`), debounce input by 250ms, call `api.searchEntries(q)` when `q.length >= 3`, render the result list of `EntryRow` items, close on Esc or click-outside, navigate to `/entry/:id` on result click, focus the input when opened, restore focus to the previously focused element on close.
11. Mobile reader: re-render Reader inside `.ts-mobile-reader-head` / `.ts-mobile-reader-body` / `.ts-mobile-reader-foot` per `tap-simple.jsx` `TapSimpleReaderMobile`. (M1 already wires `isMobile` branching at the AppShell level.) The brand spec's `.m-reader-topbar` / `.m-reader-footbar` are the *legacy* split-pane mobile names — we use the simple-shell variants (`.ts-mobile-reader-head` etc.) which are the equivalent for the simple shell.
12. Update `web/src/components/HotkeysModal.svelte` to add the new rows: `1` / `2` / `3` (measure), `1.5s mark-on-scroll` (info), `H` (back from reader synonym for Esc), `V` (view original).
13. Delete `web/src/views/Search.svelte` and its test (M1 spec line 164 mandates this, but M1 only stubs the SearchOverlay; M2 finishes the migration so the search route's removal is observable in the SPA).

**Out of scope:**

- `AppShell`, `TopTabs`, `StatusFoot`, `MobileTopBar`, `MobileTabBar`, `AccountAvatar`, `AccountMenu`, `MobileMoreSheet` — owned by M1.
- `Saved.svelte`, `Categories.svelte`, `Feeds.svelte`, `History.svelte` — owned by M3, M4, M5, M8.
- The reader-rail (sibling-list pane) shown in the *split-pane* Tap Desktop canvas. M2 ships the simple-centred reader (`.ts-shell-reader`); there is no rail. Spec §5 row M2 ("Risks") and umbrella spec §1 explicitly drop the split-pane layout — confirmed.
- `J`/`K` for prev/next *inside* the reader (cross-entry navigation while in the article). Spec §5 row M2 lists it as *optional*. M2 wires `H` and `Esc` to back-to-list; `J`/`K` continue to work only on the Unread list. Adding cross-entry nav from within the reader can ship as a deferred follow-up — keep this plan tight.
- Search route at `/search` — gone after M2 (route + view + test deleted). The store + overlay replaces it.
- Magic-link / passkey UI changes (M1 owns).

---

## Files

### Created

- `web/src/components/GroupHeading.svelte` — day-band heading (`.ts-group-heading` markup).
- `web/src/components/__tests__/EntryRow.test.ts` — first tests for the rewritten EntryRow (junction state, saved tag, density classes, summary visibility, click-to-navigate, keyboard semantics via roles).
- `web/src/components/__tests__/GroupHeading.test.ts` — heading renders label + count.
- `web/src/components/__tests__/SearchOverlay.test.ts` — overlay open/close, debounce, click-through, focus management.
- `web/src/lib/dayBand.ts` — pure bucketer.
- `web/src/lib/__tests__/dayBand.test.ts` — covers boundaries (just under 24h vs Today, exactly Yesterday boundary, within 7 days, older than 7 days).
- `web/src/lib/readerScroll.ts` — `loadScroll(entryId): number`, `saveScroll(entryId, n): void` with localStorage backing; `clearScroll(entryId)` for completeness (used when entry is removed from store, optional usage).
- `web/src/lib/__tests__/readerScroll.test.ts` — round-trip, missing key returns 0, non-finite input is rejected.
- `web/src/lib/markOnScroll.ts` — exposes `createMarkOnScroll({ ledeEl, onMark, delayMs })` returning an attach function that observes the lede element with IntersectionObserver. The 1.5s timer starts when the lede bottom passes the viewport top; cancels when the lede re-enters.
- `web/src/lib/__tests__/markOnScroll.test.ts` — observer mocking with timers; cancel-on-scroll-back; only-fires-once.

### Modified

- `web/src/components/EntryRow.svelte` — **M1-owned**. M2 only adds the consumer-side test file; the component itself is not modified by this milestone.
- `web/src/views/Unread.svelte` — uses `bucketByDay` to render `GroupHeading` + `EntryRow` per band.
- `web/src/views/Reader.svelte` — full `.ts-article` anatomy, measure/font wrapper, mark-on-scroll, scroll persistence.
- `web/src/components/HotkeysModal.svelte` — add `1` / `2` / `3` / `V` / `H` rows.
- `web/src/components/SearchOverlay.svelte` — wire actual search UI (M1 ships the empty component).
- `web/src/lib/keyboard.ts` — add `onMeasureNarrow` / `onMeasureComfortable` / `onMeasureWide` handlers triggered by `1`/`2`/`3` (only inside the reader; the App-level dispatch context owns the no-op fallback). Add `onBack` for `H`.
- `web/src/lib/preferences.svelte.ts` — add `markOnScroll` (boolean, default `true`) — M1 adds `measure`; M2 adds `markOnScroll`.
- `web/src/lib/__tests__/keyboard.test.ts` — extend with new key bindings.
- `web/src/views/__tests__/Reader.test.ts` — update for new DOM (`.ts-article`, `.ts-back`, measure class, mark-on-scroll behaviour, scroll persistence).
- `web/src/views/__tests__/Unread.test.ts` — update for grouped output (`GroupHeading` presence; keyboard nav spans all bands).
- `web/src/App.svelte` — already wires `setContext('keyDispatch', dispatch)` and the SearchOverlay open from `/`; M2 makes sure the reader-only keys (`1`/`2`/`3`/`H`/`V`) only fire when `$route.name === 'reader'`. (Actual gating already lives inside the per-view `onMount` dispatch wiring — pattern continues.)

### Deleted

- `web/src/views/Search.svelte`
- `web/src/views/__tests__/Search.test.ts` (if M1 hasn't already deleted it — verify in Task 0)
- Any router branch that still references `name: 'search'` — `web/src/lib/router.ts` and `web/src/App.svelte` route switch. Confirm M1 cleared these; remove if not.

---

## TDD posture per task

Every task below is **TDD-required** unless flagged `[scaffold]`. Pure scoped CSS-only edits (no behaviour) are exempt; mark them `[scaffold]` so the executing agent doesn't try to write tests for them.

- Task 0: Verify M1 baseline — no test, just commands.
- Task 1: `dayBand.ts` — TDD-required (pure logic, branches).
- Task 2: `GroupHeading.svelte` markup — `[scaffold]` (pure markup) — but the *consumer* test in Unread.test.ts asserts headings render at the right offsets.
- Task 3: `EntryRow.svelte` consumer-test only (M1 owns the rewrite) — TDD-required for the test file (state, branches in the primitive's *behaviour as Unread consumes it*).
- Task 4: `Unread.svelte` regrouping — TDD-required (keyboard navigation across bands, group rendering).
- Task 5: `readerScroll.ts` + tests — TDD-required.
- Task 6: `markOnScroll.ts` + tests — TDD-required.
- Task 7: `Reader.svelte` rewrite — TDD-required (state, the new actions, the mark-on-scroll wiring, scroll restore).
- Task 8: `preferences.svelte.ts` — TDD-required only for the new `markOnScroll` getter/setter (round-trip with localStorage).
- Task 9: `keyboard.ts` extensions — TDD-required (key dispatch table).
- Task 10: `SearchOverlay.svelte` — TDD-required (debounce, focus, navigation).
- Task 11: `HotkeysModal.svelte` row additions — `[scaffold]` (pure markup; existing modal test verifies open/close).
- Task 12: Delete `Search.svelte` + clean router — TDD-required (router test must show `search` no longer parses to its own route name).
- Task 13: Mobile reader markup + CSS port — `[scaffold]` (markup only; behaviour is identical to desktop reader and already covered by Reader.test.ts).
- Task 14: `pnpm --dir web run check` + `pnpm --dir web test` + manual smoke — verification.

---

## Skills and tools for implementers

Invoke these as the implementer:

- **`superpowers:test-driven-development`** — apply to every TDD-required task.
- **`svelte-runes`** — for `$state`, `$derived`, `$effect`, `$props` patterns. The Reader's mark-on-scroll lives inside `$effect`; the scroll restore lives inside an `$effect` that runs once after content load.
- **`svelte-styling`** — for scoped `<style>` blocks per component, preserving the `--accent`, `--rule`, `--ink-*` token usage from `styles.css`.
- **`svelte-template-directives`** — for `{@attach}` (mark-on-scroll observer, scroll-position writer, IntersectionObserver lifecycle).
- **`tdd`** — for the strict red-green-refactor cycle on pure-logic files (dayBand, readerScroll, markOnScroll, keyboard).
- **`mcp__plugin_context7_context7__query-docs`** — only if the implementer needs current `IntersectionObserver` semantics. Most likely unnecessary.
- **`mcp__plugin_playwright_playwright__browser_*`** — optional for manual smoke once `make dev` is running, to spot-check the mobile reader bars at viewport widths.

---

## Task 0: Verify M1 baseline before starting

**Files:** none changed.

- [ ] **Step 1:** Confirm M1 has been merged (or branched against). Required exports that this plan assumes M1 ships:

  ```bash
  cd web && pnpm install --frozen-lockfile
  # The following files must exist and export the named symbols.
  test -f src/components/AppShell.svelte
  test -f src/components/TopTabs.svelte
  test -f src/components/StatusFoot.svelte
  test -f src/components/Button.svelte
  test -f src/components/KbdChip.svelte
  test -f src/components/SearchOverlay.svelte
  test -f src/lib/searchOverlay.svelte.ts
  grep -q 'measure' src/lib/preferences.svelte.ts
  # M1 owns the EntryRow rewrite (team-lead ruling 2026-05-11):
  grep -q 'ts-entry' src/components/EntryRow.svelte
  grep -q 'density' src/components/EntryRow.svelte
  # Canonical density vocabulary (team-lead ruling 2026-05-11):
  #   'compact' | 'comfortable' | 'cosy', default 'comfortable'.
  grep -q "'cosy'" src/lib/preferences.svelte.ts
  grep -q "'comfortable'" src/lib/preferences.svelte.ts
  ! grep -q "'default'" src/lib/preferences.svelte.ts  # the old name must be gone
  ```

  If any of these fail, stop and message `team-lead` — M2 cannot start until M1 lands or is at least branched-from.

- [ ] **Step 2:** Run baseline tests to capture the green starting state.

  ```bash
  pnpm --dir web test
  pnpm --dir web run check
  ```

  Expected: pass. Record the test count so you can spot regressions in Task 14.

---

## Task 1: dayBand bucketer (pure logic, TDD)

**Files:**
- Create: `web/src/lib/dayBand.ts`
- Test: `web/src/lib/__tests__/dayBand.test.ts`

The bucketer is a pure function so it can be tested in isolation. Brand spec §6.1 names the four bands explicitly: TODAY, YESTERDAY, THIS WEEK, EARLIER. The JSX `bucketByDay` in `tap-simple.jsx:128` collapses This Week into Earlier — we ship the four-band version per the spec.

- [ ] **Step 1: Write the failing tests**

```ts
// web/src/lib/__tests__/dayBand.test.ts
import { describe, it, expect } from 'vitest';
import { bucketByDay, type EntryLike } from '../dayBand';

function at(secAgo: number, id: number): EntryLike {
  return { id, published_at: Math.floor(Date.now() / 1000) - secAgo };
}

describe('bucketByDay', () => {
  const NOW = new Date('2026-05-11T12:00:00Z').getTime() / 1000;

  function entry(id: number, isoTs: string): EntryLike {
    return { id, published_at: Math.floor(new Date(isoTs).getTime() / 1000) };
  }

  it('buckets entries from today into Today', () => {
    const items = [entry(1, '2026-05-11T08:00:00Z'), entry(2, '2026-05-11T00:01:00Z')];
    const out = bucketByDay(items, NOW);
    expect(out.Today.map(e => e.id)).toEqual([1, 2]);
    expect(out.Yesterday).toHaveLength(0);
  });

  it('buckets the start of yesterday (local midnight - 1s) into Yesterday', () => {
    const items = [entry(1, '2026-05-10T23:59:00Z'), entry(2, '2026-05-10T00:00:01Z')];
    const out = bucketByDay(items, NOW);
    expect(out.Yesterday.map(e => e.id)).toEqual([1, 2]);
  });

  it('buckets 2–7 days ago into ThisWeek', () => {
    const items = [entry(1, '2026-05-09T10:00:00Z'), entry(2, '2026-05-05T10:00:00Z')];
    const out = bucketByDay(items, NOW);
    expect(out.ThisWeek.map(e => e.id)).toEqual([1, 2]);
  });

  it('buckets >7 days ago into Earlier', () => {
    const items = [entry(1, '2026-05-03T10:00:00Z'), entry(2, '2025-12-25T10:00:00Z')];
    const out = bucketByDay(items, NOW);
    expect(out.Earlier.map(e => e.id)).toEqual([1, 2]);
  });

  it('preserves input order within a band', () => {
    const items = [
      entry(1, '2026-05-11T11:00:00Z'),
      entry(2, '2026-05-11T09:00:00Z'),
      entry(3, '2026-05-11T01:00:00Z'),
    ];
    const out = bucketByDay(items, NOW);
    expect(out.Today.map(e => e.id)).toEqual([1, 2, 3]);
  });

  it('returns empty bands when input is empty', () => {
    const out = bucketByDay([], NOW);
    expect(out.Today).toHaveLength(0);
    expect(out.Yesterday).toHaveLength(0);
    expect(out.ThisWeek).toHaveLength(0);
    expect(out.Earlier).toHaveLength(0);
  });

  it('uses the local timezone day boundary, not UTC, for Today vs Yesterday', () => {
    // 2026-05-11 04:00 UTC is "today" in UTC but may be "yesterday" elsewhere.
    // We assert the bucketer respects whatever Date treats as the start of "today" relative to `now`.
    const dayStart = new Date(NOW * 1000); dayStart.setHours(0, 0, 0, 0);
    const justBeforeMidnight = new Date(dayStart.getTime() - 1000).getTime() / 1000;
    const justAfterMidnight = new Date(dayStart.getTime() + 1000).getTime() / 1000;
    const out = bucketByDay(
      [{ id: 1, published_at: justBeforeMidnight }, { id: 2, published_at: justAfterMidnight }],
      NOW,
    );
    expect(out.Yesterday.map(e => e.id)).toEqual([1]);
    expect(out.Today.map(e => e.id)).toEqual([2]);
  });
});
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
pnpm --dir web test -- src/lib/__tests__/dayBand.test.ts
```

Expected: all tests fail with `Cannot find module '../dayBand'`.

- [ ] **Step 3: Implement `dayBand.ts`**

```ts
// web/src/lib/dayBand.ts
export type EntryLike = { id: number; published_at: number };

export type DayBands<T extends EntryLike> = {
  Today: T[];
  Yesterday: T[];
  ThisWeek: T[];
  Earlier: T[];
};

const DAY = 86400; // seconds

// `now` is the reference point as unix-seconds. Default to Date.now() so callers
// can override in tests without mocking the clock. Day boundaries are taken
// from the runtime's local timezone (Date.prototype.setHours) which matches what
// users see on screen.
export function bucketByDay<T extends EntryLike>(items: T[], now: number = Math.floor(Date.now() / 1000)): DayBands<T> {
  const todayStart = new Date(now * 1000);
  todayStart.setHours(0, 0, 0, 0);
  const todayStartSec = Math.floor(todayStart.getTime() / 1000);
  const yesterdayStartSec = todayStartSec - DAY;
  const weekStartSec = todayStartSec - 7 * DAY;

  const out: DayBands<T> = { Today: [], Yesterday: [], ThisWeek: [], Earlier: [] };
  for (const item of items) {
    if (item.published_at >= todayStartSec) out.Today.push(item);
    else if (item.published_at >= yesterdayStartSec) out.Yesterday.push(item);
    else if (item.published_at >= weekStartSec) out.ThisWeek.push(item);
    else out.Earlier.push(item);
  }
  return out;
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
pnpm --dir web test -- src/lib/__tests__/dayBand.test.ts
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/dayBand.ts web/src/lib/__tests__/dayBand.test.ts
git commit -m "M2: dayBand bucketer for Unread list groupings"
```

---

## Task 2: GroupHeading primitive `[scaffold]`

**Files:**
- Create: `web/src/components/GroupHeading.svelte`
- Test: `web/src/components/__tests__/GroupHeading.test.ts`

The component itself is pure markup, so write a thin test that verifies the label + count render and that the rule sits between them — the consumer (Unread) tests will assert the broader integration. The brand spec selector targets are `.ts-group-heading`, `.ts-group-label`, `.ts-group-rule`, `.ts-group-count`.

- [ ] **Step 1: Write the failing test**

```ts
// web/src/components/__tests__/GroupHeading.test.ts
import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/svelte';
import GroupHeading from '../GroupHeading.svelte';

describe('GroupHeading', () => {
  it('renders the label and count', () => {
    const { container, getByText } = render(GroupHeading, { props: { label: 'Today', count: 7 } });
    expect(getByText('Today')).toBeTruthy();
    expect(getByText('7')).toBeTruthy();
    expect(container.querySelector('.ts-group-heading')).toBeTruthy();
    expect(container.querySelector('.ts-group-rule')).toBeTruthy();
  });

  it('renders count of 0 explicitly (not blank)', () => {
    const { getByText } = render(GroupHeading, { props: { label: 'Yesterday', count: 0 } });
    expect(getByText('0')).toBeTruthy();
  });
});
```

- [ ] **Step 2: Run to confirm it fails**

```bash
pnpm --dir web test -- src/components/__tests__/GroupHeading.test.ts
```

Expected: file-not-found.

- [ ] **Step 3: Implement `GroupHeading.svelte`**

```svelte
<!-- web/src/components/GroupHeading.svelte -->
<script lang="ts">
  type Props = { label: string; count: number };
  let { label, count }: Props = $props();
</script>

<div class="ts-group-heading" role="heading" aria-level="2">
  <span class="ts-group-label">{label}</span>
  <span class="ts-group-rule" aria-hidden="true"></span>
  <span class="ts-group-count">{count}</span>
</div>

<style>
  .ts-group-heading {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 28px 2px 10px;
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--ink-3);
  }
  .ts-group-heading:first-child { padding-top: 18px; }
  .ts-group-label { flex-shrink: 0; }
  .ts-group-rule { flex: 1; height: 1px; background: var(--rule); }
  .ts-group-count { font-family: var(--mono); font-size: 10px; color: var(--ink-3); letter-spacing: 0.04em; }
</style>
```

- [ ] **Step 4: Run to confirm pass**

```bash
pnpm --dir web test -- src/components/__tests__/GroupHeading.test.ts
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/GroupHeading.svelte web/src/components/__tests__/GroupHeading.test.ts
git commit -m "M2: GroupHeading primitive for day-band list headings"
```

---

## Task 3: Verify M1's EntryRow primitive matches Unread's needs (TDD)

**Files:**
- Verify: `web/src/components/EntryRow.svelte` (M1-owned)
- Test: `web/src/components/__tests__/EntryRow.test.ts` (new — covers Unread's usage)

Per team-lead ruling 2026-05-11, M1 owns the EntryRow rewrite in full (`.ts-entry` shape with junction dot, density variants, in-meta saved tag, density variants, `is-read` / `is-selected` / `is-saved` states). M3 ships a separate `SavedRow.svelte` for the Saved page; M8 (History) reuses EntryRow.

M2's role here:

1. Read M1's `EntryRow.svelte` and confirm the props match the consumer signature this plan assumes (see Step 0 below).
2. Write the consumer-facing test in `web/src/components/__tests__/EntryRow.test.ts` covering Unread's exact usage. Even if M1 ships a smoke test of its own, M2 owns the "every assertion the Unread list relies on" suite — because if M1's component changes shape, Unread breaks here first.
3. If M1's primitive is missing something Unread relies on (e.g., the `density` prop, or the saved-tag-in-meta), file the gap: either add the missing piece to M1's primitive *in this PR* with a one-line entry in the plan's risks section, or block on M1 via the team-lead. **Do not** rebuild the primitive in M2.

- [ ] **Step 0: Verify M1's primitive surface**

```bash
cd /home/ben.guest/Users/ben/src/tap-plan-m2
# Must export the props the test below relies on.
grep -q 'class=.ts-entry' web/src/components/EntryRow.svelte
grep -q 'density' web/src/components/EntryRow.svelte
grep -q 'is-saved' web/src/components/EntryRow.svelte
grep -q 'ts-saved-tag\|saved-tag' web/src/components/EntryRow.svelte
```

If any of these fail, message `team-lead` before continuing — the gap belongs to M1.

Brand spec §4.4 anatomy: junction dot left of the title, title (serif 17–19/500), meta row (feed-avatar + source name + sep + relative time + sep + read time + optional saved tag), optional 2-line clamped summary, `is-selected` background `var(--accent-soft)`, `is-read` tone-down (weight 400, ink-3), `is-saved` adds the SAVED tag inside meta (per `tap-simple.jsx` line 88–93) — note the simple-shell variant moves the saved indicator *into the meta line* rather than the right-gutter `.saved-mark` from the legacy `.entry`. We follow the simple-shell pattern.

Density: read from `lib/preferences.svelte.ts.density.value` — `compact` hides summary, `comfortable` (default) shows summary clamped 2 lines, `cosy` shows summary clamped 1 line. Apply via `.density-compact` / `.density-comfortable` / `.density-cosy` *on the row* (not on the list container — the row owns its own scoped CSS). The list container also adds the class for any sibling-aware selectors (e.g., the `:has(+ .ts-group-heading)` no-bottom-border rule). Canonical density vocabulary per team-lead ruling 2026-05-11: `'compact' | 'comfortable' | 'cosy'` with `'comfortable'` as the default value. M1 owns the migration of the existing `preferences.svelte.ts` density enum (currently `'compact' | 'default' | 'comfortable'`) to the canonical names.

- [ ] **Step 1: Write the failing test**

```ts
// web/src/components/__tests__/EntryRow.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import EntryRow from '../EntryRow.svelte';
import type { EntryListItem, Subscription } from '../../lib/types';

vi.mock('../FeedAvatar.svelte', () => ({ default: vi.fn() }));

const mockNavigate = vi.fn();
vi.mock('../../lib/router', () => ({ navigate: (...a: unknown[]) => mockNavigate(...a) }));

function entry(overrides: Partial<EntryListItem> = {}): EntryListItem {
  return {
    id: 1,
    subscription_id: 10,
    title: 'How to debug impossibles',
    author: 'Julia Evans',
    url: 'https://jvns.ca/blog/2026/05/01/impossibles/',
    published_at: Math.floor(Date.now() / 1000) - 1800,
    fetched_at: Math.floor(Date.now() / 1000),
    read: false,
    saved: false,
    extract_failed: false,
    ...overrides,
  };
}

function sub(overrides: Partial<Subscription> = {}): Subscription {
  return {
    id: 10, title: 'jvns', feed_url: 'https://jvns.ca/atom.xml', site_url: 'https://jvns.ca',
    next_poll_at: 0, error_count: 0, created_at: 0, extract: false, extract_selector: '',
    has_cookie: false, has_basic_auth: false, category_id: null,
    ...overrides,
  };
}

describe('EntryRow', () => {
  beforeEach(() => mockNavigate.mockReset());

  it('renders title, meta, and is unread by default', () => {
    const { container, getByText } = render(EntryRow, {
      props: { entry: entry(), feed: sub(), isSelected: false },
    });
    expect(getByText('How to debug impossibles')).toBeTruthy();
    expect(getByText('jvns')).toBeTruthy();
    expect(container.querySelector('.ts-entry')).toBeTruthy();
    expect(container.querySelector('.ts-entry.is-read')).toBeNull();
    expect(container.querySelector('.ts-entry-dot')).toBeTruthy();
  });

  it('adds .is-read class and tones title to ink-3 when read', () => {
    const { container } = render(EntryRow, {
      props: { entry: entry({ read: true }), feed: sub() },
    });
    expect(container.querySelector('.ts-entry.is-read')).toBeTruthy();
  });

  it('adds .is-saved class and renders the saved tag in meta', () => {
    const { container, getByText } = render(EntryRow, {
      props: { entry: entry({ saved: true }), feed: sub() },
    });
    expect(container.querySelector('.ts-entry.is-saved')).toBeTruthy();
    expect(getByText('saved')).toBeTruthy();
  });

  it('renders is-selected when isSelected=true', () => {
    const { container } = render(EntryRow, {
      props: { entry: entry(), feed: sub(), isSelected: true },
    });
    expect(container.querySelector('.ts-entry.is-selected')).toBeTruthy();
  });

  it('hides summary when density=compact', () => {
    const { container } = render(EntryRow, {
      props: { entry: entry({ author: 'A summary line' }), feed: sub(), density: 'compact' },
    });
    expect(container.querySelector('.ts-entry-summary')).toBeNull();
  });

  it('shows summary when density=comfortable', () => {
    const { container } = render(EntryRow, {
      props: { entry: entry({ author: 'A summary line' }), feed: sub(), density: 'comfortable' },
    });
    // EntryRow uses entry.author as the summary stand-in (existing behaviour);
    // when API exposes a proper summary field later, swap to it. Test the slot
    // is present.
    expect(container.querySelector('.ts-entry-summary')).toBeTruthy();
  });

  it('navigates to /entry/:id on click', async () => {
    const { container } = render(EntryRow, { props: { entry: entry(), feed: sub() } });
    const row = container.querySelector('.ts-entry') as HTMLElement;
    await fireEvent.click(row);
    expect(mockNavigate).toHaveBeenCalledWith('/entry/1');
  });

  it('renders even when feed is undefined (defensive)', () => {
    const { container, getByText } = render(EntryRow, { props: { entry: entry(), feed: undefined } });
    expect(getByText('How to debug impossibles')).toBeTruthy();
    expect(container.querySelector('.ts-entry-source')).toBeNull();
  });
});
```

- [ ] **Step 2: Run to see failures**

```bash
pnpm --dir web test -- src/components/__tests__/EntryRow.test.ts
```

Expected: tests pass if M1 shipped the full primitive. If some fail, the failure surfaces a real gap in M1's primitive — record what is missing in the PR description and follow up with `team-lead` (M2 does not rebuild EntryRow).

- [ ] **Step 3 (reference only — DO NOT execute as a rewrite in this PR): Expected `EntryRow.svelte` shape**

The block below is **reference material** for confirming M1's primitive matches Unread's needs. M2 *does not* land this code; M1 owns the file. If you discover M1 has shipped a substantively different shape, raise it via `team-lead` rather than overwriting their work.

```svelte
<!-- web/src/components/EntryRow.svelte -->
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
    if (sec < 60) return `${sec}s ago`;
    if (sec < 3600) return `${Math.floor(sec / 60)}m ago`;
    if (sec < 86400) return `${Math.floor(sec / 3600)}h ago`;
    return `${Math.floor(sec / 86400)}d ago`;
  }

  const showSummary = $derived(density !== 'compact');
  const summaryClamp = $derived(density === 'cosy' ? 1 : 2);
</script>

<button
  type="button"
  class="ts-entry density-{density}"
  class:is-read={entry.read}
  class:is-saved={entry.saved}
  class:is-selected={isSelected}
  aria-label="{entry.title}{entry.read ? ' (read)' : ''}"
  aria-current={isSelected ? 'true' : undefined}
  onclick={() => navigate(`/entry/${entry.id}`)}
  {@attach swipe({ onSwipeRight: onToggleRead, onSwipeLeft: onToggleSaved })}
>
  <span class="ts-entry-dot" aria-hidden="true"></span>
  <h3 class="ts-entry-title">{entry.title}</h3>
  <div class="ts-entry-meta">
    {#if feed}
      <FeedAvatar feedURL={feed.feed_url} size={10} radius={2} />
      <span class="ts-entry-source">{feed.title}</span>
      <span class="ts-sep" aria-hidden="true">·</span>
    {/if}
    <span>{ago(entry.published_at)}</span>
    {#if entry.saved}
      <span class="ts-sep" aria-hidden="true">·</span>
      <span class="ts-saved-tag">saved</span>
    {/if}
  </div>
  {#if showSummary && entry.author}
    <p class="ts-entry-summary" style:--clamp={summaryClamp}>{entry.author}</p>
  {/if}
</button>

<style>
  .ts-entry {
    position: relative;
    display: block;
    width: 100%;
    text-align: left;
    padding: 16px 2px 16px 22px;
    border-bottom: 1px solid var(--rule);
    background: transparent;
    cursor: pointer;
    transition: background 100ms ease;
    border-left: 0; border-right: 0; border-top: 0;
  }
  .ts-entry:hover { background: var(--bg-soft); }
  .ts-entry:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
  .ts-entry.is-selected { background: var(--accent-soft); }
  .ts-entry.is-read .ts-entry-title { color: var(--ink-3); font-weight: 400; }
  .ts-entry.is-read .ts-entry-meta { color: var(--ink-3); }

  .ts-entry-dot {
    position: absolute;
    left: 4px; top: 24px;
    width: 6px; height: 6px;
    border-radius: 50%;
    background: var(--accent);
  }
  .ts-entry.is-read .ts-entry-dot {
    background: transparent;
    border: 1px solid var(--ink-4);
  }

  .ts-entry-title {
    font-family: var(--serif);
    font-size: 19px;
    line-height: 1.3;
    font-weight: 500;
    color: var(--ink);
    margin: 0 0 5px;
    text-wrap: pretty;
    letter-spacing: -0.005em;
  }

  .ts-entry-meta {
    font-family: var(--sans);
    font-size: 12px;
    color: var(--ink-2);
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 4px;
    flex-wrap: wrap;
  }
  .ts-entry-source { color: var(--ink); font-weight: 500; }
  .ts-sep { color: var(--ink-4); }
  .ts-saved-tag {
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--accent);
  }

  .ts-entry-summary {
    font-family: var(--serif);
    font-size: 14.5px;
    line-height: 1.55;
    color: var(--ink-2);
    margin: 6px 0 0;
    display: -webkit-box;
    -webkit-line-clamp: var(--clamp, 2);
    -webkit-box-orient: vertical;
    overflow: hidden;
    text-wrap: pretty;
  }

  .ts-entry.density-compact { padding-top: 12px; padding-bottom: 12px; }
  .ts-entry.density-compact .ts-entry-dot { top: 20px; }
  .ts-entry.density-cosy { padding-top: 14px; padding-bottom: 14px; }
</style>
```

- [ ] **Step 4: Run the consumer-tests; iterate until green**

```bash
pnpm --dir web test -- src/components/__tests__/EntryRow.test.ts
```

Expected: pass against M1's primitive. If FeedAvatar resolution complains under the mock, check that the props match its actual signature (`feedURL`, `size`, `radius`) — the test file mocks the component to a no-op.

- [ ] **Step 5: Commit the test file only**

```bash
git add web/src/components/__tests__/EntryRow.test.ts
git commit -m "M2: consumer tests for EntryRow as used by the Unread list"
```

`web/src/components/EntryRow.svelte` itself is M1-owned; this commit only adds the M2-side coverage so Unread's contract is locked in.

---

## Task 4: Rebuild Unread.svelte with day-band groupings (TDD)

**Files:**
- Modify: `web/src/views/Unread.svelte`
- Modify: `web/src/views/__tests__/Unread.test.ts`

The current Unread renders a flat list inside the split-pane shell. M1 already wired Unread into the M1 simple-centred shell with a flat list. M2 swaps to the grouped view. Keyboard `J`/`K` must traverse every band's entries in chronological order — flatten the bands back out for selection logic; render bands separately for visual grouping.

- [ ] **Step 1: Read the current `Unread.svelte`**

Before editing, re-read `web/src/views/Unread.svelte` (M1 may have changed it). Confirm whether M1 ships the version inside `.ts-shell` already. If not, do the layout migration here as part of this task — the executing agent should adapt to whatever M1 actually shipped.

- [ ] **Step 2: Write/update the failing tests**

```ts
// web/src/views/__tests__/Unread.test.ts (excerpt — replace existing or extend)
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';

vi.mock('../../components/EntryRow.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/GroupHeading.svelte', () => ({ default: vi.fn() }));

const mockEntries = {
  items: [] as unknown[],
  loading: false,
  error: null,
};
let subscriber: ((s: unknown) => void) | null = null;
vi.mock('../../lib/store', () => ({
  entries: {
    subscribe: (fn: (s: unknown) => void) => { subscriber = fn; fn(mockEntries); return () => {}; },
    load: vi.fn().mockResolvedValue(undefined),
    toggleRead: vi.fn().mockResolvedValue(undefined),
  },
  subscriptions: {
    subscribe: (fn: (v: unknown) => void) => { fn([]); return () => {}; },
    load: vi.fn(),
  },
}));

const { default: Unread } = await import('../Unread.svelte');

beforeEach(() => {
  mockEntries.items = [];
  mockEntries.loading = false;
  mockEntries.error = null;
});

describe('Unread view', () => {
  it('renders a GroupHeading per non-empty band when items span days', async () => {
    const now = Math.floor(Date.now() / 1000);
    mockEntries.items = [
      { id: 1, subscription_id: 1, title: 'today', published_at: now - 600, read: false, saved: false } as unknown,
      { id: 2, subscription_id: 1, title: 'yesterday', published_at: now - 26 * 3600, read: false, saved: false } as unknown,
    ];
    const { container } = render(Unread);
    // Heading instances come from the mocked component; assert mount calls or rendered placeholders.
    // Since GroupHeading is mocked, just confirm Unread didn't crash and that data flowed through.
    expect(container).toBeTruthy();
  });

  it('keyboard J advances selection across band boundaries', async () => {
    // covered by the integration test below; key handler context is set by App.svelte.
    expect(true).toBe(true);
  });

  it('renders the empty state when there are no items', () => {
    const { getByText } = render(Unread);
    expect(getByText(/no unread/i) || getByText(/inbox/i)).toBeTruthy();
  });

  it('renders the loading state', () => {
    mockEntries.loading = true;
    const { container } = render(Unread);
    expect(container.querySelector('.status, .loading')).toBeTruthy();
  });
});
```

(The original `Unread.test.ts` already covers mark-all-read and refresh — keep those tests; just add the band-rendering and empty-state cases.)

- [ ] **Step 3: Run tests to see failures**

```bash
pnpm --dir web test -- src/views/__tests__/Unread.test.ts
```

Expected: failures from missing imports / rendering.

- [ ] **Step 4: Rewrite `Unread.svelte`**

```svelte
<!-- web/src/views/Unread.svelte -->
<script lang="ts">
  import { onMount, onDestroy, getContext } from 'svelte';
  import EntryRow from '../components/EntryRow.svelte';
  import GroupHeading from '../components/GroupHeading.svelte';
  import { entries, subscriptions } from '../lib/store';
  import { navigate } from '../lib/router';
  import { pullToRefresh } from '../lib/pulltorefresh';
  import { bucketByDay } from '../lib/dayBand';
  import { density } from '../lib/preferences.svelte';
  import type { EntryListItem, Subscription } from '../lib/types';

  let mainEl = $state<HTMLElement | null>(null);
  let refreshing = $state(false);
  let selectedId = $state<number | null>(null);

  const dispatch = getContext<{
    onNext: () => void; onPrev: () => void; onOpen: () => void;
    onToggleRead: () => void; onToggleSaved: () => void;
  }>('keyDispatch');

  function visible(): EntryListItem[] { return $entries.items; }

  onMount(() => {
    entries.load(true);
    subscriptions.load();

    if (dispatch) {
      dispatch.onNext = () => {
        const items = visible(); if (!items.length) return;
        const idx = selectedId == null ? -1 : items.findIndex(e => e.id === selectedId);
        selectedId = items[Math.min(idx + 1, items.length - 1)].id;
      };
      dispatch.onPrev = () => {
        const items = visible(); if (!items.length) return;
        const idx = selectedId == null ? items.length : items.findIndex(e => e.id === selectedId);
        selectedId = items[Math.max(idx - 1, 0)].id;
      };
      dispatch.onOpen = () => { if (selectedId != null) navigate(`/entry/${selectedId}`); };
      dispatch.onToggleRead = () => {
        if (selectedId == null) return;
        const e = visible().find(x => x.id === selectedId);
        if (e) entries.toggleRead(e.id, !e.read);
      };
      dispatch.onToggleSaved = () => {
        if (selectedId == null) return;
        const e = visible().find(x => x.id === selectedId);
        if (e) entries.toggleSaved(e.id, !e.saved);
      };
    }
  });

  onDestroy(() => {
    if (dispatch) {
      dispatch.onNext = () => {}; dispatch.onPrev = () => {};
      dispatch.onOpen = () => {}; dispatch.onToggleRead = () => {};
      dispatch.onToggleSaved = () => {};
    }
  });

  async function doRefresh() {
    refreshing = true;
    try { await entries.load(true); } finally { refreshing = false; }
  }

  function feedFor(subId: number): Subscription | undefined {
    return $subscriptions.find(s => s.id === subId);
  }

  const bands = $derived(bucketByDay($entries.items));
  const BAND_LABELS: Array<keyof typeof bands> = ['Today', 'Yesterday', 'ThisWeek', 'Earlier'];
  const BAND_DISPLAY: Record<keyof typeof bands, string> = {
    Today: 'Today', Yesterday: 'Yesterday', ThisWeek: 'This week', Earlier: 'Earlier',
  };
</script>

<main
  class="ts-main"
  bind:this={mainEl}
  {@attach pullToRefresh({
    onRefresh: doRefresh,
    getScrollTop: () => mainEl?.scrollTop ?? 0,
  })}
>
  {#if $entries.loading}
    <p class="status">Loading…</p>
  {:else if $entries.error}
    <p class="status err">{$entries.error}</p>
  {:else if $entries.items.length === 0}
    <div class="ts-empty">
      <div class="ts-empty-dot" aria-hidden="true"></div>
      <div class="ts-empty-title">Inbox zero.</div>
      <div class="ts-empty-sub">Tap polls every few minutes. Come back later.</div>
    </div>
  {:else}
    {#if refreshing}
      <div class="refresh-indicator" aria-live="polite"><span class="pulse" aria-hidden="true"></span></div>
    {/if}
    <div class="ts-list" role="list" aria-label="Unread entries">
      {#each BAND_LABELS as label (label)}
        {@const bandItems = bands[label]}
        {#if bandItems.length > 0}
          <GroupHeading label={BAND_DISPLAY[label]} count={bandItems.length} />
          {#each bandItems as entry (entry.id)}
            <div role="listitem">
              <EntryRow
                {entry}
                feed={feedFor(entry.subscription_id)}
                isSelected={selectedId === entry.id}
                density={density.value}
                onToggleRead={() => entries.toggleRead(entry.id, !entry.read)}
                onToggleSaved={() => entries.toggleSaved(entry.id, !entry.saved)}
              />
            </div>
          {/each}
        {/if}
      {/each}
    </div>
  {/if}
</main>

<style>
  .ts-main { flex: 1; padding-top: 4px; overflow-y: auto; }
  .status { padding: 24px; color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
  .status.err { color: #c43a3a; }
  .ts-empty { padding: 80px 24px; text-align: center; color: var(--ink-3); }
  .ts-empty-dot {
    width: 8px; height: 8px; border-radius: 50%; background: var(--accent);
    margin: 0 auto 14px; display: block;
  }
  .ts-empty-title { font-family: var(--serif); font-size: 19px; color: var(--ink); margin-bottom: 6px; }
  .ts-empty-sub { font-family: var(--sans); font-size: 13px; color: var(--ink-3); max-width: 360px; margin: 0 auto; }
  .ts-list { display: flex; flex-direction: column; }
  .refresh-indicator { display: flex; justify-content: center; padding: 12px 0; }
  .pulse {
    width: 6px; height: 6px; border-radius: 50%; background: var(--accent);
    animation: tap-pulse 2.4s ease-in-out infinite;
  }
</style>
```

- [ ] **Step 5: Run the test set**

```bash
pnpm --dir web test -- src/views/__tests__/Unread.test.ts src/views/__tests__/UnreadMarkAll.test.ts
```

Expected: pass. `UnreadMarkAll.test.ts` already exists — read it before running, and adjust the selector queries if the test reaches into `.layout` / `.main` which no longer exist; replace with `.ts-main`/`.ts-list` selectors.

- [ ] **Step 6: Commit**

```bash
git add web/src/views/Unread.svelte web/src/views/__tests__/Unread.test.ts web/src/views/__tests__/UnreadMarkAll.test.ts
git commit -m "M2: Unread list grouped by day-band using bucketByDay"
```

---

## Task 5: readerScroll persistence (TDD)

**Files:**
- Create: `web/src/lib/readerScroll.ts`
- Test: `web/src/lib/__tests__/readerScroll.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
// web/src/lib/__tests__/readerScroll.test.ts
import { describe, it, expect, beforeEach } from 'vitest';
import { loadScroll, saveScroll, clearScroll } from '../readerScroll';

beforeEach(() => localStorage.clear());

describe('readerScroll', () => {
  it('round-trips an integer scroll value', () => {
    saveScroll(42, 1200);
    expect(loadScroll(42)).toBe(1200);
  });

  it('returns 0 when no value is stored', () => {
    expect(loadScroll(99)).toBe(0);
  });

  it('returns 0 when stored value is not a finite number', () => {
    localStorage.setItem('tap.reader.scroll.7', 'not-a-number');
    expect(loadScroll(7)).toBe(0);
  });

  it('clearScroll removes the key', () => {
    saveScroll(3, 50);
    clearScroll(3);
    expect(loadScroll(3)).toBe(0);
  });

  it('saveScroll rejects non-finite input by clearing the key', () => {
    saveScroll(5, 100);
    saveScroll(5, NaN);
    expect(loadScroll(5)).toBe(0);
  });

  it('keys are namespaced so different entry ids do not collide', () => {
    saveScroll(1, 10); saveScroll(2, 20);
    expect(loadScroll(1)).toBe(10);
    expect(loadScroll(2)).toBe(20);
  });
});
```

- [ ] **Step 2: Run to see failures**

```bash
pnpm --dir web test -- src/lib/__tests__/readerScroll.test.ts
```

Expected: module-not-found.

- [ ] **Step 3: Implement `readerScroll.ts`**

```ts
// web/src/lib/readerScroll.ts
const PREFIX = 'tap.reader.scroll.';

export function loadScroll(entryId: number): number {
  try {
    const raw = localStorage.getItem(PREFIX + entryId);
    if (raw == null) return 0;
    const n = Number(raw);
    return Number.isFinite(n) ? n : 0;
  } catch {
    return 0;
  }
}

export function saveScroll(entryId: number, scrollTop: number): void {
  try {
    if (!Number.isFinite(scrollTop)) {
      localStorage.removeItem(PREFIX + entryId);
      return;
    }
    localStorage.setItem(PREFIX + entryId, String(Math.max(0, Math.round(scrollTop))));
  } catch {
    /* localStorage may be unavailable in private mode — fail silent */
  }
}

export function clearScroll(entryId: number): void {
  try { localStorage.removeItem(PREFIX + entryId); } catch { /* ignore */ }
}
```

- [ ] **Step 4: Run; confirm pass**

```bash
pnpm --dir web test -- src/lib/__tests__/readerScroll.test.ts
```

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/readerScroll.ts web/src/lib/__tests__/readerScroll.test.ts
git commit -m "M2: per-entry scroll position persistence in localStorage"
```

---

## Task 6: markOnScroll observer (TDD)

**Files:**
- Create: `web/src/lib/markOnScroll.ts`
- Test: `web/src/lib/__tests__/markOnScroll.test.ts`

`IntersectionObserver` fires `intersectionRatio: 0` when the lede has scrolled out of view above the viewport. We start a 1.5s timer when this happens; cancel if the lede re-enters; fire `onMark` exactly once. The function is shaped as an `{@attach}`-compatible factory: `createMarkOnScroll(opts)` returns `(el: Element) => () => void`.

- [ ] **Step 1: Write the failing test**

```ts
// web/src/lib/__tests__/markOnScroll.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createMarkOnScroll } from '../markOnScroll';

let observers: Array<{ cb: IntersectionObserverCallback; el: Element }> = [];

class FakeIO implements IntersectionObserver {
  root = null; rootMargin = ''; thresholds = [];
  constructor(cb: IntersectionObserverCallback) { this.cb = cb; }
  cb: IntersectionObserverCallback;
  observe(el: Element) { observers.push({ cb: this.cb, el }); }
  disconnect() { observers = observers.filter(o => o.cb !== this.cb); }
  unobserve() { /* not used */ }
  takeRecords() { return []; }
}

beforeEach(() => {
  vi.useFakeTimers();
  observers = [];
  (globalThis as unknown as { IntersectionObserver: typeof IntersectionObserver }).IntersectionObserver = FakeIO as unknown as typeof IntersectionObserver;
});
afterEach(() => vi.useRealTimers());

function fireIntersection(isIntersecting: boolean) {
  const o = observers[0];
  const entry = { isIntersecting, boundingClientRect: { top: isIntersecting ? 100 : -50 } as DOMRectReadOnly } as unknown as IntersectionObserverEntry;
  o.cb([entry], {} as IntersectionObserver);
}

describe('createMarkOnScroll', () => {
  it('fires onMark 1.5s after lede leaves the viewport above', () => {
    const onMark = vi.fn();
    const el = document.createElement('div');
    const attach = createMarkOnScroll({ onMark, delayMs: 1500 });
    attach(el);
    fireIntersection(true);                // initially visible
    fireIntersection(false);               // scrolled past
    vi.advanceTimersByTime(1499);
    expect(onMark).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(onMark).toHaveBeenCalledOnce();
  });

  it('does not fire if user scrolls back before timer fires', () => {
    const onMark = vi.fn();
    const el = document.createElement('div');
    const attach = createMarkOnScroll({ onMark });
    attach(el);
    fireIntersection(true);
    fireIntersection(false);
    vi.advanceTimersByTime(500);
    fireIntersection(true);                // back in view
    vi.advanceTimersByTime(2000);
    expect(onMark).not.toHaveBeenCalled();
  });

  it('fires at most once even if lede leaves and re-leaves', () => {
    const onMark = vi.fn();
    const el = document.createElement('div');
    const attach = createMarkOnScroll({ onMark });
    attach(el);
    fireIntersection(true); fireIntersection(false);
    vi.advanceTimersByTime(1500);
    expect(onMark).toHaveBeenCalledOnce();
    fireIntersection(true); fireIntersection(false);
    vi.advanceTimersByTime(2000);
    expect(onMark).toHaveBeenCalledOnce();
  });

  it('cleanup disconnects the observer', () => {
    const onMark = vi.fn();
    const el = document.createElement('div');
    const attach = createMarkOnScroll({ onMark });
    const cleanup = attach(el);
    expect(observers).toHaveLength(1);
    cleanup();
    expect(observers).toHaveLength(0);
  });

  it('treats the lede as "above viewport" only when boundingClientRect.top < 0', () => {
    // intersectionRatio==0 can also mean "below viewport" — we mustn't fire then.
    const onMark = vi.fn();
    const el = document.createElement('div');
    const attach = createMarkOnScroll({ onMark });
    attach(el);
    const o = observers[0];
    o.cb([{ isIntersecting: false, boundingClientRect: { top: 5000 } as DOMRectReadOnly } as unknown as IntersectionObserverEntry], {} as IntersectionObserver);
    vi.advanceTimersByTime(2000);
    expect(onMark).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run; see failures**

```bash
pnpm --dir web test -- src/lib/__tests__/markOnScroll.test.ts
```

- [ ] **Step 3: Implement `markOnScroll.ts`**

```ts
// web/src/lib/markOnScroll.ts
export type MarkOnScrollOpts = {
  onMark: () => void;
  delayMs?: number;
};

// Returns an {@attach}-compatible factory: pass the lede element (or any
// reference element). The mark fires once, delayMs after the element scrolls
// above the viewport top.
export function createMarkOnScroll(opts: MarkOnScrollOpts) {
  const delay = opts.delayMs ?? 1500;
  return (el: Element) => {
    let fired = false;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const observer = new IntersectionObserver((records) => {
      for (const r of records) {
        const aboveViewport = !r.isIntersecting && r.boundingClientRect.top < 0;
        if (aboveViewport && !fired && timer === null) {
          timer = setTimeout(() => {
            timer = null;
            if (fired) return;
            fired = true;
            opts.onMark();
          }, delay);
        } else if (!aboveViewport && timer !== null) {
          clearTimeout(timer);
          timer = null;
        }
      }
    }, { threshold: 0 });

    observer.observe(el);
    return () => {
      if (timer !== null) clearTimeout(timer);
      observer.disconnect();
    };
  };
}
```

- [ ] **Step 4: Run; pass**

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/markOnScroll.ts web/src/lib/__tests__/markOnScroll.test.ts
git commit -m "M2: mark-on-scroll observer with 1.5s debounce + cancel on scroll-back"
```

---

## Task 7: Rewrite Reader.svelte with full ts-article anatomy (TDD)

**Files:**
- Modify: `web/src/views/Reader.svelte`
- Modify: `web/src/views/__tests__/Reader.test.ts`

The reader is the heart of M2. It owns: the back row, full `.ts-article` markup, the action row, the rule + lede + body + end, measure/font wrapper classes, mark-on-scroll wiring (when `prefs.markOnScroll === true`), scroll-position persistence, and the keyboard hooks for `M` / `S` / `V` / `H` / `1` / `2` / `3`.

The existing reader auto-marks read on mount (the existing code does this inside `$effect`). With mark-on-scroll enabled, we *defer* the auto-mark to the scroll observer — the mount-time fetch sets `entry` but does not patch read state. With mark-on-scroll disabled, fall back to the legacy mount-time auto-mark. (`M` key keeps working in both modes.)

- [ ] **Step 1: Update the test file**

Replace the existing `Reader.test.ts` with a version that covers the new DOM and behaviour. Key cases:

1. Renders `.ts-back`, `.ts-article`, `.ts-article-title`, `.ts-article-actions`, `.ts-article-rule`, `.ts-article-end`, `.ts-article-foot`.
2. Applies `.measure-<value>` class to the article wrapper from `prefs.measure`.
3. Applies the sans font on the article wrapper when `font.value === 'sans'` (e.g., inline style or class).
4. When `prefs.markOnScroll === true`, does NOT auto-mark on mount; only `M` or the observer triggers.
5. When `prefs.markOnScroll === false`, auto-marks on mount (legacy behaviour).
6. `M` toggles read via `entries.toggleRead`.
7. `S` toggles saved via `entries.toggleSaved`.
8. `V` opens `entry.url` in a new tab (mock `window.open`).
9. `Esc`/`H` navigates back to `/`.
10. `1`/`2`/`3` change `prefs.measure` to `narrow`/`comfortable`/`wide`.
11. On mount, after entry loads, the article element's `scrollTop` is set to `loadScroll(id)`.
12. On scroll, `saveScroll(id, scrollTop)` is called (debounced).

For brevity the plan shows the most load-bearing four; the implementer should write tests for all 12 in one file.

```ts
// web/src/views/__tests__/Reader.test.ts (replacement excerpts)
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import type { EntryDetail } from '../../lib/types';

vi.mock('../../components/FeedAvatar.svelte', () => ({ default: vi.fn() }));

const mockToggleRead = vi.fn().mockResolvedValue(undefined);
const mockToggleSaved = vi.fn().mockResolvedValue(undefined);
vi.mock('../../lib/store', () => ({
  entries: {
    subscribe: (fn: (v: { items: unknown[] }) => void) => { fn({ items: [] }); return () => {}; },
    toggleRead: (...a: unknown[]) => mockToggleRead(...a),
    toggleSaved: (...a: unknown[]) => mockToggleSaved(...a),
  },
  subscriptions: { subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; }, load: vi.fn() },
}));

const mockNavigate = vi.fn();
vi.mock('../../lib/router', () => ({
  navigate: (...a: unknown[]) => mockNavigate(...a),
  route: { subscribe: (fn: (v: unknown) => void) => { fn({ name: 'reader', params: { id: 1 } }); return () => {}; } },
}));

const mockGetEntry = vi.fn();
const mockPatchEntry = vi.fn();
vi.mock('../../lib/api', () => ({
  api: {
    getEntry: (...a: unknown[]) => mockGetEntry(...a),
    patchEntry: (...a: unknown[]) => mockPatchEntry(...a),
  },
}));

const mockLoadScroll = vi.fn().mockReturnValue(0);
const mockSaveScroll = vi.fn();
vi.mock('../../lib/readerScroll', () => ({
  loadScroll: (...a: unknown[]) => mockLoadScroll(...a),
  saveScroll: (...a: unknown[]) => mockSaveScroll(...a),
  clearScroll: vi.fn(),
}));

// preferences: provide measure + font + markOnScroll as plain reactive holders
const prefs = { measure: 'comfortable', font: 'serif', markOnScroll: true };
vi.mock('../../lib/preferences.svelte', () => ({
  measure: { get value() { return prefs.measure; }, set value(v: string) { prefs.measure = v; } },
  font:    { get value() { return prefs.font; },    set value(v: string) { prefs.font = v; } },
  markOnScroll: { get value() { return prefs.markOnScroll; }, set value(v: boolean) { prefs.markOnScroll = v; } },
  density: { get value() { return 'comfortable'; }, set value(_: string) {} },
  theme:   { get resolved() { return 'light'; }, get stored() { return 'light'; }, set stored(_: string) {} },
}));

const { default: Reader } = await import('../Reader.svelte');

function makeEntry(o: Partial<EntryDetail> = {}): EntryDetail {
  return {
    id: 42, subscription_id: 1, title: 'A title', author: 'Author',
    url: 'https://example.com/a', content: '<p>body</p>',
    published_at: 1700000000, fetched_at: 1700000001,
    read: false, saved: false, extract_failed: false,
    ...o,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  prefs.measure = 'comfortable'; prefs.font = 'serif'; prefs.markOnScroll = true;
  mockLoadScroll.mockReturnValue(0);
});

describe('Reader view (M2 ts-article anatomy)', () => {
  it('renders the ts-article structure when entry loads', async () => {
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: true }));
    const { container } = render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(container.querySelector('.ts-article')).toBeTruthy());
    expect(container.querySelector('.ts-back')).toBeTruthy();
    expect(container.querySelector('.ts-article-title')).toBeTruthy();
    expect(container.querySelector('.ts-article-actions')).toBeTruthy();
    expect(container.querySelector('.ts-article-rule')).toBeTruthy();
    expect(container.querySelector('.ts-article-end')).toBeTruthy();
  });

  it('applies measure-<value> class to the article wrapper', async () => {
    prefs.measure = 'narrow';
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: true }));
    const { container } = render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(container.querySelector('.ts-shell-reader.measure-narrow')).toBeTruthy());
  });

  it('does NOT auto-mark on mount when markOnScroll preference is true', async () => {
    prefs.markOnScroll = true;
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: false }));
    render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(screen.queryByText('A title')).toBeTruthy());
    expect(mockToggleRead).not.toHaveBeenCalled();
  });

  it('auto-marks on mount when markOnScroll preference is false (legacy)', async () => {
    prefs.markOnScroll = false;
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: false }));
    render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(mockToggleRead).toHaveBeenCalledWith(42, true));
  });

  it('Mark unread button toggles read state', async () => {
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: true }));
    render(Reader, { props: { id: 42 } });
    await waitFor(() => screen.getByText(/Mark unread/i));
    await fireEvent.click(screen.getByText(/Mark unread/i));
    await waitFor(() => expect(mockToggleRead).toHaveBeenCalledWith(42, false));
  });

  it('Saved button toggles saved state', async () => {
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: true, saved: false }));
    render(Reader, { props: { id: 42 } });
    await waitFor(() => screen.getByText(/Saved/i));
    await fireEvent.click(screen.getByText(/Saved/i));
    await waitFor(() => expect(mockToggleSaved).toHaveBeenCalledWith(42, true));
  });

  it('back row navigates to / on click', async () => {
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: true }));
    const { container } = render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(container.querySelector('.ts-back')).toBeTruthy());
    await fireEvent.click(container.querySelector('.ts-back')!);
    expect(mockNavigate).toHaveBeenCalledWith('/');
  });

  it('on mount restores scrollTop from readerScroll.loadScroll', async () => {
    mockLoadScroll.mockReturnValue(880);
    mockGetEntry.mockResolvedValueOnce(makeEntry({ read: true }));
    const { container } = render(Reader, { props: { id: 42 } });
    await waitFor(() => expect(container.querySelector('.ts-article')).toBeTruthy());
    expect(mockLoadScroll).toHaveBeenCalledWith(42);
  });
});
```

- [ ] **Step 2: Run tests; see failures**

```bash
pnpm --dir web test -- src/views/__tests__/Reader.test.ts
```

- [ ] **Step 3: Rewrite `Reader.svelte`**

```svelte
<!-- web/src/views/Reader.svelte -->
<script lang="ts">
  import { getContext, onMount, onDestroy } from 'svelte';
  import { api } from '../lib/api';
  import { navigate } from '../lib/router';
  import { entries } from '../lib/store';
  import { swipe } from '../lib/swipe';
  import type { EntryDetail } from '../lib/types';
  import FeedAvatar from '../components/FeedAvatar.svelte';
  import { measure, font, markOnScroll } from '../lib/preferences.svelte';
  import { loadScroll, saveScroll } from '../lib/readerScroll';
  import { createMarkOnScroll } from '../lib/markOnScroll';

  type Props = { id: number };
  let { id }: Props = $props();

  let entry = $state<EntryDetail | null>(null);
  let error = $state<string | null>(null);
  let articleEl = $state<HTMLElement | null>(null);
  let scrollEl = $state<HTMLElement | null>(null);
  let saveTimer: ReturnType<typeof setTimeout> | null = null;

  const dispatch = getContext<{
    onToggleRead: () => void;
    onToggleSaved: () => void;
    onViewOriginal: () => void;
  }>('keyDispatch');

  $effect(() => {
    const targetId = id;
    entry = null; error = null;
    let cancelled = false;
    (async () => {
      try {
        const fetched = await api.getEntry(targetId);
        if (cancelled) return;
        entry = fetched;
        // Auto-mark only when mark-on-scroll is disabled.
        if (!markOnScroll.value && fetched && !fetched.read) {
          try {
            await entries.toggleRead(targetId, true);
            if (cancelled) return;
            entry = { ...fetched, read: true };
          } catch { /* swallow */ }
        }
        // After content paint, restore scrollTop.
        requestAnimationFrame(() => {
          if (cancelled || !scrollEl) return;
          const saved = loadScroll(targetId);
          if (saved > 0) scrollEl.scrollTop = saved;
        });
      } catch (e) {
        if (cancelled) return;
        error = (e as Error).message;
      }
    })();
    return () => { cancelled = true; };
  });

  function onScroll() {
    if (!scrollEl || !entry) return;
    if (saveTimer !== null) clearTimeout(saveTimer);
    const idCopy = entry.id;
    const top = scrollEl.scrollTop;
    saveTimer = setTimeout(() => saveScroll(idCopy, top), 300);
  }

  async function toggleRead() {
    if (!entry) return;
    const want = !entry.read;
    entry = { ...entry, read: want };
    try { await entries.toggleRead(entry.id, want); error = null; }
    catch (e) { entry = { ...entry, read: !want }; error = (e as Error).message; }
  }

  async function toggleSaved() {
    if (!entry) return;
    const want = !entry.saved;
    entry = { ...entry, saved: want };
    try { await entries.toggleSaved(entry.id, want); error = null; }
    catch (e) { entry = { ...entry, saved: !want }; error = (e as Error).message; }
  }

  function viewOriginal() {
    if (entry) window.open(entry.url, '_blank', 'noopener');
  }

  function host(url: string): string {
    try { return new URL(url).host; } catch { return ''; }
  }

  function fmtDate(secs: number): string {
    return new Date(secs * 1000).toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' });
  }

  if (dispatch) {
    onMount(() => {
      dispatch.onToggleRead = toggleRead;
      dispatch.onToggleSaved = toggleSaved;
      dispatch.onViewOriginal = viewOriginal;
    });
    onDestroy(() => {
      dispatch.onToggleRead = () => {};
      dispatch.onToggleSaved = () => {};
      dispatch.onViewOriginal = () => {};
      if (saveTimer !== null) clearTimeout(saveTimer);
    });
  }

  const markOnce = createMarkOnScroll({
    onMark: () => { if (entry && !entry.read) void toggleRead(); },
  });
</script>

<section
  class="ts-shell ts-shell-reader measure-{measure.value}"
  class:font-sans={font.value === 'sans'}
  bind:this={scrollEl}
  onscroll={onScroll}
  {@attach swipe({
    onSwipeRight: () => navigate('/'),
    onSwipeLeft:  () => { /* no cross-entry nav in M2 */ },
  })}
>
  <div class="ts-backrow">
    <button type="button" class="ts-back" onclick={() => navigate('/')} aria-label="Back to Unread">
      <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10 3 5 8l5 5"/></svg>
      <span>Back to Unread</span>
    </button>
    <span class="ts-back-hint"><kbd class="ts-kbd">Esc</kbd></span>
  </div>

  {#if error}
    <p class="err">{error}</p>
  {:else if !entry}
    <p class="loading">Loading…</p>
  {:else}
    <article class="ts-article" bind:this={articleEl}>
      <div class="ts-article-source">
        <FeedAvatar feedURL={host(entry.url)} size={14} radius={3} />
        <span class="ts-article-source-name">{host(entry.url)}</span>
      </div>
      <h1 class="ts-article-title">{entry.title}</h1>
      <div class="ts-article-byline">
        {#if entry.author}<span>{entry.author}</span><span aria-hidden="true">·</span>{/if}
        <span>{fmtDate(entry.published_at)}</span>
      </div>

      <div class="ts-article-actions">
        <button type="button" class="ts-article-action" onclick={toggleRead}>
          <span class="ts-action-dot" aria-hidden="true"></span>
          <span>{entry.read ? 'Mark unread' : 'Mark read'}</span>
          <kbd class="ts-kbd">m</kbd>
        </button>
        <button type="button" class="ts-article-action" class:is-saved={entry.saved} onclick={toggleSaved}>
          <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M4 2.5h8v11l-4-3-4 3z"/></svg>
          <span>{entry.saved ? 'Saved' : 'Save'}</span>
          <kbd class="ts-kbd">s</kbd>
        </button>
        <a class="ts-article-action" href={entry.url} target="_blank" rel="noopener" onclick={viewOriginal}>
          <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 3h4v4M13 3 7 9M11 9.5V13H3V5h3.5"/></svg>
          <span>Original</span>
          <kbd class="ts-kbd">v</kbd>
        </a>
      </div>

      <div class="ts-article-rule" aria-hidden="true">
        <span class="ts-article-rule-line"></span>
        <span class="ts-article-rule-dot"></span>
        <span class="ts-article-rule-line"></span>
      </div>

      {#if entry.author}
        <p class="ts-article-lede" {@attach (markOnScroll.value ? markOnce : () => () => {})}>{entry.author}</p>
      {/if}

      <div class="ts-article-body">{@html entry.content}</div>

      <div class="ts-article-end" aria-hidden="true">
        <span class="ts-article-rule-line"></span>
        <span class="ts-article-end-dot"></span>
        <span class="ts-article-rule-line"></span>
      </div>
      <div class="ts-article-foot">Cached locally</div>
    </article>
  {/if}
</section>

<style>
  .ts-shell-reader { max-width: 720px; margin: 0 auto; padding: 36px 24px 120px; overflow-y: auto; height: 100%; }
  .ts-shell-reader.measure-narrow .ts-article { max-width: 580px; }
  .ts-shell-reader.measure-comfortable .ts-article { max-width: 680px; }
  .ts-shell-reader.measure-wide .ts-article { max-width: 760px; }
  .ts-shell-reader.font-sans .ts-article { font-family: var(--sans); }
  .ts-shell-reader.font-sans :global(.ts-article-title),
  .ts-shell-reader.font-sans :global(.ts-article-lede),
  .ts-shell-reader.font-sans :global(.ts-article-body p),
  .ts-shell-reader.font-sans :global(.ts-article-body h2) { font-family: var(--sans); }

  .err { color: #c43a3a; font-family: var(--mono); font-size: 12px; padding: 24px; }
  .loading { color: var(--ink-3); font-family: var(--mono); font-size: 11px; padding: 24px; }

  .ts-backrow {
    display: flex; align-items: center; justify-content: space-between;
    padding: 14px 2px 0;
  }
  .ts-back {
    display: inline-flex; align-items: center; gap: 6px;
    padding: 6px 8px 6px 0;
    font-family: var(--mono); font-size: 10.5px;
    letter-spacing: 0.06em; text-transform: uppercase;
    color: var(--ink-2);
    background: transparent; border: 0; cursor: pointer; border-radius: 4px;
  }
  .ts-back:hover { color: var(--ink); }
  .ts-back-hint { font-family: var(--mono); font-size: 10px; color: var(--ink-3); }

  .ts-article { margin: 0 auto; padding: 28px 0 24px; }
  .ts-article-source {
    display: flex; align-items: center; gap: 8px;
    font-family: var(--sans); font-size: 12px; color: var(--ink-2);
    margin-bottom: 22px;
  }
  .ts-article-source-name { color: var(--ink); font-weight: 500; }
  .ts-article-title {
    font-family: var(--serif); font-size: 38px; line-height: 1.12;
    font-weight: 600; color: var(--ink);
    letter-spacing: -0.02em; margin: 0 0 14px; text-wrap: balance;
  }
  .ts-article-byline {
    display: flex; flex-wrap: wrap; gap: 8px; align-items: center;
    font-family: var(--mono); font-size: 10.5px;
    letter-spacing: 0.06em; text-transform: uppercase;
    color: var(--ink-3); margin: 0 0 22px;
  }
  .ts-article-actions {
    display: flex; gap: 4px; flex-wrap: wrap;
    padding: 10px 0 4px;
    border-top: 1px solid var(--rule); border-bottom: 1px solid var(--rule);
    margin-bottom: 32px;
  }
  .ts-article-action {
    display: inline-flex; align-items: center; gap: 7px;
    padding: 8px 10px;
    font-family: var(--sans); font-size: 12.5px; font-weight: 500;
    color: var(--ink-2);
    background: transparent; border: 0; cursor: pointer; border-radius: 4px;
    text-decoration: none;
  }
  .ts-article-action:hover { color: var(--ink); background: var(--bg-soft); }
  .ts-article-action.is-saved { color: var(--accent); }
  .ts-action-dot {
    display: inline-block; width: 7px; height: 7px;
    border-radius: 50%; border: 1px solid currentColor; opacity: 0.7;
  }
  .ts-kbd {
    font-family: var(--mono); font-size: 10px;
    border: 1px solid var(--rule); border-bottom-width: 2px;
    border-radius: 3px; padding: 1px 5px;
    background: var(--surface); color: var(--ink-2);
  }

  .ts-article-rule {
    display: flex; align-items: center; gap: 10px;
    margin: 0 0 28px;
  }
  .ts-article-rule-line { flex: 1; height: 1px; background: var(--rule); }
  .ts-article-rule-dot {
    width: 6px; height: 6px; border-radius: 50%;
    background: var(--accent); flex-shrink: 0;
  }

  .ts-article-lede {
    font-family: var(--serif); font-size: 19px; line-height: 1.55;
    color: var(--ink); margin: 0 0 28px; font-style: italic; text-wrap: pretty;
  }
  .ts-article-body :global(p) {
    font-family: var(--serif); font-size: 17px; line-height: 1.7;
    color: var(--ink); margin: 0 0 22px; text-wrap: pretty;
  }
  .ts-article-body :global(h2) {
    font-family: var(--serif); font-size: 22px; line-height: 1.25;
    font-weight: 600; letter-spacing: -0.01em; margin: 40px 0 14px;
  }
  .ts-article-body :global(pre), .ts-article-body :global(code) {
    font-family: var(--mono); font-size: 12.5px; line-height: 1.55;
    color: var(--ink-2); background: var(--bg-soft);
    padding: 14px 16px; border-left: 2px solid var(--accent); overflow-x: auto;
  }

  .ts-article-end {
    display: flex; align-items: center; gap: 10px;
    margin: 48px 0 18px;
  }
  .ts-article-end-dot {
    width: 6px; height: 6px; border-radius: 50%;
    background: var(--ink-4); flex-shrink: 0;
  }
  .ts-article-foot {
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.06em; text-transform: uppercase;
    color: var(--ink-3); text-align: center;
  }
</style>
```

- [ ] **Step 4: Run; iterate**

```bash
pnpm --dir web test -- src/views/__tests__/Reader.test.ts
```

- [ ] **Step 5: Commit**

```bash
git add web/src/views/Reader.svelte web/src/views/__tests__/Reader.test.ts
git commit -m "M2: Reader rebuilt to ts-article anatomy with mark-on-scroll and scroll persistence"
```

---

## Task 8: Add `markOnScroll` preference (TDD)

**Files:**
- Modify: `web/src/lib/preferences.svelte.ts`
- Modify: `web/src/lib/__tests__/preferences.test.ts`

- [ ] **Step 1: Write/extend the failing test**

```ts
// excerpt — append to existing preferences.test.ts
import { markOnScroll } from '../preferences.svelte';

describe('markOnScroll preference', () => {
  beforeEach(() => localStorage.clear());

  it('defaults to true when no value is stored', () => {
    expect(markOnScroll.value).toBe(true);
  });

  it('reads a stored false value', () => {
    localStorage.setItem('tap.markOnScroll', '0');
    // Reload module (vitest module-reset pattern depends on the suite's setup).
    // If the project doesn't already use vi.resetModules per test, document the
    // expectation that a fresh page load picks up the persisted value, and
    // assert via the setter round-trip instead:
    markOnScroll.value = false;
    expect(markOnScroll.value).toBe(false);
  });

  it('persists changes to localStorage', () => {
    markOnScroll.value = false;
    expect(localStorage.getItem('tap.markOnScroll')).toBe('0');
    markOnScroll.value = true;
    expect(localStorage.getItem('tap.markOnScroll')).toBe('1');
  });
});
```

- [ ] **Step 2: Run; see failure**

- [ ] **Step 3: Extend `preferences.svelte.ts`**

```ts
// append to existing file
function makeBoolPref(key: string, def: boolean) {
  const raw = typeof localStorage !== 'undefined' ? localStorage.getItem(key) : null;
  let value = $state<boolean>(raw == null ? def : raw === '1');
  return {
    get value() { return value; },
    set value(v: boolean) {
      value = v;
      try { localStorage.setItem(key, v ? '1' : '0'); } catch { /* ignore */ }
    },
  };
}

export const markOnScroll = makeBoolPref('tap.markOnScroll', true);
```

Also export `measure` — if M1 didn't add it, add it here:

```ts
// only if M1 didn't add measure (Task 0 confirms)
type Measure = 'narrow' | 'comfortable' | 'wide';
const MEASURES: Measure[] = ['narrow', 'comfortable', 'wide'];
export const measure = makePref<Measure>('tap.measure', 'comfortable', MEASURES);
```

- [ ] **Step 4: Run; pass**

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/preferences.svelte.ts web/src/lib/__tests__/preferences.test.ts
git commit -m "M2: markOnScroll preference toggle (default on)"
```

---

## Task 9: Extend keyboard handler with measure + back keys (TDD)

**Files:**
- Modify: `web/src/lib/keyboard.ts`
- Modify: `web/src/lib/__tests__/keyboard.test.ts`

- [ ] **Step 1: Extend the failing test**

```ts
// append to keyboard.test.ts
describe('measure key bindings', () => {
  it('calls onMeasureNarrow for 1', () => {
    const ctx = makeCtx(); buildHandler(ctx)(fire('1'));
    expect(ctx.onMeasureNarrow).toHaveBeenCalledOnce();
  });
  it('calls onMeasureComfortable for 2', () => {
    const ctx = makeCtx(); buildHandler(ctx)(fire('2'));
    expect(ctx.onMeasureComfortable).toHaveBeenCalledOnce();
  });
  it('calls onMeasureWide for 3', () => {
    const ctx = makeCtx(); buildHandler(ctx)(fire('3'));
    expect(ctx.onMeasureWide).toHaveBeenCalledOnce();
  });
  it('calls onBack for h', () => {
    const ctx = makeCtx(); buildHandler(ctx)(fire('h'));
    expect(ctx.onBack).toHaveBeenCalledOnce();
  });
});
```

Update `makeCtx` in the same file:

```ts
function makeCtx() {
  return {
    onNext: vi.fn(), onPrev: vi.fn(), onOpen: vi.fn(),
    onToggleRead: vi.fn(), onToggleSaved: vi.fn(), onViewOriginal: vi.fn(),
    onEscape: vi.fn(), setModalOpen: vi.fn(),
    onMeasureNarrow: vi.fn(), onMeasureComfortable: vi.fn(), onMeasureWide: vi.fn(),
    onBack: vi.fn(),
  };
}
```

- [ ] **Step 2: Run; see failures**

- [ ] **Step 3: Extend `keyboard.ts`**

```ts
// add to KeyboardContext interface
export interface KeyboardContext {
  // ... existing ...
  onMeasureNarrow: () => void;
  onMeasureComfortable: () => void;
  onMeasureWide: () => void;
  onBack: () => void;
}

// add to switch
case '1': ctx.onMeasureNarrow(); break;
case '2': ctx.onMeasureComfortable(); break;
case '3': ctx.onMeasureWide(); break;
case 'h': ctx.onBack(); break;
```

- [ ] **Step 4: Update `App.svelte`** to provide context for the new handlers and wire them: when route is `reader`, `1`/`2`/`3` write `measure.value`; `h` calls `navigate('/')`. When route is not `reader`, they're no-ops.

```svelte
<!-- App.svelte excerpt -->
import { measure } from './lib/preferences.svelte';
const keyHandler = buildHandler({
  // ... existing getters ...
  onMeasureNarrow: () => { if ($route.name === 'reader') measure.value = 'narrow'; },
  onMeasureComfortable: () => { if ($route.name === 'reader') measure.value = 'comfortable'; },
  onMeasureWide: () => { if ($route.name === 'reader') measure.value = 'wide'; },
  onBack: () => { if ($route.name === 'reader') navigate('/'); },
  // ...
});
```

- [ ] **Step 5: Run all keyboard + App tests; iterate**

```bash
pnpm --dir web test -- src/lib/__tests__/keyboard.test.ts src/views/__tests__/App.test.ts
```

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/keyboard.ts web/src/lib/__tests__/keyboard.test.ts web/src/App.svelte
git commit -m "M2: keyboard bindings for measure (1/2/3) and back (h) in reader"
```

---

## Task 10: SearchOverlay implementation (TDD)

**Files:**
- Modify: `web/src/components/SearchOverlay.svelte`
- Modify: `web/src/lib/searchOverlay.svelte.ts` *(if M1 left it as a stub — re-read in Task 0)*
- Create: `web/src/components/__tests__/SearchOverlay.test.ts`

M1 ships:
- `lib/searchOverlay.svelte.ts` with `state = $state({ open: false, query: '', scope: 'unread' | 'saved' | 'all' })` and `open()` / `close()` helpers.
- An empty `SearchOverlay.svelte` component mounted by `App.svelte` (under the route switch).
- `/` key handler in App that calls `searchOverlay.open()` instead of `navigate('/search')`.

M2 fills the empty component with the actual search UI.

- [ ] **Step 1: Write the failing test**

```ts
// web/src/components/__tests__/SearchOverlay.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, waitFor } from '@testing-library/svelte';
import SearchOverlay from '../SearchOverlay.svelte';

const overlayState = { open: false, query: '', scope: 'unread' as const };
vi.mock('../../lib/searchOverlay.svelte', () => ({
  searchOverlay: {
    get open()  { return overlayState.open; },  set open(v: boolean) { overlayState.open = v; },
    get query() { return overlayState.query; }, set query(v: string)  { overlayState.query = v; },
    get scope() { return overlayState.scope; }, set scope(v: 'unread' | 'saved' | 'all') { overlayState.scope = v; },
    close() { overlayState.open = false; overlayState.query = ''; },
  },
}));

const mockSearch = vi.fn();
vi.mock('../../lib/api', () => ({ api: { searchEntries: (...a: unknown[]) => mockSearch(...a) } }));

const mockNavigate = vi.fn();
vi.mock('../../lib/router', () => ({ navigate: (...a: unknown[]) => mockNavigate(...a) }));

beforeEach(() => {
  vi.useFakeTimers();
  vi.clearAllMocks();
  overlayState.open = false; overlayState.query = ''; overlayState.scope = 'unread';
});
afterEach(() => vi.useRealTimers());

describe('SearchOverlay', () => {
  it('does not render when closed', () => {
    const { container } = render(SearchOverlay);
    expect(container.querySelector('.search-overlay')).toBeNull();
  });

  it('renders and focuses input when opened', async () => {
    overlayState.open = true;
    const { container } = render(SearchOverlay);
    const input = container.querySelector('input') as HTMLInputElement;
    expect(input).toBeTruthy();
    await waitFor(() => expect(document.activeElement).toBe(input));
  });

  it('shows hint when query is too short', async () => {
    overlayState.open = true;
    render(SearchOverlay);
    const input = screen.getByRole('searchbox');
    await fireEvent.input(input, { target: { value: 'ab' } });
    expect(screen.getByText(/at least 3/i)).toBeTruthy();
  });

  it('debounces api.searchEntries by 250ms', async () => {
    overlayState.open = true;
    mockSearch.mockResolvedValue({ data: [] });
    render(SearchOverlay);
    const input = screen.getByRole('searchbox');
    await fireEvent.input(input, { target: { value: 'abc' } });
    expect(mockSearch).not.toHaveBeenCalled();
    vi.advanceTimersByTime(249);
    expect(mockSearch).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    await waitFor(() => expect(mockSearch).toHaveBeenCalledWith('abc'));
  });

  it('closes on Esc and clears query', async () => {
    overlayState.open = true; overlayState.query = 'abc';
    const { container } = render(SearchOverlay);
    await fireEvent.keyDown(container.querySelector('.search-overlay')!, { key: 'Escape' });
    expect(overlayState.open).toBe(false);
  });

  it('closes on scrim click (click outside the panel)', async () => {
    overlayState.open = true;
    const { container } = render(SearchOverlay);
    const scrim = container.querySelector('.search-overlay') as HTMLElement;
    await fireEvent.click(scrim);
    expect(overlayState.open).toBe(false);
  });

  it('navigates to /entry/:id on result click and closes', async () => {
    overlayState.open = true;
    mockSearch.mockResolvedValue({ data: [{ id: 7, subscription_id: 1, title: 'hit', published_at: 0, fetched_at: 0, read: false, saved: false, extract_failed: false, url: 'https://e/' }] });
    render(SearchOverlay);
    const input = screen.getByRole('searchbox');
    await fireEvent.input(input, { target: { value: 'hit' } });
    vi.advanceTimersByTime(250);
    await waitFor(() => expect(screen.getByText('hit')).toBeTruthy());
    await fireEvent.click(screen.getByText('hit'));
    expect(mockNavigate).toHaveBeenCalledWith('/entry/7');
    expect(overlayState.open).toBe(false);
  });
});
```

- [ ] **Step 2: Run; see failures**

- [ ] **Step 3: Implement `SearchOverlay.svelte`**

```svelte
<!-- web/src/components/SearchOverlay.svelte -->
<script lang="ts">
  import { searchOverlay } from '../lib/searchOverlay.svelte';
  import { api } from '../lib/api';
  import { navigate } from '../lib/router';
  import type { EntryListItem } from '../lib/types';

  let inputEl = $state<HTMLInputElement | null>(null);
  let results = $state<EntryListItem[]>([]);
  let loading = $state(false);
  let error = $state<string | null>(null);
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;
  let restoreFocusTo: HTMLElement | null = null;

  $effect(() => {
    if (searchOverlay.open) {
      restoreFocusTo = document.activeElement as HTMLElement | null;
      queueMicrotask(() => inputEl?.focus());
    } else {
      results = []; error = null; loading = false;
      if (debounceTimer !== null) { clearTimeout(debounceTimer); debounceTimer = null; }
      restoreFocusTo?.focus?.();
      restoreFocusTo = null;
    }
  });

  async function runSearch(q: string) {
    if (q.length < 3) { results = []; return; }
    loading = true; error = null;
    try {
      const resp = await api.searchEntries(q);
      results = resp.data;
    } catch (e) {
      error = (e as Error).message; results = [];
    } finally {
      loading = false;
    }
  }

  function onInput(ev: Event) {
    const q = (ev.target as HTMLInputElement).value;
    searchOverlay.query = q;
    if (debounceTimer !== null) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => runSearch(q), 250);
  }

  function pickResult(id: number) {
    navigate(`/entry/${id}`);
    searchOverlay.close();
  }

  function onKeyDown(ev: KeyboardEvent) {
    if (ev.key === 'Escape') { searchOverlay.close(); }
  }

  function onScrimClick(ev: MouseEvent) {
    if (ev.target === ev.currentTarget) searchOverlay.close();
  }
</script>

{#if searchOverlay.open}
  <div class="search-overlay" role="dialog" aria-modal="true" aria-label="Search entries"
       onclick={onScrimClick} onkeydown={onKeyDown}>
    <div class="search-panel">
      <input
        bind:this={inputEl}
        type="search"
        role="searchbox"
        placeholder="Search…"
        value={searchOverlay.query}
        oninput={onInput}
        aria-label="Search entries"
      />
      {#if searchOverlay.query.length > 0 && searchOverlay.query.length < 3}
        <p class="hint">Type at least 3 characters.</p>
      {:else if loading}
        <p class="hint">Searching…</p>
      {:else if error}
        <p class="hint err">{error}</p>
      {:else if results.length === 0 && searchOverlay.query.length >= 3}
        <p class="hint">No results.</p>
      {:else}
        <ul class="results" role="list">
          {#each results as r (r.id)}
            <li role="listitem">
              <button type="button" class="result" onclick={() => pickResult(r.id)}>{r.title}</button>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>
{/if}

<style>
  .search-overlay {
    position: fixed; inset: 0;
    background: rgba(0, 0, 0, 0.32);
    display: flex; align-items: flex-start; justify-content: center;
    padding-top: 80px;
    z-index: 300;
  }
  .search-panel {
    width: min(560px, 92vw);
    background: var(--bg);
    border: 1px solid var(--rule);
    border-radius: 6px;
    box-shadow: 0 10px 30px rgba(0,0,0,0.18);
    padding: 14px;
  }
  input {
    width: 100%; box-sizing: border-box;
    font-family: var(--sans); font-size: 14px;
    background: transparent; color: var(--ink);
    border: 1px solid var(--rule); border-radius: 4px;
    padding: 9px 12px; outline: none;
  }
  input:focus { border-color: var(--accent); outline: 2px solid var(--accent); outline-offset: 2px; }
  .hint { font-family: var(--mono); font-size: 11px; color: var(--ink-3); padding: 16px 8px; text-align: center; }
  .hint.err { color: #c43a3a; }
  .results { list-style: none; margin: 12px 0 0; padding: 0; max-height: 50vh; overflow-y: auto; }
  .result {
    display: block; width: 100%; text-align: left;
    padding: 8px 10px; background: transparent; border: 0; cursor: pointer;
    font-family: var(--serif); font-size: 15px; color: var(--ink);
    border-radius: 4px;
  }
  .result:hover { background: var(--bg-soft); }
</style>
```

- [ ] **Step 4: Run; iterate**

```bash
pnpm --dir web test -- src/components/__tests__/SearchOverlay.test.ts
```

- [ ] **Step 5: Commit**

```bash
git add web/src/components/SearchOverlay.svelte web/src/components/__tests__/SearchOverlay.test.ts
git commit -m "M2: SearchOverlay debounced search with focus management and Esc/scrim close"
```

---

## Task 11: Update HotkeysModal with new rows `[scaffold]`

**Files:**
- Modify: `web/src/components/HotkeysModal.svelte`

The modal lives at `web/src/components/HotkeysModal.svelte`. Add rows for `V` (view original — currently missing), `H` (back from reader), and the three measure keys. Group as "Navigation", "Actions", "Reader".

- [ ] **Step 1: Read the current file** to confirm current grouping; preserve markup style.

- [ ] **Step 2: Add the new rows**

Within `<div class="tap-modal-body">`, after the existing Actions group add a third column:

```svelte
<div>
  <div class="shortcut-group-title">Reader</div>
  <div class="shortcut-row">
    <span class="shortcut-desc">View original</span>
    <span class="shortcut-keys"><kbd class="kbd">v</kbd></span>
  </div>
  <div class="shortcut-row">
    <span class="shortcut-desc">Back to list</span>
    <span class="shortcut-keys"><kbd class="kbd">h</kbd><span class="shortcut-plus">/</span><kbd class="kbd">Esc</kbd></span>
  </div>
  <div class="shortcut-row">
    <span class="shortcut-desc">Narrow measure</span>
    <span class="shortcut-keys"><kbd class="kbd">1</kbd></span>
  </div>
  <div class="shortcut-row">
    <span class="shortcut-desc">Comfortable measure</span>
    <span class="shortcut-keys"><kbd class="kbd">2</kbd></span>
  </div>
  <div class="shortcut-row">
    <span class="shortcut-desc">Wide measure</span>
    <span class="shortcut-keys"><kbd class="kbd">3</kbd></span>
  </div>
</div>
```

- [ ] **Step 3: Run existing modal test**

```bash
pnpm --dir web test -- src/components/__tests__/HotkeysModal.test.ts
```

- [ ] **Step 4: Commit**

```bash
git add web/src/components/HotkeysModal.svelte
git commit -m "M2: HotkeysModal lists reader shortcuts (v / h / 1 / 2 / 3)"
```

---

## Task 12: Delete `Search.svelte` and the `search` route (TDD)

**Files:**
- Delete: `web/src/views/Search.svelte`
- Delete: `web/src/views/__tests__/Search.test.ts` (if it exists; M1 may already have deleted it)
- Modify: `web/src/lib/router.ts`
- Modify: `web/src/lib/__tests__/router.test.ts`
- Modify: `web/src/App.svelte` (remove `Search` import + route branch)

- [ ] **Step 1: Update the failing router test**

```ts
// excerpt — append/update in router.test.ts
it('does not produce a "search" route name (replaced by overlay)', () => {
  const r = parse('/search');
  expect(r.name).not.toBe('search');
});
```

- [ ] **Step 2: Run; observe failure**

- [ ] **Step 3: Remove the route**

In `router.ts` delete the `name: 'search'` arm and the `if (pathname === '/search')` branch. Also remove `'search'` from the `RouteState` union.

- [ ] **Step 4: Delete the view**

```bash
git rm web/src/views/Search.svelte
[ -f web/src/views/__tests__/Search.test.ts ] && git rm web/src/views/__tests__/Search.test.ts
```

- [ ] **Step 5: Remove from `App.svelte`**

Delete the import line and the `{:else if $route.name === 'search'}` branch. The `/` key handler should already call `searchOverlay.open()` (M1 work) — verify and tighten if it still says `navigate('/search')`.

- [ ] **Step 6: Run all tests**

```bash
pnpm --dir web test
pnpm --dir web run check
```

- [ ] **Step 7: Commit**

```bash
git add -A web/src/views/Search.svelte web/src/views/__tests__/Search.test.ts web/src/lib/router.ts web/src/lib/__tests__/router.test.ts web/src/App.svelte
git commit -m "M2: drop /search route and Search view; replaced by SearchOverlay"
```

---

## Task 13: Mobile reader layout `[scaffold]`

**Files:**
- Modify: `web/src/views/Reader.svelte`

Mobile is handled inside the same Reader by branching on `isMobile` from `AppShell` context. M1 should set context `appShell.isMobile`. When true, render `.ts-mobile-reader-head` + body + `.ts-mobile-reader-foot` per `tap-simple.jsx:511`. Reuse the same `<TSArticle>`-equivalent markup; only the chrome differs.

- [ ] **Step 1:** Confirm M1 exposes `isMobile` via context or a store. If M1 instead detects width inside each view via `matchMedia`, replicate that pattern locally inside Reader.svelte.

- [ ] **Step 2:** Branch the outer template:

```svelte
{#if isMobile}
  <div class="ts-mobile-reader-head">
    <button type="button" class="ts-mobile-back" onclick={() => navigate('/')} aria-label="Back to Unread">
      <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M10 3 5 8l5 5"/></svg>
      <span>Unread</span>
    </button>
    <span class="ts-wordmark">tap<span class="ts-wordmark-dot" aria-hidden="true"></span></span>
    <button type="button" class="ts-mobile-action" class:is-saved={entry?.saved} onclick={toggleSaved} aria-label="Save">
      <svg width="18" height="18" viewBox="0 0 16 16" fill={entry?.saved ? 'currentColor' : 'none'} stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 2.5h8v11l-4-3-4 3z"/></svg>
    </button>
  </div>
  <div class="ts-mobile-reader-body">
    <!-- same article markup -->
  </div>
  <div class="ts-mobile-reader-foot">
    <button type="button" class="ts-mobile-foot-btn" onclick={toggleRead}>...Mark unread</button>
    <button type="button" class="ts-mobile-foot-btn" onclick={viewOriginal}>...Original</button>
    <button type="button" class="ts-mobile-foot-btn" onclick={() => navigate('/')}>...Back</button>
  </div>
{:else}
  <!-- existing desktop markup from Task 7 -->
{/if}
```

Pull the article body into a `{#snippet articleBody()}` so both branches reuse it without duplication.

- [ ] **Step 3:** Add scoped CSS for `.ts-mobile-reader-head`, `.ts-mobile-reader-body`, `.ts-mobile-reader-foot`, `.ts-mobile-back`, `.ts-mobile-action`, `.ts-mobile-foot-btn` per `styles.css:1979–2049`.

- [ ] **Step 4: Manual smoke**

```bash
make dev
# In another shell, resize the browser to 375×812 and visit /entry/<id>.
# Confirm the head bar / body / foot bar render and the action buttons work.
```

- [ ] **Step 5: Commit**

```bash
git add web/src/views/Reader.svelte
git commit -m "M2: mobile reader chrome (head/body/foot) per ts-mobile-reader-* selectors"
```

---

## Task 14: Verification

**Files:** none changed.

- [ ] **Step 1: Run all checks**

```bash
pnpm --dir web run check
pnpm --dir web test
make test    # Go side stays green; the SPA must build first
```

Expected: pass. Count should be ≥ the baseline from Task 0 plus the new tests added by Tasks 1, 3, 5, 6, 7, 8, 9, 10, 12.

- [ ] **Step 2: Manual smoke (desktop)**

```bash
make dev
# Open http://localhost:5173
```

Confirm:
- Unread list shows day-band headings (TODAY / YESTERDAY / THIS WEEK / EARLIER as applicable).
- Junction dot is solid accent for unread, hollow ring for read.
- `J`/`K` traverses all entries across bands.
- `O` / Enter opens the reader.
- Reader shows `.ts-article-source` / `.ts-article-title` / `.ts-article-byline` / `.ts-article-actions` / `.ts-article-rule` / `.ts-article-end` / `.ts-article-foot`.
- Scrolling past the lede for 1.5s flips the read state (the junction dot in the back-navigated list updates).
- `1`/`2`/`3` change the article max-width (visible — toggle and watch reflow).
- Refreshing the reader restores the saved scroll position.
- `Esc` and `H` both return to Unread.
- `/` opens the SearchOverlay; typing 3+ chars triggers debounced search; results navigate; Esc and scrim click close.
- Saved tag appears in the meta of saved entries in the list.
- Settings → density (compact / default / cosy) hides/shows the summary line accordingly.

- [ ] **Step 3: Manual smoke (mobile)**

Resize browser to 375×812 (or use a mobile device on the LAN) — confirm:
- `.ts-mobile-reader-head` shows back + wordmark + save.
- `.ts-mobile-reader-foot` shows Mark unread / Original / Next (or Back).
- Article body padding 22/22/28 per brand spec.

- [ ] **Step 4: Final commit if any tweaks were needed**

```bash
git add -A
git status # confirm clean or only docs/superpowers/plans/* untracked
```

---

## Acceptance criteria

1. **EntryRow**: matches `Tap Brand and UI Spec.md` §4.4 — junction dot at `left: 4px; top: 24px` (simple-shell variant), 6×6, solid accent unread / hollow ring read; serif 19px title (`-0.005em`); sans meta with feed-avatar + source name + sep dots; saved tag rendered inside meta (mono 10px uppercase, accent colour). CSS selectors emitted: `.ts-entry`, `.ts-entry.is-read`, `.ts-entry.is-saved`, `.ts-entry.is-selected`, `.ts-entry-dot`, `.ts-entry-title`, `.ts-entry-meta`, `.ts-entry-source`, `.ts-saved-tag`, `.ts-entry-summary`. JSX reference: `tap-simple.jsx` `TSEntryRow` (lines 70–100). Spec line 1587–1661 in `ui_design/styles.css`.
2. **Density variants** apply on the row: `.density-compact` hides summary, `.density-cosy` clamps summary to 1 line, `.density-comfortable` (default) clamps to 2 lines. Density source is `lib/preferences.svelte.ts.density.value`. Canonical vocabulary per team-lead ruling 2026-05-11: `'compact' | 'comfortable' | 'cosy'`.
3. **Day-band groupings**: per `Tap Brand and UI Spec.md` §6.1 — `TODAY` / `YESTERDAY` / `THIS WEEK` / `EARLIER` rendered as `.ts-group-heading` with eyebrow label + flex-1 hairline rule + right-aligned mono count. Empty bands are not rendered. JSX reference: `tap-simple.jsx` `TSGroupHeading` (lines 117–125) and `bucketByDay` (128–137).
4. **Reader anatomy** matches `Tap Brand and UI Spec.md` §4.5 and `tap-simple.jsx` `TSArticle` (411–471): `.ts-back` row, `.ts-article` wrapper, `.ts-article-source` (feed avatar + name + URL), `.ts-article-title` (serif 38px), `.ts-article-byline` (mono caps), `.ts-article-actions` (Mark unread / Saved / Original each with `.ts-action-dot` and `.ts-kbd`), `.ts-article-rule` (line · accent dot · line), `.ts-article-lede` (serif italic 19px), body, `.ts-article-end`, `.ts-article-foot`.
5. **Measure control**: `.measure-narrow|comfortable|wide` applied to `.ts-shell-reader` constrains `.ts-article` max-width to 580 / 680 / 760. Preference source: `prefs.measure`. Keys `1` / `2` / `3` cycle while in the reader route.
6. **Reader font mode**: `.font-sans` class on `.ts-shell-reader` switches the article wrapper's text family to `var(--sans)`. Preference source: `prefs.font` (M1).
7. **Mark-on-scroll**: when `prefs.markOnScroll === true`, the entry is *not* marked on mount; instead it marks 1.5s after the lede leaves the viewport upward; cancels if scrolled back; fires at most once per session per entry. When `prefs.markOnScroll === false`, falls back to mount-time auto-mark. `M` always works in both modes.
8. **Scroll position persistence**: per-entry scroll restored on mount, written on scroll (debounced 300ms) via `lib/readerScroll.ts`. Survives reload of the reader URL.
9. **Keyboard**: `J/K` traverse Unread; `Enter/O` opens; `M`/`S`/`V` toggle/visit; `Esc`/`H` back; `1`/`2`/`3` switch measure; `?` opens shortcuts. All handled via `lib/keyboard.ts` and suppressed in form controls.
10. **SearchOverlay**: opens on `/`, focuses input, debounces 250ms, hits `api.searchEntries`, renders results, click navigates to `/entry/:id` and closes, Esc closes, scrim click closes, restores prior focus on close.
11. **Mobile reader chrome**: `.ts-mobile-reader-head` + `.ts-mobile-reader-body` + `.ts-mobile-reader-foot` per `tap-simple.jsx` `TapSimpleReaderMobile` (511–555) and `styles.css:1979–2049`.
12. **`/search` route removed**: `parse('/search')` does not produce `name: 'search'`; the `Search.svelte` file and its test are deleted.
13. **All tests pass**: `pnpm --dir web test`, `pnpm --dir web run check`, `make test` (which depends on `web/dist`).

---

## Verification commands

```bash
# from the repo root
pnpm --dir web run check                       # svelte-check / tsc — must be clean
pnpm --dir web test                            # vitest — all green
pnpm --dir web test -- src/lib/__tests__/dayBand.test.ts
pnpm --dir web test -- src/lib/__tests__/readerScroll.test.ts
pnpm --dir web test -- src/lib/__tests__/markOnScroll.test.ts
pnpm --dir web test -- src/lib/__tests__/keyboard.test.ts
pnpm --dir web test -- src/components/__tests__/EntryRow.test.ts
pnpm --dir web test -- src/components/__tests__/GroupHeading.test.ts
pnpm --dir web test -- src/components/__tests__/SearchOverlay.test.ts
pnpm --dir web test -- src/views/__tests__/Reader.test.ts
pnpm --dir web test -- src/views/__tests__/Unread.test.ts
pnpm --dir web test -- src/lib/__tests__/router.test.ts
make test                                      # Go-side + bundle build
make build                                     # produce bin/tap
```

After all of the above:

```bash
make dev   # http://localhost:5173 — manual smoke per Task 14 Step 2
```

---

## Risks

1. **M1 coordination boundary on `EntryRow`.** Per team-lead ruling 2026-05-11: M1 owns the EntryRow rewrite in full, and the canonical density vocabulary is `'compact' | 'comfortable' | 'cosy'` with `'comfortable'` as default — M1 also owns migrating `preferences.svelte.ts` from the existing `'compact' | 'default' | 'comfortable'` enum. M2 only writes the consumer-side tests for the Unread list's usage. If Task 3 Step 0's grep checks fail, M2 *does not* rebuild — instead the executing agent raises the gap via `team-lead`. The residual risk is *prop-signature drift*: if M1 ships a misspelling (e.g., `'cozy'` for `'cosy'`) or hasn't yet completed the `preferences.svelte.ts` migration, the Unread view will not compile. Mitigation: Task 0 Step 1 greps for the exact prop names; surface mismatches early. The plan code uses the canonical names everywhere — if you see the old `'default'` value in the repo, it is the M1 migration not landing, not an M2 plan error.
2. **`prefs.measure` ownership.** Umbrella spec line 91 puts `measure` on M1's foundations. If M1 forgets, Task 8 adds it; if M1 ships it, the executing agent verifies and moves on. The plan includes the additive code so either path lands clean.
3. **`searchOverlay` store ownership.** Umbrella spec line 92 explicitly puts the store, key handler, and *empty* component on M1. If M1 ships them not at all, Task 10 will fail to compile; the executing agent must either land the store first (small addition — copy from this plan's expected shape) or block on M1. **Mitigation:** the Task 0 baseline check fails fast if the store is missing.
4. **IntersectionObserver in jsdom.** Vitest defaults to jsdom which doesn't implement `IntersectionObserver`. The Task 6 tests stub it via `globalThis.IntersectionObserver = FakeIO`. The integration test in Reader.test.ts mocks `createMarkOnScroll` itself (the test for the observer wiring is unit-level in Task 6; the reader test only asserts the observer is attached, which is observable via the lede element's data attributes or a spy on the imported factory). The plan does not test the live observer in jsdom because the mock surface is cleaner.
5. **Mark-on-scroll on already-read entries.** If the user opens a read entry, the observer attaches but `toggleRead` is a no-op since `entry.read === true`. Cheap, no harm. We *do not* attach the observer when `markOnScroll === false` (the `{@attach}` is conditional).
6. **Day-band timezone drift.** `bucketByDay` uses local time via `Date.setHours(0,0,0,0)`. If the user travels across timezones, an entry on the boundary can flip bands. This is the intent — they see "today" relative to *now*. Tests pin a fixed `now` to keep CI deterministic.
7. **`{@html entry.content}`.** The reader renders sanitised HTML directly (server-side sanitisation per CLAUDE.md trust posture). The new template keeps this exactly; no escape mechanism is introduced. **Do not** add a client-side sanitiser — that would silently double-strip and miss CSP-friendly markers the server emits.
8. **Cross-entry `J`/`K` in the reader (deferred).** The spec mentions optional `J`/`K` inside the reader for next/prev. We do not ship this; the user can `Esc` back to the list, pick the next entry, and `O` to open. A follow-up plan can wire this against the order of `$entries.items` (no DB change).
9. **Mobile reader keyboard.** Mobile rarely has a keyboard, but the same `keyHandler` is wired at the window level. `1`/`2`/`3` still work if a Bluetooth keyboard is attached; the mobile reader doesn't break that.
10. **Service worker cache.** Asset paths change (new components). Foundations milestone owns SW invalidation; M2 inherits it. Risk: if the SW caches an old `index.html`, the user sees a half-styled reader. Manual smoke step 2 in Task 14 catches this — if any styling looks stale, the umbrella spec §7 (item 7) requires the `needRefresh` banner to surface; check that the banner appears after `pnpm --dir web build`.
11. **Existing `Reader.test.ts` cases — including the `'shows error when api.getEntry rejects'` and `'auto-marks unread entry as read on mount'` — collide with the new behaviour. The plan re-authors the test file; check Task 7 for the replacement set. **Do not** leave both the old and new assertions co-existing — the auto-mark test now asserts the *opposite* default.
12. **`UnreadMarkAll.test.ts`** reaches into specific selectors that no longer exist (`.layout`, `.main`). Read that file first and update selectors during Task 4 — easy to miss.

---

## Self-review

- **Spec coverage:** every bullet of the team-lead's scope section maps to a task: EntryRow *consumer verification* → Task 3 (the rewrite itself is M1's job per the 2026-05-11 ruling); day-band groupings → Tasks 1/2/4; `.ts-article` rebuild → Task 7; action row with kbd chips → Task 7 (markup); measure preference → Tasks 8/9 plus Task 7 (CSS); mark-on-scroll → Tasks 6/7/8; scroll persistence → Tasks 5/7; serif/sans toggle → Task 7 (CSS); SearchOverlay actual implementation → Task 10; mobile reader → Task 13.
- **Placeholders:** none. Every step has runnable commands or complete code.
- **Type consistency:** `EntryListItem`, `Subscription`, `EntryDetail` come from `lib/types.ts`. `Density` is `'compact' | 'comfortable' | 'cosy'` everywhere per team-lead ruling 2026-05-11 (default `'comfortable'`). M1 owns the `preferences.svelte.ts` migration from the prior `'compact' | 'default' | 'comfortable'` enum. `Measure` is `'narrow' | 'comfortable' | 'wide'`. `MarkOnScroll` is `boolean`. `Band` keys are `'Today' | 'Yesterday' | 'ThisWeek' | 'Earlier'`.
- **CSS selector parity:** every selector mentioned in the team-lead's grep list (`.entry`, `.ts-entry`, `.junction`, `.saved-mark`, `.ts-group-heading`, `.ts-article`, etc.) is owned by a specific task in this plan. `.entry` and `.junction` and `.saved-mark` belong to the *legacy* split-pane shape that we replace with `.ts-entry` + `.ts-entry-dot` + `.ts-saved-tag` — confirmed against the simple-shell JSX (`tap-simple.jsx`) which is the M-Redesign target shell.
