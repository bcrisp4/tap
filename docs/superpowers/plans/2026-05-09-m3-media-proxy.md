# M3 Media Proxy + Cache Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the internal media proxy from M3 spec (`docs/specs/2026-05-09-m3-media-proxy.md`): every `<img>` URL in sanitised entries points at `/api/v1/proxy/<token>`; the proxy fetches, validates MIME, caches on disk, and serves with `Cache-Control: immutable`.

**Architecture:** Two new packages. `internal/proxy/` owns the `Signer` (HMAC-SHA256 token sign/verify), the `Cache` (FS layer with JSON sidecar, singleflight coalescing, mtime-LRU eviction), and the HTTP `Handler`. `internal/processor/` composes M2's sanitiser with an image-URL rewriter so the worker calls one `Process(html) string` instead of reaching into either concern. The signing key is generated once at first launch and persisted in a new SQLite `configuration` key/value table. `internal/sanitise` is unchanged from M2.

**Tech Stack:** Go 1.25, `modernc.org/sqlite`, `golang.org/x/sync/singleflight`, `crypto/hmac`, `crypto/sha256`, `crypto/rand`, `encoding/base64`, stdlib `net/http`. Test framework: `github.com/stretchr/testify/require` (already used by M1/M2).

---

## File structure

**Create:**

| File | Responsibility |
|---|---|
| `internal/db/migrations/0002_configuration.sql` | Adds `configuration` key/value table |
| `internal/db/config.go` | `GetConfig(ctx, key) (val []byte, ok bool, err error)` and `SetConfigIfAbsent(ctx, key, val) ([]byte, error)` |
| `internal/db/config_test.go` | Round-trip tests for config helpers |
| `internal/proxy/signer.go` | `Signer` struct + `Sign`, `Verify`, `RewriteImageURL` methods |
| `internal/proxy/signer_test.go` | Sign/Verify determinism, malformed-input rejection, constant-time-compare |
| `internal/proxy/sniff.go` | `validateImage(buf, headerCT) (sniffed string, ok bool)` + MIME allowlist constant |
| `internal/proxy/sniff_test.go` | Per-format fixtures (PNG/JPEG/GIF/WebP/AVIF) + reject cases |
| `internal/proxy/cache.go` | `Cache` struct, `Get(ctx, hash, fetch) (FetchedResource, error)`, atomic writes, singleflight, mtime-LRU |
| `internal/proxy/cache_test.go` | Hit/miss/atomic/LRU/singleflight tests |
| `internal/proxy/handler.go` | HTTP handler that wires Signer + Cache + http.Client + sniff |
| `internal/proxy/handler_test.go` | Request-flow tests against `httptest.Server` origin |
| `internal/processor/processor.go` | `Processor` struct, `New`, `Process` |
| `internal/processor/processor_test.go` | Composition tests (with and without rewriter) |

**Modify:**

| File | Change |
|---|---|
| `internal/poll/scheduler.go:17-22` | Rename `SchedulerOpts.Policy *sanitise.Policy` → `SchedulerOpts.Processor *processor.Processor`; default to `processor.New(sanitise.DefaultPolicy(), nil)`. Update line 66 to pass `Processor` into `NewWorker`. |
| `internal/poll/worker.go:13-42, 83` | Rename `WorkerOpts.Policy` → `WorkerOpts.Processor`; change type; rename panic message; update line 83 from `Policy.Sanitise` to `Processor.Process`. Drop the `sanitise` import. |
| `internal/poll/worker_test.go:68-71, 97-100, 121-124` | Replace `Policy: sanitise.DefaultPolicy()` → `Processor: processor.New(sanitise.DefaultPolicy(), nil)`. |
| `internal/poll/scheduler_test.go` | Same `Policy` → `Processor` rename in any `SchedulerOpts` literal. |
| `internal/api/api.go:12-26` | Add a third parameter `proxyHandler http.Handler` to `NewMux`; register `m.Handle("/api/v1/proxy/", proxyHandler)` when non-nil. |
| `internal/api/*_test.go` (api_test, entries_test, subscriptions_test) | Add `, nil` third arg to every `api.NewMux(...)` call. |
| `cmd/tap/main.go:67-78` | Bootstrap signing key from `configuration` table; build `proxy.NewSigner`, `proxy.NewCache`, `proxy.NewHandler`; build `processor.New(sanitise.DefaultPolicy(), signer.RewriteImageURL)`; pass processor into scheduler; pass proxy handler into `api.NewMux`. Add new flags. |
| `cmd/tap/main_test.go` | Add `, nil` to existing `api.NewMux(d, nil)` call; replace `Policy: sanitise.DefaultPolicy()` with `Processor: processor.New(...)`. Extend hostile-feed fixture with an image and assert proxy URL. |
| `README.md` | Replace M2 trust posture with M3; document `cache/` directory and configuration table; M2→M3 upgrade note. |

**Verify unchanged:**

| File | Goal |
|---|---|
| `internal/sanitise/sanitise.go` | No diff (preserves M2 behaviour exactly) |
| `internal/sanitise/sanitise_test.go` | No diff |
| `internal/urlcleaner/*` | No diff |

---

## Task 1: Configuration table migration + db helpers

**Files:**
- Create: `internal/db/migrations/0002_configuration.sql`
- Create: `internal/db/config.go`
- Create: `internal/db/config_test.go`

- [ ] **Step 1: Write the failing migration test**

Append to `internal/db/migrate_test.go` (or create a new test file at the same path — check the existing tests' style first; M1 has `migrate_test.go`):

If `internal/db/migrate_test.go` already exercises migration version counts, add a case there. Otherwise, create the test as part of `config_test.go`:

```go
package db_test

import (
	"context"
	"testing"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

func TestMigrate_AddsConfigurationTable(t *testing.T) {
	t.Parallel()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(context.Background(), d))

	var name string
	err = d.QueryRowContext(context.Background(),
		`SELECT name FROM sqlite_master WHERE type='table' AND name='configuration'`).Scan(&name)
	require.NoError(t, err)
	require.Equal(t, "configuration", name)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/db -run TestMigrate_AddsConfigurationTable -race`
Expected: FAIL — `sql: no rows in result set` because migration 0002 doesn't exist yet.

- [ ] **Step 3: Create the migration file**

Create `internal/db/migrations/0002_configuration.sql`:

```sql
CREATE TABLE configuration (
    key   TEXT PRIMARY KEY,
    value BLOB NOT NULL
);
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/db -run TestMigrate_AddsConfigurationTable -race`
Expected: PASS.

- [ ] **Step 5: Write the failing config helpers test**

Append to `internal/db/config_test.go`:

```go
func TestSetConfigIfAbsent_InsertsAndIsIdempotent(t *testing.T) {
	t.Parallel()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(context.Background(), d))

	got, err := db.SetConfigIfAbsent(context.Background(), d, "k", []byte("v1"))
	require.NoError(t, err)
	require.Equal(t, []byte("v1"), got)

	// Second call with a different value must not overwrite.
	got2, err := db.SetConfigIfAbsent(context.Background(), d, "k", []byte("v2"))
	require.NoError(t, err)
	require.Equal(t, []byte("v1"), got2)
}

func TestGetConfig_HitAndMiss(t *testing.T) {
	t.Parallel()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(context.Background(), d))

	val, ok, err := db.GetConfig(context.Background(), d, "missing")
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, val)

	_, err = db.SetConfigIfAbsent(context.Background(), d, "found", []byte("x"))
	require.NoError(t, err)

	val, ok, err = db.GetConfig(context.Background(), d, "found")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []byte("x"), val)
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `go test ./internal/db -run TestSetConfigIfAbsent -race`
Expected: FAIL — undefined: `db.SetConfigIfAbsent` (and `db.GetConfig`).

- [ ] **Step 7: Implement the helpers**

Create `internal/db/config.go`:

```go
package db

import (
	"context"
	"database/sql"
	"fmt"
)

// GetConfig returns the bytes stored under key. ok=false means the row is absent;
// (nil, false, nil) is the canonical "not found" tuple.
func GetConfig(ctx context.Context, d *sql.DB, key string) (value []byte, ok bool, err error) {
	row := d.QueryRowContext(ctx, `SELECT value FROM configuration WHERE key = ?`, key)
	var v []byte
	if err := row.Scan(&v); err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("scan configuration[%s]: %w", key, err)
	}
	return v, true, nil
}

