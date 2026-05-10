# M12 — Observability + Production Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Tap production-ready for unattended self-hosted operation by adding Prometheus metrics, OTel traces, structured log taxonomy, brute-force lockout, argon2 re-hash-on-verify, a recent-errors ring buffer, system-status panel, healthcheck subcommand, and a full security audit pass.

**Architecture:** Three new packages (`internal/metrics`, `internal/ratelimit`, `internal/ring`) integrate with existing packages via the global OTel provider pattern; `internal/tracing` adds a tracing package. The `cmd/tap/main.go` wire-up section in the spec is the integration point. TDD throughout — rate limiter, ring buffer, re-hash logic, and admin CLI are all unit-testable.

**Tech Stack:** `go.opentelemetry.io/otel` SDK, `go.opentelemetry.io/otel/bridge/prometheus` + `github.com/prometheus/client_golang`, OTLP HTTP/gRPC exporters, `golang.org/x/time/rate` (token bucket), stdlib `log/slog`.

**Spec:** `docs/specs/2026-05-10-m12-observability-hardening.md`

**Skills to invoke per task (listed inline below).**

---

## File Map

### New files
| File | Purpose |
|---|---|
| `internal/metrics/provider.go` | OTel MeterProvider + Prometheus bridge init/shutdown |
| `internal/metrics/instruments.go` | All metric instrument definitions with `# HELP` descriptions |
| `internal/metrics/handler.go` | `promhttp` HTTP handler for `GET /metrics` |
| `internal/metrics/provider_test.go` | Provider init/shutdown, no-op safety, double-init guard |
| `internal/metrics/instruments_test.go` | Prometheus text exposition contains expected metric + description |
| `internal/tracing/provider.go` | OTel TracerProvider init/shutdown, no-op when unconfigured |
| `internal/tracing/middleware.go` | Inbound HTTP span + `request_id` middleware |
| `internal/tracing/roundtripper.go` | Outbound HTTP tracing RoundTripper wrapper |
| `internal/tracing/provider_test.go` | Init with/without endpoint, Shutdown |
| `internal/tracing/middleware_test.go` | Span created, X-Request-ID header set, request_id in logs |
| `internal/ring/buffer.go` | Fixed-capacity ring buffer of structured error events |
| `internal/ring/handler.go` | `slog.Handler` wrapper that feeds the ring buffer |
| `internal/ring/buffer_test.go` | Ring semantics, Recent(), concurrency, handler integration |
| `internal/ratelimit/limiter.go` | Per-source token bucket + per-username escalating lockout |
| `internal/ratelimit/limiter_test.go` | Allow/Record*, lockout escalation, cleanup, race safety |

### Modified files
| File | What changes |
|---|---|
| `internal/auth/argon2.go` | Add `NeedsRehash(encoded string, current Params) (bool, error)` |
| `internal/auth/argon2_test.go` | Tests for `NeedsRehash` |
| `internal/api/api.go` | Add `MuxOpts` fields: `Limiter`, `RingBuffer`, `TrustedProxy`, `MetricsEnabled`, `StartTime`, `Version`; mount `/metrics`, `/api/v1/status` |
| `internal/api/auth.go` | Wire limiter into login handler; add re-hash-on-verify; emit structured log events |
| `internal/api/auth_test.go` | Rate-limited login returns 429; re-hash triggered on weak params |
| `internal/api/status.go` | New: `GET /api/v1/status` handler (admin-only) |
| `internal/api/status_test.go` | New: 200 admin, 403 user, 401 unauthed, response shape |
| `internal/api/middleware.go` | Wrap tracing middleware outermost; add `requireAdmin` helper |
| `internal/api/errors.go` | Add `ErrCodeForbidden`, `ErrCodeRateLimited` |
| `internal/poll/worker.go` | Emit `poll.start`/`poll.success`/`poll.failure` log events; increment poll metrics; add poll spans |
| `internal/proxy/handler.go` | Increment `tap_proxy_cache_hits_total` / `tap_proxy_cache_misses_total` |
| `internal/proxy/cache.go` | Increment `tap_proxy_cache_evictions_total{reason=size_cap}` and `tap_proxy_cache_bytes` |
| `internal/httpx/client.go` | Accept optional `TracingRoundTripper` in `Opts`; wrap transport when set |
| `cmd/tap/main.go` | Add 12 new flags; wire metrics/tracing Init; wrap slog handler; wire Limiter + RingBuffer into MuxOpts; wire Archiver `OnEvict`; extend shutdown sequence; emit `startup`/`shutdown` events; extend `/healthz` JSON body |
| `cmd/tap/admin.go` | Add `tap admin list`, `tap admin disable`, `tap healthcheck` subcommands |
| `cmd/tap/admin_test.go` | Tests for new subcommands |
| `web/src/lib/status.ts` | New: `getStatus()` + `StatusResponse` type |
| `web/src/lib/api.ts` | Wire `getStatus` |
| `web/src/components/SystemStatus.svelte` | New: system-status panel component |
| `web/src/views/Settings.svelte` | Conditionally render `<SystemStatus>` for admin role |
| `web/src/lib/__tests__/status.test.ts` | New: status fetch, 403 path |

---

## Task 1: Add new dependencies to go.mod

**Skills:** `golang-dependency-management`

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add OTel SDK and exporter dependencies**

```bash
cd /home/ben.guest/Users/ben/src/tap
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/sdk@latest
go get go.opentelemetry.io/otel/sdk/metric@latest
go get go.opentelemetry.io/otel/sdk/trace@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp@latest
go get go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc@latest
go get go.opentelemetry.io/otel/bridge/prometheus@latest
go get go.opentelemetry.io/contrib/instrumentation/runtime@latest
go get github.com/prometheus/client_golang/prometheus@latest
go get github.com/prometheus/client_golang/prometheus/promhttp@latest
```

- [ ] **Step 2: Verify go.mod resolves cleanly**

```bash
go mod tidy
go build ./...
```

Expected: no build errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "M12: add OTel SDK, Prometheus client, and exporter dependencies"
```

---

## Task 2: `internal/ring` — ring buffer + slog handler

**Skills:** `superpowers:test-driven-development`, `golang-concurrency`, `golang-testing`

**Files:**
- Create: `internal/ring/buffer.go`
- Create: `internal/ring/handler.go`
- Create: `internal/ring/buffer_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/ring/buffer_test.go`:

```go
package ring_test

import (
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/ring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuffer_RingSemantics(t *testing.T) {
	buf := ring.NewBuffer(3)
	buf.Add(ring.Event{Time: time.Now(), Level: "warn", Event: "a"})
	buf.Add(ring.Event{Time: time.Now(), Level: "warn", Event: "b"})
	buf.Add(ring.Event{Time: time.Now(), Level: "warn", Event: "c"})
	buf.Add(ring.Event{Time: time.Now(), Level: "warn", Event: "d"}) // overwrites "a"

	got := buf.Recent(10)
	require.Len(t, got, 3)
	// newest first
	assert.Equal(t, "d", got[0].Event)
	assert.Equal(t, "c", got[1].Event)
	assert.Equal(t, "b", got[2].Event)
}

func TestBuffer_RecentCapped(t *testing.T) {
	buf := ring.NewBuffer(10)
	buf.Add(ring.Event{Time: time.Now(), Level: "error", Event: "x"})
	buf.Add(ring.Event{Time: time.Now(), Level: "error", Event: "y"})

	got := buf.Recent(1)
	require.Len(t, got, 1)
	assert.Equal(t, "y", got[0].Event)
}

func TestBuffer_RecentZero(t *testing.T) {
	buf := ring.NewBuffer(10)
	got := buf.Recent(0)
	assert.Empty(t, got)
}

func TestBuffer_Concurrent(t *testing.T) {
	buf := ring.NewBuffer(100)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf.Add(ring.Event{Time: time.Now(), Level: "warn", Event: "concurrent"})
			_ = buf.Recent(5)
		}()
	}
	wg.Wait()
}

func TestHandler_WarnAndErrorFeedBuffer(t *testing.T) {
	buf := ring.NewBuffer(10)
	h := ring.NewHandler(slog.DiscardHandler, buf)
	logger := slog.New(h)

	logger.Info("this is info", "event", "info.event")
	logger.Warn("this is warn", "event", "warn.event")
	logger.Error("this is error", "event", "error.event")
	logger.Debug("this is debug", "event", "debug.event")

	got := buf.Recent(10)
	require.Len(t, got, 2) // only warn + error
	assert.Equal(t, "error.event", got[0].Event) // newest first
	assert.Equal(t, "warn.event", got[1].Event)
}

func TestHandler_EventKeyExtracted(t *testing.T) {
	buf := ring.NewBuffer(10)
	h := ring.NewHandler(slog.DiscardHandler, buf)
	slog.New(h).Warn("poll failed", "event", "poll.failure", "feed_id", 42)

	got := buf.Recent(1)
	require.Len(t, got, 1)
	assert.Equal(t, "poll.failure", got[0].Event)
	assert.Equal(t, int64(42), got[0].Attrs["feed_id"])
}

func TestHandler_NextHandlerReceivesAllRecords(t *testing.T) {
	buf := ring.NewBuffer(10)
	var count int
	counting := &countingHandler{&count}
	h := ring.NewHandler(counting, buf)
	logger := slog.New(h)

	logger.Debug("d")
	logger.Info("i")
	logger.Warn("w")
	logger.Error("e")

	assert.Equal(t, 4, count) // all records forwarded
	assert.Len(t, buf.Recent(10), 2) // only warn+error buffered
}

type countingHandler struct{ n *int }

func (c *countingHandler) Enabled(_ context.Context, _ slog.Level) bool  { return true }
func (c *countingHandler) Handle(_ context.Context, _ slog.Record) error  { *c.n++; return nil }
func (c *countingHandler) WithAttrs(_ []slog.Attr) slog.Handler           { return c }
func (c *countingHandler) WithGroup(_ string) slog.Handler                { return c }
```

- [ ] **Step 2: Run tests — expect compile failure**

```bash
go test ./internal/ring/... 2>&1 | head -20
```

Expected: package not found.

- [ ] **Step 3: Implement `internal/ring/buffer.go`**

```go
// Package ring provides an in-memory fixed-capacity ring buffer for structured
// error events, and a slog.Handler wrapper that feeds it automatically.
package ring

import (
	"sync"
	"time"
)

// Event is a structured error or warning captured from the slog stream.
type Event struct {
	Time  time.Time
	Level string         // "warn" or "error"
	Event string         // event taxonomy key, e.g. "poll.failure"
	Attrs map[string]any // all slog attributes on the log record
}

