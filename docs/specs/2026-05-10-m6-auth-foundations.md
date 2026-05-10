# M6 — Auth foundations

**Status:** Draft, awaiting review.

## Context

Tap is the self-hosted feed reader described in [`../concept.md`](../concept.md). M6 is the sixth of twelve milestones — see [`../roadmap.md`](../roadmap.md). M1's walking skeleton ([`2026-05-08-m1-walking-skeleton.md`](2026-05-08-m1-walking-skeleton.md)), M2's sanitisation pipeline ([`2026-05-08-m2-sanitisation.md`](2026-05-08-m2-sanitisation.md)), M3's media proxy ([`2026-05-09-m3-media-proxy.md`](2026-05-09-m3-media-proxy.md)), M4's polling discipline ([`2026-05-09-m4-polling-discipline.md`](2026-05-09-m4-polling-discipline.md)), and M5's article extraction ([`2026-05-09-m5-article-extraction.md`](2026-05-09-m5-article-extraction.md)) have shipped.

Through M5, every `/api/v1/*` route is open: anyone with network access to the listener can list subscriptions, mark entries read, post a feed URL. The loopback-default bind keeps that quiet for binary deployments on a single host, but it falls apart the moment Tap is exposed beyond loopback (containerised, behind a reverse proxy, on a tailnet). Concept §7 specifies the closure: per-user accounts, password login, sessions with idle and absolute expiry, CSRF discipline. M5's "Out of scope" table also pinned per-feed credentials — cookie and basic-auth — to M6, because the same redaction discipline applies uniformly to every credential-bearing field on the server.

The shape of M6 is two related landings. One is the user-account auth: a new `internal/auth` package, two new tables (`users` + `sessions`), four new endpoints under `/api/v1/`, two pieces of middleware, a `Login.svelte` view in the SPA, and `tap admin` subcommands for offline account management. The other is per-feed credentials: three new columns on `subscriptions`, write-only DTO discipline on the existing endpoints, and a small `httpx.ApplyFeedCreds` helper layered onto polling and article-fetch requests. Concept §6.10's redaction invariant — "accepted on writes, never returned on reads" — covers both landings.

TOTP, passkeys, session listing/revocation, admin reset paths in the SPA, and brute-force lockout are out of scope and pinned to M7 / M12.

## Goal

After M6, a fresh boot of Tap with `TAP_ADMIN_USERNAME=ben TAP_ADMIN_PASSWORD=...` set in the environment creates an admin user automatically, and the SPA opens a login screen. A correct username + password POST to `/api/v1/sessions` mints a session, sets an `HttpOnly` cookie, and returns a CSRF token; the SPA echoes that token as `X-CSRF-Token` on every state-changing request. Every `/api/v1/*` route except `POST /sessions` and `/healthz` rejects requests without a valid session with `401 invalid_session`. State-changing requests without a matching CSRF token return `403 csrf_invalid`.

A subscription created with `{"feed_url":"https://private.example.com/feed", "basic_auth_user":"ben", "basic_auth_pass":"..."}` polls the feed with an `Authorization: Basic ...` header. Subsequent `GET /api/v1/subscriptions` calls return `has_basic_auth: true` and never the password. PATCH with `{"basic_auth_pass":""}` clears the password; PATCH with the field omitted keeps it.

`tap admin create` from the host creates additional users without bringing the server down. `tap admin passwd <username>` resets a forgotten password and force-logs-out any active sessions. The same code path runs whether `users` is empty (first admin) or already populated (additional accounts) — concept §10's "same code paths" guarantee.

## In scope

### New package: `internal/auth`

```
internal/auth/
  argon2.go        # Hash, Verify, Params, ValidatePassword
  argon2_test.go
  tokens.go        # MintSessionToken, MintCSRFToken
  tokens_test.go
```

Public surface:

```go
type Params struct {
    Time, Memory, Threads uint32
    SaltLen, KeyLen       uint32
}

// DefaultParams: OWASP 2026 recommendation. Pinned as a constant; if a
// future milestone bumps these, that milestone is also responsible for the
// re-hash-on-verify path (see Risks; landed in M12).
var DefaultParams = Params{
    Time:    2,
    Memory:  64 * 1024, // KiB → 64 MiB
    Threads: 1,
    SaltLen: 16,
    KeyLen:  32,
}

func Hash(password string, p Params) (string, error)
func Verify(encoded, password string) (bool, error)
func ValidatePassword(s string) error // ErrPasswordTooShort if len < 8

// MintSessionToken returns (cookieValue, tokenHash, err). cookieValue is
// 32 base64url bytes for the Set-Cookie header; tokenHash is the sha256-hex
// of the underlying random bytes for storage on sessions.token_hash.
func MintSessionToken() (cookieValue, tokenHash string, err error)

// MintCSRFToken returns 32 base64url bytes.
func MintCSRFToken() (string, error)
```

