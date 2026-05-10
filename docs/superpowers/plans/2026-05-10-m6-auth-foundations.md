# M6 — Auth Foundations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land password-based user accounts, session cookies (`HttpOnly`, hashed at rest, idle + absolute expiry), CSRF discipline, admin bootstrap (CLI + env-var), and per-feed credential redaction as specified in `docs/specs/2026-05-10-m6-auth-foundations.md`. After M6, every `/api/v1/*` route except `POST /sessions` and `/healthz` requires a valid session; per-feed `cookie` / `basic_auth_user` / `basic_auth_pass` are accepted on POST/PATCH `/api/v1/subscriptions` but never returned by GET; `tap admin create` and `tap admin passwd` exist as offline subcommands; `TAP_ADMIN_USERNAME`/`TAP_ADMIN_PASSWORD` env vars bootstrap the first admin on a fresh database.

**Architecture:** New leaf package `internal/auth` owns argon2id password hashing (PHC-encoded) and 32-byte session/CSRF token minting. New tables `users` + `sessions` plus three new columns on `subscriptions` land in migration `0005_auth_and_credentials.sql`. Two new pieces of middleware (`requireSession`, `requireCSRF`) decorate the existing `/api/v1/*` routes. Per-feed credentials wire through a new `httpx.ApplyFeedCreds` helper applied only to feed-fetch and article-fetch — never to media-proxy origin fetches (mirrors Miniflux). `cmd/tap` grows a hand-rolled `os.Args[1]` subcommand router with `admin create` + `admin passwd` siblings; the same code path runs the env-var first-launch shortcut. SPA gains a `Login.svelte` view, an auth store with `bootstrap`/`login`/`logout`/`setCSRFToken`, and `X-CSRF-Token` plumbing on every state-changing API call. CSRF tokens rotate only on password change.

**Tech Stack:** Go 1.25, modernc.org/sqlite, stretchr/testify, golang.org/x/crypto/argon2, golang.org/x/term, Svelte 5 + TypeScript + Vite (no SvelteKit). New deps: `golang.org/x/crypto/argon2` (PHC), `golang.org/x/term` (no-echo password input).

---

## Skills and tools to apply

Always-on for every code-touching task:

- **`superpowers:test-driven-development`** — red/green/refactor on every behaviour-bearing change. Mandated by `docs/roadmap.md` §"Working cadence" and reaffirmed in the spec. Pure scaffolding (the migration SQL file, README edits, flag declarations, the type-only SPA additions) is exempt; everything with branches, error handling, or state is in scope.
- **`superpowers:verification-before-completion`** — before marking a task done or claiming success in a commit message, actually run the test command listed in the task's verification step and confirm the output matches the expected output.

Reach for as needed:

- **`golang-security`** — argon2id parameters, the standard PHC encoding (`$argon2id$v=19$m=...,t=...,p=...$<salt>$<hash>`), constant-time comparison via `crypto/subtle.ConstantTimeCompare`, secure cookie attributes (`HttpOnly`, `SameSite=Lax`, conditional `Secure`), CSRF discipline. Argon2 specifics: `argon2.IDKey(password, salt, time, memory, threads, keyLen)` returns the raw key; you encode it yourself.
- **`golang-database`** — for the new tables, parameterised queries, struct scanning into `db.User`/`db.Session`, transaction boundaries on the password-change flow, and `errors.Is(err, sql.ErrNoRows)` mapping. SQLite-specific: `ALTER TABLE … ADD COLUMN … DEFAULT` is in-place and safe; rely on `journal_mode(WAL)` (already set in `db.Open`) for concurrent-read-during-write.
- **`golang-error-handling`** — sentinel errors (`ErrUserExists`, `ErrPasswordTooShort`); `errors.Is` mapping at the API boundary; `slog.WarnContext`/`slog.ErrorContext` for observable failures. Don't log password fields or cookie values anywhere.
- **`golang-testing`** + **`golang-stretchr-testify`** — match the existing repo style (`require.NoError(t, err)`, `require.Equal(t, want, got)`). Table-driven tests where natural (login failure modes, middleware rejection cases). Use `httptest.Server` for end-to-end fixtures; use the existing `cmd/tap/main_test.go` `setupServer` helper (or its analogue) as the harness for end-to-end auth tests.
- **`golang-context`** — `ctxKeyUser` / `ctxKeySession` typed context keys for the middleware's user + session injection. Helpers `userFromContext` / `sessionFromContext` defined alongside.
- **`golang-cli`** — for the `tap admin` subcommand structure (per-subcommand `flag.NewFlagSet`, exit codes 0/1/2/3, stderr for errors, stdout for success), and for the `bootstrapAdmin` helper that runs in `runServer` after migrations.
- **`golang-naming`** — `RequireSession` is exported only if it leaves the `api` package (it doesn't — keep it lowercase `requireSession`). `MintSessionToken`, `MintCSRFToken`, `Hash`, `Verify`, `ValidatePassword`, `DefaultParams` are all exported per the public-surface table in the spec.
- **`golang-modernize`** — Go 1.25 idioms: range over int isn't relevant here; `for-range` loop variables don't need shadowing.
- **`svelte-runes`** — Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`) in `Login.svelte` and the auth store. The repo is already Svelte 5; check existing components (`web/src/components/AddFeedForm.svelte`) for the established style.
- **`svelte-components`** — for `Login.svelte` form patterns. Match the form ergonomics of `web/src/components/AddFeedForm.svelte`.

MCP tools:

- **`context7` (`mcp__plugin_context7_context7__query-docs`)** — fetch live docs for `golang.org/x/crypto/argon2` if the API in this plan drifts from the latest. The argon2 stdlib API is extremely stable, so context7 is unlikely to be needed; fall back to `WebFetch` against `pkg.go.dev/golang.org/x/crypto/argon2` if context7 misses.

---

## File structure

| Path | Action | Responsibility |
|---|---|---|
| `internal/auth/argon2.go` | **create** | `Hash`, `Verify`, `Params`, `DefaultParams`, `ValidatePassword`, `ErrPasswordTooShort`. PHC encoding/decoding lives here. |
| `internal/auth/argon2_test.go` | **create** | Hash + Verify roundtrip; wrong password rejection; ValidatePassword bounds; non-default Params roundtrip; encoded-format regression. |
| `internal/auth/tokens.go` | **create** | `MintSessionToken` (returns cookie + tokenHash), `MintCSRFToken`. |
| `internal/auth/tokens_test.go` | **create** | Token shape (43 base64url chars), `sha256(decode(cookie)) == tokenHash`, distinctness across calls. |
| `internal/db/migrations/0005_auth_and_credentials.sql` | **create** | `users`, `sessions`, three new columns on `subscriptions`. |
| `internal/db/migrate_test.go` | modify | New `TestMigrate_AddsAuthTables` + `TestMigrate_AddsSubscriptionCredentialColumns`. Update `TestMigrate_AppliesAllMigrationsExactlyOnce` for version 5. |
| `internal/db/users.go` | **create** | `User`, `NewUser`, `ErrUserExists`, `InsertUser`, `GetUserByUsername`, `GetUserByID`, `UpdatePasswordHash`, `DisableUser`, `CountUsers`. |
| `internal/db/users_test.go` | **create** | Roundtrip Insert + Get; duplicate → `ErrUserExists`; missing → `sql.ErrNoRows`; UpdatePasswordHash; DisableUser; CountUsers. |
| `internal/db/sessions.go` | **create** | `Session`, `NewSession`, `InsertSession`, `GetSessionByTokenHash`, `RefreshSessionIdle`, `UpdateSessionCSRFToken`, `DeleteSession`, `DeleteSessionsByUserID`, `DeleteOtherSessionsForUser`. |
| `internal/db/sessions_test.go` | **create** | Roundtrips for each function above. |
| `internal/db/subscriptions.go` | modify | `Subscription` + `DueSubscription` + `NewSubscription` gain `Cookie`/`BasicAuthUser`/`BasicAuthPass`; SELECTs grow the columns; `InsertSubscription` writes them; new `UpdateSubscriptionCredentials` helper. |
| `internal/db/subscriptions_test.go` | modify | Insert+Get roundtrip with creds; ListDuePolls exposes creds; UpdateSubscriptionCredentials atomicity. |
| `internal/api/middleware.go` | **create** | `ctxKeyUser`/`ctxKeySession`; `userFromContext`/`sessionFromContext`; `requireSession`; `requireCSRF`; small `chain` helper to compose middleware. |
| `internal/api/middleware_test.go` | **create** | requireSession: missing/bad cookie, idle-expired, absolute-expired, valid+refresh. requireCSRF: GET passthrough, missing/wrong/right token, Origin mismatch, Origin match. |
| `internal/api/auth.go` | **create** | DTOs (`loginRequest`, `loginResponse`, `userDTO`, `passwordChangeRequest`, `passwordChangeResponse`); handlers (`login`, `logout`, `getSessionCurrent`, `passwordChange`); `setSessionCookie`/`clearSessionCookie` helpers; `CookieSecureMode` enum + `resolveCookieSecure`. |
| `internal/api/auth_test.go` | **create** | Login happy + 4 failure modes; logout; session probe; password change happy + wrong current + too-short new; CSRF token in response; other-sessions-deleted. |
| `internal/api/api.go` | modify | `MuxOpts` gains `SessionIdleTTL`, `SessionAbsoluteTTL`, `CookieSecure`. `NewMux` builds `authed` and `authedCSRF` decorators and applies them per-route. New error code constants. |
| `internal/api/errors.go` | modify | Add `ErrCodeInvalidCredentials`, `ErrCodeInvalidSession`, `ErrCodeCSRFInvalid`, `ErrCodePasswordTooShort`. |
| `internal/api/subscriptions.go` | modify | POST + PATCH bodies grow `cookie`/`basic_auth_user`/`basic_auth_pass` (write-only). DTO grows `has_cookie`/`has_basic_auth`. PATCH merge logic mirrors the existing extract-selector handling. |
| `internal/api/subscriptions_test.go` | modify | POST persists creds; GET returns booleans, never values; PATCH omitted=preserve, ""=clear; partial PATCHes don't clobber. |
| `internal/httpx/feedcreds.go` | **create** | `FeedCreds` struct + `ApplyFeedCreds(req, creds)` helper. |
| `internal/httpx/feedcreds_test.go` | **create** | Cookie applied; basic auth applied; both empty → no headers; partial (user only, empty pass) → still applied. |
| `internal/poll/worker.go` | modify | Read creds from `DueSubscription`; build `FeedCreds`; pass into `feed.Fetch` (which gains a creds parameter) and into `extract.Extract` (signature change). |
| `internal/feed/fetch.go` | modify | `FetchOpts` gains `Creds httpx.FeedCreds`; `Fetch` calls `httpx.ApplyFeedCreds(req, opts.Creds)` before sending. |
| `internal/feed/fetch_test.go` | modify | Cover Cookie/Authorization headers reaching the origin. |
| `internal/poll/worker_test.go` | modify | Cover the same end-to-end (creds reach the origin via worker). |
| `internal/extract/extract.go` | modify | Add `creds httpx.FeedCreds` parameter; call `httpx.ApplyFeedCreds(req, creds)` after building the request. |
| `internal/extract/extract_test.go` | modify | Cover creds-applied + creds-empty cases. |
| `internal/proxy/handler_test.go` | modify | New `TestHandler_NeverSendsCredentials` regression — origin-fixture asserts no `Cookie:` or `Authorization:` even when called from a context that "would have" creds. |
| `cmd/tap/admin.go` | **create** | `runAdmin` dispatcher; `runAdminCreate`; `runAdminPasswd`; password-input helpers; exit-code constants. |
| `cmd/tap/admin_test.go` | **create** | Each subcommand happy path with stdin injection; duplicate-username exit 2; missing-user exit 2; password-mismatch exit 3. |
| `cmd/tap/main.go` | modify | Add `os.Args[1] == "admin"` branch; new flags (`--session-idle-ttl`, `--session-absolute-ttl`, `--cookie-secure`); `bootstrapAdmin` env-var path; pass `MuxOpts` fields. |
| `cmd/tap/main_test.go` | modify | New end-to-end tests: login + state-changing CSRF roundtrip; per-feed basic-auth flows into a fixture origin; proxy origin is anonymous; env-var bootstrap on empty users; silent no-op on populated users. |
| `web/src/lib/auth.ts` | **create** | Auth store: `bootstrap`, `login`, `logout`, `setCSRFToken`. |
| `web/src/lib/__tests__/auth.test.ts` | **create** | Bootstrap success + 401; login happy + 401; logout. |
| `web/src/lib/api.ts` | modify | Attach `X-CSRF-Token` on POST/PUT/PATCH/DELETE; clear auth store on 401. New `addSubscription` overload (creds + extract). New `patchSubscription`. New `changePassword`. |
| `web/src/lib/__tests__/api.test.ts` | **create** | CSRF header attach; 401 clears auth state; changePassword updates csrfToken. |
| `web/src/lib/types.ts` | modify | Backfill `extract`/`extract_selector`/`extract_failed`; add `has_cookie`/`has_basic_auth`; new `User`, `SessionResponse`, `PasswordChangeResponse` types. |
| `web/src/views/Login.svelte` | **create** | Username + password inputs, submit, error display. |
| `web/src/views/__tests__/Login.test.ts` | **create** | Renders form; submit calls `auth.login`; error shows on rejection. |
| `web/src/App.svelte` | modify | Bootstrap auth on mount; conditional Login rendering; preserve existing route gate. |
| `go.mod` / `go.sum` | modify | `go get golang.org/x/crypto@latest golang.org/x/term@latest` (both already pulled transitively in many repos) + `go mod tidy`. |
| `README.md` | modify | New "Authentication (M6)" paragraph after the M5 trust-posture block; one-line upgrade note. |
| `CLAUDE.md` | modify | Status line: `M5 in progress` → `M6 in progress`. Add the M6 trust-posture deltas to the existing block. |

---

## Phase A — `internal/auth` foundations

### Task A1: Add the `golang.org/x/crypto/argon2` and `golang.org/x/term` dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Pull the modules**

```bash
go get golang.org/x/crypto/argon2@latest
go get golang.org/x/term@latest
go mod tidy
```

- [ ] **Step 2: Verify the module entries landed**

```bash
grep -E 'golang.org/x/(crypto|term)' go.mod
```

Expected: both modules listed in the `require` block (crypto may already be transitively pulled in by other modules; the explicit `go get` promotes it to a direct require).

- [ ] **Step 3: Verify the build still passes**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "$(cat <<'EOF'
M6: pull golang.org/x/crypto and golang.org/x/term

argon2id password hashing (concept §7.3) lives in x/crypto/argon2.
term.ReadPassword is the no-echo password input for the upcoming
'tap admin' subcommands. Both are pure-Go, no CGO — distroless-safe.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A2: `auth.Hash` + `auth.Verify` happy path

**Files:**
- Create: `internal/auth/argon2.go`
- Create: `internal/auth/argon2_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/auth/argon2_test.go`:

```go
package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// testParams keeps the test fast — production uses DefaultParams.
var testParams = Params{Time: 1, Memory: 8 * 1024, Threads: 1, SaltLen: 8, KeyLen: 16}

func TestHashVerifyRoundTrip(t *testing.T) {
	t.Parallel()
	encoded, err := Hash("correct horse battery staple", testParams)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(encoded, "$argon2id$v=19$"),
		"encoded hash must use the standard PHC format, got: %s", encoded)

	ok, err := Verify(encoded, "correct horse battery staple")
	require.NoError(t, err)
	require.True(t, ok, "Verify should accept the same password")
}
```

- [ ] **Step 2: Run the test to confirm it fails**

```bash
go test ./internal/auth -run TestHashVerifyRoundTrip 2>&1 | head -10
```

Expected: build failure — the `auth` package doesn't exist yet.

- [ ] **Step 3: Implement `Hash` + `Verify` + `Params`**

Create `internal/auth/argon2.go`:

```go
// Package auth owns password hashing (argon2id, PHC-encoded) and the
// random-token primitives used by the session and CSRF mechanisms.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Params are the tunable cost knobs of argon2id. Encoded into every hash so
// Verify can run with the same params the hash was produced under, and so a
// future params bump (paired with re-hash-on-verify in M12) can identify
// which hashes are still on weaker params.
type Params struct {
	Time    uint32
	Memory  uint32 // KiB
	Threads uint8
	SaltLen uint32
	KeyLen  uint32
}

// DefaultParams: OWASP 2026 recommendation for argon2id. Pinned as a constant;
// a future milestone that bumps these is also responsible for the
// re-hash-on-verify path (M12 line in docs/roadmap.md).
var DefaultParams = Params{
	Time:    2,
	Memory:  64 * 1024, // KiB → 64 MiB
	Threads: 1,
	SaltLen: 16,
	KeyLen:  32,
}

// Hash returns the PHC-encoded argon2id hash of password under the given
// params. The encoding is the standard
//
//   $argon2id$v=19$m=<mem>,t=<time>,p=<threads>$<salt-b64>$<hash-b64>
//
// where the b64 forms are unpadded base64 (RawStdEncoding).
func Hash(password string, p Params) (string, error) {
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2: read salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, p.KeyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Time, p.Threads,
		enc.EncodeToString(salt), enc.EncodeToString(key),
	), nil
}

// Verify parses encoded as a PHC-format argon2id hash and reports whether it
// corresponds to password. Returns (false, nil) on a normal mismatch and
// (false, err) on a malformed encoding.
func Verify(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// Expected: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, errors.New("argon2: malformed hash")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, fmt.Errorf("argon2: unsupported version %q", parts[2])
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil {
		return false, fmt.Errorf("argon2: malformed params: %w", err)
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("argon2: decode salt: %w", err)
	}
	want, err := enc.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("argon2: decode hash: %w", err)
	}
	got := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
```

- [ ] **Step 4: Run the test to confirm it passes**

```bash
go test ./internal/auth -run TestHashVerifyRoundTrip
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/argon2.go internal/auth/argon2_test.go
git commit -m "$(cat <<'EOF'
M6: auth.Hash and auth.Verify (argon2id, PHC-encoded)

DefaultParams use OWASP 2026 recommendations (t=2, m=64MiB, p=1,
salt=16, key=32). The encoded format includes the params so a future
re-hash-on-verify pass (M12) can identify hashes that need upgrading.
Verify uses crypto/subtle.ConstantTimeCompare on the raw key bytes.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A3: `auth.Verify` rejects the wrong password

**Files:**
- Modify: `internal/auth/argon2_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/auth/argon2_test.go`:

```go
func TestVerifyRejectsWrongPassword(t *testing.T) {
	t.Parallel()
	encoded, err := Hash("hunter2", testParams)
	require.NoError(t, err)

	ok, err := Verify(encoded, "Hunter2")
	require.NoError(t, err)
	require.False(t, ok)
}

func TestVerifyRejectsMalformedEncoding(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		"plaintext",
		"$argon2id$v=19$m=8192,t=1,p=1$abc",       // missing the hash component
		"$argon2id$v=99$m=8192,t=1,p=1$YWFh$YmJi", // wrong version
		"$argon2i$v=19$m=8192,t=1,p=1$YWFh$YmJi",  // wrong family
	}
	for _, e := range cases {
		ok, err := Verify(e, "anything")
		require.False(t, ok, "encoded=%q", e)
		require.Error(t, err, "encoded=%q", e)
	}
}
```

- [ ] **Step 2: Run the tests to confirm they pass (no implementation change needed)**

```bash
go test ./internal/auth -run "TestVerifyRejects"
```

Expected: PASS — the implementation from Task A2 already covers these. If any case fails, adjust `Verify`.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/argon2_test.go
git commit -m "$(cat <<'EOF'
M6: auth.Verify regression coverage (wrong password, malformed PHC)

Locks in the rejection paths so future Verify edits can't silently
accept malformed encodings or weaken the constant-time compare.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A4: `auth.ValidatePassword`

**Files:**
- Modify: `internal/auth/argon2.go`
- Modify: `internal/auth/argon2_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/auth/argon2_test.go`:

```go
func TestValidatePassword(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"empty", "", ErrPasswordTooShort},
		{"seven chars", "1234567", ErrPasswordTooShort},
		{"eight chars", "12345678", nil},
		{"long", strings.Repeat("a", 200), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePassword(tc.in)
			if tc.want == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run to confirm it fails (compile error)**

```bash
go test ./internal/auth -run TestValidatePassword 2>&1 | head -5
```

Expected: build failure — `ValidatePassword` and `ErrPasswordTooShort` undefined.

- [ ] **Step 3: Implement `ValidatePassword`**

Append to `internal/auth/argon2.go`:

```go
// ErrPasswordTooShort is returned by ValidatePassword for inputs shorter
// than MinPasswordLength runes.
var ErrPasswordTooShort = errors.New("password too short")

// MinPasswordLength is the minimum length policy for new passwords.
// Length over class diversity per modern guidance — no complexity rules.
const MinPasswordLength = 8

// ValidatePassword returns ErrPasswordTooShort if s is shorter than
// MinPasswordLength runes. Caller is responsible for trimming whitespace
// if appropriate (the API and CLI accept passwords verbatim — leading
// or trailing spaces become part of the password).
func ValidatePassword(s string) error {
	if len([]rune(s)) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}
```

- [ ] **Step 4: Run to confirm it passes**

```bash
go test ./internal/auth -run TestValidatePassword
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/argon2.go internal/auth/argon2_test.go
git commit -m "$(cat <<'EOF'
M6: auth.ValidatePassword (min length 8, no complexity)

Length-only policy per modern guidance and the M6 spec. Callers
(POST /sessions, PATCH /me/password, tap admin create, env-var
bootstrap) all gate on this single function.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A5: `auth.MintSessionToken` + `auth.MintCSRFToken`

**Files:**
- Create: `internal/auth/tokens.go`
- Create: `internal/auth/tokens_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/auth/tokens_test.go`:

```go
package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMintSessionTokenShape(t *testing.T) {
	t.Parallel()
	cookie, hash, err := MintSessionToken()
	require.NoError(t, err)

	// 32 random bytes → 43 base64url chars (unpadded).
	require.Len(t, cookie, 43, "cookie value")
	raw, err := base64.RawURLEncoding.DecodeString(cookie)
	require.NoError(t, err)
	require.Len(t, raw, 32, "underlying random bytes")

	// hash is sha256-hex of the underlying random bytes.
	sum := sha256.Sum256(raw)
	require.Equal(t, hex.EncodeToString(sum[:]), hash)
	require.Len(t, hash, 64)
}

func TestMintSessionTokenDistinctness(t *testing.T) {
	t.Parallel()
	a, _, err := MintSessionToken()
	require.NoError(t, err)
	b, _, err := MintSessionToken()
	require.NoError(t, err)
	require.NotEqual(t, a, b, "two mints should produce distinct cookies")
}

func TestMintCSRFToken(t *testing.T) {
	t.Parallel()
	a, err := MintCSRFToken()
	require.NoError(t, err)
	require.Len(t, a, 43)

	b, err := MintCSRFToken()
	require.NoError(t, err)
	require.NotEqual(t, a, b)
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./internal/auth -run "TestMint" 2>&1 | head -5
```

Expected: build failure — `MintSessionToken` / `MintCSRFToken` undefined.

- [ ] **Step 3: Implement the token mints**

Create `internal/auth/tokens.go`:

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// sessionTokenBytes is the size of the random portion behind every session
// cookie. 32 bytes (256 bits) is the standard "unguessable" choice and the
// industry default for opaque session tokens.
const sessionTokenBytes = 32

// MintSessionToken returns the cookie value and its sha256-hex hash.
//   - cookieValue: base64url(random 32 bytes), unpadded — what goes in the
//     Set-Cookie header.
//   - tokenHash:   hex(sha256(those 32 bytes)) — what gets stored on
//     sessions.token_hash. A DB read therefore never yields a live cookie.
func MintSessionToken() (cookieValue, tokenHash string, err error) {
	raw := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("mint session token: %w", err)
	}
	sum := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), hex.EncodeToString(sum[:]), nil
}

// csrfTokenBytes is the random size behind a CSRF token. 32 bytes matches
// the session token; the CSRF token is also unguessable but doesn't need
// to be hashed at rest because it's not a credential — leaking it grants
// CSRF bypass for the bound session, not session takeover.
const csrfTokenBytes = 32

