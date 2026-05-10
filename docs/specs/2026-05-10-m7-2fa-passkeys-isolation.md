# M7 — 2FA + passkeys + per-user data isolation

**Status:** Draft, awaiting review.

## Context

Tap is the self-hosted feed reader described in [`../concept.md`](../concept.md). M7 is the seventh of twelve milestones — see [`../roadmap.md`](../roadmap.md). M6's auth foundations ([`2026-05-10-m6-auth-foundations.md`](2026-05-10-m6-auth-foundations.md)) have shipped: password login, sessions with idle and absolute expiry, CSRF discipline, and admin bootstrap via CLI and env-var. M6 explicitly deferred five surfaces to M7: TOTP + recovery codes, WebAuthn / passkeys, session listing + per-session revocation, admin reset paths in the SPA, admin user-creation from the SPA, and per-user data isolation on subscriptions and entries.

M7 closes all five. It is the largest milestone to date — three interlocking concerns:

1. **Second factors.** TOTP enrolment and verification, recovery codes, WebAuthn passkey registration and login. Passkey login is inherently multi-factor (authenticator user-verification covers both possession and presence); it does not additionally prompt for TOTP.
2. **Session management surface.** Session listing with user-agent and address columns (M6 deferred these columns pending this UI), per-session revocation, "log out everywhere".
3. **Per-user data isolation.** `user_id` columns on `subscriptions` and `entries`, strict per-user query filters across the full API surface, and the admin SPA paths that make multiple users meaningful (create user, reset password, disable 2FA for another user).

Tap is pre-production. The M7 schema migration does not back-fill existing data — operators start fresh. The migration adds `user_id` as `NOT NULL` with a foreign key constraint; a fresh install has no orphaned rows to worry about.

## Goal

After M7, a user can open Settings → Security and:
- enrol TOTP from an authenticator app, confirm with a 6-digit code, and receive 8 recovery codes shown once;
- register a passkey by tapping their device's authenticator;
- list their active sessions with device and IP information and revoke any of them.

The login screen gains a "Sign in with a passkey" path. If a user has TOTP enabled, the login flow adds a second step for the 6-digit code (or a recovery code).

An admin can open the User Management view and create new users, reset passwords, and disable 2FA for any user.

Every subscription and entry is owned by the user who created it. A logged-in user (including an admin) sees only their own subscriptions and entries — there is no admin-override on feed visibility.

## In scope

### New dependency: `github.com/go-webauthn/webauthn`

Pure-Go FIDO2 / WebAuthn library. Depends on `fxamacker/cbor` and standard `crypto/*` packages. No CGO. Licensed Apache 2.0. Verified pure-Go before inclusion.

| Module | Purpose | License |
|---|---|---|
| `github.com/go-webauthn/webauthn` | WebAuthn registration + assertion ceremonies | Apache-2.0 |

TOTP is implemented using `crypto/hmac` + `crypto/sha1` from the standard library — no external TOTP dependency needed. The algorithm is RFC 6238 (TOTP) over RFC 4226 (HOTP): `HOTP(K, T)` where `T = floor(unix_time / 30)`, 6-digit output, SHA-1 HMAC. The `secret_uri` returned to the SPA is a standard `otpauth://totp/...` URI compatible with any authenticator app.

### Schema migrations

Two migrations shipped together as one logical change.

**`internal/db/migrations/0006_user_data_isolation.sql`:**

```sql
ALTER TABLE subscriptions ADD COLUMN user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE entries       ADD COLUMN user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE;

CREATE INDEX idx_subscriptions_user ON subscriptions(user_id);
CREATE INDEX idx_entries_user       ON entries(user_id, published_at DESC);
```

Since Tap is pre-production, no back-fill is performed. Operators start with a fresh database. The columns are `NOT NULL` with a foreign key constraint from day one.

**`internal/db/migrations/0007_2fa_passkeys_sessions_meta.sql`:**

```sql
-- sessions.user_id must become nullable to support anonymous WebAuthn challenge
-- sessions. SQLite cannot drop a NOT NULL constraint with ALTER TABLE, so we
-- recreate the table using the standard rename→create→copy→drop pattern.
-- The entire migration runs inside one transaction; it rolls back cleanly on failure.
ALTER TABLE sessions RENAME TO sessions_old;

CREATE TABLE sessions (
    id                  INTEGER PRIMARY KEY,
    user_id             INTEGER REFERENCES users(id) ON DELETE CASCADE,  -- NULL for anonymous challenge sessions
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

-- The three new columns (user_agent, address, webauthn_challenge) are included in
-- the table recreation above; no separate ALTER TABLE statements are needed.

-- Pending two-step login tokens (TOTP second step)
CREATE TABLE pending_logins (
    id          INTEGER PRIMARY KEY,
    token_hash  TEXT    NOT NULL UNIQUE,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  INTEGER NOT NULL,
    expires_at  INTEGER NOT NULL
);
CREATE INDEX idx_pending_logins_expires ON pending_logins(expires_at);

-- TOTP secrets (one active secret per user)
CREATE TABLE totp_secrets (
    id                INTEGER PRIMARY KEY,
    user_id           INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    secret_encrypted  BLOB    NOT NULL,  -- AES-GCM nonce+ciphertext; key from configuration table
    confirmed         INTEGER NOT NULL DEFAULT 0,
    created_at        INTEGER NOT NULL
);

-- Single-use recovery codes (8 issued per TOTP enrolment)
CREATE TABLE recovery_codes (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash    TEXT    NOT NULL,  -- argon2id, same DefaultParams as passwords
    consumed_at  INTEGER
);
CREATE INDEX idx_recovery_codes_user ON recovery_codes(user_id);

-- Passkeys (one row per registered credential)
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

`pending_logins` rows are short-lived (5-minute TTL) and consulted only during the TOTP second step. A background cleanup (on login handler entry) prunes expired rows — no separate ticker needed given the low volume.

### New package: `internal/auth/encrypt.go`

AES-GCM envelope for TOTP secrets. The server key is loaded from (or generated into) the `configuration` table on startup, alongside the existing proxy signing secret — same pattern, same table.

```go
// EncryptTOTPSecret encrypts a base32 TOTP secret using AES-256-GCM.
// Returns nonce+ciphertext concatenated.
func EncryptTOTPSecret(key []byte, secret string) ([]byte, error)