// Buffer is a fixed-capacity ring buffer of Events. Safe for concurrent use.
type Buffer struct {
	mu       sync.Mutex
	events   []Event
	capacity int
	head     int // index of the oldest slot (next write target)
	count    int // how many slots are filled
}

// NewBuffer returns a Buffer with the given capacity.
func NewBuffer(capacity int) *Buffer {
	return &Buffer{
		events:   make([]Event, capacity),
		capacity: capacity,
	}
}

// Add inserts e into the buffer, overwriting the oldest entry when full.
func (b *Buffer) Add(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events[b.head] = e
	b.head = (b.head + 1) % b.capacity
	if b.count < b.capacity {
		b.count++
	}
}

// Recent returns up to n events newest-first.
func (b *Buffer) Recent(n int) []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n <= 0 || b.count == 0 {
		return nil
	}
	if n > b.count {
		n = b.count
	}
	out := make([]Event, n)
	// head points to the next write slot, so (head-1) is the newest.
	for i := 0; i < n; i++ {
		idx := (b.head - 1 - i + b.capacity) % b.capacity
		out[i] = b.events[idx]
	}
	return out
}
```

- [ ] **Step 4: Implement `internal/ring/handler.go`**

```go
package ring

import (
	"context"
	"log/slog"
	"time"
)

// handler wraps a slog.Handler and adds warn/error records to a Buffer.
type handler struct {
	next slog.Handler
	buf  *Buffer
}

// NewHandler returns a slog.Handler that forwards every record to next and
// additionally adds warn/error records to buf.
func NewHandler(next slog.Handler, buf *Buffer) slog.Handler {
	return &handler{next: next, buf: buf}
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelWarn {
		e := Event{
			Time:  r.Time,
			Level: r.Level.String(),
			Attrs: make(map[string]any),
		}
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "event" {
				e.Event = a.Value.String()
			} else {
				e.Attrs[a.Key] = a.Value.Any()
			}
			return true
		})
		if e.Event == "" {
			e.Event = r.Message
		}
		h.buf.Add(e)
	}
	return h.next.Handle(ctx, r)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{next: h.next.WithAttrs(attrs), buf: h.buf}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{next: h.next.WithGroup(name), buf: h.buf}
}
```

- [ ] **Step 5: Run tests — expect pass**

```bash
go test ./internal/ring/... -race -v 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/ring/
git commit -m "M12: add internal/ring — fixed-capacity ring buffer + slog handler"
```

---

## Task 3: `internal/ratelimit` — brute-force lockout

**Skills:** `superpowers:test-driven-development`, `golang-concurrency`, `golang-security`, `golang-testing`

**Files:**
- Create: `internal/ratelimit/limiter.go`
- Create: `internal/ratelimit/limiter_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/ratelimit/limiter_test.go`:

```go
package ratelimit_test

import (
	"sync"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLimiter() *ratelimit.Limiter {
	return ratelimit.NewLimiter(ratelimit.Opts{
		SourceRate:      100,          // effectively unlimited for unit tests
		SourceBurst:     100,
		FailThreshold:   3,
		LockoutBase:     50 * time.Millisecond,
		LockoutMax:      200 * time.Millisecond,
		CleanupInterval: 10 * time.Millisecond,
	})
}

func TestLimiter_AllowsInitially(t *testing.T) {
	l := newTestLimiter()
	defer l.Stop()
	allowed, retryAfter := l.Allow("1.2.3.4", "alice")
	assert.True(t, allowed)
	assert.Zero(t, retryAfter)
}

func TestLimiter_LockoutAfterThreshold(t *testing.T) {
	l := newTestLimiter()
	defer l.Stop()
	for i := 0; i < 3; i++ {
		l.RecordFailure("1.2.3.4", "alice")
	}
	allowed, retryAfter := l.Allow("1.2.3.4", "alice")
	assert.False(t, allowed)
	assert.Positive(t, retryAfter)
}

func TestLimiter_LockoutEscalates(t *testing.T) {
	l := ratelimit.NewLimiter(ratelimit.Opts{
		SourceRate:      100,
		SourceBurst:     100,
		FailThreshold:   2,
		LockoutBase:     50 * time.Millisecond,
		LockoutMax:      500 * time.Millisecond,
		CleanupInterval: time.Hour, // don't GC during test
	})
	defer l.Stop()

	// First lockout: 50ms
	l.RecordFailure("x", "bob")
	l.RecordFailure("x", "bob")
	_, r1 := l.Allow("x", "bob")
	assert.InDelta(t, 50*time.Millisecond, r1, float64(10*time.Millisecond))

	// Wait for first lockout to expire
	time.Sleep(60 * time.Millisecond)

	// Second lockout: 100ms
	l.RecordFailure("x", "bob")
	l.RecordFailure("x", "bob")
	_, r2 := l.Allow("x", "bob")
	assert.Greater(t, r2, r1, "second lockout should be longer than first")
}

func TestLimiter_RecordSuccessResets(t *testing.T) {
	l := newTestLimiter()
	defer l.Stop()
	l.RecordFailure("x", "carol")
	l.RecordFailure("x", "carol")
	l.RecordSuccess("carol")

	// Should need full threshold of failures again
	l.RecordFailure("x", "carol")
	l.RecordFailure("x", "carol")
	allowed, _ := l.Allow("x", "carol")
	assert.True(t, allowed, "reset should require full threshold again")
}

func TestLimiter_SourceRateLimit(t *testing.T) {
	l := ratelimit.NewLimiter(ratelimit.Opts{
		SourceRate:      1,            // 1 per second
		SourceBurst:     2,
		FailThreshold:   100,          // never triggers lockout
		LockoutBase:     time.Hour,
		LockoutMax:      time.Hour,
		CleanupInterval: time.Hour,
	})
	defer l.Stop()

	// Exhaust burst
	ok1, _ := l.Allow("5.5.5.5", "dave")
	ok2, _ := l.Allow("5.5.5.5", "dave")
	assert.True(t, ok1)
	assert.True(t, ok2)

	// Third attempt should be rate-limited
	ok3, retry := l.Allow("5.5.5.5", "dave")
	assert.False(t, ok3)
	assert.Positive(t, retry)
}

func TestLimiter_CleanupPrunesStale(t *testing.T) {
	l := ratelimit.NewLimiter(ratelimit.Opts{
		SourceRate:      100,
		SourceBurst:     100,
		FailThreshold:   2,
		LockoutBase:     20 * time.Millisecond,
		LockoutMax:      20 * time.Millisecond,
		CleanupInterval: 10 * time.Millisecond,
	})
	defer l.Stop()

	l.RecordFailure("old", "old")
	l.RecordFailure("old", "old")
	// Wait for lockout to expire + cleanup to run
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again (state pruned)
	allowed, _ := l.Allow("old", "old")
	assert.True(t, allowed)
}

func TestLimiter_ConcurrentSafe(t *testing.T) {
	l := newTestLimiter()
	defer l.Stop()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			l.RecordFailure("src", "user")
			l.Allow("src", "user")
			l.RecordSuccess("user")
		}(i)
	}
	wg.Wait()
}
```

- [ ] **Step 2: Run — expect compile failure**

```bash
go test ./internal/ratelimit/... 2>&1 | head -5
```

- [ ] **Step 3: Implement `internal/ratelimit/limiter.go`**

```go
// Package ratelimit provides per-source token-bucket rate limiting and
// per-username escalating lockout for the login endpoint.
// All state is in-memory and resets on process restart.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Opts configures the Limiter.
type Opts struct {
	SourceRate      rate.Limit    // requests/second; use rate.Every(time.Minute/N) for N/min
	SourceBurst     int
	FailThreshold   int           // consecutive failures before first lockout
	LockoutBase     time.Duration // first lockout duration
	LockoutMax      time.Duration // cap on escalating lockout
	CleanupInterval time.Duration // how often stale entries are pruned
}

type sourceState struct {
	limiter   *rate.Limiter
	lastSeen  time.Time
}

type usernameState struct {
	failures     int
	lockoutLevel int
	lockedUntil  time.Time
	lastSeen     time.Time
}

// Limiter implements per-source rate limiting and per-username escalating lockout.
type Limiter struct {
	opts     Opts
	mu       sync.Mutex
	sources  map[string]*sourceState
	usernames map[string]*usernameState
	stopCh   chan struct{}
}

// NewLimiter returns a started Limiter. Call Stop() when done.
func NewLimiter(opts Opts) *Limiter {
	l := &Limiter{
		opts:      opts,
		sources:   make(map[string]*sourceState),
		usernames: make(map[string]*usernameState),
		stopCh:    make(chan struct{}),
	}
	go l.cleanup()
	return l
}

func (l *Limiter) getSource(source string) *sourceState {
	s, ok := l.sources[source]
	if !ok {
		s = &sourceState{limiter: rate.NewLimiter(l.opts.SourceRate, l.opts.SourceBurst)}
		l.sources[source] = s
	}
	s.lastSeen = time.Now()
	return s
}

func (l *Limiter) getUsername(username string) *usernameState {
	u, ok := l.usernames[username]
	if !ok {
		u = &usernameState{}
		l.usernames[username] = u
	}
	u.lastSeen = time.Now()
	return u
}

// Allow returns (true, 0) if the request may proceed.
// Returns (false, retryAfter) if the source is rate-limited or the username is locked out.
func (l *Limiter) Allow(source, username string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	src := l.getSource(source)
	if !src.limiter.Allow() {
		// Token bucket exhausted — calculate approximate retry delay.
		r := src.limiter.Reserve()
		delay := r.Delay()
		r.Cancel()
		return false, delay
	}

	usr := l.getUsername(username)
	if !usr.lockedUntil.IsZero() && time.Now().Before(usr.lockedUntil) {
		return false, time.Until(usr.lockedUntil)
	}

	return true, 0
}

// RecordSuccess resets the failure counter and lockout escalation for username.
func (l *Limiter) RecordSuccess(username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if u, ok := l.usernames[username]; ok {
		u.failures = 0
		u.lockoutLevel = 0
		u.lockedUntil = time.Time{}
		u.lastSeen = time.Now()
	}
}

// RecordFailure increments the failure counter for source and username.
// If the per-username counter reaches FailThreshold, a lockout is applied.
func (l *Limiter) RecordFailure(source, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	_ = l.getSource(source) // touch lastSeen

	usr := l.getUsername(username)
	usr.failures++
	if usr.failures >= l.opts.FailThreshold {
		dur := l.opts.LockoutBase
		for i := 0; i < usr.lockoutLevel; i++ {
			dur *= 2
			if dur > l.opts.LockoutMax {
				dur = l.opts.LockoutMax
				break
			}
		}
		usr.lockedUntil = time.Now().Add(dur)
		usr.failures = 0
		usr.lockoutLevel++
	}
}