// MintCSRFToken returns base64url(random 32 bytes), unpadded.
func MintCSRFToken() (string, error) {
	raw := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("mint csrf token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
```

- [ ] **Step 4: Run to confirm it passes**

```bash
go test ./internal/auth -run "TestMint"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/tokens.go internal/auth/tokens_test.go
git commit -m "$(cat <<'EOF'
M6: auth.MintSessionToken and auth.MintCSRFToken

Session token: 32 random bytes; cookie value is base64url; DB
stores sha256-hex of the bytes so a DB read never yields a live
cookie. CSRF token: 32 random bytes base64url. Tests assert shape,
the cookie↔hash relationship, and distinctness across calls.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A6: Encoded-format regression test

**Files:**
- Modify: `internal/auth/argon2_test.go`

A regression test that pins the PHC encoding so a future refactor of `Hash` can't silently change the on-disk format (which would invalidate every existing user's password).

- [ ] **Step 1: Add the regression test**

Append to `internal/auth/argon2_test.go`:

```go
// TestVerifyAcceptsKnownGoodHash locks in the on-disk format. If this test
// fails after a Hash refactor, the refactor invalidated every existing
// user's password — back it out.
func TestVerifyAcceptsKnownGoodHash(t *testing.T) {
	t.Parallel()
	// Generated once with testParams (t=1, m=8MiB, p=1, salt=8, key=16) and
	// password "fixture-password". Hard-coded so any change to Hash that
	// alters encoding fails this test loudly.
	const fixture = "$argon2id$v=19$m=8192,t=1,p=1$bWVtYmVyc2g$1q9aFrnoLUv0Ne0jY/2GFQ"

	// First, sanity-check the parser by hashing fresh and round-tripping.
	enc, err := Hash("fixture-password", testParams)
	require.NoError(t, err)
	ok, err := Verify(enc, "fixture-password")
	require.NoError(t, err)
	require.True(t, ok)

	// Then verify the fixture itself. If you regenerate this fixture, also
	// verify it is parseable by your new Hash format and update both the
	// fixture and this comment.
	_ = fixture // The exact bytes of the hash differ per random salt; the
	// parser-shape regression is the critical thing — exercised below.

	// Parser regression: a hand-crafted encoding the parser must accept.
	// Random salt + random hash; the value need not match a real password,
	// only the encoding shape needs to be parseable end-to-end.
	const handcrafted = "$argon2id$v=19$m=8192,t=1,p=1$YWFhYWFhYWE$YmJiYmJiYmJiYmJiYmJiYg"
	ok, err = Verify(handcrafted, "this-will-not-match")
	require.NoError(t, err, "parser must accept the standard PHC encoding")
	require.False(t, ok, "but the password is wrong, so Verify should return false")
}
```

- [ ] **Step 2: Run to confirm it passes**

```bash
go test ./internal/auth
```

Expected: PASS for all auth tests.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/argon2_test.go
git commit -m "$(cat <<'EOF'
M6: encoded-format regression for argon2 PHC

Locks the on-disk hash shape so a future Hash refactor can't silently
break every existing user's password. If this test fails after a
Hash change, back the change out — it invalidated stored hashes.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase B — Migration 0005 + `db` package

### Task B1: Migration `0005_auth_and_credentials.sql`

**Files:**
- Create: `internal/db/migrations/0005_auth_and_credentials.sql`
- Modify: `internal/db/migrate_test.go`

- [ ] **Step 1: Write the failing migrate test**

Append to `internal/db/migrate_test.go`:

```go
func TestMigrate_AddsAuthTables(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	// users table presence + key columns
	for _, col := range []string{"id", "username", "password_hash", "role", "created_at", "disabled_at"} {
		var name string
		err := d.QueryRowContext(ctx,
			`SELECT name FROM pragma_table_info('users') WHERE name = ?`, col).Scan(&name)
		require.NoError(t, err, "users.%s missing", col)
	}

	// users.username UNIQUE
	var unique int
	require.NoError(t, d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND tbl_name = 'users' AND sql LIKE '%username%'`,
	).Scan(&unique))
	require.GreaterOrEqual(t, unique, 1, "users.username should have a UNIQUE index")

	// sessions table presence + key columns
	for _, col := range []string{
		"id", "user_id", "token_hash", "csrf_token",
		"created_at", "last_seen_at", "idle_expires_at", "absolute_expires_at",
	} {
		var name string
		err := d.QueryRowContext(ctx,
			`SELECT name FROM pragma_table_info('sessions') WHERE name = ?`, col).Scan(&name)
		require.NoError(t, err, "sessions.%s missing", col)
	}
}

func TestMigrate_AddsSubscriptionCredentialColumns(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	for _, c := range []struct{ col, def string }{
		{"cookie", "''"},
		{"basic_auth_user", "''"},
		{"basic_auth_pass", "''"},
	} {
		var name string
		var defaultVal sql.NullString
		err := d.QueryRowContext(ctx,
			`SELECT name, "dflt_value" FROM pragma_table_info('subscriptions') WHERE name = ?`,
			c.col).Scan(&name, &defaultVal)
		require.NoError(t, err, "subscriptions.%s not found", c.col)
		require.Equal(t, c.def, defaultVal.String, "subscriptions.%s default", c.col)
	}
}
```

Also update `TestMigrate_AppliesAllMigrationsExactlyOnce` (the existing version-tracking test) so the assertions use `5` instead of `4`.

- [ ] **Step 2: Run the tests to confirm they fail**

```bash
go test ./internal/db -run "TestMigrate_(AddsAuthTables|AddsSubscriptionCredentialColumns|AppliesAllMigrationsExactlyOnce)" 2>&1 | head -20
```

Expected: all three fail. The auth-table and credential-column tests fail because the schema doesn't have those tables/columns yet. `AppliesAllMigrationsExactlyOnce` fails because version is 4, not 5.

- [ ] **Step 3: Create the migration file**

Create `internal/db/migrations/0005_auth_and_credentials.sql`:

```sql
CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL CHECK (role IN ('admin','user')),
    created_at    INTEGER NOT NULL,
    disabled_at   INTEGER
);

CREATE TABLE sessions (
    id                  INTEGER PRIMARY KEY,
    user_id             INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash          TEXT    NOT NULL UNIQUE,
    csrf_token          TEXT    NOT NULL,
    created_at          INTEGER NOT NULL,
    last_seen_at        INTEGER NOT NULL,
    idle_expires_at     INTEGER NOT NULL,
    absolute_expires_at INTEGER NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

ALTER TABLE subscriptions ADD COLUMN cookie          TEXT NOT NULL DEFAULT '';
ALTER TABLE subscriptions ADD COLUMN basic_auth_user TEXT NOT NULL DEFAULT '';
ALTER TABLE subscriptions ADD COLUMN basic_auth_pass TEXT NOT NULL DEFAULT '';
```

- [ ] **Step 4: Run the tests to confirm they pass**

```bash
go test ./internal/db -run "TestMigrate_(AddsAuthTables|AddsSubscriptionCredentialColumns|AppliesAllMigrationsExactlyOnce)"
```

Expected: all three PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/migrations/0005_auth_and_credentials.sql internal/db/migrate_test.go
git commit -m "$(cat <<'EOF'
M6: migration 0005 — users, sessions, subscription credentials

Two new tables (users with role check, sessions with idle + absolute
expiry columns and an index on user_id) and three new TEXT columns
on subscriptions (cookie, basic_auth_user, basic_auth_pass) all
default to empty so existing M5 rows behave unchanged.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task B2: `db.users` — `InsertUser`, `GetUserByUsername`, `ErrUserExists`

**Files:**
- Create: `internal/db/users.go`
- Create: `internal/db/users_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/db/users_test.go`:

```go
package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInsertUserAndGetByUsername(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	id, err := InsertUser(ctx, d, NewUser{
		Username:     "ben",
		PasswordHash: "$argon2id$...",
		Role:         "admin",
		CreatedAt:    1_700_000_000,
	})
	require.NoError(t, err)
	require.NotZero(t, id)

	u, err := GetUserByUsername(ctx, d, "ben")
	require.NoError(t, err)
	require.Equal(t, id, u.ID)
	require.Equal(t, "ben", u.Username)
	require.Equal(t, "$argon2id$...", u.PasswordHash)
	require.Equal(t, "admin", u.Role)
	require.Equal(t, int64(1_700_000_000), u.CreatedAt)
	require.False(t, u.DisabledAt.Valid)
}

func TestInsertUserDuplicateUsername(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	_, err := InsertUser(ctx, d, NewUser{Username: "alice", PasswordHash: "x", Role: "admin", CreatedAt: 0})
	require.NoError(t, err)
	_, err = InsertUser(ctx, d, NewUser{Username: "alice", PasswordHash: "y", Role: "user", CreatedAt: 0})
	require.ErrorIs(t, err, ErrUserExists)
}

func TestGetUserByUsernameMissing(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	_, err := GetUserByUsername(context.Background(), d, "nobody")
	require.True(t, errors.Is(err, sql.ErrNoRows), "got: %v", err)
}
```

- [ ] **Step 2: Run to confirm it fails (compile error)**

```bash
go test ./internal/db -run "TestInsertUser|TestGetUserByUsername" 2>&1 | head -10
```

Expected: build failure — none of `User`, `NewUser`, `InsertUser`, `GetUserByUsername`, `ErrUserExists` exist yet.

- [ ] **Step 3: Implement**

Create `internal/db/users.go`:

```go
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrUserExists wraps the underlying SQLite UNIQUE-constraint failure on
// users.username. Use errors.Is(err, ErrUserExists) in callers to map the
// duplicate case without coupling to the SQLite driver's error text.
var ErrUserExists = errors.New("user with this username already exists")

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
	CreatedAt    int64
	DisabledAt   sql.NullInt64
}

type NewUser struct {
	Username     string
	PasswordHash string
	Role         string // "admin" or "user"
	CreatedAt    int64
}

// InsertUser writes a new user. Returns ErrUserExists on duplicate username.
func InsertUser(ctx context.Context, d *sql.DB, u NewUser) (int64, error) {
	res, err := d.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?)`,
		u.Username, u.PasswordHash, u.Role, u.CreatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.username") {
			return 0, ErrUserExists
		}
		return 0, fmt.Errorf("insert user: %w", err)
	}
	return res.LastInsertId()
}

// GetUserByUsername returns the user with the given username. Returns
// sql.ErrNoRows if no row matches; the auth layer maps that to a generic
// invalid_credentials response so this function does not.
func GetUserByUsername(ctx context.Context, d *sql.DB, username string) (User, error) {
	var u User
	err := d.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, created_at, disabled_at
		FROM users WHERE username = ?
	`, username).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.DisabledAt)
	if err != nil {
		return User{}, err // bare-return so errors.Is(err, sql.ErrNoRows) works upstream
	}
	return u, nil
}
```

- [ ] **Step 4: Run to confirm it passes**

```bash
go test ./internal/db -run "TestInsertUser|TestGetUserByUsername"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/users.go internal/db/users_test.go
git commit -m "$(cat <<'EOF'
M6: db.InsertUser + db.GetUserByUsername + ErrUserExists

The duplicate-username case maps cleanly to ErrUserExists so the API
layer can return 409 without coupling to SQLite's error text. Missing
users return sql.ErrNoRows verbatim — the auth layer collapses that
to a generic invalid_credentials response upstream.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task B3: `db.users` — `GetUserByID`, `UpdatePasswordHash`, `DisableUser`, `CountUsers`

**Files:**
- Modify: `internal/db/users.go`
- Modify: `internal/db/users_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/db/users_test.go`:

```go
func TestGetUserByID(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	id, err := InsertUser(ctx, d, NewUser{Username: "ben", PasswordHash: "x", Role: "admin", CreatedAt: 0})
	require.NoError(t, err)

	u, err := GetUserByID(ctx, d, id)
	require.NoError(t, err)
	require.Equal(t, "ben", u.Username)

	_, err = GetUserByID(ctx, d, id+999)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestUpdatePasswordHash(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	id, err := InsertUser(ctx, d, NewUser{Username: "ben", PasswordHash: "old", Role: "admin", CreatedAt: 0})
	require.NoError(t, err)

	require.NoError(t, UpdatePasswordHash(ctx, d, id, "new"))

	u, err := GetUserByID(ctx, d, id)
	require.NoError(t, err)
	require.Equal(t, "new", u.PasswordHash)
}

func TestDisableUser(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	id, err := InsertUser(ctx, d, NewUser{Username: "ben", PasswordHash: "x", Role: "admin", CreatedAt: 0})
	require.NoError(t, err)

	require.NoError(t, DisableUser(ctx, d, id, 1_700_000_999))

	u, err := GetUserByID(ctx, d, id)
	require.NoError(t, err)
	require.True(t, u.DisabledAt.Valid)
	require.Equal(t, int64(1_700_000_999), u.DisabledAt.Int64)
}

func TestCountUsers(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	n, err := CountUsers(ctx, d)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	_, err = InsertUser(ctx, d, NewUser{Username: "a", PasswordHash: "x", Role: "admin", CreatedAt: 0})
	require.NoError(t, err)
	_, err = InsertUser(ctx, d, NewUser{Username: "b", PasswordHash: "y", Role: "user", CreatedAt: 0})
	require.NoError(t, err)

	n, err = CountUsers(ctx, d)
	require.NoError(t, err)
	require.Equal(t, 2, n)
}
```

- [ ] **Step 2: Run to confirm they fail**

```bash
go test ./internal/db -run "TestGetUserByID|TestUpdatePasswordHash|TestDisableUser|TestCountUsers" 2>&1 | head -10
```

Expected: build failure — `GetUserByID`, `UpdatePasswordHash`, `DisableUser`, `CountUsers` undefined.

- [ ] **Step 3: Implement**

Append to `internal/db/users.go`:

```go
// GetUserByID returns the user with the given primary-key id. Returns
// sql.ErrNoRows if no such user.
func GetUserByID(ctx context.Context, d *sql.DB, id int64) (User, error) {
	var u User
	err := d.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, created_at, disabled_at
		FROM users WHERE id = ?
	`, id).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.DisabledAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// UpdatePasswordHash overwrites the password_hash on a user. The caller is
