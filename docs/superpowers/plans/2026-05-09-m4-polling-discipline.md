# M4 Polling Discipline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the polling discipline layer for Tap — adaptive cadence, SSRF guard, per-host concurrency cap, and exponential error backoff — wrapping a single shared HTTP client used by both the polling pipeline and the media proxy.

**Architecture:** Two new leaf packages: `internal/cadence` (pure formula functions) and `internal/httpx` (shared client with SSRF dialer, per-host RoundTripper limiter, and redirect re-check). One DB migration adds `velocity_24h_x100`. The worker glues cadence + DB on the 304 and error paths; `db.UpdateAfterPoll` glues them on the success path so the post-insert velocity flows directly into the next_poll_at write. `cmd/tap/main.go` constructs the client once via `httpx.NewClient(opts)` and passes it to both `feed.Fetch` and `proxy.NewHandler`.

**Tech stack:** Go 1.25, modernc.org/sqlite (CGO_ENABLED=0), stdlib (`net/http`, `net/netip`, `net.Dialer.ControlContext`, `math/rand/v2`), no third-party deps added.

## Required skills (activate per phase)

Before starting each phase, invoke the listed skills with the `Skill` tool. They contain patterns this plan assumes.

| Phase | Required skills |
|---|---|
| All phases | `superpowers:test-driven-development` — every task in this plan is RED→GREEN→REFACTOR. The roadmap mandates it. |
| Phase 1 (cadence) | `golang-testing` — table-driven tests, subtests, `t.Run`. |
| Phase 2 (httpx) | `golang-security` (SSRF, defense in depth, allowlist defaults), `golang-concurrency` (buffered chan as semaphore + `sync.Mutex` on lazy map; `ctx.Done()` short-circuit; release-on-body-close with `sync.Once`), `golang-context` (ctx propagation through dialer and redirect callback). |
| Phase 3 (db) | `golang-database` (parameterised queries, `tx.Rollback` on error, `sql.NullString` handling). |
| Phase 4 (feed) | None new beyond TDD. |
| Phase 5 (poll) | `golang-error-handling` (wrap with `%w`, sentinel errors), `golang-context`. |
| Phase 6 (main + e2e) | `golang-cli` (stdlib `flag.Var()` for repeatable flags, deprecated alias). |

## Useful MCP tools

| Tool | When to invoke |
|---|---|
| `context7` | Fetch current docs for `net/http` (`Transport.DialContext`, `Client.CheckRedirect`), `net.Dialer.ControlContext`, `net/netip` (Prefix/Addr semantics), and `math/rand/v2`. Training data may be stale on the v2 rand API. Run e.g. `context7 query "go net.Dialer ControlContext signature"` or `context7 query "go http.Client CheckRedirect"` before writing the dialer wiring in Phase 2. |

## Plan location and git posture

- Plan path: `docs/superpowers/plans/2026-05-09-m4-polling-discipline.md` (this file).
- `docs/superpowers/` is gitignored. **Do not commit this plan.** Per the user's standing instruction, plans are working documents and don't go to git.
- Specs are committed; the M4 spec is at `docs/specs/2026-05-09-m4-polling-discipline.md` (commit `b91f031`).

## File structure

**New files:**

```
internal/cadence/
  cadence.go                 # IntervalFromVelocity, BackoffFromErrorCount,
                             # ApplyServerFloors, ParseRetryAfter, ParseCacheMaxAge
  cadence_test.go

internal/httpx/
  ssrf.go                    # SSRFPolicy, AllowAddr, AllowHostname, ParseSSRFPolicy,
                             # CheckRedirect callback
  ssrf_test.go
  hostlimit.go               # hostLimiter RoundTripper
  hostlimit_test.go
  client.go                  # NewClient(opts) — composes everything
  client_test.go

internal/db/migrations/
  0003_polling_discipline.sql
```

**Modified files:**

```
internal/feed/parse.go            # drop UserAgent, populate FetchResult on error,
                                  # add RetryAfter/CacheMaxAge fields
internal/feed/parse_test.go       # extended

internal/db/subscriptions.go      # add QueryVelocity, UpdateAfterNotModified gains velocity
internal/db/subscriptions_test.go # extended
internal/db/entries.go            # PollResult gains cadence inputs;
                                  # UpdateAfterPoll computes velocity + next_poll_at internally
internal/db/entries_test.go       # extended

internal/poll/scheduler.go        # SchedulerOpts gains Floor/Ceiling/ErrorBase/Now/Rand
internal/poll/worker.go           # new error/304/success branches
internal/poll/worker_test.go      # extended

cmd/tap/main.go                   # use httpx.NewClient; new flags; deprecated alias
cmd/tap/main_test.go              # SSRF, retry-after, error backoff e2e

README.md                         # M4 trust posture section
```

## Commit cadence

One commit per task (after the GREEN+REFACTOR step). Commit messages match prior M-series style: `M4: <subject>` for cohesive units, lowercased imperatives for sub-tasks. Examples: `M4: add cadence package`, `M4: SSRF dialer + redirect re-check`. End each message with the standard `Co-Authored-By` line.

After every task, run `make test` to verify the whole suite stays green. **Anything red is a stop-the-line.** If a previously-passing test fails after your change, fix it before proceeding — don't accumulate red.

---

## Phase 1: Pure cadence package

A new leaf package `internal/cadence` with five pure functions. No external imports beyond stdlib. No dependencies on `internal/db` or `internal/poll` so both can import it.

### Task 1.1: Create the package skeleton

**Files:**
- Create: `internal/cadence/cadence.go`
- Create: `internal/cadence/cadence_test.go`

- [ ] **Step 1: Create the package directory and stub the test file**

```bash
mkdir -p internal/cadence
```

Write `internal/cadence/cadence_test.go` with a placeholder that verifies the package compiles:

```go
package cadence

import "testing"

func TestPackageCompiles(t *testing.T) {
    // Smoke test — replaced in subsequent tasks.
}
```

- [ ] **Step 2: Stub the impl file**

Write `internal/cadence/cadence.go`:

```go
// Package cadence holds the pure functions that derive a feed's next-poll
// time from its velocity (entries/day), error history, and origin headers.
// Leaf package: imports only stdlib so internal/db and internal/poll can
// both depend on it without cycling.
package cadence
```

- [ ] **Step 3: Verify the package compiles and the smoke test passes**

```bash
go test ./internal/cadence/...
```

Expected: `ok  github.com/bcrisp4/tap/internal/cadence`

- [ ] **Step 4: Commit**

```bash
git add internal/cadence/
git commit -m "$(cat <<'EOF'
M4: add cadence package skeleton

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 1.2: `IntervalFromVelocity`

**Files:**
- Modify: `internal/cadence/cadence.go`
- Modify: `internal/cadence/cadence_test.go`

- [ ] **Step 1: Write the failing test**

Replace the smoke test with a table-driven test in `internal/cadence/cadence_test.go`:

```go
package cadence

import (
    "testing"
    "time"
)

func TestIntervalFromVelocity(t *testing.T) {
    floor := 15 * time.Minute
    ceiling := 24 * time.Hour
    cases := []struct {
        name        string
        velocityX100 int
        want        time.Duration
    }{
        {"zero velocity hits ceiling", 0, ceiling},
        {"half entry per day hits ceiling", 50, ceiling},
        {"one entry per day exactly hits ceiling", 100, ceiling},
        {"two entries per day -> 12h", 200, 12 * time.Hour},
        {"ten entries per day -> 2h24m", 1000, 2*time.Hour + 24*time.Minute},
        {"96 entries per day -> 15m floor", 9600, floor},
        {"200 entries per day clamped to floor", 20000, floor},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := IntervalFromVelocity(tc.velocityX100, floor, ceiling)
            if got != tc.want {
                t.Errorf("IntervalFromVelocity(%d) = %v; want %v", tc.velocityX100, got, tc.want)
            }
        })
    }
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/cadence/ -run TestIntervalFromVelocity -v
```

Expected: FAIL with "IntervalFromVelocity undefined".

- [ ] **Step 3: Write the minimal implementation**

Add to `internal/cadence/cadence.go`:

```go
import "time"

// IntervalFromVelocity converts a fixed-point velocity (entries/day × 100) to
// a polling interval, clamped to [floor, ceiling]. Velocities below 1 entry/day
// hit the ceiling directly; the formula 86400/velocity_per_day produces the
// raw interval before clamping.
func IntervalFromVelocity(velocityX100 int, floor, ceiling time.Duration) time.Duration {
    if velocityX100 < 100 {
        return ceiling
    }
    interval := time.Duration(int64(86400)*100/int64(velocityX100)) * time.Second
    if interval < floor {
        return floor
    }
    if interval > ceiling {
        return ceiling
    }
    return interval
}
```

- [ ] **Step 4: Verify the test passes**

```bash
go test ./internal/cadence/ -run TestIntervalFromVelocity -v
```

Expected: PASS for all 7 sub-tests.

- [ ] **Step 5: Commit**

```bash
git add internal/cadence/
git commit -m "$(cat <<'EOF'
M4: cadence.IntervalFromVelocity

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 1.3: `BackoffFromErrorCount`

**Files:**
- Modify: `internal/cadence/cadence.go`
- Modify: `internal/cadence/cadence_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/cadence/cadence_test.go`:

```go
import "math/rand/v2"

func TestBackoffFromErrorCount(t *testing.T) {
    base := 5 * time.Minute
    ceiling := 24 * time.Hour

    // No-jitter cases (rng=nil, jitterFrac=0) — pure exponential growth.
    cases := []struct {
        name       string
        errorCount int
        want       time.Duration
    }{
        {"first error -> base", 1, 5 * time.Minute},
        {"second error -> 10m", 2, 10 * time.Minute},
        {"third error -> 20m", 3, 20 * time.Minute},
        {"fourth error -> 40m", 4, 40 * time.Minute},
        {"fifth error -> 1h20m", 5, 80 * time.Minute},
        {"twelfth error caps at ceiling", 12, 24 * time.Hour},
        {"zero error_count is treated as 1", 0, 5 * time.Minute},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := BackoffFromErrorCount(tc.errorCount, base, ceiling, 0, nil)
            if got != tc.want {
                t.Errorf("BackoffFromErrorCount(%d) = %v; want %v", tc.errorCount, got, tc.want)
            }
        })
    }
}

func TestBackoffFromErrorCount_Jitter(t *testing.T) {
    base := 5 * time.Minute
    ceiling := 24 * time.Hour
    // Deterministic rng for reproducibility. ChaCha8 seed shape: [32]byte.
    rng := rand.New(rand.NewChaCha8([32]byte{1, 2, 3}))
    delay := BackoffFromErrorCount(2, base, ceiling, 0.25, rng)
    // 2nd error: base * 2 = 10m. With 25% jitter, [10m, 12m30s).
    if delay < 10*time.Minute || delay >= 12*time.Minute+30*time.Second {
        t.Errorf("delay %v outside jitter window [10m, 12m30s)", delay)
    }
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/cadence/ -run TestBackoff -v
```

Expected: FAIL with "BackoffFromErrorCount undefined".

- [ ] **Step 3: Write the minimal implementation**

Add to `internal/cadence/cadence.go`:

```go
import "math/rand/v2"

// BackoffFromErrorCount returns the delay before the next retry given the
// consecutive error count. Pure exponential: delay = min(base * 2^(n-1), ceiling).
// If jitterFrac > 0 and rng is non-nil, adds uniform random in [0, delay*jitterFrac).
//
// The jitter can push the result past ceiling — that's intentional. It bounds
// the thundering-herd risk when many feeds errored together; spec acknowledges
// the [ceiling, ceiling*1.25) range for n >= log2(ceiling/base)+1.
func BackoffFromErrorCount(errorCount int, base, ceiling time.Duration, jitterFrac float64, rng *rand.Rand) time.Duration {
    if errorCount < 1 {
        errorCount = 1
    }
    delay := base
    for i := 1; i < errorCount; i++ {
        if delay >= ceiling {
            delay = ceiling
            break
        }
        delay *= 2
    }
    if delay > ceiling {
        delay = ceiling
    }
    if jitterFrac > 0 && rng != nil {
        max := time.Duration(float64(delay) * jitterFrac)
        if max > 0 {
            delay += time.Duration(rng.Int64N(int64(max)))
        }
    }
    return delay
}
```

- [ ] **Step 4: Verify the tests pass**

```bash
go test ./internal/cadence/ -run TestBackoff -v
```

Expected: PASS for both `TestBackoffFromErrorCount` (7 sub-tests) and `TestBackoffFromErrorCount_Jitter`.

- [ ] **Step 5: Commit**

```bash
git add internal/cadence/
git commit -m "$(cat <<'EOF'
M4: cadence.BackoffFromErrorCount

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 1.4: `ApplyServerFloors`

**Files:**
- Modify: `internal/cadence/cadence.go`
- Modify: `internal/cadence/cadence_test.go`

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestApplyServerFloors(t *testing.T) {
    now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
    cases := []struct {
        name        string
        candidate   time.Time
        retryAfter  time.Time
        cacheMaxAge time.Duration
        want        time.Time
    }{
        {
            "no overrides returns candidate",
            now.Add(2 * time.Hour),
            time.Time{},
            0,
            now.Add(2 * time.Hour),
        },
        {
            "retry-after later than candidate pushes",
            now.Add(2 * time.Hour),
            now.Add(6 * time.Hour),
            0,
            now.Add(6 * time.Hour),
        },
        {
            "retry-after earlier than candidate is ignored",
            now.Add(6 * time.Hour),
            now.Add(2 * time.Hour),
            0,
            now.Add(6 * time.Hour),
        },
        {
            "cache max-age later than candidate pushes",
            now.Add(2 * time.Hour),
            time.Time{},
            6 * time.Hour,
            now.Add(6 * time.Hour),
        },
        {
            "both set, later wins",
            now.Add(2 * time.Hour),
            now.Add(4 * time.Hour),
            6 * time.Hour,
            now.Add(6 * time.Hour),
        },
        {
            "both set, retry-after later wins",
            now.Add(2 * time.Hour),
            now.Add(8 * time.Hour),
            6 * time.Hour,
            now.Add(8 * time.Hour),
        },
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := ApplyServerFloors(tc.candidate, tc.retryAfter, tc.cacheMaxAge, now)
            if !got.Equal(tc.want) {
                t.Errorf("ApplyServerFloors = %v; want %v", got, tc.want)
            }
        })
    }
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/cadence/ -run TestApplyServerFloors -v
```

Expected: FAIL with "ApplyServerFloors undefined".

- [ ] **Step 3: Write the minimal implementation**

Add:

```go
// ApplyServerFloors pushes the candidate next-poll time forward (never
// backward) based on origin response headers.
func ApplyServerFloors(candidate, retryAfter time.Time, cacheMaxAge time.Duration, now time.Time) time.Time {
    if !retryAfter.IsZero() && retryAfter.After(candidate) {
        candidate = retryAfter
    }
    if cacheMaxAge > 0 {
        cacheFloor := now.Add(cacheMaxAge)
        if cacheFloor.After(candidate) {
            candidate = cacheFloor
        }
    }
    return candidate
}
```

- [ ] **Step 4: Verify the test passes**

```bash
go test ./internal/cadence/ -run TestApplyServerFloors -v
```

Expected: PASS for all 6 sub-tests.

- [ ] **Step 5: Commit**

```bash
git add internal/cadence/
git commit -m "$(cat <<'EOF'
M4: cadence.ApplyServerFloors

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 1.5: `ParseRetryAfter`

**Files:**
- Modify: `internal/cadence/cadence.go`
- Modify: `internal/cadence/cadence_test.go`

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestParseRetryAfter(t *testing.T) {
    now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
    cases := []struct {
        name   string
        header string
        wantOK bool
        want   time.Time
    }{
        {"empty -> false", "", false, time.Time{}},
        {"whitespace -> false", "   ", false, time.Time{}},
        {"seconds form", "60", true, now.Add(60 * time.Second)},
        {"seconds form with whitespace", "  120  ", true, now.Add(120 * time.Second)},
        {"negative seconds rejected", "-5", false, time.Time{}},
        {"http-date form", "Tue, 09 May 2026 13:00:00 GMT", true, time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC)},
        {"malformed string -> false", "tomorrow please", false, time.Time{}},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got, ok := ParseRetryAfter(tc.header, now)
            if ok != tc.wantOK {
                t.Errorf("ok = %v; want %v", ok, tc.wantOK)
            }
            if ok && !got.Equal(tc.want) {
                t.Errorf("time = %v; want %v", got, tc.want)
            }
        })
    }
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/cadence/ -run TestParseRetryAfter -v
```

Expected: FAIL with "ParseRetryAfter undefined".

- [ ] **Step 3: Write the minimal implementation**

Add:

```go
import (
    "net/http"
    "strconv"
    "strings"
)

// ParseRetryAfter parses the HTTP Retry-After header per RFC 7231 §7.1.3:
// either a non-negative integer of seconds (delta-seconds) or an HTTP-date.
// Returns (target time, true) on success; (zero, false) on absent or malformed.
func ParseRetryAfter(header string, now time.Time) (time.Time, bool) {
    header = strings.TrimSpace(header)
    if header == "" {
        return time.Time{}, false
    }
    if secs, err := strconv.Atoi(header); err == nil {
        if secs < 0 {
            return time.Time{}, false
        }
        return now.Add(time.Duration(secs) * time.Second), true
    }
    if t, err := http.ParseTime(header); err == nil {
        return t, true
    }
    return time.Time{}, false
}
```

- [ ] **Step 4: Verify the test passes**

```bash
go test ./internal/cadence/ -run TestParseRetryAfter -v
```

Expected: PASS for all 7 sub-tests.

- [ ] **Step 5: Commit**

```bash
git add internal/cadence/
git commit -m "$(cat <<'EOF'
M4: cadence.ParseRetryAfter

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 1.6: `ParseCacheMaxAge`

**Files:**
- Modify: `internal/cadence/cadence.go`
- Modify: `internal/cadence/cadence_test.go`

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestParseCacheMaxAge(t *testing.T) {
    cases := []struct {
        name   string
        header string
        wantOK bool
        want   time.Duration
    }{
        {"empty -> false", "", false, 0},
        {"max-age=600", "max-age=600", true, 600 * time.Second},
        {"with other directives", "public, max-age=3600, must-revalidate", true, 3600 * time.Second},
        {"no max-age -> false", "no-cache, no-store", false, 0},
        {"max-age with whitespace", "  max-age=120  ", true, 120 * time.Second},
        {"max-age=0", "max-age=0", true, 0},
        {"negative rejected", "max-age=-5", false, 0},
        {"non-numeric rejected", "max-age=foo", false, 0},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got, ok := ParseCacheMaxAge(tc.header)
            if ok != tc.wantOK {
                t.Errorf("ok = %v; want %v", ok, tc.wantOK)
            }
            if ok && got != tc.want {
                t.Errorf("duration = %v; want %v", got, tc.want)
            }
        })
    }
}
```

- [ ] **Step 2: Run the test and verify it fails**

```bash
go test ./internal/cadence/ -run TestParseCacheMaxAge -v
```

Expected: FAIL with "ParseCacheMaxAge undefined".

- [ ] **Step 3: Write the minimal implementation**

Add:

```go
// ParseCacheMaxAge extracts max-age from a Cache-Control header.
// Returns (duration, true) when present and non-negative; (0, false) otherwise.
// max-age=0 returns (0, true) — the caller decides whether zero floors anything.
func ParseCacheMaxAge(header string) (time.Duration, bool) {
    for _, part := range strings.Split(header, ",") {
        part = strings.TrimSpace(part)
        const prefix = "max-age="
        if strings.HasPrefix(part, prefix) {
            secs, err := strconv.Atoi(part[len(prefix):])
            if err != nil || secs < 0 {
                return 0, false
            }
            return time.Duration(secs) * time.Second, true
        }
    }
    return 0, false
}
```

- [ ] **Step 4: Verify the test passes**

```bash
go test ./internal/cadence/... -v
```

Expected: PASS for all cadence tests.

- [ ] **Step 5: Commit**

```bash
git add internal/cadence/
git commit -m "$(cat <<'EOF'
M4: cadence.ParseCacheMaxAge

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 2: httpx package

**Activate skills before this phase:** `golang-security`, `golang-concurrency`, `golang-context`. The SSRF guard is a security feature with real defense-in-depth implications; the per-host limiter has subtle concurrency invariants (release-on-body-close, ctx cancellation).

**Useful MCP query before starting:** Run `context7 query "go net.Dialer ControlContext signature Go 1.21+"` to confirm the exact signature; the parameter order changed across Go versions.

### Task 2.1: SSRF policy struct + `AllowAddr`

**Files:**
- Create: `internal/httpx/ssrf.go`
- Create: `internal/httpx/ssrf_test.go`

- [ ] **Step 1: Create the package and write the failing test**

```bash
mkdir -p internal/httpx
```

Write `internal/httpx/ssrf_test.go`:

```go
package httpx

import (
    "errors"
    "net/netip"
    "testing"
)

func TestAllowAddr_DefaultRejects(t *testing.T) {
    cases := []string{
        "127.0.0.1",
        "10.0.0.1",
        "172.16.5.5",
        "192.168.1.1",
        "169.254.1.1",
        "100.64.1.1",
        "0.0.0.0",
        "::1",
        "fe80::1",
        "fc00::1",
        "::",
        "::ffff:127.0.0.1",
    }
    p := SSRFPolicy{}
    for _, s := range cases {
        t.Run(s, func(t *testing.T) {
            addr := netip.MustParseAddr(s)
            err := p.AllowAddr(addr)
            if err == nil {
                t.Errorf("expected reject for %s, got nil", s)
            }
            if !errors.Is(err, ErrSSRFBlocked) {
                t.Errorf("expected ErrSSRFBlocked, got %v", err)
            }
        })
    }
}

func TestAllowAddr_PublicAccepted(t *testing.T) {
    p := SSRFPolicy{}
    cases := []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"}
    for _, s := range cases {
        t.Run(s, func(t *testing.T) {
            addr := netip.MustParseAddr(s)
            if err := p.AllowAddr(addr); err != nil {
                t.Errorf("expected allow for %s, got %v", s, err)
            }
        })
    }
}

func TestAllowAddr_Disabled(t *testing.T) {
    p := SSRFPolicy{Disabled: true}
    addr := netip.MustParseAddr("127.0.0.1")
    if err := p.AllowAddr(addr); err != nil {
        t.Errorf("Disabled=true should allow loopback, got %v", err)
    }
}

func TestAllowAddr_CIDRAllowlist(t *testing.T) {
    p := SSRFPolicy{
        AllowCIDRs: []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24")},
    }
    if err := p.AllowAddr(netip.MustParseAddr("192.168.1.5")); err != nil {
        t.Errorf("192.168.1.5 should be allowlisted, got %v", err)
    }
    if err := p.AllowAddr(netip.MustParseAddr("192.168.2.5")); err == nil {
        t.Errorf("192.168.2.5 should be rejected (outside allowlist)")
    }
}
```

- [ ] **Step 2: Verify it fails**

```bash
go test ./internal/httpx/ -v
```

Expected: FAIL with "SSRFPolicy undefined" / "ErrSSRFBlocked undefined".

- [ ] **Step 3: Write the minimal implementation**

Write `internal/httpx/ssrf.go`:

```go
// Package httpx provides Tap's shared HTTP client with SSRF protection
// and per-host concurrency limiting. All outbound HTTP — feed polls and
// media-proxy origin fetches — flows through one client constructed via
// NewClient.
package httpx

import (
    "errors"
    "fmt"
    "net/netip"
)

// (the `strings` import will be added in Task 2.2 when AllowHostname lands.)

// ErrSSRFBlocked is returned by AllowAddr / CheckRedirect when the policy
// rejects a destination. errors.Is can be used to distinguish SSRF errors
// from other dial failures.
var ErrSSRFBlocked = errors.New("ssrf: destination not allowed")

// SSRFPolicy describes the destination policy applied to every outbound
// request. Disabled bypasses both the dialer check and the redirect re-check.
// AllowSuffixes match dot-boundary against URL hostnames before DNS;
// AllowCIDRs match resolved IPs after DNS.
type SSRFPolicy struct {
    Disabled      bool
    AllowSuffixes []string
    AllowCIDRs    []netip.Prefix
}

// defaultRejectCIDRs covers loopback, RFC1918, link-local, CGNAT, ULA,
// and the IPv4-mapped IPv6 form of all the above. CGNAT (100.64.0.0/10)
// is included because that's Tailscale's default range — operators on
// Tailscale must explicitly allowlist their tailnet.
var defaultRejectCIDRs = []netip.Prefix{
    netip.MustParsePrefix("127.0.0.0/8"),
    netip.MustParsePrefix("10.0.0.0/8"),
    netip.MustParsePrefix("172.16.0.0/12"),
    netip.MustParsePrefix("192.168.0.0/16"),
    netip.MustParsePrefix("169.254.0.0/16"),
    netip.MustParsePrefix("100.64.0.0/10"),
    netip.MustParsePrefix("0.0.0.0/8"),
    netip.MustParsePrefix("::1/128"),
    netip.MustParsePrefix("fe80::/10"),
    netip.MustParsePrefix("fc00::/7"),
    netip.MustParsePrefix("::/128"),
}

// AllowAddr returns nil if the resolved IP is permitted, ErrSSRFBlocked
// (wrapped with location detail) if not. Disabled and CIDR allowlist
// short-circuit the default-reject path.
func (p SSRFPolicy) AllowAddr(addr netip.Addr) error {
    if p.Disabled {
        return nil
    }
    // Allowlist takes precedence — explicit operator override.
    for _, cidr := range p.AllowCIDRs {
        if cidr.Contains(addr) {
            return nil
        }
    }
    // Unmap IPv4-mapped IPv6 (::ffff:127.0.0.1) so v4 rules apply.
    addrUnmapped := addr.Unmap()
    for _, reject := range defaultRejectCIDRs {
        if reject.Contains(addr) || reject.Contains(addrUnmapped) {
            return fmt.Errorf("%w: %s in %s", ErrSSRFBlocked, addr, reject)
        }
    }
    return nil
}

```

(Note: `AllowHostname` is added in Task 2.2, after its own failing test. Don't implement it here.)

- [ ] **Step 4: Verify all tests pass**

```bash
go test ./internal/httpx/ -v
```

Expected: PASS for all `TestAllowAddr*` tests.

- [ ] **Step 5: Commit**

```bash
git add internal/httpx/
git commit -m "$(cat <<'EOF'
M4: httpx.SSRFPolicy and AllowAddr

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2.2: `AllowHostname` suffix matching

**Files:**
- Modify: `internal/httpx/ssrf.go`
- Modify: `internal/httpx/ssrf_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/httpx/ssrf_test.go`:

```go
func TestAllowHostname_Suffix(t *testing.T) {
    p := SSRFPolicy{AllowSuffixes: []string{"home.lan", "ts.net"}}
    cases := []struct {
        host string
        want bool
    }{
        {"home.lan", true},
        {"nas.home.lan", true},
        {"deeply.nested.home.lan", true},
        {"notmyhome.lan", false},
        {"home.lan.evil.com", false},
        {"NAS.HOME.LAN", true}, // case insensitive
        {"foo.ts.net", true},
        {"barts.net", false},   // not dot-boundary
        {"unrelated.com", false},
    }
    for _, tc := range cases {
        t.Run(tc.host, func(t *testing.T) {
            if got := p.AllowHostname(tc.host); got != tc.want {
                t.Errorf("AllowHostname(%q) = %v; want %v", tc.host, got, tc.want)
            }
        })
    }
}

func TestAllowHostname_DisabledAllowsAll(t *testing.T) {
    p := SSRFPolicy{Disabled: true}
    if !p.AllowHostname("anything.example") {
        t.Error("Disabled should allow any hostname")
    }
}
```

- [ ] **Step 2: Run and verify it fails**

```bash
go test ./internal/httpx/ -run TestAllowHostname -v
```

Expected: FAIL with "AllowHostname undefined".

- [ ] **Step 3: Implement `AllowHostname`**

Add to `internal/httpx/ssrf.go` (`strings` already imported in 2.1 for `ParseSSRFPolicy` if you wrote it; otherwise add the import here):

```go
// AllowHostname returns true if the hostname matches an AllowSuffixes entry
// (dot-boundary). Case-insensitive. Disabled implies true.
func (p SSRFPolicy) AllowHostname(host string) bool {
    if p.Disabled {
        return true
    }
    host = strings.ToLower(host)
    for _, suffix := range p.AllowSuffixes {
        suffix = strings.ToLower(suffix)
        if host == suffix || strings.HasSuffix(host, "."+suffix) {
            return true
        }
    }
    return false
}
```

- [ ] **Step 4: Verify**

```bash
go test ./internal/httpx/ -run TestAllowHostname -v
```

Expected: PASS for all 9 sub-tests + the disabled case.

- [ ] **Step 5: Commit**