// DecryptTOTPSecret reverses EncryptTOTPSecret.
func DecryptTOTPSecret(key []byte, ciphertext []byte) (string, error)
```

The server key is 32 bytes of `crypto/rand`, stored as hex in `configuration` under key `totp_encryption_key`. Generated once at first use; stable across restarts. This is defence-in-depth against DB-read-only attacks — concept §8 explicitly accepts "co-tenant reads the DB" as out of scope, so plain TOTP storage would be defensible, but the `configuration` table already establishes the pattern for server-side secrets and the marginal cost is ~30 lines.

### `internal/auth` additions

**`internal/auth/totp.go`:**

```go
// GenerateTOTPSecret returns a base32-encoded 20-byte random secret.
func GenerateTOTPSecret() (string, error)

// TOTPSecretURI builds the otpauth://totp/... URI for QR code display.
// issuer is the app name ("Tap"), accountName is the username.
func TOTPSecretURI(secret, issuer, accountName string) string

// VerifyTOTP validates a 6-digit code against the secret.
// Accepts current window ±1 (one step of clock drift tolerance).
func VerifyTOTP(secret, code string) bool

// GenerateRecoveryCodes returns 8 cryptographically random 10-char
// alphanumeric codes (human-readable, uppercase, no ambiguous chars).
func GenerateRecoveryCodes() ([]string, error)
```

TOTP implementation uses `crypto/hmac` + `crypto/sha1` directly (RFC 6238 / RFC 4226). No external TOTP library. The ±1 window tolerance matches TOTP best practice and covers a full 30-second drift.

Recovery codes: 8 codes × 10 chars from `[A-Z2-9]` (no 0/O/1/I ambiguity). Hashed with `auth.Hash` at `DefaultParams` (same argon2id params as passwords — recovery codes are short-use but the hash should be consistent). Stored in `recovery_codes`; marked consumed on first use by setting `consumed_at`.

**`internal/auth/pending.go`:**

```go
// MintPendingToken returns (cookieValue, tokenHash, err).
// Same shape as MintSessionToken — 32 base64url bytes, sha256-hex stored.
func MintPendingToken() (value, hash string, err error)
```

### `db` layer additions

**`internal/db/totp.go`:**

```go
func InsertTOTPSecret(ctx, d, userID int64, encryptedSecret []byte) error
func GetTOTPSecret(ctx, d, userID int64) (TOTPSecret, error)  // sql.ErrNoRows if none
func ConfirmTOTPSecret(ctx, d, userID int64) error
func DeleteTOTPSecret(ctx, d, userID int64) error

func InsertRecoveryCodes(ctx, d, userID int64, hashes []string) error
func GetUnconsumedRecoveryCodes(ctx, d, userID int64) ([]RecoveryCode, error)
func ConsumeRecoveryCode(ctx, d, id int64) error  // sets consumed_at = now
func DeleteRecoveryCodes(ctx, d, userID int64) error
```

**`internal/db/passkeys.go`:**

```go
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

func InsertPasskey(ctx, d, p Passkey) (int64, error)
func GetPasskeysByUserID(ctx, d, userID int64) ([]Passkey, error)
func GetPasskeyByCredentialID(ctx, d, credentialID []byte) (Passkey, error)
func UpdatePasskeySignCounter(ctx, d, id, counter int64) error
func DeletePasskey(ctx, d, id int64) error
```

**`internal/db/pending_logins.go`:**

```go
func InsertPendingLogin(ctx, d, userID int64, tokenHash string, expiresAt int64) error
func GetPendingLoginByTokenHash(ctx, d, tokenHash string) (PendingLogin, error)
func DeletePendingLogin(ctx, d, id int64) error
func DeleteExpiredPendingLogins(ctx, d, now int64) error  // called on login handler entry
```

**`internal/db/sessions.go` additions:**

```go
// Additions to existing NewSession / Session structs:
//   UserAgent string
//   Address   string
//   WebAuthnChallenge []byte  // nil when not in a ceremony