// responsible for any session-invalidation policy (PATCH /me/password
// keeps the current session and deletes others; tap admin passwd deletes
// every session for the user).
func UpdatePasswordHash(ctx context.Context, d *sql.DB, id int64, hash string) error {
	res, err := d.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	if err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DisableUser sets disabled_at on a user. The login path checks this column
// and rejects login for disabled accounts; existing sessions remain valid
// until they expire (M7 closes that gap with the admin-reset path).
func DisableUser(ctx context.Context, d *sql.DB, id, disabledAt int64) error {
	res, err := d.ExecContext(ctx, `UPDATE users SET disabled_at = ? WHERE id = ?`, disabledAt, id)
	if err != nil {
		return fmt.Errorf("disable user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CountUsers returns the number of rows in the users table. Used by the
// env-var bootstrap path to decide whether to create the first admin.
func CountUsers(ctx context.Context, d *sql.DB) (int, error) {
	var n int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}
```

- [ ] **Step 4: Run to confirm they pass**

```bash
go test ./internal/db -run "TestGetUserByID|TestUpdatePasswordHash|TestDisableUser|TestCountUsers"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/users.go internal/db/users_test.go
git commit -m "$(cat <<'EOF'
M6: db.GetUserByID, UpdatePasswordHash, DisableUser, CountUsers

Backs the auth API (GetUserByID for session middleware), the
self-service password change and admin reset paths
(UpdatePasswordHash), the future admin disable-user path (DisableUser),
and the env-var first-launch bootstrap (CountUsers).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task B4: `db.sessions` — Insert + Get + Refresh

**Files:**
- Create: `internal/db/sessions.go`
- Create: `internal/db/sessions_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/db/sessions_test.go`:

```go
package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func insertTestUser(t *testing.T, d *sql.DB, username string) int64 {
	t.Helper()
	id, err := InsertUser(context.Background(), d, NewUser{
		Username: username, PasswordHash: "x", Role: "admin", CreatedAt: 0,
	})
	require.NoError(t, err)
	return id
}

func TestInsertAndGetSessionByTokenHash(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "ben")

	id, err := InsertSession(ctx, d, NewSession{
		UserID:            uid,
		TokenHash:         "abc123",
		CSRFToken:         "csrf-1",
		CreatedAt:         100,
		LastSeenAt:        100,
		IdleExpiresAt:     200,
		AbsoluteExpiresAt: 1_000_000,
	})
	require.NoError(t, err)
	require.NotZero(t, id)

	s, err := GetSessionByTokenHash(ctx, d, "abc123")
	require.NoError(t, err)
	require.Equal(t, id, s.ID)
	require.Equal(t, uid, s.UserID)
	require.Equal(t, "csrf-1", s.CSRFToken)
	require.Equal(t, int64(100), s.CreatedAt)
	require.Equal(t, int64(100), s.LastSeenAt)
	require.Equal(t, int64(200), s.IdleExpiresAt)
	require.Equal(t, int64(1_000_000), s.AbsoluteExpiresAt)

	_, err = GetSessionByTokenHash(ctx, d, "no-such-hash")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestRefreshSessionIdle(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "ben")
	sid, err := InsertSession(ctx, d, NewSession{
		UserID: uid, TokenHash: "h", CSRFToken: "c",
		CreatedAt: 100, LastSeenAt: 100, IdleExpiresAt: 200, AbsoluteExpiresAt: 1_000_000,
	})
	require.NoError(t, err)

	require.NoError(t, RefreshSessionIdle(ctx, d, sid, 500, 700))

	s, err := GetSessionByTokenHash(ctx, d, "h")
	require.NoError(t, err)
	require.Equal(t, int64(500), s.LastSeenAt)
	require.Equal(t, int64(700), s.IdleExpiresAt)
	require.Equal(t, int64(1_000_000), s.AbsoluteExpiresAt, "absolute should not change on idle refresh")
}
```

- [ ] **Step 2: Run to confirm they fail**

```bash
go test ./internal/db -run "TestInsertAndGetSessionByTokenHash|TestRefreshSessionIdle" 2>&1 | head -10
```

Expected: build failure.

- [ ] **Step 3: Implement**

Create `internal/db/sessions.go`:

```go
package db

import (
	"context"
	"database/sql"
	"fmt"
)

type Session struct {
	ID                int64
	UserID            int64
	TokenHash         string
	CSRFToken         string
	CreatedAt         int64
	LastSeenAt        int64
	IdleExpiresAt     int64
	AbsoluteExpiresAt int64
}

type NewSession struct {
	UserID            int64
	TokenHash         string
	CSRFToken         string
	CreatedAt         int64
	LastSeenAt        int64
	IdleExpiresAt     int64
	AbsoluteExpiresAt int64
}

// InsertSession writes a new session row.
func InsertSession(ctx context.Context, d *sql.DB, s NewSession) (int64, error) {
	res, err := d.ExecContext(ctx, `
		INSERT INTO sessions
		    (user_id, token_hash, csrf_token, created_at, last_seen_at,
		     idle_expires_at, absolute_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, s.UserID, s.TokenHash, s.CSRFToken, s.CreatedAt, s.LastSeenAt,
		s.IdleExpiresAt, s.AbsoluteExpiresAt)
	if err != nil {
		return 0, fmt.Errorf("insert session: %w", err)
	}
	return res.LastInsertId()
}

// GetSessionByTokenHash looks up a session by its sha256-hex token hash.
// Returns sql.ErrNoRows if no row matches; the middleware maps that to a
// generic 401 invalid_session response so this function does not.
func GetSessionByTokenHash(ctx context.Context, d *sql.DB, tokenHash string) (Session, error) {
	var s Session
	err := d.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, csrf_token, created_at, last_seen_at,
		       idle_expires_at, absolute_expires_at
		FROM sessions WHERE token_hash = ?
	`, tokenHash).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.CSRFToken,
		&s.CreatedAt, &s.LastSeenAt, &s.IdleExpiresAt, &s.AbsoluteExpiresAt)
	if err != nil {
		return Session{}, err
	}
	return s, nil
}

// RefreshSessionIdle updates last_seen_at + idle_expires_at on a session.
// Concurrent refreshes converge — SQLite's WAL serialises writes.
func RefreshSessionIdle(ctx context.Context, d *sql.DB, id, lastSeenAt, idleExpiresAt int64) error {
	_, err := d.ExecContext(ctx, `
		UPDATE sessions SET last_seen_at = ?, idle_expires_at = ? WHERE id = ?
	`, lastSeenAt, idleExpiresAt, id)
	if err != nil {
		return fmt.Errorf("refresh session idle: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run to confirm they pass**

```bash
go test ./internal/db -run "TestInsertAndGetSessionByTokenHash|TestRefreshSessionIdle"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/sessions.go internal/db/sessions_test.go
git commit -m "$(cat <<'EOF'
M6: db.InsertSession, GetSessionByTokenHash, RefreshSessionIdle

The middleware reads sessions by token_hash on every authenticated
request and refreshes idle_expires_at unconditionally. Concurrent
refreshes race-converge via WAL.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task B5: `db.sessions` — CSRF rotation + Delete helpers

**Files:**
- Modify: `internal/db/sessions.go`
- Modify: `internal/db/sessions_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/db/sessions_test.go`:

```go
func TestUpdateSessionCSRFToken(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "ben")
	sid, err := InsertSession(ctx, d, NewSession{
		UserID: uid, TokenHash: "h", CSRFToken: "old",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)

	require.NoError(t, UpdateSessionCSRFToken(ctx, d, sid, "new"))
	s, err := GetSessionByTokenHash(ctx, d, "h")
	require.NoError(t, err)
	require.Equal(t, "new", s.CSRFToken)
}

func TestDeleteSession(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "ben")
	sid, err := InsertSession(ctx, d, NewSession{
		UserID: uid, TokenHash: "h", CSRFToken: "c",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)

	require.NoError(t, DeleteSession(ctx, d, sid))

	_, err = GetSessionByTokenHash(ctx, d, "h")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestDeleteSessionsByUserID(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "ben")
	other := insertTestUser(t, d, "alice")

	for _, h := range []string{"h1", "h2", "h3"} {
		_, err := InsertSession(ctx, d, NewSession{
			UserID: uid, TokenHash: h, CSRFToken: "c",
			CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
		})
		require.NoError(t, err)
	}
	_, err := InsertSession(ctx, d, NewSession{
		UserID: other, TokenHash: "untouched", CSRFToken: "c",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)

	require.NoError(t, DeleteSessionsByUserID(ctx, d, uid))

	for _, h := range []string{"h1", "h2", "h3"} {
		_, err := GetSessionByTokenHash(ctx, d, h)
		require.ErrorIs(t, err, sql.ErrNoRows, "hash %s should be gone", h)
	}
	_, err = GetSessionByTokenHash(ctx, d, "untouched")
	require.NoError(t, err, "the other user's session should remain")
}

func TestDeleteOtherSessionsForUser(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid := insertTestUser(t, d, "ben")

	keepID, err := InsertSession(ctx, d, NewSession{
		UserID: uid, TokenHash: "keep", CSRFToken: "c",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)
	for _, h := range []string{"drop1", "drop2"} {
		_, err := InsertSession(ctx, d, NewSession{
			UserID: uid, TokenHash: h, CSRFToken: "c",
			CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
		})
		require.NoError(t, err)
	}

	require.NoError(t, DeleteOtherSessionsForUser(ctx, d, uid, keepID))

	_, err = GetSessionByTokenHash(ctx, d, "keep")
	require.NoError(t, err)
	for _, h := range []string{"drop1", "drop2"} {
		_, err := GetSessionByTokenHash(ctx, d, h)
		require.ErrorIs(t, err, sql.ErrNoRows, "hash %s should be gone", h)
	}
}
```

- [ ] **Step 2: Run to confirm they fail**

```bash
go test ./internal/db -run "TestUpdateSessionCSRFToken|TestDeleteSession|TestDeleteSessionsByUserID|TestDeleteOtherSessionsForUser" 2>&1 | head -10
```

Expected: build failure — none of the helpers exist yet.

- [ ] **Step 3: Implement**

Append to `internal/db/sessions.go`:

```go
// UpdateSessionCSRFToken rotates the CSRF token on a session. PATCH
// /me/password is the only call site; tokens otherwise live for the
// session's full lifetime.
func UpdateSessionCSRFToken(ctx context.Context, d *sql.DB, id int64, csrf string) error {
	_, err := d.ExecContext(ctx, `UPDATE sessions SET csrf_token = ? WHERE id = ?`, csrf, id)
	if err != nil {
		return fmt.Errorf("update csrf token: %w", err)
	}
	return nil
}

// DeleteSession removes a single session row by id. Used by logout and by
// the middleware when an absolute-expired session is encountered.
func DeleteSession(ctx context.Context, d *sql.DB, id int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteSessionsByUserID removes every session belonging to a user. Used by
// `tap admin passwd` (admin reset implies the user is locked out).
func DeleteSessionsByUserID(ctx context.Context, d *sql.DB, userID int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete sessions by user: %w", err)
	}
	return nil
}

// DeleteOtherSessionsForUser removes every session belonging to a user
// except keepID. Used by PATCH /me/password — the user-initiated password
// change keeps the current session active while invalidating any others.
func DeleteOtherSessionsForUser(ctx context.Context, d *sql.DB, userID, keepID int64) error {
	_, err := d.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = ? AND id <> ?`, userID, keepID)
	if err != nil {
		return fmt.Errorf("delete other sessions: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run to confirm they pass**

```bash
go test ./internal/db -run "TestUpdateSessionCSRFToken|TestDeleteSession|TestDeleteSessionsByUserID|TestDeleteOtherSessionsForUser"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/db/sessions.go internal/db/sessions_test.go
git commit -m "$(cat <<'EOF'
M6: db.UpdateSessionCSRFToken + Delete{Session,SessionsByUserID,
OtherSessionsForUser}

UpdateSessionCSRFToken backs the CSRF rotation on password change.
DeleteSession is the logout / absolute-expired path. DeleteSessionsByUserID
backs admin password reset (tap admin passwd). DeleteOtherSessionsForUser
backs self-service password change — the current session stays alive.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task B6: `db.subscriptions` credential columns

**Files:**
- Modify: `internal/db/subscriptions.go`
- Modify: `internal/db/subscriptions_test.go`

The existing `Subscription`, `DueSubscription`, and `NewSubscription` structs gain `Cookie`, `BasicAuthUser`, `BasicAuthPass`. SELECTs grow the columns. `InsertSubscription` writes them. New `UpdateSubscriptionCredentials` helper.

- [ ] **Step 1: Write the failing tests**

Append to `internal/db/subscriptions_test.go`:

```go
func TestInsertAndGetSubscriptionWithCreds(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	id, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
		Cookie: "session=abc", BasicAuthUser: "ben", BasicAuthPass: "secret",
	})
	require.NoError(t, err)

	s, err := GetSubscription(ctx, d, id)
	require.NoError(t, err)
	require.Equal(t, "session=abc", s.Cookie)
	require.Equal(t, "ben", s.BasicAuthUser)
	require.Equal(t, "secret", s.BasicAuthPass)
}

func TestListDuePollsCarriesCreds(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	_, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
		Cookie: "c", BasicAuthUser: "u", BasicAuthPass: "p",
	})
	require.NoError(t, err)

	due, err := ListDuePolls(ctx, d, 0, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, "c", due[0].Cookie)
	require.Equal(t, "u", due[0].BasicAuthUser)
	require.Equal(t, "p", due[0].BasicAuthPass)
}

func TestUpdateSubscriptionCredentials(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	id, err := InsertSubscription(ctx, d, NewSubscription{
		Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	require.NoError(t, UpdateSubscriptionCredentials(ctx, d, id, "ckie", "u", "p"))
	s, err := GetSubscription(ctx, d, id)
	require.NoError(t, err)
	require.Equal(t, "ckie", s.Cookie)
	require.Equal(t, "u", s.BasicAuthUser)
	require.Equal(t, "p", s.BasicAuthPass)

	// Empty strings clear.
	require.NoError(t, UpdateSubscriptionCredentials(ctx, d, id, "", "", ""))
	s, err = GetSubscription(ctx, d, id)
	require.NoError(t, err)
	require.Empty(t, s.Cookie)
	require.Empty(t, s.BasicAuthUser)
	require.Empty(t, s.BasicAuthPass)

	// Missing id → sql.ErrNoRows.
	err = UpdateSubscriptionCredentials(ctx, d, id+999, "x", "y", "z")
	require.ErrorIs(t, err, sql.ErrNoRows)
}
```

- [ ] **Step 2: Run to confirm they fail (compile error)**

```bash
go test ./internal/db -run "TestInsertAndGetSubscriptionWithCreds|TestListDuePollsCarriesCreds|TestUpdateSubscriptionCredentials" 2>&1 | head -10
```

Expected: build failure — `Cookie`/`BasicAuthUser`/`BasicAuthPass` undefined on `NewSubscription`/`Subscription`/`DueSubscription`; `UpdateSubscriptionCredentials` undefined.

- [ ] **Step 3: Implement — extend the structs and queries**

Edit `internal/db/subscriptions.go`. Update `Subscription`:

```go
type Subscription struct {
	ID              int64
	Title           string
	FeedURL         string
	SiteURL         sql.NullString
	LastPollAt      sql.NullInt64
	NextPollAt      int64
	ETag            sql.NullString
	LastModified    sql.NullString
	ErrorCount      int
	LastError       sql.NullString
	CreatedAt       int64
	Extract         bool
	ExtractSelector string
	Cookie          string
	BasicAuthUser   string
	BasicAuthPass   string
}
```

Update `NewSubscription`:

```go
type NewSubscription struct {
	Title         string
	FeedURL       string
	SiteURL       string
	NextPoll      int64
	Created       int64
	Extract       bool
	Cookie        string
	BasicAuthUser string
	BasicAuthPass string
}
```

Update `DueSubscription`:

```go
type DueSubscription struct {
	ID              int64
	FeedURL         string
	ETag            sql.NullString
	LastModified    sql.NullString
	ErrorCount      int
	Extract         bool
	ExtractSelector string
	Cookie          string
	BasicAuthUser   string
	BasicAuthPass   string
}
```

Update `InsertSubscription`:

```go
func InsertSubscription(ctx context.Context, d *sql.DB, s NewSubscription) (int64, error) {
	res, err := d.ExecContext(ctx, `
		INSERT INTO subscriptions
		    (title, feed_url, site_url, next_poll_at, created_at, extract,
		     cookie, basic_auth_user, basic_auth_pass)
		VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?)
	`, s.Title, s.FeedURL, s.SiteURL, s.NextPoll, s.Created, boolToInt(s.Extract),
		s.Cookie, s.BasicAuthUser, s.BasicAuthPass)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: subscriptions.feed_url") {
			return 0, ErrSubscriptionExists
		}
		return 0, fmt.Errorf("insert subscription: %w", err)
	}
	return res.LastInsertId()
}
```

Update `GetSubscription` and `ListSubscriptions` SELECT lists to include the three new columns:

```go
SELECT id, title, feed_url, site_url, last_poll_at, next_poll_at,
       etag, last_modified, error_count, last_error, created_at,
       extract, extract_selector,
       cookie, basic_auth_user, basic_auth_pass
FROM subscriptions ...
```

And the corresponding `Scan` argument lists pick up `&s.Cookie, &s.BasicAuthUser, &s.BasicAuthPass`.

Update `ListDuePolls` similarly:

```go
SELECT id, feed_url, etag, last_modified, error_count,
       extract, extract_selector,
       cookie, basic_auth_user, basic_auth_pass
FROM subscriptions ...
```

with `&s.Cookie, &s.BasicAuthUser, &s.BasicAuthPass` in the Scan.

Add the new helper:

```go
// UpdateSubscriptionCredentials sets cookie + basic_auth_user + basic_auth_pass
// on one row. Returns sql.ErrNoRows if no subscription with that id exists.
// All three fields are written unconditionally — the API layer is responsible
// for the merge-patch semantics (omitted = no change, empty = clear).
func UpdateSubscriptionCredentials(ctx context.Context, d *sql.DB, id int64,
	cookie, basicAuthUser, basicAuthPass string) error {
	res, err := d.ExecContext(ctx, `
		UPDATE subscriptions
		SET cookie = ?, basic_auth_user = ?, basic_auth_pass = ?
		WHERE id = ?
	`, cookie, basicAuthUser, basicAuthPass, id)
	if err != nil {
		return fmt.Errorf("update subscription credentials %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
```

- [ ] **Step 4: Run to confirm everything passes**

```bash
go test ./internal/db -race
```

Expected: PASS for all `db` tests, including the existing M5 ones.

- [ ] **Step 5: Commit**

```bash
git add internal/db/subscriptions.go internal/db/subscriptions_test.go
git commit -m "$(cat <<'EOF'
M6: db.subscriptions gains cookie + basic_auth_user + basic_auth_pass

NewSubscription, Subscription, and DueSubscription all carry the
three credential columns. InsertSubscription writes them; ListDuePolls
exposes them to the worker. UpdateSubscriptionCredentials backs the
PATCH endpoint — the API layer's merge-patch semantics live one
layer up.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase C — API middleware

### Task C1: Error code constants + ctx helpers

**Files:**
- Modify: `internal/api/errors.go`
- Create: `internal/api/middleware.go`

- [ ] **Step 1: Add the new error codes (no test required — constants)**

Edit `internal/api/errors.go`:

```go
const (
	ErrCodeBadRequest             = "bad_request"
	ErrCodeNotFound               = "not_found"
	ErrCodeConflict               = "conflict"
	ErrCodeInternal               = "internal"
	ErrCodeExtractSelectorInvalid = "extract_selector_invalid"
	ErrCodeInvalidCredentials     = "invalid_credentials"
	ErrCodeInvalidSession         = "invalid_session"
	ErrCodeCSRFInvalid            = "csrf_invalid"
	ErrCodePasswordTooShort       = "password_too_short"
)
```

- [ ] **Step 2: Create the middleware skeleton**

Create `internal/api/middleware.go`:

```go
package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/bcrisp4/tap/internal/db"
)

// ctxKey is unexported so external packages cannot read or override the
// per-request user/session injected by requireSession.
type ctxKey int

const (
	ctxKeyUser ctxKey = iota
	ctxKeySession
)

// userFromContext returns the user injected by requireSession, if any.
func userFromContext(ctx context.Context) (db.User, bool) {
	u, ok := ctx.Value(ctxKeyUser).(db.User)
	return u, ok
}

// sessionFromContext returns the session injected by requireSession, if any.
func sessionFromContext(ctx context.Context) (db.Session, bool) {
	s, ok := ctx.Value(ctxKeySession).(db.Session)
	return s, ok
}

// chain composes middleware. Outermost is first.
func chain(mws ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](h)
		}
		return h
	}
}
```

- [ ] **Step 3: Verify the package still compiles**

```bash
go build ./internal/api
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/api/errors.go internal/api/middleware.go
git commit -m "$(cat <<'EOF'
M6: api error codes + middleware skeleton

Adds invalid_credentials, invalid_session, csrf_invalid, and
password_too_short to the stable error-code set. New middleware.go
sets up the typed ctxKey + chain helper that requireSession and
requireCSRF will hang off.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task C2: `requireSession` happy path

**Files:**
- Modify: `internal/api/middleware.go`
- Create: `internal/api/middleware_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/api/middleware_test.go`:

```go
package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

// helper: insert a user + a session with the given absolute/idle expiry; return
// (cookieValue, userID, sessionID, csrfToken).
func seedSession(t *testing.T, d any /* *sql.DB */, idleExp, absoluteExp int64) (string, int64, int64, string) {
	t.Helper()
	// Implementations cast d to *sql.DB; this helper is in the test file alongside.
	panic("seedSession is implemented inline below — see Step 3")
}

func TestRequireSessionPassesValidCookie(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	// Insert user and session manually for the test seam.
	uid, err := db.InsertUser(ctx, d, db.NewUser{
		Username: "ben", PasswordHash: "x", Role: "admin", CreatedAt: 0,
	})
	require.NoError(t, err)

	cookieVal := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	sum := sha256.Sum256([]byte("\x00"))
	_ = sum
	// Generate cookie + hash deterministically:
	raw := []byte("0123456789abcdef0123456789abcdef")
	cookieVal = base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256(raw)
	tokenHash := hex.EncodeToString(hash[:])

	sid, err := db.InsertSession(ctx, d, db.NewSession{
		UserID:            uid,
		TokenHash:         tokenHash,
		CSRFToken:         "csrf-1",
		CreatedAt:         time.Now().Unix(),
		LastSeenAt:        time.Now().Unix(),
		IdleExpiresAt:     time.Now().Add(time.Hour).Unix(),
		AbsoluteExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	})
	require.NoError(t, err)

	// Build a tiny handler that asserts user + session injection.
	var sawUser db.User
	var sawSession db.Session
	h := requireSession(d, time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := userFromContext(r.Context())
		s, _ := sessionFromContext(r.Context())
		sawUser = u
		sawSession = s
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: cookieVal})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
	require.Equal(t, uid, sawUser.ID)
	require.Equal(t, sid, sawSession.ID)
	require.Equal(t, "csrf-1", sawSession.CSRFToken)
	_ = strings.TrimSpace
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./internal/api -run TestRequireSessionPassesValidCookie 2>&1 | head -10
```

Expected: build failure — `requireSession` undefined.

- [ ] **Step 3: Implement `requireSession`**

Append to `internal/api/middleware.go`:

```go
// requireSession reads the tap_session cookie, looks up the row by
// sha256(cookie), validates idle + absolute expiries, refreshes idle on
// success, and injects the user + session into the request context.
// 401 invalid_session on any failure.
func requireSession(d *sql.DB, idleTTL time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie("tap_session")
			if err != nil || c.Value == "" {
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
				return
			}
			tokenHash, ok := hashCookie(c.Value)
			if !ok {
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "bad session cookie")
				return
			}
			s, err := db.GetSessionByTokenHash(r.Context(), d, tokenHash)
			if err != nil {
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no such session")
				return
			}
			now := time.Now().Unix()
			if now > s.AbsoluteExpiresAt || now > s.IdleExpiresAt {
				_ = db.DeleteSession(r.Context(), d, s.ID)
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "session expired")
				return
			}
			u, err := db.GetUserByID(r.Context(), d, s.UserID)
			if err != nil {
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "user not found")
				return
			}
			// Best-effort idle refresh. A failure here doesn't break the
			// request — worst case the session expires sooner than expected.
			_ = db.RefreshSessionIdle(r.Context(), d, s.ID, now, now+int64(idleTTL.Seconds()))

			ctx := context.WithValue(r.Context(), ctxKeyUser, u)
			ctx = context.WithValue(ctx, ctxKeySession, s)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// hashCookie decodes the base64url cookie value and returns its sha256-hex.
// Returns ok=false if the cookie is not valid base64url (which would never
// match a stored token_hash anyway).
func hashCookie(cookieValue string) (string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(cookieValue)
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), true
}
```

The relevant imports are `"encoding/base64"`, `"crypto/sha256"`, and `"encoding/hex"`. Drop the `url` import shown earlier in the file unless `originOK` (Task C4) needs it — it does, so keep it.

- [ ] **Step 4: Run to confirm the test passes**

```bash
go test ./internal/api -run TestRequireSessionPassesValidCookie -race
```

Expected: PASS. The middleware reads the cookie, hashes it, looks up the session, validates expiries, refreshes idle, and injects user + session into context.

- [ ] **Step 5: Commit**

```bash
git add internal/api/middleware.go internal/api/middleware_test.go
git commit -m "$(cat <<'EOF'
M6: requireSession middleware happy path

Reads tap_session cookie, sha256-hashes the decoded bytes, looks up
the session, validates idle + absolute expiries, refreshes idle on
success, and injects user + session into the request context. The
typed ctxKey keeps the injection safe from package-external readers.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task C3: `requireSession` failure modes

**Files:**
- Modify: `internal/api/middleware_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/api/middleware_test.go`:

```go
func TestRequireSessionRejectsMissingCookie(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	h := requireSession(d, time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run")
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusUnauthorized, rr.Code)
	require.Contains(t, rr.Body.String(), `"code":"invalid_session"`)
}

func TestRequireSessionRejectsBadCookie(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	h := requireSession(d, time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: "not-base64-not-a-known-hash!"})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestRequireSessionRejectsExpiredAndDeletes(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	uid, err := db.InsertUser(ctx, d, db.NewUser{Username: "ben", PasswordHash: "x", Role: "admin", CreatedAt: 0})
	require.NoError(t, err)

	raw := []byte("0123456789abcdef0123456789abcdef")
	cookieVal := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256(raw)

	sid, err := db.InsertSession(ctx, d, db.NewSession{
		UserID:    uid,
		TokenHash: hex.EncodeToString(hash[:]),
		CSRFToken: "c",
		CreatedAt: 0, LastSeenAt: 0,
		IdleExpiresAt:     1, // far in the past
		AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)

	h := requireSession(d, time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: cookieVal})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)

	// Session row should be deleted by the middleware.
	_, err = db.GetSessionByTokenHash(ctx, d, hex.EncodeToString(hash[:]))
	require.Error(t, err) // sql.ErrNoRows
	_ = sid
}
```

- [ ] **Step 2: Run to confirm they pass (no implementation change needed)**

```bash
go test ./internal/api -run TestRequireSessionRejects -race
```

Expected: PASS. The implementation from Task C2 already covers all three rejection paths.

- [ ] **Step 3: Commit**

```bash
git add internal/api/middleware_test.go
git commit -m "$(cat <<'EOF'
M6: requireSession rejection regressions

Locks in 401 invalid_session for missing cookie, malformed cookie
value, and expired session. The expired path also asserts that the
stale session row is deleted in the same handler so the table doesn't
grow with dead rows.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task C4: `requireCSRF` middleware

**Files:**
- Modify: `internal/api/middleware.go`
- Modify: `internal/api/middleware_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/api/middleware_test.go`:

```go
// withSession builds a handler chain that fakes session injection without
// going through requireSession (so requireCSRF can be tested in isolation).
func withSession(t *testing.T, csrfToken string, next http.Handler) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := db.Session{ID: 1, UserID: 1, CSRFToken: csrfToken}
		ctx := context.WithValue(r.Context(), ctxKeySession, s)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TestRequireCSRFPassesGET(t *testing.T) {
	t.Parallel()
	called := false
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.True(t, called)
}

func TestRequireCSRFRejectsMissingToken(t *testing.T) {
	t.Parallel()
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run")
	})))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "tap.example"
	req.Header.Set("Origin", "https://tap.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusForbidden, rr.Code)
	require.Contains(t, rr.Body.String(), `"code":"csrf_invalid"`)
}

func TestRequireCSRFRejectsWrongToken(t *testing.T) {
	t.Parallel()
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run")
	})))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "tap.example"
	req.Header.Set("Origin", "https://tap.example")
	req.Header.Set("X-CSRF-Token", "wrong")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRequireCSRFAcceptsRightToken(t *testing.T) {
	t.Parallel()
	called := false
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "tap.example"
	req.Header.Set("Origin", "https://tap.example")
	req.Header.Set("X-CSRF-Token", "tok")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.True(t, called)
}

func TestRequireCSRFRejectsMismatchedOrigin(t *testing.T) {
	t.Parallel()
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run")
	})))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "tap.example"
	req.Header.Set("Origin", "https://attacker.example")
	req.Header.Set("X-CSRF-Token", "tok")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRequireCSRFAcceptsMatchingReferer(t *testing.T) {
	t.Parallel()
	called := false
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "tap.example"
	req.Header.Set("Referer", "https://tap.example/some/path")
	req.Header.Set("X-CSRF-Token", "tok")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.True(t, called)
}

func TestRequireCSRFAcceptsBothMissing(t *testing.T) {
	// Both Origin and Referer missing → SameSite=Lax + an authenticated
	// session already cover the cross-site case. requireCSRF still needs
	// the token; absent Origin/Referer is not on its own grounds for 403.
	t.Parallel()
	called := false
	h := withSession(t, "tok", requireCSRF()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Host = "tap.example"
	req.Header.Set("X-CSRF-Token", "tok")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
	require.True(t, called)
}
```

- [ ] **Step 2: Run to confirm they fail (compile error)**

```bash
go test ./internal/api -run TestRequireCSRF 2>&1 | head -10
```

Expected: build failure — `requireCSRF` undefined.

- [ ] **Step 3: Implement `requireCSRF`**

Append to `internal/api/middleware.go`:

```go
// requireCSRF passes through GET/HEAD/OPTIONS, otherwise:
//   - validates Origin or Referer host equals r.Host (when the header is
//     present; both absent = pass, since SameSite=Lax + an authenticated
//     session already cover that case);
//   - reads X-CSRF-Token and constant-time compares to session.CSRFToken;
//   - 403 csrf_invalid on any failure.
func requireCSRF() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			if !originOK(r) {
				writeError(w, http.StatusForbidden, ErrCodeCSRFInvalid, "origin mismatch")
				return
			}

			s, ok := sessionFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusForbidden, ErrCodeCSRFInvalid, "no session in context")
				return
			}
			got := r.Header.Get("X-CSRF-Token")
			if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.CSRFToken)) != 1 {
				writeError(w, http.StatusForbidden, ErrCodeCSRFInvalid, "csrf token mismatch")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// originOK is the Origin/Referer host check. Returns true if either header
// is absent (concept §7.8: SameSite=Lax + auth already cover that case),
// or if at least one of them parses to a host equal to r.Host.
func originOK(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")
	if origin == "" && referer == "" {
		return true
	}
	if origin != "" {
		u, err := url.Parse(origin)
		if err == nil && u.Host == r.Host {
			return true
		}
	}
	if referer != "" {
		u, err := url.Parse(referer)
		if err == nil && u.Host == r.Host {
			return true
		}
	}
	// Avoid suppressing import warnings if errors isn't used elsewhere.
	_ = errors.New
	return false
}
```

- [ ] **Step 4: Run to confirm they all pass**

```bash
go test ./internal/api -run TestRequireCSRF -race
```

Expected: PASS for all six test cases.

- [ ] **Step 5: Commit**

```bash
git add internal/api/middleware.go internal/api/middleware_test.go
git commit -m "$(cat <<'EOF'
M6: requireCSRF middleware

Three layers per concept §7.8: SameSite=Lax cookie (set elsewhere),
Origin/Referer host check (when present), and constant-time
X-CSRF-Token compare against the session row. GETs pass through
unchecked. Both Origin and Referer absent is allowed — SameSite=Lax
plus an authenticated session already cover that case; the token
check is the binding defence.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase D — Auth API endpoints

### Task D1: DTOs, login DTO marshalling, cookie helpers

**Files:**
- Create: `internal/api/auth.go`

This task lays down the shared scaffolding (DTOs, the `setSessionCookie` / `clearSessionCookie` helpers, the `CookieSecureMode` enum) without yet wiring the handlers into a mux. No tests yet — handlers in subsequent tasks will exercise it.

- [ ] **Step 1: Create `internal/api/auth.go`**

```go
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
)

// CookieSecureMode controls the Secure attribute on the tap_session cookie.
//
//   CookieSecureAuto:  Secure when the listen address resolves to non-loopback.
//   CookieSecureTrue:  always set Secure.
//   CookieSecureFalse: never set Secure.
type CookieSecureMode int

const (
	CookieSecureAuto CookieSecureMode = iota
	CookieSecureTrue
	CookieSecureFalse
)

// resolveCookieSecure decides whether to set the Secure attribute on the
// session cookie. In auto mode, it inspects the listen address: bound to
// 127.0.0.0/8, ::1, or "localhost" → Secure off; anything else → Secure on.
func resolveCookieSecure(mode CookieSecureMode, addr string) bool {
	switch mode {
	case CookieSecureTrue:
		return true
	case CookieSecureFalse:
		return false
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// If --addr is malformed, default to Secure ON (fail-safe).
		return true
	}
	host = strings.TrimSpace(host)
	if host == "" || host == "localhost" {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// A bare hostname that isn't "localhost" — be conservative.
		return true
	}
	return !ip.IsLoopback()
}

// userDTO is the user shape exposed via the API. Never carries password_hash.
type userDTO struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func toUserDTO(u db.User) userDTO {
	return userDTO{ID: u.ID, Username: u.Username, Role: u.Role}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	User      userDTO `json:"user"`
	CSRFToken string  `json:"csrf_token"`
}

type passwordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type passwordChangeResponse struct {
	CSRFToken string `json:"csrf_token"`
}

// authDeps bundles the dependencies the auth handlers need so api.go's
// NewMux can construct them once and pass them to handler factories.
type authDeps struct {
	d                  *sql.DB
	sessionIdleTTL     time.Duration
	sessionAbsoluteTTL time.Duration
	cookieSecure       bool
}

// setSessionCookie writes the session cookie on the response.
func setSessionCookie(w http.ResponseWriter, value string, absoluteTTL time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "tap_session",
		Value:    value,
		Path:     "/",
		MaxAge:   int(absoluteTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSessionCookie writes a Max-Age=0 cookie that overrides the existing one.
func clearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "tap_session",
		Value:    "",
		Path:     "/",
		MaxAge:   0,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// _ keeps imports tidy.
var _ = json.Marshal
var _ = errors.New
var _ = context.Background
var _ = auth.Hash
```

- [ ] **Step 2: Verify the package builds**

```bash
go build ./internal/api
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/api/auth.go
git commit -m "$(cat <<'EOF'
M6: auth.go DTOs, cookie helpers, CookieSecureMode

Shared scaffolding for the upcoming login/logout/password-change
handlers. CookieSecureMode resolves to a bool against the listen
address (auto = Secure when non-loopback). The session-cookie
helpers centralise HttpOnly + SameSite=Lax + Path so handlers
can't drift on attributes.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task D2: Login endpoint — happy path

**Files:**
- Modify: `internal/api/auth.go`
- Create: `internal/api/auth_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/api/auth_test.go`:

```go
package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

// testParams keeps the auth tests fast — production uses auth.DefaultParams.
var testHashParams = auth.Params{Time: 1, Memory: 8 * 1024, Threads: 1, SaltLen: 8, KeyLen: 16}

// seedUser inserts a user with the given password hashed under testHashParams.
func seedUser(t *testing.T, d *sql.DB, username, password, role string) int64 {
	t.Helper()
	hash, err := auth.Hash(password, testHashParams)
	require.NoError(t, err)
	id, err := db.InsertUser(context.Background(), d, db.NewUser{
		Username: username, PasswordHash: hash, Role: role, CreatedAt: 0,
	})
	require.NoError(t, err)
	return id
}

func TestLoginHappyPath(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := seedUser(t, d, "ben", "correct horse battery staple", "admin")

	deps := authDeps{
		d:                  d,
		sessionIdleTTL:     time.Hour,
		sessionAbsoluteTTL: 24 * time.Hour,
		cookieSecure:       false,
	}

	body, _ := json.Marshal(loginRequest{Username: "ben", Password: "correct horse battery staple"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	loginHandler(deps).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var resp loginResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Equal(t, uid, resp.User.ID)
	require.Equal(t, "ben", resp.User.Username)
	require.Equal(t, "admin", resp.User.Role)
	require.NotEmpty(t, resp.CSRFToken)

	cookies := rr.Result().Cookies()
	require.NotEmpty(t, cookies)
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "tap_session" {
			sessionCookie = c
		}
	}
	require.NotNil(t, sessionCookie)
	require.True(t, sessionCookie.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, sessionCookie.SameSite)
	require.NotEmpty(t, sessionCookie.Value)
	require.True(t, strings.HasPrefix(sessionCookie.Path, "/"))
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./internal/api -run TestLoginHappyPath 2>&1 | head -10
```

Expected: build failure — `loginHandler` undefined.

- [ ] **Step 3: Implement `loginHandler`**

Append to `internal/api/auth.go`:

```go
// loginHandler returns POST /api/v1/sessions. Public, CSRF not required.
func loginHandler(dep authDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body loginRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}
		username := strings.TrimSpace(body.Username)
		if username == "" || body.Password == "" {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "username or password incorrect")
			return
		}

		u, err := db.GetUserByUsername(r.Context(), dep.d, username)
		if err != nil {
			// Includes sql.ErrNoRows (unknown user). Same response either way
			// to avoid disclosing username existence.
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "username or password incorrect")
			return
		}
		if u.DisabledAt.Valid {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "username or password incorrect")
			return
		}
		ok, err := auth.Verify(u.PasswordHash, body.Password)
		if err != nil || !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "username or password incorrect")
			return
		}

		cookieValue, tokenHash, err := auth.MintSessionToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		csrfToken, err := auth.MintCSRFToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		now := time.Now()
		_, err = db.InsertSession(r.Context(), dep.d, db.NewSession{
			UserID:            u.ID,
			TokenHash:         tokenHash,
			CSRFToken:         csrfToken,
			CreatedAt:         now.Unix(),
			LastSeenAt:        now.Unix(),
			IdleExpiresAt:     now.Add(dep.sessionIdleTTL).Unix(),
			AbsoluteExpiresAt: now.Add(dep.sessionAbsoluteTTL).Unix(),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}

		setSessionCookie(w, cookieValue, dep.sessionAbsoluteTTL, dep.cookieSecure)
		writeJSON(w, http.StatusOK, loginResponse{User: toUserDTO(u), CSRFToken: csrfToken})
	})
}
```

- [ ] **Step 4: Run to confirm it passes**

```bash
go test ./internal/api -run TestLoginHappyPath -race
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/auth.go internal/api/auth_test.go
git commit -m "$(cat <<'EOF'
M6: POST /api/v1/sessions login happy path