// SetConfigIfAbsent inserts (key, value) if the row doesn't exist, then returns
// the row's actual value (so callers see the existing value when present).
// Idempotent: a second call with the same key is a no-op.
func SetConfigIfAbsent(ctx context.Context, d *sql.DB, key string, value []byte) ([]byte, error) {
	if _, err := d.ExecContext(ctx,
		`INSERT INTO configuration (key, value) VALUES (?, ?) ON CONFLICT(key) DO NOTHING`,
		key, value); err != nil {
		return nil, fmt.Errorf("insert configuration[%s]: %w", key, err)
	}
	got, ok, err := GetConfig(ctx, d, key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("configuration[%s] missing after upsert", key)
	}
	return got, nil
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/db -race`
Expected: PASS for all three tests (migration + Set + Get).

- [ ] **Step 9: Commit**

```bash
git add internal/db/migrations/0002_configuration.sql internal/db/config.go internal/db/config_test.go
git commit -m "Add configuration table and Get/SetConfigIfAbsent helpers

Lays the foundation for M3's signed proxy tokens. Concept §4 names this
key/value table for runtime-generated state; M3 uses it for the proxy
signing key, M6+ will reuse it.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: proxy.Signer

**Files:**
- Create: `internal/proxy/signer.go`
- Create: `internal/proxy/signer_test.go`

- [ ] **Step 1: Write the failing test for Sign**

Create `internal/proxy/signer_test.go`:

```go
package proxy_test

import (
	"strings"
	"testing"

	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/stretchr/testify/require"
)

var testKey = []byte("0123456789abcdef0123456789abcdef") // 32 bytes

func TestSigner_SignDeterministic(t *testing.T) {
	t.Parallel()
	s := proxy.NewSigner(testKey)
	tok1 := s.Sign("https://example.com/img.png")
	tok2 := s.Sign("https://example.com/img.png")
	require.Equal(t, tok1, tok2, "Sign must be deterministic for the same (key, URL)")
	require.Contains(t, tok1, ".", "token format is <b64url>.<b64url>")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/proxy -run TestSigner_SignDeterministic -race`
Expected: FAIL — `package github.com/bcrisp4/tap/internal/proxy is not in std`.

- [ ] **Step 3: Implement Sign**

Create `internal/proxy/signer.go`:

```go
// Package proxy implements the internal media proxy: signed-token URLs,
// filesystem cache with JSON sidecar metadata, MIME validation, and the
// HTTP handler that ties them together. See docs/specs/2026-05-09-m3-media-proxy.md.
package proxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// Signer signs and verifies media-proxy tokens with HMAC-SHA256.
// The HMAC is truncated to 16 bytes (128 bits) — well above any reasonable
// forge cost for an internal-only proxy and keeps tokens compact.
type Signer struct {
	key []byte
}

const hmacBytes = 16

// NewSigner copies the key. Callers may safely reuse the underlying slice.
func NewSigner(key []byte) *Signer {
	cp := make([]byte, len(key))
	copy(cp, key)
	return &Signer{key: cp}
}

// Sign returns "<b64url(rawURL)>.<b64url(hmac[:16])>". Deterministic per (key, url).
func (s *Signer) Sign(rawURL string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(rawURL))
	sum := mac.Sum(nil)[:hmacBytes]

	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(rawURL)) + "." + enc.EncodeToString(sum)
}

// RewriteImageURL returns "/api/v1/proxy/<token>" — the exact form the
// processor injects into <img src> attributes.
func (s *Signer) RewriteImageURL(rawURL string) string {
	return "/api/v1/proxy/" + s.Sign(rawURL)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/proxy -run TestSigner_SignDeterministic -race`
Expected: PASS.

- [ ] **Step 5: Write failing tests for Verify**

Append to `internal/proxy/signer_test.go`:

```go
func TestSigner_VerifyAcceptsValidToken(t *testing.T) {
	t.Parallel()
	s := proxy.NewSigner(testKey)
	tok := s.Sign("https://example.com/img.png")
	got, ok := s.Verify(tok)
	require.True(t, ok)
	require.Equal(t, "https://example.com/img.png", got)
}

func TestSigner_VerifyRejectsMismatchedHMAC(t *testing.T) {
	t.Parallel()
	s1 := proxy.NewSigner(testKey)
	s2 := proxy.NewSigner([]byte("ffffffffffffffffffffffffffffffff"))
	tok := s1.Sign("https://example.com/img.png")
	_, ok := s2.Verify(tok)
	require.False(t, ok)
}

func TestSigner_VerifyRejectsMalformed(t *testing.T) {
	t.Parallel()
	s := proxy.NewSigner(testKey)

	cases := []string{
		"",                              // empty
		"abc",                           // no dot
		"abc.def.ghi",                   // too many dots
		strings.Repeat("!", 10) + ".AA", // invalid base64 in URL component
		"AA." + strings.Repeat("!", 10), // invalid base64 in HMAC component
	}
	for _, tok := range cases {
		_, ok := s.Verify(tok)
		require.False(t, ok, "expected rejection for %q", tok)
	}
}

func TestSigner_RewriteImageURL(t *testing.T) {
	t.Parallel()
	s := proxy.NewSigner(testKey)
	got := s.RewriteImageURL("https://example.com/img.png")
	require.True(t, strings.HasPrefix(got, "/api/v1/proxy/"), "got: %s", got)
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `go test ./internal/proxy -run TestSigner_Verify -race`
Expected: FAIL — undefined: `s.Verify`.

- [ ] **Step 7: Implement Verify**

Append to `internal/proxy/signer.go`:

```go
// Verify parses tok, recomputes the HMAC, and constant-time-compares.
// Returns the original URL on success.
func (s *Signer) Verify(tok string) (string, bool) {
	dot := strings.IndexByte(tok, '.')
	if dot <= 0 || dot == len(tok)-1 {
		return "", false
	}
	if strings.Contains(tok[dot+1:], ".") {
		return "", false
	}
	enc := base64.RawURLEncoding
	rawURL, err := enc.DecodeString(tok[:dot])
	if err != nil {
		return "", false
	}
	gotMAC, err := enc.DecodeString(tok[dot+1:])
	if err != nil {
		return "", false
	}

	mac := hmac.New(sha256.New, s.key)
	mac.Write(rawURL)
	wantMAC := mac.Sum(nil)[:hmacBytes]

	if !hmac.Equal(gotMAC, wantMAC) {
		return "", false
	}
	return string(rawURL), true
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `go test ./internal/proxy -race`
Expected: PASS for all signer tests.

- [ ] **Step 9: Commit**

```bash
git add internal/proxy/signer.go internal/proxy/signer_test.go
git commit -m "Add proxy.Signer with HMAC-SHA256 token sign/verify

Tokens are <b64url(url)>.<b64url(hmac[:16])>, deterministic per (key, url)
so the future M10 service worker cache hits across page loads. Verify
constant-time-compares via hmac.Equal.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: proxy MIME sniff helper

**Files:**
- Create: `internal/proxy/sniff.go`
- Create: `internal/proxy/sniff_test.go`

- [ ] **Step 1: Write failing tests with per-format byte fixtures**

Create `internal/proxy/sniff_test.go`:

```go
package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Per-format minimal magic byte fixtures. http.DetectContentType only
// inspects the first 512 bytes; these prefixes are sufficient.
var fixtures = map[string][]byte{
	"image/png":  {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, // PNG signature
	"image/jpeg": {0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'},
	"image/gif":  []byte("GIF89a"),
	"image/webp": append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 4)...),
	"image/avif": append([]byte{0x00, 0x00, 0x00, 0x20}, []byte("ftypavif")...),
}

func TestValidateImage_AcceptsAllowlistedFormats(t *testing.T) {
	t.Parallel()
	for want, body := range fixtures {
		want, body := want, body
		t.Run(want, func(t *testing.T) {
			t.Parallel()
			got, ok := validateImage(body, want)
			require.True(t, ok, "fixture must validate")
			require.Equal(t, want, got, "sniffed type must match expected")
		})
	}
}

func TestValidateImage_RejectsTextHTML(t *testing.T) {
	t.Parallel()
	body := []byte("<!doctype html><html></html>")
	_, ok := validateImage(body, "text/html")
	require.False(t, ok)
}

func TestValidateImage_RejectsHeaderMismatch(t *testing.T) {
	t.Parallel()
	// Real PNG bytes but origin claims text/html — drop.
	_, ok := validateImage(fixtures["image/png"], "text/html")
	require.False(t, ok)
}

func TestValidateImage_AcceptsHeaderWithCharset(t *testing.T) {
	t.Parallel()
	// Origins sometimes append parameters — strip them before compare.
	got, ok := validateImage(fixtures["image/png"], "image/png; charset=binary")
	require.True(t, ok)
	require.Equal(t, "image/png", got)
}

func TestValidateImage_RejectsSVG(t *testing.T) {
	t.Parallel()
	body := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	_, ok := validateImage(body, "image/svg+xml")
	require.False(t, ok, "SVG must be rejected — not in M3 allowlist")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/proxy -run TestValidateImage -race`
Expected: FAIL — undefined: `validateImage`.

- [ ] **Step 3: Implement validateImage**

Create `internal/proxy/sniff.go`:

```go
package proxy

import (
	"net/http"
	"strings"
)

// allowedMIMETypes is the M3 image allowlist. SVG is excluded — see
// docs/roadmap.md "Deferred items" for the rationale and the two paths
// to enabling it later.
var allowedMIMETypes = map[string]struct{}{
	"image/png":  {},
	"image/jpeg": {},
	"image/gif":  {},
	"image/webp": {},
	"image/avif": {},
}

// validateImage sniffs the first up-to-512 bytes of body and confirms
// that the result is in the allowlist AND agrees with the origin's
// Content-Type header (charset / boundary parameters are ignored on the
// header side). Returns the sniffed canonical type on success.
func validateImage(body []byte, headerCT string) (string, bool) {
	sniffed := http.DetectContentType(body)
	if _, ok := allowedMIMETypes[sniffed]; !ok {
		return "", false
	}
	headerType := strings.TrimSpace(strings.SplitN(headerCT, ";", 2)[0])
	if !strings.EqualFold(headerType, sniffed) {
		return "", false
	}
	return sniffed, true
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/proxy -run TestValidateImage -race`
Expected: PASS for all five fixture cases plus the three reject cases.

If any fixture fails: Go's `http.DetectContentType` on this version doesn't return the expected MIME for that format. Adjust the fixture to include more magic bytes (the spec's Risks section flagged AVIF in particular — Go 1.25 should cover it, but if 1.24 is the local Go this may need a longer fixture).

- [ ] **Step 5: Commit**

```bash
git add internal/proxy/sniff.go internal/proxy/sniff_test.go
git commit -m "Add proxy MIME sniff with per-format fixture tests

Allowlist: png/jpeg/gif/webp/avif. Sniff is the trust anchor; origin
Content-Type must agree. Per-format fixtures pin behaviour against
future Go upgrades.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: proxy.Cache — hit, miss, atomic writes

**Files:**
- Create: `internal/proxy/cache.go`
- Create: `internal/proxy/cache_test.go`

- [ ] **Step 1: Write failing tests for hit and miss**

Create `internal/proxy/cache_test.go`:

```go
package proxy_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/stretchr/testify/require"
)

func newCache(t *testing.T, capBytes int64) (*proxy.Cache, string) {
	t.Helper()
	dir := t.TempDir()
	return proxy.NewCache(dir, capBytes), dir
}

func TestCache_MissThenHit(t *testing.T) {
	t.Parallel()
	c, dir := newCache(t, 1<<20)

	var calls int32
	fetch := func(ctx context.Context) (proxy.FetchedResource, error) {
		atomic.AddInt32(&calls, 1)
		return proxy.FetchedResource{
			Bytes:       []byte("hello"),
			ContentType: "image/png",
			ETag:        `"abc"`,
		}, nil
	}

	got, err := c.Get(context.Background(), "abcdef", fetch)
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), got.Bytes)
	require.Equal(t, "image/png", got.ContentType)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))

	// Files should exist on disk in the sharded layout.
	binPath := filepath.Join(dir, "ab", "abcdef.bin")
	metaPath := filepath.Join(dir, "ab", "abcdef.meta")
	require.FileExists(t, binPath)
	require.FileExists(t, metaPath)

	// Second call: hit, no fetcher invocation.
	got2, err := c.Get(context.Background(), "abcdef", fetch)
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), got2.Bytes)
	require.Equal(t, "image/png", got2.ContentType)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls), "fetcher must not run on hit")
}

func TestCache_FetchErrorPropagates(t *testing.T) {
	t.Parallel()
	c, dir := newCache(t, 1<<20)

	wantErr := errors.New("origin down")
	_, err := c.Get(context.Background(), "deadbeef", func(ctx context.Context) (proxy.FetchedResource, error) {
		return proxy.FetchedResource{}, wantErr
	})
	require.ErrorIs(t, err, wantErr)

	// No files should be written on fetch failure.
	require.NoFileExists(t, filepath.Join(dir, "de", "deadbeef.bin"))
	require.NoFileExists(t, filepath.Join(dir, "de", "deadbeef.meta"))
}

func TestCache_OrphanBinTreatsAsMiss(t *testing.T) {
	t.Parallel()
	c, dir := newCache(t, 1<<20)

	// Plant a .bin without a .meta — simulates crash between renames.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ab"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ab", "abcdef.bin"), []byte("stale"), 0o644))

	var calls int32
	got, err := c.Get(context.Background(), "abcdef", func(ctx context.Context) (proxy.FetchedResource, error) {
		atomic.AddInt32(&calls, 1)
		return proxy.FetchedResource{Bytes: []byte("fresh"), ContentType: "image/png"}, nil
	})
	require.NoError(t, err)
	require.Equal(t, []byte("fresh"), got.Bytes)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

var _ = sync.Mutex{} // keeps the import even when later tests are added
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/proxy -run TestCache -race`
Expected: FAIL — undefined: `proxy.Cache`, `proxy.NewCache`, `proxy.FetchedResource`.

- [ ] **Step 3: Implement Cache foundation (hit, miss, atomic writes)**

Create `internal/proxy/cache.go`:

```go
package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// FetchedResource is what a fetcher closure returns. Bytes is the full body
// (caller is responsible for body cap); ContentType is the canonical sniffed
// type from validateImage; ETag is the origin's value, stored for forward
// compatibility (M3 doesn't act on it).
type FetchedResource struct {
	Bytes       []byte
	ContentType string
	ETag        string
}

// Cache is an FS-backed LRU cache for proxy responses. Bytes live in
// <dir>/<aa>/<hash>.bin; metadata in a sibling .meta JSON file.
// Eviction is mtime-based and fires inline when adding a new entry would
// exceed capBytes. Concurrent misses for the same hash collapse via singleflight.
type Cache struct {
	dir      string
	capBytes int64
	sf       singleflight.Group
	evictMu  sync.Mutex
}

func NewCache(dir string, capBytes int64) *Cache {
	return &Cache{dir: dir, capBytes: capBytes}
}

type sidecar struct {
	ContentType string `json:"content_type"`
	ETag        string `json:"etag,omitempty"`
	ByteCount   int64  `json:"byte_count"`
	FetchedAt   int64  `json:"fetched_at"`
}

// Get returns cached bytes (hit) or invokes fetch (miss), caches the result,
// and returns the bytes. Concurrent misses for the same hash run fetch once.
func (c *Cache) Get(ctx context.Context, hash string, fetch func(ctx context.Context) (FetchedResource, error)) (FetchedResource, error) {
	if got, ok := c.tryHit(hash); ok {
		return got, nil
	}

	v, err, _ := c.sf.Do(hash, func() (any, error) {
		// Recheck under singleflight in case another goroutine just filled the cache.
		if got, ok := c.tryHit(hash); ok {
			return got, nil
		}
		got, err := fetch(ctx)
		if err != nil {
			return FetchedResource{}, err
		}
		if err := c.write(hash, got); err != nil {
			return FetchedResource{}, fmt.Errorf("cache write: %w", err)
		}
		return got, nil
	})
	if err != nil {
		return FetchedResource{}, err
	}
	return v.(FetchedResource), nil
}

func (c *Cache) paths(hash string) (binPath, metaPath, dirPath string) {
	if len(hash) < 2 {
		return "", "", ""
	}
	dirPath = filepath.Join(c.dir, hash[:2])
	binPath = filepath.Join(dirPath, hash+".bin")
	metaPath = filepath.Join(dirPath, hash+".meta")
	return
}

func (c *Cache) tryHit(hash string) (FetchedResource, bool) {
	binPath, metaPath, _ := c.paths(hash)
	if binPath == "" {
		return FetchedResource{}, false
	}
	bin, err := os.ReadFile(binPath)
	if err != nil {
		return FetchedResource{}, false
	}
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return FetchedResource{}, false
	}
	var sc sidecar
	if err := json.Unmarshal(metaBytes, &sc); err != nil {
		return FetchedResource{}, false
	}
	return FetchedResource{
		Bytes:       bin,
		ContentType: sc.ContentType,
		ETag:        sc.ETag,
	}, true
}

func (c *Cache) write(hash string, res FetchedResource) error {
	binPath, metaPath, dirPath := c.paths(hash)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return err
	}

	binTmp := binPath + ".tmp"
	metaTmp := metaPath + ".tmp"
	if err := os.WriteFile(binTmp, res.Bytes, 0o644); err != nil {
		return err
	}

	sc := sidecar{
		ContentType: res.ContentType,
		ETag:        res.ETag,
		ByteCount:   int64(len(res.Bytes)),
		FetchedAt:   time.Now().Unix(),
	}
	scBytes, err := json.Marshal(sc)
	if err != nil {
		_ = os.Remove(binTmp)
		return err
	}
	if err := os.WriteFile(metaTmp, scBytes, 0o644); err != nil {
		_ = os.Remove(binTmp)
		return err
	}

	if err := os.Rename(binTmp, binPath); err != nil {
		_ = os.Remove(binTmp)
		_ = os.Remove(metaTmp)
		return err
	}
	if err := os.Rename(metaTmp, metaPath); err != nil {
		_ = os.Remove(metaTmp)
		_ = os.Remove(binPath)
		return err
	}
	return nil
}
```

- [ ] **Step 4: Add singleflight to go.mod**

Run: `go get golang.org/x/sync/singleflight && go mod tidy`
Expected: `go.mod` and `go.sum` updated.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/proxy -race`
Expected: PASS for the three Task 4 tests.

- [ ] **Step 6: Commit**

```bash
git add internal/proxy/cache.go internal/proxy/cache_test.go go.mod go.sum
git commit -m "Add proxy.Cache with hit/miss + atomic FS writes

Bytes in <dir>/<aa>/<hash>.bin, metadata in sibling .meta JSON. Two-step
rename for crash safety; orphan files (one half present) treated as miss.
Singleflight scaffolding in place; eviction lands in a follow-up commit.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: proxy.Cache — singleflight coalescing

**Files:**
- Modify: `internal/proxy/cache_test.go` (add new test)

The Cache implementation in Task 4 already wraps `fetch` in `c.sf.Do`. This task adds the test that proves coalescing works.

- [ ] **Step 1: Write the failing test**

Append to `internal/proxy/cache_test.go`:

```go
func TestCache_SingleflightCoalesces(t *testing.T) {
	t.Parallel()
	c, _ := newCache(t, 1<<20)

	const N = 20
	var calls int32
	start := make(chan struct{})
	gate := make(chan struct{})

	fetch := func(ctx context.Context) (proxy.FetchedResource, error) {
		atomic.AddInt32(&calls, 1)
		<-gate // hold the in-flight fetch open until the test releases it
		return proxy.FetchedResource{Bytes: []byte("x"), ContentType: "image/png"}, nil
	}

	var wg sync.WaitGroup
	results := make([]error, N)
	for i := 0; i < N; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := c.Get(context.Background(), "feedface", fetch)
			results[i] = err
		}()
	}

	close(start)
	// Give all goroutines a moment to enter c.Get and either find the
	// in-flight singleflight slot or hit the fast path.
	// (singleflight has a tiny window where the first caller is in fetch
	// but hasn't yet been registered; the test tolerates one extra call by
	// asserting <= 2 below to avoid flakiness on slow runners.)
	close(gate)
	wg.Wait()

	for _, err := range results {
		require.NoError(t, err)
	}
	got := atomic.LoadInt32(&calls)
	require.LessOrEqual(t, got, int32(2), "singleflight should collapse %d concurrent misses to 1 (allow 2 for race tolerance)", N)
	require.GreaterOrEqual(t, got, int32(1))
}
```

- [ ] **Step 2: Run test to verify it passes**

Since the Cache implementation already includes singleflight, this test should pass on first run. If we wanted strict TDD here we'd revert the singleflight change first; but singleflight is a structural choice (where the fetch lives in Get's body) rather than a behaviour layer. The test pins the property regardless.

Run: `go test ./internal/proxy -run TestCache_SingleflightCoalesces -race -count=10`
Expected: PASS (10× to surface flakes).

If it fails with `calls = N` (i.e., no coalescing): the singleflight call site in `Get` is wrong. Re-check `cache.go` — the closure must wrap both the fetch and the write.

- [ ] **Step 3: Commit**

```bash
git add internal/proxy/cache_test.go
git commit -m "Add singleflight coalescing test for proxy.Cache

Pins the property that N concurrent Gets for the same hash trigger
at most 1-2 origin fetches (2 tolerates singleflight's tiny
registration window).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: proxy.Cache — mtime-LRU eviction

**Files:**
- Modify: `internal/proxy/cache.go`
- Modify: `internal/proxy/cache_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/proxy/cache_test.go`:

```go
func TestCache_EvictsOldestByMtimeWhenOverCap(t *testing.T) {
	t.Parallel()
	// Cap of 200 bytes. Each fetch returns 100 bytes. Three writes will
	// force eviction of the first.
	c, dir := newCache(t, 200)

	body := make([]byte, 100)
	makeFetch := func() func(context.Context) (proxy.FetchedResource, error) {
		return func(ctx context.Context) (proxy.FetchedResource, error) {
			return proxy.FetchedResource{Bytes: body, ContentType: "image/png"}, nil
		}
	}

	_, err := c.Get(context.Background(), "aaaa1111", makeFetch())
	require.NoError(t, err)

	// Bump the mtime difference so sort is unambiguous on coarse-mtime FS.
	// (FAT and some ext4 setups have second-resolution mtimes.)
	require.NoError(t, os.Chtimes(filepath.Join(dir, "aa", "aaaa1111.bin"), time.Now().Add(-2*time.Second), time.Now().Add(-2*time.Second)))
	require.NoError(t, os.Chtimes(filepath.Join(dir, "aa", "aaaa1111.meta"), time.Now().Add(-2*time.Second), time.Now().Add(-2*time.Second)))

	_, err = c.Get(context.Background(), "bbbb2222", makeFetch())
	require.NoError(t, err)

	// Now total is 200 bytes (at cap). A third 100-byte write triggers eviction.
	_, err = c.Get(context.Background(), "cccc3333", makeFetch())
	require.NoError(t, err)

	require.NoFileExists(t, filepath.Join(dir, "aa", "aaaa1111.bin"), "oldest should be evicted")
	require.NoFileExists(t, filepath.Join(dir, "aa", "aaaa1111.meta"))
	require.FileExists(t, filepath.Join(dir, "bb", "bbbb2222.bin"))
	require.FileExists(t, filepath.Join(dir, "cc", "cccc3333.bin"))
}
```

Note the import additions: `"os"`, `"path/filepath"`, `"time"` may already be imported from earlier tests; if not, add them.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/proxy -run TestCache_Evicts -race`
Expected: FAIL — third file written without evicting the oldest; `aaaa1111.bin` still exists.

- [ ] **Step 3: Implement eviction**

Modify `internal/proxy/cache.go`. Replace the current `write` method with:

```go
func (c *Cache) write(hash string, res FetchedResource) error {
	binPath, metaPath, dirPath := c.paths(hash)

	if err := c.evictIfOverCap(int64(len(res.Bytes))); err != nil {
		return fmt.Errorf("evict: %w", err)
	}

	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return err
	}

	binTmp := binPath + ".tmp"
	metaTmp := metaPath + ".tmp"
	if err := os.WriteFile(binTmp, res.Bytes, 0o644); err != nil {
		return err
	}

	sc := sidecar{
		ContentType: res.ContentType,
		ETag:        res.ETag,
		ByteCount:   int64(len(res.Bytes)),
		FetchedAt:   time.Now().Unix(),
	}
	scBytes, err := json.Marshal(sc)
	if err != nil {
		_ = os.Remove(binTmp)
		return err
	}
	if err := os.WriteFile(metaTmp, scBytes, 0o644); err != nil {
		_ = os.Remove(binTmp)
		return err
	}

	if err := os.Rename(binTmp, binPath); err != nil {
		_ = os.Remove(binTmp)
		_ = os.Remove(metaTmp)
		return err
	}
	if err := os.Rename(metaTmp, metaPath); err != nil {
		_ = os.Remove(metaTmp)
		_ = os.Remove(binPath)
		return err
	}
	return nil
}
```

Append the eviction helpers:

```go
type evictEntry struct {
	binPath  string
	metaPath string
	mtime    time.Time
	size     int64
}

func (c *Cache) evictIfOverCap(incoming int64) error {
	c.evictMu.Lock()
	defer c.evictMu.Unlock()

	if c.capBytes <= 0 {
		return nil
	}

	entries, total, err := c.scanCache()
	if err != nil {
		return err
	}
	if total+incoming <= c.capBytes {
		return nil
	}

	// Sort by mtime ascending — oldest first.
	sortByMtime(entries)

	for _, e := range entries {
		if total+incoming <= c.capBytes {
			break
		}
		_ = os.Remove(e.binPath)
		_ = os.Remove(e.metaPath)
		total -= e.size
	}
	return nil
}

func (c *Cache) scanCache() ([]evictEntry, int64, error) {
	var entries []evictEntry
	var total int64
	err := filepath.WalkDir(c.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		// We pair .bin files only; the .meta sibling is removed alongside.
		if filepath.Ext(path) != ".bin" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		entries = append(entries, evictEntry{
			binPath:  path,
			metaPath: path[:len(path)-len(".bin")] + ".meta",
			mtime:    info.ModTime(),
			size:     info.Size(),
		})
		total += info.Size()
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, 0, err
	}
	return entries, total, nil
}

func sortByMtime(entries []evictEntry) {
	// Insertion sort: simple, stable, fine for the small N we expect.
	for i := 1; i < len(entries); i++ {
		j := i
		for j > 0 && entries[j-1].mtime.After(entries[j].mtime) {
			entries[j-1], entries[j] = entries[j], entries[j-1]
			j--
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/proxy -race`
Expected: PASS for all cache tests.

- [ ] **Step 5: Commit**

```bash
git add internal/proxy/cache.go internal/proxy/cache_test.go
git commit -m "Add inline mtime-LRU eviction to proxy.Cache

Before each write, if current_total + incoming > cap, walk the cache
dir, sort by mtime ascending, delete oldest .bin/.meta pairs until
under cap. Held under a single mutex; rare path (cap pressure only).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: proxy.Handler

**Files:**
- Create: `internal/proxy/handler.go`
- Create: `internal/proxy/handler_test.go`

- [ ] **Step 1: Write failing test for the cold-cache happy path**

Create `internal/proxy/handler_test.go`:

```go
package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/stretchr/testify/require"
)

// PNG fixture — 8-byte signature + minimal IHDR; sufficient for sniff.
var pngFixture = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
}

func newOriginServer(t *testing.T, body []byte, ct string, status int) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", ct)
		w.Header().Set("ETag", `"abc"`)
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func newHandler(t *testing.T) (*proxy.Signer, http.Handler) {
	t.Helper()
	signer := proxy.NewSigner(testKey)
	cache := proxy.NewCache(t.TempDir(), 1<<20)
	h := proxy.NewHandler(signer, cache, http.DefaultClient, 10<<20)
	return signer, h
}

func TestHandler_ColdCacheHappyPath(t *testing.T) {
	t.Parallel()
	origin, hits := newOriginServer(t, pngFixture, "image/png", http.StatusOK)
	signer, h := newHandler(t)

	tok := signer.Sign(origin.URL + "/img.png")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
	req.SetPathValue("token", tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Equal(t, "image/png", rr.Header().Get("Content-Type"))
	require.Equal(t, "public, max-age=31536000, immutable", rr.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))

	body, _ := io.ReadAll(rr.Body)
	require.Equal(t, pngFixture, body)
	require.Equal(t, int32(1), atomic.LoadInt32(hits))
}

func TestHandler_WarmCacheNoOriginCall(t *testing.T) {
	t.Parallel()
	origin, hits := newOriginServer(t, pngFixture, "image/png", http.StatusOK)
	signer, h := newHandler(t)

	tok := signer.Sign(origin.URL + "/img.png")

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
		req.SetPathValue("token", tok)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	}
	require.Equal(t, int32(1), atomic.LoadInt32(hits), "origin should be hit exactly once across 3 requests")
}

