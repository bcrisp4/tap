# M7 — 2FA + Passkeys + Per-User Data Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land TOTP enrolment + recovery codes, WebAuthn passkey registration and login, session listing + per-session revocation, admin SPA paths (create user, reset password, disable 2FA), and per-user data isolation (`user_id NOT NULL` on subscriptions + entries, strict per-user query filters) as specified in `docs/specs/2026-05-10-m7-2fa-passkeys-isolation.md`. After M7, every subscription and entry is owned by the user who created it; even admins see only their own feeds; TOTP-enrolled users are prompted for a second factor at login; passkey-equipped users can log in without a password; session listing with device/IP metadata and per-session revocation are live in Settings → Security.

**Architecture:** Two schema migrations: `0006_user_data_isolation.sql` adds `user_id NOT NULL` to `subscriptions` and `entries`; `0007_2fa_passkeys_sessions_meta.sql` recreates the `sessions` table (making `user_id` nullable for anonymous WebAuthn challenge sessions) and adds `user_agent`, `address`, `webauthn_challenge` columns, plus creates `pending_logins`, `totp_secrets`, `recovery_codes`, and `passkeys` tables. TOTP is implemented with stdlib `crypto/hmac`+`crypto/sha1` (RFC 6238) — no external TOTP lib. Secrets are AES-256-GCM encrypted at rest using a server key generated into the existing `configuration` table. Recovery codes use `auth.Hash(DefaultParams)` (same argon2id as passwords). WebAuthn uses `github.com/go-webauthn/webauthn` (pure-Go, Apache-2.0). Login flow extends `POST /api/v1/sessions` with a TOTP second step via short-lived `pending_logins` rows; passkey login uses separate public `POST /api/v1/passkey-sessions/begin|finish` endpoints. All subscription/entry db functions gain a `userID int64` parameter. Admin endpoints (`GET/POST/PATCH/DELETE /api/v1/admin/users`) require a new `requireAdmin` middleware. SPA gains Settings → Security view, Admin view, and login-screen extensions — all functional but unstyled (M8 polishes).

**Tech Stack:** Go 1.25, modernc.org/sqlite, stretchr/testify, golang.org/x/crypto/argon2 (existing), `github.com/go-webauthn/webauthn` (new — pure-Go, no CGO), Svelte 5 + TypeScript + Vite. New Go deps: `github.com/go-webauthn/webauthn`.

---

## Skills and tools to apply

Always-on for every code-touching task:

- **`superpowers:test-driven-development`** — red/green/refactor on every behaviour-bearing change. Mandated by `docs/roadmap.md` §"Working cadence". Pure scaffolding (migration SQL, README edits, flag declarations, type-only SPA additions) is exempt; everything with branches, error handling, or state is in scope.
- **`superpowers:verification-before-completion`** — before marking a task done, run the listed `make test` or `go test` command and confirm the output matches the expected output.

Reach for as needed:

- **`golang-security`** — AES-256-GCM for TOTP secret encryption (`crypto/aes`, `crypto/rand` for nonce, `cipher.NewGCM`); `crypto/subtle.ConstantTimeCompare` for recovery-code comparison; HMAC-SHA1 for TOTP (RFC 6238); argon2id for recovery-code hashing (already landed in `internal/auth/argon2.go`). Never log TOTP secrets, recovery codes, or pending tokens.
- **`golang-database`** — parameterised queries throughout; `errors.Is(err, sql.ErrNoRows)` mapping; `strings.Contains(err.Error(), "UNIQUE constraint failed")` pattern for duplicate detection (matches existing `ErrUserExists` pattern in `internal/db/users.go`); transactions for multi-table writes (e.g. confirm TOTP + insert recovery codes atomically).
- **`golang-error-handling`** — sentinel errors (`ErrTOTPNotEnrolled`, `ErrPasskeyNotFound`) defined in their respective `db/` files; `fmt.Errorf("context: %w", err)` wrapping; single-handling rule (log OR return, never both).
- **`golang-testing`** + **`golang-stretchr-testify`** — match existing repo style (`require.NoError(t, err)`, `require.Equal(t, want, got)`). Table-driven for multi-case tests. Use `httptest.NewServer` for HTTP handler tests; in-memory SQLite (`":memory:"`) for db tests; inject `testHashParams` for argon2 cost in any test that calls `Hash`.
- **`golang-context`** — `userFromContext`/`sessionFromContext` already in `internal/api/middleware.go`; new `requireAdmin` uses the same pattern.
- **`golang-naming`** — exported types: `TOTPSecret`, `RecoveryCode`, `Passkey`, `PendingLogin`; unexported helpers: `encryptTOTPSecret`, `decryptTOTPSecret`; error vars: `ErrTOTPNotEnrolled`, `ErrTOTPAlreadyEnrolled`, `ErrPasskeyNotFound`, `ErrCannotDeleteSelf`.
- **`golang-modernize`** — Go 1.25 idioms throughout; `crypto/rand.N` for random integer generation where needed.
- **`svelte-runes`** — Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`) in all new SPA views. Match existing components for established patterns.
- **`svelte-components`** — form and modal patterns in `Security.svelte` and `Admin.svelte`.

MCP tools:

- **`context7` (`mcp__plugin_context7_context7__query-docs`)** — query `/go-webauthn/webauthn` for current `BeginRegistration`, `FinishRegistration`, `BeginDiscoverableLogin`, `FinishDiscoverableLogin` signatures before implementing passkey endpoints. The library evolves; verify the API before writing the handlers.

---

## File structure

| Path | Action | Responsibility |
|---|---|---|
| `internal/db/migrations/0006_user_data_isolation.sql` | **create** | Recreate `subscriptions` with `user_id NOT NULL` + widen `UNIQUE(feed_url)` → `UNIQUE(user_id, feed_url)`. Add `user_id NOT NULL` to `entries`. |
| `internal/db/migrations/0007_2fa_passkeys_sessions_meta.sql` | **create** | `sessions` table recreation (nullable `user_id`, new cols); `pending_logins`, `totp_secrets`, `recovery_codes`, `passkeys` tables. |
| `internal/db/subscriptions.go` | modify | All query functions gain `userID int64` param + `WHERE user_id = ?` filter. `NewSubscription` gains `UserID`. |
| `internal/db/subscriptions_test.go` | modify | Per-user isolation: userA's subs invisible to userB. |
| `internal/db/entries.go` | modify | Same `userID` filter treatment as subscriptions. **`UpdateAfterPoll`/`PollResult` must also include `UserID`** — entries.user_id is NOT NULL after migration 0006. |
| `internal/db/entries_test.go` | modify | Per-user isolation enforced on list + get; UpdateAfterPoll with UserID. |
| `internal/poll/worker.go` | modify | Pass `sub.UserID` into `PollResult.UserID` when constructing the poll result. |
| `internal/db/sessions.go` | modify | `Session`/`NewSession` gain `UserAgent`, `Address`, `WebAuthnChallenge` fields. New: `ListSessionsByUserID`, `SetWebAuthnChallenge`, `ClearWebAuthnChallenge`. |
| `internal/db/sessions_test.go` | modify | Cover new fields and new functions. |
| `internal/db/users.go` | modify | New: `ListUsers`, `DeleteUser`, `EnableUser`, `GetUserTOTPStatus`, `GetUserPasskeyCount`. |
| `internal/db/users_test.go` | modify | Cover each new function. |
| `internal/db/totp.go` | **create** | `TOTPSecret`, `RecoveryCode` types; `InsertTOTPSecret`, `GetTOTPSecret`, `ConfirmTOTPSecret`, `DeleteTOTPSecret`, `InsertRecoveryCodes`, `GetUnconsumedRecoveryCodes`, `ConsumeRecoveryCode`, `DeleteRecoveryCodes`. |
| `internal/db/totp_test.go` | **create** | Full roundtrip coverage of each function. |
| `internal/db/passkeys.go` | **create** | `Passkey` type; `InsertPasskey`, `GetPasskeysByUserID`, `GetPasskeyByCredentialID`, `UpdatePasskeySignCounter`, `DeletePasskey`. |
| `internal/db/passkeys_test.go` | **create** | Full roundtrip coverage. |
| `internal/db/pending_logins.go` | **create** | `PendingLogin` type; `InsertPendingLogin`, `GetPendingLoginByTokenHash`, `DeletePendingLogin`, `DeleteExpiredPendingLogins`. |
| `internal/db/pending_logins_test.go` | **create** | Roundtrip; expired cleanup. |
| `internal/auth/totp.go` | **create** | `GenerateTOTPSecret`, `TOTPSecretURI`, `VerifyTOTP`, `GenerateRecoveryCodes`. HMAC-SHA1 RFC 6238. ±1 window. |
| `internal/auth/totp_test.go` | **create** | GenerateTOTPSecret distinctness; VerifyTOTP valid/stale/window-edge; TOTPSecretURI shape; GenerateRecoveryCodes count/charset/distinctness. |
| `internal/auth/encrypt.go` | **create** | `EncryptTOTPSecret`, `DecryptTOTPSecret`. AES-256-GCM; nonce prepended to ciphertext. |
| `internal/auth/encrypt_test.go` | **create** | Roundtrip; wrong-key error; nonce randomness (two encryptions differ). |
| `internal/auth/pending.go` | **create** | `MintPendingToken`. Same shape as `MintSessionToken`. |
| `internal/auth/pending_test.go` | **create** | Shape and hash-relationship tests. |
| `internal/api/middleware.go` | modify | New `requireAdmin` middleware (reads role from context user, 403 `admin_required` if not admin). Session capture stores `UserAgent` + `Address` on login. |
| `internal/api/middleware_test.go` | modify | `requireAdmin`: admin passes, non-admin 403. Session captures UA + addr. |
| `internal/api/errors.go` | modify | Add 8 new error code constants. |
| `internal/api/auth.go` | modify | Extend `loginHandler` for TOTP second step (checks confirmed TOTP, mints pending token). New: `listSessionsHandler`, `revokeSessionHandler`, `revokeAllOtherSessionsHandler`. Extend `getSessionCurrentHandler` to return `has_totp`/`passkey_count`. |
| `internal/api/auth_test.go` | modify | TOTP second-step flows; session list; revoke; revoke-all. |
| `internal/api/totp.go` | **create** | `beginTOTPEnrolmentHandler`, `confirmTOTPEnrolmentHandler`, `deleteTOTPHandler`, `regenerateRecoveryCodesHandler`. Loads/stores server encryption key from `configuration` table. |
| `internal/api/totp_test.go` | **create** | Full coverage of TOTP endpoints per spec. |
| `internal/api/passkeys.go` | **create** | `beginPasskeyRegistrationHandler`, `finishPasskeyRegistrationHandler`, `listPasskeysHandler`, `deletePasskeyHandler`, `beginPasskeyLoginHandler`, `finishPasskeyLoginHandler`. Wraps `go-webauthn/webauthn`. |
| `internal/api/passkeys_test.go` | **create** | Registration begin/finish; list (no credential_id/public_key); delete; login begin/finish with mock asserter. |
| `internal/api/admin.go` | **create** | `listUsersHandler`, `createUserHandler`, `patchUserHandler`, `resetUserPasswordHandler`, `disableUserTOTPHandler`, `deleteUserHandler`. |
| `internal/api/admin_test.go` | **create** | Each endpoint: non-admin 403; happy path; edge cases per spec. |
| `internal/api/api.go` | modify | `MuxOpts` gains `WebAuthnInstance *webauthn.WebAuthn`. Wire TOTP, passkey, session-list, admin routes with correct middleware. |
| `internal/api/subscriptions.go` | modify | All handlers pass `userFromContext` user ID into db calls. |
| `internal/api/subscriptions_test.go` | modify | Cross-user isolation: userB cannot read/delete userA's subscription. |
| `internal/api/entries.go` | modify | All handlers pass user ID into db calls. |
| `internal/api/entries_test.go` | modify | Cross-user isolation on entries. |
| `cmd/tap/admin.go` | modify | New `runAdminDisableTOTP` subcommand. |
| `cmd/tap/admin_test.go` | modify | `disable-totp` happy path; missing user exit 2. |
| `cmd/tap/main.go` | modify | New flags `--webauthn-rp-id`, `--webauthn-origin`. Construct `webauthn.WebAuthn` from flags; pass into `MuxOpts`. |
| `go.mod` / `go.sum` | modify | Add `github.com/go-webauthn/webauthn`. |
| `web/src/lib/types.ts` | modify | Add `Session`, `TOTPEnrolmentBegin`, `Passkey`, `AdminUser` types; extend `User` with `has_totp`, `passkey_count`. |
| `web/src/lib/api.ts` | modify | TOTP, passkey, session-list, revoke, admin API calls. |
| `web/src/lib/auth.ts` | modify | `State` gains `totpEnabled`, `passkeyCount`; `bootstrap` populates them; add `beginPasskeyLogin`/`finishPasskeyLogin`. |
| `web/src/lib/__tests__/auth.test.ts` | modify | Passkey login flow; bootstrap populates TOTP/passkey state. |
| `web/src/lib/__tests__/api.test.ts` | modify | TOTP + passkey + admin API call coverage. |
| `web/src/lib/router.ts` | modify | Add `/settings` and `/admin` routes. |
| `web/src/views/Login.svelte` | modify | "Sign in with a passkey" button; TOTP second-step transition with pending_token in component state. |
| `web/src/views/__tests__/Login.test.ts` | modify | Passkey button present; TOTP step renders on `totp_required`; recovery code toggle works. |
| `web/src/views/settings/Security.svelte` | **create** | Session list (revoke + log-out-everywhere); TOTP enrolment/disable/regenerate flow with QR modal; passkey list with add/remove. |
| `web/src/views/__tests__/Security.test.ts` | **create** | All sub-sections covered. |
| `web/src/views/Admin.svelte` | **create** | User list table; create user; reset password; disable 2FA; disable/re-enable; delete with confirmation. |
| `web/src/views/__tests__/Admin.test.ts` | **create** | Non-admin redirect; all admin actions covered. |
| `web/src/App.svelte` | modify | Navigation links for Settings + Admin (admin-only). Route dispatch for `/settings`, `/admin`. |
| `README.md` | modify | M7 auth paragraph; breaking-migration upgrade note. |
| `CLAUDE.md` | modify | Status line M6 → M7 in progress. |
| `docs/specs/2026-05-10-m7-2fa-passkeys-isolation.md` | — | Source of truth. Do not modify during implementation. |

---

## Phase A — New dependency + schema migrations

### Task A1: Add `github.com/go-webauthn/webauthn` dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Resolve current library API**

    Invoke `mcp__plugin_context7_context7__query-docs` with library ID `/go-webauthn/webauthn` and query "BeginRegistration FinishRegistration BeginDiscoverableLogin FinishDiscoverableLogin webauthn.User interface". Read the returned signatures carefully — record `webauthn.User` interface requirements (`WebAuthnID`, `WebAuthnName`, `WebAuthnDisplayName`, `WebAuthnCredentials`) and the `webauthn.Config` struct fields.

- [ ] **Step 2: Add the module**

```bash
cd /home/ben.guest/Users/ben/src/tap
go get github.com/go-webauthn/webauthn@latest
go mod tidy
```

- [ ] **Step 3: Verify it lands clean**

```bash
grep 'go-webauthn/webauthn' go.mod
go build ./...
```

Expected: module in `require` block; build succeeds.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "M7: add go-webauthn/webauthn dependency"
```

