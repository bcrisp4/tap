# M5 — Article extraction

**Status:** Draft, awaiting review.

## Context

Tap is the self-hosted feed reader described in [`../concept.md`](../concept.md), with the visual identity and detailed design in [`../../ui_design/`](../../ui_design/). M5 is the fifth of twelve milestones — see [`../roadmap.md`](../roadmap.md). The walking skeleton from M1 ([`2026-05-08-m1-walking-skeleton.md`](2026-05-08-m1-walking-skeleton.md)), the sanitisation pipeline from M2 ([`2026-05-08-m2-sanitisation.md`](2026-05-08-m2-sanitisation.md)), the media proxy from M3 ([`2026-05-09-m3-media-proxy.md`](2026-05-09-m3-media-proxy.md)), and the polling discipline from M4 ([`2026-05-09-m4-polling-discipline.md`](2026-05-09-m4-polling-discipline.md)) are shipped: feeds are polled on an adaptive cadence through a single shared HTTP client with an SSRF guard and per-host limiter, entries are sanitised on insert, and `<img>` URLs route through the internal media proxy.

What M4 leaves on the table for M5 is the link-only feed problem. A subscription to Hacker News, Substack's "weekly roundup" digest, or any RSS feed that publishes link-only summaries renders in Tap as a wall of one-paragraph teasers — useless as a reading surface. Concept §6.5 specifies the closure: an opt-in per-subscription flag that makes the worker fetch each new entry's article URL and run it through a Readability-style extractor, with optional per-feed CSS selector overrides, and graceful degradation to the feed-provided summary on per-entry failure.

The shape of the close is small. The worker gains one new phase between item-iteration and the existing sanitise/image-rewrite pass; a new leaf package wraps the extractor; the subscriptions table gains two columns and the entries table gains one; the API gains a narrow `PATCH /api/v1/subscriptions/:id` for toggling the flag and setting the selector. Outbound article fetches go through the M4 shared client unmodified, so SSRF, per-host concurrency, and `--http-timeout` apply for free.

## Goal

After M5, a subscription created with `{"feed_url": "...", "extract": true}` polls a link-only feed and persists each entry's extracted article body, sanitised, with `<img>` URLs routed through the M3 proxy. A subscription without `extract` keeps M4 behaviour exactly. A `PATCH /api/v1/subscriptions/:id` with `{"extract_selector": ".article-body"}` overrides Readability's heuristic for one feed where it picks the wrong sub-tree. A poll that successfully extracts 30 articles takes ≤ ~10s wall-clock at default `--extract-concurrency=4`. An article fetch that fails (timeout, non-2xx, non-HTML, body cap exceeded, SSRF reject) commits the entry with the feed-provided summary and `extract_failed = 1`; the failure does not abort the poll, does not increment the subscription's `error_count`, and is logged once at WARN. Toggling extract from off→on affects future polls only — existing entries are not retroactively re-extracted.

## In scope

### New package: `internal/extract`

```
internal/extract/
  extract.go      # Extract(ctx, client, articleURL, selector, bodyCap) (string, error)
  extract_test.go
```

One entry point:

```go
func Extract(ctx context.Context, client *http.Client,
             articleURL, selector string, bodyCap int64) (string, error)
```

Selector empty → Readability mode: fetch, `io.LimitReader` cap at `bodyCap`, hand to `readability.FromReader(body, parsedURL)`, render with `Article.RenderHTML`. Selector set → fetch, parse with `golang.org/x/net/html`, run `cascadia.Compile(selector).MatchFirst(doc)`, render the matched node with `html.Render`.

Errors on: HTTP non-2xx, body cap exceeded (LimitReader + a length check on the read result), missing or non-HTML `Content-Type` (`text/html` family only — `application/xhtml+xml` is accepted; everything else rejected), malformed selector, selector matched no node, Readability returned empty content. The fetch uses the caller-supplied `*http.Client` — production wires the shared M4 client; unit tests pass `httptest.Server` clients. No internal HTTP client construction, no internal SSRF/timeout/per-host logic — those are the shared client's job.

Two new dependencies in `go.mod`:

| Module | Purpose | License |
|---|---|---|
| `codeberg.org/readeck/go-readability/v2` | Mozilla Readability port (the maintained successor to the archived `go-shiori/go-readability`). Pure Go, no CGO. | MIT |
| `github.com/andybalholm/cascadia` | Pure-Go CSS selector engine. Lighter than pulling in goquery for one `MatchFirst` call. | BSD-2-Clause |

### Schema migration

`internal/db/migrations/0004_extraction.sql`:

```sql
ALTER TABLE subscriptions ADD COLUMN extract           INTEGER NOT NULL DEFAULT 0;
ALTER TABLE subscriptions ADD COLUMN extract_selector  TEXT    NOT NULL DEFAULT '';
ALTER TABLE entries       ADD COLUMN extract_failed    INTEGER NOT NULL DEFAULT 0;
```

SQLite stores booleans as INTEGER. `ALTER TABLE … ADD COLUMN … DEFAULT` is in-place and safe per the same notes M4 made about `velocity_24h_x100`. Existing M4 subscriptions start with `extract = 0` (no behaviour change). Existing entries get `extract_failed = 0` retroactively — the column is meaningful only for entries inserted after a subscription was opted into extraction.

### `db` layer extensions

```go
type DueSubscription struct {
    // ... existing M4 fields
    Extract         bool
    ExtractSelector string
}

type NewEntry struct {
    // ... existing M4 fields
    ExtractFailed bool
}
```

`db.ListDuePolls` updates its SELECT to include `extract` and `extract_selector`. `db.UpdateAfterPoll`'s entry-insert statement writes `extract_failed`. No new helpers — the worker reads `Extract` + `ExtractSelector` off the `DueSubscription` it already receives, and writes `ExtractFailed` through `NewEntry`.

A small new `db.UpdateSubscriptionExtraction(ctx, db, id, extract, selector)` helper backs the PATCH endpoint. Single UPDATE statement; returns `sql.ErrNoRows` if the row is missing so the API layer can map to 404.

### Worker integration

`poll.WorkerOpts` gains:

```go
type ExtractFunc func(ctx context.Context, client *http.Client,
                     articleURL, selector string, bodyCap int64) (string, error)

type WorkerOpts struct {
    // ... existing M4 fields unchanged
    Extract            ExtractFunc   // default = extract.Extract; test seam
    ExtractConcurrency int           // default 4
    ExtractBodyCap     int64         // default 5 MiB
}
```

The `Extract` field is the same shape of test seam as `Now func() time.Time`: production wires `extract.Extract`, tests inject deterministic in-memory extractors without spinning up an httptest origin. Mirrored on `SchedulerOpts` and propagated into `NewWorker`.

`Worker.Run` post-fetch (success path) gets one new phase between item-iteration and the existing per-item `processor.Process` pass:

```go
type pending struct {
    item          *gofeed.Item
    content       string  // raw, pre-process
    extractFailed bool
}

pendings := make([]pending, len(res.Feed.Items))
for i, item := range res.Feed.Items {
    raw := item.Content
    if raw == "" { raw = item.Description }
    pendings[i] = pending{item: item, content: raw}
}

if sub.Extract && len(pendings) > 0 {
    g := new(errgroup.Group)
    g.SetLimit(w.opts.ExtractConcurrency)
    for i := range pendings {
        if pendings[i].item.Link == "" {
            continue // no URL → skip silently; extract_failed stays 0
        }
        g.Go(func() error {
            extracted, err := w.opts.Extract(ctx, w.client,
                pendings[i].item.Link, sub.ExtractSelector, w.opts.ExtractBodyCap)
            if err != nil {
                slog.WarnContext(ctx, "extract failed",
                    "feed_id", sub.ID,
                    "entry_url", pendings[i].item.Link,
                    "err", err)
                pendings[i].extractFailed = true
                return nil // never propagate; per-entry failures are isolated per concept §6.5
            }
            pendings[i].content = extracted
            return nil
        })
    }
    _ = g.Wait()
}

// Existing per-item processor.Process + db.NewEntry assembly proceeds unchanged,
// except NewEntry.ExtractFailed = pendings[i].extractFailed.
```

Three things worth being explicit about:

- **No-URL entries** (`item.Link == ""`) skip extraction silently and keep `extract_failed = 0`. The flag means "extraction tried and failed," not "the body is a summary." A no-URL entry was never going to be extracted; tagging it failed would be misleading.
- **Race safety:** each goroutine writes to a distinct `pendings[i]` element. No shared writes. The `errgroup` never sees a non-nil error (we swallow per-entry failures by contract), so `g.Wait()`'s return value is unused.
- **Single-transaction commit per poll (concept §6.15) still holds.** The whole batch lands together. Worst case: 4-wide concurrency × N batches × 30s `--http-timeout` per request — bounded but potentially long for a 100-entry aggregator poll. The Risks section flags this; the scheduler's worker pool keeps other feeds moving in parallel.

The 304 path and the error path are unchanged from M4 — extraction only runs on the 200/successful-parse path because that's the only path with new entries.

### API extensions

All under `/api/v1/`. Per the project conventions, every wire shape is an explicit DTO in `internal/api/`; never return `db.*` types directly. Errors via `writeError(w, status, code, message)` from `internal/api/errors.go`. Request bodies remain `http.MaxBytesReader`-capped at 1 MiB.

- **`POST /api/v1/subscriptions`** — gains an optional `extract` bool field on the request DTO. Default `false`. Subscribe-time-only setting of `extract_selector` is **not** supported (the user won't know the right selector before they've seen the feed render); they PATCH later.