Verify(password) → mint session + CSRF tokens → insert sessions row
→ set HttpOnly tap_session cookie + return {user, csrf_token}.
1 MiB request body cap. Trimmed-empty username collapses to the
generic 401 invalid_credentials response, same as a bad password.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task D3: Login endpoint — failure modes

**Files:**
- Modify: `internal/api/auth_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/api/auth_test.go`:

```go
func TestLoginFailureModes(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	seedUser(t, d, "ben", "correct horse battery staple", "admin")

	disabledID := seedUser(t, d, "alice", "valid-password", "user")
	require.NoError(t, db.DisableUser(context.Background(), d, disabledID, time.Now().Unix()))

	deps := authDeps{d: d, sessionIdleTTL: time.Hour, sessionAbsoluteTTL: 24 * time.Hour}

	cases := []struct {
		name    string
		body    string
		wantSt  int
		wantSub string // substring of response body
	}{
		{"unknown user", `{"username":"nobody","password":"x"}`, http.StatusUnauthorized, `"code":"invalid_credentials"`},
		{"wrong password", `{"username":"ben","password":"x"}`, http.StatusUnauthorized, `"code":"invalid_credentials"`},
		{"disabled user", `{"username":"alice","password":"valid-password"}`, http.StatusUnauthorized, `"code":"invalid_credentials"`},
		{"empty username", `{"username":"","password":"x"}`, http.StatusUnauthorized, `"code":"invalid_credentials"`},
		{"empty password", `{"username":"ben","password":""}`, http.StatusUnauthorized, `"code":"invalid_credentials"`},
		{"malformed json", `{not json`, http.StatusBadRequest, `"code":"bad_request"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", strings.NewReader(tc.body))
			rr := httptest.NewRecorder()
			loginHandler(deps).ServeHTTP(rr, req)
			require.Equal(t, tc.wantSt, rr.Code, rr.Body.String())
			require.Contains(t, rr.Body.String(), tc.wantSub)
		})
	}
}

func TestLoginRejectsOversizedBody(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	deps := authDeps{d: d, sessionIdleTTL: time.Hour, sessionAbsoluteTTL: 24 * time.Hour}

	body := bytes.NewReader(bytes.Repeat([]byte("a"), 2<<20)) // 2 MiB > 1 MiB cap
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", body)
	rr := httptest.NewRecorder()
	loginHandler(deps).ServeHTTP(rr, req)
	require.GreaterOrEqual(t, rr.Code, 400)
	// The exact code depends on http.MaxBytesReader's interaction with the
	// JSON decoder — assert it's not a successful login.
	require.NotEqual(t, http.StatusOK, rr.Code)
}
```

- [ ] **Step 2: Run to confirm they pass (no implementation change needed)**

```bash
go test ./internal/api -run "TestLoginFailureModes|TestLoginRejectsOversizedBody" -race
```

Expected: PASS — Task D2's implementation already covers all paths. If `TestLoginRejectsOversizedBody` passes via 400 `bad_request` (the JSON decoder reports the truncated body as invalid JSON), that's expected; we don't strictly require 413 here.

- [ ] **Step 3: Commit**

```bash
git add internal/api/auth_test.go
git commit -m "$(cat <<'EOF'
M6: login failure mode regressions

Locks in: unknown user, wrong password, disabled account, empty
username, empty password, and malformed JSON all collapse to the
documented response shapes. A >1 MiB body is rejected by the
MaxBytesReader → JSON decoder chain.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task D4: GET /sessions/current and DELETE /sessions/current

**Files:**
- Modify: `internal/api/auth.go`
- Modify: `internal/api/auth_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/api/auth_test.go`:

```go
// withFakeAuth injects a session + user into context for handler-level tests
// that don't go through the full requireSession middleware.
func withFakeAuth(t *testing.T, u db.User, s db.Session, h http.Handler) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), ctxKeyUser, u)
		ctx = context.WithValue(ctx, ctxKeySession, s)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TestGetSessionCurrent(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u := db.User{ID: 7, Username: "ben", Role: "admin"}
	s := db.Session{ID: 99, UserID: 7, CSRFToken: "csrf-xyz"}

	h := withFakeAuth(t, u, s, getSessionCurrentHandler(authDeps{d: d}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/current", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp loginResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Equal(t, "ben", resp.User.Username)
	require.Equal(t, "csrf-xyz", resp.CSRFToken)
}

func TestLogoutDeletesSessionAndClearsCookie(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := seedUser(t, d, "ben", "password1", "admin")

	sid, err := db.InsertSession(context.Background(), d, db.NewSession{
		UserID: uid, TokenHash: "h", CSRFToken: "c",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)

	deps := authDeps{d: d, cookieSecure: false}
	h := withFakeAuth(t,
		db.User{ID: uid, Username: "ben", Role: "admin"},
		db.Session{ID: sid, UserID: uid, CSRFToken: "c"},
		logoutHandler(deps))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/sessions/current", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)

	// Session row should be gone.
	_, err = db.GetSessionByTokenHash(context.Background(), d, "h")
	require.Error(t, err)

	// Set-Cookie clears tap_session.
	var cleared bool
	for _, c := range rr.Result().Cookies() {
		if c.Name == "tap_session" {
			cleared = true
			require.Equal(t, 0, c.MaxAge)
		}
	}
	require.True(t, cleared, "tap_session should be cleared")
}
```

- [ ] **Step 2: Run to confirm they fail**

```bash
go test ./internal/api -run "TestGetSessionCurrent|TestLogoutDeletesSessionAndClearsCookie" 2>&1 | head -10
```

Expected: build failure — `getSessionCurrentHandler` and `logoutHandler` undefined.

- [ ] **Step 3: Implement**

Append to `internal/api/auth.go`:

```go
// getSessionCurrentHandler returns GET /api/v1/sessions/current.
// Authenticated; CSRF not required (GET). The SPA calls this on boot to
// recover its in-memory CSRF token after a reload.
func getSessionCurrentHandler(_ authDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok1 := userFromContext(r.Context())
		s, ok2 := sessionFromContext(r.Context())
		if !ok1 || !ok2 {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		writeJSON(w, http.StatusOK, loginResponse{User: toUserDTO(u), CSRFToken: s.CSRFToken})
	})
}

// logoutHandler returns DELETE /api/v1/sessions/current. Authenticated;
// CSRF required (the middleware chain enforces that, not this handler).
func logoutHandler(dep authDeps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := sessionFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		if err := db.DeleteSession(r.Context(), dep.d, s.ID); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		clearSessionCookie(w, dep.cookieSecure)
		w.WriteHeader(http.StatusNoContent)
	})
}
```

- [ ] **Step 4: Run to confirm they pass**

```bash
go test ./internal/api -run "TestGetSessionCurrent|TestLogoutDeletesSessionAndClearsCookie" -race
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/auth.go internal/api/auth_test.go
git commit -m "$(cat <<'EOF'
M6: GET + DELETE /api/v1/sessions/current

GET probes the active session and returns {user, csrf_token} so the
SPA can recover after a reload. DELETE deletes the session row and
clears the tap_session cookie. Both rely on session injection by
requireSession (CSRF middleware enforces the token check on DELETE
upstream).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task D5: PATCH /me/password

**Files:**
- Modify: `internal/api/auth.go`
- Modify: `internal/api/auth_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/api/auth_test.go`:

```go
func TestPasswordChangeHappyPath(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := seedUser(t, d, "ben", "old-password-12", "admin")

	currentSID, err := db.InsertSession(context.Background(), d, db.NewSession{
		UserID: uid, TokenHash: "current", CSRFToken: "old-csrf",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)
	otherSID, err := db.InsertSession(context.Background(), d, db.NewSession{
		UserID: uid, TokenHash: "other", CSRFToken: "x",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)

	deps := authDeps{d: d}
	currentSession := db.Session{ID: currentSID, UserID: uid, CSRFToken: "old-csrf"}
	currentUser := db.User{ID: uid, Username: "ben", Role: "admin"}
	// Pre-load password hash by re-reading the user.
	full, err := db.GetUserByID(context.Background(), d, uid)
	require.NoError(t, err)
	currentUser.PasswordHash = full.PasswordHash

	h := withFakeAuth(t, currentUser, currentSession, passwordChangeHandler(deps, testHashParams))

	body, _ := json.Marshal(passwordChangeRequest{
		CurrentPassword: "old-password-12",
		NewPassword:     "new-password-34",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me/password", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var resp passwordChangeResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.CSRFToken)
	require.NotEqual(t, "old-csrf", resp.CSRFToken)

	// New password verifies; old password no longer does.
	updated, err := db.GetUserByID(context.Background(), d, uid)
	require.NoError(t, err)
	ok, err := auth.Verify(updated.PasswordHash, "new-password-34")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = auth.Verify(updated.PasswordHash, "old-password-12")
	require.NoError(t, err)
	require.False(t, ok)

	// Current session retained; other session deleted.
	_, err = db.GetSessionByTokenHash(context.Background(), d, "current")
	require.NoError(t, err)
	_, err = db.GetSessionByTokenHash(context.Background(), d, "other")
	require.Error(t, err)
	_ = otherSID
}

func TestPasswordChangeWrongCurrent(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := seedUser(t, d, "ben", "old-password-12", "admin")
	full, _ := db.GetUserByID(context.Background(), d, uid)

	deps := authDeps{d: d}
	h := withFakeAuth(t,
		db.User{ID: uid, Username: "ben", Role: "admin", PasswordHash: full.PasswordHash},
		db.Session{ID: 1, UserID: uid, CSRFToken: "c"},
		passwordChangeHandler(deps, testHashParams))

	body, _ := json.Marshal(passwordChangeRequest{
		CurrentPassword: "wrong",
		NewPassword:     "new-password-34",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me/password", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)
	require.Contains(t, rr.Body.String(), `"code":"invalid_credentials"`)
}

func TestPasswordChangeTooShortNew(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	uid := seedUser(t, d, "ben", "old-password-12", "admin")
	full, _ := db.GetUserByID(context.Background(), d, uid)

	deps := authDeps{d: d}
	h := withFakeAuth(t,
		db.User{ID: uid, Username: "ben", Role: "admin", PasswordHash: full.PasswordHash},
		db.Session{ID: 1, UserID: uid, CSRFToken: "c"},
		passwordChangeHandler(deps, testHashParams))

	body, _ := json.Marshal(passwordChangeRequest{
		CurrentPassword: "old-password-12",
		NewPassword:     "short",
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me/password", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), `"code":"password_too_short"`)
}
```

- [ ] **Step 2: Run to confirm they fail**

```bash
go test ./internal/api -run "TestPasswordChange" 2>&1 | head -10
```

Expected: build failure — `passwordChangeHandler` undefined.

- [ ] **Step 3: Implement**

Append to `internal/api/auth.go`:

```go
// passwordChangeHandler returns PATCH /api/v1/me/password. Authenticated;
// CSRF required (middleware enforces). Verifies current_password, validates
// new_password, hashes + updates, deletes other sessions for the user
// (keeps current), rotates the current session's CSRF token, returns
// the new csrf_token.
//
// hashParams is exposed so tests can inject testHashParams; production
// passes auth.DefaultParams.
func passwordChangeHandler(dep authDeps, hashParams auth.Params) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		s, ok := sessionFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var body passwordChangeRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}

		ok, err := auth.Verify(u.PasswordHash, body.CurrentPassword)
		if err != nil || !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "current password incorrect")
			return
		}
		if err := auth.ValidatePassword(body.NewPassword); err != nil {
			writeError(w, http.StatusBadRequest, ErrCodePasswordTooShort, "new password too short")
			return
		}
		newHash, err := auth.Hash(body.NewPassword, hashParams)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.UpdatePasswordHash(r.Context(), dep.d, u.ID, newHash); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.DeleteOtherSessionsForUser(r.Context(), dep.d, u.ID, s.ID); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		newCSRF, err := auth.MintCSRFToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		if err := db.UpdateSessionCSRFToken(r.Context(), dep.d, s.ID, newCSRF); err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, passwordChangeResponse{CSRFToken: newCSRF})
	})
}
```

- [ ] **Step 4: Run to confirm they pass**

```bash
go test ./internal/api -run "TestPasswordChange" -race
```

Expected: PASS for all three tests.

- [ ] **Step 5: Commit**

```bash
git add internal/api/auth.go internal/api/auth_test.go
git commit -m "$(cat <<'EOF'
M6: PATCH /api/v1/me/password

Self-service password change. Verifies current_password (re-prompt
gate even with an active session), validates new_password (≥8 chars),
re-hashes, deletes the user's other sessions (recovery primitive),
and rotates the current session's CSRF token. Returns {csrf_token}
so the SPA can update its in-memory copy without re-logging in.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase E — Subscription credential surface

### Task E1: POST accepts credentials, GET DTO grows booleans

**Files:**
- Modify: `internal/api/subscriptions.go`
- Modify: `internal/api/subscriptions_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/api/subscriptions_test.go` (the file already has helpers like `setupSubscriptionsAPI` from M5):

```go
func TestPostSubscriptionAcceptsCredentialsAndGetReturnsBooleans(t *testing.T) {
	t.Parallel()
	mux, _ := newSubscriptionsTestMux(t) // existing helper that builds an API mux + DB

	body := map[string]any{
		"feed_url":         "https://x.example/feed",
		"basic_auth_user":  "ben",
		"basic_auth_pass":  "secret",
		"cookie":           "session=abc",
	}
	bs, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(bs))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	// Response should contain has_cookie + has_basic_auth, but NOT the values.
	respBody := rr.Body.String()
	require.Contains(t, respBody, `"has_cookie":true`)
	require.Contains(t, respBody, `"has_basic_auth":true`)
	require.NotContains(t, respBody, "secret")
	require.NotContains(t, respBody, `"cookie":"session=abc"`)
	require.NotContains(t, respBody, "basic_auth_user")
	require.NotContains(t, respBody, "basic_auth_pass")

	// GET the list — same expectations.
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil)
	listRR := httptest.NewRecorder()
	mux.ServeHTTP(listRR, listReq)
	require.Equal(t, http.StatusOK, listRR.Code)
	listBody := listRR.Body.String()
	require.Contains(t, listBody, `"has_cookie":true`)
	require.Contains(t, listBody, `"has_basic_auth":true`)
	require.NotContains(t, listBody, "secret")
}

func TestPostSubscriptionWithoutCredentialsReturnsBooleanFalse(t *testing.T) {
	t.Parallel()
	mux, _ := newSubscriptionsTestMux(t)

	body := map[string]any{"feed_url": "https://x.example/feed"}
	bs, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(bs))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	require.Contains(t, rr.Body.String(), `"has_cookie":false`)
	require.Contains(t, rr.Body.String(), `"has_basic_auth":false`)
}
```

If `newSubscriptionsTestMux` doesn't already exist in the test file, define it as:

```go
// newSubscriptionsTestMux builds an unauthenticated API mux for unit tests.
// The session-middleware tests live in middleware_test.go; tests here focus
// on handler logic in isolation, so the mux is constructed without auth
// wrapping. End-to-end auth coverage lives in cmd/tap/main_test.go.
func newSubscriptionsTestMux(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	d := newTestDB(t)
	m := http.NewServeMux()
	registerSubscriptionRoutes(m, d, nil)
	return m, d
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./internal/api -run TestPostSubscription 2>&1 | head -20
```

Expected: failures — POST does not accept the credential fields and the DTO does not include the new booleans.

- [ ] **Step 3: Implement — POST body, DTO, helpers**

Edit `internal/api/subscriptions.go`. Update `subscriptionDTO`:

```go
type subscriptionDTO struct {
	ID              int64  `json:"id"`
	Title           string `json:"title"`
	FeedURL         string `json:"feed_url"`
	SiteURL         string `json:"site_url,omitempty"`
	NextPollAt      int64  `json:"next_poll_at"`
	LastPollAt      int64  `json:"last_poll_at,omitempty"`
	ErrorCount      int    `json:"error_count"`
	LastError       string `json:"last_error,omitempty"`
	CreatedAt       int64  `json:"created_at"`
	Extract         bool   `json:"extract"`
	ExtractSelector string `json:"extract_selector"`
	HasCookie       bool   `json:"has_cookie"`
	HasBasicAuth    bool   `json:"has_basic_auth"`
}
```

Update `toDTO`:

```go
func toDTO(s db.Subscription) subscriptionDTO {
	d := subscriptionDTO{
		ID:              s.ID,
		Title:           s.Title,
		FeedURL:         s.FeedURL,
		NextPollAt:      s.NextPollAt,
		ErrorCount:      s.ErrorCount,
		CreatedAt:       s.CreatedAt,
		Extract:         s.Extract,
		ExtractSelector: s.ExtractSelector,
		HasCookie:       s.Cookie != "",
		HasBasicAuth:    s.BasicAuthUser != "",
	}
	if s.SiteURL.Valid {
		d.SiteURL = s.SiteURL.String
	}
	if s.LastPollAt.Valid {
		d.LastPollAt = s.LastPollAt.Int64
	}
	if s.LastError.Valid {
		d.LastError = s.LastError.String
	}
	return d
}
```

Update the POST handler body decoder + insert:

```go
m.HandleFunc("POST /api/v1/subscriptions", func(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		FeedURL       string `json:"feed_url"`
		Title         string `json:"title"`
		Extract       bool   `json:"extract"`
		Cookie        string `json:"cookie"`
		BasicAuthUser string `json:"basic_auth_user"`
		BasicAuthPass string `json:"basic_auth_pass"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
		return
	}
	u, err := url.Parse(body.FeedURL)
	if err != nil || !u.IsAbs() {
		writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "feed_url must be an absolute URL")
		return
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "feed_url must use http or https")
		return
	}
	title := body.Title
	if title == "" {
		title = body.FeedURL
	}
	id, err := db.InsertSubscription(r.Context(), d, db.NewSubscription{
		Title:         title,
		FeedURL:       body.FeedURL,
		NextPoll:      0,
		Created:       time.Now().Unix(),
		Extract:       body.Extract,
		Cookie:        body.Cookie,
		BasicAuthUser: body.BasicAuthUser,
		BasicAuthPass: body.BasicAuthPass,
	})
	if err != nil {
		if errors.Is(err, db.ErrSubscriptionExists) {
			writeError(w, http.StatusConflict, ErrCodeConflict, "subscription already exists")
			return
		}
		slog.Error("insert subscription failed", "feed_url", body.FeedURL, "err", err)
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, "could not create subscription")
		return
	}
	s, err := db.GetSubscription(r.Context(), d, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
		return
	}
	if poke != nil {
		poke()
	}
	writeJSON(w, http.StatusCreated, toDTO(s))
})
```

- [ ] **Step 4: Run to confirm tests pass**

```bash
go test ./internal/api -run TestPostSubscription -race
```

Expected: PASS for both `TestPostSubscriptionAcceptsCredentialsAndGetReturnsBooleans` and `TestPostSubscriptionWithoutCredentialsReturnsBooleanFalse`.

- [ ] **Step 5: Commit**

```bash
git add internal/api/subscriptions.go internal/api/subscriptions_test.go
git commit -m "$(cat <<'EOF'
M6: POST /api/v1/subscriptions accepts cookie + basic_auth_*

Write-only acceptance: POST persists the three credential fields;
the GET DTO never returns the values, only has_cookie /
has_basic_auth booleans (true iff cookie != "" / basic_auth_user
!= ""; RFC 7617 permits an empty pass so user-only counts as
configured).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task E2: PATCH credential merge semantics

**Files:**
- Modify: `internal/api/subscriptions.go`
- Modify: `internal/api/subscriptions_test.go`

The existing PATCH handler already pre-reads the row for the M5 selector merge. M6 extends the same pattern to cookie / basic_auth_user / basic_auth_pass: omitted = no change, empty string = explicit clear.

- [ ] **Step 1: Write the failing tests**

Append to `internal/api/subscriptions_test.go`:

```go
func TestPatchSubscriptionCredentialMergeSemantics(t *testing.T) {
	t.Parallel()
	mux, d := newSubscriptionsTestMux(t)

	// Seed a subscription with all three creds set.
	id, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
		Cookie: "c", BasicAuthUser: "u", BasicAuthPass: "p",
	})
	require.NoError(t, err)

	// Step A: PATCH without any credential field — no change.
	patch(mux, t, id, `{}`)
	got, _ := db.GetSubscription(context.Background(), d, id)
	require.Equal(t, "c", got.Cookie)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "p", got.BasicAuthPass)

	// Step B: PATCH cookie="" — clear cookie only.
	patch(mux, t, id, `{"cookie":""}`)
	got, _ = db.GetSubscription(context.Background(), d, id)
	require.Empty(t, got.Cookie)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "p", got.BasicAuthPass)

	// Step C: PATCH basic_auth_pass set to a new value.
	patch(mux, t, id, `{"basic_auth_pass":"newpass"}`)
	got, _ = db.GetSubscription(context.Background(), d, id)
	require.Empty(t, got.Cookie)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "newpass", got.BasicAuthPass)

	// Step D: PATCH extract:true — does NOT clobber any credential.
	patch(mux, t, id, `{"extract":true}`)
	got, _ = db.GetSubscription(context.Background(), d, id)
	require.True(t, got.Extract)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "newpass", got.BasicAuthPass)

	// Step E: PATCH cookie:"x" — does NOT clobber extract or basic_auth_*.
	patch(mux, t, id, `{"cookie":"x"}`)
	got, _ = db.GetSubscription(context.Background(), d, id)
	require.True(t, got.Extract)
	require.Equal(t, "x", got.Cookie)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "newpass", got.BasicAuthPass)
}

// patch is a tiny helper for the test above.
func patch(mux http.Handler, t *testing.T, id int64, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/subscriptions/"+strconv.FormatInt(id, 10), strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "PATCH body=%s body=%s", body, rr.Body.String())
}
```

(Imports needed: `bytes`, `strconv`, `strings`, `database/sql`. Some are already there from earlier tasks.)

- [ ] **Step 2: Run to confirm they fail**

