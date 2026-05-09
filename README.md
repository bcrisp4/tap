# tap

Self-hosted RSS / Atom / JSON Feed reader. Single binary, embedded SQLite,
embedded SPA, no external dependencies.

- [Concept](docs/concept.md)
- [Roadmap](docs/roadmap.md)
- [UI design references](ui_design/)

## Status

M3 in progress — media proxy and FS cache. M2 sanitisation pipeline merged.
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

The binary still defaults to `-addr 127.0.0.1:8080` as defence in depth
(concept §6.11). The container variant binds `0.0.0.0:8080` because
Docker port mapping requires it.

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