// Stop halts the cleanup goroutine.
func (l *Limiter) Stop() {
	close(l.stopCh)
}

func (l *Limiter) cleanup() {
	ticker := time.NewTicker(l.opts.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.prune()
		case <-l.stopCh:
			return
		}
	}
}

func (l *Limiter) prune() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-2 * l.opts.LockoutMax)
	for k, s := range l.sources {
		if s.lastSeen.Before(cutoff) {
			delete(l.sources, k)
		}
	}
	for k, u := range l.usernames {
		if u.lastSeen.Before(cutoff) && (u.lockedUntil.IsZero() || time.Now().After(u.lockedUntil)) {
			delete(l.usernames, k)
		}
	}
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./internal/ratelimit/... -race -v 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ratelimit/
git commit -m "M12: add internal/ratelimit — per-source rate limit + per-username escalating lockout"
```

---

## Task 4: `internal/auth` — `NeedsRehash`

**Skills:** `superpowers:test-driven-development`, `golang-security`

**Files:**
- Modify: `internal/auth/argon2.go`
- Modify: `internal/auth/argon2_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/auth/argon2_test.go`:

```go
func TestNeedsRehash_FalseForCurrentParams(t *testing.T) {
	hash, err := auth.Hash("password123", auth.DefaultParams)
	require.NoError(t, err)
	needs, err := auth.NeedsRehash(hash, auth.DefaultParams)
	require.NoError(t, err)
	assert.False(t, needs)
}

func TestNeedsRehash_TrueForWeakerMemory(t *testing.T) {
	weaker := auth.Params{Time: 2, Memory: 32 * 1024, Threads: 1, SaltLen: 16, KeyLen: 32}
	hash, err := auth.Hash("password123", weaker)
	require.NoError(t, err)
	needs, err := auth.NeedsRehash(hash, auth.DefaultParams)
	require.NoError(t, err)
	assert.True(t, needs)
}

func TestNeedsRehash_TrueForWeakerTime(t *testing.T) {
	weaker := auth.Params{Time: 1, Memory: 64 * 1024, Threads: 1, SaltLen: 16, KeyLen: 32}
	hash, err := auth.Hash("password123", weaker)
	require.NoError(t, err)
	needs, err := auth.NeedsRehash(hash, auth.DefaultParams)
	require.NoError(t, err)
	assert.True(t, needs)
}

func TestNeedsRehash_ErrorOnMalformed(t *testing.T) {
	_, err := auth.NeedsRehash("not-a-phc-string", auth.DefaultParams)
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run — expect failure**

```bash
go test ./internal/auth/... -run TestNeedsRehash 2>&1 | head -10
```

Expected: `undefined: auth.NeedsRehash`.

- [ ] **Step 3: Implement `NeedsRehash` in `internal/auth/argon2.go`**

Append to the file (after the existing `Verify` function):

```go
// NeedsRehash returns true if the encoded hash was produced with params that
// are strictly weaker than current on any axis (Time, Memory, or Threads).
// The PHC encoding carries the params used at hash time, so this comparison
// is purely a string parse — no crypto work is performed.
func NeedsRehash(encoded string, current Params) (bool, error) {
	p, _, _, err := decodeHash(encoded)
	if err != nil {
		return false, fmt.Errorf("needs rehash: decode: %w", err)
	}
	return p.Time < current.Time || p.Memory < current.Memory || p.Threads < current.Threads, nil
}
```

Note: `decodeHash` is already used internally by `Verify` — it parses the PHC string. If it is unexported, confirm its signature in `internal/auth/argon2.go` and use it directly. If not available, implement a minimal PHC parser for just the `m`, `t`, `p` parameters.

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./internal/auth/... -race -v -run TestNeedsRehash 2>&1
```

Expected: all PASS.

- [ ] **Step 5: Run full auth test suite**

```bash
go test ./internal/auth/... -race 2>&1 | tail -5
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/argon2.go internal/auth/argon2_test.go
git commit -m "M12: add auth.NeedsRehash for argon2 re-hash-on-verify"
```

---

## Task 5: `internal/metrics` — provider and instruments

**Skills:** `superpowers:test-driven-development`, `golang-observability`, `golang-testing`

**Files:**
- Create: `internal/metrics/provider.go`
- Create: `internal/metrics/instruments.go`
- Create: `internal/metrics/handler.go`
- Create: `internal/metrics/provider_test.go`
- Create: `internal/metrics/instruments_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/metrics/provider_test.go`:

```go
package metrics_test

import (
	"context"
	"testing"

	"github.com/bcrisp4/tap/internal/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_NoopBeforeInit(t *testing.T) {
	// All instrument calls before Init must not panic.
	assert.NotPanics(t, func() {
		metrics.PollsTotal.Add(context.Background(), 1)
	})
}

func TestInit_Shutdown(t *testing.T) {
	err := metrics.Init(metrics.Opts{})
	require.NoError(t, err)
	err = metrics.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestInit_DoubleInit(t *testing.T) {
	_ = metrics.Init(metrics.Opts{})
	defer metrics.Shutdown(context.Background())
	err := metrics.Init(metrics.Opts{})
	assert.Error(t, err, "second Init should return error")
}
```

Create `internal/metrics/instruments_test.go`:

```go
package metrics_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bcrisp4/tap/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstruments_HelpStringsPresent(t *testing.T) {
	// Init with a fresh registry so the test is isolated.
	reg := prometheus.NewRegistry()
	err := metrics.InitWithRegistry(reg, metrics.Opts{})
	require.NoError(t, err)
	defer metrics.Shutdown(context.Background())

	// Record a value so the metric appears in output.
	metrics.PollsTotal.Add(context.Background(), 1)

	gathered, err := reg.Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range gathered {
		if mf.GetName() == "tap_polls_total" {
			assert.True(t, strings.Contains(mf.GetHelp(), "poll"), "help string should mention poll")
			found = true
		}
	}
	assert.True(t, found, "tap_polls_total should appear in gathered metrics")
}
```

- [ ] **Step 2: Run — expect compile failure**

```bash
go test ./internal/metrics/... 2>&1 | head -5
```

- [ ] **Step 3: Implement `internal/metrics/provider.go`**

```go
// Package metrics owns the OTel MeterProvider and Prometheus bridge for Tap.
// Call Init once at startup; the global provider is a no-op until then.
// All instrument variables in instruments.go are safe to call before Init.
package metrics

import (
	"context"
	"errors"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/bridge/prometheus" as prombridge
	"go.opentelemetry.io/otel/sdk/metric"
)

// Opts configures the metrics provider. OTLPEndpoint empty = Prometheus-only.
type Opts struct {
	OTLPEndpoint string
	OTLPHeaders  map[string]string
}

var (
	mu       sync.Mutex
	shutdown func(context.Context) error
	reg      = prometheus.NewRegistry()
)

// Init initialises the OTel MeterProvider. Returns an error if called twice.
func Init(opts Opts) error {
	return InitWithRegistry(reg, opts)
}

// InitWithRegistry allows tests to supply an isolated registry.
func InitWithRegistry(r *prometheus.Registry, opts Opts) error {
	mu.Lock()
	defer mu.Unlock()
	if shutdown != nil {
		return errors.New("metrics: already initialised")
	}

	bridge := prombridge.New(r)
	readers := []metric.Option{metric.WithReader(bridge)}

	// OTLP reader added when endpoint is configured.
	if opts.OTLPEndpoint != "" {
		// OTLP periodic reader (HTTP or gRPC depending on scheme — wired in
		// cmd/tap/main.go after scheme detection; see spec wire-up section).
	}

	mp := metric.NewMeterProvider(readers...)
	otel.SetMeterProvider(mp)

	shutdown = func(ctx context.Context) error {
		return mp.Shutdown(ctx)
	}
	registerInstruments()
	return nil
}

// Shutdown flushes and closes the provider.
func Shutdown(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()
	if shutdown == nil {
		return nil
	}
	err := shutdown(ctx)
	shutdown = nil
	reg = prometheus.NewRegistry() // reset for potential re-init in tests
	return err
}
```

**Note on OTLP scheme detection:** The full OTLP exporter wiring (HTTP vs gRPC) lives in `cmd/tap/main.go` per the spec wire-up section. `metrics.Init` accepts `Opts`; the caller constructs and passes the appropriate exporter reader. Revise `InitWithRegistry` to accept `...metric.Option` extra readers once the cmd integration task (Task 11) is reached.

- [ ] **Step 4: Implement `internal/metrics/instruments.go`**

```go
package metrics

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Instrument variables — safe to call before Init (no-op provider).
// Each variable's Help string appears verbatim in Prometheus exposition output.
var (
	// --- Polling ---

	// PollsTotal counts feed poll attempts by result.
	// Labels: result={success,failure,skipped}
	PollsTotal metric.Int64Counter

	// PollDuration measures wall-clock poll cycle duration in seconds.
	// Labels: result={success,failure}
	PollDuration metric.Float64Histogram

	// EntriesInserted counts new entries committed to the database.
	EntriesInserted metric.Int64Counter

	// ConditionalGetHits counts polls receiving a 304 Not Modified response.
	ConditionalGetHits metric.Int64Counter

	// --- HTTP server ---

	// HTTPRequestsTotal counts inbound HTTP requests.
	// Labels: method, route, status_class={2xx,4xx,5xx}
	HTTPRequestsTotal metric.Int64Counter

	// HTTPRequestDuration measures inbound request latency in seconds.
	// Labels: method, route
	HTTPRequestDuration metric.Float64Histogram

	// --- Media proxy ---

	// ProxyCacheHits counts proxy requests served from filesystem cache.
	ProxyCacheHits metric.Int64Counter

	// ProxyCacheMisses counts proxy requests requiring an origin fetch.
	ProxyCacheMisses metric.Int64Counter

	// ProxyCacheEvictions counts evicted cache files.
	// Labels: reason={size_cap,age_sweep}
	ProxyCacheEvictions metric.Int64Counter

	// ProxyCacheBytes tracks total media cache size in bytes.
	ProxyCacheBytes metric.Int64Gauge

	// --- Auth ---

	// LoginAttempts counts login attempts.
	// Labels: result={success,failure,rate_limited,locked_out}
	LoginAttempts metric.Int64Counter

	// Lockouts counts lockout events triggered.
	// Labels: axis={source,username}
	Lockouts metric.Int64Counter

	// ActiveSessions tracks non-expired session rows (approximate).
	ActiveSessions metric.Int64Gauge

	// --- Database ---

	// DBTxDuration measures database transaction duration in seconds.
	// Labels: op
	DBTxDuration metric.Float64Histogram
)

func registerInstruments() {
	m := otel.GetMeterProvider().Meter("tap")

	PollsTotal, _ = m.Int64Counter("tap_polls_total",
		metric.WithDescription("Total feed poll attempts labelled by result (success, failure, skipped). "+
			"Use rate(tap_polls_total[5m]) to observe poll throughput."),
		metric.WithUnit("{polls}"))

	PollDuration, _ = m.Float64Histogram("tap_poll_duration_seconds",
		metric.WithDescription("Wall-clock duration of a complete poll cycle from dispatch to commit or failure, in seconds. "+
			"Buckets: 0.1s to 60s. Use histogram_quantile(0.99, ...) for tail latency."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 5, 10, 30, 60))

	EntriesInserted, _ = m.Int64Counter("tap_entries_inserted_total",
		metric.WithDescription("Total new feed entries committed to the database across all polls. "+
			"Deduplicated entries (already seen) are not counted."),
		metric.WithUnit("{entries}"))

	ConditionalGetHits, _ = m.Int64Counter("tap_conditional_get_hits_total",
		metric.WithDescription("Number of polls that received HTTP 304 Not Modified, indicating the feed has not changed. "+
			"High ratio = low feed update frequency; good for bandwidth efficiency."),
		metric.WithUnit("{polls}"))

	HTTPRequestsTotal, _ = m.Int64Counter("tap_http_requests_total",
		metric.WithDescription("Total inbound HTTP requests labelled by method, matched route pattern (not raw path), "+
			"and response status class (2xx, 4xx, 5xx). Use for traffic and error rate dashboards."),
		metric.WithUnit("{requests}"))

	HTTPRequestDuration, _ = m.Float64Histogram("tap_http_request_duration_seconds",
		metric.WithDescription("Inbound HTTP request latency from first byte received to last byte written, in seconds. "+
			"Buckets: 5ms to 2.5s. Use histogram_quantile(0.99, ...) to surface slow routes."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5))

	ProxyCacheHits, _ = m.Int64Counter("tap_proxy_cache_hits_total",
		metric.WithDescription("Media proxy requests served from the local filesystem cache without fetching the origin. "+
			"High ratio = efficient caching. Low ratio = cold cache or high eviction rate."),
		metric.WithUnit("{requests}"))

	ProxyCacheMisses, _ = m.Int64Counter("tap_proxy_cache_misses_total",
		metric.WithDescription("Media proxy requests that required fetching the origin (cache cold or expired). "+
			"Each miss generates one outbound HTTP request to the image origin."),
		metric.WithUnit("{requests}"))

	ProxyCacheEvictions, _ = m.Int64Counter("tap_proxy_cache_evictions_total",
		metric.WithDescription("Media cache files evicted. reason=size_cap: inline LRU eviction triggered by the "+
			"configured byte cap (--proxy-cache-cap-bytes). reason=age_sweep: daily archival sweep (M11 OnEvict callback)."),
		metric.WithUnit("{files}"))

	ProxyCacheBytes, _ = m.Int64Gauge("tap_proxy_cache_bytes",
		metric.WithDescription("Current total size of the media proxy filesystem cache in bytes. "+
			"Updated after each eviction pass and each new cache write. Compare against --proxy-cache-cap-bytes."),
		metric.WithUnit("By"))

	LoginAttempts, _ = m.Int64Counter("tap_login_attempts_total",
		metric.WithDescription("Login attempts against POST /api/v1/sessions labelled by outcome. "+
			"result=rate_limited: per-source token bucket exhausted. result=locked_out: per-username lockout active."),
		metric.WithUnit("{attempts}"))

	Lockouts, _ = m.Int64Counter("tap_lockouts_total",
		metric.WithDescription("Lockout events triggered. axis=source: per-IP rate limit exceeded. "+
			"axis=username: per-username consecutive failure threshold reached (--lockout-threshold)."),
		metric.WithUnit("{lockouts}"))

	ActiveSessions, _ = m.Int64Gauge("tap_active_sessions",
		metric.WithDescription("Non-expired session rows in the database (approximate). "+
			"Updated on session create, delete, and expiry. Not a live connection count."),
		metric.WithUnit("{sessions}"))

	DBTxDuration, _ = m.Float64Histogram("tap_db_tx_duration_seconds",
		metric.WithDescription("Database transaction duration in seconds labelled by op (e.g. insert_entries, list_due_polls, get_session). "+
			"Buckets: 1ms to 1s. Use to identify slow query groups."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1))
}
```

- [ ] **Step 5: Implement `internal/metrics/handler.go`**

```go
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler returns an HTTP handler that serves the Prometheus text exposition
// of all registered instruments. Only mount when --metrics-enabled.
func Handler() http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{EnableOpenMetrics: false})
}
```

- [ ] **Step 6: Run tests — fix until passing**

```bash
go test ./internal/metrics/... -race -v 2>&1 | tail -30
```

The OTel bridge import path may need adjustment — check `go.opentelemetry.io/otel/bridge/prometheus` export names against the fetched version. Adjust import aliases as needed.

- [ ] **Step 7: Commit**

```bash
git add internal/metrics/
git commit -m "M12: add internal/metrics — OTel MeterProvider + Prometheus bridge + instrument definitions"
```

---

## Task 6: `internal/tracing` — TracerProvider + middleware + RoundTripper

**Skills:** `superpowers:test-driven-development`, `golang-observability`, `golang-context`, `golang-testing`

**Files:**
- Create: `internal/tracing/provider.go`
- Create: `internal/tracing/middleware.go`
- Create: `internal/tracing/roundtripper.go`
- Create: `internal/tracing/provider_test.go`
- Create: `internal/tracing/middleware_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/tracing/provider_test.go`:

```go
package tracing_test