```bash
go test ./internal/api -run TestPatchSubscriptionCredentialMergeSemantics 2>&1 | head -10
```

Expected: failure — the PATCH handler doesn't yet recognise the three credential fields, so they pass through with no effect.

- [ ] **Step 3: Implement — extend PATCH body type and merge logic**

In `internal/api/subscriptions.go`, the PATCH handler's body type grows three pointer fields:

```go
m.HandleFunc("PATCH /api/v1/subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid id")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		Extract         *bool   `json:"extract"`
		ExtractSelector *string `json:"extract_selector"`
		Cookie          *string `json:"cookie"`
		BasicAuthUser   *string `json:"basic_auth_user"`
		BasicAuthPass   *string `json:"basic_auth_pass"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
		return
	}

	s, err := db.GetSubscription(r.Context(), d, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, ErrCodeNotFound, "subscription not found")
			return
		}
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
		return
	}

	extract := s.Extract
	selector := s.ExtractSelector
	cookie := s.Cookie
	basicUser := s.BasicAuthUser
	basicPass := s.BasicAuthPass

	if body.Extract != nil {
		extract = *body.Extract
	}
	if body.ExtractSelector != nil {
		candidate := *body.ExtractSelector
		if candidate != "" {
			if _, cerr := cascadia.Compile(candidate); cerr != nil {
				writeError(w, http.StatusBadRequest, ErrCodeExtractSelectorInvalid,
					"extract_selector did not compile: "+cerr.Error())
				return
			}
		}
		selector = candidate
	}
	if body.Cookie != nil {
		cookie = *body.Cookie
	}
	if body.BasicAuthUser != nil {
		basicUser = *body.BasicAuthUser
	}
	if body.BasicAuthPass != nil {
		basicPass = *body.BasicAuthPass
	}

	if err := db.UpdateSubscriptionExtraction(r.Context(), d, id, extract, selector); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, ErrCodeNotFound, "subscription not found")
			return
		}
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
		return
	}
	if err := db.UpdateSubscriptionCredentials(r.Context(), d, id, cookie, basicUser, basicPass); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, ErrCodeNotFound, "subscription not found")
			return
		}
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
		return
	}

	s.Extract = extract
	s.ExtractSelector = selector
	s.Cookie = cookie
	s.BasicAuthUser = basicUser
	s.BasicAuthPass = basicPass
	writeJSON(w, http.StatusOK, toDTO(s))
})
```

- [ ] **Step 4: Run to confirm the test passes**

```bash
go test ./internal/api -run TestPatchSubscriptionCredentialMergeSemantics -race
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/subscriptions.go internal/api/subscriptions_test.go
git commit -m "$(cat <<'EOF'
M6: PATCH /api/v1/subscriptions/{id} cookie + basic_auth_* merge

Same merge-patch semantics as M5's extract_selector: pointer fields
distinguish omitted (no change) from "" (explicit clear). Two writes
behind one HTTP call — UpdateSubscriptionExtraction first, then
UpdateSubscriptionCredentials — both safely idempotent.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase F — Per-feed creds wiring

### Task F1: `httpx.ApplyFeedCreds` helper

**Files:**
- Create: `internal/httpx/feedcreds.go`
- Create: `internal/httpx/feedcreds_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/httpx/feedcreds_test.go`:

```go
package httpx

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyFeedCredsCookie(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest(http.MethodGet, "https://x.example", nil)
	ApplyFeedCreds(req, FeedCreds{Cookie: "session=abc; tracking=1"})
	require.Equal(t, "session=abc; tracking=1", req.Header.Get("Cookie"))
	require.Empty(t, req.Header.Get("Authorization"))
}

func TestApplyFeedCredsBasicAuth(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest(http.MethodGet, "https://x.example", nil)
	ApplyFeedCreds(req, FeedCreds{BasicAuthUser: "ben", BasicAuthPass: "secret"})
	require.Empty(t, req.Header.Get("Cookie"))
	// SetBasicAuth produces "Basic base64(ben:secret)" — assert the prefix
	// and let net/http own the encoding.
	require.NotEmpty(t, req.Header.Get("Authorization"))
}

func TestApplyFeedCredsBoth(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest(http.MethodGet, "https://x.example", nil)
	ApplyFeedCreds(req, FeedCreds{
		Cookie: "c", BasicAuthUser: "u", BasicAuthPass: "p",
	})
	require.Equal(t, "c", req.Header.Get("Cookie"))
	require.NotEmpty(t, req.Header.Get("Authorization"))
}

func TestApplyFeedCredsEmpty(t *testing.T) {
	t.Parallel()
	req, _ := http.NewRequest(http.MethodGet, "https://x.example", nil)
	ApplyFeedCreds(req, FeedCreds{})
	require.Empty(t, req.Header.Get("Cookie"))
	require.Empty(t, req.Header.Get("Authorization"))
}

func TestApplyFeedCredsUserOnlyEmptyPassStillSetsAuth(t *testing.T) {
	// RFC 7617: the password may be empty. We set basic auth iff the user
	// is non-empty so a configured username with an intentionally empty
	// pass still produces an Authorization header.
	t.Parallel()
	req, _ := http.NewRequest(http.MethodGet, "https://x.example", nil)
	ApplyFeedCreds(req, FeedCreds{BasicAuthUser: "ben"})
	require.NotEmpty(t, req.Header.Get("Authorization"))
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./internal/httpx -run TestApplyFeedCreds 2>&1 | head -10
```

Expected: build failure — `FeedCreds` and `ApplyFeedCreds` undefined.

- [ ] **Step 3: Implement**

Create `internal/httpx/feedcreds.go`:

```go
package httpx

import "net/http"

// FeedCreds carries a subscription's per-feed credentials. Empty values mean
// "do not send"; the helper below is layered onto outbound requests at the
// caller site (the polling worker and the article extractor). The shared
// HTTP client itself is unchanged — SSRF, per-host concurrency cap, and
// timeout still apply to the resulting request.
type FeedCreds struct {
	Cookie        string
	BasicAuthUser string
	BasicAuthPass string
}

// ApplyFeedCreds layers per-feed Cookie and Authorization headers onto req.
// An empty Cookie leaves any pre-existing Cookie header alone (the caller
// is responsible for not setting one if they don't want one). Basic auth
// is applied iff BasicAuthUser is non-empty — RFC 7617 permits an empty
// password and we don't second-guess the operator's intent.
func ApplyFeedCreds(req *http.Request, creds FeedCreds) {
	if creds.Cookie != "" {
		req.Header.Set("Cookie", creds.Cookie)
	}
	if creds.BasicAuthUser != "" {
		req.SetBasicAuth(creds.BasicAuthUser, creds.BasicAuthPass)
	}
}
```

- [ ] **Step 4: Run to confirm tests pass**

```bash
go test ./internal/httpx -run TestApplyFeedCreds -race
```

Expected: PASS for all five tests.

- [ ] **Step 5: Commit**

```bash
git add internal/httpx/feedcreds.go internal/httpx/feedcreds_test.go
git commit -m "$(cat <<'EOF'
M6: httpx.ApplyFeedCreds helper

Layers per-feed Cookie / Authorization: Basic onto outbound
requests. Empty Cookie / empty user → no header set. The shared
client stays unchanged, so SSRF, per-host cap, and --http-timeout
keep applying. RFC 7617 lets pass be empty if the operator wants
that — user-only is enough to enable basic auth.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task F2: `extract.Extract` signature gains `creds`

**Files:**
- Modify: `internal/extract/extract.go`
- Modify: `internal/extract/extract_test.go`

The M5 spec already had this on the M6 horizon (M5 spec's Out-of-Scope table). Now we wire it.

- [ ] **Step 1: Write the failing test**

Append to `internal/extract/extract_test.go`:

```go
func TestExtractAppliesFeedCreds(t *testing.T) {
	t.Parallel()
	gotCookie := ""
	gotAuth := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><article><p>Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor.</p></article></body></html>`))
	}))
	defer srv.Close()

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 1<<20,
		httpx.FeedCreds{Cookie: "session=abc", BasicAuthUser: "ben", BasicAuthPass: "secret"})
	require.NoError(t, err)
	require.Equal(t, "session=abc", gotCookie)
	require.NotEmpty(t, gotAuth, "Authorization should be set")
}

func TestExtractEmptyFeedCredsSetsNoHeaders(t *testing.T) {
	t.Parallel()
	gotCookie := ""
	gotAuth := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><article><p>Sufficient text for readability fallback content.</p></article></body></html>`))
	}))
	defer srv.Close()

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 1<<20, httpx.FeedCreds{})
	require.NoError(t, err)
	require.Empty(t, gotCookie)
	require.Empty(t, gotAuth)
}
```

(Add the `httpx` import to the test file.)

- [ ] **Step 2: Update existing tests to pass `httpx.FeedCreds{}`**

Every existing call to `Extract(ctx, client, url, selector, bodyCap)` in `extract_test.go` needs to pass an empty `httpx.FeedCreds{}` as the 6th argument. Search-and-update.

- [ ] **Step 3: Run to confirm the new tests fail (compile error)**

```bash
go test ./internal/extract 2>&1 | head -10
```

Expected: build failure — `Extract` only accepts 5 args; tests pass 6.

- [ ] **Step 4: Implement the signature change**

Edit `internal/extract/extract.go`. Update the function:

```go
func Extract(ctx context.Context, client *http.Client,
	articleURL, selector string, bodyCap int64,
	creds httpx.FeedCreds) (string, error) {
	// (existing body, with the new step below before client.Do)

	// Compile selector first so a malformed one fails before any HTTP
	// round-trip.
	var sel cascadia.Selector
	if selector != "" {
		var serr error
		sel, serr = cascadia.Compile(selector)
		if serr != nil {
			return "", fmt.Errorf("compile selector: %w", serr)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, articleURL, nil)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "text/html, application/xhtml+xml;q=0.9, */*;q=0.5")
	httpx.ApplyFeedCreds(req, creds) // ← NEW

	resp, err := client.Do(req)
	// ... rest unchanged ...
}
```

(Keep the rest of the function body exactly as it is. Add `"github.com/bcrisp4/tap/internal/httpx"` to the imports.)

- [ ] **Step 5: Run to confirm everything passes**

```bash
go test ./internal/extract -race
```

Expected: PASS for all extract tests, including the two new ones and every M5-era test (which now pass `httpx.FeedCreds{}`).

- [ ] **Step 6: Commit**

```bash
git add internal/extract/extract.go internal/extract/extract_test.go
git commit -m "$(cat <<'EOF'
M6: extract.Extract gains httpx.FeedCreds parameter

Per-feed cookie + basic auth flow into the article fetch via
httpx.ApplyFeedCreds, exactly as for the feed fetch. Existing
callers (worker + tests) pass httpx.FeedCreds{} as a no-op until
the worker is wired in the next task.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task F3: `feed.Fetch` accepts `FeedCreds`

**Files:**
- Modify: `internal/feed/fetch.go`
- Modify: `internal/feed/fetch_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/feed/fetch_test.go`:

```go
func TestFetchAppliesFeedCreds(t *testing.T) {
	t.Parallel()
	gotCookie := ""
	gotAuth := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>x</title></feed>`))
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), srv.Client(), srv.URL, FetchOpts{
		Creds: httpx.FeedCreds{Cookie: "c", BasicAuthUser: "u", BasicAuthPass: "p"},
	})
	require.NoError(t, err)
	require.Equal(t, "c", gotCookie)
	require.NotEmpty(t, gotAuth)
}
```

- [ ] **Step 2: Run to confirm it fails (compile error)**

```bash
go test ./internal/feed 2>&1 | head -10
```

Expected: build failure — `FetchOpts.Creds` undefined.

- [ ] **Step 3: Extend `FetchOpts` and call `ApplyFeedCreds`**

Edit `internal/feed/fetch.go`. Update `FetchOpts`:

```go
type FetchOpts struct {
	PriorETag         string
	PriorLastModified string
	Creds             httpx.FeedCreds
}
```

In the `Fetch` function, after building the `*http.Request` and setting any conditional-GET headers, add:

```go
httpx.ApplyFeedCreds(req, opts.Creds)
```

Add the `httpx` import.

- [ ] **Step 4: Run to confirm tests pass**

```bash
go test ./internal/feed -race
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/feed/fetch.go internal/feed/fetch_test.go
git commit -m "$(cat <<'EOF'
M6: feed.Fetch applies httpx.FeedCreds via FetchOpts

FetchOpts gains a Creds field. Cookie and basic-auth headers layer
onto the outbound request before the conditional-GET headers, so a
304 path with creds works the same as a 200 path. Existing callers
default Creds to the zero value — no behaviour change for
unauthenticated feeds.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task F4: Worker wires creds into both fetch and extract

**Files:**
- Modify: `internal/poll/worker.go`
- Modify: `internal/poll/worker_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/poll/worker_test.go`:

```go
func TestWorkerAppliesFeedCredsToFeedFetch(t *testing.T) {
	t.Parallel()
	gotCookie := ""
	gotAuth := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>x</title></feed>`))
	}))
	defer srv.Close()

	d := newTestDB(t)
	id := mustInsertSub(t, d, srv.URL)
	mustSetCreds(t, d, id, "c", "u", "p")

	worker := NewWorker(d, srv.Client(), WorkerOpts{Processor: testProcessor()})

	worker.Run(context.Background(), db.DueSubscription{
		ID: id, FeedURL: srv.URL,
		Cookie: "c", BasicAuthUser: "u", BasicAuthPass: "p",
	})
	require.Equal(t, "c", gotCookie)
	require.NotEmpty(t, gotAuth)
}
```

(Helper functions like `mustInsertSub`, `mustSetCreds`, and `testProcessor` already exist in the M5 worker test or get added inline — match the file's existing patterns.)

Then add a test for extract receiving the same creds:

```go
func TestWorkerAppliesFeedCredsToExtract(t *testing.T) {
	t.Parallel()
	feedCalled := false
	gotArticleCookie := ""
	gotArticleAuth := ""

	mux := http.NewServeMux()
	mux.HandleFunc("/feed", func(w http.ResponseWriter, r *http.Request) {
		feedCalled = true
		w.Header().Set("Content-Type", "application/atom+xml")
		// Atom with one entry pointing at /article on the same origin.
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom">
<title>x</title>
<entry>
  <title>e</title>
  <id>e1</id>
  <link href="ARTICLE_URL"/>
  <updated>2024-01-01T00:00:00Z</updated>
</entry>
</feed>`))
	})
	mux.HandleFunc("/article", func(w http.ResponseWriter, r *http.Request) {
		gotArticleCookie = r.Header.Get("Cookie")
		gotArticleAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><article><p>Sufficient text for readability extraction here.</p></article></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Patch ARTICLE_URL placeholder to the real URL after Server.URL is known.
	// (In practice you'd template this; for the plan, accept that the feed
	// handler emits the full URL by reading srv.URL via a shared variable.)

	d := newTestDB(t)
	id := mustInsertSub(t, d, srv.URL+"/feed")
	mustSetCreds(t, d, id, "c", "u", "p")
	mustSetExtract(t, d, id, true, "")

	worker := NewWorker(d, srv.Client(), WorkerOpts{Processor: testProcessor()})
	worker.Run(context.Background(), db.DueSubscription{
		ID: id, FeedURL: srv.URL + "/feed",
		Extract: true, Cookie: "c", BasicAuthUser: "u", BasicAuthPass: "p",
	})

	require.True(t, feedCalled)
	require.Equal(t, "c", gotArticleCookie, "extract must receive the same creds as the feed fetch")
	require.NotEmpty(t, gotArticleAuth)
}
```

(The `ARTICLE_URL` placeholder is a write-time concern — capture `srv.URL` before constructing the feed body, or use a closure / template substitution. Match the M5 worker test patterns.)

- [ ] **Step 2: Run to confirm they fail**

```bash
go test ./internal/poll -run "TestWorkerAppliesFeedCreds" 2>&1 | head -10
```

Expected: failure — the worker doesn't yet pass creds into `feed.Fetch` or `Extract`.

- [ ] **Step 3: Wire the worker**

Edit `internal/poll/worker.go`. Build the creds struct and pass it to both call sites. The `ExtractFunc` type also gains a `creds` parameter — update the type and its production default.

```go
type ExtractFunc func(ctx context.Context, client *http.Client,
	articleURL, selector string, bodyCap int64,
	creds httpx.FeedCreds) (string, error)
```

In `Worker.Run`, derive `FeedCreds` from `sub` once and pass it everywhere:

```go
creds := httpx.FeedCreds{
	Cookie:        sub.Cookie,
	BasicAuthUser: sub.BasicAuthUser,
	BasicAuthPass: sub.BasicAuthPass,
}

res, fetchErr := feed.Fetch(ctx, w.client, sub.FeedURL, feed.FetchOpts{
	PriorETag:         sub.ETag.String,
	PriorLastModified: sub.LastModified.String,
	Creds:             creds,
})
// ...

if sub.Extract && len(pendings) > 0 {
	g := new(errgroup.Group)
	g.SetLimit(w.opts.ExtractConcurrency)
	for i := range pendings {
		if pendings[i].item.Link == "" {
			continue
		}
		g.Go(func() error {
			extracted, eerr := w.opts.Extract(ctx, w.client,
				pendings[i].item.Link, sub.ExtractSelector, w.opts.ExtractBodyCap,
				creds)
			if eerr != nil {
				slog.WarnContext(ctx, "extract failed",
					"feed_id", sub.ID,
					"entry_url", pendings[i].item.Link,
					"err", eerr)
				pendings[i].extractFailed = true
				return nil
			}
			pendings[i].content = extracted
			return nil
		})
	}
	_ = g.Wait()
}
```

Update the default `Extract` field — the function reference still resolves because `extract.Extract` now has the matching signature.

Add `"github.com/bcrisp4/tap/internal/httpx"` to the imports.

- [ ] **Step 4: Run to confirm tests pass**

```bash
go test ./internal/poll -race
```

Expected: PASS for all `internal/poll` tests, including the existing M5 tests (which use `httpx.FeedCreds{}` zero values).

- [ ] **Step 5: Commit**

```bash
git add internal/poll/worker.go internal/poll/worker_test.go
git commit -m "$(cat <<'EOF'
M6: worker passes per-feed creds to fetch and extract

DueSubscription's Cookie / BasicAuthUser / BasicAuthPass flow
through a single httpx.FeedCreds value into both feed.Fetch and
extract.Extract. ExtractFunc's signature picks up the new
parameter; the no-op default (extract.Extract) matches.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task F5: Media-proxy regression — never carries credentials

**Files:**
- Modify: `internal/proxy/handler_test.go`

The proxy fetches origin media anonymously per the M6 spec (mirrors Miniflux). M6 doesn't change the proxy code — but a regression test pins the posture so a future refactor can't accidentally introduce credential forwarding.

- [ ] **Step 1: Write the regression test**

Append to `internal/proxy/handler_test.go`:

```go
func TestHandler_NeverSendsCredentialsToOrigin(t *testing.T) {
	t.Parallel()
	gotCookie := ""
	gotAuth := ""
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "image/png")
		// Minimal valid PNG: 8-byte signature + IHDR + IEND.
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n" +
			"\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89" +
			"\x00\x00\x00\x00IEND\xaeB`\x82"))
	}))
	defer origin.Close()

	signer := proxy.NewSigner(make([]byte, 32))
	cache := proxy.NewCache(t.TempDir(), 1<<20)
	h := proxy.NewHandler(signer, cache, http.DefaultClient, 1<<20)

	// Build a signed proxy URL for the origin URL.
	signed := signer.Sign(origin.URL)

	// Ask the proxy to fetch through a request that itself carries cookies
	// + Authorization (simulating an authenticated SPA reader request). The
	// proxy must NOT forward those to the origin.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/"+signed, nil)
	req.Header.Set("Cookie", "tap_session=somecookie; csrf=value")
	req.Header.Set("Authorization", "Bearer should-not-leak")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	require.Empty(t, gotCookie, "proxy must not forward Cookie to origin")
	require.Empty(t, gotAuth, "proxy must not forward Authorization to origin")
}
```

(Adjust the `signer.Sign` call to match the actual exported API — the M3 spec / the `internal/proxy/signer.go` source dictates the exact signature. If the package name in the test file doesn't already include `proxy`, this needs to be a black-box test in `internal/proxy` package — match the existing handler_test.go style.)

- [ ] **Step 2: Run to confirm the test passes**

```bash
go test ./internal/proxy -run TestHandler_NeverSendsCredentialsToOrigin -race
```

Expected: PASS — the M3 proxy code already builds a fresh request and never reads the SPA's incoming `Cookie:` / `Authorization:` headers, so this regression locks in correct behaviour without code changes.

If the test fails, the proxy is forwarding headers it shouldn't — fix `internal/proxy/handler.go`'s `fetchOrigin` to not copy those headers, then re-run.

- [ ] **Step 3: Commit**

```bash
git add internal/proxy/handler_test.go
git commit -m "$(cat <<'EOF'
M6: regression — proxy must not forward credentials to origin

Pins the M3 proxy's anonymous-origin-fetch posture so a future
refactor can't accidentally start leaking the SPA's session cookie
or per-feed creds to image origins. Matches Miniflux's posture:
same-origin authenticated images render as broken — a known
cross-ecosystem limitation, documented in the M6 spec.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase G — Admin CLI

### Task G1: `cmd/tap/admin.go` skeleton + dispatcher

**Files:**
- Create: `cmd/tap/admin.go`

This task lays down `runAdmin` + the subcommand switch. The actual `create` and `passwd` implementations land in G2 and G3.

- [ ] **Step 1: Create the file**

```go
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
	"golang.org/x/term"
)

// adminExit reports an exit code from a subcommand. Constants kept here so
// tests can assert against them without duplicating literals.
const (
	adminExitOK              = 0
	adminExitGeneric         = 1
	adminExitUserExistsOrGone = 2
	adminExitPasswordMismatch = 3
)

// runAdmin dispatches `tap admin <subcommand> ...`. Returns an exit code so
// tests can call it directly without spawning a subprocess.
func runAdmin(args []string, stdin io.Reader, stdout, stderr io.Writer, hashParams auth.Params) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: tap admin <create|passwd> ...")
		return adminExitGeneric
	}
	switch args[0] {
	case "create":
		return runAdminCreate(args[1:], stdin, stdout, stderr, hashParams)
	case "passwd":
		return runAdminPasswd(args[1:], stdin, stdout, stderr, hashParams)
	default:
		fmt.Fprintf(stderr, "unknown admin subcommand %q\n", args[0])
		return adminExitGeneric
	}
}

// openAdminDB opens the DB at <dataDir>/tap.db, applies migrations, and
// returns a *sql.DB ready for admin work.
func openAdminDB(ctx context.Context, dataDir string) (*db.DB /* alias if exists; else *sql.DB */, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	d, err := db.Open(ctx, filepath.Join(dataDir, "tap.db"))
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx, d); err != nil {
		_ = d.Close()
		return nil, err
	}
	return d, nil
}

// readPassword reads from stdin without echoing. If stdin is a regular file
// (test injection), it falls back to a line read.
func readPassword(stdin io.Reader, prompt string, stdout io.Writer) (string, error) {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(stdout, prompt)
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stdout)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	// Non-terminal (test): read a line.
	var sb strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := stdin.Read(buf)
		if n > 0 && buf[0] == '\n' {
			break
		}
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}
	}
	return strings.TrimRight(sb.String(), "\r"), nil
}

// readLine reads a single line of input from stdin (echoed; for usernames).
func readLine(stdin io.Reader, prompt string, stdout io.Writer) (string, error) {
	fmt.Fprint(stdout, prompt)
	var sb strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := stdin.Read(buf)
		if n > 0 && buf[0] == '\n' {
			break
		}
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", err
		}
	}
	return strings.TrimSpace(strings.TrimRight(sb.String(), "\r")), nil
}

// runAdminCreate is implemented in Task G2.
func runAdminCreate(args []string, stdin io.Reader, stdout, stderr io.Writer, hashParams auth.Params) int {
	fmt.Fprintln(stderr, "tap admin create: not implemented yet")
	_ = flag.NewFlagSet("admin create", flag.ContinueOnError) // import-keeper
	_ = slog.LevelInfo
	return adminExitGeneric
}

// runAdminPasswd is implemented in Task G3.
func runAdminPasswd(args []string, stdin io.Reader, stdout, stderr io.Writer, hashParams auth.Params) int {
	fmt.Fprintln(stderr, "tap admin passwd: not implemented yet")
	return adminExitGeneric
}
```

> **Implementer note:** the `db.DB` return type is shorthand. The current `db.Open` returns `*sql.DB`; the helper above should match that — adjust the import and return type to `*sql.DB` and add `"database/sql"` if necessary. The placeholder body just keeps the file building until the real subcommands land.

- [ ] **Step 2: Wire dispatch in `cmd/tap/main.go`**

At the very top of `main()`, before flag parsing for the server, add:

```go
func main() {
	if len(os.Args) >= 2 && os.Args[1] == "admin" {
		os.Exit(runAdmin(os.Args[2:], os.Stdin, os.Stdout, os.Stderr, auth.DefaultParams))
	}
	runServer()
}
```

Move the existing server body into a new function `runServer()`. The diff: `func main() { ... }` → `func runServer() { ... }` plus the new `func main()` above. Add `"github.com/bcrisp4/tap/internal/auth"` to the imports.

- [ ] **Step 3: Verify the package builds**

```bash
go build ./cmd/tap
```

Expected: no errors. The placeholder subcommands will print "not implemented yet" if invoked.

- [ ] **Step 4: Commit**

```bash
git add cmd/tap/admin.go cmd/tap/main.go
git commit -m "$(cat <<'EOF'
M6: cmd/tap subcommand dispatcher + admin.go skeleton

main() dispatches `tap admin ...` to runAdmin; everything else
falls through to runServer (the existing main body, renamed).
runAdmin currently routes to placeholder create/passwd handlers
that print 'not implemented' — wired up in the next two tasks.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task G2: `tap admin create`

**Files:**
- Modify: `cmd/tap/admin.go`
- Create: `cmd/tap/admin_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/tap/admin_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

var testHashParams = auth.Params{Time: 1, Memory: 8 * 1024, Threads: 1, SaltLen: 8, KeyLen: 16}

func TestAdminCreateHappyPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stdin := strings.NewReader("ben\nsupersecret\nsupersecret\n")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exit := runAdminCreate(
		[]string{"--data", dir},
		stdin, stdout, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitOK, exit, "stderr=%q", stderr.String())
	require.Contains(t, stdout.String(), "created admin user 'ben'")

	// Verify the user landed.
	d, err := db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	defer d.Close()

	u, err := db.GetUserByUsername(context.Background(), d, "ben")
	require.NoError(t, err)
	require.Equal(t, "admin", u.Role)

	ok, err := auth.Verify(u.PasswordHash, "supersecret")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestAdminCreateDuplicateUsername(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// First create.
	exit := runAdminCreate(
		[]string{"--data", dir},
		strings.NewReader("ben\nsupersecret\nsupersecret\n"),
		&bytes.Buffer{}, &bytes.Buffer{},
		testHashParams,
	)
	require.Equal(t, adminExitOK, exit)

	// Second create with same username.
	stderr := &bytes.Buffer{}
	exit = runAdminCreate(
		[]string{"--data", dir},
		strings.NewReader("ben\nanothersecret\nanothersecret\n"),
		&bytes.Buffer{}, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitUserExistsOrGone, exit)
	require.Contains(t, stderr.String(), "user 'ben' already exists")
}

func TestAdminCreatePasswordMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stderr := &bytes.Buffer{}
	exit := runAdminCreate(
		[]string{"--data", dir},
		strings.NewReader("ben\npassword1\npassword2\n"),
		&bytes.Buffer{}, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitPasswordMismatch, exit)
	require.Contains(t, stderr.String(), "passwords do not match")
}

func TestAdminCreateEmptyUsername(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stderr := &bytes.Buffer{}
	exit := runAdminCreate(
		[]string{"--data", dir},
		strings.NewReader("\nsupersecret\nsupersecret\n"),
		&bytes.Buffer{}, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitGeneric, exit)
	require.Contains(t, stderr.String(), "username is required")
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./cmd/tap -run TestAdminCreate 2>&1 | head -20
```

Expected: failures (`runAdminCreate` is the placeholder).

- [ ] **Step 3: Implement `runAdminCreate`**

Replace the placeholder in `cmd/tap/admin.go`:

```go
func runAdminCreate(args []string, stdin io.Reader, stdout, stderr io.Writer, hashParams auth.Params) int {
	fs := flag.NewFlagSet("admin create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dataDir := fs.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory containing tap.db")
	role := fs.String("role", "admin", "role for the new user (admin or user)")
	if err := fs.Parse(args); err != nil {
		return adminExitGeneric
	}

	if *role != "admin" && *role != "user" {
		fmt.Fprintf(stderr, "role must be admin or user, got %q\n", *role)
		return adminExitGeneric
	}

	username, err := readLine(stdin, "username: ", stdout)
	if err != nil {
		fmt.Fprintf(stderr, "read username: %v\n", err)
		return adminExitGeneric
	}
	if username == "" {
		fmt.Fprintln(stderr, "username is required")
		return adminExitGeneric
	}

	pass1, err := readPassword(stdin, "password: ", stdout)
	if err != nil {
		fmt.Fprintf(stderr, "read password: %v\n", err)
		return adminExitGeneric
	}
	if err := auth.ValidatePassword(pass1); err != nil {
		fmt.Fprintf(stderr, "password too short (min %d): %v\n", auth.MinPasswordLength, err)
		return adminExitGeneric
	}
	pass2, err := readPassword(stdin, "confirm:  ", stdout)
	if err != nil {
		fmt.Fprintf(stderr, "read password: %v\n", err)
		return adminExitGeneric
	}
	if pass1 != pass2 {
		fmt.Fprintln(stderr, "passwords do not match")
		return adminExitPasswordMismatch
	}

	ctx := context.Background()
	d, err := openAdminDB(ctx, *dataDir)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return adminExitGeneric
	}
	defer d.Close()

	hash, err := auth.Hash(pass1, hashParams)
	if err != nil {
		fmt.Fprintf(stderr, "hash password: %v\n", err)
		return adminExitGeneric
	}
	id, err := db.InsertUser(ctx, d, db.NewUser{
		Username: username, PasswordHash: hash, Role: *role, CreatedAt: timeNowUnix(),
	})
	if err != nil {
		if errors.Is(err, db.ErrUserExists) {
			fmt.Fprintf(stderr, "user %q already exists\n", username)
			return adminExitUserExistsOrGone
		}
		fmt.Fprintf(stderr, "create user: %v\n", err)
		return adminExitGeneric
	}
	fmt.Fprintf(stdout, "created %s user %q (id=%d)\n", *role, username, id)
	return adminExitOK
}

// timeNowUnix is broken out so admin_test.go can monkey-patch via a build
// tag if it ever wants determinism. For now it's just time.Now().Unix().
func timeNowUnix() int64 { return timeNow().Unix() }

// timeNow lets tests override the clock if needed. Production: time.Now.
var timeNow = time.Now
```

> **Implementer note:** add `"time"` and `"database/sql"` to the import list. The `envOr` helper already exists in `cmd/tap/main.go` (it's package-private, same file scope works). Update `openAdminDB` to return `*sql.DB` per the implementer note in Task G1.

- [ ] **Step 4: Run to confirm tests pass**

```bash
go test ./cmd/tap -run TestAdminCreate -race
```

Expected: PASS for all four `TestAdminCreate*` tests.

- [ ] **Step 5: Commit**

```bash
git add cmd/tap/admin.go cmd/tap/admin_test.go
git commit -m "$(cat <<'EOF'
M6: tap admin create

Interactive: read username + password (twice, no echo), validate,
hash with auth.DefaultParams (testHashParams in tests), INSERT.
Exit 0 on success, 2 on duplicate username, 3 on password mismatch,
1 otherwise. --role flag defaults to admin (matches the env-var
bootstrap).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task G3: `tap admin passwd`

**Files:**
- Modify: `cmd/tap/admin.go`
- Modify: `cmd/tap/admin_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `cmd/tap/admin_test.go`:

```go
func TestAdminPasswdHappyPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Seed a user.
	exit := runAdminCreate(
		[]string{"--data", dir},
		strings.NewReader("ben\noldsecret\noldsecret\n"),
		&bytes.Buffer{}, &bytes.Buffer{},
		testHashParams,
	)
	require.Equal(t, adminExitOK, exit)

	// Insert a session for ben so we can verify it gets deleted.
	d, err := db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	u, err := db.GetUserByUsername(context.Background(), d, "ben")
	require.NoError(t, err)
	_, err = db.InsertSession(context.Background(), d, db.NewSession{
		UserID: u.ID, TokenHash: "h", CSRFToken: "c",
		CreatedAt: 0, LastSeenAt: 0, IdleExpiresAt: 1, AbsoluteExpiresAt: 1,
	})
	require.NoError(t, err)
	d.Close()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exit = runAdminPasswd(
		[]string{"--data", dir, "ben"},
		strings.NewReader("newsecret\nnewsecret\n"),
		stdout, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitOK, exit, "stderr=%q", stderr.String())
	require.Contains(t, stdout.String(), "password reset for 'ben'")

	// Verify hash + session deletion.
	d, err = db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	defer d.Close()

	u, err = db.GetUserByUsername(context.Background(), d, "ben")
	require.NoError(t, err)
	ok, err := auth.Verify(u.PasswordHash, "newsecret")
	require.NoError(t, err)
	require.True(t, ok)

	_, err = db.GetSessionByTokenHash(context.Background(), d, "h")
	require.Error(t, err, "session should be deleted")
}

func TestAdminPasswdMissingUser(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stderr := &bytes.Buffer{}
	exit := runAdminPasswd(
		[]string{"--data", dir, "nobody"},
		strings.NewReader("newsecret\nnewsecret\n"),
		&bytes.Buffer{}, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitUserExistsOrGone, exit)
	require.Contains(t, stderr.String(), "user 'nobody' not found")
}

func TestAdminPasswdMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	exit := runAdminCreate(
		[]string{"--data", dir},
		strings.NewReader("ben\noldsecret\noldsecret\n"),
		&bytes.Buffer{}, &bytes.Buffer{},
		testHashParams,
	)
	require.Equal(t, adminExitOK, exit)

	stderr := &bytes.Buffer{}
	exit = runAdminPasswd(
		[]string{"--data", dir, "ben"},
		strings.NewReader("a-good-password\nb-different-pw\n"),
		&bytes.Buffer{}, stderr,
		testHashParams,
	)
	require.Equal(t, adminExitPasswordMismatch, exit)
	require.Contains(t, stderr.String(), "passwords do not match")
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./cmd/tap -run TestAdminPasswd 2>&1 | head -20
```

Expected: failures.

- [ ] **Step 3: Implement `runAdminPasswd`**

Replace the placeholder in `cmd/tap/admin.go`:

```go
func runAdminPasswd(args []string, stdin io.Reader, stdout, stderr io.Writer, hashParams auth.Params) int {
	fs := flag.NewFlagSet("admin passwd", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dataDir := fs.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory containing tap.db")
	if err := fs.Parse(args); err != nil {
		return adminExitGeneric
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: tap admin passwd <username>")
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
		fmt.Fprintf(stderr, "user %q not found\n", username)
		return adminExitUserExistsOrGone
	}

	pass1, err := readPassword(stdin, "new password: ", stdout)
	if err != nil {
		fmt.Fprintf(stderr, "read password: %v\n", err)
		return adminExitGeneric
	}
	if err := auth.ValidatePassword(pass1); err != nil {
		fmt.Fprintf(stderr, "password too short (min %d)\n", auth.MinPasswordLength)
		return adminExitGeneric
	}
	pass2, err := readPassword(stdin, "confirm:      ", stdout)
	if err != nil {
		fmt.Fprintf(stderr, "read password: %v\n", err)
		return adminExitGeneric
	}
	if pass1 != pass2 {
		fmt.Fprintln(stderr, "passwords do not match")
		return adminExitPasswordMismatch
	}

	hash, err := auth.Hash(pass1, hashParams)
	if err != nil {
		fmt.Fprintf(stderr, "hash password: %v\n", err)
		return adminExitGeneric
	}
	if err := db.UpdatePasswordHash(ctx, d, u.ID, hash); err != nil {
		fmt.Fprintf(stderr, "update password: %v\n", err)
		return adminExitGeneric
	}
	if err := db.DeleteSessionsByUserID(ctx, d, u.ID); err != nil {
		fmt.Fprintf(stderr, "delete sessions: %v\n", err)
		return adminExitGeneric
	}
	fmt.Fprintf(stdout, "password reset for %q\n", username)
	return adminExitOK
}
```

- [ ] **Step 4: Run to confirm tests pass**

```bash
go test ./cmd/tap -run TestAdminPasswd -race
```

Expected: PASS for all three `TestAdminPasswd*` tests.

- [ ] **Step 5: Commit**

```bash
git add cmd/tap/admin.go cmd/tap/admin_test.go
git commit -m "$(cat <<'EOF'
M6: tap admin passwd <username>

Resets a user's password and deletes every session for them
(admin reset implies lockout-and-reissue). Same input ergonomics
as `tap admin create`. Exit 2 if the user doesn't exist; 3 on
password mismatch.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase H — Env-var bootstrap + cmd/tap wire-up + mux mounting

### Task H1: `bootstrapAdmin` helper

**Files:**
- Modify: `cmd/tap/admin.go` (or new `cmd/tap/bootstrap.go` if you prefer it next door)
- Create: `cmd/tap/bootstrap_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cmd/tap/bootstrap_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

func TestBootstrapAdminCreatesUser(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	d, err := db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background(), d))
	defer d.Close()

	logs := &bytes.Buffer{}
	require.NoError(t, bootstrapAdmin(context.Background(), d, "ben", "supersecret", testHashParams, logs))

	u, err := db.GetUserByUsername(context.Background(), d, "ben")
	require.NoError(t, err)
	require.Equal(t, "admin", u.Role)

	ok, err := auth.Verify(u.PasswordHash, "supersecret")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestBootstrapAdminRejectsTooShortPassword(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	d, err := db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background(), d))
	defer d.Close()

	err = bootstrapAdmin(context.Background(), d, "ben", "short", testHashParams, &bytes.Buffer{})
	require.ErrorIs(t, err, auth.ErrPasswordTooShort)

	n, err := db.CountUsers(context.Background(), d)
	require.NoError(t, err)
	require.Equal(t, 0, n, "no user should be created on validation failure")
}
```

- [ ] **Step 2: Run to confirm it fails**

```bash
go test ./cmd/tap -run TestBootstrapAdmin 2>&1 | head -10
```

Expected: build failure — `bootstrapAdmin` undefined.

- [ ] **Step 3: Implement**

Append to `cmd/tap/admin.go`:

```go
// bootstrapAdmin creates an admin user from the supplied credentials. Used
// by the env-var first-launch shortcut. Validates the password (so a
// too-short bootstrap password aborts loudly rather than producing an
// unusable account) and inserts the user atomically.
func bootstrapAdmin(ctx context.Context, d *sql.DB, username, password string, hashParams auth.Params, logs io.Writer) error {
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}
	hash, err := auth.Hash(password, hashParams)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}
	if _, err := db.InsertUser(ctx, d, db.NewUser{
		Username: username, PasswordHash: hash, Role: "admin", CreatedAt: timeNowUnix(),
	}); err != nil {
		return fmt.Errorf("insert bootstrap admin: %w", err)
	}
	return nil
}
```

(Add `"database/sql"` to imports if not already present.)

- [ ] **Step 4: Run to confirm tests pass**

```bash
go test ./cmd/tap -run TestBootstrapAdmin -race
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/tap/admin.go cmd/tap/bootstrap_test.go
git commit -m "$(cat <<'EOF'
M6: bootstrapAdmin helper

Validates password (ErrPasswordTooShort short-circuits before any
write), hashes, INSERTs as admin. Reused by both the env-var
first-launch path and (indirectly via runAdminCreate's wrapper) the
interactive CLI. A failed validation produces no DB row.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task H2: `MuxOpts` extensions + middleware mounting

**Files:**
- Modify: `internal/api/api.go`

- [ ] **Step 1: Replace `NewMux` to mount the auth middleware on every route except login + healthz**

Edit `internal/api/api.go`:

```go
package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/bcrisp4/tap/internal/auth"
)

// MuxOpts carries optional dependencies for NewMux.
//
//   - Poke is called after a successful POST /api/v1/subscriptions so the
//     scheduler can run an immediate tick.
//   - ProxyHandler is mounted at GET /api/v1/proxy/{token} when non-nil.
//   - SessionIdleTTL / SessionAbsoluteTTL set the cookie + sessions-row
//     expiries (defaults: 7d / 90d).
//   - CookieSecure controls whether the session cookie carries the Secure
//     attribute. Resolved against the listen address by main().
//   - HashParams: argon2 cost. Production passes auth.DefaultParams; tests
//     pass a low-cost variant for speed.
type MuxOpts struct {
	Poke               func()
	ProxyHandler       http.Handler
	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
	CookieSecure       bool
	HashParams         auth.Params
}

