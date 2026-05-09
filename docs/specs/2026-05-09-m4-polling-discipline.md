# M4 — Polling discipline

**Status:** Draft, awaiting review.

## Context

Tap is the self-hosted feed reader described in [`../concept.md`](../concept.md), with the visual identity and detailed design in [`../../ui_design/`](../../ui_design/). M4 is the fourth of twelve milestones — see [`../roadmap.md`](../roadmap.md). The walking skeleton from M1 ([`2026-05-08-m1-walking-skeleton.md`](2026-05-08-m1-walking-skeleton.md)), the sanitisation pipeline from M2 ([`2026-05-08-m2-sanitisation.md`](2026-05-08-m2-sanitisation.md)), and the media proxy from M3 ([`2026-05-09-m3-media-proxy.md`](2026-05-09-m3-media-proxy.md)) are shipped: feeds are polled on a fixed 30-minute cadence, entries are sanitised on insert, and `<img>` URLs route through the internal media proxy.

After M3, the polling pipeline still has four gaps that concept §6.3, §6.4, and §6.8 call out by name:

- **No adaptive cadence.** Every feed polls at the same fixed `Cadence` regardless of how often it actually publishes. Quiet feeds waste cycles; active feeds miss timely entries. Conditional GET (`If-None-Match` / `If-Modified-Since`) is already wired in `feed.Fetch` from M1, so the bandwidth cost of the wrong cadence is bounded — but the latency cost (a 15-minute-publish feed polled at 30-minute cadence) is not.
- **No per-host concurrency cap.** A dispatcher tick that picks two feeds on the same CDN, or the proxy serving ten image fetches for a single entry on the same host, can emit a burst of concurrent requests. Concept §6.4: "politeness is non-negotiable."
- **No SSRF guard.** The shared `*http.Client` constructed inline in `cmd/tap/main.go` connects to whatever the URL points at. A user pasting a feed URL like `http://localhost:9999/admin` is unprotected; a feed redirecting to `http://10.0.0.1` is unprotected. Concept §6.8 specifies a single shared client with a destination policy and an explicit allowlist.
- **No retry/backoff curve.** A failing poll today re-polls at the same `Cadence` as a successful one. A broken feed gets the same bandwidth as a healthy one; a transiently-errored feed waits 30 minutes for its first retry instead of seconds.

M4 closes all four. The shared HTTP client gains an SSRF policy and a per-host limiter; the worker gains an adaptive cadence and an exponential error backoff; subscriptions gain a velocity counter so the cadence has something to derive from.

## Goal

After M4, polling a fast-publishing feed and a once-a-week feed produces visibly different `next_poll_at` rows: the fast feed sits at the 15-minute floor, the quiet feed at the 24-hour ceiling. A feed whose origin returns `Retry-After: 3600` waits an hour regardless of its velocity. A feed whose poll fails with a network error retries in roughly five minutes, then ten, then twenty, then forty — capped at 24 hours, with each success resetting the curve. A feed whose URL points at `127.0.0.1` is rejected before connect with a clear error in `last_error`; the same URL succeeds after the operator allowlists `127.0.0.1/32`.

The polling pipeline and the media proxy share one `*http.Client`. The same SSRF policy, the same per-host cap, and the same total-request timeout apply to feed fetches and proxy origin fetches alike — concept §6.4 and §6.8 are explicit that all outbound traffic runs through a single client.

## In scope

### New package: `internal/httpx`

```
internal/httpx/
  client.go     # NewClient(opts) *http.Client — composes everything below
  ssrf.go       # SSRFPolicy + Dialer ControlContext + redirect callback
  hostlimit.go  # per-host RoundTripper wrapper (lazy semaphores)
  *_test.go
```

Public API:

```go
type SSRFPolicy struct {
    Disabled      bool
    AllowSuffixes []string       // dot-boundary hostname suffixes
    AllowCIDRs    []netip.Prefix // CIDR blocks (v4 and v6)
}

type Opts struct {
    Timeout         time.Duration // total per-request, default 30s
    PerHostInflight int           // concurrent requests per hostname, default 4
    SSRF            SSRFPolicy
    UserAgent       string        // applied centrally; feed.Fetch stops setting its own
}

func NewClient(opts Opts) *http.Client
```