import (
	"context"
	"testing"

	"github.com/bcrisp4/tap/internal/tracing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_NoEndpoint_NoopProvider(t *testing.T) {
	err := tracing.Init(tracing.Opts{})
	require.NoError(t, err)
	defer tracing.Shutdown(context.Background())
	// No-op provider — spans are created but not exported.
	// Just verifying no panic.
}

func TestInit_Shutdown_Clean(t *testing.T) {
	_ = tracing.Init(tracing.Opts{})
	err := tracing.Shutdown(context.Background())
	assert.NoError(t, err)
}
```

Create `internal/tracing/middleware_test.go`:

```go
package tracing_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bcrisp4/tap/internal/tracing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMiddleware_SetsRequestIDHeader(t *testing.T) {
	_ = tracing.Init(tracing.Opts{})
	defer tracing.Shutdown(nil)

	handler := tracing.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/entries", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	id := rr.Header().Get("X-Request-ID")
	require.NotEmpty(t, id)
	assert.Len(t, id, 32, "X-Request-ID should be 16 hex bytes = 32 chars")
}
```

- [ ] **Step 2: Run — expect compile failure**

```bash
go test ./internal/tracing/... 2>&1 | head -5
```

- [ ] **Step 3: Implement `internal/tracing/provider.go`**

```go
// Package tracing owns the OTel TracerProvider for Tap.
// Call Init once at startup; the global provider is a no-op until then.
package tracing

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Opts configures the tracing provider.
type Opts struct {
	OTLPEndpoint string
	OTLPHeaders  map[string]string
	SampleRate   float64 // 0.0-1.0; default 0.1
	ServiceName  string
	Version      string
}

var (
	mu           sync.Mutex
	shutdownFunc func(context.Context) error
)

// Init initialises the OTel TracerProvider. When OTLPEndpoint is empty, a
// no-op provider is registered (traces disabled). Sampling: errors always
// sampled; normal requests at SampleRate.
func Init(opts Opts) error {
	mu.Lock()
	defer mu.Unlock()

	rate := opts.SampleRate
	if rate <= 0 {
		rate = 0.1
	}

	sampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(rate),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
		// OTLP exporter is wired in cmd/tap/main.go when endpoint is set.
		// Here we build the SDK provider regardless; without a batcher/exporter
		// spans are created in-process but dropped on export — effectively no-op.
	)
	otel.SetTracerProvider(tp)
	shutdownFunc = tp.Shutdown
	return nil
}

// Shutdown flushes and closes the provider.
func Shutdown(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()
	if shutdownFunc == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	err := shutdownFunc(ctx)
	shutdownFunc = nil
	return err
}
```

- [ ] **Step 4: Implement `internal/tracing/middleware.go`**

```go
package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Middleware wraps h with an OTel span for inbound requests and injects a
// request_id into the slog context and response headers.
func Middleware(h http.Handler) http.Handler {
	tracer := otel.Tracer("tap/http")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := generateRequestID()

		ctx, span := tracer.Start(r.Context(), r.Method+" "+r.Pattern)
		defer span.End()

		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.route", r.Pattern),
			attribute.String("request_id", requestID),
		)

		// Inject request_id into slog so every log line during this request
		// carries it automatically.
		ctx = slog.With("request_id", requestID)
		// Note: slog does not support context-injection natively; we store the
		// request_id in context for handlers that call slog.InfoContext(ctx,...).
		// The tracing span carries it as an attribute for OTel correlation.
		ctx = context.WithValue(ctx, requestIDKey{}, requestID)

		rr := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		w.Header().Set("X-Request-ID", requestID)
		h.ServeHTTP(rr, r.WithContext(ctx))

		span.SetAttributes(attribute.Int("http.status_code", rr.status))
	})
}

type requestIDKey struct{}

// RequestIDFromContext extracts the request_id from ctx.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey{}).(string); ok {
		return v
	}
	return ""
}

func generateRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 5: Implement `internal/tracing/roundtripper.go`**

```go
package tracing

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type tracingRoundTripper struct {
	wrapped http.RoundTripper
}

// NewRoundTripper wraps wrapped with OTel span creation for each outbound
// HTTP request. Only host is recorded (never the full URL) to avoid leaking
// credentials or tracking params into traces.
func NewRoundTripper(wrapped http.RoundTripper) http.RoundTripper {
	return &tracingRoundTripper{wrapped: wrapped}
}

func (t *tracingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	tracer := otel.Tracer("tap/http/client")
	ctx, span := tracer.Start(req.Context(), "http.client "+req.Method)
	defer span.End()

	span.SetAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("net.peer.name", req.URL.Host),
	)

	resp, err := t.wrapped.RoundTrip(req.WithContext(ctx))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
	return resp, nil
}
```

- [ ] **Step 6: Run tests — expect pass**

```bash
go test ./internal/tracing/... -race -v 2>&1 | tail -20
```

- [ ] **Step 7: Commit**

```bash
git add internal/tracing/
git commit -m "M12: add internal/tracing — TracerProvider, HTTP middleware, outbound RoundTripper"
```

---

## Task 7: `internal/api/errors.go` + new error codes

**Skills:** `golang-error-handling`

**Files:**
- Modify: `internal/api/errors.go`

- [ ] **Step 1: Add new error codes**

Read `internal/api/errors.go` first. Append:

```go
const (
    ErrCodeForbidden   = "forbidden"
    ErrCodeRateLimited = "rate_limited"
)
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/api/... 2>&1
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/errors.go
git commit -m "M12: add ErrCodeForbidden and ErrCodeRateLimited error codes"
```

---

## Task 8: `internal/api/auth.go` — wire limiter + re-hash + log taxonomy

**Skills:** `superpowers:test-driven-development`, `golang-security`, `golang-error-handling`

**Files:**
- Modify: `internal/api/auth.go`
- Modify: `internal/api/auth_test.go`
- Modify: `internal/api/api.go` (add `Limiter`, `TrustedProxy` to `MuxOpts`)

- [ ] **Step 1: Write failing tests for rate-limited login**

Append to `internal/api/auth_test.go`:

```go
func TestLogin_RateLimited_Returns429(t *testing.T) {
	// Build a limiter that immediately denies any request.
	lim := ratelimit.NewLimiter(ratelimit.Opts{
		SourceRate:      0.0001, // effectively zero
		SourceBurst:     0,
		FailThreshold:   1000,
		LockoutBase:     time.Hour,
		LockoutMax:      time.Hour,
		CleanupInterval: time.Hour,
	})
	defer lim.Stop()

	// Use the existing test helper to build a mux with the limiter.
	ts := newTestServer(t, withLimiter(lim))
	defer ts.Close()

	body := `{"username":"ben","password":"password123"}`
	resp, _ := ts.Client().Post(ts.URL+"/api/v1/sessions", "application/json",
		strings.NewReader(body))
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("Retry-After"))
}

func TestLogin_RehashOnWeakParams(t *testing.T) {
	// Hash with weaker params, then login — verify the stored hash updates.
	weaker := auth.Params{Time: 1, Memory: 32 * 1024, Threads: 1, SaltLen: 16, KeyLen: 32}
	// ... set up test DB with user hashed under weaker params ...
	// ... login succeeds ...
	// ... re-read the hash from DB and assert NeedsRehash returns false ...
}
```

Note: the `withLimiter` helper and `newTestServer` signatures follow the existing pattern in `internal/api/testing.go`. Extend that file if the helper doesn't exist.

- [ ] **Step 2: Add `Limiter` and `TrustedProxy` to `MuxOpts` in `internal/api/api.go`**

```go
// In MuxOpts struct:
Limiter      *ratelimit.Limiter // nil = no rate limiting (dev/test)
TrustedProxy bool               // trust X-Forwarded-For for source IP
MetricsEnabled bool
RingBuffer   *ring.Buffer
StartTime    time.Time
Version      string
```

Import the new packages at the top of `api.go`.

- [ ] **Step 3: Wire limiter into login handler in `internal/api/auth.go`**

In `loginHandler`, before credential verification:

```go
if opts.Limiter != nil {
    source := sourceIP(r, opts.TrustedProxy)
    allowed, retryAfter := opts.Limiter.Allow(source, body.Username)
    if !allowed {
        w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
        metrics.LoginAttempts.Add(r.Context(), 1,
            metric.WithAttributes(attribute.String("result", "rate_limited")))
        writeError(w, http.StatusTooManyRequests, ErrCodeRateLimited, "too many requests")
        return
    }
}
```

After credential check fails:

```go
if opts.Limiter != nil {
    opts.Limiter.RecordFailure(source, body.Username)
}
metrics.LoginAttempts.Add(r.Context(), 1, metric.WithAttributes(attribute.String("result", "failure")))
slog.WarnContext(r.Context(), "auth.login.failure",
    "event", "auth.login.failure",
    "username", body.Username,
    "source", sourceIP(r, opts.TrustedProxy),
    "reason", "bad_credentials")
writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "invalid credentials")
```

After successful credential check, add re-hash-on-verify:

```go
if needs, err := auth.NeedsRehash(user.PasswordHash, opts.HashParams); err == nil && needs {
    if newHash, err := auth.Hash(body.Password, opts.HashParams); err == nil {
        _ = db.UpdatePasswordHash(r.Context(), db, user.ID, newHash)
        slog.InfoContext(r.Context(), "argon2 params upgraded on verify",
            "event", "auth.rehash", "user_id", user.ID)
    }
}
if opts.Limiter != nil {
    opts.Limiter.RecordSuccess(body.Username)
}
metrics.LoginAttempts.Add(r.Context(), 1, metric.WithAttributes(attribute.String("result", "success")))
slog.InfoContext(r.Context(), "login successful",
    "event", "auth.login.success",
    "user_id", user.ID, "username", user.Username,
    "source", sourceIP(r, opts.TrustedProxy))
```

Add `sourceIP` helper at the bottom of `auth.go`:

```go
func sourceIP(r *http.Request, trustedProxy bool) string {
    if trustedProxy {
        if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
            // Take the first value (leftmost = original client).
            if idx := strings.Index(fwd, ","); idx >= 0 {
                return strings.TrimSpace(fwd[:idx])
            }
            return strings.TrimSpace(fwd)
        }
    }
    host, _, _ := net.SplitHostPort(r.RemoteAddr)
    return host
}
```

- [ ] **Step 4: Run auth tests**

```bash
go test ./internal/api/... -race -run TestLogin 2>&1 | tail -20
```

Fix failures until PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/
git commit -m "M12: wire rate limiter and re-hash-on-verify into login handler; add structured auth log events"
```

---

## Task 9: `GET /api/v1/status` — system status endpoint

**Skills:** `superpowers:test-driven-development`, `golang-testing`

**Files:**
- Create: `internal/api/status.go`
- Create: `internal/api/status_test.go`
- Modify: `internal/api/api.go` (mount the new handler)

- [ ] **Step 1: Write the failing tests**

Create `internal/api/status_test.go`:

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/api"
	"github.com/bcrisp4/tap/internal/ring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatus_AdminGets200(t *testing.T) {
	buf := ring.NewBuffer(10)
	ts := newTestServer(t, withAdminSession(), withRingBuffer(buf), withStartTime(time.Now()))
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/v1/status")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, body, "version")
	assert.Contains(t, body, "uptime_seconds")
	assert.Contains(t, body, "db")
	assert.Contains(t, body, "polls_active")
	assert.Contains(t, body, "recent_errors")
}

func TestStatus_UserGets403(t *testing.T) {
	ts := newTestServer(t, withUserSession())
	defer ts.Close()

	resp, _ := ts.Client().Get(ts.URL + "/api/v1/status")
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestStatus_NoSession401(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp, _ := ts.Client().Get(ts.URL + "/api/v1/status")
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestStatus_RecentErrors(t *testing.T) {
	buf := ring.NewBuffer(10)
	buf.Add(ring.Event{Time: time.Now(), Level: "warn", Event: "poll.failure",
		Attrs: map[string]any{"feed_id": int64(1)}})

	ts := newTestServer(t, withAdminSession(), withRingBuffer(buf))
	defer ts.Close()

	resp, _ := ts.Client().Get(ts.URL + "/api/v1/status")
	var body struct {
		RecentErrors []map[string]any `json:"recent_errors"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body.RecentErrors, 1)
	assert.Equal(t, "poll.failure", body.RecentErrors[0]["event"])
}
```

- [ ] **Step 2: Run — expect compile failure**

```bash
go test ./internal/api/... -run TestStatus 2>&1 | head -10
```

- [ ] **Step 3: Implement `internal/api/status.go`**

```go
package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/bcrisp4/tap/internal/ring"
)

type statusResponse struct {
	Version       string           `json:"version"`
	UptimeSeconds int64            `json:"uptime_seconds"`
	DB            string           `json:"db"`
	PollsActive   int64            `json:"polls_active"`
	PollsTotal    int64            `json:"polls_total"`
	LastPollAt    *int64           `json:"last_poll_at"`
	RecentErrors  []statusEvent    `json:"recent_errors"`
}

type statusEvent struct {
	Time  string         `json:"time"`
	Level string         `json:"level"`
	Event string         `json:"event"`
	Attrs map[string]any `json:"attrs"`
}

type statusDeps struct {
	db        *sql.DB
	buf       *ring.Buffer
	startTime time.Time
	version   string
	// pollsActive and pollsTotal are in-memory counters from the Scheduler.
	// Provided as functions so the handler reads the current value at call time.
	pollsActive func() int64
	pollsTotal  func() int64
	lastPollAt  func() *int64
}

