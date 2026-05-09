# tap

Self-hosted RSS / Atom / JSON Feed reader. Single binary, embedded SQLite,
embedded SPA, no external dependencies.

- [Concept](docs/concept.md)
- [Roadmap](docs/roadmap.md)
- [UI design references](ui_design/)

## Status

M2 in progress — sanitisation pipeline complete; M2 awaiting merge.
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

The binary still defaults to `-addr 127.0.0.1:8080` as defence in depth
(concept §6.11). The container variant binds `0.0.0.0:8080` because
Docker port mapping requires it.

## Upgrading from M1

M1 databases are incompatible with M2 — the entries table holds raw HTML
that the M2 sanitiser was never run against. Before starting M2:

- **Binary deployment:** delete `tap.db` from your data directory and
  re-subscribe.
- **Container deployment:** delete the `/data` volume (or its `tap.db`
  file) and re-subscribe.