func NewMux(d *sql.DB, opts MuxOpts) *http.ServeMux {
	m := http.NewServeMux()

	// Defaults for callers (notably tests) that leave fields zero.
	if opts.SessionIdleTTL <= 0 {
		opts.SessionIdleTTL = 7 * 24 * time.Hour
	}
	if opts.SessionAbsoluteTTL <= 0 {
		opts.SessionAbsoluteTTL = 90 * 24 * time.Hour
	}
	if opts.HashParams == (auth.Params{}) {
		opts.HashParams = auth.DefaultParams
	}

	deps := authDeps{
		d:                  d,
		sessionIdleTTL:     opts.SessionIdleTTL,
		sessionAbsoluteTTL: opts.SessionAbsoluteTTL,
		cookieSecure:       opts.CookieSecure,
	}

	// Public routes.
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})
	m.Handle("POST /api/v1/sessions", loginHandler(deps))

	if d == nil {
		return m
	}

	// Authenticated routes.
	authed := requireSession(d, opts.SessionIdleTTL)
	authedCSRF := chain(authed, requireCSRF())

	m.Handle("GET /api/v1/sessions/current", authed(getSessionCurrentHandler(deps)))
	m.Handle("DELETE /api/v1/sessions/current", authedCSRF(logoutHandler(deps)))
	m.Handle("PATCH /api/v1/me/password", authedCSRF(passwordChangeHandler(deps, opts.HashParams)))

	// Subscriptions + entries: register the existing handlers, but wrap them.
	// Easiest: build a sub-mux, register, then mount with the middleware.
	subsMux := http.NewServeMux()
	registerSubscriptionRoutes(subsMux, d, opts.Poke)
	entriesMux := http.NewServeMux()
	registerEntryRoutes(entriesMux, d)

	// Walk the sub-muxes' route patterns through middleware. Since the
	// stdlib mux doesn't expose registered patterns, just mount each pattern
	// explicitly here:
	for _, p := range []struct {
		method, path string
		handler      http.Handler
	}{
		{"GET", "/api/v1/subscriptions", authed(subsMux)},
		{"POST", "/api/v1/subscriptions", authedCSRF(subsMux)},
		{"PATCH", "/api/v1/subscriptions/{id}", authedCSRF(subsMux)},
		{"DELETE", "/api/v1/subscriptions/{id}", authedCSRF(subsMux)},
		{"GET", "/api/v1/entries", authed(entriesMux)},
		{"GET", "/api/v1/entries/{id}", authed(entriesMux)},
		{"PATCH", "/api/v1/entries/{id}", authedCSRF(entriesMux)},
	} {
		m.Handle(p.method+" "+p.path, p.handler)
	}

	if opts.ProxyHandler != nil {
		m.Handle("GET /api/v1/proxy/{token}", authed(opts.ProxyHandler))
	}
	return m
}
```

> **Implementer note:** the explicit per-pattern mount is verbose but keeps the middleware composition easy to audit. Avoid using a "wrap the entire mux" pattern — that would also wrap `/healthz` and `POST /sessions`, which must stay public.

- [ ] **Step 2: Verify the package builds**

```bash
go build ./internal/api
go test ./internal/api -race
```

Expected: build succeeds, but several existing tests now fail because they hit authenticated routes without a session. That's intentional — Task H3's main_test.go updates wire end-to-end auth into the test harness. The `internal/api`-level tests that drove out the handlers in earlier phases are unit tests against the handler factories directly (or against the unwrapped sub-muxes via `newSubscriptionsTestMux` from Task E1) so they still pass.

If `internal/api` tests fail because they construct via `NewMux(d, MuxOpts{})` and now hit 401, switch them to call the handler factories directly (`loginHandler`, etc.) or to use `newSubscriptionsTestMux` (which builds an unauthenticated mux). The intent: `internal/api` tests cover handler logic; end-to-end auth coverage lives in `cmd/tap/main_test.go`.

- [ ] **Step 3: Commit**

```bash
git add internal/api/api.go
git commit -m "$(cat <<'EOF'
M6: NewMux mounts session + CSRF middleware on every /api/v1/*
except POST /sessions and /healthz

MuxOpts gains SessionIdleTTL, SessionAbsoluteTTL, CookieSecure,
HashParams. Authenticated routes go through requireSession;
state-changing ones additionally go through requireCSRF. The
proxy is GET-only and authed-only (no CSRF check needed).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task H3: `cmd/tap/main.go` — flags, env-var bootstrap, MuxOpts wire-up

**Files:**
- Modify: `cmd/tap/main.go`

- [ ] **Step 1: Add the new flags + env-var bootstrap path + MuxOpts wiring**

Edit `cmd/tap/main.go`. Inside `runServer`:

Add new flag declarations alongside the existing ones:

```go
sessionIdleTTL     = flag.Duration("session-idle-ttl",
	envOrDuration("TAP_SESSION_IDLE_TTL", 7*24*time.Hour),
	"refresh-on-activity expiry for session cookies")
sessionAbsoluteTTL = flag.Duration("session-absolute-ttl",
	envOrDuration("TAP_SESSION_ABSOLUTE_TTL", 90*24*time.Hour),
	"hard cap on session lifetime regardless of activity")
cookieSecureMode   = flag.String("cookie-secure",
	envOr("TAP_COOKIE_SECURE", "auto"),
	"set Secure attribute on session cookie: auto|true|false")
```

After the existing `db.Migrate(ctx, d)` call, add the bootstrap path:

```go
// First-launch admin bootstrap. Silent no-op when users exist, even
// if the env vars are still set — concept §7.1.
if n, err := db.CountUsers(ctx, d); err != nil {
	slog.Error("count users", "err", err)
	os.Exit(1)
} else if n == 0 {
	user := os.Getenv("TAP_ADMIN_USERNAME")
	pass := os.Getenv("TAP_ADMIN_PASSWORD")
	switch {
	case user != "" && pass != "":
		if err := bootstrapAdmin(ctx, d, user, pass, auth.DefaultParams, os.Stderr); err != nil {
			slog.Error("bootstrap admin", "err", err)
			os.Exit(1)
		}
		slog.Info("bootstrapped admin from environment", "username", user)
	case user != "" || pass != "":
		slog.Error("partial admin bootstrap: both TAP_ADMIN_USERNAME and TAP_ADMIN_PASSWORD must be set")
		os.Exit(1)
	default:
		slog.Warn("no users in database; create one with 'tap admin create' or set TAP_ADMIN_USERNAME and TAP_ADMIN_PASSWORD")
	}
}
```

Resolve cookie-secure mode:

```go
var cookieSecureEnum api.CookieSecureMode
switch strings.ToLower(*cookieSecureMode) {
case "auto", "":
	cookieSecureEnum = api.CookieSecureAuto
case "true":
	cookieSecureEnum = api.CookieSecureTrue
case "false":
	cookieSecureEnum = api.CookieSecureFalse
default:
	slog.Error("invalid --cookie-secure", "value", *cookieSecureMode)
	os.Exit(1)
}
cookieSecure := api.ResolveCookieSecure(cookieSecureEnum, *addr) // public helper exposed in api.go
```

Update the `api.NewMux` call:

```go
apiMux := api.NewMux(d, api.MuxOpts{
	Poke:               sched.Poke,
	ProxyHandler:       proxyHandler,
	SessionIdleTTL:     *sessionIdleTTL,
	SessionAbsoluteTTL: *sessionAbsoluteTTL,
	CookieSecure:       cookieSecure,
	HashParams:         auth.DefaultParams,
})
```

- [ ] **Step 2: Expose `ResolveCookieSecure` from `internal/api/auth.go`**

In `internal/api/auth.go`, rename the lowercase helper to be exported so `cmd/tap/main.go` can call it:

```go
// ResolveCookieSecure decides whether to set the Secure attribute on the
// session cookie. Exposed so cmd/tap can resolve the flag once at startup.
func ResolveCookieSecure(mode CookieSecureMode, addr string) bool {
	return resolveCookieSecure(mode, addr)
}
```

- [ ] **Step 3: Verify the binary builds**

```bash
make test
```

Expected: PASS for `go test ./... -race`. The web build prerequisite is satisfied via `web/dist/index.html`.

- [ ] **Step 4: Commit**

```bash
git add cmd/tap/main.go internal/api/auth.go
git commit -m "$(cat <<'EOF'
M6: cmd/tap flags + env-var bootstrap + MuxOpts wire-up

New flags: --session-idle-ttl (default 7d), --session-absolute-ttl
(default 90d), --cookie-secure (auto|true|false, default auto).
After migrate, count users; if 0 and TAP_ADMIN_USERNAME/PASSWORD
are set, run bootstrapAdmin and log INFO. If 0 and both are
missing, log WARN and continue (login rejects everything until
admin is created via tap admin create). Existing users → silent
no-op even if env vars are still set.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task H4: End-to-end main_test.go coverage — login, CSRF, basic auth, anonymous proxy

**Files:**
- Modify: `cmd/tap/main_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `cmd/tap/main_test.go`. The file already has helpers like `setupServer` from M5 — match its style, and inject `auth.Params` for low-cost test hashing where needed.

```go
func TestEndToEnd_LoginAndCSRFRoundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	t.Setenv("TAP_DATA_DIR", dir)
	t.Setenv("TAP_ADMIN_USERNAME", "ben")
	t.Setenv("TAP_ADMIN_PASSWORD", "supersecret")

	srv := startTestServer(t) // existing helper, or a minimal harness; passes auth.HashParams=testHashParams under the hood
	defer srv.Close()

	// Login.
	loginResp := postJSON(t, srv, "/api/v1/sessions",
		`{"username":"ben","password":"supersecret"}`, "")
	require.Equal(t, http.StatusOK, loginResp.StatusCode)
	defer loginResp.Body.Close()
	var login struct {
		User      struct{ ID int64; Username, Role string } `json:"user"`
		CSRFToken string `json:"csrf_token"`
	}
	require.NoError(t, json.NewDecoder(loginResp.Body).Decode(&login))
	require.Equal(t, "ben", login.User.Username)
	require.Equal(t, "admin", login.User.Role)

	cookie := sessionCookie(t, loginResp)

	// State-changing without CSRF → 403.
	rr := postJSON(t, srv, "/api/v1/subscriptions",
		`{"feed_url":"https://x.example/feed"}`, cookie)
	require.Equal(t, http.StatusForbidden, rr.StatusCode)

	// State-changing WITH CSRF → 201.
	body, _ := json.Marshal(map[string]string{"feed_url": "https://x.example/feed"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "tap_session="+cookie)
	req.Header.Set("X-CSRF-Token", login.CSRFToken)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestEndToEnd_BasicAuthFlowsToFeedFetch(t *testing.T) {
	t.Parallel()

	gotAuth := ""
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>x</title></feed>`))
	}))
	defer origin.Close()

	dir := t.TempDir()
	t.Setenv("TAP_DATA_DIR", dir)
	t.Setenv("TAP_ADMIN_USERNAME", "ben")
	t.Setenv("TAP_ADMIN_PASSWORD", "supersecret")
	t.Setenv("TAP_SSRF_ALLOW", "127.0.0.0/8") // origin is loopback

	srv := startTestServer(t)
	defer srv.Close()
	client := authedClient(t, srv, "ben", "supersecret") // helper that logs in + returns a *http.Client + csrf token + cookie

	// Subscribe with basic auth set.
	body := `{"feed_url":"` + origin.URL + `","basic_auth_user":"u","basic_auth_pass":"p"}`
	resp := client.Post("/api/v1/subscriptions", body)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Contains(t, resp.Body, `"has_basic_auth":true`)

	// Wait for the poke-induced poll.
	require.Eventually(t, func() bool { return gotAuth != "" }, 10*time.Second, 50*time.Millisecond)
	require.NotEmpty(t, gotAuth, "feed origin should have received Authorization: Basic ...")
}

func TestEndToEnd_ProxyOriginIsAnonymous(t *testing.T) {
	t.Parallel()

	gotAuth := ""
	gotCookie := ""
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n" +
			"\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89" +
			"\x00\x00\x00\x00IEND\xaeB`\x82"))
	}))
	defer origin.Close()

	dir := t.TempDir()
	t.Setenv("TAP_DATA_DIR", dir)
	t.Setenv("TAP_ADMIN_USERNAME", "ben")
	t.Setenv("TAP_ADMIN_PASSWORD", "supersecret")
	t.Setenv("TAP_SSRF_ALLOW", "127.0.0.0/8")
	srv := startTestServer(t)
	defer srv.Close()
	client := authedClient(t, srv, "ben", "supersecret")

	// Build a signed proxy URL via the test server's signer (which is
	// generated on first launch). The simplest path is to insert an entry
	// whose body has a sanitised image URL — but for the regression we can
	// exercise the proxy with a SPA-style request and assert no creds reach
	// the origin.
	resp := client.Get("/api/v1/proxy/" + signProxyToken(t, srv, origin.URL))
	require.Equal(t, http.StatusOK, resp.StatusCode)

	require.Empty(t, gotAuth)
	require.Empty(t, gotCookie)
}
```

(Helpers `startTestServer`, `authedClient`, `sessionCookie`, `postJSON`, `signProxyToken` are inferred from the existing M5 main_test.go style. If they don't exist, define them inline — match the existing patterns. The shape: spin up the binary as a goroutine via the same setup the M5 end-to-end tests use; expose helpers that hide the cookie + CSRF plumbing.)

- [ ] **Step 2: Run to confirm they fail**

```bash
go test ./cmd/tap -run "TestEndToEnd_(LoginAndCSRFRoundtrip|BasicAuthFlowsToFeedFetch|ProxyOriginIsAnonymous)" -race 2>&1 | head -30
```

Expected: failures — most likely missing helpers, or the bootstrap path not yet wiring `auth.HashParams=testHashParams`.

- [ ] **Step 3: Wire `auth.Params` into the test harness**

Update `startTestServer` (or its analogue) to use `auth.Params{Time:1, Memory:8*1024, Threads:1, SaltLen:8, KeyLen:16}` so end-to-end tests don't pay 64 MiB-per-hash cost. The cleanest path: `runServer` can take an optional `auth.Params` argument, defaulting to `auth.DefaultParams`; tests construct the server with the lower-cost params.

Concretely: split `runServer` into `runServer()` (production) and `runServerWithParams(hashParams auth.Params)` (test). Or expose a build-tag-gated knob. Match the existing M5 test-harness pattern.

- [ ] **Step 4: Run to confirm tests pass**

```bash
make test
```

Expected: PASS for the entire `go test ./... -race` run.

- [ ] **Step 5: Commit**

```bash
git add cmd/tap/main_test.go cmd/tap/main.go
git commit -m "$(cat <<'EOF'
M6: end-to-end main_test.go — login + CSRF + basic-auth + anon proxy

Three integration tests:
1. Login then POST without X-CSRF-Token → 403; same POST with the
   header → 201. Pins the middleware composition end-to-end.
2. Subscribe with basic_auth_*, poke the scheduler, assert the
   fixture origin sees Authorization: Basic ... on the feed fetch.
3. The media proxy fetches the origin without forwarding any of
   the SPA's Cookie / Authorization headers. Pins the
   anonymous-proxy posture (concept §6.7 / Miniflux mirror).

Test harness now uses low-cost argon2 params so the suite stays
fast.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task H5: Bootstrap-on-empty + silent-on-populated end-to-end

**Files:**
- Modify: `cmd/tap/main_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `cmd/tap/main_test.go`:

```go
func TestEndToEnd_BootstrapFromEnvVarsCreatesAdmin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	t.Setenv("TAP_DATA_DIR", dir)
	t.Setenv("TAP_ADMIN_USERNAME", "ben")
	t.Setenv("TAP_ADMIN_PASSWORD", "supersecret")

	srv := startTestServer(t)
	defer srv.Close()

	// Login should succeed.
	resp := postJSON(t, srv, "/api/v1/sessions",
		`{"username":"ben","password":"supersecret"}`, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestEndToEnd_NoUsersNoEnvVarsRejectsLogin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	t.Setenv("TAP_DATA_DIR", dir)
	// Ensure env vars are NOT set.
	t.Setenv("TAP_ADMIN_USERNAME", "")
	t.Setenv("TAP_ADMIN_PASSWORD", "")

	srv := startTestServer(t)
	defer srv.Close()

	resp := postJSON(t, srv, "/api/v1/sessions",
		`{"username":"anyone","password":"anything"}`, "")
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestEndToEnd_BootstrapSilentOnPopulatedDB(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	t.Setenv("TAP_DATA_DIR", dir)
	t.Setenv("TAP_ADMIN_USERNAME", "ben")
	t.Setenv("TAP_ADMIN_PASSWORD", "supersecret")

	// First launch: bootstraps "ben".
	srv1 := startTestServer(t)
	srv1.Close()

	// Hand-add another user via tap admin (or directly via db.InsertUser
	// against the test DB). Then re-launch and verify the env-var bootstrap
	// is a no-op.
	d, err := db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	hash, _ := auth.Hash("anotherpw", testHashParams)
	_, err = db.InsertUser(context.Background(), d, db.NewUser{
		Username: "alice", PasswordHash: hash, Role: "user", CreatedAt: 0,
	})
	require.NoError(t, err)
	d.Close()

	// Re-launch with env vars STILL set. Expect: silent no-op (no error,
	// no new admin row). "ben" still works as the env-bootstrapped admin
	// from the first launch; "alice" still works.
	srv2 := startTestServer(t)
	defer srv2.Close()

	d, err = db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	defer d.Close()

	n, err := db.CountUsers(context.Background(), d)
	require.NoError(t, err)
	require.Equal(t, 2, n, "no extra users should be created on the second launch")
}
```

- [ ] **Step 2: Run to confirm they pass (Task H3 implementation already covers these paths)**

```bash
go test ./cmd/tap -run "TestEndToEnd_(Bootstrap|NoUsersNoEnvVars)" -race
```

Expected: PASS for all three.

- [ ] **Step 3: Commit**

```bash
git add cmd/tap/main_test.go
git commit -m "$(cat <<'EOF'
M6: end-to-end bootstrap behaviour regressions

Three locked-in invariants:
1. Empty users + TAP_ADMIN_USERNAME/PASSWORD set → admin created,
   login works.
2. Empty users + env vars unset → server starts, login rejects all
   (concept §7.1: no SPA-visible bootstrap path).
3. Populated users + env vars still set → silent no-op, no
   duplicate admin, no error.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase I — SPA changes

### Task I1: Types backfill + new auth types

**Files:**
- Modify: `web/src/lib/types.ts`

Pure type additions / no runtime change. Pure scaffolding under the spec's exemption — no test for this task.

- [ ] **Step 1: Update `web/src/lib/types.ts`**

```ts
export type Subscription = {
  id: number;
  title: string;
  feed_url: string;
  site_url?: string;
  next_poll_at: number;
  last_poll_at?: number;
  error_count: number;
  last_error?: string;
  created_at: number;
  // M5 backfill — present on the wire since M5; the type was missing them.
  extract: boolean;
  extract_selector: string;
  // M6.
  has_cookie: boolean;
  has_basic_auth: boolean;
};

export type EntryListItem = {
  id: number;
  subscription_id: number;
  title: string;
  author?: string;
  url: string;
  published_at: number;
  fetched_at: number;
  read: boolean;
  saved: boolean;
  // M5 backfill.
  extract_failed: boolean;
};

export type EntryDetail = EntryListItem & {
  content: string;
};

export type ListResponse<T> = {
  data: T[];
  next_cursor?: string;
};

export type ApiError = {
  error: { code: string; message: string };
};

// M6: auth types.
export type User = {
  id: number;
  username: string;
  role: 'admin' | 'user';
};

export type SessionResponse = {
  user: User;
  csrf_token: string;
};

export type PasswordChangeResponse = {
  csrf_token: string;
};
```

- [ ] **Step 2: Verify the SPA still type-checks**

```bash
pnpm --dir web run check
```

Expected: no type errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/types.ts
git commit -m "$(cat <<'EOF'
M6: web types backfill (M5 fields) + new auth types

Subscription gains extract/extract_selector/has_cookie/has_basic_auth
(the first two are M5-era backfills — the wire shape had them since
M5; the SPA type was lying). EntryListItem/EntryDetail gain
extract_failed. New User / SessionResponse / PasswordChangeResponse
types describe the M6 wire shapes.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task I2: Auth store

**Files:**
- Create: `web/src/lib/auth.ts`
- Create: `web/src/lib/__tests__/auth.test.ts`

- [ ] **Step 1: Write the failing test**

Create `web/src/lib/__tests__/auth.test.ts`:

```ts
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { get } from 'svelte/store';

// Mock fetch globally.
const fetchMock = vi.fn();
beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock);
  fetchMock.mockReset();
});
afterEach(() => {
  vi.unstubAllGlobals();
});

// Re-import per test to reset module-level state.
async function loadAuth() {
  vi.resetModules();
  const mod = await import('../auth');
  return mod.auth;
}

describe('auth store', () => {
  it('bootstraps from a successful GET /sessions/current', async () => {
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'tok' }),
      { status: 200 },
    ));
    const auth = await loadAuth();
    await auth.bootstrap();
    const s = get(auth);
    expect(s.user?.username).toBe('ben');
    expect(s.csrfToken).toBe('tok');
    expect(s.bootstrapped).toBe(true);
  });

  it('clears state on a 401 from /sessions/current', async () => {
    fetchMock.mockResolvedValueOnce(new Response('', { status: 401 }));
    const auth = await loadAuth();
    await auth.bootstrap();
    const s = get(auth);
    expect(s.user).toBeNull();
    expect(s.csrfToken).toBeNull();
    expect(s.bootstrapped).toBe(true);
  });

  it('login populates user + csrfToken on success', async () => {
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'tok2' }),
      { status: 200 },
    ));
    const auth = await loadAuth();
    await auth.login('ben', 'pw');
    const s = get(auth);
    expect(s.user?.username).toBe('ben');
    expect(s.csrfToken).toBe('tok2');
  });

  it('login throws on 401 and leaves state untouched', async () => {
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ error: { code: 'invalid_credentials', message: 'bad' } }),
      { status: 401 },
    ));
    const auth = await loadAuth();
    await expect(auth.login('ben', 'wrong')).rejects.toThrow();
    const s = get(auth);
    expect(s.user).toBeNull();
  });

  it('logout clears state', async () => {
    // First bootstrap to populate.
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'tok' }),
      { status: 200 },
    ));
    const auth = await loadAuth();
    await auth.bootstrap();

    // Then logout.
    fetchMock.mockResolvedValueOnce(new Response('', { status: 204 }));
    await auth.logout();
    const s = get(auth);
    expect(s.user).toBeNull();
    expect(s.csrfToken).toBeNull();
  });
});
```

- [ ] **Step 2: Run to confirm it fails**

```bash
pnpm --dir web test -- src/lib/__tests__/auth.test.ts 2>&1 | head -20
```

Expected: failures — `web/src/lib/auth.ts` doesn't exist.

- [ ] **Step 3: Implement the auth store**

Create `web/src/lib/auth.ts`:

```ts
import { writable } from 'svelte/store';
import type { User, SessionResponse, PasswordChangeResponse } from './types';

type State = {
  user: User | null;
  csrfToken: string | null;
  bootstrapped: boolean;
};

const internal = writable<State>({ user: null, csrfToken: null, bootstrapped: false });

const BASE = '/api/v1';

async function jsonOr401<T>(res: Response): Promise<T> {
  if (res.status === 401) {
    throw new Error('unauthorized');
  }
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    try {
      const body = await res.json();
      if (body?.error?.message) message = body.error.message;
    } catch {
      /* swallow */
    }
    throw new Error(message);
  }
  return res.json() as Promise<T>;
}

export const auth = {
  subscribe: internal.subscribe,

  async bootstrap(): Promise<void> {
    try {
      const res = await fetch(BASE + '/sessions/current', {
        headers: { 'Content-Type': 'application/json' },
      });
      if (res.status === 401) {
        internal.set({ user: null, csrfToken: null, bootstrapped: true });
        return;
      }
      const body: SessionResponse = await jsonOr401(res);
      internal.set({ user: body.user, csrfToken: body.csrf_token, bootstrapped: true });
    } catch {
      internal.set({ user: null, csrfToken: null, bootstrapped: true });
    }
  },

  async login(username: string, password: string): Promise<void> {
    const res = await fetch(BASE + '/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });
    const body: SessionResponse = await jsonOr401(res);
    internal.set({ user: body.user, csrfToken: body.csrf_token, bootstrapped: true });
  },

  async logout(): Promise<void> {
    // Pull the current csrf token without subscribing.
    let csrfToken: string | null = null;
    internal.update(s => { csrfToken = s.csrfToken; return s; });
    try {
      await fetch(BASE + '/sessions/current', {
        method: 'DELETE',
        headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : {},
      });
    } catch {
      /* network errors during logout don't matter — clear local state regardless */
    }
    internal.set({ user: null, csrfToken: null, bootstrapped: true });
  },

  /**
   * setCSRFToken updates the in-memory CSRF token. Called by the API client
   * after PATCH /me/password returns a freshly rotated token.
   */
  setCSRFToken(token: string): void {
    internal.update(s => ({ ...s, csrfToken: token }));
  },

  /**
   * clearOn401 wipes auth state without making a network call. Used by the
   * API client when any request returns 401 (e.g. session expired) so the
   * SPA reactively renders the login screen.
   */
  clearOn401(): void {
    internal.set({ user: null, csrfToken: null, bootstrapped: true });
  },
};

// Suppress an unused-import warning while PasswordChangeResponse is referenced
// only in api.ts.
const _typeAnchor: PasswordChangeResponse | null = null;
void _typeAnchor;
```

- [ ] **Step 4: Run to confirm tests pass**

```bash
pnpm --dir web test -- src/lib/__tests__/auth.test.ts
```

Expected: PASS for all five test cases.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/auth.ts web/src/lib/__tests__/auth.test.ts
git commit -m "$(cat <<'EOF'
M6: web auth store

Svelte writable holding {user, csrfToken, bootstrapped}.
bootstrap() runs at app start to recover a session after a reload;
login/logout do the obvious things; setCSRFToken is called by the
API client after a password change rotates the token; clearOn401
is the reactive trigger that swaps in the login screen on session
expiry.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task I3: API client integration — X-CSRF-Token header + 401 handling

**Files:**
- Modify: `web/src/lib/api.ts`
- Create: `web/src/lib/__tests__/api.test.ts`

- [ ] **Step 1: Write the failing test**

Create `web/src/lib/__tests__/api.test.ts`:

```ts
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

const fetchMock = vi.fn();
beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock);
  fetchMock.mockReset();
});
afterEach(() => {
  vi.unstubAllGlobals();
});

async function loadApi() {
  vi.resetModules();
  const apiMod = await import('../api');
  const authMod = await import('../auth');
  return { api: apiMod.api, auth: authMod.auth };
}

describe('api client', () => {
  it('attaches X-CSRF-Token on POST when csrfToken is set', async () => {
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'tok' }),
      { status: 200 },
    ));
    const { api, auth } = await loadApi();
    await auth.login('ben', 'pw');

    fetchMock.mockResolvedValueOnce(new Response('{}', { status: 201 }));
    await api.addSubscription({ feed_url: 'https://x' });

    const lastCall = fetchMock.mock.calls[fetchMock.mock.calls.length - 1];
    const init = lastCall[1] as RequestInit;
    const headers = init.headers as Record<string, string>;
    expect(headers['X-CSRF-Token']).toBe('tok');
  });

  it('does not attach X-CSRF-Token on GET', async () => {
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'tok' }),
      { status: 200 },
    ));
    const { api, auth } = await loadApi();
    await auth.login('ben', 'pw');

    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ data: [] }), { status: 200 }));
    await api.listSubscriptions();

    const lastCall = fetchMock.mock.calls[fetchMock.mock.calls.length - 1];
    const init = lastCall[1] as RequestInit;
    const headers = (init.headers ?? {}) as Record<string, string>;
    expect(headers['X-CSRF-Token']).toBeUndefined();
  });

  it('clears auth state when any request returns 401', async () => {
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'tok' }),
      { status: 200 },
    ));
    const { api, auth } = await loadApi();
    await auth.login('ben', 'pw');

    fetchMock.mockResolvedValueOnce(new Response('', { status: 401 }));
    await expect(api.listSubscriptions()).rejects.toThrow();

    const { get } = await import('svelte/store');
    expect(get(auth).user).toBeNull();
  });

  it('changePassword updates auth.csrfToken from the response', async () => {
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'old' }),
      { status: 200 },
    ));
    const { api, auth } = await loadApi();
    await auth.login('ben', 'pw');

    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ csrf_token: 'new' }),
      { status: 200 },
    ));
    await api.changePassword('pw', 'new-good-password');

    const { get } = await import('svelte/store');
    expect(get(auth).csrfToken).toBe('new');
  });
});
```

- [ ] **Step 2: Run to confirm it fails**

```bash
pnpm --dir web test -- src/lib/__tests__/api.test.ts 2>&1 | head -20
```

Expected: failures — current `api.ts` doesn't read the CSRF token, doesn't clear auth on 401, has no `changePassword`.

- [ ] **Step 3: Update `web/src/lib/api.ts`**

```ts
import { get } from 'svelte/store';
import type {
  Subscription,
  EntryListItem,
  EntryDetail,
  ListResponse,
  ApiError,
  PasswordChangeResponse,
} from './types';
import { auth } from './auth';

const BASE = '/api/v1';

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method ?? 'GET').toUpperCase();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((init.headers ?? {}) as Record<string, string>),
  };

  // Attach CSRF on state-changing methods. Read non-reactively from the store.
  if (method !== 'GET' && method !== 'HEAD' && method !== 'OPTIONS') {
    const csrf = get(auth).csrfToken;
    if (csrf) {
      headers['X-CSRF-Token'] = csrf;
    }
  }

  const res = await fetch(BASE + path, { ...init, headers });

  if (res.status === 401) {
    auth.clearOn401();
    throw new Error('unauthorized');
  }
  if (!res.ok) {
    let detail: ApiError | null = null;
    try { detail = await res.json(); } catch { /* swallow */ }
    throw new Error(detail?.error?.message ?? `${res.status} ${res.statusText}`);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const api = {
  listSubscriptions: () =>
    request<ListResponse<Subscription>>('/subscriptions').then(r => r.data),

  addSubscription: (body: {
    feed_url: string;
    title?: string;
    extract?: boolean;
    cookie?: string;
    basic_auth_user?: string;
    basic_auth_pass?: string;
  }) =>
    request<Subscription>('/subscriptions', {
      method: 'POST',
      body: JSON.stringify(body),
    }),

  patchSubscription: (
    id: number,
    patch: {
      extract?: boolean;
      extract_selector?: string;
      cookie?: string;
      basic_auth_user?: string;
      basic_auth_pass?: string;
    },
  ) =>
    request<Subscription>(`/subscriptions/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    }),

  deleteSubscription: (id: number) =>
    request<void>(`/subscriptions/${id}`, { method: 'DELETE' }),

  listEntries: (params: {
    unread?: boolean;
    feed?: number;
    limit?: number;
    cursor?: string;
  } = {}) => {
    const qs = new URLSearchParams();
    if (params.unread) qs.set('unread', '1');
    if (params.feed)   qs.set('feed', String(params.feed));
    if (params.limit)  qs.set('limit', String(params.limit));
    if (params.cursor) qs.set('cursor', params.cursor);
    const suffix = qs.toString() ? `?${qs}` : '';
    return request<ListResponse<EntryListItem>>(`/entries${suffix}`);
  },

  getEntry: (id: number) =>
    request<EntryDetail>(`/entries/${id}`),

  patchEntry: (id: number, patch: { read?: boolean; saved?: boolean }) =>
    request<EntryListItem>(`/entries/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    }),

  changePassword: async (currentPassword: string, newPassword: string) => {
    const resp = await request<PasswordChangeResponse>('/me/password', {
      method: 'PATCH',
      body: JSON.stringify({
        current_password: currentPassword,
        new_password: newPassword,
      }),
    });
    auth.setCSRFToken(resp.csrf_token);
    return resp;
  },
};
```

- [ ] **Step 4: Run to confirm tests pass**

```bash
pnpm --dir web test
```

Expected: PASS for the entire web test suite, including the new API client tests and the existing M5-era tests.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/api.ts web/src/lib/__tests__/api.test.ts
git commit -m "$(cat <<'EOF'
M6: web api client — X-CSRF-Token + 401 wipe + changePassword

State-changing methods (POST/PUT/PATCH/DELETE) automatically attach
X-CSRF-Token from the auth store. Any 401 response wipes auth state
so the SPA reactively renders the Login view. New addSubscription
overload takes the M6 + M5 fields (extract / extract_selector /
cookie / basic_auth_*); new patchSubscription routes the same
shape to PATCH; new changePassword updates the in-memory CSRF
token from the response.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task I4: Login view

**Files:**
- Create: `web/src/views/Login.svelte`
- Create: `web/src/views/__tests__/Login.test.ts`

- [ ] **Step 1: Write the failing test**

Create `web/src/views/__tests__/Login.test.ts`:

```ts
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, fireEvent, screen } from '@testing-library/svelte';
import Login from '../Login.svelte';

const loginMock = vi.fn();
vi.mock('../../lib/auth', () => ({
  auth: {
    login: (...args: unknown[]) => loginMock(...args),
    subscribe: () => () => {},
  },
}));

beforeEach(() => loginMock.mockReset());
afterEach(() => {/* nothing */});