- **`PATCH /api/v1/subscriptions/:id`** — new endpoint. Accepts `{extract?: bool, extract_selector?: string}`. Omitted field = no change. An empty-string `extract_selector` clears the override (next poll uses Readability). Returns 200 + the updated subscription DTO.
  - 404 if the row doesn't exist (`db.ErrNoRows` from `UpdateSubscriptionExtraction`).
  - 400 if the body is not valid JSON or the field types are wrong.
  - 400 with stable code `extract_selector_invalid` if `extract_selector` is non-empty and fails `cascadia.Compile`. Validating at PATCH time gives the user immediate feedback rather than a silent extraction failure on next poll.
  - 413 if the body exceeds the 1 MiB cap (Go's `MaxBytesReader` semantics).

  PATCH does **not** call `Scheduler.Poke()`. Toggling extract on doesn't change anything until a poll surfaces *new* entries; the next normal poll is at most one cadence-floor away (15min default) and is typically a 304, so a Poke would do no useful work. Keeping the surface narrow.

- **`GET /api/v1/subscriptions`** and the corresponding read DTO grow `extract` and `extract_selector` fields, so the SPA (now and in M8 polish) can render the toggle state.

- **Entries read DTO** grows `extract_failed`. Cheap forward-compat for an M8 SPA affordance ("Extraction failed; showing feed summary"). The list endpoint already strips the body to keep payloads small (concept §5); `extract_failed` is small enough to include in the list shape too.

The PATCH handler is mounted in `internal/api/api.go`'s `NewMux`. Method routing on `/api/v1/subscriptions/:id`: M5 only handles PATCH; GET-by-id and DELETE-by-id are not in scope (no current SPA caller). 405 for unsupported methods on the path.

### Configuration knobs

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--extract-concurrency` | `TAP_EXTRACT_CONCURRENCY` | `4` | Per-worker `errgroup.SetLimit` on parallel article fetches. M4's per-host cap further serialises same-host bursts. |
| `--extract-body-cap` | `TAP_EXTRACT_BODY_CAP` | `5242880` (5 MiB) | `io.LimitReader` cap on each article response before the parser sees it. |

The article fetch reuses the shared client's `--http-timeout` (30s default). No separate `--extract-timeout` — one knob, the global per-request deadline. If a real deployment hits a feed where 30s is too tight or too loose for articles specifically, we add `--extract-timeout` then.

### `cmd/tap/main.go` wire-up

The new flags parse in the existing flag block. The defaults flow through to `SchedulerOpts`:

```go
sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
    Processor:          proc,
    Floor:              *pollFloor,
    Ceiling:            *pollCeiling,
    ErrorBase:          *pollErrorBase,
    ExtractConcurrency: *extractConcurrency,
    ExtractBodyCap:     *extractBodyCap,
    // Extract field is left zero so NewWorker defaults it to extract.Extract
})
```

`NewWorker` defaults a nil `Extract` field to `extract.Extract`. No changes to the shared HTTP client construction — it's the same `httpx.NewClient(...)` from M4 used for both polling and the proxy, and now also for article extraction.

### README update

A new section after M4's "Trust posture":

> **Article extraction (M5).** Subscriptions can be flagged for full-article extraction (`POST /api/v1/subscriptions {..., "extract": true}` or `PATCH /api/v1/subscriptions/:id`). When enabled, the worker fetches each new entry's article URL through the shared HTTP client (so the M4 SSRF guard, per-host cap, and `--http-timeout` apply) and runs the response through Readability — or, if the subscription has an `extract_selector` CSS rule set, through that selector. Extracted HTML flows through the same M2 sanitiser and M3 image proxy as feed-provided HTML. Per-entry extraction failures degrade to the feed-provided summary with `extract_failed = 1`; they never abort the poll or count against the subscription's error budget. Toggling extract from off→on affects future polls only — existing entries are not re-fetched.

Plus an upgrade note: "Migration 0004 adds three columns (`subscriptions.extract`, `subscriptions.extract_selector`, `entries.extract_failed`); all default to off/empty. Existing M4 databases migrate cleanly."

### Tests and methodology

M5 follows the same test-first discipline as M1–M4 (`docs/roadmap.md` §"Working cadence"). Pure scaffolding (the migration SQL file, the README edit, the flag declarations) is exempt; everything with branches, error handling, or state is in scope.

Concrete test surface:

- **`internal/extract/extract_test.go`**
  - Readability mode: a fixture multi-paragraph HTML page → output contains the article body, drops nav/header/footer/sidebar.
  - Selector mode: fixture HTML with `<div class="article">…</div>` and selector `.article` → just that node's HTML.
  - Errors as distinct tests: HTTP non-2xx (404, 500); body cap exceeded (LimitReader fires); missing `Content-Type`; non-HTML `Content-Type` (`text/plain`, `application/json`); accepted `application/xhtml+xml`; malformed selector; selector matched no node; Readability returned empty content (a page with only nav).
  - Selector with multiple matches → first match returned.
  - Total-function for callers on error: result is `""`, error non-nil; no panics on any input.

- **`internal/poll/worker_test.go`** (extended)
  - `extract=false`: behaviour unchanged from M4; the `Extract` func seam is never called (verified via a counting test double).
  - `extract=true`: `Extract` called once per item with `(item.Link, sub.ExtractSelector, ExtractBodyCap)`; entries inserted with extracted content.
  - One-of-five extraction fails: that entry has `extract_failed=1` + feed summary; others fine; poll succeeds; subscription `error_count` stays 0.
  - All-five extractions fail: all entries land with summaries + `extract_failed=1`; poll succeeds; `error_count` unchanged.
  - Concurrency: `ExtractConcurrency=2` + 5 items → barrier-style fixture (each goroutine blocks on a shared channel until 2 are in flight) asserts at-most-2 in flight.
  - Item with empty `Link`: extract not called; entry inserted with summary; `extract_failed=0`.
  - Sanitisation still runs on extracted output (extracted HTML containing `<script>` comes out scrubbed — composes the existing `processor.Processor`).
  - Image rewriter still runs on extracted output (extracted `<img src="https://example/x.jpg">` comes out as a proxy URL).

- **`internal/db/subscriptions_test.go`** (extended)
  - Migration 0004 applied: three new columns exist with their defaults.
  - `ListDuePolls` returns `Extract` + `ExtractSelector` populated from row state.
  - `UpdateAfterPoll` writes `extract_failed` per entry (tested with mixed true/false in one batch).
  - `UpdateSubscriptionExtraction` updates both fields atomically; returns `sql.ErrNoRows` on missing id; round-trips through `ListDuePolls`.

- **`internal/api/subscriptions_test.go`** (extended)
  - POST `{extract: true}` persists; POST without `extract` defaults to off; POST `{extract: false}` explicit off.
  - PATCH `{extract: true}` flips the flag; subsequent GET reflects it.
  - PATCH `{extract_selector: ".article-body"}` sets the selector.
  - PATCH `{extract_selector: ""}` clears the selector.
  - PATCH atomic update: `{extract: false, extract_selector: ".x"}` in one round-trip updates both columns.
  - PATCH partial: `{extract: true}` alone doesn't clobber an existing selector; `{extract_selector: ".y"}` alone doesn't clobber the extract flag.
  - PATCH malformed selector → 400, code `extract_selector_invalid`, row unchanged (verified via a follow-up GET).
  - PATCH unknown id → 404; malformed JSON → 400; >1 MiB body → 413; unsupported method (e.g. PUT) → 405.
  - GET subscriptions DTO grows `extract` + `extract_selector`.
  - Entries GET DTO (single + list) grows `extract_failed`.

- **`cmd/tap/main_test.go`** (extended)
  - End-to-end: fixture origin serves a link-only RSS feed pointing at a multi-paragraph HTML page on the same origin; subscribe with `extract=true`; after a poll, `GET /api/v1/entries/:id` returns the extracted body (sanitised, image-rewritten through the proxy URL pattern).
  - End-to-end: article URL returns 500 → entry has `extract_failed=true` + summary; subscription `error_count = 0`.
  - End-to-end: SSRF blocks an article URL resolving to `127.0.0.1` (no allowlist) → `extract_failed=true`, summary used, no SSRF error in the subscription's `last_error` (extraction failure is per-entry, not per-poll).
  - End-to-end: PATCH a subscription to enable extract; the test then calls `Scheduler.Poke()` to avoid waiting a tick, and the resulting poll's new entries are extracted.

## Out of scope (deferred)

| Concern | Lands in |
|---|---|
| Per-feed credentials (cookie, basic-auth, outbound proxy URL) flowing into article fetches | M6 — credential redaction lands then; per-feed overrides ride along |
| SPA UI for the extract toggle, the selector editor, and the `extract_failed` badge | M8 (SPA polish) |
| Discover-feeds-from-URL workflow that auto-suggests `extract=true` for link-only feeds | M9 (add-feed flow) |
| Reading-time estimate column on entries | M8 or later — derivable client-side from the body |
| Bundled starter rule set keyed by hostname (à la miniflux's scraper rules) | Won't ship — per-feed override is the policy. Maintained host lists drift; we'd carry a maintenance lane for marginal benefit. |
| Re-extraction of existing entries when toggling extract from off→on | Won't ship — overwrite-only storage, same posture as M2; affects future polls only. |
| Manual "re-extract this entry" admin action | Won't ship in M5; potential future. |
| Storing the feed summary alongside the extracted body for a "show summary" UX | Won't ship — overwrite-only matches M2 §"Storage." |
| "Extracted shorter than summary → use summary" heuristic | Won't ship — concept §6.5 mandates trust-extraction-when-on; per-feed toggle is the user's escape hatch. |
| Per-feed override of `--extract-concurrency` / `--extract-body-cap` / `--extract-timeout` | Won't ship — global knobs are sufficient. |
| ETag / Last-Modified conditional GET on article fetches | Won't ship — articles are fetched once at insert and never re-fetched. |
| Headless-browser fallback for JavaScript-rendered article pages | Won't ship — `extract_failed` + summary is the documented limitation. Adds a non-static dependency, breaks the distroless image. |
| GET-by-id and DELETE-by-id under `/api/v1/subscriptions/:id` | Not in M5; no current SPA caller. M9 add-feed/edit-feed flow lands them. |

## Risks and open questions

- **Readability returns empty content for a non-trivial fraction of real pages.** Concept-aligned outcome: fall back to summary, set `extract_failed=1`, log at WARN. The user's escape is the per-feed CSS selector. A "per-feed extraction success rate" gauge would be a natural M12 (observability) addition; not in M5.

- **JavaScript-required pages.** We don't run a headless browser. Such pages produce `extract_failed=1` consistently; the user either tunes a selector against the server-rendered shell or turns extract off for that feed. Documented as a limitation.

- **Long extraction batches hold the worker.** Worst case: a 100-entry aggregator poll × 4-wide concurrency × 30s `--http-timeout` ≈ 12.5 min wall-clock for that one feed worker. M4's worker pool keeps other feeds moving in parallel; bumping `--extract-concurrency` is one knob-flip away. Documented; not architected around.

- **CSS selector validation at PATCH time only catches syntax, not match-zero-nodes.** The latter only surfaces at poll time as `extract_failed=1`. Acceptable — we can't dry-run against the live page without fetching it, and PATCH-time fetching would couple the API endpoint to outbound HTTP behaviour.

- **Tracker-redirector article URLs (FeedBurner, Substack click).** Shared client follows up to 10 redirects with per-hop SSRF re-check (M4). Readability sees the final URL as the page URL — correct for image base resolution. No special handling needed.

- **Articles served as `application/xhtml+xml` or with unusual encoding hints.** Readability handles standard HTML; XHTML mostly works because Go's `html` parser is lenient. Edge cases fall back cleanly to summary via `extract_failed=1`.

- **Concurrent extraction holds N HTTP responses open for the duration.** With `--extract-concurrency=4` and M4's per-host cap of 4, at most 4 connections per host at a time, and at most 4 cross-host extractions per worker simultaneously. Comfortable.

- **Single-transaction commit (concept §6.15) blocks on the slowest extraction in the batch.** The errgroup waits for all goroutines before the commit lands. This is the right behaviour — partial commits would leave the subscription in a half-polled state — but worth noting that the commit latency now depends on the longest article fetch, not just the feed parse.

- **PATCH endpoint as a future scope magnet.** M5 keeps it to two fields. Title editing, category assignment, per-feed HTTP overrides each get added in their own milestone (M9, M9, M6 respectively). Resist broadening in M5 even if it feels tempting.

- **Migration 0004 column count.** Three new columns in one migration is fine for SQLite (no per-column ALTER cost worth worrying about). Three concerns landing together because they're all part of the same logical change (extraction metadata).

## Definition of done

1. `make test` passes (`go test ./... -race`) with the new package, the migration, the worker integration, and the API extensions.
2. Subscribing to a link-only feed with `{"extract": true}` produces entries whose `content` is the extracted-then-sanitised article body, not the link summary.
3. Subscribing without `extract` (or with `false`) preserves M4 behaviour exactly — no article fetches, no extra latency, no new test failures elsewhere.
4. A poll where 1-of-5 article fetches fails: all 5 entries land; the failed one has `extract_failed = 1` + feed summary; subscription `error_count` stays 0.
5. `PATCH /api/v1/subscriptions/:id` with `{"extract_selector": ".article-body"}` causes the next poll to use that selector instead of Readability; with `{"extract_selector": ""}` reverts to Readability.
6. PATCH with a malformed selector returns 400 + stable code `extract_selector_invalid`; row is unchanged (verified via follow-up GET).
7. With `--extract-concurrency=2`, a poll with 5 extractable entries shows ≤2 concurrent in-flight `Extract` calls (barrier-style fixture).
8. Article fetches use the shared httpx client: SSRF policy applies (an article URL resolving to a private IP fails extraction unless allowlisted); per-host cap applies; `--http-timeout` applies.
9. Migration 0004 applies cleanly against an M4 database with no data loss.
10. README gains the M5 trust-posture paragraph and the migration upgrade note.
11. `make build` produces a static binary that boots cleanly against a fresh `data/` directory and against an existing M4 database.

## What this milestone deliberately does *not* prove

- That per-feed cookies / basic-auth flow into article fetches — those land with M6 credential redaction.
- That the SPA renders the extract toggle, the selector editor, or the `extract_failed` badge — M8 polish.
- That the add-feed flow auto-suggests `extract=true` for link-only feeds — M9.
- That a feed where Readability mis-extracts is recoverable without manual selector tuning.
- That failed extractions are retried on a future poll — they aren't; articles are fetched once at insert.
- That extracted content survives a re-fetch — we never re-fetch.
- That M11's archival sweep correctly handles extracted entries — orthogonal.
- That extraction success rate is observable per subscription — M12 observability.

If you find yourself adding bundled hostname→selector rules, reading-time estimates, retry logic for failed extractions, headless-browser fallback, or backfill of old entries when extract toggles on, push back. M5's job is the smallest extraction pipeline that closes the link-only-feed gap, with clean seams for M6 (credentials), M8 (SPA polish), and M9 (add-feed flow) to plug into.
