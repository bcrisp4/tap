# Security Exceptions

This file documents accepted-risk security findings that cannot be fixed within the current milestone scope. Each entry includes the finding, severity, affected path, reason for acceptance, and the condition under which it should be revisited.

## govulncheck findings (as of M12, Go 1.26.2)

### GO-2026-4971 — Panic in Dial on NUL byte (Windows)

- **Severity:** Medium
- **Fixed in:** `net@go1.26.3`
- **Affected path:** `cmd/tap/main.go` → `http.Client.CloseIdleConnections` → net dial path
- **Description:** Panic in `Dial` and `LookupPort` when handling NUL byte on Windows.
- **Reason accepted:** Tap is deployed on Linux (distroless container, systemd service). The NUL byte issue is a Windows-only path. Tap does not run on Windows.
- **Revisit when:** Go 1.26.3 is available and `go.mod` is updated.

### GO-2026-4918 — Infinite loop in HTTP/2 SETTINGS_MAX_FRAME_SIZE

- **Severity:** Medium
- **Fixed in:** `net/http@go1.26.3` (via `golang.org/x/net`)
- **Affected path:** `internal/feed/parse.go` → HTTP client do → HTTP/2 transport; `internal/httpx/client.go` → transport
- **Description:** Infinite loop in the HTTP/2 transport when a server sends a bad `SETTINGS_MAX_FRAME_SIZE` frame.
- **Reason accepted:** This requires an attacker-controlled HTTP/2 server to send a malformed frame. Feed URLs are operator-supplied (Tap is self-hosted). The SSRF guard blocks arbitrary URLs. This is a Denial-of-Service risk against a specific malicious feed origin, not a data-exfiltration risk. The risk is low given the single-admin, self-hosted deployment model.
- **Revisit when:** Go 1.26.3 is available and `go.mod` is updated.

## semgrep findings

No `ERROR`-severity findings at time of M12 implementation.

## Route authn/authz audit

All routes verified against `internal/api/api.go` mux registration — see spec table in `docs/specs/2026-05-10-m12-observability-hardening.md`.

Routes present and correctly configured:
- `/healthz` GET — unauthenticated ✓
- `/metrics` GET — unauthenticated, disabled by default, network boundary control ✓
- `POST /api/v1/sessions` — unauthenticated, rate-limited by ratelimit.Limiter ✓
- `GET /api/v1/status` — authenticated (requireSession) + admin check in handler ✓
- All other `/api/v1/*` routes — authenticated with requireSession middleware ✓
- Write routes (POST/PATCH/DELETE) — CSRF required ✓
- Admin routes — requireAdmin middleware applied ✓
- `GET /api/v1/proxy/{token}` — authenticated (token itself is signed, session required) ✓

## Outbound HTTP SSRF posture

All outbound HTTP calls go through `httpx.NewClient`:
- Feed fetch (`internal/poll/worker.go`) ✓
- Article extraction (`internal/extract`) ✓
- Media proxy origin fetch (`internal/proxy/handler.go`) ✓
- **Exception:** OTLP metric/trace export (OTel SDK internal client) — operator-configured endpoint, not user-supplied. Documented in spec §"OTLP exception". ✓

No `http.Get`, `http.Post`, or inline `http.Client{}` found outside the OTLP exception.

## MaxBytesReader audit

All write handlers verified to have `http.MaxBytesReader` with 1 MiB cap and 413 detection:
- `POST /api/v1/sessions` ✓
- `PATCH /api/v1/me/password` ✓
- `POST /api/v1/subscriptions` ✓ (M12 deferred-items fix)
- `PATCH /api/v1/subscriptions/{id}` ✓ (M12 deferred-items fix)
- `PATCH /api/v1/entries/{id}` ✓ (M12 deferred-items fix)
- M7 routes (TOTP, passkeys, admin, sessions) ✓ — verified present in M7 implementation

All return 413 on oversize body via `errors.As(*http.MaxBytesError)`.