---

### Task A2: Migration 0006 — per-user data isolation columns + widen unique constraint

**Files:**
- Create: `internal/db/migrations/0006_user_data_isolation.sql`
- Modify: `internal/db/subscriptions.go` (error string for ErrSubscriptionExists)

**Why table recreation is needed:** M1's `subscriptions` table has `feed_url TEXT NOT NULL UNIQUE` — a global uniqueness constraint. In a multi-user deployment this prevents two users from subscribing to the same feed. M7 must widen it to `UNIQUE(user_id, feed_url)`. SQLite cannot drop a constraint via `ALTER TABLE`, so `subscriptions` must be recreated. `entries` can use a plain `ALTER TABLE` (just adds a column to what will be an empty table on fresh install).

- [ ] **Step 1: Write the migration**

```sql
-- internal/db/migrations/0006_user_data_isolation.sql

-- Recreate subscriptions with user_id and UNIQUE(user_id, feed_url).
-- The global UNIQUE(feed_url) from M1 would prevent two users from
-- subscribing to the same feed URL.
ALTER TABLE subscriptions RENAME TO subscriptions_old;

CREATE TABLE subscriptions (
    id                INTEGER PRIMARY KEY,
    user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title             TEXT    NOT NULL,
    feed_url          TEXT    NOT NULL,
    site_url          TEXT,
    last_poll_at      INTEGER,
    next_poll_at      INTEGER NOT NULL,
    etag              TEXT,
    last_modified     TEXT,
    error_count       INTEGER NOT NULL DEFAULT 0,
    last_error        TEXT,
    created_at        INTEGER NOT NULL,
    velocity_24h_x100 INTEGER NOT NULL DEFAULT 0,
    extract           INTEGER NOT NULL DEFAULT 0,
    extract_selector  TEXT    NOT NULL DEFAULT '',
    cookie            TEXT    NOT NULL DEFAULT '',
    basic_auth_user   TEXT    NOT NULL DEFAULT '',
    basic_auth_pass   TEXT    NOT NULL DEFAULT '',
    UNIQUE (user_id, feed_url)
);
CREATE INDEX idx_subscriptions_next_poll ON subscriptions(next_poll_at);
CREATE INDEX idx_subscriptions_user      ON subscriptions(user_id);

INSERT INTO subscriptions
    SELECT id, 0, title, feed_url, site_url, last_poll_at, next_poll_at,
           etag, last_modified, error_count, last_error, created_at,
           velocity_24h_x100, extract, extract_selector, cookie,
           basic_auth_user, basic_auth_pass
    FROM subscriptions_old;

DROP TABLE subscriptions_old;

-- entries: just add the column (table is empty on fresh install).
ALTER TABLE entries ADD COLUMN user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE;
CREATE INDEX idx_entries_user ON entries(user_id, published_at DESC);
```

- [ ] **Step 2: Update `ErrSubscriptionExists` string match in `internal/db/subscriptions.go`**

The UNIQUE constraint name changes from `subscriptions.feed_url` to `subscriptions.user_id, subscriptions.feed_url`. Update the detection string:

```go
// Before:
if strings.Contains(err.Error(), "UNIQUE constraint failed: subscriptions.feed_url") {

// After:
if strings.Contains(err.Error(), "UNIQUE constraint failed: subscriptions.user_id, subscriptions.feed_url") {
```

Also update the `ErrSubscriptionExists` comment to say "on `(user_id, feed_url)`" instead of "on `feed_url`".

Run: `go test ./internal/db/... -run TestInsertSubscription -v`
Expected: existing duplicate-feed-url test still passes (same user, same URL → still rejected).

- [ ] **Step 3: Write a failing migration test**

Add to `internal/db/migrate_test.go`:

```go
func TestMigrate_0006_UserDataIsolation(t *testing.T) {
    d := newTestDB(t)
    ctx := context.Background()

    // Verify user_id FK is enforced: insert with non-existent user_id must fail.
    _, err := d.ExecContext(ctx,
        `INSERT INTO subscriptions (user_id, title, feed_url, next_poll_at, created_at)
         VALUES (999, 't', 'http://x.com/feed', 0, 0)`)
    require.Error(t, err)
    require.Contains(t, err.Error(), "FOREIGN KEY")

    // Verify UNIQUE is now (user_id, feed_url): two different users can subscribe
    // to the same URL. Create two users first.
    res1, err := d.ExecContext(ctx,
        `INSERT INTO users (username, password_hash, role, created_at) VALUES ('u1','h','admin',0)`)
    require.NoError(t, err)
    uid1, _ := res1.LastInsertId()
    res2, err := d.ExecContext(ctx,
        `INSERT INTO users (username, password_hash, role, created_at) VALUES ('u2','h','admin',0)`)
    require.NoError(t, err)
    uid2, _ := res2.LastInsertId()

    _, err = d.ExecContext(ctx,
        `INSERT INTO subscriptions (user_id, title, feed_url, next_poll_at, created_at)
         VALUES (?, 'Feed', 'http://shared.example/feed', 0, 0)`, uid1)
    require.NoError(t, err, "first user should be able to subscribe")

    _, err = d.ExecContext(ctx,
        `INSERT INTO subscriptions (user_id, title, feed_url, next_poll_at, created_at)
         VALUES (?, 'Feed', 'http://shared.example/feed', 0, 0)`, uid2)
    require.NoError(t, err, "second user must be able to subscribe to the same URL")

    _, err = d.ExecContext(ctx,
        `INSERT INTO subscriptions (user_id, title, feed_url, next_poll_at, created_at)
         VALUES (?, 'Feed', 'http://shared.example/feed', 0, 0)`, uid1)
    require.Error(t, err, "same user subscribing to the same URL twice must fail")
    require.Contains(t, err.Error(), "UNIQUE constraint failed")
}
```

Run: `go test ./internal/db/... -run TestMigrate_0006 -v`
Expected: PASS.

- [ ] **Step 4: Verify full test suite**

```bash
make test
```

Expected: all db tests pass including the new one.

- [ ] **Step 5: Commit**

```bash
git add internal/db/migrations/0006_user_data_isolation.sql \
        internal/db/subscriptions.go \
        internal/db/migrate_test.go
git commit -m "M7: migration 0006 — user_id + widen feed_url UNIQUE to (user_id, feed_url)"
```

---

### Task A3: Migration 0007 — 2FA, passkeys, sessions metadata

**Files:**
- Create: `internal/db/migrations/0007_2fa_passkeys_sessions_meta.sql`

- [ ] **Step 1: Write the migration**

```sql
-- internal/db/migrations/0007_2fa_passkeys_sessions_meta.sql

-- sessions.user_id must become nullable for anonymous WebAuthn challenge sessions.
-- SQLite cannot drop NOT NULL via ALTER TABLE; recreate using rename→create→copy→drop.
ALTER TABLE sessions RENAME TO sessions_old;

CREATE TABLE sessions (
    id                  INTEGER PRIMARY KEY,
    user_id             INTEGER REFERENCES users(id) ON DELETE CASCADE,
    token_hash          TEXT    NOT NULL UNIQUE,
    csrf_token          TEXT    NOT NULL,
    created_at          INTEGER NOT NULL,
    last_seen_at        INTEGER NOT NULL,
    idle_expires_at     INTEGER NOT NULL,
    absolute_expires_at INTEGER NOT NULL,
    user_agent          TEXT    NOT NULL DEFAULT '',
    address             TEXT    NOT NULL DEFAULT '',
    webauthn_challenge  BLOB
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

INSERT INTO sessions (id, user_id, token_hash, csrf_token, created_at, last_seen_at,
                      idle_expires_at, absolute_expires_at, user_agent, address, webauthn_challenge)
SELECT                id, user_id, token_hash, csrf_token, created_at, last_seen_at,
                      idle_expires_at, absolute_expires_at, '',         '',      NULL
FROM sessions_old;

DROP TABLE sessions_old;

-- The three new columns are included in the table recreation above.

CREATE TABLE pending_logins (
    id          INTEGER PRIMARY KEY,
    token_hash  TEXT    NOT NULL UNIQUE,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  INTEGER NOT NULL,
    expires_at  INTEGER NOT NULL
);
CREATE INDEX idx_pending_logins_expires ON pending_logins(expires_at);

CREATE TABLE totp_secrets (
    id                INTEGER PRIMARY KEY,
    user_id           INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    secret_encrypted  BLOB    NOT NULL,
    confirmed         INTEGER NOT NULL DEFAULT 0,
    created_at        INTEGER NOT NULL
);

CREATE TABLE recovery_codes (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash    TEXT    NOT NULL,
    consumed_at  INTEGER
);
CREATE INDEX idx_recovery_codes_user ON recovery_codes(user_id);

CREATE TABLE passkeys (
    id            INTEGER PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id BLOB    NOT NULL UNIQUE,
    public_key    BLOB    NOT NULL,
    sign_counter  INTEGER NOT NULL DEFAULT 0,
    aaguid        BLOB,
    label         TEXT    NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL
);
CREATE INDEX idx_passkeys_user ON passkeys(user_id);
```