`Hash` produces the standard PHC encoding `$argon2id$v=19$m=...,t=...,p=...$<salt-b64>$<hash-b64>`. `Verify` parses the encoding, runs argon2id with the parsed params, and constant-time compares the result. The encoded params travel with the hash so a future params bump can be paired with re-hash-on-verify.

`ValidatePassword`: minimum length 8, no maximum, no complexity rules. Stable error so the API layer can map to `400 password_too_short`.

Two new dependencies (already pure-Go, no CGO):

| Module | Purpose | License |
|---|---|---|
| `golang.org/x/crypto/argon2` | argon2id KDF | BSD-3-Clause |
| `golang.org/x/term` | `ReadPassword` for the `tap admin` CLI prompts (no echo) | BSD-3-Clause |

### Schema migration

`internal/db/migrations/0005_auth_and_credentials.sql`:

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

The two auth tables and the three subscription columns are one logical change ("M6 brings credentials into the schema") and ship in one migration. Existing M5 subscriptions get empty credential columns; no behaviour change until they're set via PATCH. Existing M5 databases have no users — login is unusable until either `tap admin create` is run or `TAP_ADMIN_USERNAME`/`TAP_ADMIN_PASSWORD` is set on the next boot.

`user_agent` and `address` columns on `sessions` are deliberately omitted — they're useful only with M7's session-listing UI and adding them now without a consumer is dead state. M7 is responsible for adding the columns and back-filling empty strings on existing rows.

### `db` layer extensions

New file `internal/db/users.go`:

```go
type User struct {
    ID           int64
    Username     string
    PasswordHash string
    Role         string
    CreatedAt    int64
    DisabledAt   sql.NullInt64
}

type NewUser struct {
    Username, PasswordHash, Role string
    CreatedAt                    int64
}

var ErrUserExists = errors.New("user with this username already exists")

func InsertUser(ctx, d, NewUser) (int64, error)
func GetUserByUsername(ctx, d, username) (User, error) // sql.ErrNoRows if missing
func GetUserByID(ctx, d, id) (User, error)
func UpdatePasswordHash(ctx, d, id int64, hash string) error
func DisableUser(ctx, d, id int64) error
func CountUsers(ctx, d) (int, error) // for the bootstrap path
```

New file `internal/db/sessions.go`:

```go
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
    UserID                              int64
    TokenHash, CSRFToken                string
    CreatedAt, LastSeenAt               int64
    IdleExpiresAt, AbsoluteExpiresAt    int64
}

func InsertSession(ctx, d, NewSession) (int64, error)
func GetSessionByTokenHash(ctx, d, tokenHash string) (Session, error)
func RefreshSessionIdle(ctx, d, id, lastSeen, idleExpires int64) error
func UpdateSessionCSRFToken(ctx, d, id int64, csrf string) error
func DeleteSession(ctx, d, id int64) error
func DeleteSessionsByUserID(ctx, d, userID int64) error
func DeleteOtherSessionsForUser(ctx, d, userID, keepID int64) error
```

`subscriptions.go` extensions:

- `Subscription` and `DueSubscription` structs gain `Cookie`, `BasicAuthUser`, `BasicAuthPass string`.
- `ListDuePolls`, `ListSubscriptions`, and `GetSubscription` SELECTs grow the three columns.
- `InsertSubscription` accepts the three credentials on `NewSubscription`.
- A new `UpdateSubscriptionCredentials(ctx, d, id, cookie, user, pass string)` helper backs the PATCH endpoint's credential surface, mirroring the existing `UpdateSubscriptionExtraction` shape.

### Middleware

New file `internal/api/middleware.go`:

```go
type ctxKey int
const (
    ctxKeyUser ctxKey = iota
    ctxKeySession
)

func userFromContext(ctx context.Context) (db.User, bool)
func sessionFromContext(ctx context.Context) (db.Session, bool)

// requireSession reads the tap_session cookie, looks up the session row by
// sha256(cookie), validates idle + absolute expiries, refreshes idle on
// success, and injects user + session into the request context. 401
// invalid_session on any failure.
func requireSession(d *sql.DB, idleTTL time.Duration) func(http.Handler) http.Handler

// requireCSRF passes through GET/HEAD/OPTIONS, otherwise:
//   - validates Origin or Referer host equals r.Host (when the header is
//     present; both absent = pass, since SameSite=Lax + an authenticated
//     session already cover that case);
//   - reads X-CSRF-Token and constant-time compares to session.CSRFToken;
//   - 403 csrf_invalid on any failure.
func requireCSRF() func(http.Handler) http.Handler
```

`requireSession`'s idle refresh is a single `UPDATE sessions SET last_seen_at = ?, idle_expires_at = ? WHERE id = ?`. Concurrent refreshes converge — SQLite's WAL serialises writes, both updates land in the same total order.

