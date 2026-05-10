# M12 — Observability + production hardening

**Status:** Draft, awaiting review.

## Context

Tap is the self-hosted feed reader described in [`../concept.md`](../concept.md). M12 is the twelfth and final milestone — see [`../roadmap.md`](../roadmap.md). All prior milestones (M1–M11) have shipped.

M12 is the "ready for unattended self-hosted production" pass. After M11, Tap is functionally complete. M12 closes the operational gaps: an operator running Tap on a VPS or homelab NAS should be able to answer "is it healthy, what is it doing, and why is this one thing slow" without attaching a debugger. It also hardens the security posture with brute-force lockout, argon2 re-hash-on-verify, and a structured security audit pass.

The three observability pillars (concept §11), brute-force resistance (concept §7.7), the `tap healthcheck` subcommand (concept §10), the recent-errors ring buffer (M1-deferred), and the system-status panel (concept §10, concept §13) all land here.

## Goal

After M12:

- A Prometheus scrape of `GET /metrics` (when `--metrics-enabled`) returns counters and histograms covering polls, HTTP traffic, media proxy cache, auth events, DB transactions, and process health. Every metric has a description string in the Prometheus exposition format explaining its meaning, units, and labels.
- An operator with an OTel collector configured via `--otlp-endpoint` receives traces for inbound API requests, poll workflows, and outbound HTTP calls; and OTLP metrics mirroring the Prometheus set. Both are off until configured.
- Log lines at `warn` and `error` level follow the event taxonomy defined in this spec, carry `request_id` for inbound-request-scoped events, and feed the in-memory ring buffer.
- The SPA's settings view (admin-only) shows a system-status panel: version, uptime, DB status, active poll count, last poll time, and the last 20 error-class events from the ring buffer.
- `GET /healthz` returns a slim JSON body usable by container orchestrators and `tap healthcheck`.
- `tap healthcheck` exits 0 on healthy, 1 on degraded — for the Dockerfile `HEALTHCHECK` directive (no shell, no curl in distroless).
- `tap admin list` and `tap admin disable` extend the M6 admin CLI.
- The login endpoint is rate-limited per source IP and per target username, with escalating lockout after repeated failures.
- Argon2 password hashes stored with params weaker than `auth.DefaultParams` are silently re-hashed on next successful login.
- A structured security audit pass verifies authn/authz on every route, SSRF posture on every outbound HTTP path, `MaxBytesReader` on every write handler, and produces zero `govulncheck` + `semgrep` findings (or explicitly documented accepted-risk exceptions).

## In scope

### New package: `internal/metrics`

```
internal/metrics/
  provider.go      # Provider, Init(opts), Shutdown, global default
  instruments.go   # All instrument definitions with description strings
  handler.go       # Prometheus /metrics HTTP handler
```

#### Provider

`metrics.Init(opts MetricsOpts)` initialises an OTel `MeterProvider` backed by two readers:
1. A Prometheus exporter (via `go.opentelemetry.io/otel/bridge/prometheus`) that feeds the `GET /metrics` handler.
2. An OTLP metric exporter (off unless `opts.OTLPEndpoint` is set).

`metrics.Shutdown(ctx)` flushes both readers and closes cleanly. Called from `cmd/tap/main.go`'s shutdown sequence, after `sched.Stop` and before `db.Close`.

The global default provider is a no-op until `Init` is called — same pattern as `slog.SetDefault`. Every `instruments.go` registration calls into the global provider, so test code that never calls `Init` incurs no overhead and no panic.

```go
type MetricsOpts struct {
    OTLPEndpoint string        // empty = Prometheus only
    OTLPHeaders  map[string]string
}

func Init(opts MetricsOpts) error
func Shutdown(ctx context.Context) error
```

#### Instrument definitions

Every instrument is defined in `instruments.go` with a Go variable, a metric name, a unit, and a description string. The description string appears verbatim in the Prometheus exposition output (`# HELP` line) so operators scraping the endpoint can understand each metric without consulting this spec.

**Polling metrics:**

| Instrument | Type | Unit | Labels | Description |
|---|---|---|---|---|
| `tap_polls_total` | Counter | `{polls}` | `result={success,failure,skipped}` | Total number of feed poll attempts, labelled by outcome. `skipped` means the poll was due but deferred (backoff, in-flight dedup). |
| `tap_poll_duration_seconds` | Histogram | `s` | `result={success,failure}` | Wall-clock duration of a complete poll cycle from dispatch to commit (or failure), in seconds. Buckets: 0.1, 0.5, 1, 5, 10, 30, 60. |
| `tap_entries_inserted_total` | Counter | `{entries}` | — | Total number of new feed entries committed to the database across all polls. Does not count deduplicated (already-seen) entries. |
| `tap_conditional_get_hits_total` | Counter | `{polls}` | — | Number of polls that received a 304 Not Modified response, indicating the feed has not changed since the last poll. |

**HTTP server metrics (inbound):**

| Instrument | Type | Unit | Labels | Description |
|---|---|---|---|---|
| `tap_http_requests_total` | Counter | `{requests}` | `method`, `route`, `status_class={2xx,4xx,5xx}` | Total inbound HTTP requests, labelled by HTTP method, matched route pattern (not raw path — avoids high-cardinality from entry IDs), and response status class. |
| `tap_http_request_duration_seconds` | Histogram | `s` | `method`, `route` | Inbound HTTP request latency from first byte received to last byte written, in seconds. Buckets: 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5. |

`route` uses the registered pattern (e.g. `GET /api/v1/entries/{id}`) not the resolved URL, to prevent label cardinality explosion from entry/subscription IDs.

**Media proxy cache metrics:**

| Instrument | Type | Unit | Labels | Description |
|---|---|---|---|---|
| `tap_proxy_cache_hits_total` | Counter | `{requests}` | — | Number of media proxy requests served from the local filesystem cache without fetching the origin. |
| `tap_proxy_cache_misses_total` | Counter | `{requests}` | — | Number of media proxy requests that required fetching the origin (cache cold or expired). |
| `tap_proxy_cache_evictions_total` | Counter | `{files}` | `reason={size_cap,age_sweep}` | Number of cached media files evicted. `size_cap` = inline eviction triggered by the configured byte cap; `age_sweep` = daily archival sweep. M11 exposes an `OnEvict func(n int)` callback on `ArchiverOpts`; M12 wires it in `cmd/tap/main.go` to increment this counter with `reason="age_sweep"` (see wire-up section below). |
| `tap_proxy_cache_bytes` | Gauge | `By` | — | Current total size of the media proxy filesystem cache in bytes. Updated after each eviction pass and each new cache write. |

**Auth metrics:**

