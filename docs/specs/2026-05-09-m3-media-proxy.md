# M3 — Media proxy + cache

**Status:** Draft, awaiting review.

## Context

Tap is the self-hosted feed reader described in [`../concept.md`](../concept.md), with the visual identity and detailed design in [`../../ui_design/`](../../ui_design/). M3 is the third of twelve milestones — see [`../roadmap.md`](../roadmap.md). The walking skeleton from M1 ([`2026-05-08-m1-walking-skeleton.md`](2026-05-08-m1-walking-skeleton.md)) and the sanitisation pipeline from M2 ([`2026-05-08-m2-sanitisation.md`](2026-05-08-m2-sanitisation.md)) are shipped: feeds are polled, entries are sanitised on insert, the SPA renders the stored HTML directly without a runtime sanitiser.

After M2, an `<img>` tag inside an entry still points at its origin host. Three gaps remain:

- **Privacy.** Every image render reveals the user's IP and `Referer` to the origin — typically a third-party CDN the user never agreed to talk to.
- **Mixed content.** A Tap deployment served over HTTPS can't render `<img src="http://…">` images at all; modern browsers block them.
- **Offline.** The service worker (M10) needs URLs it can cache. Origin URLs aren't reliably cacheable: they're cross-origin, may carry varying query strings, and can disappear if the host goes away.

M3 closes all three with the internal media proxy described in [`../concept.md`](../concept.md) §6.7.

## Goal

Every `<img>` URL in a sanitised entry points at `/api/v1/proxy/<token>` instead of its origin. Hitting the proxy URL fetches the bytes through Tap (privacy + mixed-content), caches them on the local filesystem (offline + politeness), validates that the origin returned an actual image (not HTML or anything else), and serves the cached bytes to the browser with `Cache-Control: immutable` so the next render is a free local read. Tokens are signed with a server-generated secret so a caller without the secret can't construct a proxy URL that points at an arbitrary URL of their choosing — only URLs the sanitiser has signed are reachable through the proxy.

After M3 lands, polling a feed whose entries embed images and opening one in the reader produces image renders that touched the origin once (per cold cache), with the user's browser seeing only Tap's own host.

## In scope

### New packages

```
internal/proxy/      # Signer (Sign/Verify/RewriteImageURL), Cache (FS + sidecar), Handler
internal/processor/  # Composes sanitise + image rewriting; drop-in for the worker
```

`internal/sanitise` is unchanged from M2: same code, same tests, same behaviour. The image-rewriting concern lives in the new `processor` package, not in `sanitise`, because rewriting is a separate responsibility ("redirect images through the proxy") from sanitisation ("make this HTML safe to render"). Verifying that `internal/sanitise/` has no diff in the M3 changeset is a one-line review check.

### Token format

Stateless and deterministic per source URL:

```
token = base64url_nopad(rawURL) + "." + base64url_nopad(hmac_sha256(key, rawURL)[:16])
```

Truncating the HMAC to 128 bits is standard practice and well above any reasonable forge cost for an internal-only proxy. Determinism per URL means the same source URL always produces the same token, which lets the future M10 service worker cache hits across page loads.

`proxy.Signer.Sign(rawURL string) string` returns the token. `proxy.Signer.Verify(token string) (rawURL string, ok bool)` parses, constant-time-compares the HMAC (via `hmac.Equal`), and returns the URL only on success. `proxy.Signer.RewriteImageURL(rawURL string) string` returns `/api/v1/proxy/` + the token; this is the closure passed into the processor.

### Signing-key bootstrap

A new `configuration` table holds runtime-generated state. Migration `0002_configuration.sql`:

```sql
CREATE TABLE configuration (
    key   TEXT PRIMARY KEY,
    value BLOB NOT NULL
);
```

Concept §4 calls this table out by name. `BLOB` because the signing key is 32 raw bytes from `crypto/rand`. Future runtime-generated state (e.g. M6 session-related secrets) reuses this table.

In `cmd/tap/main.go`, after migrations and before constructing the proxy:

1. `SELECT value FROM configuration WHERE key = 'proxy.signing_key'`.
2. If absent, read 32 random bytes, `INSERT … ON CONFLICT(key) DO NOTHING`, re-`SELECT`.
3. Pass the bytes into `proxy.NewSigner`.

The `ON CONFLICT DO NOTHING` is defence against any conceivable insert race. Single-process startup means there isn't really one, but the cost of the clause is zero. The key persists forever from first launch. **Rotation is not a feature** — rotation invalidates every cached image's URL, which is destructive disguised as a knob. M12 may add an explicit admin command if rotation ever becomes necessary.