A session that has crossed `absolute_expires_at` returns 401 *and* deletes the row in the same handler — keeps the table from growing with stale rows even when the user never explicitly logs out.

Mounting in `internal/api/api.go`'s `NewMux` (sketched):

```go
authed     := requireSession(db, idleTTL)
authedCSRF := chain(authed, requireCSRF())

m.HandleFunc("GET /healthz", healthz)
m.HandleFunc("POST /api/v1/sessions", login)

m.Handle("GET /api/v1/sessions/current",    authed(http.HandlerFunc(getSessionCurrent)))
m.Handle("DELETE /api/v1/sessions/current", authedCSRF(http.HandlerFunc(logout)))
m.Handle("PATCH /api/v1/me/password",       authedCSRF(http.HandlerFunc(passwordChange)))

m.Handle("GET /api/v1/subscriptions",       authed(http.HandlerFunc(listSubscriptions)))
m.Handle("POST /api/v1/subscriptions",      authedCSRF(http.HandlerFunc(createSubscription)))
// … rest of subscriptions, entries, proxy …
```

Existing routes don't move; only their registration grows the middleware decorator. The proxy handler is wrapped by `authed` only — proxy URLs are GETs.

### API extensions

All under `/api/v1/`. DTOs are explicit per project conventions (write-shape and read-shape are distinct types — credential fields physically cannot leak into a read response). Errors via the existing `writeError` from `internal/api/errors.go`. New stable error codes:

```go
const (
    ErrCodeInvalidCredentials = "invalid_credentials"
    ErrCodeInvalidSession     = "invalid_session"
    ErrCodeCSRFInvalid        = "csrf_invalid"
    ErrCodePasswordTooShort   = "password_too_short"
)
```

Auth endpoints in a new `internal/api/auth.go`:

- **`POST /api/v1/sessions`** — login. Public. CSRF not required.
  - Body: `{"username": string, "password": string}`. >1 MiB → 413.
  - Verify: user exists, not disabled, `auth.Verify` matches.
  - All four failure modes (unknown user / wrong password / disabled / malformed body) collapse to a single `401 invalid_credentials` body. Don't enumerate.
  - On success: mint session token + CSRF token; insert sessions row; set cookie.
  - Response body: `{"user": {"id":..., "username":..., "role":...}, "csrf_token": "..."}` with `200 OK`.
  - Cookie: `tap_session=<value>; HttpOnly; Path=/; SameSite=Lax; Max-Age=<absolute_ttl_seconds>`. `Secure` per the rule below.

- **`DELETE /api/v1/sessions/current`** — logout. Authenticated. CSRF required.
  - Delete the session row by ID (from context).
  - Set cookie: `tap_session=; HttpOnly; Path=/; SameSite=Lax; Max-Age=0`.
  - 204.

- **`GET /api/v1/sessions/current`** — probe. Authenticated. No CSRF (GET).
  - Returns the same shape as login: `{"user":..., "csrf_token":"..."}`.
  - SPA calls this on boot to recover its in-memory CSRF token after a reload.
  - 200.

- **`PATCH /api/v1/me/password`** — change own password. Authenticated. CSRF required.
  - Body: `{"current_password": string, "new_password": string}`.
  - Verify `current_password` against the user's hash; mismatch → 401 `invalid_credentials`.
  - Validate `new_password` (≥8); too short → 400 `password_too_short`.
  - Hash with `auth.Hash(new, DefaultParams)`. UPDATE `users.password_hash`.
  - Delete all of this user's sessions EXCEPT the current one (recovery primitive — the session-list-and-revoke UI is M7's job).
  - Mint a new CSRF token for the current session and UPDATE `sessions.csrf_token`. Token rotation on password change is the only rotation M6 does (see Risks).
  - Response body: `{"csrf_token": "<new>"}` so the SPA can update its in-memory copy. 200.

#### Cookie `Secure` rule

`--cookie-secure={auto,true,false}` flag (env `TAP_COOKIE_SECURE`, default `auto`).

- `auto`: parse `--addr`, set `Secure` iff the bound host is *not* loopback (`127.0.0.0/8`, `::1`, `localhost`). Matches the loopback-default bind posture from concept §6.11 — the local binary deployment doesn't need Secure (and can't have it without a TLS terminator); the container/exposed deployment does.
- `true` / `false`: force.

#### Subscription credential surface

`POST /api/v1/subscriptions` body grows three optional fields:

```json
{
  "feed_url": "...",
  "title": "...",
  "extract": false,
  "cookie": "session=abc; tracking=...",
  "basic_auth_user": "ben",
  "basic_auth_pass": "..."
}
```

`PATCH /api/v1/subscriptions/{id}` body grows the same three optional fields, with the M5 `extract`/`extract_selector` semantics: omitted = no change, empty string = explicit clear. The PATCH handler already pre-reads the row (for the M5 selector) and merges the credential fields the same way.