func statusHandler(deps statusDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFromContext(r.Context())
		if !ok || user.Role != "admin" {
			writeError(w, http.StatusForbidden, ErrCodeForbidden, "forbidden")
			return
		}

		dbStatus := "ok"
		if err := deps.db.PingContext(r.Context()); err != nil {
			dbStatus = "degraded"
		}

		var active, total int64
		var lastAt *int64
		if deps.pollsActive != nil {
			active = deps.pollsActive()
		}
		if deps.pollsTotal != nil {
			total = deps.pollsTotal()
		}
		if deps.lastPollAt != nil {
			lastAt = deps.lastPollAt()
		}

		events := deps.buf.Recent(20)
		recent := make([]statusEvent, len(events))
		for i, e := range events {
			recent[i] = statusEvent{
				Time:  e.Time.UTC().Format(time.RFC3339),
				Level: e.Level,
				Event: e.Event,
				Attrs: e.Attrs,
			}
		}

		resp := statusResponse{
			Version:       deps.version,
			UptimeSeconds: int64(time.Since(deps.startTime).Seconds()),
			DB:            dbStatus,
			PollsActive:   active,
			PollsTotal:    total,
			LastPollAt:    lastAt,
			RecentErrors:  recent,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}
```

- [ ] **Step 4: Mount `/api/v1/status` in `internal/api/api.go`**

In `NewMux`, after the existing authenticated routes:

```go
m.Handle("GET /api/v1/status", authed(statusHandler(statusDeps{
    db:        db,
    buf:       opts.RingBuffer,
    startTime: opts.StartTime,
    version:   opts.Version,
    // pollsActive/pollsTotal/lastPollAt: nil until scheduler exposes these counters
})))
```

- [ ] **Step 5: Run status tests**

```bash
go test ./internal/api/... -race -run TestStatus 2>&1 | tail -20
```

Fix failures until PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/status.go internal/api/status_test.go internal/api/api.go
git commit -m "M12: add GET /api/v1/status — admin-only system status endpoint"
```

---

## Task 10: Extend `GET /healthz` + mount `GET /metrics`

**Skills:** `superpowers:test-driven-development`

**Files:**
- Modify: `internal/api/api.go`
- Modify: `internal/api/api_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/api/api_test.go`:

```go
func TestHealthz_JSONBody(t *testing.T) {
	ts := newTestServer(t, withStartTime(time.Now()))
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/healthz")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.Contains(t, body, "version")
	assert.Contains(t, body, "uptime_seconds")
	assert.Contains(t, body, "db")
	assert.Contains(t, body, "polls_active")
}

func TestMetrics_EnabledReturns200(t *testing.T) {
	ts := newTestServer(t, withMetricsEnabled(true))
	defer ts.Close()

	resp, _ := ts.Client().Get(ts.URL + "/metrics")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/plain")
}

func TestMetrics_DisabledReturns404(t *testing.T) {
	ts := newTestServer(t, withMetricsEnabled(false))
	defer ts.Close()

	resp, _ := ts.Client().Get(ts.URL + "/metrics")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
```

- [ ] **Step 2: Update `/healthz` handler in `api.go`**

Replace the existing plaintext healthz handler with:

```go
m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
    dbStatus := "ok"
    if db != nil {
        if err := db.PingContext(r.Context()); err != nil {
            dbStatus = "degraded"
        }
    }
    var active int64
    if opts.PollsActive != nil {
        active = opts.PollsActive()
    }
    status := "ok"
    if dbStatus == "degraded" {
        status = "degraded"
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]any{
        "status":         status,
        "version":        opts.Version,
        "uptime_seconds": int64(time.Since(opts.StartTime).Seconds()),
        "db":             dbStatus,
        "polls_active":   active,
    })
})
```

Add `PollsActive func() int64` to `MuxOpts`.

- [ ] **Step 3: Mount `/metrics` conditionally**

In `NewMux`, after all route registrations:

```go
if opts.MetricsEnabled {
    m.Handle("GET /metrics", metrics.Handler())
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/api/... -race -run "TestHealthz|TestMetrics" 2>&1 | tail -20
```

- [ ] **Step 5: Commit**

```bash
git add internal/api/
git commit -m "M12: extend /healthz with JSON body; add /metrics endpoint (enabled by flag)"
```

---

## Task 11: `cmd/tap/main.go` — new flags + full wire-up

**Skills:** `golang-cli`, `golang-observability`, `golang-context`

**Files:**
- Modify: `cmd/tap/main.go`

- [ ] **Step 1: Add new flag declarations**

In `runServer()`, after the existing flag declarations:

```go
// M12: observability flags
logLevel       = flag.String("log-level", envOr("TAP_LOG_LEVEL", "info"), "log level: debug, info, warn, error")
metricsEnabled = flag.Bool("metrics-enabled", envOrBool("TAP_METRICS_ENABLED", false), "enable GET /metrics Prometheus scrape endpoint")
otlpEndpoint   = flag.String("otlp-endpoint", envOr("TAP_OTLP_ENDPOINT", ""), "OTel collector endpoint (empty = disabled); http://, https://, or grpc:// scheme")
otlpHeaders    = flag.String("otlp-headers", envOr("TAP_OTLP_HEADERS", ""), "comma-separated key=value OTLP auth headers")
traceSampleRate = flag.Float64("trace-sample-rate", envOrFloat64("TAP_TRACE_SAMPLE_RATE", 0.1), "fraction of normal traces to sample (0.0-1.0)")

// M12: rate limiting flags
loginRate        = flag.String("login-rate", envOr("TAP_LOGIN_RATE", "10/min"), "per-source login rate limit (N/min or N/s)")
loginBurst       = flag.Int("login-burst", envOrInt("TAP_LOGIN_BURST", 5), "per-source burst allowance")
lockoutThreshold = flag.Int("lockout-threshold", envOrInt("TAP_LOCKOUT_THRESHOLD", 5), "consecutive per-username failures before first lockout")
lockoutBase      = flag.Duration("lockout-base", envOrDuration("TAP_LOCKOUT_BASE", 30*time.Second), "initial lockout duration")
lockoutMax       = flag.Duration("lockout-max", envOrDuration("TAP_LOCKOUT_MAX", time.Hour), "maximum lockout duration after escalation")
trustedProxy     = flag.Bool("trusted-proxy", envOrBool("TAP_TRUSTED_PROXY", false), "trust X-Forwarded-For for source IP in rate limiting and auth logs")
```

Also add `envOrFloat64` helper:

```go
func envOrFloat64(k string, def float64) float64 {
    if v := os.Getenv(k); v != "" {
        if f, err := strconv.ParseFloat(v, 64); err == nil {
            return f
        }
        fmt.Fprintf(os.Stderr, "warning: %s=%q is not a valid float64; using default %g\n", k, v, def)
    }
    return def
}
```

- [ ] **Step 2: Update `configureLogger` to support log level**

```go
func configureLogger(format, level string) {
    var lvl slog.Level
    switch strings.ToLower(level) {
    case "debug":
        lvl = slog.LevelDebug
    case "warn":
        lvl = slog.LevelWarn
    case "error":
        lvl = slog.LevelError
    default:
        lvl = slog.LevelInfo
    }
    var h slog.Handler
    opts := &slog.HandlerOptions{Level: lvl}
    if format == "text" {
        h = slog.NewTextHandler(os.Stdout, opts)
    } else {
        h = slog.NewJSONHandler(os.Stdout, opts)
    }
    slog.SetDefault(slog.New(h))
}
```

Call with `configureLogger(*logFmt, *logLevel)`.

- [ ] **Step 3: Wire metrics + tracing + ring buffer before server start**

In `runServer()`, after `configureLogger` and before DB open, insert:

```go
startTime := time.Now()

// Initialise metrics. OTLP metric reader wired when endpoint is set.
metricsOpts := metrics.Opts{OTLPHeaders: parseOTLPHeaders(*otlpHeaders)}
if *otlpEndpoint != "" {
    metricsOpts.OTLPEndpoint = *otlpEndpoint
}
if err := metrics.Init(metricsOpts); err != nil {
    slog.Error("init metrics", "err", err)
    os.Exit(1)
}

// Wrap the slog default handler with the ring buffer.
ringBuf := ring.NewBuffer(100)
slog.SetDefault(slog.New(ring.NewHandler(slog.Default().Handler(), ringBuf)))

// Initialise tracing.
if err := tracing.Init(tracing.Opts{
    OTLPEndpoint: *otlpEndpoint,
    OTLPHeaders:  parseOTLPHeaders(*otlpHeaders),
    SampleRate:   *traceSampleRate,
    ServiceName:  "tap",
    Version:      version,
}); err != nil {
    slog.Error("init tracing", "err", err)
    os.Exit(1)
}

slog.Info("tap starting", "event", "startup", "addr", *addr, "data_dir", *dataDir, "version", version)
```

Add `parseOTLPHeaders` helper:

```go
func parseOTLPHeaders(raw string) map[string]string {
    m := make(map[string]string)
    for _, pair := range strings.Split(raw, ",") {
        pair = strings.TrimSpace(pair)
        if pair == "" {
            continue
        }
        k, v, _ := strings.Cut(pair, "=")
        m[strings.TrimSpace(k)] = strings.TrimSpace(v)
    }
    return m
}
```

- [ ] **Step 4: Wire rate limiter and Archiver OnEvict**

After scheduler setup:

```go
limiter := ratelimit.NewLimiter(ratelimit.Opts{
    SourceRate:      parseLoginRate(*loginRate),
    SourceBurst:     *loginBurst,
    FailThreshold:   *lockoutThreshold,
    LockoutBase:     *lockoutBase,
    LockoutMax:      *lockoutMax,
    CleanupInterval: 5 * time.Minute,
})
```

Add `parseLoginRate` helper:

```go
func parseLoginRate(s string) rate.Limit {
    s = strings.TrimSpace(s)
    if strings.HasSuffix(s, "/min") {
        n, err := strconv.Atoi(strings.TrimSuffix(s, "/min"))
        if err == nil && n > 0 {
            return rate.Every(time.Minute / time.Duration(n))
        }
    }
    if strings.HasSuffix(s, "/s") {
        n, err := strconv.Atoi(strings.TrimSuffix(s, "/s"))
        if err == nil && n > 0 {
            return rate.Limit(n)
        }
    }
    return rate.Every(6 * time.Second) // default 10/min
}
```

Wire Archiver OnEvict (M11 must be merged first):

```go
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
```

**Note:** If M11 is not merged yet, leave `OnEvict` nil and add a TODO comment. The build must pass.

- [ ] **Step 5: Update `api.NewMux` call with new opts**

```go
apiMux := api.NewMux(d, api.MuxOpts{
    Poke:               sched.Poke,
    ProxyHandler:       proxyHandler,
    SessionIdleTTL:     *sessionIdleTTL,
    SessionAbsoluteTTL: *sessionAbsoluteTTL,
    CookieSecure:       cookieSecure,
    HashParams:         auth.DefaultParams,
    Limiter:            limiter,
    TrustedProxy:       *trustedProxy,
    MetricsEnabled:     *metricsEnabled,
    RingBuffer:         ringBuf,
    StartTime:          startTime,
    Version:            version,
    PollsActive:        func() int64 { return sched.ActiveCount() },
})
```

Note: `sched.ActiveCount()` must be added to `internal/poll/scheduler.go` if not present — add it as a simple atomic counter.

- [ ] **Step 6: Extend shutdown sequence**

```go
shutdownCtx, sCancel := context.WithTimeout(context.Background(), 30*time.Second)
defer sCancel()
_ = srv.Shutdown(shutdownCtx)
archiver.Stop()
sched.Stop()
limiter.Stop()
client.CloseIdleConnections()
_ = tracing.Shutdown(shutdownCtx)
_ = metrics.Shutdown(shutdownCtx)
// db.Close() deferred already
slog.Info("shutdown complete", "event", "shutdown", "reason", "signal")
```

- [ ] **Step 7: Wire tracing middleware on the main mux**

```go
mux := http.NewServeMux()
mux.Handle("/api/", tracing.Middleware(apiMux))
mux.Handle("/healthz", tracing.Middleware(apiMux))
mux.Handle("/", server.SPAHandler())
```

- [ ] **Step 8: Build and smoke test**

```bash
make build 2>&1 | tail -10
./bin/tap --help 2>&1 | grep -E "metrics|otlp|lockout|trusted"
```

Expected: new flags appear in help output.

- [ ] **Step 9: Commit**

```bash
git add cmd/tap/main.go
git commit -m "M12: wire metrics, tracing, ring buffer, rate limiter, and new flags in cmd/tap/main.go"
```

---

## Task 12: `tap admin list` + `tap admin disable` + `tap healthcheck`

**Skills:** `superpowers:test-driven-development`, `golang-cli`, `golang-testing`

**Files:**
- Modify: `cmd/tap/admin.go`
- Modify: `cmd/tap/admin_test.go`

- [ ] **Step 1: Write failing tests**

Append to `cmd/tap/admin_test.go`:

```go
func TestAdminList_Empty(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runAdmin([]string{"list", "--data", t.TempDir()},
		strings.NewReader(""), &stdout, &stderr, auth.DefaultParams)
	assert.Equal(t, adminExitOK, code)
	assert.Contains(t, stdout.String(), "ID")   // table header
}

func TestAdminList_WithUsers(t *testing.T) {
	dir := t.TempDir()
	// Create a user first.
	var out, _ strings.Builder
	runAdmin([]string{"create", "--data", dir},
		strings.NewReader("alice\npassword123\npassword123\n"),
		&out, &strings.Builder{}, auth.Params{Time: 1, Memory: 16 * 1024, Threads: 1, SaltLen: 16, KeyLen: 32})

	var stdout strings.Builder
	code := runAdmin([]string{"list", "--data", dir},
		strings.NewReader(""), &stdout, &strings.Builder{}, auth.DefaultParams)
	assert.Equal(t, adminExitOK, code)
	assert.Contains(t, stdout.String(), "alice")
}

func TestAdminDisable_Success(t *testing.T) {
	dir := t.TempDir()
	var out strings.Builder
	runAdmin([]string{"create", "--data", dir},
		strings.NewReader("bob\npassword123\npassword123\n"),
		&out, &strings.Builder{},
		auth.Params{Time: 1, Memory: 16 * 1024, Threads: 1, SaltLen: 16, KeyLen: 32})

	var stdout, stderr strings.Builder
	code := runAdmin([]string{"disable", "bob", "--data", dir},
		strings.NewReader(""), &stdout, &stderr, auth.DefaultParams)
	assert.Equal(t, adminExitOK, code)
	assert.Contains(t, stdout.String(), "disabled user 'bob'")
}

func TestAdminDisable_NotFound(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runAdmin([]string{"disable", "nobody", "--data", t.TempDir()},
		strings.NewReader(""), &stdout, &stderr, auth.DefaultParams)
	assert.Equal(t, adminExitUserExistsOrGone, code)
	assert.Contains(t, stderr.String(), "not found")
}

func TestAdminDisable_AlreadyDisabled(t *testing.T) {
	dir := t.TempDir()
	var out strings.Builder
	runAdmin([]string{"create", "--data", dir},
		strings.NewReader("carl\npassword123\npassword123\n"),
		&out, &strings.Builder{},
		auth.Params{Time: 1, Memory: 16 * 1024, Threads: 1, SaltLen: 16, KeyLen: 32})
	// Disable once.
	runAdmin([]string{"disable", "carl", "--data", dir},
		strings.NewReader(""), &strings.Builder{}, &strings.Builder{}, auth.DefaultParams)
	// Disable again.
	code := runAdmin([]string{"disable", "carl", "--data", dir},
		strings.NewReader(""), &strings.Builder{}, &strings.Builder{}, auth.DefaultParams)
	assert.Equal(t, adminExitPasswordMismatch, code) // exit 3 = already disabled
}
```

- [ ] **Step 2: Implement new subcommands in `cmd/tap/admin.go`**

Add to the `runAdmin` switch:

```go
case "list":
    return runAdminList(args[1:], stdout, stderr)
case "disable":
    return runAdminDisable(args[1:], stdout, stderr)
```

Implement `runAdminList`:

```go
func runAdminList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("admin list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dataDir := fs.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory")
	if err := fs.Parse(args); err != nil {
		return adminExitGeneric
	}

	ctx := context.Background()
	d, err := openAdminDB(ctx, *dataDir)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return adminExitGeneric
	}
	defer d.Close()

	users, err := db.ListUsers(ctx, d)
	if err != nil {
		fmt.Fprintf(stderr, "list users: %v\n", err)
		return adminExitGeneric
	}

	fmt.Fprintf(stdout, "%-4s  %-20s  %-6s  %-20s  %s\n", "ID", "USERNAME", "ROLE", "CREATED", "DISABLED")
	for _, u := range users {
		created := time.Unix(u.CreatedAt, 0).UTC().Format(time.RFC3339)
		disabled := "no"
		if u.DisabledAt.Valid {
			disabled = "yes (" + time.Unix(u.DisabledAt.Int64, 0).UTC().Format(time.RFC3339) + ")"
		}
		fmt.Fprintf(stdout, "%-4d  %-20s  %-6s  %-20s  %s\n",
			u.ID, u.Username, u.Role, created, disabled)
	}
	return adminExitOK
}
```

Implement `runAdminDisable`:

```go
func runAdminDisable(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("admin disable", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dataDir := fs.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory")
	if err := fs.Parse(args); err != nil {
		return adminExitGeneric
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: tap admin disable <username>")
		return adminExitGeneric
	}
	username := fs.Arg(0)

	ctx := context.Background()
	d, err := openAdminDB(ctx, *dataDir)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return adminExitGeneric
	}
	defer d.Close()

	u, err := db.GetUserByUsername(ctx, d, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Fprintf(stderr, "user '%s' not found\n", username)
			return adminExitUserExistsOrGone
		}
		fmt.Fprintf(stderr, "lookup user: %v\n", err)
		return adminExitGeneric
	}

	if u.DisabledAt.Valid {
		fmt.Fprintf(stderr, "user '%s' is already disabled\n", username)
		return adminExitPasswordMismatch // exit 3 per spec
	}

	if err := db.DisableUser(ctx, d, u.ID); err != nil {
		fmt.Fprintf(stderr, "disable user: %v\n", err)
		return adminExitGeneric
	}
	if err := db.DeleteSessionsByUserID(ctx, d, u.ID); err != nil {
		fmt.Fprintf(stderr, "delete sessions: %v\n", err)
		return adminExitGeneric
	}
	fmt.Fprintf(stdout, "disabled user '%s'\n", username)
	return adminExitOK
}
```

- [ ] **Step 3: Implement `tap healthcheck` in `cmd/tap/admin.go`** (or a new `cmd/tap/healthcheck.go`)

Add to `main()` in `cmd/tap/main.go`:

```go
if len(os.Args) >= 2 && os.Args[1] == "healthcheck" {
    os.Exit(runHealthcheck(os.Args[2:]))
}
```

Implement `runHealthcheck`:

```go
func runHealthcheck(args []string) int {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	addr := fs.String("addr", envOr("TAP_ADDR", "127.0.0.1:8080"), "server address")
	timeout := fs.Duration("timeout", 5*time.Second, "HTTP request timeout")
	_ = fs.Parse(args)

	host, port, err := net.SplitHostPort(*addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid addr:", err)
		return 1
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}

	client := &http.Client{Timeout: *timeout}
	url := "http://" + net.JoinHostPort(host, port) + "/healthz"
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck failed:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return 0
	}
	fmt.Fprintln(os.Stderr, "healthcheck: unexpected status", resp.StatusCode)
	return 1
}
```

- [ ] **Step 4: Run admin tests**

```bash
go test ./cmd/tap/... -race -run "TestAdmin" 2>&1 | tail -20
```

- [ ] **Step 5: Commit**

```bash
git add cmd/tap/
git commit -m "M12: add tap admin list, tap admin disable, and tap healthcheck subcommands"
```

---

## Task 13: Poll worker metrics and log taxonomy

**Skills:** `golang-observability`, `golang-error-handling`

**Files:**
- Modify: `internal/poll/worker.go`

- [ ] **Step 1: Add poll metrics and structured log events to worker**

In `internal/poll/worker.go`, in the poll execution function:

```go
// Before poll:
slog.InfoContext(ctx, "polling feed", "event", "poll.start", "feed_id", sub.ID, "feed_url", sub.FeedURL)
start := time.Now()

// After successful poll:
elapsed := time.Since(start).Seconds()
metrics.PollsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "success")))
metrics.PollDuration.Record(ctx, elapsed, metric.WithAttributes(attribute.String("result", "success")))
metrics.EntriesInserted.Add(ctx, int64(insertedCount))
if conditional304 {
    metrics.ConditionalGetHits.Add(ctx, 1)
}
slog.InfoContext(ctx, "poll succeeded",
    "event", "poll.success",
    "feed_id", sub.ID,
    "entries_inserted", insertedCount,
    "duration_ms", int64(elapsed*1000),
    "conditional_hit", conditional304)

// After failed poll:
metrics.PollsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "failure")))
metrics.PollDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attribute.String("result", "failure")))
slog.WarnContext(ctx, "poll failed",
    "event", "poll.failure",
    "feed_id", sub.ID,
    "error", err.Error(),
    "error_count", sub.ErrorCount+1)
```

- [ ] **Step 2: Add poll spans**

Wrap the poll execution with a span:

```go
tracer := otel.Tracer("tap/poll")
ctx, span := tracer.Start(ctx, "poll.feed")
defer span.End()
span.SetAttributes(attribute.Int64("feed_id", sub.ID))
```

- [ ] **Step 3: Build and run poll tests**

```bash
go test ./internal/poll/... -race 2>&1 | tail -10
```

- [ ] **Step 4: Commit**

```bash
git add internal/poll/worker.go
git commit -m "M12: add poll metrics and structured log events to poll worker"
```

---

## Task 14: Proxy cache metrics

**Skills:** `golang-observability`

**Files:**
- Modify: `internal/proxy/handler.go`
- Modify: `internal/proxy/cache.go`

- [ ] **Step 1: Wire cache hit/miss counters in `handler.go`**

In the proxy handler, where the cache lookup result is known:

```go
// Cache hit:
metrics.ProxyCacheHits.Add(r.Context(), 1)

// Cache miss (before origin fetch):
metrics.ProxyCacheMisses.Add(r.Context(), 1)
```

- [ ] **Step 2: Wire eviction and bytes gauge in `cache.go`**

After a size-cap eviction completes:

```go
metrics.ProxyCacheEvictions.Add(context.Background(), int64(evictedCount),
    metric.WithAttributes(attribute.String("reason", "size_cap")))
metrics.ProxyCacheBytes.Record(context.Background(), currentBytes)
```

After a new file is written to cache:

```go
metrics.ProxyCacheBytes.Record(context.Background(), currentBytes)
```

- [ ] **Step 3: Run proxy tests**

```bash
go test ./internal/proxy/... -race 2>&1 | tail -10
```

- [ ] **Step 4: Commit**

```bash
git add internal/proxy/
git commit -m "M12: add proxy cache hit/miss/eviction/bytes metrics"
```

---

## Task 15: SPA — system-status panel

**Skills:** (frontend; no specific Go skill needed)

**Files:**
- Create: `web/src/lib/status.ts`
- Create: `web/src/components/SystemStatus.svelte`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/views/Settings.svelte` (or wherever the settings view lives)
- Create: `web/src/lib/__tests__/status.test.ts`

- [ ] **Step 1: Write the failing test**

Create `web/src/lib/__tests__/status.test.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { getStatus } from '../status';

const mockFetch = vi.fn();
global.fetch = mockFetch;

describe('getStatus', () => {
  beforeEach(() => { mockFetch.mockReset(); });

  it('returns status on 200', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        version: '0.12.0', uptime_seconds: 100, db: 'ok',
        polls_active: 0, polls_total: 0, last_poll_at: null, recent_errors: []
      })
    });
    const s = await getStatus();
    expect(s.version).toBe('0.12.0');
  });

  it('throws on 403', async () => {
    mockFetch.mockResolvedValue({ ok: false, status: 403 });
    await expect(getStatus()).rejects.toThrow();
  });
});
```

- [ ] **Step 2: Run — expect failure**

```bash
pnpm --dir web test -- src/lib/__tests__/status.test.ts 2>&1 | tail -10
```

- [ ] **Step 3: Implement `web/src/lib/status.ts`**

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

export async function getStatus(): Promise<StatusResponse> {
  const resp = await fetch('/api/v1/status');
  if (!resp.ok) throw new Error(`status ${resp.status}`);
  return resp.json();
}
```

