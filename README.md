# tap

Self-hosted RSS / Atom / JSON Feed reader. Single binary, embedded SQLite,
embedded SPA, no external dependencies.

- [Concept](docs/concept.md)
- [Roadmap](docs/roadmap.md)
- [UI design references](ui_design/)

## Status

M5 in review — article extraction (per-subscription opt-in Readability
or per-feed CSS selector mode, bounded-parallel inside the worker,
fail-soft to feed summary on per-entry error). M4 polling discipline
merged. M3 media proxy + FS cache merged. M2 sanitisation pipeline merged.
See [`docs/specs/`](docs/specs/) for milestone specs.

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

The binary still defaults to `-addr 127.0.0.1:8080` as defence in depth
(concept §6.11). The container variant binds `0.0.0.0:8080` because
Docker port mapping requires it.

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