The read DTO `subscriptionDTO` grows `has_cookie` / `has_basic_auth` booleans:

```go
type subscriptionDTO struct {
    // … existing fields …
    HasCookie    bool `json:"has_cookie"`
    HasBasicAuth bool `json:"has_basic_auth"`
}
```

`has_cookie` is true iff `cookie != ""`. `has_basic_auth` is true iff `basic_auth_user != ""` — RFC 7617 permits an empty password, so the username alone is the "is basic auth configured" signal; requiring both fields to be non-empty would mis-report valid empty-password cases as unconfigured. `cookie`, `basic_auth_user`, and `basic_auth_pass` never appear on a read-shape DTO.

### Per-feed credentials wiring

New helper in `internal/httpx`:

```go
type FeedCreds struct {
    Cookie        string
    BasicAuthUser string
    BasicAuthPass string
}

// ApplyFeedCreds layers per-feed Cookie and Authorization headers onto req.
// Empty Cookie → no Cookie header set (any pre-existing one preserved).
// BasicAuth applied iff BasicAuthUser != "" (empty pass is permitted by HTTP).
func ApplyFeedCreds(req *http.Request, creds FeedCreds)
```

Called by:

- The poll worker's feed-fetch step, before sending the request through the shared client.
- `internal/extract.Extract`, which gains a `creds FeedCreds` parameter and calls `ApplyFeedCreds` on the per-article request before sending.

NOT called by:

- The media proxy origin fetch path (`internal/proxy/handler.go`). Mirrors Miniflux's posture — see Risks.

`extract.Extract` signature change:

```go
func Extract(ctx context.Context, client *http.Client,
             articleURL, selector string, bodyCap int64,
             creds httpx.FeedCreds) (string, error)
```

Existing M5 callers (tests and the worker) update to pass the new parameter. `internal/poll/worker.go` reads `Cookie`, `BasicAuthUser`, `BasicAuthPass` off the `DueSubscription` and constructs `FeedCreds` once per poll, passing it to both the feed fetch and the `Extract` call.

### Admin bootstrap

`cmd/tap/main.go` grows a small subcommand router. Hand-rolled `os.Args[1]` switch — no cobra.

```go
switch {
case len(os.Args) >= 2 && os.Args[1] == "admin":
    runAdmin(os.Args[2:])  // cmd/tap/admin.go
default:
    runServer(os.Args[1:]) // existing server entry point
}
```

New file `cmd/tap/admin.go`. Subcommands:

`tap admin create [--data <dir>] [--role admin|user]`:
- Read username from stdin (`bufio.Scanner`).
- Read password (no echo, via `golang.org/x/term`). Re-prompt to confirm. Mismatch → exit 3.
- Validate password; too short → re-prompt.
- Hash via `auth.Hash`. INSERT into `users`.
- `ErrUserExists` → exit 2 with stderr `user 'username' already exists`.
- Success → stdout `created admin user 'username' (id=N)`, exit 0.

`tap admin passwd <username> [--data <dir>]`:
- `GetUserByUsername`; missing → exit 2 with stderr `user 'username' not found`.
- Read password (no echo) twice. Mismatch → exit 3.
- Validate, hash, UPDATE password_hash.
- `DeleteSessionsByUserID` — admin reset implies the user is being locked out and re-given a working credential.
- Success → stdout `password reset for 'username'`, exit 0.

Both subcommands open the DB read-write, run migrations (so they work on a fresh data dir), and close cleanly. Both honour `--data` / `TAP_DATA_DIR` exactly like the server.

Env-var first-launch shortcut, in `runServer` after `db.Migrate`:

```go
n, err := db.CountUsers(ctx, d)
if err != nil { slog.Error("count users", "err", err); os.Exit(1) }

if n == 0 {
    user := os.Getenv("TAP_ADMIN_USERNAME")
    pass := os.Getenv("TAP_ADMIN_PASSWORD")
    switch {
    case user != "" && pass != "":
        if err := bootstrapAdmin(ctx, d, user, pass); err != nil {
            slog.Error("bootstrap admin", "err", err); os.Exit(1)
        }
        slog.Info("bootstrapped admin from environment", "username", user)
    case user != "" || pass != "":
        slog.Error("partial admin bootstrap: both TAP_ADMIN_USERNAME and TAP_ADMIN_PASSWORD must be set"); os.Exit(1)
    default:
        slog.Warn("no users in database; create one with 'tap admin create' or set TAP_ADMIN_USERNAME and TAP_ADMIN_PASSWORD")
    }
}
// users exist → silent no-op even if env vars are still set (concept §7.1).
```

`bootstrapAdmin` validates the password via `auth.ValidatePassword` first — a too-short bootstrap password aborts startup loudly rather than producing a user nobody can log in as.