- [ ] **Step 2: Add migration test**

Add to `internal/db/migrate_test.go`:

```go
func TestMigrate_0007_2FAAndPasskeys(t *testing.T) {
    d := newTestDB(t)
    ctx := context.Background()
    // Verify sessions table has new columns
    _, err := d.ExecContext(ctx,
        `SELECT user_agent, address, webauthn_challenge FROM sessions LIMIT 1`)
    require.NoError(t, err)
    // Verify new tables exist
    for _, tbl := range []string{"pending_logins", "totp_secrets", "recovery_codes", "passkeys"} {
        _, err = d.ExecContext(ctx, `SELECT 1 FROM `+tbl+` LIMIT 1`)
        require.NoError(t, err, "table %s should exist", tbl)
    }
    // sessions.user_id should be nullable (INSERT with NULL user_id should succeed)
    _, err = d.ExecContext(ctx,
        `INSERT INTO sessions (user_id, token_hash, csrf_token, created_at, last_seen_at, idle_expires_at, absolute_expires_at)
         VALUES (NULL, 'testhash0007', 'csrf', 0, 0, 9999999999, 9999999999)`)
    require.NoError(t, err, "sessions.user_id should be nullable for anonymous challenge sessions")
}
```

Run: `go test ./internal/db/... -run TestMigrate_0007 -v`
Expected: PASS (migration applied by test setup).

- [ ] **Step 3: Verify full test suite**

```bash
make test
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/db/migrations/0007_2fa_passkeys_sessions_meta.sql internal/db/migrate_test.go
git commit -m "M7: migration 0007 — 2FA, passkeys, sessions metadata"
```

---

## Phase B — `internal/auth` additions

### Task B1: TOTP primitives

**Files:**
- Create: `internal/auth/totp.go`, `internal/auth/totp_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/auth/totp_test.go`:

```go
package auth_test

import (
    "strings"
    "testing"
    "time"

    "github.com/bcrisp4/tap/internal/auth"
    "github.com/stretchr/testify/require"
)

func TestGenerateTOTPSecret(t *testing.T) {
    s1, err := auth.GenerateTOTPSecret()
    require.NoError(t, err)
    require.NotEmpty(t, s1)
    // base32 charset only
    require.True(t, isBase32(s1), "expected base32 string, got %q", s1)
    s2, err := auth.GenerateTOTPSecret()
    require.NoError(t, err)
    require.NotEqual(t, s1, s2, "two calls should return distinct secrets")
}

func TestTOTPSecretURI(t *testing.T) {
    uri := auth.TOTPSecretURI("JBSWY3DPEHPK3PXP", "Tap", "alice")
    require.True(t, strings.HasPrefix(uri, "otpauth://totp/"), "uri should start with otpauth://totp/")
    require.Contains(t, uri, "secret=JBSWY3DPEHPK3PXP")
    require.Contains(t, uri, "issuer=Tap")
}

func TestVerifyTOTP(t *testing.T) {
    secret, err := auth.GenerateTOTPSecret()
    require.NoError(t, err)

    // Generate a code for the current window
    now := time.Now().Unix()
    code := auth.GenerateTOTPCode(secret, now)

    require.True(t, auth.VerifyTOTP(secret, code), "current window code should be valid")
    require.False(t, auth.VerifyTOTP(secret, "000000"), "wrong code should be invalid")

    // Code two windows ago (60+ seconds stale) should fail
    staleCode := auth.GenerateTOTPCode(secret, now-60)
    require.False(t, auth.VerifyTOTP(secret, staleCode), "2-window-old code should be rejected")
}

func TestGenerateRecoveryCodes(t *testing.T) {
    codes, err := auth.GenerateRecoveryCodes()
    require.NoError(t, err)
    require.Len(t, codes, 8)
    seen := make(map[string]bool)
    for _, c := range codes {
        require.Len(t, c, 10, "each code should be 10 chars")
        for _, ch := range c {
            require.True(t, isRecoveryCodeChar(ch), "unexpected char %q in code %q", ch, c)
        }
        require.False(t, seen[c], "codes should be distinct")
        seen[c] = true
    }
}

func isBase32(s string) bool {
    const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
    for _, c := range s {
        if !strings.ContainsRune(alphabet, c) {
            return false
        }
    }
    return true
}

func isRecoveryCodeChar(r rune) bool {
    const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
    return strings.ContainsRune(alphabet, r)
}
```

Run: `go test ./internal/auth/... -run TestGenerateTOTPSecret -v`
Expected: FAIL (compile error — functions not yet defined).

- [ ] **Step 2: Implement `internal/auth/totp.go`**

```go
package auth

import (
    "crypto/hmac"
    "crypto/rand"
    "crypto/sha1" //nolint:gosec // TOTP (RFC 6238) mandates SHA-1
    "encoding/base32"
    "encoding/binary"
    "fmt"
    "math"
    "net/url"
    "strings"
    "time"
)

// GenerateTOTPSecret returns a base32-encoded 20-byte random secret.
func GenerateTOTPSecret() (string, error) {
    raw := make([]byte, 20)
    if _, err := rand.Read(raw); err != nil {
        return "", fmt.Errorf("generate totp secret: %w", err)
    }
    return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

// TOTPSecretURI returns the otpauth://totp/... URI for QR code display.
func TOTPSecretURI(secret, issuer, accountName string) string {
    label := url.PathEscape(issuer + ":" + accountName)
    v := url.Values{}
    v.Set("secret", secret)
    v.Set("issuer", issuer)
    v.Set("algorithm", "SHA1")
    v.Set("digits", "6")
    v.Set("period", "30")
    return "otpauth://totp/" + label + "?" + v.Encode()
}

// GenerateTOTPCode generates the 6-digit TOTP code for a given unix timestamp.
// Exported so tests can generate known codes for VerifyTOTP.
func GenerateTOTPCode(secret string, unixTime int64) string {
    key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(
        strings.ToUpper(secret))
    if err != nil {
        return ""
    }
    counter := uint64(math.Floor(float64(unixTime) / 30))
    buf := make([]byte, 8)
    binary.BigEndian.PutUint64(buf, counter)
    mac := hmac.New(sha1.New, key) //nolint:gosec // RFC 6238 mandates SHA-1
    _, _ = mac.Write(buf)
    h := mac.Sum(nil)
    offset := h[len(h)-1] & 0x0f
    code := (int(h[offset]&0x7f)<<24 |
        int(h[offset+1])<<16 |
        int(h[offset+2])<<8 |
        int(h[offset+3])) % 1_000_000
    return fmt.Sprintf("%06d", code)
}

// VerifyTOTP validates a 6-digit code against the TOTP secret.
// Accepts the current window ±1 (one step of clock-drift tolerance).
func VerifyTOTP(secret, code string) bool {
    now := time.Now().Unix()
    for _, offset := range []int64{-1, 0, 1} {
        if GenerateTOTPCode(secret, now+offset*30) == code {
            return true
        }
    }
    return false
}

// recoveryAlphabet excludes 0, O, 1, I to prevent misreading.
const recoveryAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GenerateRecoveryCodes returns 8 cryptographically random 10-char codes.
func GenerateRecoveryCodes() ([]string, error) {
    codes := make([]string, 8)
    for i := range codes {
        buf := make([]byte, 10)
        if _, err := rand.Read(buf); err != nil {
            return nil, fmt.Errorf("generate recovery codes: %w", err)
        }
        var sb strings.Builder
        for _, b := range buf {
            sb.WriteByte(recoveryAlphabet[int(b)%len(recoveryAlphabet)])
        }
        codes[i] = sb.String()
    }
    return codes, nil
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/auth/... -run 'TestGenerateTOTPSecret|TestTOTPSecretURI|TestVerifyTOTP|TestGenerateRecoveryCodes' -v
```

Expected: all 4 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/auth/totp.go internal/auth/totp_test.go
git commit -m "M7: TOTP primitives — GenerateTOTPSecret, VerifyTOTP, GenerateRecoveryCodes"
```

---

### Task B2: AES-GCM TOTP secret encryption

**Files:**
- Create: `internal/auth/encrypt.go`, `internal/auth/encrypt_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/auth/encrypt_test.go`:

```go
package auth_test

import (
    "testing"

    "github.com/bcrisp4/tap/internal/auth"
    "github.com/stretchr/testify/require"
)

func TestEncryptDecryptTOTPSecret(t *testing.T) {
    key := make([]byte, 32)
    for i := range key { key[i] = byte(i) }

    secret := "JBSWY3DPEHPK3PXP"
    ciphertext, err := auth.EncryptTOTPSecret(key, secret)
    require.NoError(t, err)
    require.NotEmpty(t, ciphertext)

    got, err := auth.DecryptTOTPSecret(key, ciphertext)
    require.NoError(t, err)
    require.Equal(t, secret, got)
}

func TestEncryptTOTPSecret_WrongKey(t *testing.T) {
    key1 := make([]byte, 32)
    key2 := make([]byte, 32)
    for i := range key2 { key2[i] = 0xFF }

    ct, err := auth.EncryptTOTPSecret(key1, "TESTSECRET")
    require.NoError(t, err)

    _, err = auth.DecryptTOTPSecret(key2, ct)
    require.Error(t, err, "wrong key should fail to decrypt")
}

func TestEncryptTOTPSecret_NonceRandomness(t *testing.T) {
    key := make([]byte, 32)
    secret := "SAMEPLAINTEXT"

    ct1, err := auth.EncryptTOTPSecret(key, secret)
    require.NoError(t, err)
    ct2, err := auth.EncryptTOTPSecret(key, secret)
    require.NoError(t, err)
    require.NotEqual(t, ct1, ct2, "two encryptions of the same secret should differ (random nonce)")
}
```

Run: `go test ./internal/auth/... -run TestEncrypt -v`
Expected: FAIL (compile error).

- [ ] **Step 2: Implement `internal/auth/encrypt.go`**

```go
package auth

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "fmt"
    "io"
)

// EncryptTOTPSecret encrypts a base32 TOTP secret using AES-256-GCM.
// The 12-byte nonce is prepended to the ciphertext in the returned slice.
// key must be exactly 32 bytes.
func EncryptTOTPSecret(key []byte, secret string) ([]byte, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, fmt.Errorf("encrypt totp secret: new cipher: %w", err)
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("encrypt totp secret: new gcm: %w", err)
    }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, fmt.Errorf("encrypt totp secret: read nonce: %w", err)
    }
    ciphertext := gcm.Seal(nonce, nonce, []byte(secret), nil)
    return ciphertext, nil
}

// DecryptTOTPSecret reverses EncryptTOTPSecret.
// Returns an error if the key is wrong or the ciphertext is malformed.
func DecryptTOTPSecret(key []byte, ciphertext []byte) (string, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", fmt.Errorf("decrypt totp secret: new cipher: %w", err)
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("decrypt totp secret: new gcm: %w", err)
    }
    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return "", fmt.Errorf("decrypt totp secret: ciphertext too short")
    }
    nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ct, nil)
    if err != nil {
        return "", fmt.Errorf("decrypt totp secret: %w", err)
    }
    return string(plaintext), nil
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/auth/... -run TestEncrypt -v
```

Expected: all 3 tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/auth/encrypt.go internal/auth/encrypt_test.go
git commit -m "M7: AES-256-GCM encryption for TOTP secrets"
```

---

### Task B3: Pending login token

**Files:**
- Create: `internal/auth/pending.go`, `internal/auth/pending_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/auth/pending_test.go`:

```go
package auth_test

import (
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "testing"

    "github.com/bcrisp4/tap/internal/auth"
    "github.com/stretchr/testify/require"
)

func TestMintPendingToken(t *testing.T) {
    value, hash, err := auth.MintPendingToken()
    require.NoError(t, err)
    // value is 43 unpadded base64url chars (32 bytes)
    require.Len(t, value, 43)
    // hash is sha256 of the raw bytes
    raw, err := base64.RawURLEncoding.DecodeString(value)
    require.NoError(t, err)
    sum := sha256.Sum256(raw)
    require.Equal(t, hex.EncodeToString(sum[:]), hash)
    // Two calls return distinct values
    v2, _, _ := auth.MintPendingToken()
    require.NotEqual(t, value, v2)
}
```