func TestHandler_BadTokenReturns404(t *testing.T) {
	t.Parallel()
	_, h := newHandler(t)
	for _, tok := range []string{"", "garbage", "abc.def", strings.Repeat("a", 100) + "." + strings.Repeat("b", 22)} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
		req.SetPathValue("token", tok)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code, "bad token %q should 404", tok)
	}
}

func TestHandler_OriginReturnsHTML_415(t *testing.T) {
	t.Parallel()
	origin, _ := newOriginServer(t, []byte("<html></html>"), "text/html", http.StatusOK)
	signer, h := newHandler(t)

	tok := signer.Sign(origin.URL + "/x")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
	req.SetPathValue("token", tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusUnsupportedMediaType, rr.Code)
}

func TestHandler_OriginReturns404_PassesThrough(t *testing.T) {
	t.Parallel()
	origin, _ := newOriginServer(t, []byte("not found"), "text/plain", http.StatusNotFound)
	signer, h := newHandler(t)

	tok := signer.Sign(origin.URL + "/missing")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
	req.SetPathValue("token", tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_OriginOversizeBody_502(t *testing.T) {
	t.Parallel()
	huge := make([]byte, 0, 2_000_000)
	huge = append(huge, pngFixture...)
	for len(huge) < 2_000_000 {
		huge = append(huge, 0x00)
	}
	origin, _ := newOriginServer(t, huge, "image/png", http.StatusOK)

	signer := proxy.NewSigner(testKey)
	cache := proxy.NewCache(t.TempDir(), 1<<20)
	h := proxy.NewHandler(signer, cache, http.DefaultClient, 1<<20) // 1 MiB body cap

	tok := signer.Sign(origin.URL + "/big")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+tok, nil)
	req.SetPathValue("token", tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadGateway, rr.Code)
}

```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/proxy -run TestHandler -race`
Expected: FAIL — undefined: `proxy.NewHandler`.

- [ ] **Step 3: Implement Handler**

Create `internal/proxy/handler.go`:

```go
package proxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
)

// Handler serves GET /api/v1/proxy/{token}. It verifies the token, consults
// the cache, and on miss fetches the origin URL through the supplied client.
type Handler struct {
	signer  *Signer
	cache   *Cache
	client  *http.Client
	bodyCap int64
}

// NewHandler constructs the proxy HTTP handler. bodyCap is the per-response
// byte limit applied via http.MaxBytesReader.
func NewHandler(signer *Signer, cache *Cache, client *http.Client, bodyCap int64) http.Handler {
	if signer == nil || cache == nil {
		panic("proxy.NewHandler: signer and cache are required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &Handler{signer: signer, cache: cache, client: client, bodyCap: bodyCap}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tok := r.PathValue("token")
	rawURL, ok := h.signer.Verify(tok)
	if !ok {
		http.NotFound(w, r)
		return
	}

	hash := urlHashOf(rawURL)
	got, err := h.cache.Get(r.Context(), hash, func(ctx context.Context) (FetchedResource, error) {
		return h.fetchOrigin(ctx, rawURL)
	})
	if err != nil {
		var fe *fetchError
		if errors.As(err, &fe) {
			http.Error(w, http.StatusText(fe.status), fe.status)
			return
		}
		slog.WarnContext(r.Context(), "proxy fetch failed", "url", rawURL, "err", err)
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", got.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(got.Bytes)))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(got.Bytes)
}

// fetchError carries an HTTP status to surface back to the client without
// being treated as an internal cache write failure.
type fetchError struct {
	status int
	msg    string
}

func (e *fetchError) Error() string { return e.msg }

func (h *Handler) fetchOrigin(ctx context.Context, rawURL string) (FetchedResource, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return FetchedResource{}, &fetchError{status: http.StatusBadGateway, msg: err.Error()}
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return FetchedResource{}, &fetchError{status: http.StatusBadGateway, msg: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Drain a few bytes to free the connection and propagate the status.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
		return FetchedResource{}, &fetchError{status: resp.StatusCode, msg: fmt.Sprintf("origin %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(http.MaxBytesReader(nil, resp.Body, h.bodyCap))
	if err != nil {
		return FetchedResource{}, &fetchError{status: http.StatusBadGateway, msg: err.Error()}
	}

	sniffed, ok := validateImage(body, resp.Header.Get("Content-Type"))
	if !ok {
		return FetchedResource{}, &fetchError{status: http.StatusUnsupportedMediaType, msg: "MIME validation failed"}
	}

	return FetchedResource{
		Bytes:       body,
		ContentType: sniffed,
		ETag:        resp.Header.Get("ETag"),
	}, nil
}

func urlHashOf(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/proxy -race`
Expected: PASS for all handler tests.

If the oversize-body test fails: `http.MaxBytesReader` reads up to `cap+1` bytes before erroring. The test uses 2 MiB body / 1 MiB cap — should reliably trigger.

If `OriginReturns404_PassesThrough` fails: check that `fetchError.status` is what the handler returns via `http.Error`. The status text body is fine; the test only asserts the code.

- [ ] **Step 5: Commit**

```bash
git add internal/proxy/handler.go internal/proxy/handler_test.go
git commit -m "Add proxy.Handler tying signer + cache + origin fetch

Verify token → cache.Get(hash, fetcher). Origin failures (4xx, 5xx,
oversize, MIME mismatch) propagate without writing to cache. Success
emits Content-Type from sniff, immutable Cache-Control, nosniff.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: processor package

**Files:**
- Create: `internal/processor/processor.go`
- Create: `internal/processor/processor_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/processor/processor_test.go`:

```go
package processor_test

import (
	"strings"
	"testing"

	"github.com/bcrisp4/tap/internal/processor"
	"github.com/bcrisp4/tap/internal/sanitise"
	"github.com/stretchr/testify/require"
)

func TestProcessor_NilRewriter_MatchesSanitiseBytewise(t *testing.T) {
	t.Parallel()
	pol := sanitise.DefaultPolicy()
	p := processor.New(pol, nil)

	cases := []string{
		`<p>hello</p>`,
		`<p>x</p><script>alert(1)</script>`,
		`<a href="https://example.com/?utm_source=x">link</a>`,
		`<img src="https://example.com/img.png">`,
		`<iframe src="https://www.youtube.com/embed/abc"></iframe>`,
		``,
	}
	for _, in := range cases {
		require.Equal(t, pol.Sanitise(in), p.Process(in), "input: %q", in)
	}
}

func TestProcessor_RewriterReplacesImgSrc(t *testing.T) {
	t.Parallel()
	pol := sanitise.DefaultPolicy()
	rewriter := func(s string) string { return "/PROXY/" + s }
	p := processor.New(pol, rewriter)

	in := `<img src="https://example.com/img.png">`
	out := p.Process(in)
	require.Contains(t, out, `src="/PROXY/https://example.com/img.png"`)
	require.NotContains(t, out, `src="https://example.com/img.png"`, "raw URL must be replaced: %s", out)
}

func TestProcessor_RewriterDoesNotTouchAnchorOrIframe(t *testing.T) {
	t.Parallel()
	pol := sanitise.DefaultPolicy()
	rewriter := func(s string) string { return "/PROXY/" + s }
	p := processor.New(pol, rewriter)

	in := `<a href="https://example.com/page">x</a><iframe src="https://www.youtube.com/embed/abc"></iframe>`
	out := p.Process(in)
	require.Contains(t, out, `href="https://example.com/page"`)
	require.Contains(t, out, `src="https://www.youtube.com/embed/abc"`)
	require.NotContains(t, out, "/PROXY/", "rewriter must touch <img> only: %s", out)
}

func TestProcessor_TotalFunctionContract(t *testing.T) {
	t.Parallel()
	pol := sanitise.DefaultPolicy()
	rewriter := func(s string) string { return "/PROXY/" + s }
	p := processor.New(pol, rewriter)

	bad := []string{
		"",
		"<<<<<",
		strings.Repeat("<img src='x'>", 1000),
		string([]byte{0xff, 0xfe, 0xfd}),
	}
	for _, in := range bad {
		// Must not panic; must return a string.
		_ = p.Process(in)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/processor -race`
Expected: FAIL — `package github.com/bcrisp4/tap/internal/processor is not in std`.

- [ ] **Step 3: Implement Processor**

Create `internal/processor/processor.go`:

```go
// Package processor composes M2's HTML sanitiser with M3's image-URL
// rewriter so the polling worker has a single Process(rawHTML) string
// entry point. The sanitiser is unchanged from M2; the rewriting walk
// is a small post-pass over the cleaned HTML.
package processor

import (
	"bytes"
	"log/slog"
	"strings"

	"github.com/bcrisp4/tap/internal/sanitise"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type Processor struct {
	sanitiser   *sanitise.Policy
	imgRewriter func(string) string
}

// New returns a Processor. If rewriter is nil, Process is equivalent to
// sanitiser.Sanitise (used by tests that don't care about the proxy).
func New(s *sanitise.Policy, rewriter func(string) string) *Processor {
	return &Processor{sanitiser: s, imgRewriter: rewriter}
}

// Process sanitises rawHTML, then (if a rewriter is configured) replaces
// every <img src> with the rewriter's output. Total function: never panics,
// never errors. Falls back to the sanitised-but-not-rewritten output if the
// post-pass parse fails.
func (p *Processor) Process(rawHTML string) string {
	cleaned := p.sanitiser.Sanitise(rawHTML)
	if p.imgRewriter == nil || cleaned == "" {
		return cleaned
	}
	out, err := rewriteImageURLs(cleaned, p.imgRewriter)
	if err != nil {
		slog.Error("processor.Process: rewrite failed; returning sanitised-only output", "err", err)
		return cleaned
	}
	return out
}

func rewriteImageURLs(s string, rewriter func(string) string) (string, error) {
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(s), body)
	if err != nil {
		return "", err
	}
	root := &html.Node{Type: html.ElementNode, Data: "root"}
	for _, n := range nodes {
		root.AppendChild(n)
	}
	walkAndRewrite(root, rewriter)

	var buf bytes.Buffer
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func walkAndRewrite(n *html.Node, rewriter func(string) string) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "img" {
			for i, a := range c.Attr {
				if a.Key == "src" {
					c.Attr[i].Val = rewriter(a.Val)
					break
				}
			}
		}
		walkAndRewrite(c, rewriter)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/processor -race`
Expected: PASS for all four tests.

- [ ] **Step 5: Commit**

```bash
git add internal/processor/processor.go internal/processor/processor_test.go
git commit -m "Add internal/processor: sanitise + image rewrite composition

The worker calls Process(html); Processor calls sanitise.Sanitise then
walks the cleaned HTML rewriting <img src>. <a href> and <iframe src>
are untouched. Total function — falls back to sanitised-only output on
post-pass failure.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: Refactor poll.SchedulerOpts / WorkerOpts: Policy → Processor

**Files:**
- Modify: `internal/poll/worker.go`
- Modify: `internal/poll/scheduler.go`
- Modify: `internal/poll/worker_test.go`
- Modify: `internal/poll/scheduler_test.go`

This task is mechanical but spans several files. Do it as one commit (the rename is atomic).

- [ ] **Step 1: Update worker.go**

Replace `internal/poll/worker.go` lines 1-42 with:

```go
package poll

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/feed"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/mmcdole/gofeed"
)

type WorkerOpts struct {
	Cadence   time.Duration        // fixed retry/next-poll interval for M1
	Processor *processor.Processor // applied to every entry's HTML body. Required (panics on nil).
}

type Worker struct {
	db     *sql.DB
	client *http.Client
	opts   WorkerOpts
}

// NewWorker requires a non-nil Processor. Defaulting it here would silently
// hide tests that forget to pass one — the public construction surface
// (Scheduler) supplies the production default.
func NewWorker(d *sql.DB, c *http.Client, o WorkerOpts) *Worker {
	if o.Processor == nil {
		panic("poll.NewWorker: Processor is required")
	}
	if o.Cadence <= 0 {
		o.Cadence = 30 * time.Minute
	}
	if c == nil {
		c = http.DefaultClient
	}
	return &Worker{db: d, client: c, opts: o}
}
```

Replace line 83 (the sanitise call) with:

```go
		content = w.opts.Processor.Process(content)
```

- [ ] **Step 2: Update scheduler.go**

Replace `internal/poll/scheduler.go` lines 13-15 (imports) with:

```go
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/bcrisp4/tap/internal/sanitise"
)
```

Replace `SchedulerOpts` (lines 17-22):

```go
type SchedulerOpts struct {
	TickInterval time.Duration        // default 60s
	Workers      int                  // default 3
	Cadence      time.Duration        // default 30m
	Processor    *processor.Processor // default processor.New(sanitise.DefaultPolicy(), nil)
}
```

Replace lines 57-66 (defaults block + worker construction). Find:

```go
	if o.Policy == nil {
		o.Policy = sanitise.DefaultPolicy()
	}
	parentCtx, parentCancel := context.WithCancel(base)
	s := &Scheduler{
		db:           d,
		client:       c,
		opts:         o,
		inflight:     NewInflight(),
		worker:       NewWorker(d, c, WorkerOpts{Cadence: o.Cadence, Policy: o.Policy}),
```

Replace with:

```go
	if o.Processor == nil {
		o.Processor = processor.New(sanitise.DefaultPolicy(), nil)
	}
	parentCtx, parentCancel := context.WithCancel(base)
	s := &Scheduler{
		db:           d,
		client:       c,
		opts:         o,
		inflight:     NewInflight(),
		worker:       NewWorker(d, c, WorkerOpts{Cadence: o.Cadence, Processor: o.Processor}),
```

- [ ] **Step 3: Update worker_test.go**

In `internal/poll/worker_test.go`, replace each occurrence of:

```go
		Policy:  sanitise.DefaultPolicy(),
```

with:

```go
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
```

There are three call sites: lines 70, 99, 123 (approximately).

Update the imports at the top of the file to include the processor package:

```go
import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/bcrisp4/tap/internal/sanitise"
	"github.com/stretchr/testify/require"
)
```

- [ ] **Step 4: Update scheduler_test.go**

Run: `grep -n "Policy:" internal/poll/scheduler_test.go`

Replace each `Policy: sanitise.DefaultPolicy()` with `Processor: processor.New(sanitise.DefaultPolicy(), nil)`. Add the processor import at the top of the file.

If there are no `Policy:` occurrences (the scheduler tests may not exercise the policy at all), no test changes are needed — only the production-code rename matters.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/poll -race`
Expected: PASS for all poll-package tests.

If `worker_test.go` still has `sanitise` import errors after the rename (the import becomes unused if Processor takes its own DefaultPolicy under the hood — it doesn't, sanitise is still imported), `goimports` will sort it out: `goimports -w internal/poll/`.

- [ ] **Step 6: Commit**

```bash
git add internal/poll/worker.go internal/poll/scheduler.go internal/poll/worker_test.go internal/poll/scheduler_test.go
git commit -m "Refactor poll: Policy → Processor on WorkerOpts/SchedulerOpts

Mechanical rename ahead of the M3 wiring. Worker now calls
Processor.Process instead of Policy.Sanitise; default Processor is
processor.New(sanitise.DefaultPolicy(), nil) — semantically identical
to the M2 default until main.go injects a rewriter.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: api.NewMux accepts proxy handler

**Files:**
- Modify: `internal/api/api.go`
- Modify: `internal/api/api_test.go`
- Modify: `internal/api/entries_test.go`
- Modify: `internal/api/subscriptions_test.go`

- [ ] **Step 1: Update api.go**

Replace `internal/api/api.go` with:

```go
package api

import (
	"database/sql"
	"net/http"
)

// NewMux returns the API mux. db is required for everything except /healthz.
// poke (optional) is called after a successful POST /api/v1/subscriptions so the
// scheduler can run an immediate tick instead of waiting for the next interval.
// proxyHandler (optional) is mounted at /api/v1/proxy/ when non-nil.
func NewMux(db *sql.DB, poke func(), proxyHandler http.Handler) *http.ServeMux {
	m := http.NewServeMux()

	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})

	if db != nil {
		registerSubscriptionRoutes(m, db, poke)
		registerEntryRoutes(m, db)
	}

	if proxyHandler != nil {
		// {token} is a Go 1.22+ ServeMux path placeholder; the handler
		// reads it via r.PathValue("token").
		m.Handle("GET /api/v1/proxy/{token}", proxyHandler)
	}

	return m
}
```

- [ ] **Step 2: Update all api.NewMux call sites**

Run: `grep -rn "api.NewMux(" internal/api/ cmd/`
Expected output: a list of call sites, e.g.:

```
internal/api/api_test.go:14:    mux := api.NewMux(d, nil)
internal/api/entries_test.go:N: mux := api.NewMux(d, nil)
internal/api/subscriptions_test.go:N: mux := api.NewMux(d, nil)
cmd/tap/main_test.go:46: mux := api.NewMux(d, nil)
cmd/tap/main.go:76: apiMux := api.NewMux(d, sched.Poke)
```

For every `api.NewMux(d, X)` call where the third arg is missing, append `, nil` (we'll wire the real handler in cmd/tap/main.go in Task 11). Do NOT modify cmd/tap/main.go yet — that's Task 11.

For the test files (`internal/api/*_test.go` and `cmd/tap/main_test.go`), each call becomes `api.NewMux(d, nil, nil)`.

- [ ] **Step 3: Run tests to verify they still pass**

Run: `go test ./internal/api ./cmd/tap -race`
Expected: PASS — the API tests are unaffected by the new param when nil.

If any test fails compilation: a call site was missed. Re-run the grep.

- [ ] **Step 4: Commit**

```bash
git add internal/api/api.go internal/api/api_test.go internal/api/entries_test.go internal/api/subscriptions_test.go cmd/tap/main_test.go
git commit -m "Add proxyHandler param to api.NewMux

Wires /api/v1/proxy/ as an optional route on the API mux. main.go will
pass a real handler in the next commit; tests pass nil.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 11: main.go wire-up

**Files:**
- Modify: `cmd/tap/main.go`

- [ ] **Step 1: Update main.go imports and flags**

Replace `cmd/tap/main.go` lines 1-28 (imports + flag block):

```go
// Package main is the Tap binary entry point.
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bcrisp4/tap/internal/api"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/poll"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/bcrisp4/tap/internal/sanitise"
	"github.com/bcrisp4/tap/internal/server"
)

func main() {
	var (
		addr             = flag.String("addr", "127.0.0.1:8080", "HTTP listen address (set 0.0.0.0:8080 in containers)")
		dataDir          = flag.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory containing tap.db")
		logFmt           = flag.String("log-format", "json", "log format: json or text")
		proxyCacheDir    = flag.String("proxy-cache-dir", envOr("TAP_PROXY_CACHE_DIR", ""), "media cache directory (default: <data>/cache)")
		proxyCacheCap    = flag.Int64("proxy-cache-cap-bytes", envOrInt64("TAP_PROXY_CACHE_CAP_BYTES", 524288000), "media cache size cap in bytes")
		proxyFetchTO     = flag.Duration("proxy-fetch-timeout", envOrDuration("TAP_PROXY_FETCH_TIMEOUT", 30*time.Second), "per-fetch deadline for media proxy origin requests")
		proxyBodyCap     = flag.Int64("proxy-body-cap-bytes", envOrInt64("TAP_PROXY_BODY_CAP_BYTES", 10485760), "per-response body cap for media proxy origin fetches")
	)
	flag.Parse()

	configureLogger(*logFmt)
```

(The trailing `)` and `flag.Parse()` close the existing block.)

- [ ] **Step 2: Add envOrInt64 and envOrDuration helpers**

Append to `cmd/tap/main.go` (near the bottom, alongside `envOr`):

```go
func envOrInt64(k string, def int64) int64 {
	if v := os.Getenv(k); v != "" {
		var n int64
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
		slog.Warn("invalid int64 env var; falling back to default", "key", k, "value", v, "default", def)
	}
	return def
}

func envOrDuration(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		slog.Warn("invalid duration env var; falling back to default", "key", k, "value", v, "default", def)
	}
	return def
}
```

- [ ] **Step 3: Replace the wire-up block**

Replace the original wire-up block (everything from the standalone `client := &http.Client{...}` declaration through `mux.Handle("/", server.SPAHandler())` — roughly lines 53–79 of the original M2 main.go) with the consolidated block below. The new block defines `client` once with the proxy fetch timeout, since the same client is shared between feed polls and proxy origin fetches.

```go
	// Resolve proxy cache dir: explicit flag wins; otherwise <dataDir>/cache.
	cacheDir := *proxyCacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(*dataDir, "cache")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		slog.Error("create cache dir", "path", cacheDir, "err", err)
		os.Exit(1)
	}

	// Bootstrap the proxy signing key from the configuration table.
	proxyKey, err := loadOrCreateProxyKey(ctx, d)
	if err != nil {
		slog.Error("load proxy signing key", "err", err)
		os.Exit(1)
	}

	signer := proxy.NewSigner(proxyKey)
	cache := proxy.NewCache(cacheDir, *proxyCacheCap)

	// HTTP client used for both feed polls and proxy origin fetches.
	// M4 will replace this with a shared SSRF-aware client.
	client := &http.Client{
		Timeout: *proxyFetchTO,
		Transport: &http.Transport{
			MaxIdleConns:        32,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	proxyHandler := proxy.NewHandler(signer, cache, client, *proxyBodyCap)

	proc := processor.New(sanitise.DefaultPolicy(), signer.RewriteImageURL)

	sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
		Processor: proc,
	})
	sched.Start()

	mux := http.NewServeMux()
	apiMux := api.NewMux(d, sched.Poke, proxyHandler)
	mux.Handle("/api/", apiMux)
	mux.Handle("/healthz", apiMux)
	mux.Handle("/", server.SPAHandler())
```

(Note: this removes the original standalone `client` declaration at lines 56-65; the merged block above defines it once with the proxy timeout.)

- [ ] **Step 4: Add the loadOrCreateProxyKey helper**

Append to `cmd/tap/main.go`:

```go
const proxySigningKeySize = 32

func loadOrCreateProxyKey(ctx context.Context, d *sql.DB) ([]byte, error) {
	if v, ok, err := db.GetConfig(ctx, d, "proxy.signing_key"); err != nil {
		return nil, fmt.Errorf("read signing key: %w", err)
	} else if ok && len(v) == proxySigningKeySize {
		return v, nil
	}

	key := make([]byte, proxySigningKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	got, err := db.SetConfigIfAbsent(ctx, d, "proxy.signing_key", key)
	if err != nil {
		return nil, err
	}
	return got, nil
}
```

Add `"database/sql"` to the import block.

- [ ] **Step 5: Build to verify**

Run: `go build ./cmd/tap`
Expected: clean build.

If "imported and not used" errors fire on `sanitise` or `processor`: re-check that the wire-up block uses both.

- [ ] **Step 6: Run all tests**

Run: `make test`
Expected: PASS (note `make test` rebuilds web/dist, which is required for the cmd/tap main_test.go to construct SPAHandler).

The end-to-end test in `cmd/tap/main_test.go` should still pass even though it doesn't exercise the new proxy path yet — that's Task 12.

- [ ] **Step 7: Commit**

```bash
git add cmd/tap/main.go
git commit -m "Wire up M3 media proxy in cmd/tap/main.go

Bootstrap signing key from configuration table on first launch (and
read it on subsequent launches). Build proxy.NewSigner / NewCache /
NewHandler; pass the signer's RewriteImageURL into a Processor that
wraps sanitise.DefaultPolicy(); thread the Processor through the
scheduler. The proxy handler mounts at /api/v1/proxy/ via api.NewMux.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 12: End-to-end test extension

**Files:**
- Modify: `cmd/tap/main_test.go`

- [ ] **Step 1: Update the existing test to construct the full mux**

The current test in `cmd/tap/main_test.go` constructs only `api.NewMux(d, nil, nil)`. Extend it so it also wires a real proxy handler against an `httptest.Server` origin. Add a new test alongside the existing one:

Append to `cmd/tap/main_test.go`:

```go
func TestEndToEnd_ProxyURLsRewriteAndServe(t *testing.T) {
	t.Parallel()

	// Origin server: returns a PNG fixture for any path.
	pngFixture := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	}
	var imageHits int32
	imageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&imageHits, 1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngFixture)
	}))
	defer imageSrv.Close()

	imageURL := imageSrv.URL + "/img.png"
	atom := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Imgs</title>
  <id>urn:imgs</id>
  <updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>HasImg</title>
    <id>urn:imgs:1</id>
    <link href="https://example.com/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;hi&lt;/p&gt;&lt;img src=&quot;` + imageURL + `&quot;&gt;</content>
  </entry>
</feed>`
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	defer feedSrv.Close()

	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(context.Background(), d))

	// Generate a signing key and insert into configuration (simulating bootstrap).
	key := []byte("0123456789abcdef0123456789abcdef")
	_, err = db.SetConfigIfAbsent(context.Background(), d, "proxy.signing_key", key)
	require.NoError(t, err)

	signer := proxy.NewSigner(key)
	cache := proxy.NewCache(t.TempDir(), 1<<20)
	proxyHandler := proxy.NewHandler(signer, cache, http.DefaultClient, 10<<20)

	mux := api.NewMux(d, nil, proxyHandler)

	// Subscribe.
	body := strings.NewReader(`{"feed_url":"` + feedSrv.URL + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	// Drive a poll with the production-shaped processor (rewriter wired in).
	proc := processor.New(sanitise.DefaultPolicy(), signer.RewriteImageURL)
	sched := poll.NewScheduler(context.Background(), d, http.DefaultClient, poll.SchedulerOpts{
		Workers:   1,
		Cadence:   time.Hour,
		Processor: proc,
	})
	defer sched.Stop()
	sched.Tick(context.Background())
	require.NoError(t, sched.Wait(5*time.Second))

	// Fetch the entry list, then the entry detail; the body must contain a proxy URL.
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/v1/entries", nil))
	require.Equal(t, http.StatusOK, rr2.Code)
	var listResp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&listResp))
	require.Len(t, listResp.Data, 1)

	entryID := int64(listResp.Data[0]["id"].(float64))
	rr3 := httptest.NewRecorder()
	mux.ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, "/api/v1/entries/"+strconv.FormatInt(entryID, 10), nil))
	require.Equal(t, http.StatusOK, rr3.Code, rr3.Body.String())
	var detail struct {
		Content string `json:"content"`
	}
	require.NoError(t, json.NewDecoder(rr3.Body).Decode(&detail))
	require.Contains(t, detail.Content, `src="/api/v1/proxy/`, "img src must be rewritten: %s", detail.Content)
	require.NotContains(t, detail.Content, imageURL, "raw origin URL must not appear: %s", detail.Content)

	// Extract the proxy URL from the body and GET it.
	const marker = `src="/api/v1/proxy/`
	idx := strings.Index(detail.Content, marker)
	require.GreaterOrEqual(t, idx, 0)
	end := strings.Index(detail.Content[idx+len(marker):], `"`)
	require.GreaterOrEqual(t, end, 0)
	proxyURL := "/api/v1/proxy/" + detail.Content[idx+len(marker):idx+len(marker)+end]

	rr4 := httptest.NewRecorder()
	mux.ServeHTTP(rr4, httptest.NewRequest(http.MethodGet, proxyURL, nil))
	require.Equal(t, http.StatusOK, rr4.Code, rr4.Body.String())
	require.Equal(t, "image/png", rr4.Header().Get("Content-Type"))
	require.Equal(t, "public, max-age=31536000, immutable", rr4.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", rr4.Header().Get("X-Content-Type-Options"))
	require.Equal(t, pngFixture, rr4.Body.Bytes())
	require.Equal(t, int32(1), atomic.LoadInt32(&imageHits))

	// Second request: served from cache, no new origin hit.
	rr5 := httptest.NewRecorder()
	mux.ServeHTTP(rr5, httptest.NewRequest(http.MethodGet, proxyURL, nil))
	require.Equal(t, http.StatusOK, rr5.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&imageHits), "second request must hit cache")

	// Restart simulation: reload the signing key from the DB and rebuild the
	// signer + handler. The previously-issued token must still verify.
	keyAgain, ok, err := db.GetConfig(context.Background(), d, "proxy.signing_key")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, key, keyAgain)

	signer2 := proxy.NewSigner(keyAgain)
	tok := strings.TrimPrefix(proxyURL, "/api/v1/proxy/")
	gotURL, ok := signer2.Verify(tok)
	require.True(t, ok)
	require.Equal(t, imageURL, gotURL)
}
```

Add new imports to the file:

```go
	"strconv"  // already present, keep
	"strings"  // already present
	"sync/atomic"

	"github.com/bcrisp4/tap/internal/processor"
	"github.com/bcrisp4/tap/internal/proxy"