### SPA changes

New file `web/src/views/Login.svelte`:
- Username + password inputs, submit button, error display.
- Submits to `auth.login(...)`. On 401, displays `Invalid username or password.` On other errors, displays the server's `error.message` or a generic fallback.
- Functional only — visual polish lives in M8.

New file `web/src/lib/auth.ts`:

```ts
import { writable } from 'svelte/store';

export type User = { id: number; username: string; role: 'admin' | 'user' };
type State = { user: User | null; csrfToken: string | null; bootstrapped: boolean };

const internal = writable<State>({ user: null, csrfToken: null, bootstrapped: false });

export const auth = {
  subscribe: internal.subscribe,
  async bootstrap()                              { /* GET /sessions/current; populate or clear */ },
  async login(username: string, password: string) { /* POST /sessions */ },
  async logout()                                 { /* DELETE /sessions/current */ },
  setCSRFToken(token: string)                    { /* used by the password-change response handler */ },
};
```

`web/src/lib/api.ts`:
- `request()` reads the auth store's `csrfToken`. For non-GET methods, attaches `X-CSRF-Token: <token>`.
- 401 response from any endpoint clears the auth store; `App.svelte` reactively renders Login when `auth.user == null`.
- New `addSubscription` overload accepts the optional credential fields. New `patchSubscription(id, patch)` for arbitrary field PATCHes (the server-side endpoint exists from M5; M6 makes the client first-class).
- New `changePassword(current, new)` updates `auth.csrfToken` from the response body.

`web/src/lib/types.ts`:
- `Subscription` gains `extract`, `extract_selector`, `extract_failed` (backfill — the M5 read DTO already returns these; the SPA type was missing them) plus `has_cookie`, `has_basic_auth`.
- `EntryListItem` and `EntryDetail` gain `extract_failed` (also M5 backfill).
- New types `User`, `SessionResponse`, `PasswordChangeResponse`.

`App.svelte`:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { route } from './lib/router';
  import { auth } from './lib/auth';
  import Login from './views/Login.svelte';
  import Unread from './views/Unread.svelte';
  import Reader from './views/Reader.svelte';

  onMount(() => auth.bootstrap());
</script>

