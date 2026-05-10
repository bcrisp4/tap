# tap

Self-hosted RSS / Atom / JSON Feed reader. Single binary, embedded SQLite,
embedded SPA, no external dependencies.

- [Concept](docs/concept.md)
- [Roadmap](docs/roadmap.md)
- [UI design references](ui_design/)

## Status

M6 in progress — auth foundations (password login, sessions, CSRF, admin
bootstrap, per-feed credential redaction). M5 article extraction merged.
M4 polling discipline merged. M3 media proxy + FS cache merged. M2
sanitisation pipeline merged. See [`docs/specs/`](docs/specs/) for
milestone specs.

## Development

Requires Go 1.25+, pnpm, Make.

```bash
make dev      # run Go on :8080 and Vite on :5173 — open http://localhost:5173
make build    # build single static binary at bin/tap
make docker   # build distroless container image
```

## Trust posture

Feed HTML is sanitised on the server before storage (M2): scripts, on-event
handlers, dangerous URL schemes, iframes outside a small allowlist, 1×1
tracking pixels, and well-known tracking parameters in `<a href>` and
`<img src>` URLs are all stripped. The SPA renders the stored HTML
directly without a runtime sanitiser.

Article images are fetched through Tap's media proxy (M3) and cached on
the local filesystem. Origin sites see only Tap's IP — never the user's
browser — and mixed-content image URLs work even when Tap is served over
HTTPS. The proxy enforces an image-only MIME allowlist (PNG, JPEG, GIF,
WebP, AVIF) and rejects any response that doesn't match. Proxy URLs are
HMAC-signed against a server-generated secret in the database, so a peer
with API access can't construct proxy URLs that point at arbitrary URLs.

Outbound HTTP runs through one shared client (M4). Destinations resolving
to loopback, RFC1918, link-local, CGNAT, or ULA are rejected before
connect. Tailscale users on the default `100.64.0.0/10` CGNAT range need
`--ssrf-allow=100.64.0.0/10` (or their tailnet's specific subnet, or
`--ssrf-allow=<tailnet>.ts.net` for a hostname suffix). Redirects are
re-checked independently. The same per-hostname concurrency cap
(default 4) applies to feed fetches and media-proxy origin fetches.
Polling cadence is adaptive: a fast feed polls every 15 minutes, a quiet
feed every 24 hours; origin-mandated `Retry-After` and
`Cache-Control: max-age` are honoured as floors. Failed polls back off
exponentially (5m → 10m → 20m → 40m … capped at 24h, with 25% jitter).
Set `--ssrf-disabled` only on fully-trusted networks; the binary logs a
startup WARN when this flag is on.

Subscriptions can opt into full-article extraction (M5):
`POST /api/v1/subscriptions {..., "extract": true}` or
`PATCH /api/v1/subscriptions/:id {"extract": true}`. When enabled, the
worker fetches each new entry's article URL through the shared HTTP
client (so the M4 SSRF guard, per-host cap, and `--http-timeout` apply)
and runs the response through Readability — or, if the subscription
has an `extract_selector` CSS rule set, through that selector.
Extracted HTML flows through the same M2 sanitiser and M3 image proxy
as feed-provided HTML. Per-entry extraction failures degrade to the
feed-provided summary with `extract_failed = 1`; they never abort the
poll or count against the subscription's error budget. Toggling
extract from off→on affects future polls only — existing entries are
not re-fetched.

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

The binary still defaults to `-addr 127.0.0.1:8080` as defence in depth
(concept §6.11). The container variant binds `0.0.0.0:8080` because
Docker port mapping requires it.

**Authentication (M7).** TOTP (RFC 6238) is available as an optional
second factor; users enrol from Settings → Security. Recovery codes
(8 single-use codes, shown once at enrolment) allow disabling TOTP
without admin involvement. Passkeys (WebAuthn discoverable credentials)
are an alternative login method; a passkey login does not additionally
prompt for TOTP. Active sessions are listed in Settings → Security with
device and IP information; any session can be revoked individually or
all at once. Admins can create users, reset passwords, and disable 2FA
from the SPA user-management view (or from the CLI:
`tap admin disable-totp <username>`). Each user's subscriptions and
entries are isolated — no user (including admins) can see another
user's feeds. TOTP secrets are AES-256-GCM encrypted at rest using a
server key stored in the `configuration` table. The WebAuthn relying
party ID and origin must be configured explicitly for non-loopback
deployments (`--webauthn-rp-id`, `--webauthn-origin`).