| Instrument | Type | Unit | Labels | Description |
|---|---|---|---|---|
| `tap_login_attempts_total` | Counter | `{attempts}` | `result={success,failure,rate_limited,locked_out}` | Total login attempts against `POST /api/v1/sessions`, labelled by outcome. `rate_limited` = per-source token bucket exhausted; `locked_out` = per-username lockout active. |
| `tap_lockouts_total` | Counter | `{lockouts}` | `axis={source,username}` | Number of lockout events triggered. `source` = per-IP rate limit exceeded; `username` = per-username consecutive failure threshold reached. |
| `tap_active_sessions` | Gauge | `{sessions}` | — | Number of non-expired session rows in the database. Updated on session create, delete, and expiry. Approximation — counts rows, not live connections. |

**Database metrics:**

| Instrument | Type | Unit | Labels | Description |
|---|---|---|---|---|
| `tap_db_tx_duration_seconds` | Histogram | `s` | `op` | Duration of database transactions, in seconds. `op` is a short label identifying the query group (e.g. `insert_entries`, `list_due_polls`, `get_session`). Buckets: 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1. |

**Process metrics:**

Standard Go runtime metrics via `go.opentelemetry.io/contrib/instrumentation/runtime` (goroutine count, heap alloc, GC pause, thread count). These appear under the `go_*` and `process_*` namespaces in Prometheus exposition format. No custom instruments needed — the contrib package handles registration.

**Per-user labels:** `user_id` labels are deliberately not added to any instrument. Per-user label cardinality grows with the user count (M7+) and provides little operator value — aggregate metrics are sufficient for diagnosing poll failures, auth anomalies, and cache pressure. This decision is recorded here so it is not revisited without a concrete use case.

**`feed_id` labels:** `tap_polls_total` and `tap_poll_duration_seconds` intentionally do not carry a `feed_id` label. Feed IDs are low-cardinality within a single deployment but the aggregate view is more useful for dashboards; per-feed detail is in logs and traces.

#### Scrape endpoint

`GET /metrics` is mounted on the main API listener (same port as the API and SPA). It is disabled by default; enabled via `--metrics-enabled`. The handler is `promhttp.HandlerFor` pointed at the bridge registry, with `promhttp.HandlerOpts{EnableOpenMetrics: false}` for maximum compatibility with Prometheus scrapers.

Because `/metrics` is on the main port, operators must not expose the main port externally without also considering metrics visibility. This is documented in the README. The loopback-default bind (concept §6.11) means a stock deployment is not exposed.

---

### New package: `internal/ratelimit`

```
internal/ratelimit/
  limiter.go       # Limiter, Opts, per-source + per-username state
  limiter_test.go
```

In-memory only. Resets on process restart — concept §3 explicitly blesses in-memory-only live counters.

#### Public API

```go
type Opts struct {
    SourceRate      rate.Limit    // requests/second for per-source bucket; default 10/60 (≈ 10/min)
    SourceBurst     int           // burst allowance; default 5
    FailThreshold   int           // consecutive failures before first lockout; default 5
    LockoutBase     time.Duration // initial lockout duration; default 30s
    LockoutMax      time.Duration // cap on escalating lockout; default 1h
    CleanupInterval time.Duration // GC interval for stale entries; default 5m
}

func NewLimiter(opts Opts) *Limiter

// Allow is called before credential verification.
// Returns (true, 0) if the request may proceed.
// Returns (false, retryAfter) if the source is rate-limited or the username is locked out.
// retryAfter is the duration the caller must wait before retrying.
func (l *Limiter) Allow(source, username string) (allowed bool, retryAfter time.Duration)

// RecordSuccess resets the failure counter and lockout escalation level for username.
// Called after a successful credential verification.
func (l *Limiter) RecordSuccess(username string)

// RecordFailure increments the per-source and per-username failure counters.
// If the per-username counter reaches FailThreshold, a lockout is applied with
// duration = LockoutBase * 2^(lockoutLevel), capped at LockoutMax.
// Called after a failed credential verification.
func (l *Limiter) RecordFailure(source, username string)

// Stop stops the background cleanup goroutine. Must be called on shutdown.
func (l *Limiter) Stop()
```

#### Escalating lockout

On each `RecordFailure` call that crosses the threshold:

```
lockoutDuration = min(LockoutBase * 2^lockoutLevel, LockoutMax)
lockoutLevel++
```

`lockoutLevel` is per-username and resets to 0 on `RecordSuccess`. So the sequence (with defaults) is: failure×5 → 30s, failure×5 more → 60s, failure×5 more → 120s, … capped at 1h.

Between lockout periods, additional failures during the lockout window do not re-trigger a new lockout — the existing lockout-until timestamp stands.

#### Source IP extraction

Source IP comes from `r.RemoteAddr`. When `--trusted-proxy` is set, the limiter extracts the first non-empty value from `X-Forwarded-For` instead. Without this flag, `X-Forwarded-For` is ignored to prevent source spoofing by an attacker who sets the header directly.

#### Login handler integration

In `internal/api/auth.go`, `loginHandler` is updated:

```go
// Before credential check:
allowed, retryAfter := limiter.Allow(source, body.Username)
if !allowed {
    w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
    writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
    metrics.LoginAttempts.Add(ctx, 1, label("result", "rate_limited"))
    return
}

// ... credential verification ...

if !ok {
    limiter.RecordFailure(source, body.Username)
    metrics.LoginAttempts.Add(ctx, 1, label("result", "failure"))
    writeError(w, http.StatusUnauthorized, "invalid_credentials", "...")
    return
}

limiter.RecordSuccess(body.Username)
metrics.LoginAttempts.Add(ctx, 1, label("result", "success"))
```

The limiter is constructed in `cmd/tap/main.go` and passed into `api.MuxOpts`.

#### Configuration knobs

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--login-rate` | `TAP_LOGIN_RATE` | `10/min` | Per-source requests per minute. Format: `N/min` or `N/s`. |
| `--login-burst` | `TAP_LOGIN_BURST` | `5` | Per-source burst allowance. |
| `--lockout-threshold` | `TAP_LOCKOUT_THRESHOLD` | `5` | Consecutive per-username failures before first lockout. |
| `--lockout-base` | `TAP_LOCKOUT_BASE` | `30s` | Initial lockout duration. |
| `--lockout-max` | `TAP_LOCKOUT_MAX` | `1h` | Maximum lockout duration after escalation. |
| `--trusted-proxy` | `TAP_TRUSTED_PROXY` | `false` | Trust `X-Forwarded-For` for source IP extraction. |

---

### `internal/auth` extension — re-hash-on-verify

New function in `internal/auth/argon2.go`:

```go
// NeedsRehash returns true if the encoded hash was produced with params
// that are strictly weaker than current on any axis (Time, Memory, or Threads).
// The PHC encoding carries the params used at hash time, so this comparison
// is purely a string parse — no crypto work.
// Returns an error if the encoded string is malformed.
func NeedsRehash(encoded string, current Params) (bool, error)
```

Login handler change in `internal/api/auth.go`, after a successful `auth.Verify`:

```go
if needsRehash, err := auth.NeedsRehash(user.PasswordHash, auth.DefaultParams); err == nil && needsRehash {
    if newHash, hashErr := auth.Hash(password, auth.DefaultParams); hashErr == nil {
        _ = db.UpdatePasswordHash(ctx, d, user.ID, newHash)
        slog.InfoContext(ctx, "auth.rehash", "user_id", user.ID)
        // No separate metric counter for rehash — auth.rehash log event is sufficient signal.
    }
}
```

The re-hash is synchronous (the plaintext password is available only in this window). `UpdatePasswordHash` failure is silently swallowed after logging — the login still succeeds; the re-hash will be retried on next login. The structured log event `auth.rehash` (see taxonomy below) informs operators that params have been upgraded.

M12 does not change the numeric values of `auth.DefaultParams` unless OWASP guidance has moved at the time of implementation. The re-hash-on-verify mechanism is the deliverable; the params bump is paired — if the values are raised, the mechanism automatically upgrades every user's hash on next login.

---

### OTel traces — `internal/tracing`

```
internal/tracing/
  provider.go     # TracerProvider, Init(opts), Shutdown, global default
  middleware.go   # HTTP server middleware (span + request_id)
  roundtripper.go # Outbound HTTP tracing RoundTripper wrapper