```bash
git add internal/httpx/
git commit -m "$(cat <<'EOF'
M4: httpx.AllowHostname dot-boundary suffix matching

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2.3: `ParseSSRFPolicy` from CLI form

**Files:**
- Modify: `internal/httpx/ssrf.go`
- Modify: `internal/httpx/ssrf_test.go`

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestParseSSRFPolicy_Empty(t *testing.T) {
    p, err := ParseSSRFPolicy(false, nil)
    if err != nil {
        t.Fatal(err)
    }
    if p.Disabled || len(p.AllowSuffixes) != 0 || len(p.AllowCIDRs) != 0 {
        t.Errorf("expected empty policy, got %+v", p)
    }
}

func TestParseSSRFPolicy_AutoDetect(t *testing.T) {
    p, err := ParseSSRFPolicy(false, []string{
        "192.168.1.0/24",         // CIDR
        "127.0.0.1",              // bare IPv4 -> /32
        "::1",                    // bare IPv6 -> /128
        "marlin-tet.ts.net",      // hostname suffix
        "  10.0.0.0/8  ",         // whitespace tolerant
        "",                       // empty entries skipped
    })
    if err != nil {
        t.Fatal(err)
    }
    if len(p.AllowCIDRs) != 4 {
        t.Errorf("AllowCIDRs len = %d; want 4 (got %v)", len(p.AllowCIDRs), p.AllowCIDRs)
    }
    if len(p.AllowSuffixes) != 1 || p.AllowSuffixes[0] != "marlin-tet.ts.net" {
        t.Errorf("AllowSuffixes = %v", p.AllowSuffixes)
    }
}

func TestParseSSRFPolicy_MalformedCIDR(t *testing.T) {
    _, err := ParseSSRFPolicy(false, []string{"not/a/cidr"})
    if err == nil {
        t.Error("expected error for malformed entry")
    }
}

func TestParseSSRFPolicy_DisabledFlag(t *testing.T) {
    p, err := ParseSSRFPolicy(true, nil)
    if err != nil {
        t.Fatal(err)
    }
    if !p.Disabled {
        t.Error("Disabled should be true")
    }
}
```

- [ ] **Step 2: Run and verify it fails**

```bash
go test ./internal/httpx/ -run TestParseSSRFPolicy -v
```

Expected: FAIL with "ParseSSRFPolicy undefined".

- [ ] **Step 3: Write the implementation**

Add to `internal/httpx/ssrf.go`:

```go
// ParseSSRFPolicy converts CLI-form entries into a usable policy. Each entry is
// auto-detected: contains "/" -> CIDR; bare IP literal -> /32 or /128;
// otherwise -> hostname suffix. A malformed entry fails with a clear error
// rather than being silently ignored.
func ParseSSRFPolicy(disabled bool, entries []string) (SSRFPolicy, error) {
    p := SSRFPolicy{Disabled: disabled}
    for _, e := range entries {
        e = strings.TrimSpace(e)
        if e == "" {
            continue
        }
        if strings.Contains(e, "/") {
            prefix, err := netip.ParsePrefix(e)
            if err != nil {
                return SSRFPolicy{}, fmt.Errorf("ssrf-allow: invalid CIDR %q: %w", e, err)
            }
            p.AllowCIDRs = append(p.AllowCIDRs, prefix)
            continue
        }
        if addr, err := netip.ParseAddr(e); err == nil {
            bits := 32
            if addr.Is6() {
                bits = 128
            }
            p.AllowCIDRs = append(p.AllowCIDRs, netip.PrefixFrom(addr, bits))
            continue
        }
        p.AllowSuffixes = append(p.AllowSuffixes, strings.ToLower(e))
    }
    return p, nil
}
```

- [ ] **Step 4: Verify**

```bash
go test ./internal/httpx/ -v
```

Expected: PASS for all `TestParseSSRFPolicy*` tests.

- [ ] **Step 5: Commit**

```bash
git add internal/httpx/
git commit -m "$(cat <<'EOF'
M4: httpx.ParseSSRFPolicy CLI form parser

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2.4: Redirect `CheckRedirect` callback

**Files:**
- Modify: `internal/httpx/ssrf.go`
- Modify: `internal/httpx/ssrf_test.go`

- [ ] **Step 1: Write the failing test**

Append:

```go
import (
    "net/http"
    "net/url"
)

func mustURL(t *testing.T, s string) *url.URL {
    t.Helper()
    u, err := url.Parse(s)
    if err != nil {
        t.Fatalf("parse %q: %v", s, err)
    }
    return u
}

func TestCheckRedirect_DepthLimit(t *testing.T) {
    p := SSRFPolicy{}
    via := make([]*http.Request, 10)
    req := &http.Request{URL: mustURL(t, "https://example.com")}
    if err := p.CheckRedirect(req, via); err == nil {
        t.Error("expected error at 10 redirects")
    }
}

func TestCheckRedirect_LiteralPrivateIP(t *testing.T) {
    p := SSRFPolicy{}
    req := &http.Request{URL: mustURL(t, "http://127.0.0.1:9999/x")}
    if err := p.CheckRedirect(req, nil); err == nil {
        t.Error("expected reject for literal loopback in redirect")
    }
}

func TestCheckRedirect_AllowlistedSuffix(t *testing.T) {
    p := SSRFPolicy{AllowSuffixes: []string{"home.lan"}}
    req := &http.Request{URL: mustURL(t, "http://nas.home.lan/x")}
    if err := p.CheckRedirect(req, nil); err != nil {
        t.Errorf("expected allow for suffix-allowlisted host, got %v", err)
    }
}

func TestCheckRedirect_HostnameDeferToDialer(t *testing.T) {
    // Hostname not allowlisted, not an IP literal — callback returns nil
    // and the dialer's ControlContext does the actual reject.
    p := SSRFPolicy{}
    req := &http.Request{URL: mustURL(t, "https://example.com")}
    if err := p.CheckRedirect(req, nil); err != nil {
        t.Errorf("expected nil (defer to dialer), got %v", err)
    }
}

func TestCheckRedirect_Disabled(t *testing.T) {
    p := SSRFPolicy{Disabled: true}
    req := &http.Request{URL: mustURL(t, "http://127.0.0.1/x")}
    if err := p.CheckRedirect(req, nil); err != nil {
        t.Errorf("Disabled should allow, got %v", err)
    }
}
```

- [ ] **Step 2: Verify it fails**

```bash
go test ./internal/httpx/ -run TestCheckRedirect -v
```

Expected: FAIL with "CheckRedirect undefined".

- [ ] **Step 3: Implement**

Add to `internal/httpx/ssrf.go`:

```go
// CheckRedirect is suitable for http.Client.CheckRedirect. It enforces a
// 10-redirect depth limit, then re-validates the destination:
//   - if Disabled, allow.
//   - if the URL hostname is suffix-allowlisted, allow.
//   - if the URL hostname is a literal IP, check it against AllowAddr.
//   - otherwise return nil — the dialer's ControlContext will check the
//     resolved IP when the redirect's connect runs.
//
// The callback is necessary but not sufficient: the dialer is the
// load-bearing layer for hostname-based redirects.
func (p SSRFPolicy) CheckRedirect(req *http.Request, via []*http.Request) error {
    if len(via) >= 10 {
        return errors.New("stopped after 10 redirects")
    }
    if p.Disabled {
        return nil
    }
    host := req.URL.Hostname()
    if p.AllowHostname(host) {
        return nil
    }
    if addr, err := netip.ParseAddr(host); err == nil {
        return p.AllowAddr(addr)
    }
    return nil
}
```

- [ ] **Step 4: Verify**

```bash
go test ./internal/httpx/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/httpx/
git commit -m "$(cat <<'EOF'
M4: httpx.SSRFPolicy.CheckRedirect

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2.5: Per-host limiter `RoundTripper`

**Activate skill before this task:** `golang-concurrency`. The pattern is buffered chan as semaphore with a lazy `map[string]chan struct{}` guarded by a single `sync.Mutex`. Critical invariants: `ctx.Done()` short-circuits the acquire path, slot is released on `Body.Close()` not on `RoundTrip` return, double-close is idempotent (`sync.Once`).

**Files:**
- Create: `internal/httpx/hostlimit.go`
- Create: `internal/httpx/hostlimit_test.go`

- [ ] **Step 1: Write the failing test (basic concurrency)**

Write `internal/httpx/hostlimit_test.go`:

```go
package httpx

import (
    "context"
    "io"
    "net/http"
    "strings"
    "sync"
    "sync/atomic"
    "testing"
    "time"
)

// fakeRT counts concurrent in-flight calls and (optionally) blocks on a chan
// so the test can orchestrate ordering.
type fakeRT struct {
    inflight    atomic.Int32
    maxObserved atomic.Int32
    block       chan struct{} // nil means don't block
}

func (f *fakeRT) RoundTrip(req *http.Request) (*http.Response, error) {
    n := f.inflight.Add(1)
    for {
        m := f.maxObserved.Load()
        if n <= m || f.maxObserved.CompareAndSwap(m, n) {
            break
        }
    }
    defer f.inflight.Add(-1)
    if f.block != nil {
        select {
        case <-f.block:
        case <-req.Context().Done():
            return nil, req.Context().Err()
        }
    }
    return &http.Response{
        StatusCode: 200,
        Body:       io.NopCloser(strings.NewReader("")),
        Request:    req,
    }, nil
}

func mustReq(t *testing.T, ctx context.Context, urlStr string) *http.Request {
    t.Helper()
    req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
    if err != nil {
        t.Fatalf("NewRequest: %v", err)
    }
    return req
}

func TestHostLimiter_SerialisesSameHost(t *testing.T) {
    inner := &fakeRT{block: make(chan struct{})}
    h := newHostLimiter(inner, 1)

    var wg sync.WaitGroup
    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            req := mustReq(t, context.Background(), "http://example.com/")
            resp, err := h.RoundTrip(req)
            if err == nil {
                _ = resp.Body.Close()
            }
        }()
    }

    // Give goroutines time to enqueue; verify only 1 is in flight.
    time.Sleep(20 * time.Millisecond)
    if got := inner.inflight.Load(); got > 1 {
        t.Errorf("inflight = %d; want 1 max with cap=1", got)
    }
    // Drain.
    close(inner.block)
    wg.Wait()
    if got := inner.maxObserved.Load(); got != 1 {
        t.Errorf("maxObserved = %d; want exactly 1", got)
    }
}

func TestHostLimiter_DifferentHostsParallel(t *testing.T) {
    inner := &fakeRT{block: make(chan struct{})}
    h := newHostLimiter(inner, 1)

    var wg sync.WaitGroup
    for _, host := range []string{"a.example", "b.example", "c.example"} {
        wg.Add(1)
        go func(host string) {
            defer wg.Done()
            req := mustReq(t, context.Background(), "http://"+host+"/")
            resp, err := h.RoundTrip(req)
            if err == nil {
                _ = resp.Body.Close()
            }
        }(host)
    }
    time.Sleep(20 * time.Millisecond)
    if got := inner.inflight.Load(); got != 3 {
        t.Errorf("inflight = %d; want 3 (different hosts)", got)
    }
    close(inner.block)
    wg.Wait()
}

func TestHostLimiter_ContextCancelDuringWait(t *testing.T) {
    inner := &fakeRT{block: make(chan struct{})}
    h := newHostLimiter(inner, 1)

    // Hold one slot.
    holder := make(chan struct{})
    go func() {
        req := mustReq(t, context.Background(), "http://example.com/")
        resp, _ := h.RoundTrip(req)
        if resp != nil {
            <-holder
            _ = resp.Body.Close()
        }
    }()
    time.Sleep(20 * time.Millisecond)

    // Second request: cancel its context while it's queued.
    ctx, cancel := context.WithCancel(context.Background())
    done := make(chan error, 1)
    go func() {
        req := mustReq(t, ctx, "http://example.com/")
        _, err := h.RoundTrip(req)
        done <- err
    }()
    time.Sleep(20 * time.Millisecond)
    cancel()
    select {
    case err := <-done:
        if err == nil {
            t.Error("expected ctx error, got nil")
        }
    case <-time.After(time.Second):
        t.Fatal("RoundTrip did not return after ctx cancel")
    }
    close(inner.block)
    close(holder)
}

func TestHostLimiter_ReleaseOnBodyClose(t *testing.T) {
    inner := &fakeRT{} // no blocking — request returns immediately
    h := newHostLimiter(inner, 1)

    // First request: keep body open.
    req1 := mustReq(t, context.Background(), "http://example.com/")
    resp1, err := h.RoundTrip(req1)
    if err != nil {
        t.Fatal(err)
    }

    // Second request to same host: should block on the slot.
    req2 := mustReq(t, context.Background(), "http://example.com/")
    done := make(chan struct{})
    go func() {
        resp2, _ := h.RoundTrip(req2)
        if resp2 != nil {
            _ = resp2.Body.Close()
        }
        close(done)
    }()
    select {
    case <-done:
        t.Fatal("second request returned before first body closed")
    case <-time.After(20 * time.Millisecond):
    }

    // Close the first body — slot frees, second request proceeds.
    _ = resp1.Body.Close()
    select {
    case <-done:
    case <-time.After(time.Second):
        t.Fatal("second request did not proceed after first body closed")
    }
}

func TestHostLimiter_DoubleCloseIdempotent(t *testing.T) {
    inner := &fakeRT{}
    h := newHostLimiter(inner, 1)
    req := mustReq(t, context.Background(), "http://example.com/")
    resp, err := h.RoundTrip(req)
    if err != nil {
        t.Fatal(err)
    }
    _ = resp.Body.Close()
    _ = resp.Body.Close() // must not panic, must not over-release
    // If over-released, we'd have a slot in deficit and the next request
    // would block forever. Verify by issuing one more:
    done := make(chan struct{})
    go func() {
        req2 := mustReq(t, context.Background(), "http://example.com/")
        if r, _ := h.RoundTrip(req2); r != nil {
            _ = r.Body.Close()
        }
        close(done)
    }()
    select {
    case <-done:
    case <-time.After(time.Second):
        t.Fatal("third request blocked — slot was double-released")
    }
}
```

- [ ] **Step 2: Verify it fails**

```bash
go test ./internal/httpx/ -run TestHostLimiter -v -race
```

Expected: FAIL with "newHostLimiter undefined".

- [ ] **Step 3: Implement**

Write `internal/httpx/hostlimit.go`:

```go
package httpx

import (
    "io"
    "net/http"
    "sync"
)

// hostLimiter is a RoundTripper that caps concurrent in-flight requests per
// hostname using a buffered chan as a counting semaphore. The map of chans
// is lazy — a host's chan is allocated on its first request — and guarded
// by a single mutex held only while reading/writing the map (not while
// waiting on a slot). The slot is released on response Body.Close() so
// HTTP/1.1 keep-alive connections aren't reused before the prior caller
// has finished reading.
type hostLimiter struct {
    inner http.RoundTripper
    n     int
    mu    sync.Mutex
    sem   map[string]chan struct{}
}

func newHostLimiter(inner http.RoundTripper, n int) *hostLimiter {
    return &hostLimiter{inner: inner, n: n, sem: map[string]chan struct{}{}}
}

func (h *hostLimiter) acquireChan(host string) chan struct{} {
    h.mu.Lock()
    defer h.mu.Unlock()
    s, ok := h.sem[host]
    if !ok {
        s = make(chan struct{}, h.n)
        h.sem[host] = s
    }
    return s
}

func (h *hostLimiter) RoundTrip(req *http.Request) (*http.Response, error) {
    if h.n <= 0 {
        return h.inner.RoundTrip(req)
    }
    sem := h.acquireChan(req.URL.Hostname())

    select {
    case sem <- struct{}{}:
    case <-req.Context().Done():
        return nil, req.Context().Err()
    }

    resp, err := h.inner.RoundTrip(req)
    if err != nil {
        <-sem // release on error
        return nil, err
    }
    resp.Body = &releasingBody{
        ReadCloser: resp.Body,
        release:    func() { <-sem },
    }
    return resp, nil
}

type releasingBody struct {
    io.ReadCloser
    once    sync.Once
    release func()
}

func (b *releasingBody) Close() error {
    err := b.ReadCloser.Close()
    b.once.Do(b.release)
    return err
}
```

- [ ] **Step 4: Verify with race detector**

```bash
go test ./internal/httpx/ -run TestHostLimiter -v -race
```

Expected: PASS for all 5 hostlimit tests with no race warnings.

- [ ] **Step 5: Commit**

```bash
git add internal/httpx/
git commit -m "$(cat <<'EOF'
M4: httpx per-host limiter with lazy semaphore + release-on-body-close

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 2.6: `NewClient` composition

**Files:**
- Create: `internal/httpx/client.go`
- Create: `internal/httpx/client_test.go`

- [ ] **Step 1: Write the failing test**

Write `internal/httpx/client_test.go`:

```go
package httpx

import (
    "io"
    "net/http"
    "net/http/httptest"
    "net/netip"
    "strings"
    "testing"
    "time"
)

func TestNewClient_RejectsLoopbackByDefault(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _, _ = w.Write([]byte("ok"))
    }))
    defer srv.Close()

    client := NewClient(Opts{
        Timeout: 5 * time.Second,
        SSRF:    SSRFPolicy{}, // default-reject
    })
    _, err := client.Get(srv.URL)
    if err == nil {
        t.Fatal("expected SSRF reject, got nil")
    }
    if !strings.Contains(err.Error(), "ssrf") {
        t.Errorf("expected ssrf error, got %v", err)
    }
}

func TestNewClient_AllowlistAcceptsLoopback(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _, _ = w.Write([]byte("ok"))
    }))
    defer srv.Close()

    client := NewClient(Opts{
        Timeout: 5 * time.Second,
        SSRF: SSRFPolicy{
            AllowCIDRs: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
        },
    })
    resp, err := client.Get(srv.URL)
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    if string(body) != "ok" {
        t.Errorf("body = %q; want ok", body)
    }
}

func TestNewClient_DisabledAcceptsLoopback(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
    defer srv.Close()

    client := NewClient(Opts{
        SSRF: SSRFPolicy{Disabled: true},
    })
    resp, err := client.Get(srv.URL)
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()
}

func TestNewClient_UserAgentApplied(t *testing.T) {
    var sawUA string
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sawUA = r.Header.Get("User-Agent")
    }))
    defer srv.Close()

    client := NewClient(Opts{
        SSRF:      SSRFPolicy{Disabled: true},
        UserAgent: "tap-test/1.0",
    })
    _, err := client.Get(srv.URL)
    if err != nil {
        t.Fatal(err)
    }
    if sawUA != "tap-test/1.0" {
        t.Errorf("server saw UA = %q; want tap-test/1.0", sawUA)
    }
}

func TestNewClient_RedirectReChecks(t *testing.T) {
    // Origin srv2 is allowlisted; srv1 redirects to a literal-loopback URL
    // that's outside the allowlist. The redirect callback rejects.
    blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
    defer blocked.Close()

    srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Redirect to a literal IP outside the allowlist (e.g., 127.0.0.2)
        http.Redirect(w, r, "http://127.0.0.2:1/", http.StatusFound)
    }))
    defer srv1.Close()

    // Allow srv1's host (127.0.0.1) but not 127.0.0.2.
    client := NewClient(Opts{
        SSRF: SSRFPolicy{
            AllowCIDRs: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")},
        },
    })
    _, err := client.Get(srv1.URL)
    if err == nil {
        t.Fatal("expected SSRF reject on redirect, got nil")
    }
}
```

- [ ] **Step 2: Verify it fails**

```bash
go test ./internal/httpx/ -run TestNewClient -v
```

Expected: FAIL with "NewClient undefined" / "Opts undefined".

- [ ] **Step 3: Implement**

Write `internal/httpx/client.go`:

```go
package httpx

import (
    "context"
    "net"
    "net/http"
    "net/netip"
    "syscall"
    "time"
)

// Opts configures the shared HTTP client. Zero values mean "use sensible
// defaults" — Timeout 30s, PerHostInflight 4, no UserAgent header injection.
type Opts struct {
    Timeout         time.Duration
    PerHostInflight int
    SSRF            SSRFPolicy
    UserAgent       string
}

// NewClient builds the shared HTTP client. The composition top-down:
//   - http.Client (with Timeout, CheckRedirect)
//     - hostLimiter RoundTripper (per-host concurrency cap)
//       - uaRoundTripper (User-Agent injection, only if Opts.UserAgent != "")
//         - http.Transport (connection pool + dialer)
//           - net.Dialer with ControlContext (SSRF check post-DNS)
//
// The dialer chooses between strict (with SSRF check) and permissive based
// on the URL hostname so that suffix-allowlisted hosts can resolve to
// otherwise-rejected IPs (e.g. a Tailscale tailnet hostname → 100.x.y.z).
func NewClient(opts Opts) *http.Client {
    if opts.Timeout <= 0 {
        opts.Timeout = 30 * time.Second
    }
    if opts.PerHostInflight <= 0 {
        opts.PerHostInflight = 4
    }

    permissive := &net.Dialer{
        Timeout:   30 * time.Second,
        KeepAlive: 30 * time.Second,
    }
    strict := &net.Dialer{
        Timeout:        30 * time.Second,
        KeepAlive:      30 * time.Second,
        ControlContext: makeSSRFControl(opts.SSRF),
    }

    dial := func(ctx context.Context, network, address string) (net.Conn, error) {
        host, _, err := net.SplitHostPort(address)
        if err != nil {
            return nil, err
        }
        if opts.SSRF.AllowHostname(host) {
            return permissive.DialContext(ctx, network, address)
        }
        return strict.DialContext(ctx, network, address)
    }

    transport := &http.Transport{
        DialContext:         dial,
        MaxIdleConns:        32,
        MaxIdleConnsPerHost: 4,
        IdleConnTimeout:     90 * time.Second,
        TLSHandshakeTimeout: 10 * time.Second,
    }

    var rt http.RoundTripper = transport
    if opts.UserAgent != "" {
        rt = &uaRoundTripper{ua: opts.UserAgent, inner: rt}
    }
    rt = newHostLimiter(rt, opts.PerHostInflight)

    return &http.Client{
        Timeout:       opts.Timeout,
        Transport:     rt,
        CheckRedirect: opts.SSRF.CheckRedirect,
    }
}

type uaRoundTripper struct {
    ua    string
    inner http.RoundTripper
}

func (u *uaRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
    if req.Header.Get("User-Agent") == "" {
        // Don't mutate the caller's request; clone the header.
        cloned := req.Clone(req.Context())
        cloned.Header = req.Header.Clone()
        cloned.Header.Set("User-Agent", u.ua)
        return u.inner.RoundTrip(cloned)
    }
    return u.inner.RoundTrip(req)
}

func makeSSRFControl(policy SSRFPolicy) func(ctx context.Context, network, address string, c syscall.RawConn) error {
    return func(ctx context.Context, network, address string, c syscall.RawConn) error {
        host, _, err := net.SplitHostPort(address)
        if err != nil {
            return err
        }
        addr, err := netip.ParseAddr(host)
        if err != nil {
            return err
        }
        return policy.AllowAddr(addr)
    }
}
```

- [ ] **Step 4: Verify**

```bash
go test ./internal/httpx/... -v -race
```

Expected: PASS for all httpx tests including the new client tests.

- [ ] **Step 5: Commit**

```bash
git add internal/httpx/
git commit -m "$(cat <<'EOF'
M4: httpx.NewClient — compose dialer + limiter + UA + redirect re-check

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 3: DB migration & velocity helpers

**Activate skill before this phase:** `golang-database`. Patterns this phase relies on: `tx.BeginTx`, `defer tx.Rollback` on the err path, parameterised queries with `?` placeholders, `sql.NullString` handling.

### Task 3.1: Migration `0003_polling_discipline.sql`

**Files:**
- Create: `internal/db/migrations/0003_polling_discipline.sql`
- Modify: `internal/db/migrate_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/db/migrate_test.go`:

```go
func TestMigrate_AddsVelocityColumn(t *testing.T) {
    ctx := context.Background()
    d, err := Open(ctx, ":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer d.Close()
    if err := Migrate(ctx, d); err != nil {
        t.Fatalf("Migrate: %v", err)
    }
    var name string
    var defaultVal sql.NullString
    err = d.QueryRowContext(ctx, `
        SELECT name, "dflt_value" FROM pragma_table_info('subscriptions')
        WHERE name = 'velocity_24h_x100'
    `).Scan(&name, &defaultVal)
    if err != nil {
        t.Fatalf("velocity_24h_x100 column not found: %v", err)
    }
    if defaultVal.String != "0" {
        t.Errorf("default = %q; want 0", defaultVal.String)
    }
}
```

You may need to add `database/sql` and `context` imports if not already present.

- [ ] **Step 2: Verify it fails**

```bash
make test 2>&1 | grep -E "(FAIL|PASS).*(Migrate|velocity)"
# or: go test ./internal/db/ -run TestMigrate_AddsVelocityColumn -v
```

Expected: FAIL — column does not exist.

