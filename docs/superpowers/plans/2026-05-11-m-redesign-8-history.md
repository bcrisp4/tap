# M-Redesign-8 (History) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the M1 `/history` stub view with a real History page — a flat chronological list of every entry the user has ever received, grouped by day-band (Today / Yesterday / This week / Earlier), with cursor pagination. Pure frontend; no backend changes.

**Architecture:** New `views/History.svelte` mounted at `/history` inside the foundations `.ts-shell` desktop and `.tap.is-mobile` shells. Fetches `/api/v1/entries` (no `unread` and no `saved` filter — returns *all* entries newest-first). Day-bands computed client-side from `entry.published_at` via a shared `bucketByDay` helper that **M-Redesign-2 owns and places at `web/src/lib/dayBands.ts`** (this plan depends on that helper existing; if M2 has not landed, we fall back to vendoring it locally and consolidating in a follow-up — see Risks). Pagination uses the API's existing `next_cursor`; a "Load more" button appends pages. No new stores, no router changes beyond what M1 already shipped.

**Tech Stack:** Svelte 5 (runes — `$state`, `$derived`, `$effect`, `$props`), TypeScript, Vitest + `@testing-library/svelte`. Scoped Svelte `<style>` blocks port `.ts-shell` / `.ts-main` / `.ts-list` / `.ts-group-heading` / `.ts-empty` rules from `ui_design/styles.css`.

---

## Scope

**In scope:**