```

#### Provider

`tracing.Init(opts TracingOpts)` initialises an OTel `TracerProvider`:
- When `opts.OTLPEndpoint` is empty: a no-op provider (traces disabled).
- When set: an OTLP/HTTP or OTLP/gRPC exporter (protocol auto-detected from URL scheme: `http://` or `https://` → OTLP/HTTP; `grpc://` → OTLP/gRPC), with a `ParentBased(TraceIDRatio(opts.SampleRate))` sampler. Errors are always sampled regardless of `SampleRate`.

```go
type TracingOpts struct {
    OTLPEndpoint string
    OTLPHeaders  map[string]string
    SampleRate   float64 // 0.0–1.0; default 0.1
    ServiceName  string  // "tap"
    Version      string  // injected from build
}

func Init(opts TracingOpts) error
func Shutdown(ctx context.Context) error
```

`metrics.Init` and `tracing.Init` are called from the same block in `cmd/tap/main.go` during startup, before the HTTP server starts. Both `Shutdown` calls are deferred in reverse order of Init.

#### Inbound HTTP middleware

`tracing.Middleware` wraps each handler. For each request:

1. Generate a `request_id` (16 hex chars from `crypto/rand`).
2. Start an OTel span: name = `{method} {route_pattern}` (e.g. `GET /api/v1/entries/{id}`). Attributes: `http.method`, `http.route`, `http.status_code` (set on response), `http.request_content_length`, `request_id`.
3. Inject `request_id` into the request context via `slog.With` so every log line emitted downstream carries `request_id=...`.
4. Set `X-Request-ID` on the response header so upstream proxies and clients can correlate.
5. End the span on handler return, setting `http.status_code`.

The middleware sits outermost (before `requireSession`, `requireCSRF`).

#### Poll workflow spans

`internal/poll/worker.go` wraps each feed poll:

- Parent span: `poll.feed` — attributes: `feed_id`, `feed_url` (host only).
- Child span: `poll.fetch` — the outbound HTTP request (automatically covered by the RoundTripper wrapper below).
- Child span: `poll.parse` — gofeed parse.
- Child span: `poll.sanitise` — `processor.Process` for each new entry.
- Child span: `poll.commit` — DB transaction.

Worker spans are started with `context.Background()` (polls run outside inbound request context) but share the same `TracerProvider` so they appear in the same trace backend.

#### Outbound HTTP tracing RoundTripper

`tracing.NewRoundTripper(wrapped http.RoundTripper) http.RoundTripper` wraps the transport in `httpx.NewClient`. For each outbound request:

- Start a child span (child of the calling context's span, if any): name = `http.client {method}`. Attributes: `http.method`, `net.peer.name` (host only — never the full URL to avoid leaking credentials or tracking params in traces), `http.status_code` (set on response).
- End span on response return or error.

`httpx.NewClient` gains an optional `Tracer` field in `httpx.Opts`; when set, the RoundTripper chain gains the tracing wrapper outermost (after the host limiter).

#### OTLP exception for the metrics/tracing exporters themselves

The OTel SDK's OTLP HTTP client is a raw `*http.Client` constructed internally by the SDK — it does not go through Tap's `httpx.NewClient`. This is intentional and acceptable: OTLP endpoints are operator-configured server addresses, not user-supplied URLs, so SSRF risk is negligible. This exception is documented explicitly here and in the security audit section.

#### Configuration knobs

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--otlp-endpoint` | `TAP_OTLP_ENDPOINT` | (unset = disabled) | OTel collector endpoint. `http://`, `https://`, or `grpc://` scheme. Both metrics and traces export here. |
| `--otlp-headers` | `TAP_OTLP_HEADERS` | (unset) | Comma-separated `key=value` pairs for OTLP auth (e.g. Honeycomb, Grafana Cloud). |
| `--trace-sample-rate` | `TAP_TRACE_SAMPLE_RATE` | `0.1` | Fraction of normal traces to sample (0.0–1.0). Error traces always sampled. |
| `--metrics-enabled` | `TAP_METRICS_ENABLED` | `false` | Enable `GET /metrics` scrape endpoint on the main listener. |

---

### Structured log taxonomy

M1 landed `slog` with text/JSON format selectable at startup. M12 firms up the event taxonomy — named `event` keys and a consistent attribute set so logs are filterable and aggregatable downstream (Loki, ELK, etc.).

**Configuration:**

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--log-level` | `TAP_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. |
| `--log-format` | `TAP_LOG_FORMAT` | `text` (binary) / `json` (container) | `text` or `json`. Container Dockerfile sets `TAP_LOG_FORMAT=json`. |

**Event taxonomy:**

Every structured log event carries an `event` key. The table below lists all defined event values, their level, and required attributes.

| `event` value | Level | Required attributes | Notes |
|---|---|---|---|
| `poll.start` | debug | `feed_id`, `feed_url` | Emitted when a worker begins polling a feed. |
| `poll.success` | info | `feed_id`, `entries_inserted`, `duration_ms`, `conditional_hit` | `conditional_hit=true` when the origin returned 304. |
| `poll.failure` | warn | `feed_id`, `error`, `error_count` | `error_count` is the running consecutive failure counter on the subscription row. |
| `poll.skipped` | debug | `feed_id`, `reason` | Poll was due but not dispatched. `reason`: `backoff`, `in_flight`, `ssrf_blocked`. |
| `auth.login.success` | info | `user_id`, `username`, `source` | `source` is the client IP (from `RemoteAddr` or `X-Forwarded-For` if `--trusted-proxy`). |
| `auth.login.failure` | warn | `username`, `source`, `reason` | `reason`: `bad_credentials`, `disabled`, `locked_out`, `rate_limited`. Never logs the attempted password. |
| `auth.lockout` | warn | `username`, `source`, `lockout_duration_ms` | Emitted when a lockout is triggered (threshold crossed). |
| `auth.password_change` | info | `user_id` | Password successfully changed via `PATCH /api/v1/me/password`. |
| `auth.rehash` | info | `user_id` | Argon2 hash silently upgraded to current `DefaultParams` on successful login. |
| `auth.session.create` | debug | `user_id`, `session_id` | New session minted. |
| `auth.session.expire` | debug | `user_id`, `session_id`, `reason` | `reason`: `idle`, `absolute`. Emitted when `requireSession` deletes an expired row. |
| `archival.sweep.start` | info | — | Daily archival sweep begins. |
| `archival.sweep.complete` | info | `entries_deleted`, `tombstones_written`, `cache_files_evicted`, `duration_ms` | `tombstones_written` matches `entries_deleted` (one tombstone per deleted entry). M11 must emit these exact attribute keys. |
| `proxy.cache.hit` | debug | `token_prefix` | `token_prefix` = first 8 chars of the proxy token (not the full token). |
| `proxy.cache.miss` | debug | `token_prefix`, `origin_host` | `origin_host` only, not the full URL. |
| `proxy.cache.evict` | info | `files_evicted`, `bytes_freed` | Inline LRU eviction pass triggered by size cap. |
| `worker.panic` | error | `feed_id`, `stack` | Poll worker recovered from a panic. `stack` is the formatted stack trace. |
| `http.request` | info | `method`, `path`, `status`, `duration_ms`, `request_id` | Emitted by the request logging middleware for every inbound request. `path` is the route pattern, not the raw URL. |
| `startup` | info | `version`, `addr`, `data_dir` | Server started. |
| `shutdown` | info | `reason` | `reason`: `signal`, `deadline`. |

**`request_id` propagation:** Every inbound API request gets a `request_id` (16 hex chars from `crypto/rand`) generated by the tracing middleware and injected into the `slog` context. All log lines emitted during that request carry `request_id=...` automatically.

**Sensitive field discipline:**
- `password` is never logged.
- `username` appears in auth log events for operator diagnosis; absent from general HTTP logs.
- `source` (client IP) appears only in auth events where it is load-bearing for lockout diagnosis. It is absent from `http.request` events to avoid storing IPs unnecessarily.
- Per-feed credentials (`cookie`, `basic_auth_pass`) are never logged. `feed_url` in `poll.start`/`poll.failure` is the feed URL, not any credential.

---

### `internal/ring` — recent-errors ring buffer

```
internal/ring/
  buffer.go      # Buffer, Event, NewBuffer
  buffer_test.go
  handler.go     # slog.Handler wrapper that feeds the ring buffer
```

```go
type Event struct {
    Time  time.Time
    Level string         // "warn" or "error"
    Event string         // event taxonomy key, e.g. "poll.failure"
    Attrs map[string]any // all slog attributes on the log record
}

type Buffer struct { /* unexported — mutex-guarded ring */ }

func NewBuffer(capacity int) *Buffer  // default capacity 100

func (b *Buffer) Add(e Event)
func (b *Buffer) Recent(n int) []Event // newest-first, capped at min(n, len(buffer))
```

**`slog.Handler` wrapper:** `ring.NewHandler(next slog.Handler, buf *Buffer) slog.Handler` wraps any existing handler. On `Handle`, if the record's level is `>= slog.LevelWarn`, the event is added to `buf`. The `event` attribute value is extracted from the record's attributes to populate `Event.Event`; all other attributes go into `Event.Attrs`. This means the ring buffer and the log stream stay in sync with zero manual `buf.Add` call sites in business logic.

The ring buffer is constructed in `cmd/tap/main.go` and the `slog` default handler is wrapped before the server starts. Capacity defaults to 100; not configurable (YAGNI — 100 events is ample for the system-status panel's "last 20" view).

---

### `GET /healthz` extension

The existing `/healthz` endpoint (unauthenticated, `GET`) is extended to return a JSON body:

```json
{
  "status": "ok",
  "version": "0.12.0",
  "uptime_seconds": 3612,
  "db": "ok",
  "polls_active": 2
}
```

`status` is `"ok"` or `"degraded"`. `"degraded"` when the DB ping (`SELECT 1`) fails. `db` mirrors `status` for the DB check specifically. `polls_active` is the count of feed polls currently in-flight (in-process counter on the scheduler, already implied by concept §10). `uptime_seconds` is `time.Since(startTime)` rounded to seconds.

The endpoint remains unauthenticated — container orchestrators and `tap healthcheck` both call it without credentials. No `recent_errors` here; those require authentication.

---

### `GET /api/v1/status` — authenticated system status

New endpoint, authenticated, admin-only (`user.Role == "admin"` check after `requireSession`). No CSRF required (GET).

Response:

```json
{
  "version": "0.12.0",
  "uptime_seconds": 3612,
  "db": "ok",
  "polls_active": 2,
  "polls_total": 1847,
  "last_poll_at": 1746123456,
  "recent_errors": [
    {
      "time": "2026-05-10T09:00:00Z",
      "level": "warn",
      "event": "poll.failure",
      "attrs": {"feed_id": 12, "error": "connection refused", "error_count": 3}
    }
  ]
}
```

`recent_errors` returns the last 20 events from the ring buffer (newest first). `polls_total` and `last_poll_at` come from in-memory counters on the `Scheduler`. `polls_active` is the current in-flight count.

Non-admin authenticated users receive `403 forbidden` (new error code `ErrCodeForbidden = "forbidden"`). Unauthenticated requests receive `401 invalid_session`.

**SPA note:** The system-status panel is admin-only in the SPA. The settings view conditionally renders the panel section only when `$auth.user.role === 'admin'`. Other milestones' SPA work (M7 session listing, M8 settings polish) should be aware that the settings view has an admin-only section that must not be hidden by a blanket non-admin settings guard. This is flagged to the coordinator.

---

### `tap healthcheck` subcommand

New subcommand in `cmd/tap/admin.go` (alongside `tap admin`):

```
tap healthcheck [--addr <addr>] [--timeout <duration>]
```

Makes an HTTP GET to `http://{host}:{port}/healthz` derived from `--addr` (default `127.0.0.1:8080`). If the response is 200, exits 0. Any other outcome (non-200, connection refused, timeout) exits 1.

Timeout defaults to 5s. The subcommand is intentionally simple — it is the target of `HEALTHCHECK CMD ["/tap", "healthcheck"]` in the Dockerfile, where no shell or curl is available.

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD ["/tap", "healthcheck"]
```

---

### Admin CLI extensions

M6 landed `tap admin create` and `tap admin passwd`. M12 adds:

**`tap admin list [--data <dir>]`**

Prints a table of all users: ID, username, role, created-at (RFC3339), disabled status. Example output:

```
ID  USERNAME  ROLE   CREATED              DISABLED
1   ben       admin  2026-05-10T09:00:00Z no
2   alice     user   2026-05-10T10:00:00Z no
3   bob       user   2026-05-10T11:00:00Z yes (2026-05-11T08:00:00Z)
```

No password hashes, no session tokens. Exits 0 on success (including empty user table).

**`tap admin disable <username> [--data <dir>]`**

Sets `disabled_at` on the named user row (same column as M6's `DisableUser`). Calls `DeleteSessionsByUserID` to force-logout any active sessions. Stdout: `disabled user 'username'`. Exits 0 on success, 2 if user not found, exit 3 if the user is already disabled.

**`tap admin disable-totp`** is in M7's spec — it belongs alongside the 2FA implementation. It is not stubbed here.

---

### SPA: system-status panel

New section in the settings view (admin-only). Rendered only when `$auth.user.role === 'admin'`.

New file `web/src/lib/status.ts`:
```ts
export type StatusResponse = {
  version: string;
  uptime_seconds: number;
  db: 'ok' | 'degraded';
  polls_active: number;
  polls_total: number;
  last_poll_at: number | null;
  recent_errors: Array<{
    time: string;
    level: 'warn' | 'error';
    event: string;
    attrs: Record<string, unknown>;
  }>;
};

export async function getStatus(): Promise<StatusResponse> { /* GET /api/v1/status */ }
```

New component `web/src/components/SystemStatus.svelte`:
- Calls `getStatus()` on mount and every 60 seconds.
- Displays: version + uptime (human-readable, e.g. "3h 12m"), DB status (green/red indicator), active polls count, last poll time (relative, e.g. "2 minutes ago"), and a scrollable list of recent errors with timestamp, event key, and key attrs.
- Functional styling only — visual polish is M8.

---

### Security review pass

This section constitutes the M12 security audit. It is a structured checklist intended to be re-run against the codebase at the end of M12 implementation.

#### Route authn/authz table

Every route in `internal/api/api.go` must appear in this table, covering all routes from M1 through M12. Any gap found during implementation is a bug fix in M12.

| Route | Method | Session required | CSRF required | Admin only | Notes |
|---|---|---|---|---|---|
| `/healthz` | GET | No | No | No | Unauthenticated by design. |
| `/metrics` | GET | No | No | No | Enabled only with `--metrics-enabled`. Network boundary is the control (loopback-default). |
| `/api/v1/sessions` | POST | No | No | No | Login — public by definition. Rate-limited by `ratelimit.Limiter`. |
| `/api/v1/sessions/current` | GET | Yes | No | No | Session probe. Returns current user + CSRF token. |
| `/api/v1/sessions/current` | DELETE | Yes | Yes | No | Logout — deletes current session. |
| `/api/v1/me/password` | PATCH | Yes | Yes | No | Password change. |
| `/api/v1/me/totp` | POST | Yes | Yes | No | M7. Begin TOTP enrolment (returns secret_uri). |
| `/api/v1/me/totp/confirm` | POST | Yes | Yes | No | M7. Confirm TOTP enrolment with 6-digit code; returns recovery codes. |
| `/api/v1/me/totp` | DELETE | Yes | Yes | No | M7. Disable TOTP (requires current code or recovery code). |
| `/api/v1/me/totp/recovery-codes` | POST | Yes | Yes | No | M7. Regenerate recovery codes (requires current TOTP code). |
| `/api/v1/me/passkeys/registration/begin` | POST | Yes | Yes | No | M7. Begin WebAuthn passkey registration ceremony. |
| `/api/v1/me/passkeys/registration/finish` | POST | Yes | Yes | No | M7. Finish WebAuthn passkey registration; stores credential. |
| `/api/v1/me/passkeys` | GET | Yes | No | No | M7. List caller's registered passkeys (id, label, created_at only — no public key). |
| `/api/v1/me/passkeys/{id}` | DELETE | Yes | Yes | No | M7. Remove a passkey. 404 if not found or belongs to another user. |
| `/api/v1/passkey-sessions/begin` | POST | No | No | No | M7. Begin passkey login ceremony — creates anonymous challenge session. |
| `/api/v1/passkey-sessions/finish` | POST | No | No | No | M7. Finish passkey login — upgrades anonymous session to a user session. |
| `/api/v1/sessions` | GET | Yes | No | No | M7. List all active sessions for the current user. |
| `/api/v1/sessions/{id}` | DELETE | Yes | Yes | No | M7. Revoke a specific session. 403 if it is the current session. |
| `/api/v1/sessions` | DELETE | Yes | Yes | No | M7. Revoke all sessions except the current one ("log out everywhere"). |
| `/api/v1/admin/users` | GET | Yes | No | Yes | M7. List all users. |
| `/api/v1/admin/users` | POST | Yes | Yes | Yes | M7. Create a user. |
| `/api/v1/admin/users/{id}` | PATCH | Yes | Yes | Yes | M7. Update role or disabled status. |
| `/api/v1/admin/users/{id}/password-reset` | POST | Yes | Yes | Yes | M7. Issue a one-time temporary password; force-logout that user. |
| `/api/v1/admin/users/{id}/disable-totp` | POST | Yes | Yes | Yes | M7. Remove another user's TOTP secret + recovery codes. |
| `/api/v1/admin/users/{id}` | DELETE | Yes | Yes | Yes | M7. Delete a user (cascades subscriptions, entries, sessions). |
| `/api/v1/status` | GET | Yes | No | Yes | M12. System-status panel data. Admin-only. |
| `/api/v1/subscriptions` | GET | Yes | No | No | List caller's feeds. |
| `/api/v1/subscriptions` | POST | Yes | Yes | No | Create feed. |
| `/api/v1/subscriptions/{id}` | GET | Yes | No | No | M9. Get a single subscription. 404 if not found or belongs to another user. |
| `/api/v1/subscriptions/{id}` | PATCH | Yes | Yes | No | Update feed. |
| `/api/v1/subscriptions/{id}` | DELETE | Yes | Yes | No | Delete feed. |
| `/api/v1/entries` | GET | Yes | No | No | List caller's entries. Accepts `?category=<id>` (M9). |
| `/api/v1/entries/{id}` | GET | Yes | No | No | Get entry (full body). |
| `/api/v1/entries/{id}` | PATCH | Yes | Yes | No | Mark read/saved. |
| `/api/v1/categories` | GET | Yes | No | No | M9. List caller's categories with unread counts. |
| `/api/v1/categories` | POST | Yes | Yes | No | M9. Create a category. |
| `/api/v1/categories/{id}` | PATCH | Yes | Yes | No | M9. Rename a category. |
| `/api/v1/categories/{id}` | DELETE | Yes | Yes | No | M9. Delete a category (feeds become uncategorised). |
| `/api/v1/categories/{id}/mark-read` | POST | Yes | Yes | No | M9. Bulk mark all entries in category as read. |
| `/api/v1/search` | GET | Yes | No | No | M9. FTS5 search across caller's entries. |
| `/api/v1/opml` | GET | Yes | No | No | M9. Export OPML 2.0. |
| `/api/v1/opml` | POST | Yes | Yes | No | M9. Import OPML. Body cap: 10 MiB (per-route override). |
| `/api/v1/discover` | POST | Yes | Yes | No | M9. Discover feed candidates from a URL. |
| `/api/v1/proxy/{token}` | GET | Yes | No | No | Media proxy — GET, no CSRF. Signed token gates access. |

#### Outbound HTTP SSRF posture

Every outbound HTTP call must go through `httpx.NewClient`. The following paths are verified:

| Path | Client | Notes |
|---|---|---|
| Feed fetch (`internal/poll/worker.go`) | `httpx.NewClient` | SSRF-guarded, per-host cap applied. |
| Article extraction (`internal/extract`) | `httpx.NewClient` | SSRF-guarded, per-host cap applied. |
| Media proxy origin fetch (`internal/proxy/handler.go`) | `httpx.NewClient` | SSRF-guarded, per-host cap applied. |
| OTLP metric/trace export (OTel SDK) | OTel SDK internal client | **Exception.** Operator-configured endpoint; not user-supplied. SSRF risk negligible. Documented explicitly. |

Any `http.Get`, `http.Post`, or inline `http.Client{}` construction found outside this table during M12 implementation is a bug fix.

#### Input validation — `MaxBytesReader` audit

Every write handler must apply `http.MaxBytesReader(w, r.Body, maxBytes)` before `json.Decode`. The following handlers are verified as part of M12 (including the deferred-items fix for `subscriptions.go` and `entries.go`):

| Handler | Body cap | Status |
|---|---|---|
| `POST /api/v1/sessions` | 1 MiB | M6 fixed |
| `PATCH /api/v1/me/password` | 1 MiB | M6 fixed |
| `POST /api/v1/subscriptions` | 1 MiB | **Deferred item — fixed in M12** |
| `PATCH /api/v1/subscriptions/{id}` | 1 MiB | **Deferred item — fixed in M12** |
| `PATCH /api/v1/entries/{id}` | 1 MiB | **Deferred item — fixed in M12** |
| `POST /api/v1/me/totp` | 1 MiB | M7 — verify present |
| `POST /api/v1/me/totp/confirm` | 1 MiB | M7 — verify present |
| `DELETE /api/v1/me/totp` | 1 MiB | M7 — verify present |
| `POST /api/v1/me/totp/recovery-codes` | 1 MiB | M7 — verify present |
| `POST /api/v1/me/passkeys/registration/begin` | 1 MiB | M7 — verify present |
| `POST /api/v1/me/passkeys/registration/finish` | 1 MiB | M7 — verify present |
| `POST /api/v1/passkey-sessions/begin` | 1 MiB | M7 — verify present |
| `POST /api/v1/passkey-sessions/finish` | 1 MiB | M7 — verify present |
| `POST /api/v1/admin/users` | 1 MiB | M7 — verify present |
| `PATCH /api/v1/admin/users/{id}` | 1 MiB | M7 — verify present |
| `POST /api/v1/admin/users/{id}/password-reset` | 1 MiB | M7 — verify present |
| `POST /api/v1/admin/users/{id}/disable-totp` | 1 MiB | M7 — verify present |
| `POST /api/v1/categories` | 1 MiB | M9 — verify present |
| `PATCH /api/v1/categories/{id}` | 1 MiB | M9 — verify present |
| `POST /api/v1/categories/{id}/mark-read` | (no body) | M9 — no body cap needed |
| `POST /api/v1/opml` | **10 MiB** | M9 — per-route override (OPML files can be large) |
| `POST /api/v1/discover` | 1 MiB | M9 — verify present |

All write handlers with a body cap return `413 Request Entity Too Large` (via `errors.As(*http.MaxBytesError)`) when the body exceeds the cap. The M9 OPML import handler uses a 10 MiB cap — the roadmap deferred-items entry explicitly called out that the default 1 MiB cap is too small for typical OPML files. The M12 audit verifies the 10 MiB cap is applied on that route and not the default. The "deferred item — fixed in M12" rows close the roadmap deferred-items entry for the three M6-era handlers.

#### Tooling

Two tools run as part of M12's definition of done:

- **`govulncheck ./...`** — zero findings, or each finding explicitly documented with accepted-risk rationale.
- **`semgrep --config=p/golang`** — zero findings at `ERROR` severity, or each finding explicitly documented with accepted-risk rationale. The `/security-review` skill is invoked during implementation to drive this pass.

Any finding that cannot be fixed within M12 scope (e.g. a transitive dependency with no fix available) is documented in a `docs/security-exceptions.md` file with: the finding, the severity, the affected path, the reason it is accepted, and the condition under which it should be revisited.

---

### New dependencies

All pure-Go, no CGO. All compatible with `CGO_ENABLED=0`.

| Module | Purpose | License |
|---|---|---|
| `go.opentelemetry.io/otel` | OTel SDK core | Apache-2.0 |
| `go.opentelemetry.io/otel/sdk/metric` | Metric SDK | Apache-2.0 |
| `go.opentelemetry.io/otel/sdk/trace` | Trace SDK | Apache-2.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp` | OTLP metric export (HTTP) | Apache-2.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` | OTLP trace export (HTTP) | Apache-2.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc` | OTLP metric export (gRPC) | Apache-2.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | OTLP trace export (gRPC) | Apache-2.0 |
| `go.opentelemetry.io/otel/bridge/prometheus` | OTel → Prometheus bridge | Apache-2.0 |
| `go.opentelemetry.io/contrib/instrumentation/runtime` | Go runtime metrics | Apache-2.0 |
| `github.com/prometheus/client_golang/prometheus` | Prometheus registry + types | Apache-2.0 |
| `github.com/prometheus/client_golang/prometheus/promhttp` | Prometheus HTTP handler | Apache-2.0 |
| `golang.org/x/time/rate` | Token bucket (per-source rate limit) | BSD-3-Clause |

`golang.org/x/time` is likely already transitively present; if so, no new dependency.

---

### Configuration summary (all new M12 knobs)

| Flag | Env | Default | Section |
|---|---|---|---|
| `--metrics-enabled` | `TAP_METRICS_ENABLED` | `false` | Metrics |
| `--otlp-endpoint` | `TAP_OTLP_ENDPOINT` | (unset) | Tracing/Metrics |
| `--otlp-headers` | `TAP_OTLP_HEADERS` | (unset) | Tracing/Metrics |
| `--trace-sample-rate` | `TAP_TRACE_SAMPLE_RATE` | `0.1` | Tracing |
| `--log-level` | `TAP_LOG_LEVEL` | `info` | Logging |
| `--log-format` | `TAP_LOG_FORMAT` | `text` | Logging |
| `--login-rate` | `TAP_LOGIN_RATE` | `10/min` | Rate limit |
| `--login-burst` | `TAP_LOGIN_BURST` | `5` | Rate limit |
| `--lockout-threshold` | `TAP_LOCKOUT_THRESHOLD` | `5` | Lockout |
| `--lockout-base` | `TAP_LOCKOUT_BASE` | `30s` | Lockout |
| `--lockout-max` | `TAP_LOCKOUT_MAX` | `1h` | Lockout |
| `--trusted-proxy` | `TAP_TRUSTED_PROXY` | `false` | Rate limit / Logging |

---

### `cmd/tap/main.go` wire-up additions

M12 adds the following to the server startup block in `cmd/tap/main.go`, before the HTTP server starts:

```go
// 1. Initialise metrics and tracing providers (both shut down after sched.Stop in reverse order).
if err := metrics.Init(metrics.MetricsOpts{
    OTLPEndpoint: *otlpEndpoint,
    OTLPHeaders:  parseHeaders(*otlpHeaders),
}); err != nil {
    slog.Error("init metrics", "err", err); os.Exit(1)
}
defer metrics.Shutdown(shutdownCtx)

if err := tracing.Init(tracing.TracingOpts{
    OTLPEndpoint: *otlpEndpoint,
    OTLPHeaders:  parseHeaders(*otlpHeaders),
    SampleRate:   *traceSampleRate,
    ServiceName:  "tap",
    Version:      version,
}); err != nil {
    slog.Error("init tracing", "err", err); os.Exit(1)
}
defer tracing.Shutdown(shutdownCtx)

// 2. Wrap the slog default handler with the ring buffer handler.
ringBuf := ring.NewBuffer(100)
slog.SetDefault(slog.New(ring.NewHandler(slog.Default().Handler(), ringBuf)))

// 3. Wire the Archiver's OnEvict callback to the proxy cache evictions counter.
archiver := archival.NewArchiver(d, archival.ArchiverOpts{
    Horizon:     *archiveHorizon,
    CacheAgeCap: *cacheAgeCap,
    Interval:    *archiveInterval,
    CacheDir:    cacheDir,
    OnEvict: func(n int) {
        metrics.ProxyCacheEvictions.Add(context.Background(), int64(n),
            metric.WithAttributes(attribute.String("reason", "age_sweep")))
    },
})

// 4. Pass the ring buffer and rate limiter into the API mux.
limiter := ratelimit.NewLimiter(ratelimit.Opts{
    SourceRate:      parseRate(*loginRate),
    SourceBurst:     *loginBurst,
    FailThreshold:   *lockoutThreshold,
    LockoutBase:     *lockoutBase,
    LockoutMax:      *lockoutMax,
    CleanupInterval: 5 * time.Minute,
})
defer limiter.Stop()

apiMux := api.NewMux(d, api.MuxOpts{
    // ... existing fields ...
    RingBuffer:    ringBuf,
    Limiter:       limiter,
    TrustedProxy:  *trustedProxy,
    MetricsEnabled: *metricsEnabled,
    StartTime:     startTime,
    Version:       version,
})
```

Shutdown ordering (additions to the existing reverse-order sequence):

```
srv.Shutdown(ctx)   // drain HTTP first
archiver.Stop()     // let any in-progress sweep finish
sched.Stop()        // let in-flight polls finish
limiter.Stop()      // stop cleanup goroutine
tracing.Shutdown()  // flush traces
metrics.Shutdown()  // flush metrics
db.Close()          // close DB last
```

`metrics.Shutdown` and `tracing.Shutdown` are deferred immediately after `Init` so they run in LIFO order relative to `db.Close`. The `defer` ordering means `db.Close` runs after both flush — correct.

---

## Out of scope (deferred or won't ship)

| Concern | Notes |
|---|---|
| `tap admin disable-totp` | In M7's spec — landed alongside the 2FA implementation. Not added here. |
| Per-user metrics labels (`user_id` on any instrument) | Decided against: high cardinality, low operator value. Not revisited without a concrete use case. |
| Persistent lockout state (survives restart) | In-memory only per concept §3. |
| Metrics endpoint on a separate port | Same port as main API; network boundary is the control. |
| Log file output | Logs are structured to stdout only (concept §10). Operators forward via Docker logging driver or systemd journal. |
| Audit log table | Concept §10: stdout is the audit log. No separate DB table. |
| OTLP metric + trace to different endpoints | Both go to the same `--otlp-endpoint`. Operators with separate metric/trace backends use an OTel collector to fan out. |
| Baggage propagation across service boundaries | Tap is single-process; no cross-service context to propagate. |

## Tests and methodology

M12 follows the TDD discipline of M1–M11. Pure scaffolding (flag declarations, metric description strings, Dockerfile HEALTHCHECK line) is exempt. Everything with branches, error handling, or state is in scope.

### `internal/metrics`

- `Init` with a no-op exporter succeeds; `Shutdown` succeeds.
- `Init` called twice returns an error (double-init guard).
- After a counter increment, the Prometheus exposition output contains the expected `# HELP` description string, `# TYPE`, and the correct value.
- All instrument calls before `Init` do not panic (no-op provider).
- `Init` with an `OTLPEndpoint` set registers both the Prometheus bridge and the OTLP exporter.

### `internal/ratelimit`

- `Allow(source, username)` returns `(true, 0)` with no prior state.
- After N `RecordFailure` calls on the same username (N = `FailThreshold`), `Allow` returns `(false, >0)`.
- Lockout duration doubles on successive lockout threshold crossings: lockout 1 = `LockoutBase`, lockout 2 = `LockoutBase*2`, lockout 3 = `LockoutBase*4`, capped at `LockoutMax`.
- `RecordSuccess` after N failures resets failure counter; subsequent `RecordFailure` sequence starts escalation from level 0 again.
- Per-source rate limit: burst exhausted → `Allow` returns `(false, >0)` for that source.
- Cleanup goroutine prunes stale entries (test with `CleanupInterval=10ms`, `LockoutMax=20ms`; verify map shrinks).
- Concurrent `Allow` + `RecordFailure` calls from multiple goroutines under `-race` do not race.
- `Stop` terminates the cleanup goroutine cleanly (no goroutine leak, verified with `goleak`).

### `internal/auth` (extension)

- `NeedsRehash` returns `false` for a hash produced with `DefaultParams`.
- `NeedsRehash` returns `true` for a hash produced with `Params{Time: 1, Memory: 32*1024, Threads: 1, SaltLen: 16, KeyLen: 32}` (weaker than `DefaultParams`).
- `NeedsRehash` returns `true` when only `Memory` is weaker.
- `NeedsRehash` returns `true` when only `Time` is weaker.
- `NeedsRehash` returns error on a malformed PHC string.
- Login handler integration: when user's stored hash has weaker params, a successful login calls `db.UpdatePasswordHash`; the new hash parses to `DefaultParams`.
- Login handler integration: `UpdatePasswordHash` failure is swallowed (login still returns 200, no error to client).

### `internal/ring`

- `Add` beyond capacity overwrites the oldest entry (ring semantics).
- `Recent(n)` returns newest-first, length = `min(n, items_added, capacity)`.
- `Recent(0)` returns empty slice.
- Concurrent `Add` + `Recent` calls from multiple goroutines under `-race` do not race.
- `slog.Handler` wrapper: a `warn`-level log line via `slog.Default()` adds to the buffer; a `debug`-level line does not; an `error`-level line does.
- Handler wrapper: `Event.Event` is populated from the `event` attribute key in the log record.
- Handler wrapper: the wrapped `next` handler still receives every record regardless of level (the ring buffer is additive, not filtering).

### `internal/tracing`

- `Init` with empty `OTLPEndpoint` succeeds and sets a no-op provider; `Shutdown` succeeds.
- `Init` with `OTLPEndpoint = "http://localhost:4318"` registers an OTLP/HTTP exporter.
- `Init` with `OTLPEndpoint = "grpc://localhost:4317"` registers an OTLP/gRPC exporter.
- HTTP middleware: every request produces a span with `http.method` and `http.route` attributes set; `http.status_code` is set after the handler returns.
- HTTP middleware: `X-Request-ID` response header is set; the value is 16 hex chars.
- HTTP middleware: log lines emitted during the handler carry `request_id` matching the response header value.
- RoundTripper wrapper: outbound HTTP call produces a child span with `net.peer.name` set to the host only (no path, no query string, no credentials).

### `internal/api` (extensions)

- `GET /metrics` returns 200 with `Content-Type: text/plain` containing `# HELP` lines when `--metrics-enabled`.
- `GET /metrics` returns 404 when metrics are disabled.
- `GET /api/v1/status` returns 200 with correct shape for an admin session.
- `GET /api/v1/status` returns 403 `forbidden` for a user-role session.
- `GET /api/v1/status` returns 401 `invalid_session` with no session.
- `GET /api/v1/status` `recent_errors` array contains the last 20 ring buffer events, newest first.
- `GET /healthz` returns 200 with JSON body containing `status`, `version`, `uptime_seconds`, `db`, `polls_active`.
- `GET /healthz` returns `status: "degraded"` when DB ping fails (inject error via interface).
- Login: `Allow` returns false → 429 with `Retry-After` header; `tap_login_attempts_total{result="rate_limited"}` incremented.
- Login: valid credentials after lockout clears (after `retryAfter` elapses) → 200.
- Login: successful login with weak-params hash → `UpdatePasswordHash` called; `tap_login_attempts_total{result="success"}` incremented.
- `tap_http_requests_total` and `tap_http_request_duration_seconds` are incremented/recorded after each request (verified via Prometheus gather).

### `cmd/tap` (healthcheck)

- `tap healthcheck` against a running server on the correct port returns exit 0.
- `tap healthcheck` when no server is listening returns exit 1 within the timeout.
- `tap healthcheck` against a server returning non-200 returns exit 1.

### `cmd/tap/admin` (extensions)

- `tap admin list` against an empty DB prints the header and no rows; exits 0.
- `tap admin list` with two users prints both rows correctly; exits 0.
- `tap admin disable <username>`: `disabled_at` set, sessions deleted, exits 0.
- `tap admin disable <username>` for unknown user: stderr message, exits 2.
- `tap admin disable <username>` for already-disabled user: exits 3.
- Login with a `tap admin disable`-d account returns 401 `invalid_credentials`.

### SPA

- `status.test.ts`: `GET /api/v1/status` 200 → `StatusResponse` type populated; `recent_errors` array renders correctly.
- `status.test.ts`: 403 response → panel section does not render.
- `SystemStatus.svelte` renders version, uptime, DB status, polls_active, last_poll_at (relative time), and recent_errors list.
- Settings view: `SystemStatus` section is rendered only when `$auth.user.role === 'admin'`; not rendered for user-role sessions.

### Security review (definition of done)

- Route authn/authz table (above) verified against the actual `internal/api/api.go` mux registration. Any discrepancy is a bug fix.
- All write handlers confirmed to have `http.MaxBytesReader` with 1 MiB cap; all return 413 on oversize body.
- No `http.Get`, `http.Post`, or inline `http.Client{}` construction outside the documented OTLP exception.
- `govulncheck ./...` — zero findings, or each finding in `docs/security-exceptions.md`.
- `semgrep --config=p/golang` — zero `ERROR`-severity findings, or each finding in `docs/security-exceptions.md`.
- `/security-review` skill invoked and its output reviewed.

## Definition of done

1. `make test` passes (`go test ./... -race`) including all new packages.
2. `--metrics-enabled`: `GET /metrics` returns valid Prometheus exposition with `# HELP` descriptions for every instrument in the table above.
3. `--otlp-endpoint` set: metrics and traces export to the endpoint without error on startup.
4. Default config (no `--metrics-enabled`, no `--otlp-endpoint`): no extra overhead; `/metrics` returns 404; no OTLP connections attempted.
5. Login with N+1 consecutive wrong passwords for the same username: `Allow` returns 429 with `Retry-After`; the lockout clears after the returned duration.
6. Login with a password stored under weaker argon2 params: subsequent `GET /api/v1/me` (or login) shows the hash has been upgraded to `DefaultParams`.
7. `tap healthcheck` exits 0 against a running server; exits 1 with no server.
8. `tap admin list` shows all users; `tap admin disable <user>` sets `disabled_at` and deletes their sessions; subsequent login for that user returns 401.
9. `GET /api/v1/status` returns 200 for admin, 403 for user role, 401 for no session.
10. `GET /healthz` returns JSON body with correct fields.
11. The route authn/authz table is verified against actual mux registration; all write handlers have `MaxBytesReader`.
12. `govulncheck ./...` and `semgrep --config=p/golang` pass (or findings documented in `docs/security-exceptions.md`).
13. `/security-review` skill has been run and its output reviewed; any actionable findings resolved or documented.
14. The deferred-items fix (413 on oversize body for `subscriptions.go` and `entries.go`) is included and tested.
15. `make build` produces a static binary; `docker build` produces a distroless image with a working `HEALTHCHECK`.

## What this milestone deliberately does not prove

- That per-user metrics labels (`user_id`) are useful — decided against; see Out of scope.
- That OTLP lockout state survives restart — in-memory by design.
- That `tap admin disable-totp` works — M7 owns 2FA and that subcommand.
- That the metrics endpoint requires authentication — network boundary is the control (loopback-default). Operators exposing Tap externally are responsible for their own reverse-proxy auth on `/metrics` if needed.
- That Tap integrates with any specific observability backend — Tap emits; the backend is the operator's choice.