(Note: `go test ./...` requires `web/dist` per the project's `embed.FS`; either run `make test` or `pnpm --dir web build` first. For db-only tests `go test ./internal/db/` works without the SPA build.)

- [ ] **Step 3: Add the migration**

Write `internal/db/migrations/0003_polling_discipline.sql`:

```sql
ALTER TABLE subscriptions ADD COLUMN velocity_24h_x100 INTEGER NOT NULL DEFAULT 0;
```

- [ ] **Step 4: Verify**

```bash
go test ./internal/db/ -run TestMigrate_AddsVelocityColumn -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/
git commit -m "$(cat <<'EOF'
M4: migration 0003 — add velocity_24h_x100 column

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3.2: `db.QueryVelocity` helper

**Files:**
- Modify: `internal/db/subscriptions.go`
- Modify: `internal/db/subscriptions_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/db/subscriptions_test.go`:

```go
func TestQueryVelocity_RollingWindow(t *testing.T) {
    ctx := context.Background()
    d, err := Open(ctx, ":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer d.Close()
    if err := Migrate(ctx, d); err != nil {
        t.Fatal(err)
    }

    now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
    subID, err := InsertSubscription(ctx, d, NewSubscription{
        Title: "Test", FeedURL: "http://x/feed", NextPoll: 0, Created: now.Unix(),
    })
    if err != nil {
        t.Fatal(err)
    }

    // Insert 14 entries within the last 7 days, 1 entry from 8 days ago (out of window).
    for i := 0; i < 14; i++ {
        if _, err := d.ExecContext(ctx, `
            INSERT INTO entries (subscription_id, hash, title, url, content, published_at, fetched_at)
            VALUES (?, ?, '', '', '', ?, ?)
        `, subID, fmt.Sprintf("h%d", i), now.Add(-time.Duration(i)*time.Hour).Unix(), now.Unix()); err != nil {
            t.Fatal(err)
        }
    }
    if _, err := d.ExecContext(ctx, `
        INSERT INTO entries (subscription_id, hash, title, url, content, published_at, fetched_at)
        VALUES (?, 'old', '', '', '', ?, ?)
    `, subID, now.Add(-8*24*time.Hour).Unix(), now.Unix()); err != nil {
        t.Fatal(err)
    }

    velocity, err := QueryVelocity(ctx, d, subID, now)
    if err != nil {
        t.Fatal(err)
    }
    // 14 entries / 7 days * 100 = 200
    if velocity != 200 {
        t.Errorf("velocity = %d; want 200", velocity)
    }
}

func TestQueryVelocity_NoEntries(t *testing.T) {
    ctx := context.Background()
    d, err := Open(ctx, ":memory:")
    if err != nil {
        t.Fatal(err)
    }
    defer d.Close()
    _ = Migrate(ctx, d)

    subID, _ := InsertSubscription(ctx, d, NewSubscription{
        Title: "Empty", FeedURL: "http://e/feed", NextPoll: 0, Created: 0,
    })
    v, err := QueryVelocity(ctx, d, subID, time.Now())
    if err != nil {
        t.Fatal(err)
    }
    if v != 0 {
        t.Errorf("velocity for empty feed = %d; want 0", v)
    }
}
```

(You may need to add `fmt` and `time` imports.)

- [ ] **Step 2: Verify it fails**

```bash
go test ./internal/db/ -run TestQueryVelocity -v
```

Expected: FAIL with "QueryVelocity undefined".

- [ ] **Step 3: Implement**

Append to `internal/db/subscriptions.go`:

```go
import "time"  // add if not already imported

// QueryVelocity returns the rolling 7-day entries-per-day rate × 100 for a
// single subscription. Returns 0 if the subscription has no entries in the
// window. The caller is responsible for calling this inside the same logical
// poll boundary so the count reflects the post-insert state.
func QueryVelocity(ctx context.Context, d *sql.DB, subID int64, now time.Time) (int, error) {
    cutoff := now.Add(-7 * 24 * time.Hour).Unix()
    var velocity int
    err := d.QueryRowContext(ctx, `
        SELECT COUNT(*) * 100 / 7
        FROM entries
        WHERE subscription_id = ? AND published_at >= ?
    `, subID, cutoff).Scan(&velocity)
    if err != nil {
        return 0, fmt.Errorf("query velocity: %w", err)
    }
    return velocity, nil
}
```

- [ ] **Step 4: Verify**

```bash
go test ./internal/db/ -run TestQueryVelocity -v
```

Expected: PASS for both sub-tests.

- [ ] **Step 5: Commit**

```bash
git add internal/db/
git commit -m "$(cat <<'EOF'
M4: db.QueryVelocity rolling-7d entries/day×100

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3.3: `UpdateAfterNotModified` writes velocity

**Files:**
- Modify: `internal/db/subscriptions.go`
- Modify: `internal/db/subscriptions_test.go`

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestUpdateAfterNotModified_WritesVelocity(t *testing.T) {
    ctx := context.Background()
    d, _ := Open(ctx, ":memory:")
    defer d.Close()
    _ = Migrate(ctx, d)

    subID, _ := InsertSubscription(ctx, d, NewSubscription{
        Title: "T", FeedURL: "http://x/", NextPoll: 0, Created: 0,
    })

    if err := UpdateAfterNotModified(ctx, d, subID, 1000, 2000, 350); err != nil {
        t.Fatal(err)
    }

    var velocity int
    var lastPoll, nextPoll int64
    err := d.QueryRowContext(ctx,
        `SELECT velocity_24h_x100, last_poll_at, next_poll_at FROM subscriptions WHERE id = ?`,
        subID).Scan(&velocity, &lastPoll, &nextPoll)
    if err != nil {
        t.Fatal(err)
    }
    if velocity != 350 || lastPoll != 1000 || nextPoll != 2000 {
        t.Errorf("got velocity=%d last=%d next=%d; want 350,1000,2000",
            velocity, lastPoll, nextPoll)
    }
}
```

- [ ] **Step 2: Verify it fails**

```bash
go test ./internal/db/ -run TestUpdateAfterNotModified_WritesVelocity -v
```

Expected: FAIL — current `UpdateAfterNotModified` has 4 params; test passes 5.

- [ ] **Step 3: Update the function signature**

In `internal/db/subscriptions.go`, replace `UpdateAfterNotModified`:

```go
// UpdateAfterNotModified bumps timestamps on a 304 path and writes the
// recomputed velocity. error_count and last_error are reset.
func UpdateAfterNotModified(ctx context.Context, d *sql.DB, subID int64, nowUnix, nextPollAt int64, velocityX100 int) error {
    _, err := d.ExecContext(ctx, `
        UPDATE subscriptions
        SET last_poll_at      = ?,
            next_poll_at      = ?,
            error_count       = 0,
            last_error        = NULL,
            velocity_24h_x100 = ?
        WHERE id = ?
    `, nowUnix, nextPollAt, velocityX100, subID)
    if err != nil {
        return fmt.Errorf("record 304: %w", err)
    }
    return nil
}
```

- [ ] **Step 4: Update existing callers temporarily**

Search for `UpdateAfterNotModified` callers and patch the signature. Currently only `internal/poll/worker.go:69`:

```bash
grep -rn "UpdateAfterNotModified" --include="*.go"
```

In `internal/poll/worker.go`, change the call to pass `0` as a placeholder (Phase 5 replaces this with real velocity logic):

```go
_ = db.UpdateAfterNotModified(ctx, w.db, sub.ID, now, nextPoll, 0)
```

This keeps the build green; Phase 5 wires the real velocity.

- [ ] **Step 5: Verify**

```bash
go test ./... 2>&1 | tail -30
# or: go test ./internal/db/ ./internal/poll/ -v
```

Expected: PASS. (If `go test ./...` complains about web/dist, run `pnpm --dir web build` first or use `make test`.)

- [ ] **Step 6: Commit**

```bash
git add internal/db/ internal/poll/
git commit -m "$(cat <<'EOF'
M4: UpdateAfterNotModified writes velocity_24h_x100

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 3.4: `UpdateAfterPoll` computes velocity + next_poll_at

**Files:**
- Modify: `internal/db/entries.go`
- Modify: `internal/db/entries_test.go`

This is the largest DB change. `PollResult` gains cadence inputs; `UpdateAfterPoll` computes the post-insert velocity and the next-poll time inside the existing transaction.

- [ ] **Step 1: Write the failing test**

Add to `internal/db/entries_test.go` (read the file first to see existing imports/helpers):

```go
import (
    "github.com/bcrisp4/tap/internal/cadence"
)

func TestUpdateAfterPoll_ComputesVelocityAndNextPoll(t *testing.T) {
    ctx := context.Background()
    d, _ := Open(ctx, ":memory:")
    defer d.Close()
    _ = Migrate(ctx, d)

    now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
    subID, _ := InsertSubscription(ctx, d, NewSubscription{
        Title: "Daily", FeedURL: "http://d/", NextPoll: 0, Created: now.Unix(),
    })

    // Insert 14 entries with published_at within the last 7 days.
    var newEntries []NewEntry
    for i := 0; i < 14; i++ {
        newEntries = append(newEntries, NewEntry{
            Hash:        fmt.Sprintf("h%d", i),
            Title:       fmt.Sprintf("e%d", i),
            URL:         "http://x",
            Content:     "",
            PublishedAt: now.Add(-time.Duration(i) * 12 * time.Hour).Unix(),
        })
    }

    inserted, err := UpdateAfterPoll(ctx, d, subID, PollResult{
        NowUnix:     now.Unix(),
        NewEntries:  newEntries,
        Floor:       15 * time.Minute,
        Ceiling:     24 * time.Hour,
        RetryAfter:  time.Time{},
        CacheMaxAge: 0,
    })
    if err != nil {
        t.Fatal(err)
    }
    if inserted != 14 {
        t.Errorf("inserted = %d; want 14", inserted)
    }

    var velocity int
    var nextPoll int64
    _ = d.QueryRowContext(ctx, `
        SELECT velocity_24h_x100, next_poll_at FROM subscriptions WHERE id = ?
    `, subID).Scan(&velocity, &nextPoll)
    // 14 entries / 7 days * 100 = 200 (entries per day × 100 = 2.0)
    if velocity != 200 {
        t.Errorf("velocity = %d; want 200", velocity)
    }
    // 86400/2 = 43200s = 12h
    expected := now.Add(cadence.IntervalFromVelocity(200, 15*time.Minute, 24*time.Hour)).Unix()
    if nextPoll != expected {
        t.Errorf("next_poll = %d; want %d (12h after now)", nextPoll, expected)
    }
}

func TestUpdateAfterPoll_RetryAfterPushesNextPoll(t *testing.T) {
    ctx := context.Background()
    d, _ := Open(ctx, ":memory:")
    defer d.Close()
    _ = Migrate(ctx, d)

    now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
    retryAfter := now.Add(6 * time.Hour)
    subID, _ := InsertSubscription(ctx, d, NewSubscription{
        Title: "Active", FeedURL: "http://a/", NextPoll: 0, Created: now.Unix(),
    })

    // 200 entries/day * 100 = 20000 -> raw interval = 432s = floor 15m.
    var newEntries []NewEntry
    for i := 0; i < 1400; i++ {
        newEntries = append(newEntries, NewEntry{
            Hash:        fmt.Sprintf("h%d", i),
            Title:       "e",
            URL:         "http://x",
            PublishedAt: now.Add(-time.Duration(i) * time.Minute).Unix(),
        })
    }
    _, err := UpdateAfterPoll(ctx, d, subID, PollResult{
        NowUnix:    now.Unix(),
        NewEntries: newEntries,
        Floor:      15 * time.Minute,
        Ceiling:    24 * time.Hour,
        RetryAfter: retryAfter,
    })
    if err != nil {
        t.Fatal(err)
    }
    var nextPoll int64
    _ = d.QueryRowContext(ctx, `SELECT next_poll_at FROM subscriptions WHERE id = ?`, subID).Scan(&nextPoll)
    if nextPoll != retryAfter.Unix() {
        t.Errorf("next_poll = %d; want %d (retry-after wins over 15m floor)", nextPoll, retryAfter.Unix())
    }
}
```

- [ ] **Step 2: Verify it fails**

```bash
go test ./internal/db/ -run TestUpdateAfterPoll -v
```

Expected: FAIL — `PollResult` doesn't have `Floor`, `Ceiling`, `RetryAfter`, `CacheMaxAge` fields; `NextPollAt` removed.

- [ ] **Step 3: Update `PollResult` and `UpdateAfterPoll`**

In `internal/db/entries.go`, replace `PollResult`:

```go
import (
    // existing imports
    "time"
    "github.com/bcrisp4/tap/internal/cadence"
)

// PollResult is the success result of a feed fetch+parse worth committing.
// The cadence inputs let UpdateAfterPoll compute next_poll_at + velocity in
// one transaction so the post-insert count flows directly into the schedule.
type PollResult struct {
    NewETag         sql.NullString
    NewLastModified sql.NullString
    NowUnix         int64
    NewEntries      []NewEntry
    Floor           time.Duration
    Ceiling         time.Duration
    RetryAfter      time.Time     // zero -> no server floor
    CacheMaxAge     time.Duration // 0 -> no server floor
}
```

Replace `UpdateAfterPoll`:

```go
// UpdateAfterPoll commits the success result of a poll in one transaction:
// insert entries (silently dropping duplicates on the (subscription_id, hash)
// unique constraint), recompute velocity from the post-insert state, derive
// the next-poll time via the cadence formula with server-mandated floors, and
// update the subscription row.
func UpdateAfterPoll(ctx context.Context, d *sql.DB, subID int64, r PollResult) (insertedCount int, err error) {
    tx, err := d.BeginTx(ctx, nil)
    if err != nil {
        return 0, fmt.Errorf("begin tx: %w", err)
    }
    defer func() {
        if err != nil {
            _ = tx.Rollback()
        }
    }()

    for _, e := range r.NewEntries {
        res, ierr := tx.ExecContext(ctx, `
            INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at)
            VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?)
            ON CONFLICT (subscription_id, hash) DO NOTHING
        `, subID, e.Hash, e.Title, e.Author, e.URL, e.Content, e.PublishedAt, r.NowUnix)
        if ierr != nil {
            err = fmt.Errorf("insert entry: %w", ierr)
            return 0, err
        }
        n, _ := res.RowsAffected()
        insertedCount += int(n)
    }

    // Compute velocity from the live (post-insert) state.
    cutoff := r.NowUnix - 7*24*60*60
    var velocity int
    if err = tx.QueryRowContext(ctx, `
        SELECT COUNT(*) * 100 / 7 FROM entries
        WHERE subscription_id = ? AND published_at >= ?
    `, subID, cutoff).Scan(&velocity); err != nil {
        err = fmt.Errorf("compute velocity: %w", err)
        return 0, err
    }

    // Derive next-poll time.
    now := time.Unix(r.NowUnix, 0).UTC()
    interval := cadence.IntervalFromVelocity(velocity, r.Floor, r.Ceiling)
    nextPoll := cadence.ApplyServerFloors(now.Add(interval), r.RetryAfter, r.CacheMaxAge, now)

    if _, err = tx.ExecContext(ctx, `
        UPDATE subscriptions
        SET etag              = ?,
            last_modified     = ?,
            last_poll_at      = ?,
            next_poll_at      = ?,
            error_count       = 0,
            last_error        = NULL,
            velocity_24h_x100 = ?
        WHERE id = ?
    `, r.NewETag, r.NewLastModified, r.NowUnix, nextPoll.Unix(), velocity, subID); err != nil {
        err = fmt.Errorf("update subscription: %w", err)
        return 0, err
    }

    if err = tx.Commit(); err != nil {
        err = fmt.Errorf("commit: %w", err)
        return 0, err
    }
    return insertedCount, nil
}
```

- [ ] **Step 4: Update existing callers**

```bash
grep -rn "UpdateAfterPoll\|NextPollAt:" --include="*.go" | grep -v _test.go
```

In `internal/poll/worker.go:94`, the existing call uses `db.PollResult{NewETag:..., NewLastModified:..., NextPollAt:nextPoll, NowUnix:now, NewEntries:newEntries}`. Update to set the cadence fields and drop `NextPollAt`:

```go
inserted, err := db.UpdateAfterPoll(ctx, w.db, sub.ID, db.PollResult{
    NewETag:         nullStr(res.ETag),
    NewLastModified: nullStr(res.LastModified),
    NowUnix:         now,
    NewEntries:      newEntries,
    Floor:           w.opts.Floor,
    Ceiling:         w.opts.Ceiling,
    // RetryAfter and CacheMaxAge wired in Phase 5
})
```

`WorkerOpts` doesn't have `Floor`/`Ceiling` yet — Phase 5 adds them. To keep the build green now, temporarily add zero-value defaults at call site and update WorkerOpts in Task 5.1:

For now, leave `Floor` and `Ceiling` unset; they'll arrive via WorkerOpts changes in Task 5.1. Use placeholder constants:

```go
inserted, err := db.UpdateAfterPoll(ctx, w.db, sub.ID, db.PollResult{
    NewETag:         nullStr(res.ETag),
    NewLastModified: nullStr(res.LastModified),
    NowUnix:         now,
    NewEntries:      newEntries,
    Floor:           15 * time.Minute,
    Ceiling:         24 * time.Hour,
})
```

This keeps the build green until Task 5.1 plumbs the real values.

- [ ] **Step 5: Verify**

```bash
go test ./internal/db/ ./internal/poll/ -v
```

Expected: PASS for the two new `UpdateAfterPoll` tests and all existing poll tests (they may need adjusting because they observe `next_poll_at` — fix any breakage by updating the assertions to match the new cadence-derived value).

If existing `worker_test.go` tests assert `next_poll_at = now + 30*time.Minute` (M3-era), update them to assert against `cadence.IntervalFromVelocity(velocity, 15m, 24h)` — typically `24h` for fresh fixture feeds with 0 velocity. Mark such updates as "Phase 3 follow-on" in the commit.

- [ ] **Step 6: Commit**