Run: `go test ./internal/auth/... -run TestMintPendingToken -v`
Expected: FAIL (compile error).

- [ ] **Step 2: Implement `internal/auth/pending.go`**

```go
package auth

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
    "fmt"
)

// MintPendingToken returns (value, tokenHash, err) for a two-step login token.
// Same shape as MintSessionToken: value is base64url(32 random bytes),
// tokenHash is hex(sha256(those 32 bytes)) for storage.
func MintPendingToken() (value, tokenHash string, err error) {
    raw := make([]byte, 32)
    if _, err := rand.Read(raw); err != nil {
        return "", "", fmt.Errorf("mint pending token: %w", err)
    }
    sum := sha256.Sum256(raw)
    return base64.RawURLEncoding.EncodeToString(raw), hex.EncodeToString(sum[:]), nil
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/auth/... -run TestMintPendingToken -v
```

Expected: PASS.

- [ ] **Step 4: Run full auth test suite**

```bash
go test ./internal/auth/... -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/pending.go internal/auth/pending_test.go
git commit -m "M7: MintPendingToken for TOTP two-step login"
```

---

## Phase C — `internal/db` additions

### Task C1: TOTP and recovery code db functions

**Files:**
- Create: `internal/db/totp.go`, `internal/db/totp_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/db/totp_test.go`:

```go
package db_test

import (
    "context"
    "testing"

    "github.com/bcrisp4/tap/internal/db"
    "github.com/stretchr/testify/require"
)

func TestTOTPSecret_Roundtrip(t *testing.T) {
    d := newTestDB(t)
    ctx := context.Background()
    userID := insertTestUser(t, d, "alice")

    err := db.InsertTOTPSecret(ctx, d, userID, []byte("encryptedbytes"))
    require.NoError(t, err)

    s, err := db.GetTOTPSecret(ctx, d, userID)
    require.NoError(t, err)
    require.Equal(t, userID, s.UserID)
    require.Equal(t, []byte("encryptedbytes"), s.SecretEncrypted)
    require.False(t, s.Confirmed)
}

func TestTOTPSecret_ConfirmAndDelete(t *testing.T) {
    d := newTestDB(t)
    ctx := context.Background()
    userID := insertTestUser(t, d, "bob")

    require.NoError(t, db.InsertTOTPSecret(ctx, d, userID, []byte("enc")))
    require.NoError(t, db.ConfirmTOTPSecret(ctx, d, userID))

    s, err := db.GetTOTPSecret(ctx, d, userID)
    require.NoError(t, err)
    require.True(t, s.Confirmed)

    require.NoError(t, db.DeleteTOTPSecret(ctx, d, userID))
    _, err = db.GetTOTPSecret(ctx, d, userID)
    require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestRecoveryCodes_InsertConsumeDelete(t *testing.T) {
    d := newTestDB(t)
    ctx := context.Background()
    userID := insertTestUser(t, d, "carol")

    hashes := []string{"hash1", "hash2", "hash3"}
    require.NoError(t, db.InsertRecoveryCodes(ctx, d, userID, hashes))

    codes, err := db.GetUnconsumedRecoveryCodes(ctx, d, userID)
    require.NoError(t, err)
    require.Len(t, codes, 3)

    require.NoError(t, db.ConsumeRecoveryCode(ctx, d, codes[0].ID))
    remaining, err := db.GetUnconsumedRecoveryCodes(ctx, d, userID)
    require.NoError(t, err)
    require.Len(t, remaining, 2)

    require.NoError(t, db.DeleteRecoveryCodes(ctx, d, userID))
    empty, err := db.GetUnconsumedRecoveryCodes(ctx, d, userID)
    require.NoError(t, err)
    require.Empty(t, empty)
}
```

Run: `go test ./internal/db/... -run 'TestTOTP|TestRecovery' -v`
Expected: FAIL (compile error — types not yet defined).

- [ ] **Step 2: Implement `internal/db/totp.go`**

```go
package db

import (
    "context"
    "database/sql"
    "fmt"
    "time"
)

type TOTPSecret struct {
    ID              int64
    UserID          int64
    SecretEncrypted []byte
    Confirmed       bool
    CreatedAt       int64
}

type RecoveryCode struct {
    ID         int64
    UserID     int64
    CodeHash   string
    ConsumedAt sql.NullInt64
}

func InsertTOTPSecret(ctx context.Context, d *sql.DB, userID int64, encrypted []byte) error {
    _, err := d.ExecContext(ctx,
        `INSERT INTO totp_secrets (user_id, secret_encrypted, confirmed, created_at)
         VALUES (?, ?, 0, ?)
         ON CONFLICT(user_id) DO UPDATE SET secret_encrypted=excluded.secret_encrypted,
         confirmed=0, created_at=excluded.created_at`,
        userID, encrypted, time.Now().Unix())
    if err != nil {
        return fmt.Errorf("insert totp secret: %w", err)
    }
    return nil
}

func GetTOTPSecret(ctx context.Context, d *sql.DB, userID int64) (TOTPSecret, error) {
    var s TOTPSecret
    err := d.QueryRowContext(ctx,
        `SELECT id, user_id, secret_encrypted, confirmed, created_at
         FROM totp_secrets WHERE user_id = ?`, userID).
        Scan(&s.ID, &s.UserID, &s.SecretEncrypted, &s.Confirmed, &s.CreatedAt)
    if err != nil {
        return TOTPSecret{}, err
    }
    return s, nil
}

func ConfirmTOTPSecret(ctx context.Context, d *sql.DB, userID int64) error {
    _, err := d.ExecContext(ctx,
        `UPDATE totp_secrets SET confirmed = 1 WHERE user_id = ?`, userID)
    if err != nil {
        return fmt.Errorf("confirm totp secret: %w", err)
    }
    return nil
}

func DeleteTOTPSecret(ctx context.Context, d *sql.DB, userID int64) error {
    _, err := d.ExecContext(ctx, `DELETE FROM totp_secrets WHERE user_id = ?`, userID)
    if err != nil {
        return fmt.Errorf("delete totp secret: %w", err)
    }
    return nil
}

func InsertRecoveryCodes(ctx context.Context, d *sql.DB, userID int64, hashes []string) error {
    tx, err := d.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("insert recovery codes: begin tx: %w", err)
    }
    defer tx.Rollback() //nolint:errcheck
    for _, h := range hashes {
        if _, err := tx.ExecContext(ctx,
            `INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)`, userID, h); err != nil {
            return fmt.Errorf("insert recovery codes: %w", err)
        }
    }
    return tx.Commit()
}

func GetUnconsumedRecoveryCodes(ctx context.Context, d *sql.DB, userID int64) ([]RecoveryCode, error) {
    rows, err := d.QueryContext(ctx,
        `SELECT id, user_id, code_hash, consumed_at FROM recovery_codes
         WHERE user_id = ? AND consumed_at IS NULL`, userID)
    if err != nil {
        return nil, fmt.Errorf("get recovery codes: %w", err)
    }
    defer rows.Close()
    var codes []RecoveryCode
    for rows.Next() {
        var c RecoveryCode
        if err := rows.Scan(&c.ID, &c.UserID, &c.CodeHash, &c.ConsumedAt); err != nil {
            return nil, fmt.Errorf("scan recovery code: %w", err)
        }
        codes = append(codes, c)
    }
    return codes, rows.Err()
}

func ConsumeRecoveryCode(ctx context.Context, d *sql.DB, id int64) error {
    _, err := d.ExecContext(ctx,
        `UPDATE recovery_codes SET consumed_at = ? WHERE id = ?`, time.Now().Unix(), id)
    if err != nil {
        return fmt.Errorf("consume recovery code: %w", err)
    }
    return nil
}

func DeleteRecoveryCodes(ctx context.Context, d *sql.DB, userID int64) error {
    _, err := d.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, userID)
    if err != nil {
        return fmt.Errorf("delete recovery codes: %w", err)
    }
    return nil
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/db/... -run 'TestTOTP|TestRecovery' -v
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/db/totp.go internal/db/totp_test.go
git commit -m "M7: db layer for TOTP secrets and recovery codes"
```

---

### Task C2: Passkey db functions

**Files:**
- Create: `internal/db/passkeys.go`, `internal/db/passkeys_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/db/passkeys_test.go`:

```go
package db_test

import (
    "context"
    "testing"
    "time"

    "github.com/bcrisp4/tap/internal/db"
    "github.com/stretchr/testify/require"
)

func TestPasskey_Roundtrip(t *testing.T) {
    d := newTestDB(t)
    ctx := context.Background()
    userID := insertTestUser(t, d, "dave")

    p := db.Passkey{
        UserID:       userID,
        CredentialID: []byte("credid1"),
        PublicKey:    []byte("pubkey1"),
        SignCounter:  0,
        AAGUID:       []byte("aaguid1"),
        Label:        "MacBook Touch ID",
        CreatedAt:    time.Now().Unix(),
    }
    id, err := db.InsertPasskey(ctx, d, p)
    require.NoError(t, err)
    require.Positive(t, id)

    got, err := db.GetPasskeyByCredentialID(ctx, d, []byte("credid1"))
    require.NoError(t, err)
    require.Equal(t, userID, got.UserID)
    require.Equal(t, "MacBook Touch ID", got.Label)
}

func TestPasskey_ListAndDelete(t *testing.T) {
    d := newTestDB(t)
    ctx := context.Background()
    u1 := insertTestUser(t, d, "eve")
    u2 := insertTestUser(t, d, "frank")

    for i, cred := range []string{"c1", "c2"} {
        _, err := db.InsertPasskey(ctx, d, db.Passkey{
            UserID: u1, CredentialID: []byte(cred),
            PublicKey: []byte("pk"), CreatedAt: int64(i),
        })
        require.NoError(t, err)
    }

    list, err := db.GetPasskeysByUserID(ctx, d, u1)
    require.NoError(t, err)
    require.Len(t, list, 2)

    list2, err := db.GetPasskeysByUserID(ctx, d, u2)
    require.NoError(t, err)
    require.Empty(t, list2, "u2 should have no passkeys")

    require.NoError(t, db.DeletePasskey(ctx, d, list[0].ID))
    list, err = db.GetPasskeysByUserID(ctx, d, u1)
    require.NoError(t, err)
    require.Len(t, list, 1)
}

func TestPasskey_UpdateSignCounter(t *testing.T) {
    d := newTestDB(t)
    ctx := context.Background()
    userID := insertTestUser(t, d, "grace")

    id, err := db.InsertPasskey(ctx, d, db.Passkey{
        UserID: userID, CredentialID: []byte("cred"),
        PublicKey: []byte("pk"), CreatedAt: 0,
    })
    require.NoError(t, err)
    require.NoError(t, db.UpdatePasskeySignCounter(ctx, d, id, 42))

    got, err := db.GetPasskeyByCredentialID(ctx, d, []byte("cred"))
    require.NoError(t, err)
    require.Equal(t, int64(42), got.SignCounter)
}
```

Run: `go test ./internal/db/... -run TestPasskey -v`
Expected: FAIL (compile error).

- [ ] **Step 2: Implement `internal/db/passkeys.go`**

```go
package db

import (
    "context"
    "database/sql"
    "fmt"
)

type Passkey struct {
    ID           int64
    UserID       int64
    CredentialID []byte
    PublicKey    []byte
    SignCounter  int64
    AAGUID       []byte
    Label        string
    CreatedAt    int64
}

func InsertPasskey(ctx context.Context, d *sql.DB, p Passkey) (int64, error) {
    res, err := d.ExecContext(ctx,
        `INSERT INTO passkeys (user_id, credential_id, public_key, sign_counter, aaguid, label, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
        p.UserID, p.CredentialID, p.PublicKey, p.SignCounter, p.AAGUID, p.Label, p.CreatedAt)
    if err != nil {
        return 0, fmt.Errorf("insert passkey: %w", err)
    }
    return res.LastInsertId()
}