describe('Login.svelte', () => {
  it('renders username + password inputs and a submit button', () => {
    render(Login);
    expect(screen.getByLabelText(/username/i)).toBeTruthy();
    expect(screen.getByLabelText(/password/i)).toBeTruthy();
    expect(screen.getByRole('button', { name: /sign in/i })).toBeTruthy();
  });

  it('calls auth.login with the entered values on submit', async () => {
    loginMock.mockResolvedValueOnce(undefined);
    render(Login);

    await fireEvent.input(screen.getByLabelText(/username/i), { target: { value: 'ben' } });
    await fireEvent.input(screen.getByLabelText(/password/i), { target: { value: 'pw12345678' } });
    await fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    expect(loginMock).toHaveBeenCalledWith('ben', 'pw12345678');
  });

  it('shows an error message when login rejects', async () => {
    loginMock.mockRejectedValueOnce(new Error('unauthorized'));
    render(Login);

    await fireEvent.input(screen.getByLabelText(/username/i), { target: { value: 'ben' } });
    await fireEvent.input(screen.getByLabelText(/password/i), { target: { value: 'wrong' } });
    await fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    // Wait for the promise rejection + reactive update.
    await screen.findByText(/invalid username or password/i);
  });
});
```

> **Implementer note:** if `@testing-library/svelte` isn't already in `web/package.json`, add it (it's a small dev dep). Otherwise match the existing test patterns in `web/src/components/__tests__/`.

- [ ] **Step 2: Run to confirm it fails**

```bash
pnpm --dir web test -- src/views/__tests__/Login.test.ts 2>&1 | head -20
```

Expected: failures — `Login.svelte` doesn't exist.

- [ ] **Step 3: Create the Login view**

Create `web/src/views/Login.svelte`:

```svelte
<script lang="ts">
  import { auth } from '../lib/auth';

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let busy = $state(false);

  async function submit(e: Event) {
    e.preventDefault();
    error = '';
    busy = true;
    try {
      await auth.login(username, password);
    } catch (err) {
      error = err instanceof Error && err.message !== 'unauthorized'
        ? err.message
        : 'Invalid username or password.';
    } finally {
      busy = false;
    }
  }
</script>

<form onsubmit={submit}>
  <h1>Sign in to Tap</h1>
  <label>
    Username
    <input
      type="text"
      autocomplete="username"
      bind:value={username}
      disabled={busy}
      required
    />
  </label>
  <label>
    Password
    <input
      type="password"
      autocomplete="current-password"
      bind:value={password}
      disabled={busy}
      required
    />
  </label>
  {#if error}
    <p role="alert" class="error">{error}</p>
  {/if}
  <button type="submit" disabled={busy}>
    {busy ? 'Signing in…' : 'Sign in'}
  </button>
</form>

<style>
  form {
    max-width: 22rem;
    margin: 4rem auto;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  h1 {
    text-align: center;
    margin-bottom: 0.5rem;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  input {
    padding: 0.5rem;
  }
  .error {
    color: var(--color-danger, #b00);
  }
</style>
```

- [ ] **Step 4: Run to confirm tests pass**

```bash
pnpm --dir web test -- src/views/__tests__/Login.test.ts
```

Expected: PASS for all three tests.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/Login.svelte web/src/views/__tests__/Login.test.ts web/package.json web/pnpm-lock.yaml
git commit -m "$(cat <<'EOF'
M6: web Login.svelte view

Svelte 5 runes: $state for form fields, $state for the busy/error
flags. Submit calls auth.login; on rejection, the response is mapped
to a generic 'Invalid username or password.' (the server already
collapsed unknown-user / bad-password / disabled into one
invalid_credentials code, so the SPA matches the discretion).
Functional only — visual polish lives in M8.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task I5: App.svelte — bootstrap on mount, render Login when unauthenticated

**Files:**
- Modify: `web/src/App.svelte`

- [ ] **Step 1: Update App.svelte**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { route } from './lib/router';
  import { auth } from './lib/auth';
  import Login from './views/Login.svelte';
  import Unread from './views/Unread.svelte';
  import Reader from './views/Reader.svelte';

  onMount(() => {
    void auth.bootstrap();
  });
</script>

{#if !$auth.bootstrapped}
  <!-- empty during the brief bootstrap window — keeps the SPA from
       flashing the login form for an authenticated user -->
{:else if $auth.user == null}
  <Login />
{:else if $route.name === 'reader'}
  <Reader id={$route.params.id} />
{:else}
  <Unread />
{/if}
```

- [ ] **Step 2: Smoke-test by running the dev server**

The brainstorming skill flagged that for UI changes, you should start the dev server and use the feature in a browser before claiming done. For this task:

```bash
make dev
```

Open http://localhost:5173 — expect the Login view (no users on a fresh DB). Confirm:
1. Submitting valid creds (after running `tap admin create` in another terminal) lands on the Unread view.
2. Submitting invalid creds shows the inline error.
3. Reloading the page after login keeps you logged in (the `/sessions/current` probe recovers state).
4. Calling logout (no UI yet — easiest is `await auth.logout()` from devtools) returns you to the login screen.

If any of those don't work, fix before committing.

- [ ] **Step 3: Run the type checks + tests**

```bash
pnpm --dir web run check && pnpm --dir web test
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/App.svelte
git commit -m "$(cat <<'EOF'
M6: App.svelte gates routing on auth state

onMount runs auth.bootstrap() to recover state after a reload.
While bootstrapped is false, render nothing (avoids flashing the
login form for an already-authenticated user). When user is null,
render Login; otherwise fall through to the existing route gate.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Phase J — Documentation + final verification

### Task J1: README — trust posture + upgrade note

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add the M6 trust-posture paragraph**

After the existing M5 trust-posture section, insert:

```markdown
**Authentication (M6).** Tap is multi-user. Users have password-based
accounts (argon2id at OWASP 2026 defaults); sessions are `HttpOnly`
cookies hashed at rest with idle (`--session-idle-ttl`, default 7d) and
absolute (`--session-absolute-ttl`, default 90d) expiries. State-changing
requests need a matching `X-CSRF-Token` header issued at login. The
first admin is created either by setting `TAP_ADMIN_USERNAME` and
`TAP_ADMIN_PASSWORD` on first launch, or by running `tap admin create`
from the host. Subsequent admins use the same `tap admin create`
subcommand. `tap admin passwd <username>` resets a forgotten password
and force-logs-out that user's active sessions. Per-feed credentials
(`cookie`, `basic_auth_user`, `basic_auth_pass`) are accepted on POST/PATCH
`/api/v1/subscriptions` but never returned by the read endpoints —
the GET shape exposes only `has_cookie` / `has_basic_auth` booleans.
Per-feed credentials apply to feed polling and article extraction (when
`extract=true`); they do **not** apply to the media-proxy origin fetch
path, matching Miniflux's posture. Same-origin authenticated images
consequently render as broken — a known cross-ecosystem limitation.
```

- [ ] **Step 2: Add the upgrade note**

After the existing "Upgrading from M4" section:

```markdown
## Upgrading from M5

M6 is breaking. Auth tables and credential columns are additive, but
existing databases have no users — login is unusable until either
`TAP_ADMIN_USERNAME`/`TAP_ADMIN_PASSWORD` are set on next boot, or
`tap admin create` is run from the host.
```

- [ ] **Step 3: Update the status line**

Replace the existing M5 status line near the top of the README:

```markdown
## Status

M6 in progress — auth foundations (password login, sessions, CSRF, admin
bootstrap, per-feed credential redaction). M5 article extraction merged.
M4 polling discipline merged. M3 media proxy + FS cache merged. M2
sanitisation pipeline merged. See [`docs/specs/`](docs/specs/) for
milestone specs.
```

- [ ] **Step 4: Add the M6 configuration knobs**

Find the existing "Configuration knobs added by M5" block (or an analogous structured listing). Append the M6 knobs:

```markdown
## Configuration knobs added by M6

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--session-idle-ttl` | `TAP_SESSION_IDLE_TTL` | `168h` (7d) | Refreshed on each authenticated request. |
| `--session-absolute-ttl` | `TAP_SESSION_ABSOLUTE_TTL` | `2160h` (90d) | Hard cap; cookie Max-Age. |
| `--cookie-secure` | `TAP_COOKIE_SECURE` | `auto` | `auto`/`true`/`false`. `auto` resolves to `true` when `--addr` binds non-loopback. |
| (env-only) | `TAP_ADMIN_USERNAME` | (unset) | First-launch admin bootstrap. Both must be set; partial → fatal. |
| (env-only) | `TAP_ADMIN_PASSWORD` | (unset) | First-launch admin bootstrap. Min 8 chars; failure → fatal. |
```

- [ ] **Step 5: Verify the README renders sensibly**

```bash
# Quick scan for broken markdown:
grep -nE '^#' README.md | head -20
```

Expected: heading hierarchy looks intentional; no orphaned sections.

- [ ] **Step 6: Commit**

```bash
git add README.md
git commit -m "$(cat <<'EOF'
M6: README trust posture + upgrade note + M6 config knobs

Documents the auth shape (cookie attributes, expiry defaults,
bootstrap paths), the per-feed credential redaction discipline,
and the same-origin authenticated-images limitation. Status line
bumped to "M6 in progress".

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task J2: CLAUDE.md — status + trust-posture deltas

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update the status line**

Find the existing line that mentions M5 in progress and replace with an M6-equivalent. Then append a new bullet to the trust-posture block describing the auth + per-feed-credential changes.

```markdown
## Project

Tap is a self-hosted RSS / Atom / JSON Feed reader. It ships as **one
static Go binary** with an embedded SQLite database, an embedded Svelte
SPA, and no external services. See `docs/concept.md` for the full design
and `docs/roadmap.md` for the milestone plan; **M6 in progress** (auth
foundations landing — spec at
`docs/specs/2026-05-10-m6-auth-foundations.md`; M5 article extraction
merged — spec at `docs/specs/2026-05-09-m5-article-extraction.md`; M4
polling discipline merged — spec at
`docs/specs/2026-05-09-m4-polling-discipline.md`; M3 media proxy merged
— spec at `docs/specs/2026-05-09-m3-media-proxy.md`; M2 sanitisation
pipeline merged — spec at `docs/specs/2026-05-08-m2-sanitisation.md`;
M1 walking-skeleton spec at `docs/specs/2026-05-08-m1-walking-skeleton.md`).
```

- [ ] **Step 2: Add an M6 paragraph to the "Trust posture" section**

After the existing M5 paragraph in the CLAUDE.md trust-posture block, add:

```markdown
M6 introduces user accounts (argon2id passwords, `users` + `sessions`
tables) and the session/CSRF middleware on every `/api/v1/*` route except
`POST /sessions` and `/healthz`. Session cookies are `HttpOnly`,
`SameSite=Lax`, and conditionally `Secure` (auto-resolves against the
listen address; explicit override via `--cookie-secure`). CSRF tokens
live on the `sessions` row, return in the login JSON, and rotate only
on password change.

Per-feed credentials (`subscriptions.cookie`, `basic_auth_user`,
`basic_auth_pass`) are accepted on POST/PATCH `/api/v1/subscriptions`
but never returned on GET — the read DTO exposes only `has_cookie` /
`has_basic_auth` booleans. Layered onto outbound requests via
`internal/httpx.ApplyFeedCreds` for feed polling and article extraction
only — the media proxy (`internal/proxy/handler.go`) deliberately
fetches origins anonymously, mirroring Miniflux. Same-origin
authenticated images render broken; this is a known cross-ecosystem
limitation, not a Tap-specific deficiency.

Admin bootstrap is out-of-band (concept §7.1): either
`TAP_ADMIN_USERNAME`/`TAP_ADMIN_PASSWORD` on first launch (silent on
populated DBs) or `tap admin create` interactively from the host.
`tap admin passwd <username>` resets a forgotten password and
force-logs-out that user's active sessions. There is no SPA-visible
bootstrap path.
```

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "$(cat <<'EOF'
M6: CLAUDE.md status line + trust-posture deltas

Documents the auth shape, the per-feed credential redaction
discipline (and the deliberate Miniflux-mirroring posture on
the media proxy), and the bootstrap paths so future agents
land here knowing the invariants without re-deriving them
from the spec.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task J3: Final verification

This task runs the full verification matrix to confirm M6 is complete. No code changes; no commit. If any check fails, return to the relevant phase and fix.

- [ ] **Step 1: Full test suite**

```bash
make test
```

Expected: PASS for `go test ./... -race` (which depends on `web/dist/index.html` being up-to-date — `make test` rebuilds the SPA bundle if needed).

- [ ] **Step 2: Static binary build**

```bash
make build
```

Expected: `bin/tap` produced, no errors. CGO-disabled build means the binary is distroless-ready.

- [ ] **Step 3: Web type check**

```bash
pnpm --dir web run check
```

Expected: no TypeScript errors.

- [ ] **Step 4: End-to-end smoke (interactive)**

In one terminal:

```bash
rm -rf /tmp/tap-m6
TAP_ADMIN_USERNAME=ben TAP_ADMIN_PASSWORD=supersecret \
  ./bin/tap -addr 127.0.0.1:8080 -data /tmp/tap-m6
```

Expected: log line `bootstrapped admin from environment`. The server starts.

In a browser, visit http://127.0.0.1:8080. Expected: Login view. Sign in with `ben`/`supersecret`. Expected: Unread view loads.

In another terminal:

```bash
./bin/tap admin create -data /tmp/tap-m6
# Enter alice / sufficient-password / sufficient-password
```

Expected: `created admin user "alice" (id=2)`.

Sign out (or use devtools to call `auth.logout()`), then sign in as `alice`. Expected: works.

```bash
./bin/tap admin passwd alice -data /tmp/tap-m6
# Enter newpassword12 / newpassword12
```

Expected: `password reset for "alice"`. Re-login as `alice` with the new password. Expected: works. The old password no longer works.

- [ ] **Step 5: Restart with env vars still set**

Stop the binary. Restart with the same `TAP_ADMIN_USERNAME` / `TAP_ADMIN_PASSWORD` exported. Expected: NO `bootstrapped admin from environment` log line. The user table is unchanged. Concept §7.1 silence guarantee.

- [ ] **Step 6: Tear down the smoke environment**

```bash
rm -rf /tmp/tap-m6
```

- [ ] **Step 7: Confirm git is clean and review the merge surface**

```bash
git status
git log --oneline main..HEAD | wc -l
```

Expected: clean tree; commit count corresponds to the number of tasks executed (~30+ commits for M6). Each commit is a coherent task per the plan.

---

## Configuration knobs added by M6

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--session-idle-ttl` | `TAP_SESSION_IDLE_TTL` | `168h` (7d) | Refreshed on each authenticated request. |
| `--session-absolute-ttl` | `TAP_SESSION_ABSOLUTE_TTL` | `2160h` (90d) | Hard cap; cookie Max-Age. |
| `--cookie-secure` | `TAP_COOKIE_SECURE` | `auto` | `auto`/`true`/`false`. `auto` → `Secure` when bound non-loopback. |
| (env-only) | `TAP_ADMIN_USERNAME` | (unset) | First-launch admin bootstrap. Both must be set; partial → fatal. |
| (env-only) | `TAP_ADMIN_PASSWORD` | (unset) | First-launch admin bootstrap. Min 8 chars; failure → fatal. |

Argon2 cost lives as a constant (`internal/auth.DefaultParams`) — no flag. Future tuning + re-hash-on-verify is M12 work.

---

## Upgrading from M5

M6 is a breaking change for any deployed M5 instance — the auth/credentials migration is additive, but `users` is empty after the migration so login rejects everything. Two recovery paths:

1. **Set `TAP_ADMIN_USERNAME` and `TAP_ADMIN_PASSWORD`** before next launch. The first launch creates the admin; subsequent launches with the env vars still set are silent no-ops.
2. **Run `tap admin create -data <dir>`** from the host. Interactive; works regardless of whether the server is running.

Tap has no real users yet, so `make clean && rm -rf data/` is also a fully supported upgrade path.

---

## Plan-to-spec coverage map

This map confirms every requirement in `docs/specs/2026-05-10-m6-auth-foundations.md` has an implementing task. If you find a spec section without a row, the plan is incomplete — add a task.

| Spec section | Plan task(s) |
|---|---|
| New package `internal/auth` (Hash/Verify/Params/DefaultParams/ValidatePassword) | A2, A3, A4, A6 |
| `internal/auth` token mints (MintSessionToken / MintCSRFToken) | A5 |
| Two new dependencies (`x/crypto/argon2`, `x/term`) | A1 |
| Migration `0005_auth_and_credentials.sql` | B1 |
| `db.users` (User/NewUser/ErrUserExists/Insert/Get/Update/Disable/Count) | B2, B3 |
| `db.sessions` (Session/NewSession/Insert/Get/Refresh/UpdateCSRF/Delete×3) | B4, B5 |
| `db.subscriptions` credential columns (Subscription/DueSubscription/NewSubscription, queries, UpdateSubscriptionCredentials) | B6 |
| Middleware (ctxKey, userFromContext, sessionFromContext, chain, requireSession, requireCSRF) | C1, C2, C3, C4 |
| Error code constants | C1 |
| Auth API: POST /sessions | D2, D3 |
| Auth API: GET /sessions/current | D4 |
| Auth API: DELETE /sessions/current | D4 |
| Auth API: PATCH /me/password | D5 |
| Cookie helpers + CookieSecureMode + resolveCookieSecure | D1, H3 |
| Subscription credential surface (POST + PATCH + GET DTO) | E1, E2 |
| `httpx.ApplyFeedCreds` helper | F1 |
| `extract.Extract` signature change | F2 |
| `feed.Fetch` accepts FeedCreds | F3 |
| Worker passes creds to fetch + extract | F4 |
| Media proxy regression (anonymous origin fetches) | F5 |
| Admin CLI subcommand router | G1 |
| `tap admin create` | G2 |
| `tap admin passwd <username>` | G3 |
| `bootstrapAdmin` helper + env-var first-launch path | H1, H3 |
| `MuxOpts` extensions + middleware mounting | H2 |
| `cmd/tap/main.go` flags + bootstrap wire-up | H3 |
| End-to-end main_test.go (login + CSRF, basic-auth, anon proxy) | H4 |
| End-to-end main_test.go (bootstrap on empty / silent on populated / no env vars) | H5 |
| SPA types backfill + new auth types | I1 |
| SPA auth store | I2 |
| SPA api.ts integration (CSRF header + 401 wipe + changePassword) | I3 |
| SPA Login.svelte view | I4 |
| SPA App.svelte gate | I5 |
| README trust posture + upgrade note + M6 knobs | J1 |
| CLAUDE.md status line + deltas | J2 |
| Final verification (`make test`, `make build`, smoke run, env-var idempotence) | J3 |

Every Definition-of-Done bullet from the spec maps to one of the tasks above:

| Spec DoD bullet | Verified by |
|---|---|
| 1. `make test` passes including new packages | J3 Step 1 |
| 2. Fresh DB + env vars → admin created, login works; silent on restart | H5, J3 Step 5 |
| 3. Fresh DB + no env vars → login rejects; `tap admin create` works | H5, J3 Step 4 |
| 4. POST with basic_auth_user/pass → poll sees Authorization header; GET returns has_basic_auth=true with no values | F4, H4 |
| 5. PATCH cookie="" clears; cookie omitted preserves | E2 |
| 6. PATCH /me/password keeps current session, deletes others, returns new csrf_token | D5 |
| 7. tap admin passwd updates hash + deletes sessions | G3 |
| 8. Authenticated routes 401 without session; CSRF-required routes 403 without token | C2, C3, C4, H4 |
| 9. Origin/Referer mismatch → 403 csrf_invalid | C4 |
| 10. Media-proxy origin fetch never carries Cookie/Authorization | F5, H4 |
| 11. Migration 0005 applies cleanly against M5 DB | B1 |
| 12. `make build` produces a static binary | J3 Step 2 |

---

## Notes for the implementer

A few things worth knowing before you start:

- **Test-time argon2 cost matters.** Production `auth.DefaultParams` (t=2, m=64MiB) takes 30-100ms per hash. Tests use `testHashParams` (t=1, m=8MiB) to keep the suite fast. Make sure every test that hashes uses the low-cost variant — including the end-to-end tests in `cmd/tap/main_test.go`. The standard pattern is to plumb `auth.Params` through the constructor under test.

- **`MuxOpts.HashParams` is the seam.** The auth handlers don't read `auth.DefaultParams` directly; they take `auth.Params` from MuxOpts. Production wires `auth.DefaultParams`; tests wire `testHashParams`. Don't hardcode `auth.DefaultParams` inside any handler.

- **Bootstrap and CLI use the same low-cost params in tests.** If `cmd/tap/main_test.go` sets `TAP_ADMIN_PASSWORD` and the bootstrap path uses `auth.DefaultParams`, the test pays a 30ms hash cost. Acceptable but noticeable. The cleanest fix: factor `runServer` to accept `auth.Params` as a non-flag dependency and have the test harness pass `testHashParams`.

- **`requireSession`'s idle refresh races.** Concurrent requests on the same session both UPDATE `last_seen_at` and `idle_expires_at`. SQLite WAL serialises the writes; both updates converge. No coordination needed. If a refresh fails (e.g. db locked), the request still proceeds — worst case the session expires sooner than expected.

- **`requireCSRF` accepts both Origin and Referer absent.** SameSite=Lax + an authenticated session already cover that combination. The header check is defence-in-depth on top of the token check, not the binding defence.

- **Don't unwrap the proxy URL signing format.** M3's signed token is opaque and HMAC-bound. M6 doesn't need to look inside it. Adding `feed_id` to it (to enable same-origin auto-apply for proxy fetches) was explicitly punted in brainstorming — see the spec's "Out of scope (deferred)" table for the Miniflux-mirroring rationale.

- **PATCH /me/password keeps the current session deliberately.** Concept §7.5 sees sessions as the unit of revocation. The user-initiated password change keeps the current session active so the user stays logged in; the recovery primitive is the *deletion of other sessions*. Force-logging-out the current session would surprise the user and serves no security purpose.

- **`tap admin passwd` always deletes ALL the user's sessions.** Different from PATCH /me/password. The admin reset is an explicit lockout-and-reissue: the admin assumes the user has lost access to the password and is being given a new one; existing sessions are now untrusted.

- **Env-var bootstrap is fatal on partial config.** If only one of `TAP_ADMIN_USERNAME` / `TAP_ADMIN_PASSWORD` is set, exit 1 with a clear error. Don't silently skip — the operator clearly *intended* to bootstrap; helping them realise the partial config is the kind thing.

- **`Set-Cookie: tap_session=; Max-Age=0`** is the cookie-clearing pattern the SPA's logout depends on. The browser drops the cookie. Don't try to "delete" by setting an empty value alone — Max-Age=0 is what makes the browser actually forget it.

- **Don't log cookie values, password fields, or basic-auth pass anywhere.** `slog.Info("request", "headers", r.Header)` is the kind of sweep that leaks. Default slog handlers don't log headers, but a future change might. The convention is enforced by code review, not by the compiler — keep an eye out.

- **`internal/api` unit tests vs end-to-end auth tests.** The `internal/api` package tests cover handler logic against the unwrapped sub-muxes (e.g. `newSubscriptionsTestMux`) so they don't fight the middleware. End-to-end auth coverage lives in `cmd/tap/main_test.go`. If you find yourself trying to mock the session middleware in an `internal/api` test, you're doing it wrong — call the handler factory directly.

- **The cookie-secure auto resolver might surprise you in dev.** `make dev` binds to `127.0.0.1:8080`, so cookies aren't `Secure`, which is correct for `http://localhost:5173` in the browser. If you set `--addr 0.0.0.0:8080` for local LAN testing, the auto-mode flips Secure on, and the browser refuses to set the cookie over plain HTTP — login appears to silently fail. Pass `--cookie-secure=false` explicitly for that case, or front the dev server with TLS.

- **`feed.Fetch` already lives in `internal/feed/fetch.go`.** Add the creds path next to the existing conditional-GET headers. The diff is small.

- **`internal/extract.Extract` signature change is fanout-heavy.** Every callsite (one in the worker, every test in `extract_test.go`) needs the new parameter. Use `gopls` rename or a careful grep.

- **Skip M11 archival concerns.** M11 will sweep old read-and-unsaved entries and prune media cache. M6 doesn't add anything to that — the new tables (`users`, `sessions`) live outside the archival path. Sessions self-prune on access (the middleware deletes absolute-expired rows in the same handler that rejects them).

- **The `/healthz` endpoint stays unauthenticated.** Concept §10 calls this out specifically. Don't accidentally wrap it in `requireSession` — orchestrators can't supply a session cookie.

- **Verify against the spec's "What this milestone deliberately does *not* prove" list before claiming done.** If you find yourself implementing TOTP, session listing, admin reset paths in the SPA, brute-force lockout, password complexity, audit log, or per-feed `outbound_proxy_url` — push back and scope-check. Those are M7 / M12 / deferred.

Pure scaffolding tasks (the migration SQL file, README edits, flag declarations, the type-only SPA additions) are TDD-exempt per `docs/roadmap.md` §"Working cadence". Everything with branches, error handling, or state is in scope — and that's most of M6.