```bash
git add internal/db/ internal/poll/
git commit -m "$(cat <<'EOF'
M4: UpdateAfterPoll computes velocity + next_poll_at in tx

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 4: feed package updates

### Task 4.1: Drop `UserAgent` from `FetchOpts`

**Files:**
- Modify: `internal/feed/parse.go`
- Modify: `internal/feed/parse_test.go`

The shared client owns User-Agent now; `feed.Fetch` stops setting it.

- [ ] **Step 1: Write the failing test**

Add to `internal/feed/parse_test.go`:

```go
func TestFetch_DoesNotSetUserAgent(t *testing.T) {
    var sawUA string
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sawUA = r.Header.Get("User-Agent")
        _, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title></channel></rss>`))
    }))
    defer srv.Close()

    _, err := Fetch(context.Background(), srv.Client(), srv.URL, FetchOpts{})
    if err != nil {
        t.Fatal(err)
    }
    // The default httptest client doesn't set User-Agent unless Fetch does.
    // After M4, Fetch shouldn't set it (centrally set on the shared client).
    // Go's default UA "Go-http-client/1.1" is OK; what we want to verify is
    // that *our* "tap/0.1" string isn't on the wire.
    if strings.Contains(sawUA, "tap/") {
        t.Errorf("Fetch should not set tap-specific UA; got %q", sawUA)
    }
}
```

(Add imports: `context`, `net/http`, `net/http/httptest`, `strings` if missing.)

- [ ] **Step 2: Verify it fails**

```bash
go test ./internal/feed/ -run TestFetch_DoesNotSetUserAgent -v
```

Expected: FAIL — Fetch sets User-Agent today.

- [ ] **Step 3: Update Fetch**

In `internal/feed/parse.go`:

- Remove `UserAgent string` from `FetchOpts`.
- Remove the `User-Agent` header set lines (`req.Header.Set("User-Agent", ua)` and the surrounding `ua := opts.UserAgent` / `if ua == "" { ua = "tap/0.1..." }` block).

Before:

```go
type FetchOpts struct {
    PriorETag         string
    PriorLastModified string
    UserAgent         string
}
// ...
ua := opts.UserAgent
if ua == "" {
    ua = "tap/0.1 (+https://github.com/bcrisp4/tap)"
}
req.Header.Set("User-Agent", ua)
```

After:

```go
type FetchOpts struct {
    PriorETag         string
    PriorLastModified string
}
// (UA lines removed)
```

- [ ] **Step 4: Verify**

```bash
go test ./internal/feed/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/feed/
git commit -m "$(cat <<'EOF'
M4: feed.Fetch no longer sets User-Agent (httpx owns it)

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 4.2: Populate `FetchResult` on error + parse server headers

**Files:**
- Modify: `internal/feed/parse.go`
- Modify: `internal/feed/parse_test.go`

`FetchResult` gains `RetryAfter time.Time` and `CacheMaxAge time.Duration`. `Fetch` populates these even when returning an error so the worker can floor `next_poll_at` against `Retry-After` on a 503.

- [ ] **Step 1: Write the failing test**

Append to `internal/feed/parse_test.go`:

```go
func TestFetch_PopulatesRetryAfterOnError(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Retry-After", "120")
        w.WriteHeader(http.StatusServiceUnavailable)
    }))
    defer srv.Close()

    res, err := Fetch(context.Background(), srv.Client(), srv.URL, FetchOpts{})
    if err == nil {
        t.Fatal("expected error on 503")
    }
    if res.RetryAfter.IsZero() {
        t.Error("expected RetryAfter populated even on error")
    }
}

func TestFetch_PopulatesCacheMaxAgeOnSuccess(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Cache-Control", "public, max-age=3600")
        _, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title></channel></rss>`))
    }))
    defer srv.Close()

    res, err := Fetch(context.Background(), srv.Client(), srv.URL, FetchOpts{})
    if err != nil {
        t.Fatal(err)
    }
    if res.CacheMaxAge != 3600*time.Second {
        t.Errorf("CacheMaxAge = %v; want 3600s", res.CacheMaxAge)
    }
}
```

- [ ] **Step 2: Verify it fails**

```bash
go test ./internal/feed/ -run TestFetch_Populates -v
```

Expected: FAIL — fields don't exist.

- [ ] **Step 3: Update `FetchResult` and `Fetch`**

In `internal/feed/parse.go`, update `FetchResult`:

```go
import (
    // existing imports
    "time"
    "github.com/bcrisp4/tap/internal/cadence"
)

type FetchResult struct {
    Status       int
    ETag         string
    LastModified string
    RetryAfter   time.Time     // zero if absent
    CacheMaxAge  time.Duration // 0 if absent
    Feed         *gofeed.Feed  // nil on 304 or error
}
```

Update `Fetch` to populate the new fields and return `FetchResult` on error too:

```go
func Fetch(ctx context.Context, client *http.Client, feedURL string, opts FetchOpts) (FetchResult, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
    if err != nil {
        return FetchResult{}, fmt.Errorf("new request: %w", err)
    }
    if opts.PriorETag != "" {
        req.Header.Set("If-None-Match", opts.PriorETag)
    }
    if opts.PriorLastModified != "" {
        req.Header.Set("If-Modified-Since", opts.PriorLastModified)
    }
    req.Header.Set("Accept", "application/atom+xml, application/rss+xml, application/json, application/xml;q=0.9, */*;q=0.5")

    resp, err := client.Do(req)
    if err != nil {
        return FetchResult{}, fmt.Errorf("http: %w", err)
    }
    defer resp.Body.Close()

    res := FetchResult{
        Status:       resp.StatusCode,
        ETag:         resp.Header.Get("ETag"),
        LastModified: resp.Header.Get("Last-Modified"),
    }
    if t, ok := cadence.ParseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
        res.RetryAfter = t
    }
    if d, ok := cadence.ParseCacheMaxAge(resp.Header.Get("Cache-Control")); ok {
        res.CacheMaxAge = d
    }

    switch {
    case resp.StatusCode == http.StatusNotModified:
        return res, nil
    case resp.StatusCode >= 200 && resp.StatusCode < 300:
        const maxBody = 10 << 20
        f, err := gofeed.NewParser().Parse(io.LimitReader(resp.Body, maxBody))
        if err != nil {
            return res, fmt.Errorf("parse feed: %w", err)
        }
        res.Feed = f
        return res, nil
    default:
        return res, fmt.Errorf("unexpected status %d", resp.StatusCode)
    }
}
```

The key change: error returns now use `res` (populated) rather than `FetchResult{}`.

- [ ] **Step 4: Verify**

```bash
go test ./internal/feed/ -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/feed/
git commit -m "$(cat <<'EOF'
M4: feed.Fetch parses Retry-After / Cache-Control; populates result on error

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 5: poll package wiring

**Activate skill before this phase:** `golang-error-handling`. The worker's branches return errors only when fatal (DB-level); poll-level failures (network, parse) are recorded as `last_error` rather than propagating up.

### Task 5.1: Extend `WorkerOpts` and `SchedulerOpts`

**Files:**
- Modify: `internal/poll/scheduler.go`
- Modify: `internal/poll/worker.go`
- Modify: `internal/poll/scheduler_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/poll/scheduler_test.go`:

```go
import (
    "math/rand/v2"
)

func TestSchedulerOpts_DefaultsApplied(t *testing.T) {
    ctx := context.Background()
    d, _ := db.Open(ctx, ":memory:")
    defer d.Close()
    _ = db.Migrate(ctx, d)

    s := NewScheduler(ctx, d, http.DefaultClient, SchedulerOpts{})
    if s.opts.Floor == 0 {
        t.Error("Floor should default to 15m")
    }
    if s.opts.Ceiling == 0 {
        t.Error("Ceiling should default to 24h")
    }
    if s.opts.ErrorBase == 0 {
        t.Error("ErrorBase should default to 5m")
    }
    if s.opts.Now == nil {
        t.Error("Now should default to time.Now")
    }
    if s.opts.Rand == nil {
        t.Error("Rand should default to a fresh rand.Rand")
    }
}
```

(Imports may need adjusting; check existing scheduler_test.go for context.)

- [ ] **Step 2: Verify it fails**

```bash
go test ./internal/poll/ -run TestSchedulerOpts_DefaultsApplied -v
```

Expected: FAIL — fields don't exist.

- [ ] **Step 3: Extend `SchedulerOpts` and `WorkerOpts`**

In `internal/poll/scheduler.go`:

```go
import (
    "math/rand/v2"
    // existing imports
)

type SchedulerOpts struct {
    TickInterval time.Duration
    Workers      int
    Cadence      time.Duration // legacy; M4 supersedes with Floor/Ceiling
    Processor    *processor.Processor
    Floor        time.Duration   // adaptive cadence floor; default 15m
    Ceiling      time.Duration   // adaptive cadence ceiling; default 24h
    ErrorBase    time.Duration   // base of exponential error backoff; default 5m
    Now          func() time.Time
    Rand         *rand.Rand
}
```

In `NewScheduler`, after the existing default-application:

```go
if o.Floor <= 0 {
    o.Floor = 15 * time.Minute
}
if o.Ceiling <= 0 {
    o.Ceiling = 24 * time.Hour
}
if o.ErrorBase <= 0 {
    o.ErrorBase = 5 * time.Minute
}
if o.Now == nil {
    o.Now = time.Now
}
if o.Rand == nil {
    var seed [32]byte
    binary.LittleEndian.PutUint64(seed[:8], uint64(time.Now().UnixNano()))
    o.Rand = rand.New(rand.NewChaCha8(seed))
}
```

(Add `encoding/binary` to imports.)

Pass them through to the worker in the `NewWorker` call:

```go
worker: NewWorker(d, c, WorkerOpts{
    Cadence:   o.Cadence,
    Processor: o.Processor,
    Floor:     o.Floor,
    Ceiling:   o.Ceiling,
    ErrorBase: o.ErrorBase,
    Now:       o.Now,
    Rand:      o.Rand,
}),
```

In `internal/poll/worker.go`, extend `WorkerOpts`:

```go
type WorkerOpts struct {
    Cadence   time.Duration // legacy; ignored once Floor/Ceiling set
    Processor *processor.Processor
    Floor     time.Duration
    Ceiling   time.Duration
    ErrorBase time.Duration
    Now       func() time.Time
    Rand      *rand.Rand
}
```

In `NewWorker`, apply the same defaults as the scheduler does (so direct callers — tests — don't need to populate everything):

```go
if o.Floor <= 0 {
    o.Floor = 15 * time.Minute
}
if o.Ceiling <= 0 {
    o.Ceiling = 24 * time.Hour
}
if o.ErrorBase <= 0 {
    o.ErrorBase = 5 * time.Minute
}
if o.Now == nil {
    o.Now = time.Now
}
if o.Rand == nil {
    var seed [32]byte
    binary.LittleEndian.PutUint64(seed[:8], uint64(time.Now().UnixNano()))
    o.Rand = rand.New(rand.NewChaCha8(seed))
}
```

- [ ] **Step 4: Verify**

```bash
go test ./internal/poll/ -run TestSchedulerOpts_DefaultsApplied -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/poll/
git commit -m "$(cat <<'EOF'
M4: WorkerOpts/SchedulerOpts gain Floor/Ceiling/ErrorBase/Now/Rand

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5.2: Worker error-path uses exponential backoff

**Files:**
- Modify: `internal/poll/worker.go`
- Modify: `internal/poll/worker_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/poll/worker_test.go`:

```go
import (
    "math/rand/v2"
    "github.com/bcrisp4/tap/internal/cadence"
)

func TestWorker_ErrorPath_ExponentialBackoff(t *testing.T) {
    ctx := context.Background()
    d, _ := db.Open(ctx, ":memory:")
    defer d.Close()
    _ = db.Migrate(ctx, d)

    // Origin always 503 with no Retry-After.
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusServiceUnavailable)
    }))
    defer srv.Close()

    fixedNow := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
    seed := [32]byte{} // deterministic
    rng := rand.New(rand.NewChaCha8(seed))
    w := NewWorker(d, srv.Client(), WorkerOpts{
        Processor: processor.New(sanitise.DefaultPolicy(), nil),
        Floor:     15 * time.Minute,
        Ceiling:   24 * time.Hour,
        ErrorBase: 5 * time.Minute,
        Now:       func() time.Time { return fixedNow },
        Rand:      rng,
    })
    subID, _ := db.InsertSubscription(ctx, d, db.NewSubscription{
        Title: "Bad", FeedURL: srv.URL, NextPoll: 0, Created: fixedNow.Unix(),
    })
    sub := db.DueSubscription{ID: subID, FeedURL: srv.URL}

    // First poll: error_count goes from 0 -> 1, delay >= 5m.
    w.Run(ctx, sub)
    var ec int
    var nextPoll int64
    _ = d.QueryRowContext(ctx, `SELECT error_count, next_poll_at FROM subscriptions WHERE id = ?`, subID).
        Scan(&ec, &nextPoll)
    if ec != 1 {
        t.Errorf("error_count = %d; want 1", ec)
    }
    earliest := fixedNow.Add(5 * time.Minute).Unix()
    latest := fixedNow.Add(5*time.Minute + 5*time.Minute/4).Unix()
    if nextPoll < earliest || nextPoll >= latest {
        t.Errorf("next_poll = %d; want in [%d, %d) (5m + jitter)", nextPoll, earliest, latest)
    }
}

func TestWorker_RetryAfterOverridesBackoff(t *testing.T) {
    ctx := context.Background()
    d, _ := db.Open(ctx, ":memory:")
    defer d.Close()
    _ = db.Migrate(ctx, d)

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Retry-After", "3600")
        w.WriteHeader(http.StatusServiceUnavailable)
    }))
    defer srv.Close()

    fixedNow := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
    seed := [32]byte{}
    rng := rand.New(rand.NewChaCha8(seed))
    w := NewWorker(d, srv.Client(), WorkerOpts{
        Processor: processor.New(sanitise.DefaultPolicy(), nil),
        Floor:     15 * time.Minute,
        Ceiling:   24 * time.Hour,
        ErrorBase: 5 * time.Minute,
        Now:       func() time.Time { return fixedNow },
        Rand:      rng,
    })
    subID, _ := db.InsertSubscription(ctx, d, db.NewSubscription{
        Title: "Slow", FeedURL: srv.URL, NextPoll: 0, Created: fixedNow.Unix(),
    })
    sub := db.DueSubscription{ID: subID, FeedURL: srv.URL}

    w.Run(ctx, sub)
    var nextPoll int64
    _ = d.QueryRowContext(ctx, `SELECT next_poll_at FROM subscriptions WHERE id = ?`, subID).Scan(&nextPoll)
    expected := fixedNow.Add(time.Hour).Unix()
    if nextPoll < expected {
        t.Errorf("next_poll = %d; want >= %d (Retry-After:3600)", nextPoll, expected)
    }
}
```

- [ ] **Step 2: Verify it fails**

```bash
go test ./internal/poll/ -run TestWorker_ErrorPath -v
```

Expected: FAIL — current worker uses fixed Cadence on errors.

- [ ] **Step 3: Update `Worker.Run` error branch**

In `internal/poll/worker.go`, replace the error-path block. Read the current file first; find the section like:

```go
if err != nil {
    slog.WarnContext(ctx, "poll error", ...)
    _ = db.UpdateAfterError(ctx, w.db, sub.ID, err.Error(), nextPoll)
    return
}
```

Replace with:

```go
if fetchErr != nil {
    delay := cadence.BackoffFromErrorCount(sub.ErrorCount+1, w.opts.ErrorBase, w.opts.Ceiling, 0.25, w.opts.Rand)
    next := now.Add(delay)
    if !res.RetryAfter.IsZero() && res.RetryAfter.After(next) {
        next = res.RetryAfter
    }
    slog.WarnContext(ctx, "poll error",
        "feed_id", sub.ID, "feed_url", sub.FeedURL,
        "error_count", sub.ErrorCount+1,
        "next_poll_at", next.Unix(),
        "err", fetchErr)
    _ = db.UpdateAfterError(ctx, w.db, sub.ID, fetchErr.Error(), next.Unix())
    return
}
```

This requires `db.DueSubscription` to expose `ErrorCount`. Check `internal/db/subscriptions.go` — currently `DueSubscription` has only `ID, FeedURL, ETag, LastModified`. Extend it:

```go
type DueSubscription struct {
    ID           int64
    FeedURL      string
    ETag         sql.NullString
    LastModified sql.NullString
    ErrorCount   int
}
```

And update `ListDuePolls` SELECT to include `error_count`:

```go
rows, err := d.QueryContext(ctx, `
    SELECT id, feed_url, etag, last_modified, error_count
    FROM subscriptions
    WHERE next_poll_at <= ?
    ORDER BY next_poll_at
    LIMIT ?
`, now, limit)
// ...
err := rows.Scan(&s.ID, &s.FeedURL, &s.ETag, &s.LastModified, &s.ErrorCount)
```

Also rename the existing local `err` to `fetchErr` and `nextPoll` accordingly. Update the success-path call to compute `now` once at the top of `Run` from `w.opts.Now()`:

```go
func (w *Worker) Run(ctx context.Context, sub db.DueSubscription) {
    defer func() { /* unchanged panic recovery */ }()

    now := w.opts.Now()

    res, fetchErr := feed.Fetch(ctx, w.client, sub.FeedURL, feed.FetchOpts{
        PriorETag:         sub.ETag.String,
        PriorLastModified: sub.LastModified.String,
    })
    // ... error branch above ...
    // ... 304 branch (Task 5.3) ...
    // ... success branch (Task 5.4) ...
}
```

- [ ] **Step 4: Verify**

```bash
go test ./internal/poll/ -run TestWorker -v
```

Expected: PASS for the new error-path tests. Existing 304/success tests may fail — they'll be repaired in Tasks 5.3 / 5.4.

- [ ] **Step 5: Commit**

```bash
git add internal/poll/ internal/db/
git commit -m "$(cat <<'EOF'
M4: worker error path uses exponential backoff + Retry-After

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5.3: Worker 304-path uses adaptive cadence

**Files:**
- Modify: `internal/poll/worker.go`
- Modify: `internal/poll/worker_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/poll/worker_test.go`:

```go
func TestWorker_NotModified_RecomputesVelocityAndCadence(t *testing.T) {
    ctx := context.Background()
    d, _ := db.Open(ctx, ":memory:")
    defer d.Close()
    _ = db.Migrate(ctx, d)

    fixedNow := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

    // Insert sub with prior etag, plus 14 entries in last 7 days (velocity=200).
    subID, _ := db.InsertSubscription(ctx, d, db.NewSubscription{
        Title: "Active", FeedURL: "", NextPoll: 0, Created: fixedNow.Unix(),
    })
    _, _ = d.ExecContext(ctx, `UPDATE subscriptions SET etag='abc' WHERE id=?`, subID)
    for i := 0; i < 14; i++ {
        _, _ = d.ExecContext(ctx, `INSERT INTO entries
            (subscription_id, hash, title, url, content, published_at, fetched_at)
            VALUES (?, ?, '', '', '', ?, ?)`,
            subID, fmt.Sprintf("h%d", i),
            fixedNow.Add(-time.Duration(i)*12*time.Hour).Unix(), fixedNow.Unix())
    }

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("If-None-Match") == "abc" {
            w.WriteHeader(http.StatusNotModified)
            return
        }
        t.Errorf("expected If-None-Match header")
    }))
    defer srv.Close()
    _, _ = d.ExecContext(ctx, `UPDATE subscriptions SET feed_url=? WHERE id=?`, srv.URL, subID)

    w := NewWorker(d, srv.Client(), WorkerOpts{
        Processor: processor.New(sanitise.DefaultPolicy(), nil),
        Floor:     15 * time.Minute,
        Ceiling:   24 * time.Hour,
        Now:       func() time.Time { return fixedNow },
    })
    sub := db.DueSubscription{
        ID: subID, FeedURL: srv.URL,
        ETag: sql.NullString{String: "abc", Valid: true},
    }
    w.Run(ctx, sub)

    var velocity int
    var nextPoll int64
    _ = d.QueryRowContext(ctx, `SELECT velocity_24h_x100, next_poll_at FROM subscriptions WHERE id = ?`,
        subID).Scan(&velocity, &nextPoll)
    if velocity != 200 {
        t.Errorf("velocity = %d; want 200", velocity)
    }
    expected := fixedNow.Add(12 * time.Hour).Unix()
    if nextPoll != expected {
        t.Errorf("next_poll = %d; want %d (12h)", nextPoll, expected)
    }
}
```

- [ ] **Step 2: Verify it fails**

```bash
go test ./internal/poll/ -run TestWorker_NotModified -v
```

Expected: FAIL — current 304 path uses fixed Cadence.

- [ ] **Step 3: Update the 304 branch**

In `internal/poll/worker.go`, replace the 304-path block:

```go
if res.Status == http.StatusNotModified {
    velocity, verr := db.QueryVelocity(ctx, w.db, sub.ID, now)
    if verr != nil {
        slog.ErrorContext(ctx, "query velocity", "feed_id", sub.ID, "err", verr)
        return
    }
    interval := cadence.IntervalFromVelocity(velocity, w.opts.Floor, w.opts.Ceiling)
    next := cadence.ApplyServerFloors(now.Add(interval), res.RetryAfter, res.CacheMaxAge, now)
    slog.DebugContext(ctx, "poll 304",
        "feed_id", sub.ID, "velocity", velocity,
        "next_poll_at", next.Unix())
    _ = db.UpdateAfterNotModified(ctx, w.db, sub.ID, now.Unix(), next.Unix(), velocity)
    return
}
```

(Imports may need `github.com/bcrisp4/tap/internal/cadence`.)

- [ ] **Step 4: Verify**

```bash
go test ./internal/poll/ -run TestWorker_NotModified -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/poll/
git commit -m "$(cat <<'EOF'
M4: worker 304 path uses adaptive cadence + server floors

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 5.4: Worker success-path passes cadence inputs to `UpdateAfterPoll`

