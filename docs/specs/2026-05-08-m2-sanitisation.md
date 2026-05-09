# M2 — Sanitisation pipeline

**Status:** Draft, awaiting review.

## Context

Tap is the self-hosted feed reader described in [`../concept.md`](../concept.md), with the visual identity and detailed design in [`../../ui_design/`](../../ui_design/). M2 is the second of twelve milestones — see [`../roadmap.md`](../roadmap.md). The walking skeleton from M1 ([`2026-05-08-m1-walking-skeleton.md`](2026-05-08-m1-walking-skeleton.md)) is shipped: the binary polls feeds, persists entries, serves them through the SPA, and renders raw HTML straight from the feed with no cleaning pass at all.

That last bit is the M1 trade-off the README warns about — a hostile feed can plant stored XSS in the reader. M2 closes that gap.

The shape of the close: a server-side sanitisation pass between `feed.Fetch()` and the DB commit, producing final-form HTML the SPA can render directly. Per [`../concept.md`](../concept.md) §6.6 the client never runs a runtime sanitiser; sanitisation is the server's job, once, on insert.

## Goal

Every entry's HTML is sanitised on the server before commit. After M2 lands, a poll of a feed that contains `<script>`, on-event handlers, `javascript:` URLs, a 1×1 tracking pixel, a tracking-param-laden `<a href>`, and an iframe pointing at an arbitrary host produces an `entries.content` row that has all of those scrubbed — and a poll of a feed containing a legitimate YouTube or Vimeo embed preserves the iframe. The README's M1 hostile-feed warning is removed and replaced with the M2 trust posture.

## In scope

### New packages

Two small, stateless, pure-Go packages:

```
internal/sanitise/   # one entry point: Policy.Sanitise(rawHTML) string
internal/urlcleaner/ # one entry point: Clean(rawURL) string
```

Both are total functions: they never error, never panic. Worst case they return an empty string or the input unchanged. URL hygiene must never silently break an entry; sanitisation must never fail a poll.

### Sanitisation policy

The engine is `microcosm-cc/bluemonday` — pure-Go, allowlist-based, the well-trodden Go choice (Hugo, Caddy, the Go-rewrite era of miniflux). The policy starts from `bluemonday.UGCPolicy()` (allows formatting, links, lists, blockquotes, code, headings, tables, images) and then:

- **URL schemes are tightened to `http`, `https`, `mailto`.** `javascript:`, `data:`, `vbscript:` and any others are dropped wherever they appear (`href`, `src`, etc.). UGCPolicy is permissive about schemes by default; we narrow it.
- **`<iframe>` survives only if its `src` host matches the allowlist.** The default list is taken from miniflux (`internal/reader/sanitizer/sanitizer.go:121-135`):

  ```
  bandcamp.com           open.spotify.com       soundcloud.com
  cdn.embedly.com        player.bilibili.com    vk.com
  dailymotion.com        player.twitch.tv       w.soundcloud.com
  framatube.org          player.vimeo.com       youtube-nocookie.com
                                                 youtube.com
  ```

  Matching is exact host equality after stripping a leading `www.` — same posture as miniflux's `urllib.DomainWithoutWWW`. Suffix matching would let `evil.youtube.com` slip through; exact-match is the safe default. The list is hard-coded in M2; per-user override lands later (see *Out of scope*).
- **`<img>` is dropped if both `width` and `height` are `"0"` or `"1"`.** This is the pixel-tracker heuristic, mirroring miniflux's `isPixelTracker`. No host blocklist — the size heuristic catches the common case and a host list would drift.
- **`<a href>` and `<img src>` URLs run through `urlcleaner.Clean`** to strip tracking parameters before being written to the sanitised output.

### URL cleaning

`internal/urlcleaner` strips well-known tracking parameters from URLs. The static lists are seeded from miniflux's `internal/reader/urlcleaner/urlcleaner.go` and cover:

- **Inbound trackers** (whole-name match): `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content`, `mc_eid`, `mc_cid`, `hsCtaTracking`, `_hsmi`, `_hsenc`, `mkt_tok`, `vero_*`, `oly_*`, `wickedid`, …
- **Outbound click-trackers** (whole-name match): `fbclid`, `gclid`, `dclid`, `msclkid`, `yclid`, `igshid`, `ScCid`, `s_cid`, …
- **Prefixes**: `utm_`, `mtm_`, `pk_`.

The file carries an attribution comment:

```go
// Parameter list adapted from github.com/miniflux/v2 internal/reader/urlcleaner.
// Original copyright miniflux contributors, Apache-2.0.
```