{#if !$auth.bootstrapped}
  <!-- briefly empty during bootstrap; keeps the SPA from flashing the login form
       for an authenticated user -->
{:else if $auth.user == null}
  <Login />
{:else if $route.name === 'reader'}
  <Reader id={$route.params.id} />
{:else}
  <Unread />
{/if}
```

### Configuration knobs

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--session-idle-ttl` | `TAP_SESSION_IDLE_TTL` | `168h` (7d) | Refreshed on each authenticated request. |
| `--session-absolute-ttl` | `TAP_SESSION_ABSOLUTE_TTL` | `2160h` (90d) | Hard cap; cookie Max-Age. |
| `--cookie-secure` | `TAP_COOKIE_SECURE` | `auto` | `auto`/`true`/`false`. `auto` = Secure when bound non-loopback. |
| (env-only) | `TAP_ADMIN_USERNAME` | (unset) | First-launch admin bootstrap. |
| (env-only) | `TAP_ADMIN_PASSWORD` | (unset) | First-launch admin bootstrap. |

Argon2 params live as constants in `internal/auth.DefaultParams` rather than flags. Operators don't tune them; tests inject lower-cost params for speed.

`TAP_ADMIN_*` are env-only — accepting passwords on the command line leaks via shell history and `ps`.

### `cmd/tap/main.go` wire-up

The server entry point grows three flag bindings (idle TTL, absolute TTL, cookie-secure mode), the user-count + bootstrap call, and the new mux options:

```go
apiMux := api.NewMux(d, api.MuxOpts{
    Poke:               sched.Poke,
    ProxyHandler:       proxyHandler,
    SessionIdleTTL:     *sessionIdleTTL,
    SessionAbsoluteTTL: *sessionAbsoluteTTL,
    CookieSecure:       resolveCookieSecure(*cookieSecureMode, *addr),
})
```

`cmd/tap/admin.go` lives next to `main.go`. The `runAdmin` path opens the same DB, runs migrations, then dispatches to the subcommand.

### README update

After the existing M5 paragraph in the trust-posture section:

> **Authentication (M6).** Tap is multi-user. Users have password-based accounts (argon2id at OWASP 2026 defaults); sessions are `HttpOnly` cookies hashed at rest with idle (`--session-idle-ttl`, default 7d) and absolute (`--session-absolute-ttl`, default 90d) expiries. State-changing requests need a matching `X-CSRF-Token` header issued at login. The first admin is created either by setting `TAP_ADMIN_USERNAME` and `TAP_ADMIN_PASSWORD` on first launch, or by running `tap admin create` from the host. Subsequent admins use the same `tap admin create` subcommand. `tap admin passwd <username>` resets a forgotten password and force-logs-out that user's active sessions. Per-feed credentials (`cookie`, `basic_auth_user`, `basic_auth_pass`) are accepted on POST/PATCH `/api/v1/subscriptions` but never returned by the read endpoints — the GET shape exposes only `has_cookie` / `has_basic_auth` booleans. Per-feed credentials apply to feed polling and article extraction (when `extract=true`); they do **not** apply to the media-proxy origin fetch path, matching Miniflux's posture. Same-origin authenticated images consequently render as broken — a known cross-ecosystem limitation.

Plus a one-line upgrade note: "M6 is breaking. Auth tables and credential columns are additive, but existing databases have no users — login is unusable until either `TAP_ADMIN_*` env vars are set on next boot or `tap admin create` is run from the host."

### Tests and methodology

M6 follows the test-first discipline of M1–M5 (`docs/roadmap.md` §"Working cadence"). Pure scaffolding (the migration SQL file, README edit, flag declarations, the type-only SPA additions) is exempt; everything with branches, error handling, or state is in scope.

- **`internal/auth/argon2_test.go`** — Hash + Verify roundtrip; Verify rejects wrong password; fixture-encoded hash regression (catches accidental param-format drift); ValidatePassword (≥8 ok, <8 → ErrPasswordTooShort, empty → error, length-9 ok); Hash uses the supplied params (round-trip a non-default Params and assert Verify still works).
- **`internal/auth/tokens_test.go`** — MintSessionToken: cookie value is 43 base64url chars (32 bytes → 43 unpadded), tokenHash is 64 hex chars; `sha256(base64url-decode(cookie)) == tokenHash`. MintCSRFToken: 43 base64url chars; two calls return distinct values.
- **`internal/db/users_test.go`** — InsertUser + GetUserByUsername roundtrip; duplicate username → `ErrUserExists`; unknown username/id → `sql.ErrNoRows`; `UpdatePasswordHash`; `DisableUser` sets `disabled_at`; `CountUsers` against empty + populated tables.
- **`internal/db/sessions_test.go`** — InsertSession + GetSessionByTokenHash roundtrip; `RefreshSessionIdle` updates `last_seen_at` + `idle_expires_at`; `UpdateSessionCSRFToken`; `DeleteSession`; `DeleteSessionsByUserID` removes all rows for the user; `DeleteOtherSessionsForUser` keeps the named row.
- **`internal/api/middleware_test.go`** — `requireSession`: missing cookie → 401 invalid_session; bad cookie value (no DB row) → 401; idle-expired → 401 + row deleted; absolute-expired → 401 + row deleted; valid → handler called, `last_seen_at` refreshed. `requireCSRF`: GET passthrough; POST with no `X-CSRF-Token` → 403 csrf_invalid; POST with wrong token → 403; POST with right token → handler called; POST with mismatched Origin → 403; POST with Origin equal to `r.Host` → handler called.
- **`internal/api/auth_test.go`** — POST /sessions: valid → 200 + cookie + body; bad password → 401 invalid_credentials; unknown user → 401 (same code/message); disabled user → 401 (same); >1 MiB body → 413; bad JSON → 400. DELETE /sessions/current: row deleted, Set-Cookie clears. GET /sessions/current: returns `{user, csrf_token}`. PATCH /me/password: valid → 200 + new csrf_token, current session retained, other sessions for the user deleted; wrong `current_password` → 401 invalid_credentials; too-short `new_password` → 400 password_too_short.
- **`internal/api/subscriptions_test.go`** (extended) — POST with `cookie`/`basic_auth_*`: persists; subsequent GET returns `has_cookie`/`has_basic_auth` booleans, never the values. PATCH `cookie=""` clears; PATCH cookie omitted preserves; same for `basic_auth_*`. PATCH partial: `{extract:true}` alone preserves the cookie; `{cookie:"x"}` alone preserves extract.
- **`internal/poll/worker_test.go`** (extended) — feed fetch with creds set: outbound request includes `Cookie:` and/or `Authorization: Basic ...`; with creds unset: neither header present.
- **`internal/extract/extract_test.go`** (extended) — Extract with non-empty `FeedCreds.BasicAuth`: outbound request carries `Authorization:`; with empty `FeedCreds`: no auth header. Existing M5 tests update to pass `httpx.FeedCreds{}`.
- **`internal/proxy/handler_test.go`** (extended) — proxy origin fetch never carries `Cookie:` or `Authorization:` even when the originating subscription has them set (regression — explicit posture lock).
- **`cmd/tap/main_test.go`** (extended) — Boot with empty users + no env vars: server starts, POST /sessions with any creds returns 401. Boot with empty users + `TAP_ADMIN_USERNAME`/`PASSWORD` set: admin row exists, login works. Boot with users + env vars still set: silent no-op (`CountUsers` unchanged across the restart). End-to-end: subscribe with basic_auth, poll, fixture origin sees the auth header. End-to-end: media proxy fetch from the same authenticated origin returns 401 (documented limitation; explicit assertion).
- **`cmd/tap/admin_test.go`** (new) — `tap admin create` with simulated stdin: row inserted, exit 0. Duplicate username: stderr message, exit 2. `tap admin passwd <user>`: hash updated, sessions for that user deleted, exit 0. Missing user: stderr message, exit 2. Password mismatch: exit 3.
- **`web/src/lib/__tests__/auth.test.ts`** (new) — `auth.bootstrap` on cold start: GET /sessions/current 200 → user + csrfToken populated, bootstrapped true; GET 401 → user null, bootstrapped true. `auth.login` happy + 401. `auth.logout` clears state.
- **`web/src/lib/__tests__/api.test.ts`** (extended) — `request()` attaches `X-CSRF-Token` on POST/PUT/PATCH/DELETE when `csrfToken` is set; 401 from any endpoint clears auth state; `changePassword` updates `auth.csrfToken` from the response body.
- **`web/src/views/__tests__/Login.test.ts`** (new) — renders form; submit calls `auth.login` with form values; error text shows on rejection.

## Out of scope (deferred)

| Concern | Lands in |
|---|---|
| TOTP enrolment, recovery codes, WebAuthn / passkeys | M7 |
| Session listing + per-session revocation in the SPA | M7 |
| `user_agent` / `address` columns on `sessions` | M7 (lands with the list-sessions UI that needs them) |
| Admin "reset password" / "disable 2FA" actions in the SPA | M7 |
| Admin user-creation from the SPA | M7 |
| Per-source / per-account brute-force lockout, login rate-limit | M12 |
| Re-hash-on-verify for argon2 param drift | M12 (paired with any future bump of `auth.DefaultParams`) |
| Per-feed `outbound_proxy_url` (concept §4) | Won't ship in M6 — needs per-feed `http.Transport`. Future milestone or follow-up. |
| Per-feed user-agent override, HTTP/2 toggle, self-signed-cert toggle | Won't ship — orthogonal; add when a real feed needs them. |
| Email column on users + email-based recovery | Concept §7.6: SMTP is not a Tap dependency. Recovery is admin-mediated via the CLI. |
| CSRF token rotation outside password-change | Won't ship — token lifetime equals session lifetime; see Risks. |
| GET-by-id under `/api/v1/subscriptions/:id` | Not in M6 — no SPA caller. M9 add-feed/edit-feed flow lands it. |
| Bootstrapping the first admin from the SPA | Concept §7.1: out-of-band only. CLI + env-var. |
| Bind to non-loopback by default | Concept §6.11 holds. Container deploy already binds `0.0.0.0` because the network namespace is the boundary. |
| Per-feed credentials applied to media-proxy origin fetches | Mirrors Miniflux/TTRSS posture. Same-origin authenticated images are a known cross-ecosystem limitation; documented in README. |
| Audit log table | Concept §10: no separate audit log. Authentication events emit structured logs to stdout; the M12 recent-errors ring buffer is the in-process surface. |
| Password complexity rules (uppercase / digit / symbol mandates) | Won't ship — modern guidance favours length over class diversity. Min 8, no max. |

## Risks and open questions

- **Argon2 param drift over hardware lifetime.** `DefaultParams` are baked as constants. Hardware gets faster; in five years a `t=2, m=64MiB` hash takes a fraction of today's wall-clock to compute and the security-cost mismatch widens. Standard fix when we eventually bump the params: re-hash-on-verify (compare stored params to current; if weaker, re-hash with current and UPDATE on a successful login). The encoded PHC format makes this purely additive. **M12 owns this** — paired with the bump itself, so shipping the bump alone can't be the bug.

- **CSRF token doesn't rotate within a session except on password change.** Matches Discourse / GitLab posture and concept §7.5's intent that sessions are the unit of revocation. If the token leaks (XSS, log mistake, browser extension), the leak is good for the remaining session lifetime — up to 90 days. Tap's XSS surface is small (M2 sanitiser; no third-party scripts; same-origin SPA), but not zero. M7+ can add per-request rotation if a real concern surfaces; the cost is races on in-flight requests and silent lockouts when rotation is mishandled.

- **Idle-refresh races.** Two concurrent authenticated requests both UPDATE `last_seen_at` and `idle_expires_at` on the same session. SQLite's WAL serialises writes, both updates land in total order, and the row converges. No coordination needed.

- **Session row growth without explicit logout.** Users who close the browser without logging out leave session rows around. The middleware's "absolute-expired → DELETE on next access" covers the long tail; sessions hit absolute expiry within 90 days regardless of activity. A future M11/M12 sweep could prune more eagerly; not in M6.

- **Disabled user with active session.** `disabled_at` is checked by the login path, not by `requireSession`. A user disabled while logged in stays logged in until session expiry. The M7 admin-reset path (which deletes that user's sessions) closes the loop. M6 alone leaves the gap.

- **Bootstrap env-var leakage.** `TAP_ADMIN_PASSWORD` lives in the process environment. On Linux, anyone with `/proc/<pid>/environ` access (typically root or the process owner) sees it. K8s secret mounts mitigate but don't eliminate. Documented in the README as: "use a secret manager, not a plain shell export, in production."

- **Per-feed credentials redaction depends on convention, not types.** The DTO write-shape and read-shape are physically distinct Go types — a typo on the read side can't include the credential field, because the credential field doesn't exist on that type. But the *convention* (don't add credential fields to the read DTO) is enforced by code review, not the compiler. A future helper that auto-derives a "redacted view" of the row could harden this; not in M6.

- **`tap admin create` running concurrently with the server.** Two processes opening the same SQLite file. WAL mode handles concurrent reads + writes correctly, but there's a brief window where the new admin can race a server-side login probe. The new user lands; the in-flight login uses old data. Acceptable.

- **Loopback-default still applies.** M6's authentication does *not* relax the `127.0.0.1:8080` default in concept §6.11. Auth is defence-in-depth, not the primary network boundary.

- **`POST /sessions` is unauthenticated and unrate-limited.** A peer with network access to the listener can attempt arbitrary password guesses against arbitrary usernames. Acceptable for the loopback-default deployment threat model. M12's brute-force lockout closes this for non-loopback deployments.

- **Migration column count.** 0005 lands two tables and three columns in one file. Same M5 rationale: one milestone = one logical change = one migration.

## Definition of done

1. `make test` passes (`go test ./... -race`) including the new packages, the migration, the middleware, the API extensions, the worker + extract extensions, and the SPA tests.
2. Fresh DB + `TAP_ADMIN_USERNAME=ben TAP_ADMIN_PASSWORD=secret123` env vars: server boots, admin row exists, login through the SPA succeeds. A subsequent restart with the env vars still set leaves users unchanged (silent no-op).
3. Fresh DB + no env vars: server boots, login rejects everything; `tap admin create` interactively from the host creates a user; SPA login succeeds afterwards.
4. With a logged-in admin session: `POST /api/v1/subscriptions {"feed_url":"https://test/", "basic_auth_user":"u", "basic_auth_pass":"p"}` succeeds, the next poll's outbound request includes `Authorization: Basic ...`, and `GET /api/v1/subscriptions` returns `has_basic_auth: true` with no credential value in the body.
5. PATCH `/api/v1/subscriptions/{id}` with `{"basic_auth_pass":""}` clears the password column; PATCH with the field omitted preserves it.
6. `PATCH /api/v1/me/password` keeps the current session active, deletes the user's other sessions, and returns a new `csrf_token` in the response body. Subsequent state-changing requests with the *old* token return 403 csrf_invalid.
7. `tap admin passwd <username>` updates the hash and deletes all of that user's sessions; the user re-logs-in with the new password and any prior session cookie returns 401.
8. Every existing `/api/v1/*` route except `POST /sessions` and `/healthz` returns 401 invalid_session without a valid session cookie. State-changing requests without a matching `X-CSRF-Token` return 403 csrf_invalid.
9. Origin/Referer mismatched against `r.Host` on a state-changing request returns 403 csrf_invalid.
10. The media-proxy origin fetch path never carries `Cookie:` or `Authorization:` even when the originating subscription has them set.
11. Migration 0005 applies cleanly against an M5 database; existing subscriptions get empty credential columns; existing entries unchanged.
12. `make build` produces a static binary that boots cleanly against a fresh `data/` directory and against an existing M5 database.

## What this milestone deliberately does *not* prove

- That TOTP, recovery codes, or passkeys work — M7.
- That a user can list and revoke their active sessions — M7.
- That an admin can reset another user's password from the SPA — M7. CLI works.
- That an admin can create another user from the SPA — M7.
- That brute-force login attempts are rate-limited or locked out — M12.
- That argon2 params will be tuned over time — M12 (paired with re-hash-on-verify).
- That same-origin authenticated images render — known cross-ecosystem limitation, mirrors Miniflux.
- That per-feed `outbound_proxy_url` works — deferred.
- That CSRF tokens rotate aggressively — they don't, by design.
- That an audit log exists in the database — concept §10's "stdout is the audit log" stands.

If you find yourself adding TOTP enrolment, a session-list UI, admin reset paths in the SPA, login rate-limiting, an audit log table, password complexity rules, or per-feed `outbound_proxy_url`, push back. M6's job is the smallest authentication surface that closes the open-API gap, with clean seams for M7 (passkeys, 2FA, session-list UI, admin reset paths) and M12 (lockout, observability, argon2 tuning) to plug into.