**Files:**
- Modify: `internal/poll/worker.go`
- Modify: `internal/poll/worker_test.go`

- [ ] **Step 1: Write the failing test**

Append:

```go
func TestWorker_Success_RetryAfterAdvisoryFloor(t *testing.T) {
    ctx := context.Background()
    d, _ := db.Open(ctx, ":memory:")
    defer d.Close()
    _ = db.Migrate(ctx, d)

    fixedNow := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Retry-After", "3600")
        _, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title>
            <item><title>e1</title><link>http://x/1</link><pubDate>Sat, 09 May 2026 11:00:00 GMT</pubDate></item>
        </channel></rss>`))
    }))
    defer srv.Close()

    subID, _ := db.InsertSubscription(ctx, d, db.NewSubscription{
        Title: "X", FeedURL: srv.URL, NextPoll: 0, Created: fixedNow.Unix(),
    })

    w := NewWorker(d, srv.Client(), WorkerOpts{
        Processor: processor.New(sanitise.DefaultPolicy(), nil),
        Floor:     15 * time.Minute,
        Ceiling:   24 * time.Hour,
        Now:       func() time.Time { return fixedNow },
    })
    w.Run(ctx, db.DueSubscription{ID: subID, FeedURL: srv.URL})

    var nextPoll int64
    _ = d.QueryRowContext(ctx, `SELECT next_poll_at FROM subscriptions WHERE id = ?`, subID).
        Scan(&nextPoll)
    expected := fixedNow.Add(time.Hour).Unix()
    if nextPoll < expected {
        t.Errorf("next_poll = %d; want >= %d (Retry-After:3600)", nextPoll, expected)
    }
}
```

- [ ] **Step 2: Verify it fails**

```bash
go test ./internal/poll/ -run TestWorker_Success_RetryAfter -v
```

Expected: FAIL — success path doesn't propagate `RetryAfter` yet.

- [ ] **Step 3: Update the success branch**

In `internal/poll/worker.go`, replace the success-path call to `db.UpdateAfterPoll`:

```go
inserted, err := db.UpdateAfterPoll(ctx, w.db, sub.ID, db.PollResult{
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

Also delete the no-longer-used local `nextPoll := time.Now().Add(w.opts.Cadence).Unix()` and the assignment of `now` from `time.Now().Unix()` (now `now` is `time.Time` from `w.opts.Now()`).

- [ ] **Step 4: Verify**

```bash
go test ./internal/poll/... -v -race
```

Expected: PASS for all worker / scheduler tests. Repair any remaining tests that observe `next_poll_at` against M3-era expectations.

- [ ] **Step 5: Commit**

```bash
git add internal/poll/
git commit -m "$(cat <<'EOF'
M4: worker success path threads cadence inputs into UpdateAfterPoll

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 6: cmd/tap/main.go wiring + end-to-end tests

**Activate skill before this phase:** `golang-cli`. Patterns: stdlib `flag.Var()` for repeatable flags, `flag.String()` deprecation pattern, `os.Getenv` for env defaults.

### Task 6.1: New flags + repeatable `--ssrf-allow`

**Files:**
- Modify: `cmd/tap/main.go`

- [ ] **Step 1: Add a `stringSlice` flag.Value type and the new flag declarations**

Append to `cmd/tap/main.go` near the `envOr*` helpers:

```go
// stringSlice implements flag.Value for repeatable string flags.
// Env-var form is comma-separated; flag form is repeatable.
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
    *s = append(*s, v)
    return nil
}
```

(Add `strings` to imports if not already present.)

In `main()`, replace the existing flag block to add the new flags. The flag block becomes:

```go
var (
    addr           = flag.String("addr", "127.0.0.1:8080", "HTTP listen address (set 0.0.0.0:8080 in containers)")
    dataDir        = flag.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory containing tap.db")
    logFmt         = flag.String("log-format", "json", "log format: json or text")

    httpTimeout    = flag.Duration("http-timeout", envOrDuration("TAP_HTTP_TIMEOUT", 30*time.Second), "total per-request HTTP deadline")
    perHostInfl    = flag.Int("per-host-inflight", envOrInt("TAP_PER_HOST_INFLIGHT", 4), "concurrent outbound HTTP requests per hostname")
    ssrfDisabled   = flag.Bool("ssrf-disabled", envOrBool("TAP_SSRF_DISABLED", false), "disable the SSRF guard (use only on fully trusted networks)")
    ssrfAllow      stringSlice
    pollFloor      = flag.Duration("poll-floor", envOrDuration("TAP_POLL_FLOOR", 15*time.Minute), "adaptive cadence floor")
    pollCeiling    = flag.Duration("poll-ceiling", envOrDuration("TAP_POLL_CEILING", 24*time.Hour), "adaptive cadence ceiling and error backoff cap")
    pollErrorBase  = flag.Duration("poll-error-base", envOrDuration("TAP_POLL_ERROR_BASE", 5*time.Minute), "base of exponential error backoff")
    userAgent      = flag.String("user-agent", envOr("TAP_USER_AGENT", "tap/0.1 (+https://github.com/bcrisp4/tap)"), "User-Agent header on outbound HTTP")

    proxyCacheDir  = flag.String("proxy-cache-dir", envOr("TAP_PROXY_CACHE_DIR", ""), "media cache directory (default: <data>/cache)")
    proxyCacheCap  = flag.Int64("proxy-cache-cap-bytes", envOrInt64("TAP_PROXY_CACHE_CAP_BYTES", 524288000), "media cache size cap in bytes")
    proxyFetchTO   = flag.Duration("proxy-fetch-timeout", 0, "DEPRECATED: alias for --http-timeout")
    proxyBodyCap   = flag.Int64("proxy-body-cap-bytes", envOrInt64("TAP_PROXY_BODY_CAP_BYTES", 10485760), "per-response body cap for media proxy origin fetches")
)
flag.Var(&ssrfAllow, "ssrf-allow", "SSRF allowlist entry (CIDR, IP literal, or hostname suffix). Repeatable; env TAP_SSRF_ALLOW is comma-separated.")

// Hydrate ssrfAllow from env if not set via flag.
if len(ssrfAllow) == 0 {
    if env := os.Getenv("TAP_SSRF_ALLOW"); env != "" {
        for _, p := range strings.Split(env, ",") {
            ssrfAllow = append(ssrfAllow, p)
        }
    }
}

flag.Parse()
```

Add helper functions for the new env-var types if not already present:

```go
func envOrInt(k string, def int) int {
    if v := os.Getenv(k); v != "" {
        if n, err := strconv.Atoi(v); err == nil {
            return n
        }
        fmt.Fprintf(os.Stderr, "warning: %s=%q is not a valid int; using default %d\n", k, v, def)
    }
    return def
}

func envOrBool(k string, def bool) bool {
    if v := os.Getenv(k); v != "" {
        if b, err := strconv.ParseBool(v); err == nil {
            return b
        }
        fmt.Fprintf(os.Stderr, "warning: %s=%q is not a valid bool; using default %v\n", k, v, def)
    }
    return def
}
```

There's no test for this task — pure scaffolding (per the M4 spec's TDD exemption clause). The next task wires these into the client and exercises them via end-to-end tests.

- [ ] **Step 2: Verify the binary still builds**

```bash
go build ./cmd/tap/
```

Expected: success.

- [ ] **Step 3: Commit**

```bash
git add cmd/tap/
git commit -m "$(cat <<'EOF'
M4: cmd/tap flags — http-timeout, per-host-inflight, ssrf-*, poll-*

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6.2: Replace inline `http.Client` with `httpx.NewClient`

**Files:**
- Modify: `cmd/tap/main.go`
- Modify: `cmd/tap/main_test.go`

- [ ] **Step 1: Write the failing end-to-end test**

Append to `cmd/tap/main_test.go`:

```go
import (
    "github.com/bcrisp4/tap/internal/httpx"
)

func TestE2E_SSRFRejectsLoopbackByDefault(t *testing.T) {
    ctx := context.Background()
    tmp := t.TempDir()
    d, _ := db.Open(ctx, filepath.Join(tmp, "tap.db"))
    defer d.Close()
    _ = db.Migrate(ctx, d)

    // Origin runs on 127.0.0.1 via httptest.
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title></channel></rss>`))
    }))
    defer srv.Close()

    // Default-reject SSRF policy.
    client := httpx.NewClient(httpx.Opts{Timeout: 5 * time.Second, SSRF: httpx.SSRFPolicy{}})
    sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
        Processor: processor.New(sanitise.DefaultPolicy(), nil),
        Workers:   1,
        Floor:     15 * time.Minute,
        Ceiling:   24 * time.Hour,
    })
    sched.Start()
    defer sched.Stop()

    subID, _ := db.InsertSubscription(ctx, d, db.NewSubscription{
        Title: "Loopback", FeedURL: srv.URL, NextPoll: 0, Created: time.Now().Unix(),
    })
    _ = sched.Wait(2 * time.Second)
    _ = sched.Tick(ctx)
    _ = sched.Wait(2 * time.Second)

    var lastError sql.NullString
    _ = d.QueryRowContext(ctx, `SELECT last_error FROM subscriptions WHERE id=?`, subID).Scan(&lastError)
    if !lastError.Valid || !strings.Contains(lastError.String, "ssrf") {
        t.Errorf("expected ssrf error, got %q", lastError.String)
    }
}