func ListSessionsByUserID(ctx, d, userID int64) ([]Session, error)
func SetWebAuthnChallenge(ctx, d, sessionID int64, challenge []byte) error
func ClearWebAuthnChallenge(ctx, d, sessionID int64) error
```

**`internal/db/subscriptions.go` and `internal/db/entries.go` changes:**

All queries that list, get, or mutate subscriptions or entries gain a `userID int64` parameter and a `WHERE user_id = ?` (or `AND user_id = ?`) clause. `NewSubscription` gains `UserID int64`. `InsertSubscription` stores the caller's user ID. This is the complete enforcement surface — no query that touches subscriptions or entries escapes the filter.

**`internal/db/users.go` additions:**

```go
func ListUsers(ctx, d) ([]User, error)                // admin: list all users
func DeleteUser(ctx, d, id int64) error                // cascades sessions, subs, entries
func EnableUser(ctx, d, id int64) error                // clears disabled_at
func GetUserTOTPStatus(ctx, d, userID int64) (hasTOTP, confirmed bool, err error)
func GetUserPasskeyCount(ctx, d, userID int64) (int, error)
```

### Middleware changes

**`internal/api/middleware.go`** — `requireSession` already injects the user and session into context. Two additions:

- On login, the session row now captures `user_agent` (from `User-Agent` request header) and `address` (from `X-Forwarded-For` → `RemoteAddr`, first non-private IP wins — keeps the value honest for deployments behind a single reverse proxy without over-trusting arbitrary header chains).
- A new `requireAdmin` middleware reads the user from context and returns `403 forbidden` (new error code `admin_required`) if `role != "admin"`.

### API surface

All new endpoints under `/api/v1/`. DTOs follow the M6 convention: write-shapes and read-shapes are distinct Go types; secrets never appear on reads.

New stable error codes:

```go
const (
    ErrCodeTOTPRequired          = "totp_required"
    ErrCodeTOTPInvalid           = "totp_invalid"
    ErrCodeRecoveryCodeInvalid   = "recovery_code_invalid"
    ErrCodeTOTPNotEnrolled       = "totp_not_enrolled"
    ErrCodeTOTPAlreadyEnrolled   = "totp_already_enrolled"
    ErrCodePasskeyNotFound       = "passkey_not_found"
    ErrCodeCannotRevokeCurrentSession = "cannot_revoke_current_session"
    ErrCodeAdminRequired         = "admin_required"
)
```

#### Login flow change (`POST /api/v1/sessions`, existing endpoint)

If the authenticating user has a confirmed TOTP secret:
1. First POST (username + password) validates credentials. On success, instead of minting a session, it mints a pending token (32 base64url bytes, stored hashed in `pending_logins` with a 5-minute TTL) and returns `{"totp_required": true, "pending_token": "<value>"}` with `200 OK`.
2. Second POST body: `{"pending_token": "<value>", "totp_code": "123456"}` OR `{"pending_token": "<value>", "recovery_code": "ABCD123456"}`. Validates the pending token (not expired, not already used — delete on lookup), then validates the TOTP code or recovery code. On success, mints a full session (same shape as the non-TOTP login response). On failure: `401 totp_invalid` or `401 recovery_code_invalid`. Pending token is deleted regardless of outcome to prevent replay.

Passkey login does not go through `POST /api/v1/sessions` — it has its own endpoints (below) and never prompts for TOTP.

#### TOTP endpoints (`internal/api/totp.go`)

- **`POST /api/v1/me/totp`** — begin enrolment. Authenticated. CSRF required. Fails with `409 totp_already_enrolled` if a confirmed secret already exists.
  - Generates a new TOTP secret, encrypts it, stores with `confirmed = false` (replacing any prior unconfirmed attempt).
  - Returns `{"secret_uri": "otpauth://totp/...", "secret": "<base32>"}`. The `secret` field is for manual entry; `secret_uri` encodes it for QR display. Neither field is returned again after confirm.

- **`POST /api/v1/me/totp/confirm`** — confirm enrolment. Authenticated. CSRF required.
  - Body: `{"code": "123456"}`. Validates against the unconfirmed secret. On success, sets `confirmed = true`, generates 8 recovery codes, hashes and stores them.
  - Returns `{"recovery_codes": ["...", ...]}` — shown once, never again.
  - `totp_invalid` if the code doesn't match.

- **`DELETE /api/v1/me/totp`** — disable TOTP. Authenticated. CSRF required.
  - Body: `{"code": "123456"}` OR `{"recovery_code": "..."}`. Must validate to proceed.
  - Deletes `totp_secrets` row + all `recovery_codes` rows for the user.
  - `totp_not_enrolled` if no confirmed secret exists.

- **`POST /api/v1/me/totp/recovery-codes`** — regenerate recovery codes. Authenticated. CSRF required.
  - Body: `{"code": "123456"}`. Must validate.
  - Deletes existing codes, generates and stores 8 new ones.
  - Returns `{"recovery_codes": ["...", ...]}`.

#### Passkey endpoints (`internal/api/passkeys.go`)

Registration (requires active session — user must be logged in to register a passkey):

- **`POST /api/v1/me/passkeys/registration/begin`** — Authenticated. CSRF required.
  - Calls `webauthn.BeginRegistration(user)`. Stores the challenge on `sessions.webauthn_challenge`. Returns `CredentialCreation` JSON.

- **`POST /api/v1/me/passkeys/registration/finish`** — Authenticated. CSRF required.
  - Body: attestation response JSON + `{"label": "MacBook Touch ID"}`.
  - Reads challenge from session, calls `webauthn.FinishRegistration`. Clears `sessions.webauthn_challenge`. Inserts passkey row.
  - Returns `{"id": N, "label": "...", "created_at": N}`.

- **`GET /api/v1/me/passkeys`** — Authenticated. No CSRF (GET).
  - Returns `[{"id": N, "label": "...", "created_at": N}]`. Never returns `credential_id` or `public_key`.

- **`DELETE /api/v1/me/passkeys/{id}`** — Authenticated. CSRF required.
  - Deletes the passkey row. Returns 204. `passkey_not_found` if not found or belongs to another user.

Passkey login (public, no session required — new login path):

- **`POST /api/v1/passkey-sessions/begin`** — Public. No CSRF.
  - Creates a short-lived anonymous session solely to hold the challenge (same `sessions` table, `user_id` = 0 sentinel or stored as `NULL` — use `NULL` with a schema tweak: `user_id INTEGER REFERENCES users(id) ON DELETE CASCADE` — NULL until the ceremony completes).
  - Returns `{"session_id": N, "options": <CredentialRequestOptions JSON>}`.

- **`POST /api/v1/passkey-sessions/finish`** — Public. No CSRF.
  - Body: `{"session_id": N, "assertion": <AssertionResponse JSON>}`.
  - The session is looked up directly by the integer `session_id` from the request body, not via the `tap_session` cookie — this endpoint does not go through `requireSession` (the browser has no session cookie for the challenge session). Reads the challenge from the row, calls `webauthn.FinishAssertion`. On success, updates the passkey's `sign_counter`, upgrades the anonymous session to a full user session (sets `user_id`, mints cookie, returns same shape as password login). On failure, deletes the anonymous session and returns `401 invalid_credentials`.
  - No TOTP step — passkey login is inherently multi-factor per concept §7.3.

Schema tweak for passkey login: `sessions.user_id` becomes `INTEGER REFERENCES users(id) ON DELETE CASCADE` (nullable, without `NOT NULL`) to support anonymous challenge sessions. Migration 0007 handles this. The `requireSession` middleware continues to reject sessions where `user_id IS NULL`.

#### Session management (`internal/api/auth.go` additions)

- **`GET /api/v1/sessions`** — Authenticated. No CSRF (GET).
  - Returns all active sessions for the current user: `[{"id": N, "created_at": N, "last_seen_at": N, "idle_expires_at": N, "user_agent": "...", "address": "...", "current": bool}]`.
  - `current: true` on the session matching the request's session ID.

- **`DELETE /api/v1/sessions/{id}`** — Authenticated. CSRF required.
  - Revokes a specific session belonging to the current user. `cannot_revoke_current_session` if `id` matches the current session. `404` if not found or belongs to another user.

- **`DELETE /api/v1/sessions`** — Authenticated. CSRF required.
  - Deletes all sessions for the current user except the current one. Returns 204.

#### Admin endpoints (`internal/api/admin.go`)

All require `requireAdmin` middleware (role = admin). CSRF required on all state-changing methods.

- **`GET /api/v1/admin/users`** — List all users.
  - Returns `[{"id": N, "username": "...", "role": "...", "created_at": N, "disabled_at": N|null, "has_totp": bool, "passkey_count": N}]`.

- **`POST /api/v1/admin/users`** — Create a user.
  - Body: `{"username": "...", "password": "...", "role": "admin"|"user"}`.
  - Validates password length (≥8). Hashes with `auth.Hash(DefaultParams)`. Returns `201 Created` + user DTO. `ErrUserExists` → `409 user_already_exists`.

- **`PATCH /api/v1/admin/users/{id}`** — Update role or disabled status.
  - Body: `{"role": "admin"|"user"}` and/or `{"disabled": true|false}`.
  - `disabled: true` → sets `disabled_at = now`. `disabled: false` → clears `disabled_at`. Returns updated user DTO.

- **`POST /api/v1/admin/users/{id}/password-reset`** — Issue a one-time temporary password.
  - Generates a 16-char random alphanumeric temporary password (no complexity rules, just length).
  - Hashes it, updates `users.password_hash`. Deletes all sessions for that user.
  - Returns `{"temporary_password": "..."}` — shown once.

- **`POST /api/v1/admin/users/{id}/disable-totp`** — Remove another user's TOTP.
  - Deletes that user's `totp_secrets` row and all `recovery_codes` rows.
  - Returns 204.

- **`DELETE /api/v1/admin/users/{id}`** — Delete a user.
  - Cascades: sessions, subscriptions (and their entries via FK cascade), passkeys, totp_secrets, recovery_codes.
  - An admin may not delete themselves: `400 cannot_delete_self`.
  - Returns 204.

### WebAuthn configuration

The `go-webauthn/webauthn` library requires a `webauthn.Config` at construction:

```go
wc := &webauthn.Config{
    RPDisplayName: "Tap",
    RPID:          rpID,    // hostname of the Tap instance, e.g. "tap.example.com"
    RPOrigins:     []string{origin},  // e.g. "https://tap.example.com"
}
```

Two new configuration knobs:

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--webauthn-rp-id` | `TAP_WEBAUTHN_RP_ID` | derived from `--addr` host | Relying party ID. Must match the origin's hostname. |
| `--webauthn-origin` | `TAP_WEBAUTHN_ORIGIN` | derived from `--addr` + `--cookie-secure` | Full origin URL, e.g. `https://tap.example.com`. |