func GetPasskeysByUserID(ctx context.Context, d *sql.DB, userID int64) ([]Passkey, error) {
    rows, err := d.QueryContext(ctx,
        `SELECT id, user_id, credential_id, public_key, sign_counter, aaguid, label, created_at
         FROM passkeys WHERE user_id = ? ORDER BY created_at`, userID)
    if err != nil {
        return nil, fmt.Errorf("get passkeys: %w", err)
    }
    defer rows.Close()
    var out []Passkey
    for rows.Next() {
        var p Passkey
        if err := rows.Scan(&p.ID, &p.UserID, &p.CredentialID, &p.PublicKey,
            &p.SignCounter, &p.AAGUID, &p.Label, &p.CreatedAt); err != nil {
            return nil, fmt.Errorf("scan passkey: %w", err)
        }
        out = append(out, p)
    }
    return out, rows.Err()
}

func GetPasskeyByCredentialID(ctx context.Context, d *sql.DB, credentialID []byte) (Passkey, error) {
    var p Passkey
    err := d.QueryRowContext(ctx,
        `SELECT id, user_id, credential_id, public_key, sign_counter, aaguid, label, created_at
         FROM passkeys WHERE credential_id = ?`, credentialID).
        Scan(&p.ID, &p.UserID, &p.CredentialID, &p.PublicKey,
            &p.SignCounter, &p.AAGUID, &p.Label, &p.CreatedAt)
    if err != nil {
        return Passkey{}, err
    }
    return p, nil
}

func UpdatePasskeySignCounter(ctx context.Context, d *sql.DB, id, counter int64) error {
    _, err := d.ExecContext(ctx,
        `UPDATE passkeys SET sign_counter = ? WHERE id = ?`, counter, id)
    if err != nil {
        return fmt.Errorf("update passkey sign counter: %w", err)
    }
    return nil
}