func TestE2E_SSRFAllowlistAcceptsLoopback(t *testing.T) {
    ctx := context.Background()
    tmp := t.TempDir()
    d, _ := db.Open(ctx, filepath.Join(tmp, "tap.db"))
    defer d.Close()
    _ = db.Migrate(ctx, d)

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title></channel></rss>`))
    }))
    defer srv.Close()

    policy, _ := httpx.ParseSSRFPolicy(false, []string{"127.0.0.0/8"})
    client := httpx.NewClient(httpx.Opts{Timeout: 5 * time.Second, SSRF: policy})
    sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
        Processor: processor.New(sanitise.DefaultPolicy(), nil),
        Workers:   1,
        Floor:     15 * time.Minute,
        Ceiling:   24 * time.Hour,
    })
    sched.Start()
    defer sched.Stop()

    subID, _ := db.InsertSubscription(ctx, d, db.NewSubscription{
        Title: "Allowed", FeedURL: srv.URL, NextPoll: 0, Created: time.Now().Unix(),
    })
    _ = sched.Tick(ctx)
    _ = sched.Wait(2 * time.Second)

    var lastError sql.NullString
    _ = d.QueryRowContext(ctx, `SELECT last_error FROM subscriptions WHERE id=?`, subID).Scan(&lastError)
    if lastError.Valid {
        t.Errorf("expected no error, got %q", lastError.String)
    }
}
```

(Imports may need adjustments — read the existing main_test.go to match style.)

- [ ] **Step 2: Verify the test fails (or compiles but the existing main.go doesn't yet wire httpx)**

```bash
go test ./cmd/tap/ -run TestE2E_SSRF -v
```

Expected: FAIL — main.go uses inline `http.Client` and would happily hit loopback.

- [ ] **Step 3: Replace inline client in main.go**

In `cmd/tap/main.go`:

```go
import (
    // existing imports
    "github.com/bcrisp4/tap/internal/httpx"
)

// (after migrations, before constructing the proxy)
ssrfPolicy, err := httpx.ParseSSRFPolicy(*ssrfDisabled, []string(ssrfAllow))
if err != nil {
    slog.Error("parse ssrf-allow", "err", err)
    os.Exit(1)
}
if *ssrfDisabled {
    slog.Warn("SSRF guard disabled — outbound HTTP unrestricted")
}

// Resolve the http-timeout, honouring the deprecated --proxy-fetch-timeout.
timeout := *httpTimeout
if *proxyFetchTO > 0 {
    slog.Warn("--proxy-fetch-timeout is deprecated; use --http-timeout")
    if *httpTimeout == 30*time.Second { // user didn't override http-timeout
        timeout = *proxyFetchTO
    }
}

client := httpx.NewClient(httpx.Opts{
    Timeout:         timeout,
    PerHostInflight: *perHostInfl,
    SSRF:            ssrfPolicy,
    UserAgent:       *userAgent,
})
```

Delete the old inline `client := &http.Client{...}` block.

Update the scheduler construction to pass the new opts:

```go
sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
    Processor: proc,
    Floor:     *pollFloor,
    Ceiling:   *pollCeiling,
    ErrorBase: *pollErrorBase,
})
```

- [ ] **Step 4: Verify**

```bash
make test 2>&1 | tail -30
```

Expected: PASS for all tests including the new e2e SSRF tests.

- [ ] **Step 5: Commit**

```bash
git add cmd/tap/
git commit -m "$(cat <<'EOF'
M4: cmd/tap uses httpx.NewClient for shared HTTP

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task 6.3: End-to-end retry-after and per-host limiter tests

**Files:**
- Modify: `cmd/tap/main_test.go`

- [ ] **Step 1: Write the tests**

Append:

```go
func TestE2E_RetryAfterFloorOnSuccess(t *testing.T) {
    ctx := context.Background()
    tmp := t.TempDir()
    d, _ := db.Open(ctx, filepath.Join(tmp, "tap.db"))
    defer d.Close()
    _ = db.Migrate(ctx, d)

    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Retry-After", "3600")
        _, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title></channel></rss>`))
    }))
    defer srv.Close()

    policy, _ := httpx.ParseSSRFPolicy(false, []string{"127.0.0.0/8"})
    client := httpx.NewClient(httpx.Opts{Timeout: 5 * time.Second, SSRF: policy})
    sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
        Processor: processor.New(sanitise.DefaultPolicy(), nil),
        Workers:   1,
        Floor:     15 * time.Minute,
        Ceiling:   24 * time.Hour,
    })
    sched.Start()
    defer sched.Stop()

    pollStart := time.Now().Unix()
    subID, _ := db.InsertSubscription(ctx, d, db.NewSubscription{
        Title: "Slow", FeedURL: srv.URL, NextPoll: 0, Created: pollStart,
    })
    _ = sched.Tick(ctx)
    _ = sched.Wait(2 * time.Second)

    var nextPoll int64
    _ = d.QueryRowContext(ctx, `SELECT next_poll_at FROM subscriptions WHERE id=?`, subID).Scan(&nextPoll)
    if nextPoll < pollStart+3600 {
        t.Errorf("next_poll = %d; want >= %d", nextPoll, pollStart+3600)
    }
}

func TestE2E_PerHostInflightSerialises(t *testing.T) {
    var inflight, maxObserved atomic.Int32
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        n := inflight.Add(1)
        for {
            m := maxObserved.Load()
            if n <= m || maxObserved.CompareAndSwap(m, n) {
                break
            }
        }
        time.Sleep(50 * time.Millisecond)
        inflight.Add(-1)
        _, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title></channel></rss>`))
    }))
    defer srv.Close()

    policy, _ := httpx.ParseSSRFPolicy(false, []string{"127.0.0.0/8"})
    client := httpx.NewClient(httpx.Opts{
        Timeout:         5 * time.Second,
        PerHostInflight: 1,
        SSRF:            policy,
    })

    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            resp, err := client.Get(srv.URL)
            if err == nil {
                _, _ = io.Copy(io.Discard, resp.Body)
                _ = resp.Body.Close()
            }
        }()
    }
    wg.Wait()

    if got := maxObserved.Load(); got != 1 {
        t.Errorf("maxObserved = %d; want 1 with PerHostInflight=1", got)
    }
}
```

- [ ] **Step 2: Verify**

```bash
go test ./cmd/tap/ -run TestE2E -v -race
```

Expected: PASS for all five end-to-end tests.

- [ ] **Step 3: Commit**

```bash
git add cmd/tap/
git commit -m "$(cat <<'EOF'
M4: e2e tests for retry-after floor and per-host limiter

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase 7: README

### Task 7.1: M4 trust posture section

**Files:**
- Modify: `README.md`

There's no test for this — pure docs. Write the section, eyeball it, commit.

- [ ] **Step 1: Read the existing README**

```bash
sed -n '1,200p' README.md
```

Find the "Trust posture: M2/M3" section (added in commit `7276ee9`).

- [ ] **Step 2: Append the M4 section**

After the M3 trust-posture paragraph, add:

```markdown
**Trust posture: M4.** Outbound HTTP runs through one shared client. Destinations resolving to loopback, RFC1918, link-local, CGNAT, or ULA are rejected before connect. Tailscale users on the default `100.64.0.0/10` CGNAT range need `--ssrf-allow=100.64.0.0/10` (or their tailnet's specific subnet, or `--ssrf-allow=<tailnet>.ts.net` for a hostname suffix). Redirects are re-checked independently. The same per-hostname concurrency cap (default 4) applies to feed fetches and media-proxy origin fetches. Polling cadence is adaptive: a fast feed polls every 15 minutes, a quiet feed every 24 hours; origin-mandated `Retry-After` and `Cache-Control: max-age` are honoured as floors. Failed polls back off exponentially (5m → 10m → 20m → 40m … capped at 24h, with 25% jitter). Set `--ssrf-disabled` only on fully-trusted networks; the binary logs a startup WARN when this flag is on.

**Upgrading from M3.** One column added to `subscriptions`: `velocity_24h_x100`. Existing rows start at velocity 0 (24h ceiling) and back-fill on their next successful poll or 304.

**Configuration knobs added by M4:**
- `--http-timeout` (env `TAP_HTTP_TIMEOUT`, default `30s`) — total per-request HTTP deadline. Replaces `--proxy-fetch-timeout` (deprecated alias kept for one release).
- `--per-host-inflight` (env `TAP_PER_HOST_INFLIGHT`, default `4`) — concurrent outbound requests per hostname.
- `--ssrf-disabled` (env `TAP_SSRF_DISABLED`, default `false`) — global escape hatch.
- `--ssrf-allow` (env `TAP_SSRF_ALLOW`, default `""`) — repeatable allowlist entry. CSV in env. Auto-detect: `/`-bearing entries are CIDR; bare IPs become `/32` or `/128`; otherwise hostname suffix.
- `--poll-floor` / `--poll-ceiling` / `--poll-error-base` — adaptive cadence and error-backoff tuning.
- `--user-agent` (env `TAP_USER_AGENT`) — set centrally on the shared client.
```

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "$(cat <<'EOF'
M4: README — trust posture, Tailscale CGNAT note, config knobs

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Final verification

- [ ] **Run the full test suite with race detector**

```bash
make test
```

Expected: PASS, no race warnings.

- [ ] **Verify the binary builds statically**

```bash
make build
```

Expected: `bin/tap` produced; size similar to M3.

- [ ] **Boot the binary against a fresh data dir**

```bash
rm -rf /tmp/tap-m4-smoke && bin/tap -data /tmp/tap-m4-smoke -addr 127.0.0.1:8081 &
sleep 2
curl -s http://127.0.0.1:8081/healthz
kill %1
```

Expected: `200 OK` on `/healthz`; logs show migrations applied (`0001_initial`, `0002_configuration`, `0003_polling_discipline`).

- [ ] **Boot against an existing M3 database**

If a stale M3 database exists from a prior milestone:

```bash
bin/tap -data /path/to/old-m3-data -addr 127.0.0.1:8081 &
sleep 2
sqlite3 /path/to/old-m3-data/tap.db "PRAGMA table_info(subscriptions)" | grep velocity_24h_x100
kill %1
```

Expected: `velocity_24h_x100|INTEGER|...|0`. The migration ran cleanly.

- [ ] **Update CLAUDE.md status line**

Edit `CLAUDE.md` so the project header reflects M4 completion. Find the line near the top:

```
M3 in review (media proxy + cache implemented, awaiting merge — ...)
```

Replace with:

```
**M4 in review** (polling discipline implemented, awaiting merge — spec at `docs/specs/2026-05-09-m4-polling-discipline.md`; M3 media proxy merged — spec at `docs/specs/2026-05-09-m3-media-proxy.md`; ...)
```

Also extend the "Trust posture" section:

```
- The shared HTTP client is constructed via `httpx.NewClient(opts)`. SSRF is enforced by `internal/httpx/ssrf.go` (`SSRFPolicy.AllowAddr` for the dialer, `CheckRedirect` for redirects). Per-host concurrency is capped at 4 by default in `internal/httpx/hostlimit.go`.
- The polling cadence is adaptive (`internal/cadence/`), driven by the `velocity_24h_x100` column on `subscriptions`. The floor is 15min, the ceiling 24h.
- The deprecated default of strict serialisation (`PerHostInflight=1`) is intentionally relaxed to 4 in M4 — see the spec's "Risks" section. Don't drop it back to 1 without a corresponding plan-level discussion.
```

- [ ] **Commit the CLAUDE.md status bump**

```bash
git add CLAUDE.md
git commit -m "$(cat <<'EOF'
M4: CLAUDE.md status line + trust-posture references

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Spec coverage check

Mapping each spec section to a task:

| Spec section | Tasks |
|---|---|
| New package `internal/httpx` | 2.1, 2.3, 2.4, 2.5, 2.6 |
| SSRF policy (default rejects, allowlist, dialer, redirect) | 2.1, 2.2, 2.3, 2.4, 2.6 |
| Allowlist syntax | 2.3 |
| Per-host limiter | 2.5 |
| Adaptive cadence (column, formula, recompute) | 1.2, 3.1, 3.2, 3.4 |
| Server-mandated floor | 1.4, 1.5, 1.6, 3.4, 5.3, 5.4 |
| Error backoff | 1.3, 5.2 |
| Pure cadence functions in `internal/cadence` | 1.1–1.6 |
| Worker integration | 5.1, 5.2, 5.3, 5.4 |
| `feed.Fetch` shrinks (no UA) | 4.1 |
| `feed.FetchResult` populated on error | 4.2 |
| Wire-up in `cmd/tap/main.go` | 6.2 |
| Configuration knobs | 6.1 |
| Deprecated `--proxy-fetch-timeout` alias | 6.1, 6.2 |
| README update | 7.1 |
| End-to-end SSRF tests | 6.2, 6.3 |
| End-to-end Retry-After / per-host tests | 6.3 |

All spec sections are covered. Definition-of-done items 1-13 are validated by the Final Verification section and the per-task assertions.

---

## Plan complete

Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints.

Which approach?