For local development (`--addr 127.0.0.1:8080`), defaults derive to `rpID = "localhost"`, `origin = "http://localhost:8080"`. This matches WebAuthn's localhost exception (HTTP allowed for localhost). Operators must set both knobs for any non-loopback deployment.

### SPA changes

All views are functional but unstyled. M8 is responsible for visual polish.

**`web/src/lib/auth.ts`** (extended):
- `State` gains `totpEnabled: boolean`, `passkeyCount: number`.
- `auth.bootstrap()` populates these from `GET /api/v1/sessions/current`. The M6 response shape for this endpoint is extended in M7 to include `{"user": {..., "has_totp": bool, "passkey_count": N}, "csrf_token": "..."}` — the server-side `getSessionCurrent` handler gains these two fields from the `db` helpers added in this milestone.
- New `auth.beginPasskeyLogin()` and `auth.finishPasskeyLogin(sessionId, assertion)`.

**`web/src/lib/api.ts`** (extended):
- TOTP: `beginTOTPEnrolment()`, `confirmTOTPEnrolment(code)`, `disableTOTP(code|recoveryCode)`, `regenerateRecoveryCodes(code)`.
- Passkeys: `beginPasskeyRegistration()`, `finishPasskeyRegistration(attestation, label)`, `listPasskeys()`, `deletePasskey(id)`.
- Sessions: `listSessions()`, `revokeSession(id)`, `revokeAllOtherSessions()`.
- Admin: `listUsers()`, `createUser(username, password, role)`, `patchUser(id, patch)`, `resetUserPassword(id)`, `disableUserTOTP(id)`, `deleteUser(id)`.