```

Also update the existing `TestEndToEnd_SubscribePollServeEntries`: replace `Policy: sanitise.DefaultPolicy()` with `Processor: processor.New(sanitise.DefaultPolicy(), nil)` (so the existing test compiles after Task 9's rename).

- [ ] **Step 2: Run the test**

Run: `make test`
(Yes, `make test` and not just `go test` — the embed directive requires `web/dist` and `make test` builds it.)
Expected: PASS for the new test plus all existing ones.

If `TestEndToEnd_ProxyURLsRewriteAndServe` fails with `img src must be rewritten`: the production wiring isn't producing proxy URLs. Re-check that the test constructs the processor with `signer.RewriteImageURL`, not `nil`.

- [ ] **Step 3: Commit**

```bash
git add cmd/tap/main_test.go
git commit -m "Add end-to-end test for the M3 proxy round trip

Subscribes to a feed whose entry contains an <img>; polls; asserts the
sanitised body contains /api/v1/proxy/<token>; GETs the proxy URL and
asserts content + headers + cache hit on second request; verifies the
signing key persists and an old token verifies after a simulated restart.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 13: README update

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Read current README and locate the M2 trust-posture section**

Run: `grep -n -A 5 "Trust posture" README.md`

If the M2 section is structured as a single paragraph, replace it with the M3 expansion. If it's structured as numbered/bulleted points, follow the same shape.