A parse-error or non-URL input is returned unchanged — defensive contract so URL hygiene never breaks an entry.

### Worker integration

The seam is in `internal/poll/worker.go`. The current code at lines 65–83 builds `db.NewEntry{...Content: content}` from raw `item.Content`/`item.Description`. M2 changes:

1. `Worker` carries a `sanitise.Policy` (constructed once at scheduler/worker startup with `sanitise.DefaultPolicy()`).
2. Before constructing each `NewEntry`, the worker runs `content = w.policy.Sanitise(content)`.
3. A pre-sanitise size cap of 1 MiB truncates pathologically large entries — defence-in-depth on top of `feed.Fetch`'s 10 MiB body cap.

The DB layer (`db.UpdateAfterPoll`, `db.NewEntry`) is unchanged. The single-transaction commit per poll from M1 still holds — sanitisation is a per-item compute step entirely upstream of the DB.

### Storage

`entries.content` keeps its `TEXT NOT NULL` shape from M1; sanitised HTML overwrites the raw bytes at insert. We don't keep raw HTML around. This is what miniflux does (`internal/reader/processor/processor.go:165` writes the sanitised output straight back into `entry.Content`); their schema is a single `content text` column (`internal/database/migrations.go:84`). Trade-off: a future sanitiser-policy loosening can't retroactively grant new tags to old entries without re-fetching, and feeds may have dropped those entries from their window. We accept that — Tap is a self-hosted reader, not an archive.

### No schema migration

M1 entries are dropped on the M2 upgrade path, but not via a SQL migration. M1 is a pre-production walking skeleton; the M2 upgrade instruction in the README is "delete `tap.db` (or the `/data` volume) and re-subscribe." This avoids carrying a one-shot drop-and-reset migration that has no future after the M1→M2 step. The migration runner from M1 still picks up genuine future schema changes.

### README update

- The "M1 deployment safety" section is removed.
- Replaced with a short M2 trust-posture paragraph: feed HTML is sanitised server-side before storage; the SPA renders the stored HTML directly without a runtime sanitiser; the loopback-default bind remains as defence-in-depth (concept §6.11).
- An "Upgrading from M1" line: M1 databases are incompatible with M2 — delete `tap.db` (binary deployment) or the `/data` volume (container deployment) and re-subscribe.

### Tests and methodology

M2 follows the same test-first discipline as M1 (`docs/roadmap.md` §"Working cadence"). The discipline is non-negotiable for behaviour-bearing code; the only exemption is structural scaffolding (package skeleton, attribution comment, the README edit).

Concrete test surface:

- **`internal/urlcleaner`**
  - Table-driven: each tracking parameter (full-name and prefix lists) gets stripped from a sample URL.
  - Legitimate parameters survive (`page`, `id`, `q`).
  - Non-URL inputs and parse-error inputs are returned unchanged.
  - Empty-query-after-strip leaves no trailing `?` on the result.
- **`internal/sanitise`**
  - UGCPolicy smoke: `<script>` stripped, `onclick="…"` and other on-event handlers stripped.
  - URL schemes: `javascript:` and `data:` URLs in `href` and `src` are dropped.
  - `TestIframeAllowlistDefaults` — table-driven, one canonical embed URL per allowed host (positive cases pulled from vendor docs); negatives include `evil.com`, `youtube.com.evil.com`, `m.youtube.com`, `https://youtube.com.evil.com/embed/X`.
  - `TestPixelTracker` — `width=1 height=1` and `width=0 height=0` dropped; `width=1 height=2`, `width=2 height=1`, missing dims survive.
  - `TestUrlCleanerIntegration` — `<a href="…?utm_source=foo">` and `<img src="…?fbclid=bar">` come out without the tracking params after sanitise.
  - Total-function contract: malformed HTML, empty input, all-script input never panic; output is always a string.
- **`internal/poll/worker_test.go`** — a worker test injects a `sanitise.Policy` and asserts that the content reaching `db.NewEntry` has been sanitised (script-stripped, URL-cleaned).
- **`cmd/tap/main_test.go`** — the existing M1 end-to-end test gains a fixture entry containing `<script>`, an `onclick`, and a tracking-param `<a href>`. The poll-then-`GET /api/v1/entries/:id` round-trip asserts the body comes back clean.

## Out of scope (deferred)

