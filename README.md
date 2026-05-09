# tap

Self-hosted RSS / Atom / JSON Feed reader. Single binary, embedded SQLite,
embedded SPA, no external dependencies.

- [Concept](docs/concept.md)
- [Roadmap](docs/roadmap.md)
- [UI design references](ui_design/)

## Status

Pre-M1 — walking-skeleton implementation in progress.
See [`docs/specs/`](docs/specs/) for milestone specs.

## Development

Requires Go 1.25+, pnpm, Make.

```bash
make dev      # run Go on :8080 and Vite on :5173 — open http://localhost:5173
make build    # build single static binary at bin/tap
make docker   # build distroless container image
```

## M1 deployment safety

In M1 the server renders feed HTML **without sanitisation** — that lands in M2.
The binary defaults to `-addr 127.0.0.1:8080`, which contains the risk to the
local machine. The container variant binds `0.0.0.0:8080` because Docker port
mapping requires it.

**Do not reverse-proxy the M1 container to anywhere a hostile-feed author can
reach** — Tailscale, LAN, the public internet. A malicious feed can plant
stored XSS in your reader otherwise. M2 closes this gap.