**`web/src/views/Login.svelte`** (extended):
- "Sign in with a passkey" button triggers WebAuthn assertion via the Web Authentication API (`navigator.credentials.get`). On success, calls `POST /api/v1/passkey-sessions/finish`.
- TOTP second-step: when login response has `totp_required: true`, the form transitions to a 6-digit code input with a "Use a recovery code instead" toggle. Stores `pending_token` in component state only (never persisted).

**New `web/src/views/settings/Security.svelte`** (or section within a unified `Settings.svelte`):

Sessions sub-section:
- Table of active sessions: user_agent, address, last seen, relative time.
- Current session row is visually distinguished and has no revoke button.
- "Log out everywhere" button at the top.
- Per-row "Revoke" button.

TOTP sub-section:
- Not enrolled state: "Set up authenticator app" → opens a modal showing the QR code and manual entry secret. Confirm step: 6-digit code input. Success: recovery codes shown once in a modal with "I've saved these" gate.
- Enrolled state: "Disable 2FA" button (prompts for current TOTP code) + "Regenerate recovery codes" button (prompts for current TOTP code, shows new codes once).

Passkeys sub-section:
- List of registered passkeys with label, created date, and "Remove" button.
- "Add a passkey" button → triggers WebAuthn registration via `navigator.credentials.create`. Prompts for a label after success.

**New `web/src/views/Admin.svelte`:**
- Visible only when `$auth.user?.role === 'admin'`.
- User list table: username, role, created, disabled status badge, has_totp indicator, passkey count.
- "Create user" button → inline form (username, password, role selector).
- Per-row dropdown/actions: "Reset password" (shows temporary password in a modal with copy button), "Disable 2FA" (confirmation prompt), "Disable account" / "Re-enable account", "Delete" (confirmation modal).

**`web/src/lib/router.ts`** (extended):
- New `/admin` route → `Admin` view, guarded in `App.svelte` on `role === 'admin'`.
- New `/settings` route → `Settings` view (or the security section thereof). M8 can elaborate the settings routing.

**`web/src/App.svelte`** (extended):
- Navigation gains "Settings" and (if admin) "Users" links.
- Route guard: `/admin` redirects to `/` if `role !== 'admin'`.