func DeletePasskey(ctx context.Context, d *sql.DB, id int64) error {
    _, err := d.ExecContext(ctx, `DELETE FROM passkeys WHERE id = ?`, id)
    if err != nil {
        return fmt.Errorf("delete passkey: %w", err)
    }
    return nil
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/db/... -run TestPasskey -v
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/db/passkeys.go internal/db/passkeys_test.go
git commit -m "M7: db layer for passkeys"
```

---

### Task C3: Pending logins db functions

**Files:**
- Create: `internal/db/pending_logins.go`, `internal/db/pending_logins_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/db/pending_logins_test.go`:

```go
package db_test

import (
    "context"
    "database/sql"
    "testing"
    "time"

    "github.com/bcrisp4/tap/internal/db"
    "github.com/stretchr/testify/require"
)

func TestPendingLogin_Roundtrip(t *testing.T) {
    d := newTestDB(t)
    ctx := context.Background()
    userID := insertTestUser(t, d, "hannah")

    now := time.Now().Unix()
    err := db.InsertPendingLogin(ctx, d, userID, "testhash123", now+300)
    require.NoError(t, err)

    pl, err := db.GetPendingLoginByTokenHash(ctx, d, "testhash123")
    require.NoError(t, err)
    require.Equal(t, userID, pl.UserID)

    require.NoError(t, db.DeletePendingLogin(ctx, d, pl.ID))
    _, err = db.GetPendingLoginByTokenHash(ctx, d, "testhash123")
    require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestPendingLogin_DeleteExpired(t *testing.T) {
    d := newTestDB(t)
    ctx := context.Background()
    userID := insertTestUser(t, d, "ivan")

    now := time.Now().Unix()
    // expired
    require.NoError(t, db.InsertPendingLogin(ctx, d, userID, "expiredhash", now-1))
    // not expired
    require.NoError(t, db.InsertPendingLogin(ctx, d, userID, "validhash", now+300))

    require.NoError(t, db.DeleteExpiredPendingLogins(ctx, d, now))

    _, err := db.GetPendingLoginByTokenHash(ctx, d, "expiredhash")
    require.ErrorIs(t, err, sql.ErrNoRows, "expired token should be deleted")

    _, err = db.GetPendingLoginByTokenHash(ctx, d, "validhash")
    require.NoError(t, err, "valid token should survive")
}
```

Run: `go test ./internal/db/... -run TestPendingLogin -v`
Expected: FAIL (compile error).

- [ ] **Step 2: Implement `internal/db/pending_logins.go`**

```go
package db

import (
    "context"
    "database/sql"
    "fmt"
    "time"
)

type PendingLogin struct {
    ID        int64
    TokenHash string
    UserID    int64
    CreatedAt int64
    ExpiresAt int64
}

func InsertPendingLogin(ctx context.Context, d *sql.DB, userID int64, tokenHash string, expiresAt int64) error {
    _, err := d.ExecContext(ctx,
        `INSERT INTO pending_logins (user_id, token_hash, created_at, expires_at) VALUES (?, ?, ?, ?)`,
        userID, tokenHash, time.Now().Unix(), expiresAt)
    if err != nil {
        return fmt.Errorf("insert pending login: %w", err)
    }
    return nil
}

func GetPendingLoginByTokenHash(ctx context.Context, d *sql.DB, tokenHash string) (PendingLogin, error) {
    var p PendingLogin
    err := d.QueryRowContext(ctx,
        `SELECT id, token_hash, user_id, created_at, expires_at
         FROM pending_logins WHERE token_hash = ?`, tokenHash).
        Scan(&p.ID, &p.TokenHash, &p.UserID, &p.CreatedAt, &p.ExpiresAt)
    if err != nil {
        return PendingLogin{}, err
    }
    return p, nil
}

func DeletePendingLogin(ctx context.Context, d *sql.DB, id int64) error {
    _, err := d.ExecContext(ctx, `DELETE FROM pending_logins WHERE id = ?`, id)
    if err != nil {
        return fmt.Errorf("delete pending login: %w", err)
    }
    return nil
}

func DeleteExpiredPendingLogins(ctx context.Context, d *sql.DB, now int64) error {
    _, err := d.ExecContext(ctx, `DELETE FROM pending_logins WHERE expires_at <= ?`, now)
    if err != nil {
        return fmt.Errorf("delete expired pending logins: %w", err)
    }
    return nil
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/db/... -run TestPendingLogin -v
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/db/pending_logins.go internal/db/pending_logins_test.go
git commit -m "M7: db layer for pending two-step login tokens"
```

---

### Task C4: Update `db.Session`, sessions functions, and per-user subscription/entry filters

**Files:**
- Modify: `internal/db/sessions.go`, `internal/db/sessions_test.go`
- Modify: `internal/db/subscriptions.go`, `internal/db/subscriptions_test.go`
- Modify: `internal/db/entries.go`, `internal/db/entries_test.go`
- Modify: `internal/db/users.go`, `internal/db/users_test.go`

- [ ] **Step 1: Write failing per-user isolation tests (RED)**

Add to `internal/db/subscriptions_test.go`:

```go
func TestSubscriptions_UserIsolation(t *testing.T) {
    d := newTestDB(t)
    ctx := context.Background()
    u1 := insertTestUser(t, d, "u1")
    u2 := insertTestUser(t, d, "u2")

    _, err := db.InsertSubscription(ctx, d, db.NewSubscription{
        UserID: u1, Title: "Feed A", FeedURL: "http://a.com/feed",
        NextPoll: 0, Created: 0,
    })
    require.NoError(t, err)

    list, err := db.ListSubscriptions(ctx, d, u2)
    require.NoError(t, err)
    require.Empty(t, list, "u2 should not see u1's subscriptions")

    list, err = db.ListSubscriptions(ctx, d, u1)
    require.NoError(t, err)
    require.Len(t, list, 1)
}
```

Add equivalent `TestEntries_UserIsolation` to `internal/db/entries_test.go`. Add `TestSessions_ListByUserID` to `internal/db/sessions_test.go`.

Run: `go test ./internal/db/... -run 'TestSubscriptions_UserIsolation|TestEntries_UserIsolation|TestSessions_ListByUserID' -v`
Expected: FAIL (compile error — `userID` parameter not yet accepted).

- [ ] **Step 2: Add per-user filter to subscriptions queries (GREEN)**

In `internal/db/subscriptions.go`, add `userID int64` parameter to `ListSubscriptions`, `GetSubscription`, `InsertSubscription`, `UpdateSubscription*`, and `DeleteSubscription`. Add `WHERE user_id = ?` (or `AND user_id = ?`) to each. Add `UserID int64` to `NewSubscription`.

- [ ] **Step 3: Same treatment for entries, including `UpdateAfterPoll` (GREEN)**

In `internal/db/entries.go`, add `userID int64` to `ListEntries`, `GetEntry`, `UpdateEntry`. Add `WHERE user_id = ?` clauses.

**CRITICAL:** Also update `UpdateAfterPoll`. After migration 0006, `entries.user_id` is `NOT NULL`. The existing `INSERT INTO entries` in `UpdateAfterPoll` (at `internal/db/entries.go:198`) does not include `user_id`, so every poll will fail with a NOT NULL constraint. Fix by:
1. Adding `userID int64` to `PollResult` (or as a separate parameter to `UpdateAfterPoll`).
2. Including `user_id` in the INSERT:

```go
res, ierr := tx.ExecContext(ctx, `
    INSERT INTO entries (subscription_id, user_id, hash, title, author, url, content, published_at, fetched_at, extract_failed)
    VALUES (?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)
    ON CONFLICT (subscription_id, hash) DO NOTHING
`, subID, r.UserID, e.Hash, e.Title, e.Author, e.URL, e.Content, e.PublishedAt, r.NowUnix, boolToInt(e.ExtractFailed))
```

`r.UserID` is populated by the poll worker from `DueSubscription.UserID` (which it already reads for per-feed credentials). Update `poll/worker.go` accordingly when it constructs the `PollResult`.

- [ ] **Step 4: Update `db.Session`, `db.NewSession`, and all session query functions in `internal/db/sessions.go`**

Add `UserAgent string`, `Address string`, `WebAuthnChallenge []byte` fields to both structs. Update:
- `InsertSession`: include `user_agent`, `address`, `webauthn_challenge` in the INSERT.
- `GetSessionByTokenHash`: extend the SELECT list and Scan call to include the three new columns. This is load-bearing — `requireSession` calls this function, and the session management endpoints need `UserAgent`/`Address` for the session listing UI. Omitting them causes silent zero-values in all session rows returned by the middleware.

Add:

```go
// ListSessionsByUserID returns all non-anonymous sessions for the user.
func ListSessionsByUserID(ctx context.Context, d *sql.DB, userID int64) ([]Session, error) {
    rows, err := d.QueryContext(ctx,
        `SELECT id, user_id, token_hash, csrf_token, created_at, last_seen_at,
                idle_expires_at, absolute_expires_at, user_agent, address
         FROM sessions WHERE user_id = ? ORDER BY created_at DESC`, userID)
    // ... scan rows, return []Session, rows.Err()
}

func SetWebAuthnChallenge(ctx context.Context, d *sql.DB, sessionID int64, challenge []byte) error {
    _, err := d.ExecContext(ctx,
        `UPDATE sessions SET webauthn_challenge = ? WHERE id = ?`, challenge, sessionID)
    return err
}

func ClearWebAuthnChallenge(ctx context.Context, d *sql.DB, sessionID int64) error {
    _, err := d.ExecContext(ctx,
        `UPDATE sessions SET webauthn_challenge = NULL WHERE id = ?`, sessionID)
    return err
}
```

- [ ] **Step 5: Add new user db functions**

In `internal/db/users.go`, add:

```go
func ListUsers(ctx context.Context, d *sql.DB) ([]User, error)
func DeleteUser(ctx context.Context, d *sql.DB, id int64) error
func EnableUser(ctx context.Context, d *sql.DB, id int64) error
func GetUserTOTPStatus(ctx context.Context, d *sql.DB, userID int64) (hasTOTP, confirmed bool, err error)
func GetUserPasskeyCount(ctx context.Context, d *sql.DB, userID int64) (int, error)
```

- [ ] **Step 6: Run tests (confirm GREEN)**

```bash
go test ./internal/db/... -race
```

Expected: all pass including isolation tests.

- [ ] **Step 7: Commit**

```bash
git add internal/db/sessions.go internal/db/sessions_test.go \
        internal/db/subscriptions.go internal/db/subscriptions_test.go \
        internal/db/entries.go internal/db/entries_test.go \
        internal/db/users.go internal/db/users_test.go
git commit -m "M7: per-user isolation on subscriptions/entries; sessions metadata; user admin helpers"
```

---

## Phase D — API layer

### Task D1: Error codes + `requireAdmin` middleware

**Files:**
- Modify: `internal/api/errors.go`
- Modify: `internal/api/middleware.go`, `internal/api/middleware_test.go`

- [ ] **Step 1: Add error codes to `internal/api/errors.go`**

```go
const (
    // existing codes omitted for brevity — append these:
    ErrCodeTOTPRequired               = "totp_required"
    ErrCodeTOTPInvalid                = "totp_invalid"
    ErrCodeRecoveryCodeInvalid        = "recovery_code_invalid"
    ErrCodeTOTPNotEnrolled            = "totp_not_enrolled"
    ErrCodeTOTPAlreadyEnrolled        = "totp_already_enrolled"
    ErrCodePasskeyNotFound            = "passkey_not_found"
    ErrCodeCannotRevokeCurrentSession = "cannot_revoke_current_session"
    ErrCodeAdminRequired              = "admin_required"
)
```

- [ ] **Step 2: Write failing test for `requireAdmin`**

Add to `internal/api/middleware_test.go`:

```go
func TestRequireAdmin(t *testing.T) {
    reached := false
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        reached = true
        w.WriteHeader(http.StatusOK)
    })

    t.Run("admin passes", func(t *testing.T) {
        reached = false
        w := httptest.NewRecorder()
        r := httptest.NewRequest(http.MethodGet, "/", nil)
        // inject admin user into context
        ctx := context.WithValue(r.Context(), ctxKeyUser, db.User{Role: "admin"})
        requireAdmin()(handler).ServeHTTP(w, r.WithContext(ctx))
        require.True(t, reached)
        require.Equal(t, http.StatusOK, w.Code)
    })

    t.Run("non-admin 403", func(t *testing.T) {
        reached = false
        w := httptest.NewRecorder()
        r := httptest.NewRequest(http.MethodGet, "/", nil)
        ctx := context.WithValue(r.Context(), ctxKeyUser, db.User{Role: "user"})
        requireAdmin()(handler).ServeHTTP(w, r.WithContext(ctx))
        require.False(t, reached)
        require.Equal(t, http.StatusForbidden, w.Code)
    })
}
```

Run: `go test ./internal/api/... -run TestRequireAdmin -v`
Expected: FAIL (requireAdmin not defined).

- [ ] **Step 3: Add `requireAdmin` to `internal/api/middleware.go`**

```go
// requireAdmin rejects requests whose context user is not role="admin".
// Must be chained after requireSession.
func requireAdmin() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            u, ok := userFromContext(r.Context())
            if !ok || u.Role != "admin" {
                writeError(w, http.StatusForbidden, ErrCodeAdminRequired, "admin access required")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

Also update `InsertSession` call site in `loginHandler` (in `auth.go`) to capture `UserAgent` from `r.Header.Get("User-Agent")` and `Address` extracted from `r.RemoteAddr` (first non-private IP from `X-Forwarded-For`, falling back to `RemoteAddr`).

- [ ] **Step 4: Run tests**

```bash
go test ./internal/api/... -run TestRequireAdmin -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/errors.go internal/api/middleware.go internal/api/middleware_test.go
git commit -m "M7: requireAdmin middleware + new error codes"
```

---

### Task D2: Extend login handler for TOTP second step + session management endpoints

**Files:**
- Modify: `internal/api/auth.go`, `internal/api/auth_test.go`

- [ ] **Step 1: Write failing tests for TOTP login and session list**

Add to `internal/api/auth_test.go`:

```go
func TestLogin_TOTPRequired(t *testing.T) {
    // Setup: create user with confirmed TOTP
    // POST /api/v1/sessions → expect 200 {totp_required: true, pending_token: "..."}
    // No cookie should be set
}

func TestLogin_TOTPSecondStep_ValidCode(t *testing.T) {
    // POST /api/v1/sessions with pending_token + totp_code → 200 + session cookie
}

func TestLogin_TOTPSecondStep_InvalidCode(t *testing.T) {
    // POST /api/v1/sessions with pending_token + wrong code → 401 totp_invalid
}

func TestLogin_TOTPSecondStep_RecoveryCode(t *testing.T) {
    // POST /api/v1/sessions with pending_token + recovery_code → 200; code marked consumed
}

func TestListSessions(t *testing.T) {
    // GET /api/v1/sessions authenticated → returns list; current: true on active session
}

func TestRevokeSession(t *testing.T) {
    // DELETE /api/v1/sessions/{id}: own non-current → 204; current → 400; other user's → 404
}

func TestRevokeAllOtherSessions(t *testing.T) {
    // DELETE /api/v1/sessions: deletes others; current survives
}
```

Run: `go test ./internal/api/... -run 'TestLogin_TOTP|TestListSessions|TestRevokeSession' -v`
Expected: FAIL (handlers not yet extended).

- [ ] **Step 2: Extend `loginHandler` for TOTP**

The login handler already validates password. After successful validation, check `db.GetTOTPSecret(ctx, dep.d, u.ID)` — if a confirmed secret exists, instead of minting a full session:
1. Call `db.DeleteExpiredPendingLogins` (cleanup).
2. Mint a pending token via `auth.MintPendingToken()`.
3. Insert into `pending_logins` with 5-minute TTL.
4. Return `{"totp_required": true, "pending_token": "<value>"}` — no cookie.

For the second step, detect the request shape by presence of `pending_token` field:
1. Look up the pending login by `sha256(pending_token)`.
2. Validate not expired; delete it regardless of outcome.
3. Look up the user; decrypt TOTP secret; call `auth.VerifyTOTP` or `auth.Verify` on recovery code hash.
4. On success, mint full session and set cookie.

- [ ] **Step 3: Add session list + revoke handlers**

```go
// listSessionsHandler returns GET /api/v1/sessions.
func listSessionsHandler(dep authDeps) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        u, _ := userFromContext(r.Context())
        s, _ := sessionFromContext(r.Context())
        sessions, err := db.ListSessionsByUserID(r.Context(), dep.d, u.ID)
        if err != nil {
            writeError(w, http.StatusInternalServerError, ErrCodeInternal, err.Error())
            return
        }
        // Map to DTO: include current: true where session.ID == s.ID
        // Spec says the endpoint returns [...] directly, not {"data": [...]}.
        writeJSON(w, http.StatusOK, toSessionDTOs(sessions, s.ID))
    })
}

// revokeSessionHandler handles DELETE /api/v1/sessions/{id}.
func revokeSessionHandler(dep authDeps) http.Handler { ... }

// revokeAllOtherSessionsHandler handles DELETE /api/v1/sessions.
func revokeAllOtherSessionsHandler(dep authDeps) http.Handler { ... }
```

- [ ] **Step 4: Extend `getSessionCurrentHandler` to return `has_totp` and `passkey_count`**

The response struct needs two new fields:

```go
type sessionCurrentResponse struct {
    User      userCurrentDTO `json:"user"`
    CSRFToken string         `json:"csrf_token"`
}

type userCurrentDTO struct {
    ID           int64  `json:"id"`
    Username     string `json:"username"`
    Role         string `json:"role"`
    HasTOTP      bool   `json:"has_totp"`
    PasskeyCount int    `json:"passkey_count"`
}
```

Handler calls `db.GetUserTOTPStatus` and `db.GetUserPasskeyCount` to populate these.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/api/... -run 'TestLogin_TOTP|TestListSessions|TestRevokeSession|TestRevokeAll' -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/auth.go internal/api/auth_test.go
git commit -m "M7: TOTP second step in login; session list + revoke endpoints"
```

---

### Task D3: TOTP enrolment endpoints

**Files:**
- Create: `internal/api/totp.go`, `internal/api/totp_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/api/totp_test.go` with table-driven tests covering:
- `POST /api/v1/me/totp` unauthenticated → 401; authenticated no secret → 200 + `secret_uri` + `secret`; already confirmed → 409 `totp_already_enrolled`
- `POST /api/v1/me/totp/confirm` valid code → 200 + 8 `recovery_codes`; invalid code → 401 `totp_invalid`; no pending (unconfirmed) secret → 404 `totp_not_enrolled` (reuse the same code as the DELETE endpoint — both mean "no active TOTP enrolment to act on")
- `DELETE /api/v1/me/totp` valid code → 204; invalid → 401; not enrolled → 409 `totp_not_enrolled`
- `POST /api/v1/me/totp/recovery-codes` valid code → 200 + 8 new codes; old codes gone

Use `auth.TestParams` (low-cost params) for any recovery code hashing in tests:

```go
var testHashParams = auth.Params{Time: 1, Memory: 64, Threads: 1, SaltLen: 16, KeyLen: 32}
```

Run: `go test ./internal/api/... -run TestTOTP -v`
Expected: FAIL (compile error).

- [ ] **Step 2: Implement TOTP server-key helper**

Add `internal/api/totp.go`:

```go
package api

import (
    "context"
    "crypto/rand"
    "database/sql"
    "fmt"

    "github.com/bcrisp4/tap/internal/db"
)

const totpKeyConfigKey = "totp_encryption_key"

// loadOrCreateTOTPKey loads the AES-256 server key for TOTP secret encryption
// from the configuration table, generating it on first call.
// Uses db.SetConfigIfAbsent so concurrent first-calls converge on one key.
func loadOrCreateTOTPKey(d *sql.DB) ([]byte, error) {
    // Generate a candidate key (discarded if one already exists)
    candidate := make([]byte, 32)
    if _, err := rand.Read(candidate); err != nil {
        return nil, fmt.Errorf("generate totp key candidate: %w", err)
    }
    // SetConfigIfAbsent inserts if absent, returns existing value either way.
    got, err := db.SetConfigIfAbsent(context.Background(), d, totpKeyConfigKey, candidate)
    if err != nil {
        return nil, fmt.Errorf("load or create totp key: %w", err)
    }
    // got is exactly 32 bytes (either the just-inserted candidate or a pre-existing key)
    if len(got) != 32 {
        return nil, fmt.Errorf("totp key has unexpected length %d", len(got))
    }
    return got, nil
}
```

`db.GetConfig` signature: `(ctx, d, key) ([]byte, bool, error)`. `db.SetConfigIfAbsent` signature: `(ctx, d, key string, value []byte) ([]byte, error)` — inserts the row if absent, returns the winning value. Both are in `internal/db/config.go` (landed in M3).

- [ ] **Step 3: Implement TOTP handlers**

Implement each of the four handlers following the spec. Key points:
- `beginTOTPEnrolmentHandler` calls `auth.GenerateTOTPSecret()`, encrypts with `loadOrCreateTOTPKey`, inserts with `confirmed=false` via `db.InsertTOTPSecret`.
- `confirmTOTPEnrolmentHandler` decrypts stored secret, calls `auth.VerifyTOTP`, on success wraps `db.ConfirmTOTPSecret` + `db.InsertRecoveryCodes` in a single `db.BeginTx` transaction so both succeed or both roll back atomically. Generate and hash the 8 codes before opening the transaction; pass the hashes in.
- `deleteTOTPHandler` accepts either `code` (TOTP) or `recovery_code`; validates, then wraps `db.DeleteTOTPSecret` + `db.DeleteRecoveryCodes` in a single transaction — a crash between the two would leave the user in an inconsistent state.
- `regenerateRecoveryCodesHandler` validates TOTP code, deletes old codes, inserts new.

Inject `hashParams auth.Params` into handler factories (same pattern as `passwordChangeHandler`) so tests can pass low-cost params.

- [ ] **Step 4: Run tests**

```bash
go test ./internal/api/... -run TestTOTP -race -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/totp.go internal/api/totp_test.go
git commit -m "M7: TOTP enrolment, confirm, disable, recovery-code-regen endpoints"
```

---

### Task D4: Passkey endpoints

**Files:**
- Create: `internal/api/passkeys.go`, `internal/api/passkeys_test.go`

- [ ] **Step 1: Invoke context7 for current WebAuthn API**

Before writing any code, call:
```
mcp__plugin_context7_context7__query-docs
  libraryId: /go-webauthn/webauthn
  query: BeginRegistration FinishRegistration BeginDiscoverableLogin FinishDiscoverableLogin webauthn.User interface credentials storage
```

Record the exact signatures for `(*WebAuthn).BeginRegistration(user User, opts ...RegistrationOption)`, `FinishRegistration(user User, session SessionData, response *protocol.ParsedCredentialCreationData)`, `BeginDiscoverableLogin(opts ...LoginOption)`, and `FinishDiscoverableLogin(session SessionData, handler DiscoverableUserHandler)`. The library wraps credential creation/assertion responses in `protocol.ParsedCredentialCreationData` and `protocol.ParsedCredentialRequestData` respectively.

- [ ] **Step 2: Define a `webAuthnUser` adapter type**

The `go-webauthn/webauthn` library requires a `webauthn.User` interface. Define a local adapter in `internal/api/passkeys.go`:

```go
type webAuthnUser struct {
    id          []byte // user.ID as little-endian int64
    name        string
    displayName string
    credentials []webauthn.Credential
}

func (u webAuthnUser) WebAuthnID() []byte                         { return u.id }
func (u webAuthnUser) WebAuthnName() string                       { return u.name }
func (u webAuthnUser) WebAuthnDisplayName() string                { return u.displayName }
func (u webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }
```

Map `db.Passkey` → `webauthn.Credential` before passing to the library.

- [ ] **Step 3: Write failing tests**

Create `internal/api/passkeys_test.go`. Because WebAuthn ceremonies require a browser, test at the handler boundary with a mock: inject a `*webauthn.WebAuthn` configured for `rpID="localhost"` and `origin="http://localhost"`. Use `httptest.Server`.

Test cases:
- Registration begin → 200 + non-empty `publicKey` JSON; challenge stored on session.
- Registration finish with invalid attestation body → 400.
- `GET /api/v1/me/passkeys` → list without `credential_id`/`public_key` fields.
- `DELETE /api/v1/me/passkeys/{id}` own → 204; other user's → 404.
- Passkey login begin → anonymous session created; challenge stored.
- Passkey login finish with invalid session_id → 401.

Run: `go test ./internal/api/... -run TestPasskey -v`
Expected: FAIL (compile error).

- [ ] **Step 4: Implement passkey handlers**

Key implementation notes for `finishPasskeyLoginHandler`:
- Look up session by integer `session_id` from request body — **not** via `requireSession` cookie middleware (this endpoint is public).
- Use `webauthn.FinishDiscoverableLogin` with a handler that maps `credentialID` → `db.GetPasskeyByCredentialID` → builds `webAuthnUser`.
- On success: update `sign_counter`, upgrade the anonymous session (set `user_id`, mint new `token_hash` and `csrf_token`, set cookie), return login response.
- On failure: delete the anonymous session, return 401.

- [ ] **Step 5: Run tests**

```bash
go test ./internal/api/... -run TestPasskey -race -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/passkeys.go internal/api/passkeys_test.go
git commit -m "M7: passkey registration + login endpoints"
```

---

### Task D5: Admin endpoints

**Files:**
- Create: `internal/api/admin.go`, `internal/api/admin_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/api/admin_test.go` covering all six endpoints per spec. Key cases:
- Non-admin user on any admin route → 403 `admin_required`.
- `POST /api/v1/admin/users` valid → 201; duplicate → 409; short password → 400 `password_too_short`.
- `POST /api/v1/admin/users/{id}/password-reset` → 200 + `temporary_password`; user's sessions deleted.
- `DELETE /api/v1/admin/users/{id}` cascade: user's subscriptions gone; self-delete → 400 `cannot_delete_self`.

Run: `go test ./internal/api/... -run TestAdmin -v`
Expected: FAIL.

- [ ] **Step 2: Implement `internal/api/admin.go`**

```go
package api

// listUsersHandler handles GET /api/v1/admin/users.
// deleteUserHandler handles DELETE /api/v1/admin/users/{id}.
//   - Reads path value: id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
//   - Checks u.ID != targetID (cannot delete self → 400 cannot_delete_self)
//   - Calls db.DeleteUser which cascades via FK
// createUserHandler handles POST /api/v1/admin/users.
// patchUserHandler handles PATCH /api/v1/admin/users/{id}.
// resetUserPasswordHandler handles POST /api/v1/admin/users/{id}/password-reset.
//   - Generates 16-char random alphanumeric temp password from crypto/rand
//   - Hashes with auth.Hash(hashParams)
//   - Updates password_hash
//   - Calls db.DeleteSessionsByUserID
//   - Returns {"temporary_password": "..."} — shown once
// disableUserTOTPHandler handles POST /api/v1/admin/users/{id}/disable-totp.
//   - Wraps db.DeleteTOTPSecret + db.DeleteRecoveryCodes in a single transaction
//     (same pattern as deleteTOTPHandler — both deletes must succeed or both roll back)
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/api/... -run TestAdmin -race -v
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/api/admin.go internal/api/admin_test.go
git commit -m "M7: admin user-management endpoints"
```

---

### Task D6: Update existing subscription + entry handlers with user isolation; wire all routes in `api.go`

**Files:**
- Modify: `internal/api/subscriptions.go`, `internal/api/subscriptions_test.go`
- Modify: `internal/api/entries.go`, `internal/api/entries_test.go`
- Modify: `internal/api/api.go`

- [ ] **Step 1: Write failing cross-user isolation tests (RED)**

Add to `internal/api/subscriptions_test.go`:

```go
func TestSubscriptions_CrossUserIsolation(t *testing.T) {
    // Setup: two users, userA creates a subscription
    // userB GET /api/v1/subscriptions → 200 with empty list
    // userB DELETE /api/v1/subscriptions/{userA_sub_id} → 404
}
```

Add equivalent `TestEntries_CrossUserIsolation` to `internal/api/entries_test.go`.

Run: `go test ./internal/api/... -run 'CrossUserIsolation' -v`
Expected: FAIL (handlers don't yet pass userID to db calls, so userB sees userA's data).

- [ ] **Step 2: Update subscription + entry handlers (GREEN)**

In each handler that calls a db function, extract `u, _ := userFromContext(r.Context())` and pass `u.ID` as the `userID` parameter. Failure to do so will cause cross-user data leaks — verify no db call omits the user ID.

Run: `go test ./internal/api/... -run 'CrossUserIsolation' -v`
Expected: PASS.

- [ ] **Step 3: Update `MuxOpts` and `NewMux` in `internal/api/api.go`**

Add `WebAuthnInstance *webauthn.WebAuthn` to `MuxOpts`. Register new routes:

```go
// TOTP
m.Handle("POST /api/v1/me/totp",                  authedCSRF(beginTOTPEnrolmentHandler(deps)))
m.Handle("POST /api/v1/me/totp/confirm",           authedCSRF(confirmTOTPEnrolmentHandler(deps)))
m.Handle("DELETE /api/v1/me/totp",                 authedCSRF(deleteTOTPHandler(deps)))
m.Handle("POST /api/v1/me/totp/recovery-codes",    authedCSRF(regenerateRecoveryCodesHandler(deps)))

// Passkeys
m.Handle("POST /api/v1/me/passkeys/registration/begin",  authedCSRF(beginPasskeyRegistrationHandler(deps)))
m.Handle("POST /api/v1/me/passkeys/registration/finish", authedCSRF(finishPasskeyRegistrationHandler(deps)))
m.Handle("GET /api/v1/me/passkeys",                      authed(listPasskeysHandler(deps)))
m.Handle("DELETE /api/v1/me/passkeys/{id}",              authedCSRF(deletePasskeyHandler(deps)))

// Passkey login (public)
m.Handle("POST /api/v1/passkey-sessions/begin",  http.HandlerFunc(beginPasskeyLoginHandler(deps)))
m.Handle("POST /api/v1/passkey-sessions/finish", http.HandlerFunc(finishPasskeyLoginHandler(deps)))

// Sessions
m.Handle("GET /api/v1/sessions",         authed(listSessionsHandler(deps)))
m.Handle("DELETE /api/v1/sessions",      authedCSRF(revokeAllOtherSessionsHandler(deps)))
m.Handle("DELETE /api/v1/sessions/{id}", authedCSRF(revokeSessionHandler(deps)))

// Admin
adminAuthed := chain(authed, requireAdmin())
adminAuthedCSRF := chain(authed, requireAdmin(), requireCSRF())
m.Handle("GET /api/v1/admin/users",                              adminAuthed(listUsersHandler(deps)))
m.Handle("POST /api/v1/admin/users",                             adminAuthedCSRF(createUserHandler(deps, opts.HashParams)))
m.Handle("PATCH /api/v1/admin/users/{id}",                       adminAuthedCSRF(patchUserHandler(deps)))
m.Handle("POST /api/v1/admin/users/{id}/password-reset",         adminAuthedCSRF(resetUserPasswordHandler(deps, opts.HashParams)))
m.Handle("POST /api/v1/admin/users/{id}/disable-totp",           adminAuthedCSRF(disableUserTOTPHandler(deps)))
m.Handle("DELETE /api/v1/admin/users/{id}",                      adminAuthedCSRF(deleteUserHandler(deps)))
```

- [ ] **Step 4: Run full Go test suite**

```bash
make test
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/api/subscriptions.go internal/api/subscriptions_test.go \
        internal/api/entries.go internal/api/entries_test.go \
        internal/api/api.go
git commit -m "M7: wire all new routes; user isolation in subscriptions + entries handlers"
```

---

## Phase E — CLI + config

### Task E1: `tap admin disable-totp` subcommand + WebAuthn config flags

**Files:**
- Modify: `cmd/tap/admin.go`, `cmd/tap/admin_test.go`
- Modify: `cmd/tap/main.go`

- [ ] **Step 1: Write failing test for `disable-totp`**

Add to `cmd/tap/admin_test.go`:

```go
func TestAdminDisableTOTP_HappyPath(t *testing.T) {
    // Setup: create DB, insert user with TOTP
    // Run: tap admin disable-totp <username> --data <dir>
    // Assert: totp_secrets row gone; recovery_codes gone; exit 0
}

func TestAdminDisableTOTP_MissingUser(t *testing.T) {
    // Run with non-existent username → exit 2
}
```

Run: `go test ./cmd/tap/... -run TestAdminDisableTOTP -v`
Expected: FAIL.

- [ ] **Step 2: Add `runAdminDisableTOTP` to `cmd/tap/admin.go`**

Follow the same pattern as `runAdminPasswd`:
- `GetUserByUsername`; missing → `fmt.Fprintf(os.Stderr, "user %q not found\n", username); os.Exit(2)`.
- Open DB read-write; run migrations.
- Wrap `db.DeleteTOTPSecret` + `db.DeleteRecoveryCodes` in a single transaction (both must succeed or both roll back).
- Commit; `fmt.Printf("2FA disabled for %q\n", username); os.Exit(0)`.

Add `disable-totp` to the `runAdmin` dispatcher switch.

- [ ] **Step 3: Add WebAuthn config flags to `cmd/tap/main.go`**

```go
webauthnRPID   := flag.String("webauthn-rp-id",   "", "WebAuthn relying party ID (default: derived from --addr)")
webauthnOrigin := flag.String("webauthn-origin",  "", "WebAuthn origin URL (default: derived from --addr + --cookie-secure)")
```

After existing flag parsing, derive defaults if empty:

```go
if *webauthnRPID == "" {
    host, _, _ := net.SplitHostPort(*addr)
    if host == "" { host = "localhost" }
    *webauthnRPID = host
}
if *webauthnOrigin == "" {
    // cookieSecure is already resolved earlier in runServer via
    // api.ResolveCookieSecure(cookieSecureEnum, *addr) — reuse it directly.
    scheme := "http"
    if cookieSecure { scheme = "https" }
    *webauthnOrigin = scheme + "://" + *webauthnRPID
}
```

Construct `webauthn.WebAuthn`:

```go
wa, err := webauthn.New(&webauthn.Config{
    RPDisplayName: "Tap",
    RPID:          *webauthnRPID,
    RPOrigins:     []string{*webauthnOrigin},
})
if err != nil {
    slog.Error("webauthn config", "err", err)
    os.Exit(1)
}
```

Pass `wa` into `api.MuxOpts{WebAuthnInstance: wa, ...}`.

- [ ] **Step 4: Run tests**

```bash
go test ./cmd/tap/... -run TestAdminDisableTOTP -v
make test
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/tap/admin.go cmd/tap/admin_test.go cmd/tap/main.go
git commit -m "M7: tap admin disable-totp; WebAuthn config flags"
```

---

## Phase F — SPA

### Task F1: Update types, auth store, and api client

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/auth.ts`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/__tests__/auth.test.ts`
- Modify: `web/src/lib/__tests__/api.test.ts`

- [ ] **Step 1: Write failing tests for new auth store methods (RED)**

Add to `web/src/lib/__tests__/auth.test.ts`:

```typescript
test('bootstrap populates has_totp and passkey_count from /sessions/current', async () => {
  // mock GET /api/v1/sessions/current to return user with has_totp: true, passkey_count: 2
  await auth.bootstrap();
  expect(get(auth).user?.has_totp).toBe(true);
  expect(get(auth).user?.passkey_count).toBe(2);
});

test('beginPasskeyLogin calls POST /passkey-sessions/begin', async () => {
  // mock the endpoint; assert it was called
  const result = await auth.beginPasskeyLogin();
  expect(result).toHaveProperty('sessionId');
  expect(result).toHaveProperty('options');
});
```

Run: `pnpm --dir web test -- src/lib/__tests__/auth.test.ts`
Expected: FAIL (methods/fields not yet defined).

- [ ] **Step 2: Update `web/src/lib/types.ts`**

Add:

```typescript
export type Session = {
  id: number;
  created_at: number;
  last_seen_at: number;
  idle_expires_at: number;
  user_agent: string;
  address: string;
  current: boolean;
};

export type TOTPEnrolmentBegin = {
  secret_uri: string;
  secret: string;
};

export type PasskeyListItem = {
  id: number;
  label: string;
  created_at: number;
};

export type AdminUser = {
  id: number;
  username: string;
  role: 'admin' | 'user';
  created_at: number;
  disabled_at: number | null;
  has_totp: boolean;
  passkey_count: number;
};
```

Extend `User` type:

```typescript
export type User = {
  id: number;
  username: string;
  role: 'admin' | 'user';
  has_totp: boolean;
  passkey_count: number;
};
```

- [ ] **Step 3: Update `web/src/lib/auth.ts` (GREEN)**

```typescript
type State = {
  user: User | null;
  csrfToken: string | null;
  bootstrapped: boolean;
};
```

Add `beginPasskeyLogin` and `finishPasskeyLogin` methods. `bootstrap()` now populates `has_totp` and `passkey_count` from the extended `/sessions/current` response.

- [ ] **Step 4: Update `web/src/lib/api.ts`**

Add all the new API call functions as described in the spec's SPA section. Each function follows the existing pattern: `request()` with the right method, path, and body.

- [ ] **Step 5: Run tests (confirm GREEN)**

```bash
pnpm --dir web test -- src/lib/__tests__/auth.test.ts src/lib/__tests__/api.test.ts
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/types.ts web/src/lib/auth.ts web/src/lib/api.ts \
        web/src/lib/__tests__/auth.test.ts web/src/lib/__tests__/api.test.ts
git commit -m "M7: SPA types, auth store, api client extensions"
```

---

### Task F2: Extend Login.svelte for passkey + TOTP second step

**Files:**
- Modify: `web/src/views/Login.svelte`
- Modify: `web/src/views/__tests__/Login.test.ts`

- [ ] **Step 1: Write failing tests**

Add to `web/src/views/__tests__/Login.test.ts`:

```typescript
test('renders passkey button', () => {
  render(Login);
  expect(screen.getByRole('button', { name: /sign in with a passkey/i })).toBeInTheDocument();
});

test('TOTP step renders after totp_required response', async () => {
  // mock auth.login to return totp_required: true
  render(Login);
  await fireEvent.submit(screen.getByRole('form'));
  expect(screen.getByLabelText(/authentication code/i)).toBeInTheDocument();
});

test('recovery code toggle switches input', async () => {
  // after TOTP step visible, click "use recovery code"
  // expect different placeholder / label
});
```

Run: `pnpm --dir web test -- src/views/__tests__/Login.test.ts`
Expected: FAIL.

- [ ] **Step 2: Extend `Login.svelte`**

```svelte
<script lang="ts">
  import { auth } from '../lib/auth';

  let step = $state<'credentials' | 'totp'>('credentials');
  let pendingToken = $state('');
  let useRecoveryCode = $state(false);
  let error = $state('');

  async function handleSubmit(e: SubmitEvent) { /* ... */ }
  async function handlePasskeyLogin() {
    const { sessionId, options } = await auth.beginPasskeyLogin();
    const credential = await navigator.credentials.get({ publicKey: options });
    await auth.finishPasskeyLogin(sessionId, credential);
  }
</script>

{#if step === 'credentials'}
  <form onsubmit={handleSubmit}>
    <!-- username + password fields -->
    <button type="submit">Sign in</button>
    <button type="button" onclick={handlePasskeyLogin}>Sign in with a passkey</button>
  </form>
{:else}
  <!-- TOTP second step -->
  {#if useRecoveryCode}
    <input type="text" aria-label="Recovery code" />
  {:else}
    <input type="text" aria-label="Authentication code" maxlength="6" />
    <button type="button" onclick={() => useRecoveryCode = true}>Use a recovery code instead</button>
  {/if}
{/if}
```

- [ ] **Step 3: Run tests**

```bash
pnpm --dir web test -- src/views/__tests__/Login.test.ts
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/Login.svelte web/src/views/__tests__/Login.test.ts
git commit -m "M7: Login — passkey button and TOTP second step"
```

---

### Task F3: Settings Security view

**Files:**
- Create: `web/src/views/settings/Security.svelte`
- Create: `web/src/views/__tests__/Security.test.ts`

- [ ] **Step 1: Write failing tests**

Create `web/src/views/__tests__/Security.test.ts`:

```typescript
test('session list shows current session marked', async () => { /* ... */ });
test('revoke button calls revokeSession', async () => { /* ... */ });
test('log out everywhere calls revokeAllOtherSessions', async () => { /* ... */ });
test('TOTP not enrolled: setup button visible', () => { /* ... */ });
test('TOTP enrolled: disable and regenerate visible', () => { /* ... */ });
test('passkey list renders; Add button visible', () => { /* ... */ });
```

Run: `pnpm --dir web test -- src/views/__tests__/Security.test.ts`
Expected: FAIL.

- [ ] **Step 2: Implement `Security.svelte`**

Three logical sub-sections as Svelte 5 components or inline sections:
1. **Sessions** — `onMount` fetches `api.listSessions()`; renders table with user_agent, address, last_seen relative time; current row has no revoke button; "Log out everywhere" button top-right.
2. **TOTP** — reads `$auth.totpEnabled`; shows setup or disable/regenerate accordingly; QR code display uses an `<img src={secretUri}>` rendered via `auth.beginTOTPEnrolment()`; confirm step is a 6-digit code input.
3. **Passkeys** — `onMount` fetches `api.listPasskeys()`; renders list + remove per passkey; "Add a passkey" calls `api.beginPasskeyRegistration()` → `navigator.credentials.create` → `api.finishPasskeyRegistration(attestation, label)`.

- [ ] **Step 3: Run tests**

```bash
pnpm --dir web test -- src/views/__tests__/Security.test.ts
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/views/settings/Security.svelte web/src/views/__tests__/Security.test.ts
git commit -m "M7: Settings → Security view (session list, TOTP, passkeys)"
```

---

### Task F4: Admin view

**Files:**
- Create: `web/src/views/Admin.svelte`
- Create: `web/src/views/__tests__/Admin.test.ts`

- [ ] **Step 1: Write failing tests**

Create `web/src/views/__tests__/Admin.test.ts`:

```typescript
test('non-admin: Admin renders nothing or redirects', () => { /* ... */ });
test('admin: user list renders', async () => { /* ... */ });
test('create user form submits', async () => { /* ... */ });
test('reset password shows temporary password', async () => { /* ... */ });
test('delete user shows confirmation', async () => { /* ... */ });
```

Run: `pnpm --dir web test -- src/views/__tests__/Admin.test.ts`
Expected: FAIL.

- [ ] **Step 2: Implement `Admin.svelte`**

```svelte
<script lang="ts">
  import { auth } from '../lib/auth';
  import { listUsers, createUser, resetUserPassword,
           disableUserTOTP, patchUser, deleteUser } from '../lib/api';

  // Guard: only render if admin
  const isAdmin = $derived($auth.user?.role === 'admin');

  let users = $state<AdminUser[]>([]);
  let tempPassword = $state('');

  // onMount: fetch users if admin
</script>

{#if isAdmin}
  <!-- user list table, create form, per-row actions -->
{/if}
```

- [ ] **Step 3: Update router and App.svelte**

In `web/src/lib/router.ts` add `/settings` and `/admin` routes. In `App.svelte`:
- Render `<Security />` for `/settings` route.
- Render `<Admin />` for `/admin` route with admin guard redirect.
- Add nav links.

- [ ] **Step 4: Run tests**

```bash
pnpm --dir web test -- src/views/__tests__/Admin.test.ts
pnpm --dir web run check
```

Expected: all pass; no TypeScript errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/Admin.svelte web/src/views/__tests__/Admin.test.ts \
        web/src/lib/router.ts web/src/App.svelte
git commit -m "M7: Admin view + /settings and /admin routes"
```

---

## Phase G — Final verification + docs

### Task G1: Full test suite + build verification

**Files:** none (verification only)

- [ ] **Step 1: Run the full test suite with race detection**

```bash
make test
```

Expected: `go test ./... -race` passes, all SPA tests pass.

- [ ] **Step 2: Verify the binary builds clean**

```bash
make build
```

Expected: `bin/tap` produced with no errors.

- [ ] **Step 3: Smoke test — fresh DB boot**

```bash
TAP_ADMIN_USERNAME=ben TAP_ADMIN_PASSWORD=secret123 ./bin/tap --data /tmp/tap-m7-test &
sleep 2
curl -s -c /tmp/cookies.txt -X POST http://localhost:8080/api/v1/sessions \
  -H 'Content-Type: application/json' \
  -d '{"username":"ben","password":"secret123"}' | jq .
```

Expected: `{"user": {"id": 1, "username": "ben", "role": "admin", "has_totp": false, "passkey_count": 0}, "csrf_token": "..."}`

```bash
kill %1
rm -rf /tmp/tap-m7-test
```

- [ ] **Step 4: Commit**

```bash
# No code changes — just verify. No commit needed.
```

---

### Task G2: README + CLAUDE.md updates

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add M7 auth paragraph to README**

After the existing M6 Authentication paragraph, add:

> **Authentication (M7).** TOTP (RFC 6238) is available as an optional second factor; users enrol from Settings → Security. Recovery codes (8 single-use codes, shown once at enrolment) allow disabling TOTP without admin involvement. Passkeys (WebAuthn discoverable credentials) are an alternative login method; a passkey login does not additionally prompt for TOTP. Active sessions are listed in Settings → Security with device and IP information; any session can be revoked individually or all at once. Admins can create users, reset passwords, and disable 2FA from the SPA user-management view (or from the CLI: `tap admin disable-totp <username>`). Each user's subscriptions and entries are isolated — no user (including admins) can see another user's feeds. TOTP secrets are AES-256-GCM encrypted at rest using a server key stored in the `configuration` table. The WebAuthn relying party ID and origin must be configured explicitly for non-loopback deployments (`--webauthn-rp-id`, `--webauthn-origin`).

Add upgrade note: "M7 is breaking. The schema migration adds `user_id NOT NULL` to subscriptions and entries. Start with a fresh database."

- [ ] **Step 2: Update CLAUDE.md**

Change `M6 in progress` → `M7 in progress` in the status line and update the spec path reference.

- [ ] **Step 3: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "M7: README auth paragraph; CLAUDE.md status update"
```

---

### Task G3: Final `make test` + commit

- [ ] **Step 1: Run complete test suite one last time**

```bash
make test
```

Expected: all pass.

- [ ] **Step 2: Run `/simplify`**

Invoke the `simplify` skill to review all changed code for reuse, quality, and efficiency. Fix any issues found before declaring done.

- [ ] **Step 3: Confirm no outstanding uncommitted changes**

```bash
git status
```

Expected: clean working tree.