### Endpoint

`GET /api/v1/proxy/{token}` mounted on the existing API mux (no `/healthz`-level exception; it goes through the same `api.NewMux` factory). No auth check — matches M1/M2; auth lands in M6.

Request flow:

1. `signer.Verify(token)` → 404 on bad signature. No oracle distinguishing "URL valid but signature wrong" from "garbage token."
2. Cache lookup keyed on `sha256(rawURL)` hex. On hit, read sidecar for `content_type`, stream `.bin`, set response headers.
3. On miss, singleflight-coalesced origin fetch (see *Origin fetch* below).

Response headers on success (hit or freshly cached miss):

```
Content-Type: <from sidecar>
Content-Length: <.bin file size>
Cache-Control: public, max-age=31536000, immutable
X-Content-Type-Options: nosniff
```

No cookies, no `Vary`, no `Referer` mirroring back to the client.

### Cache directory layout

`${TAP_DATA_DIR}/cache/`, sibling of `tap.db`. Two files per cached image:

```
cache/<aa>/<hash>.bin    # raw bytes
cache/<aa>/<hash>.meta   # JSON sidecar
```

Where `hash` is `sha256(rawURL)` hex-encoded and `<aa>` is its first two hex chars. 256 buckets keep any single directory small. Buckets are `mkdir`'d on first write; we don't pre-create all 256.

Source URLs were already passed through `urlcleaner.Clean` upstream in the sanitiser, so two URLs differing only in tracking parameters won't generate duplicate cache entries.

### Sidecar format

```json
{
  "content_type": "image/jpeg",
  "etag": "\"abc123\"",
  "byte_count": 12345,
  "fetched_at": 1746754800
}
```

`etag` is stored for forward compatibility (concept §6.7 mentions it). M3 doesn't act on it because `Cache-Control: immutable` means the browser never asks for revalidation, and the proxy treats source URLs as immutable (no conditional GET against origin). Including it now is one string in a small JSON file; adding it later would require a backfill or a "missing field treated as empty" branch in the reader.

### Atomic writes

Write `.bin.tmp` and `.meta.tmp`, then `os.Rename` both in sequence. POSIX rename is atomic on the same filesystem. Recovery from a crash between renames: a `.bin` with a missing `.meta` (or vice versa) is treated as a cache miss; the next request re-fetches and overwrites. No reconciliation logic.

### MIME allowlist

Five formats:

```
image/png  image/jpeg  image/gif  image/webp  image/avif
```

SVG is excluded (see *Out of scope*). Type detection at fetch time:

1. Sniff the response body's first 512 bytes with `http.DetectContentType` for PNG, JPEG, GIF, and WebP. Go's stdlib has no AVIF detector (the ftyp container family is sniffed only as `video/mp4`), so a small `detectAVIF` helper in `internal/proxy/sniff.go` parses the ISO Base Media File Format ftyp box and matches the `avif` / `avis` brands.
2. Compare against the origin's `Content-Type` header (charset/parameter stripped).
3. Accept only if the sniffed type is in the allowlist **and** the origin's claim agrees.

The sniff is the trust anchor — origins lie about Content-Type all the time; bytes don't. The header check is defence in depth: an origin claiming `text/html` while serving PNG bytes is misconfigured at best, hostile at worst, and in either case we'd rather drop the response. A unit test fixture per allowlisted format pins the sniff behaviour against future Go upgrades; a `TestDetectAVIF` table covers the brand-walker edge cases (`avis` brand, `avif` as a compatible-brand alongside an `mp42` major brand, the minor-version skip).

### Origin fetch

The proxy handler accepts `*http.Client` in its constructor and reuses the client already constructed in `cmd/tap/main.go` (the one `feed.Fetch` uses — 30s total timeout, idle pool). M4 will swap that client for the SSRF-aware shared client; the swap is local to `main.go`.

- **Body cap:** 10 MiB via `http.MaxBytesReader` on the response. Matches the existing feed body cap. Pathological images get 502 to the client.
- **Buffering:** the full body is buffered in memory before being written to disk and returned to the client. Bounded memory pressure: `body_cap × concurrent unique misses`. Streaming-while-writing was considered and rejected — it complicates the singleflight model and pushes M3 over its complexity budget for marginal benefit.
- **No conditional GET against origin.** Treat URLs as content-addressable; the only refresh path is eviction-then-refetch.
- **Redirects:** Go's default redirect behaviour (follow up to 10) applies. M4's SSRF guard re-checks each hop independently; M3 trusts whatever the client does by default.