### Configuration knobs (additions to M6's set)

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--webauthn-rp-id` | `TAP_WEBAUTHN_RP_ID` | derived from `--addr` | WebAuthn relying party ID (hostname). |
| `--webauthn-origin` | `TAP_WEBAUTHN_ORIGIN` | derived from `--addr` + `--cookie-secure` | WebAuthn origin URL. |

### `cmd/tap/admin.go` additions

Two new subcommands (M6 shipped `create` and `passwd`):

`tap admin disable-totp <username> [--data <dir>]`:
- `GetUserByUsername`; missing → exit 2.
- Deletes `totp_secrets` + `recovery_codes` rows for that user.
- Stdout: `2FA disabled for 'username'`, exit 0.

(Password reset CLI already landed in M6 as `tap admin passwd`.)

### README update

After the M6 auth paragraph:

> **Authentication (M7).** TOTP (RFC 6238) is available as an optional second factor; users enrol from Settings → Security. Recovery codes (8 single-use codes, shown once at enrolment) allow disabling TOTP without admin involvement. Passkeys (WebAuthn discoverable credentials) are an alternative login method; a passkey login does not additionally prompt for TOTP. Active sessions are listed in Settings → Security with device and IP information; any session can be revoked individually or all at once. Admins can create users, reset passwords, and disable 2FA from the SPA user-management view (or from the CLI: `tap admin disable-totp <username>`). Each user's subscriptions and entries are isolated — no user (including admins) can see another user's feeds. TOTP secrets are AES-256-GCM encrypted at rest using a server key stored in the `configuration` table. The WebAuthn relying party ID and origin must be configured explicitly for non-loopback deployments (`--webauthn-rp-id`, `--webauthn-origin`).

Plus upgrade note: "M7 is breaking. The schema migration adds `user_id NOT NULL` to subscriptions and entries. Start with a fresh database."

### Tests and methodology

M7 follows the test-first discipline of M1–M6 (`docs/roadmap.md` §"Working cadence"). Pure scaffolding (migration SQL, README edit, flag declarations, type-only SPA additions) is exempt; everything with branches, error handling, or state is in scope.

**`internal/auth/totp_test.go`:**
- `GenerateTOTPSecret` returns a valid base32 string; two calls return distinct values.
- `VerifyTOTP`: valid code at T=now → true; stale code at T=now-2 → false; ±1 window edge cases.
- `TOTPSecretURI`: returns well-formed `otpauth://totp/` URI containing the issuer, account name, and secret.
- `GenerateRecoveryCodes`: returns exactly 8 codes; each is 10 chars from the allowed alphabet; all 8 are distinct.

**`internal/auth/encrypt_test.go`:**
- `EncryptTOTPSecret` + `DecryptTOTPSecret` roundtrip.
- Decrypt with wrong key → error.
- Two encryptions of the same secret produce different ciphertexts (nonce randomness).

**`internal/auth/pending_test.go`:**
- `MintPendingToken`: value is 43 base64url chars; `sha256(decode(value)) == hash`.

**`internal/db/totp_test.go`:**
- InsertTOTPSecret + GetTOTPSecret roundtrip; `confirmed` starts false.
- `ConfirmTOTPSecret` sets confirmed.
- Duplicate insert (UNIQUE on user_id) → error.
- `InsertRecoveryCodes` + `GetUnconsumedRecoveryCodes`: 8 codes returned before any consumed; 7 after `ConsumeRecoveryCode`; 0 after `DeleteRecoveryCodes`.

**`internal/db/passkeys_test.go`:**
- InsertPasskey + GetPasskeyByCredentialID roundtrip.
- `GetPasskeysByUserID` returns all for the user, none for another user.
- `UpdatePasskeySignCounter`.
- `DeletePasskey`.

**`internal/db/pending_logins_test.go`:**
- InsertPendingLogin + GetPendingLoginByTokenHash roundtrip.
- Expired token (expires_at in the past) is deleted by `DeleteExpiredPendingLogins`.
- `DeletePendingLogin` removes the row.

**`internal/db/sessions_test.go`** (extended):
- `ListSessionsByUserID` returns only sessions for the given user.
- `SetWebAuthnChallenge` / `ClearWebAuthnChallenge` roundtrip.
- Session creation captures `user_agent` and `address`.

**`internal/db/subscriptions_test.go`** (extended):
- `InsertSubscription` stores `user_id`; `ListSubscriptions(userA)` returns only userA's subscriptions; `ListSubscriptions(userB)` returns none when userB has none.
- `GetSubscription(id, userB)` on a subscription owned by userA → `sql.ErrNoRows`.

**`internal/db/entries_test.go`** (extended):
- Same pattern as subscriptions: per-user filter enforced on list and get.

**`internal/db/users_test.go`** (extended):
- `ListUsers` returns all users.
- `DeleteUser` cascades subscriptions + entries.
- `GetUserTOTPStatus` returns correct has/confirmed values at each stage of enrolment.

**`internal/api/totp_test.go`:**
- `POST /api/v1/me/totp`: unauthenticated → 401; authenticated, no existing secret → 200 + secret_uri; already enrolled (confirmed) → 409 totp_already_enrolled.
- `POST /api/v1/me/totp/confirm`: valid code → 200 + 8 recovery_codes; invalid code → 401 totp_invalid; no pending secret → 404.
- `DELETE /api/v1/me/totp`: valid code → 204; invalid code → 401 totp_invalid; not enrolled → 409 totp_not_enrolled.
- `POST /api/v1/me/totp/recovery-codes`: valid code → 200 + 8 new codes; old codes are gone.

**`internal/api/passkeys_test.go`:**
- Registration begin → 200 + CredentialCreation JSON; challenge stored on session.
- Registration finish: valid attestation → 201; challenge cleared; invalid attestation → 400.
- `GET /api/v1/me/passkeys`: returns list without credential_id or public_key.
- `DELETE /api/v1/me/passkeys/{id}`: own passkey → 204; other user's passkey → 404.
- Passkey login begin → anonymous session created, challenge stored.
- Passkey login finish: valid assertion → 200 + session cookie; invalid → 401; expired challenge session → 401.