- [ ] **Step 2: Edit the trust-posture section**

The replacement block (preserve the surrounding heading style):

> **Trust posture.** Feed HTML is sanitised on the server before storage; the SPA renders the stored HTML directly without a runtime sanitiser. Article images are fetched through Tap's media proxy: origin sites see only Tap's IP, and mixed-content image URLs work even when Tap is served over HTTPS. The loopback-default bind remains as defence-in-depth (concept §6.11).

- [ ] **Step 3: Add operations note for the cache directory**

In whichever section documents `${TAP_DATA_DIR}` and the `data/` layout, add:

> Tap also writes a media cache under `${TAP_DATA_DIR}/cache/`. The default cap is 500 MiB; tune via `--proxy-cache-cap-bytes` or `TAP_PROXY_CACHE_CAP_BYTES`. Safe to delete at any time — the next request re-fetches.

- [ ] **Step 4: Add the M2 → M3 upgrade note**

Wherever the M1 → M2 upgrade note lives, append:

> **M2 → M3.** No destructive change required. M3 adds the `configuration` table (one row, the proxy signing key) on next start. Existing entries keep their direct `<img src="origin">` URLs and won't be retroactively proxied; new entries get proxied URLs. To proxy all entries' images, delete `tap.db` (binary deployment) or the `/data` volume (container deployment) and re-subscribe.