> **M7 is a breaking migration.** The schema migration adds `user_id NOT NULL`
> to subscriptions and entries. Start with a fresh database when upgrading
> from M6.

## Configuration knobs added by M4

- `--http-timeout` (env `TAP_HTTP_TIMEOUT`, default `30s`) — total
  per-request HTTP deadline. (Replaces M3's `--proxy-fetch-timeout`,
  which is removed.)
- `--per-host-inflight` (env `TAP_PER_HOST_INFLIGHT`, default `4`) —
  concurrent outbound requests per hostname.
- `--ssrf-disabled` (env `TAP_SSRF_DISABLED`, default `false`) — global
  escape hatch.
- `--ssrf-allow` (env `TAP_SSRF_ALLOW`, default `""`) — repeatable
  allowlist entry. CSV in env. Auto-detect: `/`-bearing entries are
  CIDR; bare IPs become `/32` or `/128`; otherwise hostname suffix.
- `--poll-floor` / `--poll-ceiling` / `--poll-error-base` — adaptive
  cadence and error-backoff tuning.
- `--user-agent` (env `TAP_USER_AGENT`) — set centrally on the shared
  client.

## Configuration knobs added by M5

- `--extract-concurrency` (env `TAP_EXTRACT_CONCURRENCY`, default `4`)
  — per-worker parallel article fetches when a subscription has
  `extract=true`. The M4 per-host cap further serialises same-host
  bursts.
- `--extract-body-cap-bytes` (env `TAP_EXTRACT_BODY_CAP_BYTES`,
  default `5242880` (5 MiB)) — per-article HTTP body cap before the
  extractor parses it.

## Configuration knobs added by M6

| Flag | Env | Default | Notes |
|---|---|---|---|
| `--session-idle-ttl` | `TAP_SESSION_IDLE_TTL` | `168h` (7d) | Refreshed on each authenticated request. |
| `--session-absolute-ttl` | `TAP_SESSION_ABSOLUTE_TTL` | `2160h` (90d) | Hard cap; cookie Max-Age. |
| `--cookie-secure` | `TAP_COOKIE_SECURE` | `auto` | `auto`/`true`/`false`. `auto` resolves to `true` when `--addr` binds non-loopback. |
| (env-only) | `TAP_ADMIN_USERNAME` | (unset) | First-launch admin bootstrap. Both must be set; partial → fatal. |
| (env-only) | `TAP_ADMIN_PASSWORD` | (unset) | First-launch admin bootstrap. Min 8 chars; failure → fatal. |

## Data layout

`${TAP_DATA_DIR}` (default `./data` for the binary, `/data` for the container)
holds two things:

- `tap.db` — the SQLite database. Subscriptions, entries, configuration.
- `cache/` — the media proxy cache. Sharded by URL hash. Default size cap
  500 MiB, configurable via `--proxy-cache-cap-bytes` or
  `TAP_PROXY_CACHE_CAP_BYTES`. Safe to delete at any time — the next
  request re-fetches.

## Upgrading from M1

M1 databases are incompatible with M2 — the entries table holds raw HTML
that the M2 sanitiser was never run against. Before starting M2:

- **Binary deployment:** delete `tap.db` from your data directory and
  re-subscribe.
- **Container deployment:** delete the `/data` volume (or its `tap.db`
  file) and re-subscribe.

## Upgrading from M2

No destructive change required. M3 only adds the `configuration` table
(holding the proxy signing key, generated on first M3 launch). Existing
entries keep their direct `<img src="origin">` URLs and won't be
retroactively rewritten to proxy URLs — only entries inserted from M3
onwards get proxied URLs. To proxy all entries' images, delete `tap.db`
(binary) or the `/data` volume (container) and re-subscribe.

## Upgrading from M3

No destructive change required. M4 adds one column to `subscriptions`:
`velocity_24h_x100`. Existing rows start at velocity 0 (24h ceiling) and
back-fill on their next successful poll or 304.

## Upgrading from M4

No destructive change required. Migration 0004 adds three columns:
`subscriptions.extract`, `subscriptions.extract_selector`, and
`entries.extract_failed`. All default to off/empty; existing
subscriptions stay non-extract until opted in via PATCH.

## Upgrading from M5

M6 is breaking. Auth tables and credential columns are additive, but
existing databases have no users — login is unusable until either
`TAP_ADMIN_USERNAME`/`TAP_ADMIN_PASSWORD` are set on next boot, or
`tap admin create` is run from the host.