**`internal/api/auth_test.go`** (extended):
- `POST /api/v1/sessions` with TOTP-enrolled user: valid password → 200 `{totp_required: true, pending_token: "..."}`, no cookie set.
- Second step with valid code → 200 + session cookie.
- Second step with wrong code → 401 totp_invalid.
- Second step with expired pending_token → 401 invalid_credentials.
- Second step with valid recovery code → 200 + session cookie; code marked consumed.
- `GET /api/v1/sessions`: returns session list for current user, not other users.
- `DELETE /api/v1/sessions/{id}`: own non-current session → 204 + row deleted; current session → 400 cannot_revoke_current_session; other user's session → 404.
- `DELETE /api/v1/sessions`: all other sessions deleted; current session survives.

**`internal/api/admin_test.go`:**
- `GET /api/v1/admin/users`: non-admin → 403 admin_required; admin → 200 + list.
- `POST /api/v1/admin/users`: valid → 201; duplicate username → 409; short password → 400.
- `POST /api/v1/admin/users/{id}/password-reset`: → 200 + temporary_password; user's sessions deleted.
- `POST /api/v1/admin/users/{id}/disable-totp`: → 204; totp_secrets + recovery_codes rows gone.
- `PATCH /api/v1/admin/users/{id}`: disable → disabled_at set; re-enable → cleared.
- `DELETE /api/v1/admin/users/{id}`: → 204 + cascade; self-delete → 400 cannot_delete_self.

**`internal/api/middleware_test.go`** (extended):
- `requireAdmin`: admin user → handler called; non-admin → 403 admin_required.
- Session creation with `User-Agent` and `X-Forwarded-For` headers: values stored on the row.

**`internal/api/subscriptions_test.go`** (extended):
- `POST /api/v1/subscriptions` as userA; `GET /api/v1/subscriptions` as userB → empty list.
- `DELETE /api/v1/subscriptions/{id}` as userB on userA's subscription → 404.

**`cmd/tap/admin_test.go`** (extended):
- `tap admin disable-totp <user>`: TOTP secret + codes deleted, exit 0.
- Missing user → exit 2.

**`web/src/lib/__tests__/auth.test.ts`** (extended):
- `auth.beginPasskeyLogin` calls `POST /passkey-sessions/begin`.
- TOTP second step: login returning `totp_required` stores `pending_token`, transitions form state.

**`web/src/views/__tests__/Login.test.ts`** (extended):
- "Sign in with a passkey" button present.
- TOTP step renders on `totp_required` response; "Use a recovery code instead" toggle switches input type.

**`web/src/views/__tests__/Security.test.ts`** (new):
- Session list renders with current session marked.
- "Revoke" button calls `api.revokeSession(id)`.
- "Log out everywhere" calls `api.revokeAllOtherSessions()`.
- TOTP unenrolled → setup button visible; enrolled → disable + regenerate visible.
- TOTP enrolment modal shows QR URI; confirm step calls `api.confirmTOTPEnrolment`.
- Passkey list renders; "Add" triggers `beginPasskeyRegistration`.

**`web/src/views/__tests__/Admin.test.ts`** (new):
- Non-admin: Admin view not rendered / redirected.
- Admin: user list rendered; "Create user" form submits; "Reset password" shows temporary password; "Delete" shows confirmation.

## Out of scope (deferred)

| Concern | Lands in |
|---|---|
| Per-source / per-account brute-force lockout, login rate-limit | M12 |
| Re-hash-on-verify for argon2 param drift | M12 |
| Per-user iframe-host allowlist override | Deferred items (roadmap) |
| Categories (per-user with `user_id`) | M9 |
| M11 tombstones `user_id` scoping | M11 (FK chain through subscriptions implies per-user; M11 is responsible for correct scoping) |
| Email column on users + email-based recovery | Won't ship — SMTP is not a Tap dependency |
| WebAuthn as a strict second factor (in addition to, not instead of, password) | Won't ship — concept §7.3 explicitly positions passkeys as an alternative login method |
| Admin-visible feeds (override per-user isolation) | Won't ship — strict per-user privacy is the stated posture; no admin override |
| Per-feed `outbound_proxy_url` | Future milestone |
| M12 observability for auth events labelled by `user_id` | M12 (high-cardinality label decision deferred) |

## Risks and open questions

- **WebAuthn RP ID / origin misconfiguration.** If `--webauthn-rp-id` doesn't match the browser's origin hostname, registration and assertion will fail with cryptic errors. The binary should validate at startup that `rpID` is a suffix of the parsed origin hostname and log a clear error if not. Operators using a reverse proxy with a different hostname than `--addr` must set both knobs explicitly.

- **Anonymous challenge sessions for passkey login.** The `passkey-sessions/begin` endpoint creates a row in `sessions` with `user_id = NULL`. These rows are invisible to `requireSession` (which rejects NULL user_id) and must be cleaned up if the ceremony is abandoned. The middleware's absolute-expiry delete path already handles stale sessions; anonymous challenge sessions get the same 5-minute TTL via `idle_expires_at = now + 5m` and `absolute_expires_at = now + 5m`. No separate cleanup ticker needed.

- **WebAuthn challenge reuse.** A single session row holds `webauthn_challenge`. If a user clicks "Add a passkey" twice in quick succession, the second click overwrites the first challenge. The first registration ceremony will fail at finish (challenge mismatch). This is acceptable: the user sees an error and tries again. A future improvement could issue one-time challenge rows; not needed now.