`NewClient` builds:

1. A `net.Dialer` whose `ControlContext` resolves the target IP and rejects against `SSRFPolicy` before TCP connect.
2. An `http.Transport` using that dialer, with the same connection-pool tunables that M3 inlined in `main.go` (`MaxIdleConns: 32`, `MaxIdleConnsPerHost: 4`, `IdleConnTimeout: 90s`, `TLSHandshakeTimeout: 10s`).
3. A `hostLimiter` `RoundTripper` wrapping the transport, holding lazy `chan struct{}` of size `PerHostInflight` per hostname.
4. An `*http.Client` with `Timeout: opts.Timeout`, `Transport: hostLimiter`, and a `CheckRedirect` that re-validates each hop against `SSRFPolicy`.

The pieces compose so that `feed.Fetch` and `proxy.NewHandler` keep their `*http.Client` parameter unchanged — they don't know the client is SSRF-aware. That's the test seam: per-package unit tests pass a vanilla `httptest.Server` client; the SSRF wiring is exercised in `httpx`'s own tests and in one `cmd/tap` end-to-end fixture.

### SSRF policy

Reject by default any address whose resolved IP falls into:

| Family | CIDR(s) |
|---|---|
| IPv4 loopback | `127.0.0.0/8` |
| IPv4 RFC1918 | `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` |
| IPv4 link-local | `169.254.0.0/16` |
| IPv4 CGNAT | `100.64.0.0/10` (RFC 6598; Tailscale's default range) |
| IPv4 unspecified | `0.0.0.0/8` |
| IPv6 loopback | `::1/128` |
| IPv6 link-local | `fe80::/10` |
| IPv6 ULA | `fc00::/7` |
| IPv6 unspecified | `::/128` |
| IPv4-mapped IPv6 | `::ffff:0:0/96` (re-checked against the IPv4 rules) |

Two escape hatches:

- **Global disable.** `Opts.SSRF.Disabled = true` bypasses both layers — the dialer check and the redirect re-check. WARN-logged once at startup. Operators on fully-trusted internal networks can take it.
- **Allowlist.** Hostname suffixes match dot-boundary against `req.URL.Hostname()` before DNS. CIDR blocks match against the resolved IP after DNS. A destination that matches either kind of allowlist entry skips the reject rules entirely.

The dialer plugs into Go's `net.Dialer.ControlContext`, which fires after DNS resolution and after socket creation, before `connect()`. The `address` argument is `host:port` with the resolved IP literal; we parse the IP and check against the policy. If DNS returns multiple A/AAAA records, Go tries them in order — each one hits `ControlContext` separately, so a poisoned response of `[1.2.3.4, 127.0.0.1]` cannot slip past.

Redirects re-check via `http.Client.CheckRedirect`:

```go
func checkRedirect(req *http.Request, via []*http.Request) error {
    if len(via) >= 10 { return errors.New("too many redirects") }
    if !policy.AllowURL(req.URL) { return errors.New("ssrf: redirect blocked") }
    return nil
}
```

`AllowURL` short-circuits on the suffix allowlist; the IP check still happens at the dialer when the redirect's connect runs. The redirect callback is a *necessary but not sufficient* gate — it stops the obvious case (redirect to `http://localhost`) cheaply, and the dialer catches the harder case (redirect to a hostname that resolves to a private IP).

### Allowlist syntax

A single repeatable flag/env: `--ssrf-allow=<entry>`, comma-separated for env vars. Entry syntax auto-detected:

- Contains `/` → parse as `netip.Prefix`. Both v4 and v6.
- Looks like a bare IP literal (no `/`) → treat as `/32` or `/128`.
- Otherwise → hostname suffix, dot-boundary. `home.lan` matches `home.lan` and `nas.home.lan`; never `notmyhome.lan`.

Examples:

```
--ssrf-allow=127.0.0.1/32                # specific localhost
--ssrf-allow=192.168.1.0/24              # one LAN subnet
--ssrf-allow=marlin-tet.ts.net           # a Tailscale tailnet by name
--ssrf-allow=100.64.0.0/10               # all of CGNAT (Tailscale-friendly)
```

Parsing is strict: a malformed entry fails startup with a clear error rather than being silently ignored.

### Per-host limiter

A `RoundTripper` wrapping `http.Transport`:

```go
type hostLimiter struct {
    inner http.RoundTripper
    n     int
    mu    sync.Mutex
    sem   map[string]chan struct{}
}
```

`RoundTrip`:

1. Extract `req.URL.Hostname()` (already lowercased and IDN-punycoded by `net/url`).
2. Lazily create a buffered channel of size `n` for this hostname under `mu`.
3. Send a token (blocking on full); abort with `req.Context().Err()` if the context cancels first.
4. Call `inner.RoundTrip`.
5. Wrap the response body so `Body.Close()` releases the token. On RoundTrip error, release immediately.

Releasing on body close (not on RoundTrip return) is critical — under HTTP/1.1 keep-alive, the connection stays pinned to the request until the body is read, so releasing earlier would let a second request race for the same connection slot. Releasing on body close also bounds memory: a caller that forgets to drain the body deadlocks itself, not the limiter.

Per-host scope is by hostname, not by resolved IP. Concept §6.4 specifies "per-hostname" — two distinct CDNs sharing an anycast IP queue separately, and one hostname with multiple IPs queues together. Both behaviours are correct for a politeness signal whose unit is "a hostname I subscribe to," not "an endpoint I happen to connect to."

### Adaptive cadence

A new column on `subscriptions`:

```sql
ALTER TABLE subscriptions ADD COLUMN velocity_24h_x100 INTEGER NOT NULL DEFAULT 0;
```

Stored as fixed-point ×100 — entries-per-day with two implicit decimal digits. A feed with 14 entries in the last 7 days: `14 / 7 * 100 = 200`, i.e. 2.0 entries/day.

Recomputed on every successful poll path (200 and 304), using a live count over the entries table:

```sql
SELECT COUNT(*) * 100 / 7
FROM entries
WHERE subscription_id = ? AND published_at >= ?  -- now - 7*86400
```

For the 200 path, the count happens *after* the new entries are inserted so the just-inserted entries contribute. The simplest way to honour this is to do the count inside the same transaction that inserts the entries and updates the subscription row — the spec doesn't mandate the exact tx layout, only the post-insert ordering. Recomputing on 304 is necessary so the rolling 7-day window decays as old entries age out. A feed that published once a month ago and not since: velocity drops naturally toward 0, cadence drifts toward the 24h ceiling. On error, velocity is preserved — the error didn't tell us anything about publication rate.

Cadence formula:

```
velocity_per_day = velocity_24h_x100 / 100
interval = clamp(86400s / max(velocity_per_day, 1), floor, ceiling)
```

Examples:

| velocity (entries/day) | raw interval | clamped (15m–24h) |
|---|---|---|
| 0 | ∞ | 24h ceiling |
| 0.5 | 48h | 24h ceiling |
| 2 | 12h | 12h |
| 10 | ~2h24m | ~2h24m |
| 96 | 15m | 15m floor |
| 200 | ~7m | 15m floor |

Floor (15min) and ceiling (24h) are configurable but not advertised as everyday knobs.

### Server-mandated floor

After computing the velocity-based interval on a successful poll (200 or 304), the worker pushes `next_poll_at` further out (never earlier) based on origin response headers:

| Header | Treatment |
|---|---|
| `Retry-After: <seconds>` | duration; floor by `now + duration`. |
| `Retry-After: <http-date>` | absolute time; floor by that time. |
| `Cache-Control: max-age=<seconds>` | duration; floor by `now + max-age`. Only on success/304 paths. |

Order on success:

1. `candidate = now + velocity_interval`
2. If `Retry-After`: `candidate = max(candidate, retry_after_time)`
3. If `Cache-Control: max-age`: `candidate = max(candidate, now + max_age)`
4. `next_poll_at = candidate`

The cadence floor (15min) is **not** re-applied after a server-mandated push-out. If origin says "wait until tomorrow," we wait until tomorrow.

Server-mandated floors land in `next_poll_at` directly; no separate `retry_after_until` column. A separate column would be operationally interesting (visible reason for a quiet feed) but `last_error` already captures the failure case and the `Retry-After` log line records the success case for forwarding.

### Error backoff

Replaces the cadence formula entirely on failure:

```
delay  = min(error_base * 2^(error_count - 1), ceiling)
jitter = uniform random in [0, delay/4)
next_poll_at = now + delay + jitter
```

`error_count` increments on each failure. A successful poll (200) or a 304 resets `error_count` to 0 and switches the worker back to the cadence formula. Defaults: `error_base = 5min`, ceiling reuses `--poll-ceiling = 24h`.

Concrete progression with defaults:

| failures | delay | with 25% jitter (rough range) |
|---|---|---|
| 1 | 5m | 5m–6m15s |
| 2 | 10m | 10m–12m30s |
| 3 | 20m | 20m–25m |
| 4 | 40m | 40m–50m |
| 5 | 1h20m | 1h20m–1h40m |
| 6 | 2h40m | 2h40m–3h20m |
| 7 | 5h20m | 5h20m–6h40m |
| 8+ | 24h cap | 24h–30h |

Jitter bounds the thundering-herd risk when many feeds on the same host all errored at once and recover together. 25% is a common default — enough to spread, not so much it doubles the wait.

If the failing response carried `Retry-After`, take the larger of (server time, computed backoff). The server is allowed to ask for *more* patience than our curve, never less.

### Pure cadence functions

The arithmetic lives in a new leaf package `internal/cadence` so both `internal/db` and `internal/poll` can import it without cycling:

```go
package cadence

func IntervalFromVelocity(velocityX100 int, floor, ceiling time.Duration) time.Duration

func BackoffFromErrorCount(errorCount int, base, ceiling time.Duration,
                           jitterFrac float64, rng *rand.Rand) time.Duration

func ApplyServerFloors(candidate time.Time, retryAfter time.Time,
                       cacheMaxAge time.Duration, now time.Time) time.Time

func ParseRetryAfter(header string, now time.Time) (time.Time, bool)

func ParseCacheMaxAge(header string) (time.Duration, bool)
```

All five are pure (modulo `rng`, which is the test seam for jitter). The worker calls them on the 304 and error paths; `db.UpdateAfterPoll` calls them on the success path (so the post-insert velocity flows directly into the next_poll_at write). The scheduler constructs the rng once (`rand.New(rand.NewSource(time.Now().UnixNano()))`) and threads it through.

### Worker integration

`poll.WorkerOpts` gains:

```go
type WorkerOpts struct {
    Processor *processor.Processor // unchanged
    Floor     time.Duration        // default 15m
    Ceiling   time.Duration        // default 24h
    ErrorBase time.Duration        // default 5m
    Now       func() time.Time     // default time.Now; test seam
    Rand      *rand.Rand           // jitter source; nil-safe
}
```

The cadence-related opts are mirrored on `SchedulerOpts` and propagated into `NewWorker`.

`Worker.Run` post-fetch keeps the existing sequential if/return shape:

```go
res, fetchErr := feed.Fetch(ctx, w.client, sub.FeedURL, ...)
now := w.opts.Now()

if fetchErr != nil {
    delay := cadence.BackoffFromErrorCount(sub.ErrorCount+1, w.opts.ErrorBase, w.opts.Ceiling, 0.25, w.opts.Rand)
    next := now.Add(delay)
    if !res.RetryAfter.IsZero() && res.RetryAfter.After(next) {
        next = res.RetryAfter
    }
    db.UpdateAfterError(ctx, w.db, sub.ID, fetchErr.Error(), next.Unix())
    return
}

if res.Status == http.StatusNotModified {
    velocity := db.QueryVelocity(ctx, w.db, sub.ID, now)
    interval := cadence.IntervalFromVelocity(velocity, w.opts.Floor, w.opts.Ceiling)
    next := cadence.ApplyServerFloors(now.Add(interval), res.RetryAfter, res.CacheMaxAge, now)
    db.UpdateAfterNotModified(ctx, w.db, sub.ID, now.Unix(), next.Unix(), velocity)
    return
}

// Success: build entries, then commit (insert + post-insert velocity + next_poll_at).
newEntries := buildEntries(sub.ID, res.Feed.Items, w.opts.Processor)
db.UpdateAfterPoll(ctx, w.db, sub.ID, db.PollResult{
    NewETag:         nullStr(res.ETag),
    NewLastModified: nullStr(res.LastModified),
    NowUnix:         now.Unix(),
    NewEntries:      newEntries,
    Floor:           w.opts.Floor,
    Ceiling:         w.opts.Ceiling,
    RetryAfter:      res.RetryAfter,
    CacheMaxAge:     res.CacheMaxAge,
})
```

`db.UpdateAfterPoll` gains the cadence inputs (floor, ceiling, retry-after, cache-max-age) and computes velocity + next_poll_at internally — that keeps the post-insert count and the next_poll_at write in one transaction without exposing the worker to the tx boundary. The pure cadence functions live in a new leaf package `internal/cadence` (not `internal/poll/cadence.go`) so both `db` and `poll` can import them without cycling — `poll → db → cadence` is a clean chain.

`db.UpdateAfterNotModified` gains a `velocity` parameter and writes it to the new column. `db.UpdateAfterError` is unchanged in shape — `error_count`, `last_error`, `next_poll_at`. Velocity on the row stays at its prior value across an error.

`feed.FetchResult` gains `RetryAfter time.Time` and `CacheMaxAge time.Duration` (zero values when absent). `feed.Fetch` parses both headers using the new pure functions and **populates `FetchResult` even on error returns** — today it returns `FetchResult{}` on non-2xx/304, which would lose the `Retry-After` header on a 503. The shape becomes `func Fetch(...) (FetchResult, error)` where the result is always populated from the response headers if a response was received.

### `feed.Fetch` shrinks

The User-Agent header is now set centrally on the shared client (via `httpx.Opts.UserAgent`). `feed.FetchOpts.UserAgent` is removed; `feed.Fetch` no longer sets `User-Agent` on its requests. The `Accept` header — feed-specific — stays on `feed.Fetch`.

### Wire-up in `cmd/tap/main.go`

```go
ssrf, err := httpx.ParseSSRFPolicy(*ssrfDisabled, *ssrfAllow)
if err != nil { … }

client := httpx.NewClient(httpx.Opts{
    Timeout:         *httpTimeout,
    PerHostInflight: *perHostInflight,
    SSRF:            ssrf,
    UserAgent:       *userAgent,
})

proxyHandler := proxy.NewHandler(signer, cache, client, *proxyBodyCap)

sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
    Processor: proc,
    Floor:     *pollFloor,
    Ceiling:   *pollCeiling,
    ErrorBase: *pollErrorBase,
})
```

The inline `&http.Client{...}` block in M3's `main.go` is gone. The TODO comment at `main.go:84` is resolved.

### Configuration knobs

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--http-timeout` | `TAP_HTTP_TIMEOUT` | `30s` | Total per-request deadline on the shared client. |
| `--per-host-inflight` | `TAP_PER_HOST_INFLIGHT` | `4` | Concurrent outbound requests per hostname. See *Risks* for the deviation from concept §6.4. |
| `--ssrf-disabled` | `TAP_SSRF_DISABLED` | `false` | Global escape hatch. WARN at startup if enabled. |
| `--ssrf-allow` | `TAP_SSRF_ALLOW` | `""` | Repeatable flag; CSV in env. Auto-detect CIDR vs hostname suffix. |
| `--poll-floor` | `TAP_POLL_FLOOR` | `15m` | Adaptive cadence floor. |
| `--poll-ceiling` | `TAP_POLL_CEILING` | `24h` | Adaptive cadence ceiling and error-backoff cap. |
| `--poll-error-base` | `TAP_POLL_ERROR_BASE` | `5m` | Base of exponential error backoff. |
| `--user-agent` | `TAP_USER_AGENT` | `tap/0.1 (+https://github.com/bcrisp4/tap)` | Set centrally on the shared client. |

M3's `--proxy-fetch-timeout` / `TAP_PROXY_FETCH_TIMEOUT` is removed (Tap is pre-production; we don't carry deprecation aliases across milestones). Operators using it must switch to `--http-timeout` / `TAP_HTTP_TIMEOUT`.

### README update

A new section after M3's "Trust posture":

> **Trust posture: M4.** Outbound HTTP runs through one shared client. Destinations resolving to loopback, RFC1918, link-local, CGNAT, ULA, or other private ranges are rejected before connect. Tailscale users on the default `100.64.0.0/10` CGNAT range need `--ssrf-allow=100.64.0.0/10` (or their tailnet's specific subnet). Redirects are re-checked independently. The same per-hostname concurrency cap (default 4) applies to feed fetches and media-proxy origin fetches. Polling cadence is adaptive: a fast feed polls every 15 minutes, a quiet feed every 24 hours; origin-mandated `Retry-After` and `Cache-Control: max-age` are honoured as floors. Failed polls back off exponentially.

Plus an upgrade note: "M3 databases get one column added — `velocity_24h_x100`. Existing rows start at velocity 0 (24h ceiling) and back-fill on their next successful poll."

### Tests and methodology

M4 follows the same test-first discipline as M1, M2, and M3 (`docs/roadmap.md` §"Working cadence"). Pure scaffolding (the migration SQL file, the README edit, the flag declarations) is exempt; everything with branches, error handling, or state is in scope.

Concrete test surface:

- **`internal/httpx/ssrf_test.go`**
  - Dialer rejects each of: 127.0.0.1, 10.x, 172.16.x, 192.168.x, 169.254.x, 100.64.x, ::1, fe80::, fc00::, ::ffff:127.0.0.1.
  - Allowlist suffix: `home.lan` matches `home.lan` and `nas.home.lan`, rejects `notmyhome.lan`.
  - Allowlist CIDR: `192.168.1.0/24` accepts `192.168.1.5`, rejects `192.168.2.5`.
  - Bare IP literal: `127.0.0.1` parsed as `127.0.0.1/32`.
  - `Disabled = true` flips every reject case to accept.
  - `ParseSSRFPolicy` round-trip from CLI form, including a malformed entry → error.
  - Multi-record DNS: synthetic resolver returns `[1.2.3.4, 127.0.0.1]`; the second address is rejected even if the first was allowed.
  - Redirect callback: an origin that 302s to a private IP is rejected; chain depth limit of 10 is enforced.

- **`internal/httpx/hostlimit_test.go`**
  - `n=1`: two concurrent requests to one host serialise (assert via timestamps + a barrier-aware origin handler).
  - `n=4`: five concurrent requests to one host show 4 in flight, 1 queued.
  - Two concurrent requests to *different* hosts run in parallel.
  - Slot released on body close, not on RoundTrip return (assert by holding the body open and observing a queued request still blocked).
  - Slot released on response error (origin closes connection mid-handshake; subsequent request proceeds).
  - Context cancellation while waiting for a slot returns the ctx error.

- **`internal/httpx/client_test.go`**
  - End-to-end via `httptest.Server`: SSRF policy + per-host cap composed correctly.
  - Body cap and timeout from `Opts` propagate to the client.
  - User-Agent applied centrally, including over redirects.

- **`internal/cadence/cadence_test.go`**
  - `IntervalFromVelocity` table: velocity → expected interval, including boundary cases (0, exactly-floor, exactly-ceiling).
  - `BackoffFromErrorCount` deterministic rng → expected delays for n = 1…10.
  - `ApplyServerFloors`: candidate already past retry-after (no-op); retry-after later (pushes); cache-max-age later (pushes); both set, the later wins.
  - `ParseRetryAfter`: seconds form, HTTP-date form, malformed (returns false).
  - `ParseCacheMaxAge`: present with various other directives, absent, malformed.

- **`internal/poll/worker_test.go`** (extended)
  - Successful poll with N entries → velocity recomputed → `next_poll_at = now + clamp(7d/velocity, floor, ceiling)`.
  - 304 → velocity recomputed (window slides) → cadence re-derived; `error_count = 0`.
  - Error → `next_poll_at = now + base * 2^(n-1)` with deterministic rng; `error_count` increments.
  - Origin sends `Retry-After: 3600` on 200 → `next_poll_at >= now + 1h`.
  - Origin sends `Retry-After: 3600` on 503 → `next_poll_at = max(error_backoff, now + 1h)`.
  - Origin sends `Cache-Control: max-age=7200` on 304 → `next_poll_at >= now + 2h`.
  - `error_count` resets on next 200 *and* on next 304.

- **`internal/db/subscriptions_test.go`** (extended)
  - Migration 0003 applied: column exists with default 0.
  - `UpdateAfterPoll` writes `velocity_24h_x100`.
  - `UpdateAfterNotModified` writes `velocity_24h_x100`.
  - `RecomputeVelocity` returns the rolling 7-day count×100/7, including boundary at exactly 7 days (inclusive of `now - 7d`).

- **`cmd/tap/main_test.go`** (extended)
  - With SSRF enabled and no allowlist, subscribing to a feed URL whose host resolves to `127.0.0.1` polls and records an SSRF error in `last_error`.
  - With `--ssrf-allow=127.0.0.1/32`, the same poll succeeds.
  - With `--ssrf-disabled`, the poll succeeds and a startup WARN log is emitted (captured via `slog.SetDefault`).
  - With `--per-host-inflight=1`, two concurrent requests to the same fixture origin serialise (counter assertion).
  - A fixture origin returning `Retry-After: 3600` on 503: post-poll, `next_poll_at >= now + 1h`.
  - After two poll errors, the third (successful) poll resets `error_count` to 0.
  - A redirect from an allowlisted host to a non-allowlisted private IP is rejected by the redirect callback; `last_error` reflects the redirect block.

## Out of scope (deferred)

| Concern | Lands in |
|---|---|
| Per-feed HTTP overrides (user agent, cookie, basic-auth, outbound proxy URL, HTTP/2 toggle, self-signed-cert toggle) | M6 (credential redaction lands then; per-feed overrides ride along) |
| Per-feed extraction CSS rules | M5 |
| Article extraction itself (and its outbound fetches running through the shared client) | M5 |
| Velocity-derived UI affordance ("polls every ~Xm") in the SPA | M8 |
| Per-feed override of `PerHostInflight` for friendly CDNs | Won't ship — global cap is sufficient. |
| Negative caching of repeated origin failures (proxy) | Won't ship — error backoff on the polling side and the per-host cap on the proxy side together bound the bandwidth cost. |
| Status panel showing recent errors / per-feed cadence | M12 |
| Honour `RFC9111` cache directives beyond `max-age` (`s-maxage`, `must-revalidate`, etc.) | Won't ship — `max-age` covers the polite case; the rest are origin-server semantics that don't translate to a polling cadence. |
| Separate `retry_after_until` column on `subscriptions` | Won't ship — server-mandated floors land in `next_poll_at` directly. |
| IP-based per-host limiter (bucket by resolved IP rather than hostname) | Won't ship — concept §6.4 specifies "per-hostname"; reasonable for a politeness signal whose unit is "a hostname I subscribe to." |

## Risks and open questions

- **`PerHostInflight = 4` deviates from concept §6.4's literal "strict serialisation per host."** Documented as a deliberate UX trade-off: cap=1 makes cold-cache reader opens with N images on the same CDN noticeably slow (N × per-image latency, serialised). 4 is "small" per concept §6.4's spirit, browser-like (Chrome's per-host HTTP/1.1 cap is 6), and well below thundering-herd territory for typical CDNs. The cap is one config flag away from 1 if a real deployment reports the opposite problem; M10's service-worker pre-warm will reduce the cold-cache surface anyway.
- **DNS rebinding.** Mitigated by `Dialer.ControlContext` running per-resolved-IP, not per-URL. Once Go has picked an IP for the connect, that's the IP `ControlContext` sees and the IP that gets used. An attacker who races DNS responses against the connect cannot slip past.
- **Tailscale CGNAT default reject.** Tailscale's default is `100.64.0.0/10`, which collides with our default-reject CGNAT rule. Documented in the README's M4 trust-posture section. Operators on Tailscale must explicitly allowlist their tailnet (CIDR or hostname suffix). The user's tailnet `marlin-tet.ts.net` would be allowlisted by `--ssrf-allow=marlin-tet.ts.net` for hostname-based subscriptions, or `--ssrf-allow=100.64.0.0/10` for IP-based.
- **`Cache-Control: max-age=0`.** Treated as "no floor; just use velocity cadence." A literal `next_poll = now + 0` would re-fire immediately on the next tick.
- **Exponential backoff with a busy feed.** A 15-min-cadence feed that errors once gets pushed out to ~5min — *closer* than its normal cadence. Intentional: the curve is gentle initially, grows fast. After 3 errors it's at 20min, longer than cadence. The first retry-soon is desirable for transient errors.
- **Velocity recompute on 304.** Cheap (one indexed `COUNT`) but it's an additional write inside the 304 transaction. Worth noting in case future profiling flags it; the alternative (recompute only on 200) leaves stale velocity numbers that could keep a long-quiet feed at a misleadingly-fast cadence.
- **Velocity column addition.** SQLite's `ALTER TABLE … ADD COLUMN … DEFAULT 0` is fast and safe. Existing rows start at velocity 0 (24h ceiling) and back-fill on their next successful poll or 304.
- **`Retry-After` parsing.** The HTTP-date form is rare in practice but spec'd; we parse it for completeness. Malformed `Retry-After` is logged at DEBUG and ignored.
- **Hostname normalisation for the limiter.** `req.URL.Hostname()` returns lowercase punycode. `Example.COM:443` and `example.com` queue together. IPs (`192.168.1.5`) get their own keys. We don't try to coalesce by resolved IP — the per-hostname unit matches concept §6.4's wording and keeps the policy comprehensible from the URL alone.

## Definition of done

1. `make test` passes (`go test ./... -race`) with the new package, the migration, and the extended worker / cmd tests.
2. A subscription whose feed publishes 14 entries/week settles at `next_poll_at` ~12h ahead after one successful poll.
3. A subscription whose poll fails with a network error has `next_poll_at` ~5min ahead with `error_count = 1`; second consecutive failure ~10min ahead with `error_count = 2`; recovers to a cadence-derived `next_poll_at` and `error_count = 0` on the first successful poll.
4. A poll receiving `Retry-After: 3600` (success or error) has `next_poll_at >= now + 1h` regardless of velocity.
5. With SSRF enabled and no allowlist, attempting to subscribe to `http://127.0.0.1:9999/feed` records an SSRF error in `last_error`; the same URL succeeds with `--ssrf-allow=127.0.0.1/32`.
6. With `--ssrf-disabled`, the loopback subscription succeeds and a startup WARN log is emitted.
7. With `--per-host-inflight=1`, two concurrent requests to the same hostname serialise (verified via fixture origin's concurrency counter); with `=4`, four run in parallel and the fifth waits.
8. A redirect from an allowlisted host to a non-allowlisted private IP is rejected by the redirect callback before the next connect; `last_error` reflects the redirect block.
9. `internal/feed/parse.go` no longer sets `User-Agent` (the shared client owns it); `feed.FetchOpts.UserAgent` is removed.
10. `cmd/tap/main.go` uses `httpx.NewClient(...)` exclusively; the inline `&http.Client{...}` block from M3 is gone.
11. M3's `--proxy-fetch-timeout` flag and `TAP_PROXY_FETCH_TIMEOUT` env var are removed (replaced by `--http-timeout` / `TAP_HTTP_TIMEOUT`).
12. README gains the M4 trust-posture section, the Tailscale CGNAT note, and the adaptive-cadence summary.
13. `make build` produces a static binary that boots cleanly against a fresh `data/` directory and against an existing M3 database (the `velocity_24h_x100` migration applies cleanly with no data loss).

## What this milestone deliberately does *not* prove

- That per-feed HTTP overrides (cookie, basic-auth, outbound proxy URL) work — those land with M6 credential redaction.
- That article-extraction outbound fetches respect the shared client — M5 wires extraction in; M4 only proves the client itself.
- That M11's archival sweep correctly evicts stale entries — orthogonal to M4.
- That the SPA surfaces "polls every ~Xm" anywhere — M8 polish.
- That `Cache-Control` directives beyond `max-age` are honoured.
- That the velocity-counter column is denormalised against any future cross-subscription analytics — it's a per-row scheduling input, not a reporting field.

If you find yourself adding per-feed HTTP overrides, article extraction, the SPA cadence affordance, or full RFC 9111 compliance, push back. M4's job is the smallest possible polling discipline that closes the SSRF / per-host-burst / cadence-tuning / retry gaps and leaves M5 (extraction) a stable shared client to plug into.