- [ ] **Step 4: Run test — expect pass**

```bash
pnpm --dir web test -- src/lib/__tests__/status.test.ts 2>&1 | tail -5
```

- [ ] **Step 5: Implement `web/src/components/SystemStatus.svelte`**

```svelte
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { getStatus, type StatusResponse } from '../lib/status';

  let status = $state<StatusResponse | null>(null);
  let error = $state<string | null>(null);
  let interval: ReturnType<typeof setInterval>;

  async function refresh() {
    try {
      status = await getStatus();
      error = null;
    } catch (e) {
      error = e instanceof Error ? e.message : 'failed';
    }
  }

  onMount(() => {
    refresh();
    interval = setInterval(refresh, 60_000);
  });

  onDestroy(() => clearInterval(interval));

  function formatUptime(s: number): string {
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    return h > 0 ? `${h}h ${m}m` : `${m}m`;
  }
</script>

{#if error}
  <p>Error loading status: {error}</p>
{:else if status}
  <section>
    <h3>System Status</h3>
    <dl>
      <dt>Version</dt><dd>{status.version}</dd>
      <dt>Uptime</dt><dd>{formatUptime(status.uptime_seconds)}</dd>
      <dt>Database</dt><dd>{status.db}</dd>
      <dt>Active polls</dt><dd>{status.polls_active}</dd>
    </dl>
    {#if status.recent_errors.length > 0}
      <h4>Recent errors</h4>
      <ul>
        {#each status.recent_errors as e}
          <li><time>{e.time}</time> [{e.level}] {e.event}</li>
        {/each}
      </ul>
    {:else}
      <p>No recent errors.</p>
    {/if}
  </section>
{:else}
  <p>Loading…</p>
{/if}
```