### Singleflight coalescing

`golang.org/x/sync/singleflight`, keyed on the URL hash. Concurrent cache misses for the same URL collapse to one origin fetch; all callers receive the same buffered bytes. Cache hits skip singleflight entirely (cheap disk read, no thundering herd to suppress).

### Inline LRU eviction on miss

Before writing a new cache entry: if `current_total_bytes + new_size > cap`, evict oldest-by-mtime until under cap. Implementation: `filepath.WalkDir` over `cache/`, collect `(path, mtime, size)`, sort by mtime ascending, delete `.bin` + `.meta` pairs in order. Held under a single in-process mutex for the duration of eviction (rare — only fires under cap pressure).

The daily age-based eviction sweep is M11. M3 only evicts under cap pressure.

### Failure handling

| Origin response | Proxy returns | Cached? |
|---|---|---|
| 200 OK, sniff in allowlist, MIME header agrees | 200 + bytes | yes |
| 200 OK, sniff not in allowlist or MIME mismatch | 415 | no |
| 200 OK, body exceeds 10 MiB | 502 | no |
| 3xx (followed up to default redirect cap) | depends on final response | as above |
| 4xx | proxied status (404 → 404, etc.) | no |
| 5xx, network error, timeout | 502 | no |

No negative caching. A hostile feed embedding a deliberately broken URL can make Tap re-hit the origin every render; M4's per-host concurrency cap is the bandwidth answer for that.

### Worker integration

`poll.SchedulerOpts.Policy *sanitise.Policy` becomes `poll.SchedulerOpts.Processor *processor.Processor`. `poll.NewWorker` stops requiring a non-nil Policy and instead requires a non-nil Processor. Worker's `Run` method changes one line:

```go
content = w.opts.Processor.Process(content)   // was: w.opts.Policy.Sanitise(content)
```

Wire-up in `cmd/tap/main.go`:

```go
key := loadOrCreateProxyKey(ctx, d)
signer := proxy.NewSigner(key)
cache := proxy.NewCache(cacheDir, capBytes)
proxyHandler := proxy.NewHandler(signer, cache, client, bodyCap)

proc := processor.New(sanitise.DefaultPolicy(), signer.RewriteImageURL)
sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{Processor: proc})
```

The proxy handler mounts under `/api/v1/proxy/` on the existing api mux.

### Processor shape

```go
// internal/processor
type Processor struct {
    sanitiser   *sanitise.Policy
    imgRewriter func(string) string  // optional; nil means no rewriting
}

func New(p *sanitise.Policy, rewriter func(string) string) *Processor

func (p *Processor) Process(rawHTML string) string
```

`Process` calls `sanitiser.Sanitise` first, then (if `imgRewriter != nil`) parses the cleaned HTML and walks it to rewrite `<img src>` attributes only. `<a href>` and `<iframe src>` are left untouched. Cost: one extra `html.ParseFragment` per entry — microseconds for a 5 KB body, well below the round-trip cost of any DB write the worker is also doing.

Total-function contract: never panics, never errors. A malformed input that breaks the second parse falls back to returning the sanitised-but-not-rewritten output rather than swallowing the entry.

### Configuration knobs