- New `views/History.svelte` (replaces M1's "Coming soon — M8" stub).
- Initial load of `/api/v1/entries` with **no filter flags** (`unread` omitted, `saved` omitted) — server returns all entries newest-first.
- Day-band grouping client-side: **Today / Yesterday / This week / Earlier** computed from `entry.published_at` (unix seconds, server-supplied).
- Loading / error / empty paths.
- Cursor pagination: a "Load more" `Button` at the bottom of the list when `next_cursor` is present; click appends the next page and updates the cursor. No infinite scroll.
- Mobile parity: same view content, rendered under the M1 mobile shell with the M1 mobile More-sheet routing to `/history`.
- Reuse foundations primitives only: `EntryRow`, `EmptyState`, `GroupHeading`, `Button`. **Do not** introduce new primitives.
- Test coverage for: load (groups render in correct order), error path, empty path, pagination (cursor consumed and next page appended), and the History-specific listEntries call (no `unread`, no `saved`).

**Out of scope:**

- Backend changes — `/api/v1/entries` already supports this exact call shape.
- Filtering by feed / category / search inside History (defer; if needed, a later milestone).
- Mark-as-unread or save toggles from inside the History row (the existing `EntryRow` may surface them via M2; if so, they continue to work — we don't add or remove behaviour).
- Mark-all-anything bulk actions (not in design for History).
- Owning the `bucketByDay` helper — M-Redesign-2 owns its placement and shape.
- Infinite scroll / virtualised lists (defer; "Load more" is fine for the page sizes Tap users will see).
- Read-status filters or "include unread in history" toggle (the design treats History as the full timeline; if it shows unread entries, that's the design intent).
- "Read-only" filtering. The umbrella spec is internally inconsistent on this point: §2.4 (line 99) says History fetches `/api/v1/entries` "with no `unread` flag" (i.e. all entries), while §5 row M8 (line 199) says "flat chronological list of **read entries** `?unread=0`". This plan follows the §2.4 reading because `api.listEntries` (see `web/src/lib/api.ts:115–132`) only adds `unread=1` to the query when the JS value is truthy — there is no current way to request `unread=0` against the existing endpoint, so the §5 wording cannot be honoured as a pure-frontend change. The team-lead has been notified to reconcile the umbrella spec; the implementation matches §2.4.

## Files

**Created:**

- `web/src/views/History.svelte` — the page component.
- `web/src/views/__tests__/History.test.ts` — vitest suite.

**Modified:**

- (None expected.) The `/history` route, the desktop tab, and the mobile More-sheet entry are wired in M1. M8 only swaps the stub component for the real one. If M1's foundations plan registers the stub *by importing a placeholder* rather than by route-name dispatch in `App.svelte`, this plan switches the import to `views/History.svelte`; that's a one-line edit, not a route-table change.

**Deleted:**

- (None.) The M1 stub becomes unused once the real view is imported in its place; M1's primitive registration of stub pages should overwrite cleanly.

**Depends on (must exist before this plan runs):**

- M-Redesign-1 (Foundations): `.ts-shell` chrome, `EntryRow`, `EmptyState`, `Button` (must accept `onclick`, `disabled`, a `quiet` variant, and pass `data-*` attributes through to the inner `<button>` — see Task 5 precondition), the `/history` route, the desktop top-tab, the mobile More-sheet entry.
- M-Redesign-2 (Unread + Reader): `web/src/lib/dayBands.ts` exporting `bucketByDay(entries, now?: number): { today: T[]; yesterday: T[]; thisWeek: T[]; earlier: T[] }` and `GroupHeading.svelte`. If M2 places the helper elsewhere or names it differently, coordinate via PR comments and update this plan before implementation begins.

---

## Skills and tools for implementers

Before writing any code:

- Invoke `superpowers:test-driven-development` — every behaviour-bearing path here (load, error, empty, pagination, day-band ordering) has a test written first.
- Invoke `svelte-runes` — this view uses `$state`, `$derived`, `$effect`, `$props`. The existing `web/src/views/Saved.svelte` is a Svelte 5 runes example to copy patterns from.
- Invoke `svelte-styling` — the page ports `.ts-shell` / `.ts-main` / `.ts-list` rules into a scoped `<style>` block. Use `:global(...)` only where the parent owns rules that apply to descendant primitives that already own their classes.
- Invoke `tdd` if unfamiliar with vitest's red/green cycle.

For library docs:

- `mcp__plugin_context7_context7__query-docs` — for Svelte 5 runes (`$state` / `$derived` / `$effect`) or `@testing-library/svelte` (`render`, `screen`, `waitFor`, `findByText`) details. Default to the latest stable Svelte 5 reference.

Reference reading (read before coding):

- `docs/specs/2026-05-11-ui-redesign.md` — umbrella spec; especially §2.2 (routing table — confirms `/history` route name `history` and that History lives in the desktop top tabs and mobile More sheet), §2.4 (no backend changes), §3.2 (primitives list), §6 (testing posture).
- `ui_design/Tap Brand and UI Spec.md` §6.1 (Unread list patterns — History reuses entry rows and group headings), §6.3 (Saved — closest sibling page; mirror its shell choice and empty-state shape), §6.8 (mobile More sheet contains History).
- `ui_design/tap-simple.jsx` lines 116–137 (TSGroupHeading and the reference `bucketByDay` heuristic — note: the JSX version buckets on the cosmetic `ago` string; we bucket on real `published_at` seconds instead).
- `ui_design/styles.css` lines 1444–1700 (`.ts-shell`, `.ts-main`, `.ts-group-heading`, `.ts-list`, `.ts-empty`).
- `web/src/views/Saved.svelte` and `web/src/views/__tests__/Saved.test.ts` — closest sibling. History mirrors its load → empty → list shape, plus pagination.
- `web/src/lib/api.ts` `listEntries` (lines 115–132) — the API call. Note: only sets `unread=1` when truthy and `saved=1` when truthy, so omitting both yields the full list.

---

## Step-by-step tasks

> **TDD posture per task is called out inline.** All five behavioural paths (load, error, empty, pagination, listEntries call shape) are red-first. Pure CSS-port steps are scaffolding and are exempt; they ship in the same task block as their behavioural sibling so the commit always contains a tested artefact.

### Task 1: Wire the History view skeleton (loading + error + empty states)

**TDD:** Required. Tests cover the loading, error, and empty branches. No day-band logic yet.

**Files:**
- Create: `web/src/views/History.svelte`
- Create: `web/src/views/__tests__/History.test.ts`

- [ ] **Step 1: Write the failing test for the loading state**

`web/src/views/__tests__/History.test.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import { api } from '../../lib/api';

vi.mock('../../components/EntryRow.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/GroupHeading.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/EmptyState.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/Button.svelte', () => ({ default: vi.fn() }));

const { default: History } = await import('../History.svelte');

describe('History view', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the loading state while listEntries is pending', () => {
    vi.spyOn(api, 'listEntries').mockReturnValue(new Promise(() => {}) as any);
    render(History);
    expect(screen.getByText(/Loading/i)).toBeTruthy();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts`
Expected: FAIL — module `../History.svelte` does not exist.

- [ ] **Step 3: Write the minimal History.svelte to make the loading test pass**

`web/src/views/History.svelte`:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../lib/api';
  import type { EntryListItem } from '../lib/types';

  let items = $state<EntryListItem[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let cursor = $state<string | undefined>(undefined);

  async function loadInitial() {
    loading = true;
    error = null;
    try {
      const r = await api.listEntries({ limit: 100 });
      items = r.data;
      cursor = r.next_cursor;
    } catch (e) {
      error = (e as Error).message;
    } finally {
      loading = false;
    }
  }

  onMount(loadInitial);
</script>

<main class="ts-main" aria-label="History">
  {#if loading}
    <p class="status">Loading…</p>
  {:else if error}
    <p class="status err">{error}</p>
  {:else if items.length === 0}
    <p class="status empty">No history yet.</p>
  {:else}
    <p class="status">{items.length} entries</p>
  {/if}
</main>

<style>
  .ts-main { flex: 1; padding-top: 4px; }
  .status { padding: 24px; color: var(--ink-3); font-family: var(--mono); font-size: 11px; }
  .status.err { color: #b14; }
</style>
```

Note: `next_cursor` matches the `ListResponse<T>` shape in `web/src/lib/types.ts:53–56`.

- [ ] **Step 4: Run the test to verify it passes**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts`
Expected: PASS.

- [ ] **Step 5: Add the error-path test**

Append to `web/src/views/__tests__/History.test.ts`:

```ts
  it('renders the error message when listEntries rejects', async () => {
    vi.spyOn(api, 'listEntries').mockRejectedValueOnce(new Error('boom'));
    render(History);
    expect(await screen.findByText('boom')).toBeTruthy();
  });
```

- [ ] **Step 6: Run the suite and confirm green**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts`
Expected: PASS for both tests. The error path is already handled by the catch in `loadInitial`.

- [ ] **Step 7: Add the empty-state test**

Append to the suite:

```ts
  it('renders an empty-state when no entries exist', async () => {
    vi.spyOn(api, 'listEntries').mockResolvedValueOnce({ data: [], next_cursor: undefined } as any);
    render(History);
    expect(await screen.findByText(/No history yet/i)).toBeTruthy();
  });
```

- [ ] **Step 8: Run and confirm green**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts`
Expected: PASS — 3 tests.

- [ ] **Step 9: Commit**

```bash
git add web/src/views/History.svelte web/src/views/__tests__/History.test.ts
git commit -m "M-Redesign-8: skeleton History view with load/error/empty paths"
```

---

### Task 2: Verify the listEntries call shape (no `unread`, no `saved`)

**TDD:** Required. This is a single behavioural assertion that History is the *full* timeline, not a filtered view.

**Files:**
- Test: `web/src/views/__tests__/History.test.ts`

- [ ] **Step 1: Write the failing test**

Append to `web/src/views/__tests__/History.test.ts`:

```ts
  it('calls api.listEntries with no unread and no saved filters', async () => {
    const spy = vi.spyOn(api, 'listEntries').mockResolvedValueOnce({ data: [], next_cursor: undefined } as any);
    render(History);
    await waitFor(() => expect(spy).toHaveBeenCalled());
    const args = spy.mock.calls[0][0] ?? {};
    expect((args as any).unread).toBeUndefined();
    expect((args as any).saved).toBeUndefined();
  });
```

- [ ] **Step 2: Run the test to verify it passes (already green by construction)**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts`
Expected: PASS — Task 1's `loadInitial` already calls `api.listEntries({ limit: 100 })` with no `unread` and no `saved`. This test pins the behaviour against regression.

If the test fails because `loadInitial` was written with `unread: false` (an easy mistake), correct `loadInitial` to omit the key — `api.listEntries` only adds `unread=1` to the query when the JS value is **truthy**, so passing `false` is equivalent to omitting, but omitting is the canonical form and matches what the test asserts.

- [ ] **Step 3: Commit**

```bash
git add web/src/views/__tests__/History.test.ts
git commit -m "M-Redesign-8: pin History to unfiltered listEntries call"
```

---

### Task 3: Render entry rows when the API returns data

**TDD:** Required. Asserts the page emits one row per entry returned and uses `EntryRow`.

**Files:**
- Modify: `web/src/views/History.svelte`
- Modify: `web/src/views/__tests__/History.test.ts`

- [ ] **Step 1: Write the failing test**

Append to `web/src/views/__tests__/History.test.ts`:

```ts
  it('renders one list item per entry returned', async () => {
    vi.spyOn(api, 'listEntries').mockResolvedValueOnce({
      data: [
        { id: 1, subscription_id: 1, title: 'one', url: 'https://x/1',
          published_at: Math.floor(Date.now() / 1000) - 60,
          fetched_at: 1, read: true, saved: false, extract_failed: false },
        { id: 2, subscription_id: 1, title: 'two', url: 'https://x/2',
          published_at: Math.floor(Date.now() / 1000) - 120,
          fetched_at: 1, read: true, saved: false, extract_failed: false },
      ],
      next_cursor: undefined,
    } as any);
    const { container } = render(History);
    await waitFor(() => expect(container.querySelector('ul[role="list"]')).toBeTruthy());
    const items = container.querySelectorAll('li[role="listitem"]');
    expect(items).toHaveLength(2);
  });
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts -t "one list item per entry"`
Expected: FAIL — no `ul[role="list"]` in current markup.

- [ ] **Step 3: Replace the placeholder list render in History.svelte**

In `web/src/views/History.svelte`, replace the `{:else}` branch's `<p class="status">{items.length} entries</p>` with:

```svelte
  {:else}
    <ul class="ts-list" role="list" aria-label="History entries">
      {#each items as entry (entry.id)}
        <li role="listitem">
          <EntryRow {entry} feed={feedFor(entry.subscription_id)} />
        </li>
      {/each}
    </ul>
```

And in the `<script>` add the imports and the `feedFor` lookup:

```ts
  import EntryRow from '../components/EntryRow.svelte';
  import { subscriptions } from '../lib/store';

  function feedFor(subId: number) {
    return $subscriptions.find(s => s.id === subId);
  }
```

Mirror `views/Unread.svelte:67–69`, which uses `$subscriptions.find(...)` inside a `<script>`-declared function under Svelte 5 runes — verified working in this codebase. Copy that pattern verbatim.

Also load subscriptions on mount so `feedFor` finds them:

```ts
  import { subscriptions } from '../lib/store';
  onMount(async () => {
    await subscriptions.load();
    await loadInitial();
  });
```

Replace the standalone `onMount(loadInitial)` with this combined `onMount`. Order: subscriptions first (cheap, cached often), then entries; the existing `Unread.svelte` does the two in parallel via separate `onMount` calls — either is fine, but sequential keeps the test deterministic.

Add `.ts-list` styling in the `<style>` block:

```css
  .ts-list { display: flex; flex-direction: column; list-style: none; margin: 0; padding: 0; }
  .ts-list li { display: contents; }
```

- [ ] **Step 4: Run all tests for the file and confirm green**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts`
Expected: PASS — 4 tests.

If the loading test now fails because `subscriptions.load()` was added in the `onMount` chain and isn't mocked, mock it at the top of the test file alongside the existing component mocks:

```ts
vi.mock('../../lib/store', () => ({
  subscriptions: {
    subscribe: (fn: any) => { fn([]); return () => {}; },
    load: vi.fn().mockResolvedValue(undefined),
  },
}));
```

This avoids a real `api.listSubscriptions` call from inside the store and keeps the test focused on `listEntries`.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/History.svelte web/src/views/__tests__/History.test.ts
git commit -m "M-Redesign-8: render entry rows for unfiltered history"
```

---

### Task 4: Group entries by day-band using the shared `bucketByDay` helper

**TDD:** Required. Tests assert (a) the bands render in `Today → Yesterday → This week → Earlier` order, (b) empty bands are skipped, (c) the helper is the one M-Redesign-2 exports (not a private copy).

**Pre-task coordination check.** Before starting:

1. `git log origin/main..origin/plan/m-redesign-2-unread-reader -- web/src/lib/ 2>/dev/null` (or check the M2 PR diff) to confirm M2's plan places the helper at `web/src/lib/dayBands.ts` and exports `bucketByDay`.
2. If the M2 plan diverges (different filename or signature), update the import in this task and the test mocks to match **before writing any code**. Surface the divergence in a PR comment so the assumption is documented.

**Files:**
- Modify: `web/src/views/History.svelte`
- Modify: `web/src/views/__tests__/History.test.ts`

- [ ] **Step 1: Write the failing test for ordered day-band rendering**

Append:

```ts
  it('renders day-band group headings in Today → Yesterday → This week → Earlier order', async () => {
    const now = Math.floor(Date.now() / 1000);
    const day = 86400;
    vi.spyOn(api, 'listEntries').mockResolvedValueOnce({
      data: [
        { id: 1, subscription_id: 1, title: 'today',     url: 'https://x/1', published_at: now - 60,           fetched_at: 1, read: true,  saved: false, extract_failed: false },
        { id: 2, subscription_id: 1, title: 'yesterday', url: 'https://x/2', published_at: now - 1 * day - 60, fetched_at: 1, read: true,  saved: false, extract_failed: false },
        { id: 3, subscription_id: 1, title: 'thisweek',  url: 'https://x/3', published_at: now - 3 * day,      fetched_at: 1, read: true,  saved: false, extract_failed: false },
        { id: 4, subscription_id: 1, title: 'earlier',   url: 'https://x/4', published_at: now - 30 * day,     fetched_at: 1, read: true,  saved: false, extract_failed: false },
      ],
      next_cursor: undefined,
    } as any);

    const { container } = render(History);
    await waitFor(() => expect(container.querySelector('ul[role="list"]')).toBeTruthy());

    // GroupHeading is mocked, so we can't read its text directly. Read the
    // `label` prop the view passes by reading data-attributes on a thin
    // test helper, OR un-mock GroupHeading and assert via the rendered DOM.
    // We choose the simpler path: un-mock GroupHeading for this test only.
    // (Move the GroupHeading mock into a per-test setup rather than the
    // file-level vi.mock; see Step 3 for the refactor.)
    const headings = Array.from(container.querySelectorAll('[data-band]'))
      .map(el => el.getAttribute('data-band'));
    expect(headings).toEqual(['today', 'yesterday', 'thisWeek', 'earlier']);
  });
```

Why `data-band` and not text? The real `GroupHeading.svelte` (M2) renders human-readable labels (`Today`, `This week`); the test stays robust to copy changes by reading the band *key* via a `data-band` attribute that History.svelte stamps onto the wrapper around each band. This avoids coupling the test to design copy.

- [ ] **Step 2: Run the test to verify it fails**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts -t "day-band"`
Expected: FAIL — no `[data-band]` elements yet.

- [ ] **Step 3: Implement grouping in History.svelte**

At the top of `<script>` in `web/src/views/History.svelte`:

```ts
  import GroupHeading from '../components/GroupHeading.svelte';
  import { bucketByDay } from '../lib/dayBands';
```

Replace the `<ul>` block in the template with:

```svelte
  {:else}
    <div class="ts-list">
      {#each bands as { key, label, entries: bandEntries } (key)}
        {#if bandEntries.length > 0}
          <section data-band={key}>
            <GroupHeading label={label} count={bandEntries.length} />
            <ul role="list" aria-label="{label} entries">
              {#each bandEntries as entry (entry.id)}
                <li role="listitem">
                  <EntryRow {entry} feed={feedFor(entry.subscription_id)} />
                </li>
              {/each}
            </ul>
          </section>
        {/if}
      {/each}
    </div>
```

And derive `bands`:

```ts
  const bands = $derived(() => {
    const b = bucketByDay(items);
    return [
      { key: 'today',     label: 'Today',     entries: b.today },
      { key: 'yesterday', label: 'Yesterday', entries: b.yesterday },
      { key: 'thisWeek',  label: 'This week', entries: b.thisWeek },
      { key: 'earlier',   label: 'Earlier',   entries: b.earlier },
    ];
  });
```

**Important:** `bands` must be `$derived` of `items` so re-loads and pagination append cleanly without recomputing manually.

**ARIA note.** The previous `<ul role="list">` at the top level is removed; the test from Task 3 (`'renders one list item per entry returned'`) now needs to find the `<ul>` inside *any* `<section data-band>`. Update that older test's `querySelector('ul[role="list"]')` to `querySelectorAll('ul[role="list"]')` and the assertion to `expect(container.querySelectorAll('li[role="listitem"]')).toHaveLength(2)` — `li`s aggregate across sections.

Also: drop the `file-level` `vi.mock('../../components/GroupHeading.svelte', ...)` from the test file so the real `GroupHeading` renders. It is a primitive owned by M1 / M2 and is safe to render in tests. `EntryRow` stays mocked because it pulls in `swipe` and other browser dependencies that don't matter for History-level assertions.

- [ ] **Step 4: Run all tests and confirm green**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts`
Expected: PASS — 5 tests.

- [ ] **Step 5: Add a test that empty bands are skipped**

Append:

```ts
  it('omits day-band sections that have no entries', async () => {
    const now = Math.floor(Date.now() / 1000);
    const day = 86400;
    vi.spyOn(api, 'listEntries').mockResolvedValueOnce({
      data: [
        { id: 1, subscription_id: 1, title: 'only today', url: 'https://x/1', published_at: now - 60, fetched_at: 1, read: true, saved: false, extract_failed: false },
        { id: 2, subscription_id: 1, title: 'only earlier', url: 'https://x/2', published_at: now - 30 * day, fetched_at: 1, read: true, saved: false, extract_failed: false },
      ],
      next_cursor: undefined,
    } as any);

    const { container } = render(History);
    await waitFor(() => expect(container.querySelector('section[data-band]')).toBeTruthy());

    const bandKeys = Array.from(container.querySelectorAll('section[data-band]'))
      .map(el => el.getAttribute('data-band'));
    expect(bandKeys).toEqual(['today', 'earlier']);
  });
```

- [ ] **Step 6: Run and confirm green**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts`
Expected: PASS — 6 tests. The `{#if bandEntries.length > 0}` guard handles this.

- [ ] **Step 7: Commit**

```bash
git add web/src/views/History.svelte web/src/views/__tests__/History.test.ts
git commit -m "M-Redesign-8: group history entries by day-band"
```

---

### Task 5: Cursor pagination via a "Load more" button

**TDD:** Required. Two tests: (a) the Load-more button renders only when `next_cursor` is non-empty, (b) clicking it appends entries and consumes the cursor.

**Files:**
- Modify: `web/src/views/History.svelte`
- Modify: `web/src/views/__tests__/History.test.ts`

- [ ] **Step 1: Write the failing test for cursor visibility**

```ts
  it('shows a Load more button only when next_cursor is present', async () => {
    const now = Math.floor(Date.now() / 1000);
    vi.spyOn(api, 'listEntries').mockResolvedValueOnce({
      data: [{ id: 1, subscription_id: 1, title: 't', url: 'https://x/1', published_at: now - 60, fetched_at: 1, read: true, saved: false, extract_failed: false }],
      next_cursor: 'CURSOR-1',
    } as any);
    const { container } = render(History);
    await waitFor(() => expect(container.querySelector('section[data-band]')).toBeTruthy());
    expect(container.querySelector('[data-action="load-more"]')).toBeTruthy();
  });

  it('hides Load more when next_cursor is undefined', async () => {
    const now = Math.floor(Date.now() / 1000);
    vi.spyOn(api, 'listEntries').mockResolvedValueOnce({
      data: [{ id: 1, subscription_id: 1, title: 't', url: 'https://x/1', published_at: now - 60, fetched_at: 1, read: true, saved: false, extract_failed: false }],
      next_cursor: undefined,
    } as any);
    const { container } = render(History);
    await waitFor(() => expect(container.querySelector('section[data-band]')).toBeTruthy());
    expect(container.querySelector('[data-action="load-more"]')).toBeFalsy();
  });
```

- [ ] **Step 2: Run and confirm both fail**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts -t "Load more"`
Expected: FAIL — no Load-more button yet.

- [ ] **Step 3: Add the Load-more button to History.svelte**

In `<script>`:

```ts
  let loadingMore = $state(false);

  async function loadMore() {
    if (!cursor || loadingMore) return;
    loadingMore = true;
    try {
      const r = await api.listEntries({ limit: 100, cursor });
      items = [...items, ...r.data];
      cursor = r.next_cursor;
    } catch (e) {
      error = (e as Error).message;
    } finally {
      loadingMore = false;
    }
  }
```

**Precondition.** This task consumes the M1-owned `Button.svelte` primitive. M1 must deliver `Button.svelte` with:

1. `onclick` (lowercase, Svelte 5 event-attribute form) prop wired to the inner `<button>`.
2. `disabled` prop wired to the inner `<button>`.
3. A `quiet` variant (per `docs/specs/2026-05-11-ui-redesign.md` §3.2 primitive table — "default / primary / accent / danger / quiet").
4. Pass-through of arbitrary `data-*` attributes onto the inner `<button>` so the test can locate it by `[data-action="load-more"]`.

If any of those four are missing when M8 runs, raise the gap with planner-m1 before writing this step. **Do not** fall back to a raw `<button class="ts-btn">` in History; primitive duplication is the failure mode we're avoiding.

Add the import at the top of `<script>`:

```ts
  import Button from '../components/Button.svelte';
```

In the template, after the bands block (still inside `{:else}`):

```svelte
      {#if cursor}
        <div class="load-more-row">
          <Button
            quiet
            data-action="load-more"
            disabled={loadingMore}
            onclick={loadMore}
          >
            {loadingMore ? 'Loading…' : 'Load more'}
          </Button>
        </div>
      {/if}
```

Add the row style:

```css
  .load-more-row { display: flex; justify-content: center; padding: 24px 0; }
```

- [ ] **Step 4: Run and confirm both tests pass**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts -t "Load more"`
Expected: PASS.

- [ ] **Step 5: Write the failing test for "click Load more appends entries"**

```ts
  it('appends entries and consumes the cursor when Load more is clicked', async () => {
    const now = Math.floor(Date.now() / 1000);
    const day = 86400;
    const spy = vi.spyOn(api, 'listEntries')
      .mockResolvedValueOnce({
        data: [{ id: 1, subscription_id: 1, title: 'page1-today', url: 'https://x/1', published_at: now - 60, fetched_at: 1, read: true, saved: false, extract_failed: false }],
        next_cursor: 'CURSOR-1',
      } as any)
      .mockResolvedValueOnce({
        data: [{ id: 2, subscription_id: 1, title: 'page2-earlier', url: 'https://x/2', published_at: now - 30 * day, fetched_at: 1, read: true, saved: false, extract_failed: false }],
        next_cursor: undefined,
      } as any);

    const { container } = render(History);
    await waitFor(() => expect(container.querySelector('[data-action="load-more"]')).toBeTruthy());

    (container.querySelector('[data-action="load-more"]') as HTMLButtonElement).click();

    await waitFor(() => expect(spy).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(container.querySelector('[data-action="load-more"]')).toBeFalsy());

    expect(spy.mock.calls[1][0]).toEqual(expect.objectContaining({ cursor: 'CURSOR-1', limit: 100 }));
    expect(container.querySelectorAll('li[role="listitem"]')).toHaveLength(2);
  });
```

- [ ] **Step 6: Run the test and confirm it passes**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts -t "appends entries"`
Expected: PASS.

If the `li` count is `0` because the EntryRow mock has been broken by a prior change, the test still asserts the right behaviour — the `li[role="listitem"]` wrapper is in History.svelte itself, not inside EntryRow, so it should not depend on the mock. If you do hit a `0`, check that the `{#each}` is wrapping `<li role="listitem">` and not nested wrongly.

- [ ] **Step 7: Commit**

```bash
git add web/src/views/History.svelte web/src/views/__tests__/History.test.ts
git commit -m "M-Redesign-8: paginate history via Load more"
```

---

### Task 6: Replace the M1 stub registration with the real History view

**TDD:** Not required — this is a single import swap. The Task-1-through-5 tests already prove the real view works; once it's mounted, `pnpm --dir web run check` catches a wrong path.

**Files:**
- Modify: the single file where M1's foundations plan registered the `/history` stub. The most likely locations are `web/src/App.svelte` or whatever foundations-introduced route-dispatch component (e.g. `web/src/lib/router.ts` only handles parsing — the dispatch lives in `App.svelte` based on the existing patterns in `views/Saved.svelte` and `views/Unread.svelte`). Inspect M1's diff before touching anything.

- [ ] **Step 1: Locate the stub registration**

Run: `git grep -n "history" web/src/App.svelte web/src/lib/router.ts`
Expected output: a line in `App.svelte` that mounts a stub component for the `history` route (per M1's plan, an `EmptyState` reading "Coming soon — M8").

If `App.svelte` uses a `{#if $route.name === 'history'}` block with an inline stub component, replace the inline content with `<History />`.
If M1 introduced a `views/HistoryStub.svelte`, swap the import to `views/History.svelte`.

- [ ] **Step 2: Swap the import**

In `web/src/App.svelte`, change:

```svelte
  import HistoryStub from './views/HistoryStub.svelte';
```

to:

```svelte
  import History from './views/History.svelte';
```

and update the corresponding route block. If M1's structure differs, mirror the analogous change.

- [ ] **Step 3: Run svelte-check to verify the import resolves cleanly**

Run: `pnpm --dir web run check`
Expected: 0 errors related to History.

- [ ] **Step 4: Run the full test suite to verify nothing else regressed**

Run: `pnpm --dir web test`
Expected: PASS across the board.

- [ ] **Step 5: Manually smoke-test the route**

This is a feature change in the UI, so visual verification matters before claiming complete:

```bash
make dev
```

Then visit `http://localhost:5173/history` and confirm:
- desktop top tab "History" lights up (M1 wires this),
- the page loads under `.ts-shell`,
- entries are grouped by day-band in the expected order,
- "Load more" appears iff the API returns a `next_cursor`,
- empty-state copy reads "No history yet" (or whatever the final copy lands at — see Acceptance criteria),
- mobile (≤768px viewport): the page is reachable from the More sheet's `History` row, the mobile top-bar shows "History" as the page title, and the rows scale down to mobile-entry padding per `styles.css` lines 1772–1783.

- [ ] **Step 6: Commit**

```bash
git add web/src/App.svelte
git commit -m "M-Redesign-8: mount real History view in place of M1 stub"
```

---

### Task 7: Polish the empty state and copy

**TDD:** Behaviour-light; one DOM-presence test.

**Files:**
- Modify: `web/src/views/History.svelte`
- Modify: `web/src/views/__tests__/History.test.ts`

The brand spec §6.3 (Saved) sets the empty-state pattern: accent dot, serif title, sans sub. History reuses the M1 `EmptyState` primitive.

- [ ] **Step 1: Drop the EmptyState mock and tighten the existing empty-state test**

First, **un-mock `EmptyState`** at the top of `web/src/views/__tests__/History.test.ts`. From Task 1 step 1 the file-level mock block reads:

```ts
vi.mock('../../components/EntryRow.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/GroupHeading.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/EmptyState.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/Button.svelte', () => ({ default: vi.fn() }));
```

Remove the `EmptyState` line. (Task 4 step 3 already removed the `GroupHeading` mock for the same reason; this is the parallel move for the same reason — these are M1-owned primitives whose real rendering is the canonical source of truth in DOM assertions. Reading `mock.calls[0][1].props` against a Svelte 5 `vi.fn()`-mocked default export is not a supported pattern in this repo — see `Reader.test.ts`, `UnreadMarkAll.test.ts` for the convention: mocks are for *isolation*, assertions read the parent's own rendered DOM.)

Then strengthen the empty-state test originally written in Task 1 step 7. Replace it with:

```ts
  it('renders the empty-state copy when there is no history', async () => {
    vi.spyOn(api, 'listEntries').mockResolvedValueOnce({ data: [], next_cursor: undefined } as any);
    render(History);
    expect(await screen.findByText(/No history yet/i)).toBeTruthy();
    expect(await screen.findByText(/Subscribed feeds will accumulate/i)).toBeTruthy();
  });
```

This asserts against the real `EmptyState` rendering — both the title and the sub. No mock-call introspection.

- [ ] **Step 2: Run and confirm the existing test fails on the new sub-copy assertion**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts -t "empty-state"`
Expected: FAIL — the current empty branch is `<p class="status empty">No history yet.</p>` (from Task 1 step 3), which does not render the sub-copy text and does not use `EmptyState`. The title-match may pass on the literal string in the `<p>`, but the sub-match will fail. Either way, the test now requires the view to consume `EmptyState`.

- [ ] **Step 3: Switch the empty branch to use the EmptyState primitive**

In `web/src/views/History.svelte`, add the import:

```svelte
  import EmptyState from '../components/EmptyState.svelte';
```

Replace the empty branch:

```svelte
  {:else if items.length === 0}
    <EmptyState
      title="No history yet"
      sub="Subscribed feeds will accumulate here as they're polled. Come back after the next poll."
    />
```

(The sub copy is concrete and short, matches the design's voice — see brand spec §1.3. Adjust at review only if the reviewer prefers different copy; if you change it, update the regex in Step 1's test to match the new wording.)

**Precondition note:** this assumes M1's `EmptyState.svelte` accepts `title` and `sub` props and renders them as visible text. Confirm by reading `web/src/components/EmptyState.svelte` before this step. If M1 named the props differently (e.g. `heading` / `body` / `description`), use M1's names and update the regex assertions to match the rendered text — the assertion shape (text match via `findByText`) does not change.

- [ ] **Step 4: Run all tests for the file and confirm green**

Run: `pnpm --dir web test -- src/views/__tests__/History.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/History.svelte web/src/views/__tests__/History.test.ts
git commit -m "M-Redesign-8: use EmptyState primitive for History empty branch"
```

---

### Task 8: Final verification

- [ ] **Step 1: Full test suite green**

Run: `pnpm --dir web test`
Expected: all suites pass, including unmodified neighbours (Saved, Unread, Reader, Settings, etc.).

- [ ] **Step 2: Type check green**

Run: `pnpm --dir web run check`
Expected: 0 errors, 0 warnings related to `views/History.svelte` or `views/__tests__/History.test.ts`.

- [ ] **Step 3: Go test suite green (web/dist must be rebuilt because SPAHandler asserts on it)**

Run: `make test`
Expected: PASS across `go test ./... -race`. This is required because the Make target also rebuilds `web/dist`, which is what `internal/server.SPAHandler` validates at construction time per `CLAUDE.md` "Build coupling."

- [ ] **Step 4: Manual smoke on desktop and mobile viewports**

`make dev`, then exercise:

| Scenario | Expected |
|---|---|
| `/history` on desktop, fresh user with zero entries | EmptyState reads "No history yet" inside `.ts-shell` |
| `/history` with a few entries spanning Today + Earlier | Two `.ts-group-heading` rules, entries in the right band, newest first within each band |
| `/history` with > 100 entries (cursor present) | "Load more" appears below the last band; clicking appends a second page and the button vanishes when `next_cursor` is undefined |
| Slow network (DevTools throttling) | "Loading…" copy visible until the first page arrives |
| API 500 (simulate via DevTools or with a one-shot stub) | Error message renders in place |
| Mobile viewport (≤768px) | Reachable via More sheet, page title "History" in mobile top bar, entries use mobile padding |
| Theme switch (light/dark/sepia) | Colours invert correctly via the foundations theme tokens; no hard-coded greys |

- [ ] **Step 5: Push and open PR**

PR is opened separately from the plan PR — execution of this plan happens on a different branch.

---

## Acceptance criteria

1. Visiting `/history` (desktop or mobile) shows a flat chronological list of every entry the server returns, grouped into Today / Yesterday / This week / Earlier bands.
2. No backend changes — the only network call is `GET /api/v1/entries?limit=100` (and `&cursor=…` for subsequent pages).
3. Empty state ("No history yet") renders when the API returns zero entries on the first page.
4. Error state renders when the API call rejects.
5. "Load more" pagination is functional: clicking it appends the next page and consumes the cursor; the button disappears when `next_cursor` is undefined.
6. Mobile parity: the route is reachable from the More sheet, the page reuses the mobile shell, and entry rows use mobile padding.
7. All three themes (light, dark, sepia) render correctly; no hard-coded colours outside the `var(--…)` token system.
8. `pnpm --dir web test` passes (no regressions in other view tests).
9. `pnpm --dir web run check` passes with no new errors.
10. `make test` passes (Go suite still green after rebuilding `web/dist`).
11. The `bucketByDay` helper is imported from `web/src/lib/dayBands.ts` (M2's location). No private copy of the helper exists in `views/History.svelte`.

## Verification commands

Single-file test loop while developing:

```bash
pnpm --dir web test -- src/views/__tests__/History.test.ts --watch
```

Final pre-PR sweep:

```bash
pnpm --dir web run check
pnpm --dir web test
make test
make dev   # manual smoke
```

## Risks

1. **`bucketByDay` location and signature drift from M2.** M2 owns the helper. If M2 lands later than M8 or names/places it differently, M8 cannot import it cleanly.
   - *Mitigation*: gate Task 4 on confirming M2's plan (or merged code) before writing imports. If absolutely necessary to land M8 first, vendor a temporary `bucketByDay` inside `views/History.svelte` itself with a `// TODO(M2): consolidate into lib/dayBands.ts` marker and a tracked follow-up task; do not export it. The implementer must surface this in the PR description so the M2 reviewer catches the duplicate.
   - *Signature*: this plan assumes `bucketByDay(entries, now?: number) => { today, yesterday, thisWeek, earlier }`. If M2's signature is `bucketByDay(entries)` only, drop the `now` parameter — that's fine; the helper uses `Date.now()` internally.

2. **The M1 stub registration shape isn't determined yet.** Task 6 swaps the stub for the real component, but the exact swap depends on whether M1's foundations plan introduces a `views/HistoryStub.svelte` file or inlines the stub in `App.svelte`.
   - *Mitigation*: Task 6 includes a `git grep` step to locate the registration before editing. Worst case is a five-line edit; nothing about M8 depends on the specific approach M1 picked.

3. **Pagination UX with very deep history.** "Load more" is fine for the user counts Tap targets (single-user self-host, typically a few thousand entries lifetime). At very high cardinality (tens of thousands), a virtualised list would render faster.
   - *Mitigation*: defer until a real user reports a problem. Adding virtualisation is straightforward later; "Load more" doesn't paint us into a corner.

4. **Server-side ordering assumption.** This plan assumes `/api/v1/entries` returns entries newest-first when no filter is set, per the umbrella spec §2.4. If that assumption breaks (e.g. a future API change reorders results), bands will appear in the wrong sequence inside themselves.
   - *Mitigation*: rely on the umbrella spec, do not re-implement sorting client-side. If the assumption ever breaks, fix it server-side; do not paper over it in the view.

5. **No keyboard navigation in History.** The brand spec §7 lists shortcuts (`J`/`K`, etc.), but they're scoped to Unread + Reader and were already handled by M2. History reuses the M1 shell and inherits whatever global key dispatch the shell provides; this plan does not add History-specific shortcuts.
   - *Mitigation*: if a future milestone adds `G H` (Go to History), it slots into the existing keyboard handler in M1.

6. **`subscriptions` store coupling.** History uses `feedFor(subscription_id)` to enrich entry rows with feed metadata. If `subscriptions.load()` fails silently (which it does today, per `web/src/lib/store.ts:84–91`), some rows will render without a feed name. This is consistent with `views/Unread.svelte`'s behaviour — not a regression.
   - *Mitigation*: nothing extra; the silent-failure pattern is already the project convention.

---

## Self-review notes

- **Spec coverage:** §2.2 routing — covered (Task 6). §2.4 no-backend — covered (Task 2 pins the call shape). §3.2 primitives — only reuses listed primitives (Task 1 / 7). §6 testing posture — load / error / empty / pagination / day-band ordering each have a red-first test (Tasks 1, 2, 4, 5, 7). Brand spec §6.3 empty-state pattern — covered (Task 7). §6.8 mobile shell — covered by reuse of M1 (Task 6 manual smoke).
- **Placeholder scan:** no TBDs, no "add appropriate error handling," every code step shows the actual code. The only conditional ("if M1 inlined the stub vs. created `HistoryStub.svelte`") is bounded by the `git grep` step in Task 6.
- **Type consistency:** `EntryListItem`, `Subscription`, `ListResponse<T>` are read directly from `web/src/lib/types.ts:36–56`. `next_cursor` (snake_case) matches the type. `bucketByDay` returns `{ today, yesterday, thisWeek, earlier }` consistently in Task 4's `$derived` and the test data.
