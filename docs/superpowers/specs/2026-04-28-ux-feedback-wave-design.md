# UX Feedback Wave — Design

**Date:** 2026-04-28
**Status:** Approved (brainstorming complete; ready for plan)
**Scope:** Post-v1 UX cleanup spanning settings, sidebar, OPML wiring, per-route copy, reader chrome, unread highlight, history dim, search copy, and the live-update cache layer.

## 1. Motivation

Tap shipped v1 (Plans 00–18, PRs #1–21). User feedback identified rough edges across multiple surfaces, plus one architectural gap: mutations don't propagate to every list cache, so marking an entry read on `/unread` leaves `/history` stale until refresh. This design wraps the visual/copy fixes and the live-update fix into a single coordinated wave.

## 2. Architecture

Two foundational changes enable the rest:

### 2.1 Backend: enriched poller errors

`/api/v1/system/status` currently returns `recent_errors: string[]`. Change to:

```json
{
  "recent_errors": [
    { "feed_id": 17, "feed_title": "Hacker News", "error": "context deadline exceeded", "at": 1714329600 }
  ]
}
```

- Storage: `internal/poller/` ring buffer (still 25 entries) captures `feed.ID`, `feed.Title`, error message, and unix timestamp at the point of failure, instead of flattening to a string.
- Single-binary deployment — no backward-compat needed; bump the response shape directly.
- No new endpoint, no new table.

### 2.2 Frontend: live-update cache layer

Two helpers in a new `web/src/lib/api/cache-patch.ts`:

```ts
function patchEntryEverywhere(qc: QueryClient, id: number, patch: Partial<Entry>): void
function removeEntryEverywhere(qc: QueryClient, predicate: (e: Entry) => boolean): void
```

Both iterate `qc.getQueriesData()` over three root namespaces — `['entries']`, `['history']`, `['search']` — applying the patch/removal to each list's `data.data[]` and adjusting `data.pagination.total` where the change crosses the list's filter (e.g. marking-read in unread drops the row + decrements total).

**Per-mutation wiring** (in `web/src/lib/api/queries.ts`):

| Mutation | Optimistic in `onMutate` | Invalidate in `onSettled` |
|---|---|---|
| `useToggleRead` | `patchEntryEverywhere(id, {read})`; drop+decrement in unread when marking read | `['entries']`, `['history']`, `['search']` |
| `useToggleSaved` | `patchEntryEverywhere(id, {saved})`; drop in `/saved` when unsaving | same |
| `useBulkUpdate` | iterate ids, patch each | same |
| `useDeleteFeed` | `removeEntryEverywhere(e => e.feed_id === id)`; drop `keys.feed(id)` and any `keys.entries({feed_id: id, …})` | `['entries']`, `['history']`, `['search']`, `['feeds']` |

Optimistic patch + invalidation backstop is a TanStack Query idiom — they don't fight: the patched value renders until refetch resolves, and a successful refetch returns the same shape, so no flicker.

**Pagination caveat:** dropping a row from the unread list won't synthesize a new tail entry; the next refetch repaginates. Acceptable.

**Rollback:** capture the cache snapshot in `onMutate`'s `previous` context (matches existing `useToggleRead.onMutate` pattern), restore in `onError`. Show a "couldn't sync" toast on rollback.

**Minimal toast surface** (new — `web/src/lib/components/Toast.svelte` + a tiny `web/src/lib/toast.svelte.ts` rune store with `push(message, kind)` and a 4s auto-dismiss). Mounted once in `+layout.svelte`. Single-message-at-a-time is fine for this wave; a queue can come later if needed. Used by mutation rollbacks (PR #2) and OPML import/export feedback (PR #3).

### 2.3 Offline mutation persistence

Today, `persistQueryClient` (`web/src/lib/query-client.ts`) persists queries but not mutations. A user who marks an entry read offline, then reloads the tab before reconnecting, loses the queued mutation — the optimistic patch survives in IDB but the server write is gone.

Fix:

```ts
persistQueryClient({
  queryClient: client,
  persister,
  maxAge: 7 * 24 * 60 * 60 * 1000,
  dehydrateOptions: {
    shouldDehydrateMutation: m => m.state.isPaused,
  },
});
```

Plus call `client.getMutationCache().resumePausedMutations()` once after rehydration completes (in `+layout.svelte` or wherever `bindOnlineManager()` is wired), so a reload-while-offline → reconnect path drains without requiring a UI interaction.

## 3. UI Surface Changes

### 3.1 Sidebar / menu

`web/src/lib/components/Sidebar.svelte`, `SidebarFooter.svelte`.

- **Settings icon** → Lucide `Settings` (cog). Replace existing sliders SVG.
- **OPML entry point.** New `SidebarFooter` button, sibling to Settings. Icon: Lucide `FolderInput`. On click, opens a small popover with two actions:
  - **Import OPML…** — file picker → `POST /api/v1/opml/import` (multipart). Toast `"Imported N feeds"` on success; error toast with server message on failure.
  - **Export OPML** — `GET /api/v1/opml/export` → browser download `tap-subscriptions.opml`.
- **Per-feed warning.** In the feed list loop, when `feed.error_count > 0`, render a Lucide `AlertTriangle` (~12px, `--accent-warn`) right of the feed title with `title=` set to truncated `last_error`. Click target stays the existing feed-page link.

### 3.2 Settings page

`web/src/routes/settings/+page.svelte`, `web/src/lib/components/StatsPanel.svelte`.

- **Drop the About card.** Side effect: removes the "self-hosted river of unread" copy.
- **GitHub link inline in StatsPanel.** Right-aligned next to the `tap v{version}` heading. Lucide `Github` icon, ~14px, icon-only, `title="View source on GitHub"`, `target="_blank"`, `rel="noreferrer noopener"`.
- **Recent poller errors → linked rows.** Render each error as `[feed title] · {truncated error} · {relative time}`, where the feed title is a link to `/feeds/<feed_id>`. Single-line truncation (`text-overflow: ellipsis`); full error text in `title=`. If `feed_id` doesn't resolve to a current feed, render the title as plain text. If `recent_errors` is empty, hide the section header entirely (currently shows an empty container).

### 3.3 Unread page

`web/src/routes/+page.svelte`, `web/src/lib/components/TopBar.svelte`.

- **No implicit selection until j/k pressed.** Change `selectedId` derivation:
  ```ts
  const selectedId = $derived(
    userSelectedId !== null && visible.some(e => e.id === userSelectedId)
      ? userSelectedId
      : null
  );
  ```
  In the keyboard handler, the first j/k press sets `userSelectedId = visible[0]?.id` directly (first press selects, second press moves). The existing `showKeyboardHighlight = selected && inputMode.mode === 'keyboard'` gate already silences mouse/touch; this change fixes the "first row paints highlighted on initial paint" path.
- **Remove the unread count from TopBar on /unread.** Stop passing `total` from `+page.svelte` to `TopBar`. Audit other `TopBar` consumers (`/feeds/[id]`, `/saved`, `/history`); if the count slot has no remaining consumers, delete it from `TopBar.svelte`.

### 3.4 Feed view

`web/src/routes/feeds/[id]/+page.svelte`.

Empty-state copy: `no entries yet · the poller may still be catching up` → `no entries`. Mono lowercase, matching the file's existing convention (`loading…`, `feed not found` on lines 105/107).

### 3.5 Reader

`web/src/routes/entry/[id]/+page.svelte`, `web/src/lib/components/reader/ReaderBody.svelte`.

- **Remove the collapsed `ReaderRail`** in the desktop branch. Drop the `<ReaderRail entries={…} collapsed onBack={back} />` element and remove the corresponding column from `.reader-shell` layout. The back-to-unread affordance is already in `ReaderHeader` (the `UNREAD` button at line 43). The `ReaderRail` component itself stays in the codebase for any future "siblings inline" feature; just no longer mounted in collapsed mode.
- **Reader content max-width 680 → 820** (`ReaderBody.svelte:111`). At 17px body type that's ~77ch, comfortable for long-form.

### 3.6 History page

`web/src/routes/history/+page.svelte`, `web/src/lib/components/EntryRow.svelte`.

- Add `dimRead?: boolean = true` prop on `EntryRow` (default preserves today's behavior on `/unread`, `/feeds/[id]`, etc.).
- Pass `dimRead={false}` from `/history`'s render.
- Change the `.entry.is-read` rule in `EntryRow.svelte:225-230` to `.entry.is-read:not(.no-dim)`, and apply the `no-dim` class when `dimRead === false`. Read entries on `/history` render at full ink.

### 3.7 Search

`web/src/routes/search/+page.svelte`.

- Placeholder: `search the river…` → `Search posts…` (line 77).
- Internal class names / component names like `RiverList.svelte`, `.river-*` are not user-visible. Out of scope.

## 4. Testing

- **Cache helpers** (`web/src/lib/api/cache-patch.test.ts`, new): seed a `QueryClient` with synthetic `entries`/`history`/`search` lists; run `patchEntryEverywhere`/`removeEntryEverywhere`; assert each list reflects the change and pagination totals adjust. Pure unit, no fetch.
- **Mutation hooks** (extend `web/src/lib/api/queries.test.ts` or create): each of `useToggleRead`, `useToggleSaved`, `useBulkUpdate`, `useDeleteFeed` patches the three namespaces optimistically, calls invalidate on settle, rolls back on error.
- **Offline persistence** (Playwright): `page.context().setOffline(true)`; mark-read on a cached entry; reload; assert entry still shows read; `setOffline(false)`; assert the server receives the PUT (intercept via route handler) and a hard refresh from the server still shows the entry as read.
- **No-implicit-selection** (Playwright): load `/unread`; assert no `.is-selected` row in DOM; press `j`; assert first row gains `.is-selected`.
- **Per-feed warning indicator** (Playwright or unit on Sidebar): seed `useFeeds` with a feed that has `error_count > 0`; assert `AlertTriangle` icon renders with the expected `title=`.
- **Backend**: extend `internal/poller/` and `internal/api/system_test.go` to assert `recent_errors` items have `feed_id`/`feed_title`/`at`/`error`.

## 5. Implementation ordering

Four PRs, sequenced for low coupling:

1. **Backend — enriched poller errors.** Standalone, low-risk. `internal/poller/` ring buffer changes + `internal/api/system.go` response shape + tests. Settings UI keeps rendering raw error strings until PR #3.
2. **Frontend — cache layer + offline-mutation persistence.** `cache-patch.ts`, mutation hook rewires, `query-client.ts` `dehydrateOptions`, `resumePausedMutations()` boot call, toast surface for rollback errors, tests. No visual changes; the rest of the wave depends on these primitives only loosely (per-feed warning + linked errors don't touch them).
3. **Sidebar/settings/OPML.** Settings icon swap, OPML popover + import/export wiring, per-feed warning indicator, settings-page rework (drop About, inline GitHub link, linked poller-error rows). Depends on PR #1 for the linked errors.
4. **Per-route polish.** Unread no-implicit-selection + count removal, feed-view empty-state copy, reader rail removal + max-width bump, history `dimRead`, search placeholder. All visual; share one Playwright run.

PR #2 and PR #3 are independently parallelisable. PR #4 can run in parallel with either after #1 lands.

## 6. Out of scope

- Toast/notification system beyond the minimal "couldn't sync" rollback toast in PR #2. A general toast surface is its own design.
- Renaming internal `RiverList.svelte` / `.river-*` classes.
- Mobile-only adjustments — these fixes target the desktop surfaces the feedback called out. Mobile reuses `EntryRow`, the cache layer, and the offline persistence fix transparently; mobile-specific copy/layout changes are a separate wave.
- Rate-limiting OPML import (single-user app; not a concern at this scale).