New flags / env vars in `cmd/tap/main.go`. Stdlib `flag` only; bytes-as-int64 keeps the surface trivial.

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--proxy-cache-dir` | `TAP_PROXY_CACHE_DIR` | `${TAP_DATA_DIR}/cache` | Sibling of `tap.db`. |
| `--proxy-cache-cap-bytes` | `TAP_PROXY_CACHE_CAP_BYTES` | `524288000` (500 MiB) | Inline LRU eviction trigger. |
| `--proxy-fetch-timeout` | `TAP_PROXY_FETCH_TIMEOUT` | `30s` | Per-fetch deadline. `flag.Duration`. |
| `--proxy-body-cap-bytes` | `TAP_PROXY_BODY_CAP_BYTES` | `10485760` (10 MiB) | Response body limit. |

A `parseSize("500MiB")` helper would be friendlier for hand-edited deployments. Deliberately not in M3 to keep the surface small.

### README update

- The "Trust posture: M2" section gains an M3 paragraph: "Article images are fetched through Tap's media proxy. Origin sites see only Tap's IP. Mixed-content image URLs work even when Tap is served over HTTPS."
- New operations note: "Cache directory at `${TAP_DATA_DIR}/cache/`. Default cap 500 MiB. Safe to delete at any time — next request re-fetches."
- Upgrading from M2: "M2 databases work as-is; M3 only adds the `configuration` table. Existing entries keep their direct `<img src="origin">` URLs and won't be retroactively proxied. To proxy all entries' images, delete `tap.db` and re-subscribe."

### Tests and methodology

M3 follows the same test-first discipline as M1 and M2 (`docs/roadmap.md` §"Working cadence"). Pure scaffolding (the migration SQL file, the README edit) is exempt; everything with branches, error handling, or state is in scope.

Concrete test surface:

- **`internal/proxy/signer_test.go`**
  - `Sign` is deterministic for the same `(key, URL)`.
  - `Verify` accepts valid tokens, rejects mismatched HMAC, rejects malformed input (no dot, bad base64, empty token, single component).
  - `Verify` constant-time-compares (the test asserts use of `hmac.Equal`, not `==`).
  - `RewriteImageURL` returns `/api/v1/proxy/` + the token.

- **`internal/proxy/cache_test.go`**
  - Cache hit reads bytes + sidecar from disk.
  - Cache miss invokes the supplied fetcher, writes `.bin` + `.meta` atomically (assert via temp-file presence in the middle of a synchronised fetcher).
  - Oversize body rejected without writing either file.
  - mtime-LRU evicts oldest first when over cap.
  - Recovery: a cache dir with a `.bin` and no `.meta` (or vice versa) treats the entry as a miss.
  - Singleflight collapses concurrent misses for the same key (assert fetcher invoked once across N goroutines).

- **`internal/proxy/handler_test.go`**
  - Valid token + cold cache → fetch invoked → 200 + bytes + correct headers.
  - Valid token + warm cache → no fetcher invocation → 200 from disk.
  - Bad token → 404.
  - Origin 4xx → status proxied through, not cached.
  - Origin returns `text/html` → 415, not cached.
  - Origin returns oversize body → 502, not cached.
  - Headers asserted: `Content-Type`, `Content-Length`, `Cache-Control`, `X-Content-Type-Options`.

- **`internal/proxy/sniff_test.go`** — one fixture per allowlisted format (PNG, JPEG, GIF, WebP, AVIF) asserting `http.DetectContentType` returns the expected MIME. Pins behaviour against future Go upgrades.

- **`internal/processor/processor_test.go`**
  - With nil rewriter, output equals `sanitise.DefaultPolicy().Sanitise` output byte-for-byte.
  - With rewriter, every `<img src>` is replaced with the rewriter's output.
  - `<a href>` and `<iframe src>` untouched.
  - Total-function contract: malformed input never panics, never errors.

- **`internal/db`** — migration 0002 creates the `configuration` table; `db.GetConfig(ctx, key)` / `db.SetConfigIfAbsent(ctx, key, value)` round-trip a value, and `SetConfigIfAbsent` is idempotent (second call with the same key is a no-op, returning the existing value).

- **`cmd/tap/main_test.go`** — extended hostile-feed fixture gains an `<img src="https://example.com/foo.png">` entry. Poll → `GET /api/v1/entries/:id` asserts the body contains `<img src="/api/v1/proxy/...">`. The test then GETs that proxy URL against an `httptest.Server` origin and asserts 200 + bytes. A close-and-reopen-DB cycle within the test asserts a previously-issued token still verifies after restart (signing-key persistence).

## Out of scope (deferred)

| Concern | Lands in |
|---|---|
| SSRF guard on proxy origin fetches (RFC1918 / loopback / link-local / ULA reject + suffix/CIDR allowlist + redirect re-check) | M4 |
| Per-host concurrency cap on proxy origin fetches | M4 |
| Article extraction, per-feed CSS rules | M5 |
| Multi-user, sessions, CSRF, admin bootstrap | M6 |
| TOTP, passkeys, recovery codes | M7 |
| Themes, mobile, swipe gestures, keyboard shortcuts | M8 |
| Categories, OPML, search, add-feed flow | M9 |
| Service worker offline-cache of proxy URLs, warm-cache driver | M10 |
| Daily age-based cache eviction sweep | M11 |
| OTel logs/metrics/traces, system-status panel, admin CLI | M12 |
| SVG support in MIME allowlist | Deferred items — needs an SVG sanitiser (XML allowlist walker mirroring M2's HTML post-pass, or rasterise SVG → PNG inside the proxy). No well-trodden pure-Go SVG sanitiser exists; both approaches are real options. |
| Conditional GET / revalidation against origin | Won't ship — the `Cache-Control: immutable` posture treats source URLs as content-addressable. Eviction-then-refetch is the only refresh path. |
| Negative caching of failed origin fetches | Won't ship in M3 — M4's per-host concurrency cap is the bandwidth answer for hostile-broken-URL feeds. |
| Re-rewriting M2-era entries to proxy URLs | Won't ship — overwrite-only sanitisation model from M2 carries forward. Operators delete `tap.db` and re-subscribe for full effect. |
| Signing-key rotation | Won't ship — rotation invalidates every cached image URL. M12 may add an explicit admin operation if needed. |
| `parseSize("500MiB")` helper for cache-cap flag | Cheap follow-up; deliberately not in M3 to keep the configuration surface trivial. |

## Risks and open questions

- **HMAC truncation to 128 bits.** Standard practice for HMAC-SHA256 truncation. For an internal-only proxy, well above any reasonable forge cost. If we ever expose proxy URLs to untrusted parties (we don't — the SPA is the only intended client per concept §5), switch to full 256 bits.
- **Cache-size accounting walks the directory.** O(N) on every cap-pressure check. For a 500 MiB cap of ~100 KiB images that's ~5,000 files — a few ms. If we ever cap at 50 GiB we'll keep a running total in memory and update on add/evict.
- **mtime-LRU vs atime-LRU.** atime is unreliable on Linux (`noatime` is common). mtime ("when written") means an often-read file with cold mtime gets evicted. Acceptable for immutable-image semantics — re-fetch is the only cost.
- **Full-buffer body in memory.** `body_cap × concurrent unique misses`. Realistic peak for single-user self-hosted Tap is well under a hundred concurrent unique misses, so worst case is hundreds of MiB. Streaming-while-writing would lower this at the cost of complicating singleflight; revisit if real deployments hit memory pressure.
- **`http.DetectContentType` and AVIF.** Go 1.25 covers all five allowlisted formats. The per-format fixture test pins behaviour against future Go upgrades.
- **Per-user iframe allowlist (carried from M2 deferred items).** Unrelated to M3 scope but called out for the post-M6 backlog.

## Definition of done

1. `make test` passes (`go test ./... -race`) with the new packages and the migration.
2. Subscribing to a fixture feed with `<img src="https://...">` entries and polling produces sanitised HTML whose `<img>` tags point at `/api/v1/proxy/<token>`.
3. `GET /api/v1/proxy/<token>` for a valid token returns the origin bytes with `Content-Type` from the sniffed MIME, `Cache-Control: public, max-age=31536000, immutable`, and `X-Content-Type-Options: nosniff`. The cached bytes appear under `${TAP_DATA_DIR}/cache/<aa>/<hash>.{bin,meta}`.
4. Restarting the server and re-requesting the same proxy URL serves from the cache without an origin fetch (verified via fixture origin's hit count).
5. Forcing the cap to a small value and fetching past it evicts older files first (mtime-ordered).
6. Two concurrent GETs for the same cold proxy URL trigger one origin fetch (verified via fixture origin's hit count).
7. A proxy URL whose origin returns `text/html` returns 415 to the client and writes no cache files.
8. README's M2 trust posture is extended with M3's; the `cache/` directory and the configuration table are documented.
9. `make build` produces a static binary that boots cleanly against a fresh `data/` directory.
10. `internal/sanitise/` has no diff in the M3 changeset (refactor preserves M2 behaviour exactly).

## What this milestone deliberately does *not* prove

- That proxy origin fetches respect SSRF, per-host concurrency caps, or honour `Retry-After`. (M4.)
- That the cache evicts unused entries at rest without traffic. (M11 daily sweep.)
- That SVGs render safely. (Deferred items.)
- That the proxy revalidates with origin when source bytes change. (Treated as immutable per source URL.)
- That the service worker pre-warms the proxy cache for offline reading. (M10.)
- That conditional requests / `If-None-Match` between client and proxy work. (Not needed; immutable cache-control covers the common case.)
- That signing-key rotation works. (Intentionally not a feature.)

If you find yourself adding daily age-based eviction, conditional GET against origin, SVG sanitisation, or per-host throttling here, push back. The point of M3 is the smallest media proxy that closes the privacy + mixed-content + offline gaps and gives M10 a stable URL shape to cache against. Anything beyond that belongs in a later milestone.