- **TOTP window drift.** ±1 window (one 30-second step) tolerates ~30 seconds of clock drift. For extreme drift, the user must sync their device clock. This is the industry standard.

- **Recovery code hashing cost at enrolment.** `POST /api/v1/me/totp/confirm` hashes all 8 recovery codes sequentially using `auth.Hash(DefaultParams)` (argon2id t=2, m=64MiB). On typical self-hosted hardware this takes 1–2 seconds total — acceptable for a one-time enrolment flow, not a hot path. Test code must inject lower-cost params (same pattern as the existing auth tests in M6) to keep the test suite fast.

- **Recovery code exhaustion.** 8 codes; no auto-regeneration. A user who exhausts all codes and forgets their TOTP device must contact an admin (or use `tap admin disable-totp` from the host). This is the intended recovery path — concept §7.6 specifies admin-mediated recovery.

- **TOTP secret leakage via QR code.** The `POST /api/v1/me/totp` response returns the raw `secret` for manual entry. It is transmitted over HTTPS (or cleartext for loopback-only deployments). This is acceptable — the QR / manual entry step is the only moment the secret crosses the wire; it is never returned again. The SPA should not log or cache the secret value.

- **`user_id` on `sessions` nullable change.** Changing `sessions.user_id` from `NOT NULL` to nullable (for passkey login challenge sessions) requires care in the migration. SQLite cannot alter column constraints directly; the migration recreates the table via the standard SQLite alter pattern (rename → create new → copy → drop old). This is the only place in the M7 migrations that requires a table recreation. The migration is wrapped in a transaction; if it fails, the database rolls back cleanly.

- **M11 tombstones.** The coordinator notes flag this: tombstones don't exist yet (M11), but when they do, the `(subscription_id, hash)` dedup contract implies per-user scoping because subscriptions are now per-user. M11 is responsible for correctly scoping tombstone consults and insertions by user. M7 does not need a tombstones `user_id` column because the table doesn't exist yet.

- **M9 categories.** Categories will need `user_id` when they land in M9. This is M9's responsibility — the pattern is now established by M7's subscription/entry isolation.

- **M12 metrics with `user_id` labels.** Auth event metrics labelled by `user_id` are high-cardinality; M12 must decide whether to label at all or use only aggregate counters. M7 does not emit metrics; M12 owns this decision.

## Definition of done

1. `make test` passes (`go test ./... -race`) including all new packages, migrations, API extensions, and SPA tests.
2. Fresh DB + `TAP_ADMIN_USERNAME=ben TAP_ADMIN_PASSWORD=...`: server boots, admin can log in, admin can create a second user from the SPA. The second user's subscription list is empty and does not show the admin's subscriptions.
3. TOTP enrolment: admin opens Settings → Security, scans QR with an authenticator app, enters the 6-digit code, receives recovery codes. Subsequent login prompts for the 6-digit code. A valid recovery code also completes login and the code is then rejected on second use.
4. Passkey registration: admin clicks "Add a passkey", browser authenticator prompts, passkey appears in the list. "Sign in with a passkey" on the login screen completes login without a password or TOTP prompt.
5. Session listing: both sessions (password + passkey) appear in Settings → Security with distinct user-agent strings. Revoking one from the other session invalidates the cookie.
6. Admin resets another user's password from the SPA: temporary password shown once; old sessions for that user return 401; user logs in with the temporary password.
7. Admin disables another user's TOTP from the SPA: subsequent login for that user does not prompt for TOTP.
8. `DELETE /api/v1/admin/users/{id}` cascades: the deleted user's subscriptions, entries, sessions, passkeys, and TOTP data are gone.
9. Per-user isolation: userA subscribes to a feed; userB logs in and `GET /api/v1/subscriptions` returns an empty list; `GET /api/v1/entries` returns an empty list; `DELETE /api/v1/subscriptions/{userA_sub_id}` as userB returns 404.
10. `tap admin disable-totp <username>` removes TOTP for that user; login no longer prompts for TOTP.
11. Migration 0006 and 0007 apply cleanly against a fresh M6 database.
12. `make build` produces a static binary that boots cleanly against a fresh `data/` directory.
13. WebAuthn works end-to-end on localhost with default configuration (no flags needed for local development).

## What this milestone deliberately does *not* prove

- That brute-force login attempts are rate-limited or locked out — M12.
- That argon2 params are tuned or re-hashed on verify — M12.
- That per-user iframe-host allowlist overrides work — deferred items.
- That categories are per-user — M9 owns that column.
- That tombstones are per-user — M11 owns that.
- That M12 observability metrics label auth events by user — M12 decides the cardinality trade-off.
- That WebAuthn works as a strict second factor on top of a password — concept §7.3 positions passkeys as an alternative, not an addition.
- That an admin can see another user's feeds — strict per-user isolation is the stated posture; no admin override will ever ship.
- That OPML import/export is per-user — M9.
- That email-based password recovery works — SMTP is not a Tap dependency.

If you find yourself adding brute-force lockout, per-user iframe allowlists, categories, FTS, or OPML, push back. M7's job is the smallest surface that closes the 2FA gap, wires passkey login, enforces per-user data isolation, and hands M8 a complete SPA surface to polish.