| Concern | Lands in |
|---|---|
| Image-URL rewriting, media proxy, signed tokens, FS cache, MIME allowlist | M3 |
| Per-user iframe-host allowlist (DB-stored preference, settings UI) | Deferred items (post-M6) — see [`../roadmap.md`](../roadmap.md) |
| Adaptive cadence, per-host concurrency cap, SSRF guard, retry/backoff | M4 |
| Article extraction, per-feed CSS rules | M5 — the `sanitise.Policy` shape is designed so the extractor plugs in without reworking the package |
| Multi-user, sessions, CSRF, admin bootstrap | M6 |
| TOTP, passkeys, recovery codes | M7 |
| Themes, mobile, swipe gestures, keyboard shortcuts | M8 |
| Categories, OPML, search, add-feed flow | M9 |
| Service worker, offline reading, PWA manifest | M10 |
| Daily archival, tombstones, media-cache eviction | M11 |
| OTel logs/metrics/traces, healthcheck subcommand, admin CLI | M12 |
| Known-tracker host blocklist for images | Won't ship — pixel-size heuristic is the policy. Maintained host lists drift; miniflux deliberately doesn't keep one. |
| Re-sanitising existing rows when policy changes | Won't ship — overwrite-only storage means policy tweaks affect new entries only. Acceptable for a reader, not an archive. |
| Schema migration for the M1→M2 upgrade | Won't ship — operators delete `tap.db` and re-subscribe. |

## Risks and open questions

- **bluemonday's URL-rewriter surface.** bluemonday exposes attribute-value regex matching but no callable URL transformer. To run `urlcleaner.Clean` on every `href`/`src`, the iframe-host check on every iframe `src`, and the pixel-tracker drop on every img, M2 uses a single `golang.org/x/net/html` post-pass over bluemonday's output that handles all three concerns in one traversal. If this turns out clumsy in practice we'll evaluate writing a small custom sanitiser the way miniflux did. Cheap to revisit.
- **Tracking-param list maintenance.** The miniflux list is well-curated but not exhaustive (no Klaviyo `_kx`, no LinkedIn `li_fat_id`). We treat it as a snapshot and revisit when a real feed surfaces a missed parameter. Not a per-milestone concern.
- **Per-entry size cap of 1 MiB.** Picked from intuition, not measurement. Lives as a named constant, trivial to tune. If a legitimate long-form blog post exceeds it we raise the cap.
- **iframe defaults inherited from miniflux include `vk.com`, `bilibili`, `dailymotion`.** Some operators may not want those hosts. The per-user override (deferred items section of the roadmap) is the answer; M2 ships miniflux's full list as the default to mirror a known-working policy.
- **`sanitise.Policy` shape and M5.** The policy is a struct so M5 (article extraction) can build a relaxed variant without forking the package. Whether the struct field set is right becomes clear when M5 lands; until then we keep the surface minimal.

## Definition of done

1. `make test` passes (`go test ./... -race`) with the new packages and worker integration.
2. A poll of a synthetic hostile feed (script tags, on-event handlers, `javascript:` URL, 1×1 pixel, tracking-param `<a href>`, iframe to an unallowed host) produces an `entries.content` row free of all of those.
3. A poll of a feed containing a YouTube `<iframe src="https://www.youtube.com/embed/…">` and a Vimeo `<iframe src="https://player.vimeo.com/video/…">` preserves both iframes verbatim. (Iframe `src` URLs are not run through `urlcleaner` — only `<a href>` and `<img src>` are.)
4. README's M1 hostile-feed warning is replaced with the M2 trust posture and the M1→M2 upgrade note.
5. `internal/urlcleaner/urlcleaner.go` carries the miniflux attribution comment.
6. Smoke test: `make build` produces a static binary that boots cleanly against a fresh `data/` directory.

## What this milestone deliberately does *not* prove

- That image URLs are proxied or cached. (M3.)
- That the iframe allowlist is operator-configurable. (Deferred items section of the roadmap; lands post-M6.)
- That the sanitiser handles every adversarial HTML construct ever published. (We rely on bluemonday's track record; security review pass is M12.)
- That extracted article HTML flows through the same sanitiser. (M5 — the seam exists; the wiring lands then.)
- That tracking-parameter coverage is exhaustive. (Snapshot from miniflux; updates are reactive.)

If you find yourself broadening the iframe allowlist, adding host-based blocklists, or building a re-sanitisation path for old rows, push back. The point of this milestone is to land the smallest sanitiser that closes the M1 stored-XSS gap, with a clean seam for M3 to plug image proxying into. Anything beyond that belongs in a later milestone.