- [ ] **Step 5: Verify build smoke**

Run: `make build`
Expected: `bin/tap` exists, no errors.

- [ ] **Step 6: Boot the binary against a fresh data dir**

```bash
rm -rf /tmp/tap-m3-smoke && mkdir -p /tmp/tap-m3-smoke
TAP_DATA_DIR=/tmp/tap-m3-smoke ./bin/tap -addr 127.0.0.1:8089 &
sleep 2
curl -sf http://127.0.0.1:8089/healthz
kill %1
```

Expected: `healthz` returns `ok`. The data dir contains `tap.db` and `cache/`.

- [ ] **Step 7: Commit**

```bash
git add README.md
git commit -m "Update README for M3: media proxy + cache

Trust posture extended; cache directory documented; M2 → M3 upgrade
note added (no destructive migration required).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 14: Final verification

- [ ] **Step 1: Verify internal/sanitise has no diff**

Run: `git log -p main..HEAD -- internal/sanitise internal/urlcleaner | wc -l`
Expected: `0`. Any non-zero output means M2 sanitisation behaviour was modified — investigate before merging.

- [ ] **Step 2: Run the full test suite**

Run: `make test`
Expected: PASS, no race detector warnings.

- [ ] **Step 3: Build the binary**

Run: `make build`
Expected: `bin/tap` exists.

- [ ] **Step 4: Build the docker image**

Run: `make docker`
Expected: image builds; under 30 MB.

- [ ] **Step 5: Manual smoke against a real feed**

Subscribe to a real feed with image-bearing entries (e.g. The Verge RSS) via the SPA. Open an entry. Inspect the network tab: image requests should go to `/api/v1/proxy/<token>` and return 200 with `Cache-Control: immutable`.

- [ ] **Step 6: Inspect the cache directory**

Run: `ls data/cache/*/* | head -20`
Expected: pairs of `<hash>.bin` and `<hash>.meta` files in two-character bucket directories.

- [ ] **Step 7: Verify the spec's definition of done**

Walk through the 10 items in the spec's "Definition of done" section one by one. Every item should be true. If any item is not, file a follow-up task.

- [ ] **Step 8: No commit**

This task verifies the work; it doesn't add code.

---

## Self-review notes

- **Spec coverage.** Each in-scope spec section maps to a task: configuration table & helpers (T1), Signer (T2), MIME allowlist (T3), Cache layout / sidecar / atomic writes (T4), singleflight (T5), LRU eviction (T6), Handler (T7), Processor (T8), worker integration (T9), api wire-up (T10), main.go wire-up (T11), end-to-end test (T12), README (T13), verification (T14).
- **Spec out-of-scope items not covered (correct):** SSRF, per-host concurrency cap, daily age-based eviction, SVG, conditional GET, negative caching, key rotation, parseSize helper.
- **Risk: Task 5's flake tolerance.** The singleflight test allows `calls <= 2` to absorb the registration-window race that singleflight has. If 1-of-N rather than at-most-2 ever matters as a stronger guarantee, swap to a manual deduplication test (channel-based) — but for M3's purposes, "the property holds in practice" is enough.
- **Risk: Task 6's mtime resolution.** The eviction test uses `os.Chtimes` to backdate the first file by 2 seconds, which is robust against second-resolution mtime filesystems. Sub-second precision isn't relied upon.
- **Risk: Task 11's flag plumbing.** `envOrInt64` and `envOrDuration` are tiny; if either misbehaves, the flag default still holds (they only override defaults when env is set and parses cleanly). Worst case: a misconfigured env is silently ignored with a warning log.