- [ ] **Step 6: Conditionally render in settings view**

In the settings view (find the file with `<Settings` or role=admin checks):

```svelte
{#if $auth.user?.role === 'admin'}
  <SystemStatus />
{/if}
```

Import at the top: `import SystemStatus from '../components/SystemStatus.svelte';`

- [ ] **Step 7: Run frontend tests and type-check**

```bash
pnpm --dir web test 2>&1 | tail -10
pnpm --dir web run check 2>&1 | tail -10
```

- [ ] **Step 8: Commit**

```bash
git add web/src/
git commit -m "M12: add system-status panel — getStatus(), SystemStatus component, admin-only render"
```

---

## Task 16: `MaxBytesReader` audit — deferred-items fix

**Skills:** `golang-security`

**Files:**
- Modify: `internal/api/subscriptions.go`
- Modify: `internal/api/entries.go`

- [ ] **Step 1: Verify current state of subscriptions.go write handlers**

```bash
grep -n "MaxBytesReader\|json.NewDecoder\|json.Decode" /home/ben.guest/Users/ben/src/tap/internal/api/subscriptions.go
```

- [ ] **Step 2: Add `MaxBytesReader` to POST /subscriptions handler**

In the `createSubscription` handler, before `json.NewDecoder`:

```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB
```

After `json.Decode`, ensure `*http.MaxBytesError` is handled:

```go
if err != nil {
    var maxBytesErr *http.MaxBytesError
    if errors.As(err, &maxBytesErr) {
        writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body too large")
        return
    }
    writeError(w, http.StatusBadRequest, "bad_request", "invalid request body")
    return
}
```

Apply the same pattern to `PATCH /subscriptions/{id}` and `PATCH /entries/{id}`.

- [ ] **Step 3: Write regression tests for 413**

Append to `internal/api/subscriptions_test.go`:

```go
func TestCreateSubscription_BodyTooLarge(t *testing.T) {
	ts := newTestServer(t, withAdminSession())
	defer ts.Close()

	// 2 MiB body
	body := strings.Repeat("x", 2<<20)
	resp, _ := ts.Client().Post(ts.URL+"/api/v1/subscriptions", "application/json",
		strings.NewReader(body))
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
}
```

- [ ] **Step 4: Run and verify**

```bash
go test ./internal/api/... -race -run "TestCreate.*Large\|TestPatch.*Large" 2>&1 | tail -10
```

- [ ] **Step 5: Commit**

```bash
git add internal/api/subscriptions.go internal/api/entries.go internal/api/subscriptions_test.go internal/api/entries_test.go
git commit -m "M12: fix 413 on oversize body for subscriptions and entries write handlers (deferred-items)"
```

---

## Task 17: Security audit pass

**Skills:** `golang-security` (invoke `/security-review` skill)

**Files:**
- Possibly modify various handlers based on findings

- [ ] **Step 1: Run govulncheck**

```bash
govulncheck ./... 2>&1
```

Document any findings in `docs/security-exceptions.md` if unfixable within M12.

- [ ] **Step 2: Run semgrep**

```bash
semgrep --config=p/golang ./... 2>&1 | grep -E "ERROR|WARN" | head -30
```

Fix `ERROR`-severity findings. Document `WARN`-severity accepted risks.

- [ ] **Step 3: Invoke the `/security-review` skill**

Use the `security-review` skill in your session to drive a structured audit of the current branch changes.

- [ ] **Step 4: Verify route authn/authz table**

```bash
grep -n "m.Handle\|m.HandleFunc" /home/ben.guest/Users/ben/src/tap/internal/api/api.go
```

Cross-reference each route against the table in `docs/specs/2026-05-10-m12-observability-hardening.md` §"Route authn/authz table". Confirm every route is in the table and has the correct middleware.

- [ ] **Step 5: Verify no raw `http.Get` / inline `http.Client{}`**

```bash
grep -rn "http\.Get\|http\.Post\|http\.Client{" /home/ben.guest/Users/ben/src/tap/internal/ /home/ben.guest/Users/ben/src/tap/cmd/ 2>&1
```

Any hit outside the documented OTLP exception is a bug — fix before continuing.

- [ ] **Step 6: Verify MaxBytesReader on all write handlers**

```bash
grep -rn "MaxBytesReader" /home/ben.guest/Users/ben/src/tap/internal/api/ 2>&1
```

Every POST/PATCH handler must have a hit. Compare against the table in the spec.

- [ ] **Step 7: Commit audit results**

```bash
git add docs/security-exceptions.md  # if created
git commit -m "M12: security audit pass — govulncheck, semgrep, route audit, MaxBytesReader audit"
```

---

## Task 18: Full test suite + integration smoke test

**Skills:** `golang-testing`

**Files:** none new

- [ ] **Step 1: Run the full Go test suite with race detector**

```bash
make test 2>&1 | tail -30
```

Expected: all PASS, no races.

- [ ] **Step 2: Run the SPA test suite and type check**

```bash
pnpm --dir web test 2>&1 | tail -10
pnpm --dir web run check 2>&1 | tail -10
```

Expected: all PASS, zero type errors.

- [ ] **Step 3: Build the static binary**

```bash
make build 2>&1 | tail -5
```

Expected: `bin/tap` produced without errors.

- [ ] **Step 4: Smoke test `tap healthcheck` against a running server**

```bash
TAP_ADMIN_USERNAME=testadmin TAP_ADMIN_PASSWORD=testpassword123 \
  ./bin/tap --data /tmp/tap-m12-test &
SERVER_PID=$!
sleep 2
./bin/tap healthcheck
echo "exit: $?"
kill $SERVER_PID
```

Expected: `exit: 0`.

- [ ] **Step 5: Smoke test `/metrics` disabled by default**

```bash
curl -s http://127.0.0.1:8080/metrics | head -5
```

Expected: 404 response.

- [ ] **Step 6: Final commit**

```bash
git add -A
git commit -m "M12: final integration — all tests green, smoke tests pass"
```

---

## Self-Review Checklist

### Spec coverage

| Spec requirement | Task |
|---|---|
| `internal/metrics` package with global provider | Task 5 |
| `internal/ring` ring buffer + slog handler | Task 2 |
| `internal/ratelimit` limiter | Task 3 |
| `internal/tracing` package | Task 6 |
| `auth.NeedsRehash` + login re-hash | Tasks 4, 8 |
| New error codes `forbidden`, `rate_limited` | Task 7 |
| Login rate limiting + lockout wiring | Task 8 |
| `GET /api/v1/status` admin-only | Task 9 |
| `/healthz` JSON body | Task 10 |
| `GET /metrics` endpoint | Task 10 |
| New flags (12 total) | Task 11 |
| `cmd/tap/main.go` wire-up | Task 11 |
| `tap admin list` | Task 12 |
| `tap admin disable` | Task 12 |
| `tap healthcheck` | Task 12 |
| Poll metrics + log events | Task 13 |
| Proxy cache metrics | Task 14 |
| SPA system-status panel | Task 15 |
| `MaxBytesReader` deferred-items fix | Task 16 |
| `govulncheck` + `semgrep` + audit | Task 17 |
| Full test suite | Task 18 |

### Open dependency note

**M11 `OnEvict` callback:** Task 11 wires `archiver.ArchiverOpts.OnEvict` to increment `metrics.ProxyCacheEvictions`. This requires M11 to be merged first. If M11 is not yet merged at implementation time, leave `OnEvict` nil and add a `TODO(m12): wire OnEvict once M11 merges` comment. The build must pass regardless.

**M7 routes in route audit (Task 17):** The route audit table in the spec covers all M7 and M9 routes. If M7/M9 are not yet merged, audit only the routes that exist in the current codebase and note the pending rows.

**`sched.ActiveCount()`:** Task 11 assumes `poll.Scheduler` exposes `ActiveCount() int64`. If this method does not exist after M11 merges, add it as a simple `sync/atomic.Int64` counter incremented on dispatch and decremented on completion.
