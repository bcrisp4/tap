# Tap — Concept

This document describes the high-level concept and key design choices for Tap, a self-hosted feed reader.

------------------------------------------------------------------------

## 1. Purpose and scope

Tap is a self-hosted RSS / Atom / JSON Feed reader. It runs as one self-contained server process backed by one embedded relational database file and one filesystem directory for a media cache. It serves a private REST API for one client: a single-page web app served from the same process.

The product end-to-end:

- polls subscribed feeds on an adaptive schedule, deduplicates and ingests new entries;
- optionally extracts the full article body for link-only feeds;
- sanitises every entry's HTML and routes every image through an internal proxy;
- exposes the result through a private REST surface that the SPA consumes online or offline;
- archives old+read+unsaved entries with a daily archival job

Out of scope:
- Hosted offering
- Compatibility layers for other APIs e.g. Google Reader

------------------------------------------------------------------------

## 2. Deployment shape

The release artifact is a single OCI container image. The image runs as a non-root user against a single writable volume that holds the database file and the media cache. There are no external services: no PostgreSQL, no Redis, no queue, no object store.

A second deployment shape — a single statically-linked binary runnable directly on a Linux host — falls out of the container build naturally and is supported. The binary should be self-contained enough to drop into a distroless / scratch-style base image with no shell and no libc dependencies. End users self-host on heterogeneous hardware (Raspberry Pi, NAS, VPS) and need predictable cross-compile.

The web client is built ahead of time and embedded into the server binary, so deployment is one image, one volume, one port.

------------------------------------------------------------------------

## 3. Architecture in broad strokes

A single server process with three concurrent concerns running alongside the HTTP server:

1.  **HTTP server.** Serves the REST API under `/api/v1/`, the embedded SPA at every other path, a media proxy under `/api/v1/proxy/`, and a flat health endpoint.
2.  **Poller.** A scheduler that ticks on a fixed cadence, picks feeds whose next-poll time has arrived, and dispatches them to a small pool of workers. Each worker handles one feed end to end: fetch, parse, optionally extract per-entry content, sanitise, commit.
3.  **Archival ticker.** A daily sweep that deletes old read-and- unsaved entries (recording tombstones) and prunes the media cache.

State lives in three places:

- **Relational state** (subscriptions, entries, full-text index, tombstones, enclosures, icons, user configuration) lives in an embedded database. The database is the source of truth and the queue: the next-poll time on each subscription row is the schedule.
- **Binary state** (cached image bytes for the media proxy) lives in a sharded filesystem cache directory.
- **Live counters** (active polls, last poll timestamp, recent errors) live in process memory only and are recomputed on restart.

The web client is a single-page app embedded in the server binary at build time. It supports offline reading: cached entries remain readable when the server is unreachable, and state-changing actions performed while offline (mark read, save, etc.) are queued locally and replayed on reconnect.

------------------------------------------------------------------------

## 4. Data model (conceptual)

The relational side carries:

- **Users.** A row for each user, including login credentials (password hash, optional TOTP secret), role (admin or user), basic profile, an optional disabled-at timestamp, and inline preferences stored as columns.
- **Sessions.** Active login sessions, one row per session, carrying a hashed session token, the owning user, creation and last-seen timestamps, idle and absolute expiries, and the user agent and address that minted it.
- **Passkeys.** WebAuthn credentials registered against a user, one row per credential, carrying the credential identifier, public key, signature counter, authenticator metadata, and a user-supplied label.
- **Recovery codes.** Single-use codes issued at 2FA enrolment, stored hashed and marked consumed on first use.
- **Categories.** A flat per-user list. A subscription belongs to at most one category. Deleting a category leaves its feeds uncategorised, not orphaned.
- **Subscriptions (feeds).** Per-feed state including title, feed URL, site URL, polling state (last/next poll time, error count, ETag / Last-Modified for conditional GET), per-feed HTTP overrides (user agent, cookie, basic-auth credentials, outbound proxy URL, per-feed TLS posture), an opt-in flag for full-content extraction, and a velocity counter (entries observed in the last seven days).
- **Entries.** Per-entry content, sanitised HTML body, metadata (title, author, URL, publish time, reading-time estimate), and two independent boolean state flags: `read` and `saved`. A saved entry can be either read or unread.
- **Tombstones.** A small table of `(feed, entry-hash)` pairs for entries that the archival sweep has deleted. Consulted before insert so a re-published entry does not reappear as unread.
- **Full-text index.** An index over entry title and content, maintained in lockstep with the entries table.
- **Enclosures.** Podcast / video / image attachments associated with an entry.
- **Icons.** Site favicons, content-addressed by hash so feeds from the same site share a row.
- **Configuration.** A small key/value table for runtime-generated state (e.g. the proxy signing secret).

Each entry carries a stable hash derived from the feed-provided GUID when present, falling back to the entry URL or a title+date composite. The hash plus the feed identifier is the duplicate- detection contract: a re-fetched feed cannot create duplicate entries.

------------------------------------------------------------------------

## 5. Public surface

The server exposes one REST API under `/api/v1/`. The SPA is the only intended client. There is no public API contract beyond what the SPA needs.

The conceptual surface covers:

- authenticating (login, logout, password change), enrolling and managing passkeys, enrolling and disabling TOTP, regenerating recovery codes;
- listing and revoking the caller's active sessions;
- (admin only) creating, disabling, deleting, and resetting other users;
- subscribing, listing, updating, and unsubscribing from feeds;
- discovering feed candidates from a webpage URL;
- nudging the scheduler to re-poll a feed sooner;
- listing, filtering, paginating, and updating entries (mark read, toggle saved); bulk mark-read by feed or category;
- full-text search across entries;
- managing categories;
- importing and exporting OPML;
- reporting system status (version, uptime, recent errors);
- serving media proxy and favicon resources;
- a flat health endpoint for orchestrators.

Conventions worth fixing up front:

- list endpoints return a `data` array plus pagination metadata;
- list responses strip the entry body to keep payloads small; the full body is fetched per entry when the reader opens;
- errors return a stable, enumerated error code so clients can switch on the code, not the message;
- credential-bearing fields are accepted on writes but never returned on reads, even by the owner;
- the only unauthenticated routes are login, the health endpoint, the static SPA assets, and (when the user table is empty) the first-launch bootstrap path; everything else under `/api/v1/` requires a valid session;
- any non-API path falls through to the embedded SPA so client-side routing works for deep links.

The exact wire shapes, query parameters, error codes, and routing patterns are implementation choices.

------------------------------------------------------------------------

## 6. Key design decisions

Each of the following is a load-bearing decision with a rationale. Mechanics are deliberately omitted.

### 6.1 Single self-contained binary

The deployment unit is one image with no native dependencies and one writable volume. Every backend dependency must be compatible with a fully-static, no-libc build. This rules out database drivers and sanitisers that require linking against C libraries, and forces the choice of an embedded database engine that has a pure equivalent in the chosen language.

Rationale: portability across heterogeneous self-hosted hardware, predictable cross-compile, no surprise system dependencies.

### 6.2 Database is the queue

The polling schedule lives on the subscription row as a `next_poll_at` timestamp. There is no separate queue table and no in-memory queue. The dispatcher selects due feeds directly from the subscription table, excluding any currently in flight.

Rationale: an in-memory queue is stale on restart; a separate queue table duplicates information already present on the subscription row. A poll is idempotent (the entry-hash uniqueness contract drops duplicates), so a re-dispatched poll after a crash is safe.

Adding a new subscription writes a `next_poll_at` in the past, so the very next dispatcher tick picks it up — a freshly-subscribed feed is polled immediately rather than waiting for the adaptive cadence to elapse.

### 6.3 Adaptive polling

Each feed is polled at a cadence derived from how often it actually publishes. Quiet feeds drift towards a daily ceiling; active feeds poll as often as every fifteen minutes. Server-mandated backoff (`Retry-After`) and `Cache-Control: max-age` floor the cadence so Tap respects origin guidance. Conditional GET (`If-None-Match` / `If-Modified-Since`) is sent on every poll to make the no-change case cheap.

Rationale: a fixed interval is wrong for every feed simultaneously. Velocity-based scheduling matches publication patterns; conditional GET handles bandwidth and politeness on top.

### 6.4 Per-host concurrency cap

A small per-hostname semaphore is acquired before every outbound HTTP request — feed fetches, article fetches, and media proxy origin fetches all share the same cap. The default is strict serialisation per host.

Rationale: a single dispatcher tick that polls a feed and extracts its articles plus their images on the same CDN could otherwise emit a burst of concurrent requests. Politeness is non-negotiable.

### 6.5 Optional per-entry article extraction

Some feeds publish full article bodies; others publish link-only summaries (Hacker News, newsletters). When a subscription is flagged for extraction, the worker fetches each new entry's article URL and runs it through a readability-style extractor (with optional CSS scraper-rule overrides per feed). Per-entry failures degrade gracefully: the entry is committed with the feed-provided summary and a flag indicating extraction failed.

Rationale: link-only feeds are unreadable in a feed reader without extraction; full-content feeds shouldn't pay extraction's CPU and latency cost. One bad article must not fail the poll for the entire feed.

### 6.6 Universal sanitisation and media proxy

Every entry's HTML — whether extracted or feed-provided — is sanitised on the server before commit and emerges as final-form HTML that the client can render directly. Sanitisation is allowlist-based (tags, attributes, URL schemes), strips scripts and event handlers unconditionally, removes obvious tracking pixels, and bounds DOM traversal depth so adversarial markup cannot exhaust resources.

In the same pass, every image URL is rewritten to point at an internal media proxy, and `<iframe>` embeds are restricted to a host allowlist (overridable) for trusted embed CDNs.

Rationale: the client should trust the server's output and never run a runtime sanitiser. Sanitising on the server is also where the allowlist policy is least likely to drift from what the client actually renders.

### 6.7 Internal media proxy

Article images are fetched through `/api/v1/proxy/{token}` rather than directly from the origin. The token is a signed reference to the source URL (signed with a server-generated secret stored alongside other config); cached responses are stored on the filesystem under a sharded layout with a small metadata sidecar (content type, ETag).

Three constraints justify the proxy:

- **Privacy.** Origin sites see only Tap's IP, never the user's browser.
- **Mixed-content.** HTTP-only image URLs work even when Tap is served over HTTPS.
- **Offline.** Pre-loaded entries can have their proxy URLs cached by the client's service worker for offline reading.

Eviction is two-layered: an inline pass on every miss when total bytes exceed a configurable cap, and a daily age-based pass run by the archival ticker. Concurrent misses for the same URL coalesce so the origin sees one fetch.

The MIME type of the proxied response is constrained to an image allowlist; anything else is rejected.

### 6.8 SSRF guard with explicit allowlist

Every outbound HTTP request runs through a single shared client. The client rejects requests whose post-DNS-resolution destination is loopback, RFC1918, link-local, or ULA. Two escape hatches exist:

- a global flag that disables the check entirely;
- an allowlist of hostname suffixes and CIDR blocks that bypass the check. Suffix matching is dot-boundary; CIDR matching is standard.

Redirects re-check the destination independently so an allowlisted host cannot redirect to an arbitrary internal address.

Rationale: Tap users routinely run on a private network. Blanket RFC1918 block makes that impossible; blanket allowlist invites SSRF. The dual-layer is the minimum granularity that covers both.

### 6.9 Tombstones for re-imports

The archival sweep records the hash of every deleted entry in a small tombstone table. The polling commit path consults tombstones before insert. A republished entry — feed edit, backdate, Atom republish — does not reappear in the unread list.

Rationale: without tombstones the "I already read this" trust model breaks. Soft-delete (keep deleted rows forever, filter at read time) accumulates dead rows indefinitely.

### 6.10 Credential redaction

Per-feed credentials (cookie, basic-auth username/password, outbound proxy URL) are stored in the database and used on outbound requests but are *never* returned by any read endpoint. Writes go through dedicated request shapes that accept these fields, and the client's edit form omits empty inputs from the request body so editing a subscription without re-entering its cookie does not clear it.

Rationale: A peer with network access to the API could otherwise scrape every stored credential. This is defence-in-depth on top of the loopback-default bind.

### 6.11 Local-by-default

The server binds to loopback by default; the container variant binds to all interfaces because the container's network namespace is the boundary.

### 6.12 SPA with offline-first reading

The client is a single-page app. It supports offline reading: the app shell, recently-viewed entries, and their proxied images are cached on the device, and state-changing actions made while offline are queued and replayed on reconnect. The client does not run a runtime HTML sanitiser; it trusts the server-sanitised output.

Mutations made offline must survive a full reload — not just a reconnect — so the offline queue is persisted to the browser's local storage and drained both on the offline-to-online transition and at boot.

The SPA is the only intended client. The product does not commit to a public API contract.

### 6.13 Auto-mark-read on reader open

Opening an entry in the reader fires a mark-read action exactly once per entry per page lifetime. The user can explicitly toggle the state back to unread without the auto-mark refiring.

Rationale: A scroll-based or time-based heuristic creates surprises; an explicit-only flow forces busywork in the common case.

### 6.14 Daily archival as one sweep

A single ticker fires at a fixed interval (every twenty-four hours by default) and runs two passes: delete read-and-unsaved entries older than a configurable horizon (recording tombstones in the same transaction), then unlink media cache files older than a configurable age.

### 6.15 Single-transaction commit per poll

A successful poll commits in one transaction: insert new entries (silently dropping duplicates), recompute the velocity counter, and update the subscription row's polling state. A 304 response skips the inserts and only updates the subscription row. A failed poll updates only the failure-tracking fields. Concurrent readers never see a half-formed view of a poll.

------------------------------------------------------------------------

## 7. Authentication and accounts

Tap is multi-user. Every API request that touches user-owned data is authenticated; the only unauthenticated paths are the login route, the health endpoint, the static SPA assets, and — when the user table is empty — the first-launch bootstrap path.

### 7.1 Provisioning model

Account creation is admin-bootstrapped. The first admin is created out of band: a CLI subcommand on the binary creates an admin interactively, and an environment-variable shortcut on first launch creates one non-interactively when the user table is empty. The shortcut is silent on subsequent launches, so a Kubernetes-secret-style deployment can leave the secret mounted across restarts without surprise. Subsequent accounts are created by an admin from the SPA. There is no public signup, and there is no web setup wizard.

Rationale: Tap is a private reader. Open signup is unwanted; admin-owned provisioning matches what self-hosted users expect, keeps the access list trivially auditable, and avoids dragging in email infrastructure.

### 7.2 Roles

Two roles: *admin* and *user*. Admins create, disable, delete, and reset other accounts. Users manage only their own account and their own content. The role distinction is the only authorisation primitive — there are no per-feed or per-category ACLs.

### 7.3 Credential model

Every account has a password as its baseline credential. Passwords are stored hashed with a modern password-hashing function. Users may additionally register one or more passkeys (WebAuthn discoverable credentials); a passkey is an alternative login method, not a replacement for the password.

A passkey login is treated as already-multi-factor: the authenticator's user verification covers possession and presence in a single step, so passkey login does not also prompt for a second factor. Passkey private keys never reach the server — only the public key, credential identifier, signature counter, and authenticator metadata are stored — so a database leak does not compromise passkey credentials.

### 7.4 Two-factor authentication

Two-factor is optional and user-controlled. Each user enables it on their own account; there is no global enforcement and no admin override. The methods are time-based one-time codes from a standard authenticator app and a small set of single-use recovery codes shown once at enrolment. Recovery codes are stored hashed and marked consumed on first use.

A user who wants hardware-backed second-factor protection registers a passkey instead, so a separate WebAuthn-as-second-factor mode is intentionally not offered.

### 7.5 Sessions

A successful login establishes a session represented by an opaque, unguessable token delivered as a cookie that the browser will not expose to JavaScript and will only return to Tap. The server stores the token in hashed form so reading the database does not yield live credentials. Each session row carries the user, creation and last-seen timestamps, an idle expiry that refreshes on activity, an absolute expiry that caps total lifetime regardless of activity, and the user agent and address that minted the session.

Users can list their active sessions in settings and revoke any of them individually; an explicit "log out everywhere" revokes all of a user's sessions in one action.

Rationale: sessions are the unit users intuitively reason about and the natural unit of revocation when a device is lost. Hashed-at-rest tokens narrow the blast radius of a database leak.

### 7.6 Account recovery

Recovery is admin-mediated. The admin has per-user "reset password" and "disable 2FA" actions in the SPA; a password reset issues a one-time temporary password shown once that the user must change at next login. The same operations are available as CLI subcommands so the admin can recover their own account from the host. A user who still holds their recovery codes can disable their own 2FA without admin involvement.

Rationale: SMTP is not a Tap dependency anywhere else, and adding it solely for password reset is a large surface for a rare event. Admin-mediated reset matches both the deployment shape and self-hosted user expectations.

### 7.7 Brute-force resistance

The login endpoint is rate-limited along two axes: per source address and per target username. After a threshold of consecutive failures on a single account, that account enters a short, escalating lockout so a single attacker cannot churn through guesses against one user without also being throttled by their address. Authentication events — success, failure, lockout, password change, 2FA enable/disable, passkey enrolment, admin user-management actions — are emitted to stdout as structured logs and reflected in the in-memory recent-errors ring buffer behind the system-status panel.

### 7.8 CSRF and origin discipline

State-changing requests must originate from the SPA's own origin. The session cookie's same-site posture covers the common cross-site case; an explicit origin/referer check and a CSRF token issued by the SPA at boot cover the residual surface. The credential-redaction discipline elsewhere in the document (per-feed cookies, basic-auth, outbound proxy URL) extends naturally: user-account credentials follow the same write-only contract — accepted on writes, never returned on reads.

### 7.9 Local-by-default still applies

Authentication is defence-in-depth, not the primary network boundary. The loopback-default bind remains the first line of defence on the binary deployment, and the container's network namespace plays the same role for the container deployment. Auth becomes the boundary only when an operator binds Tap to a non-loopback interface or exposes the container outside its host.

------------------------------------------------------------------------

## 8. Security and privacy

The threat model Tap defends against:

- a hostile feed serves malicious HTML or images — blocked by the sanitiser, the media proxy, and inbound size caps;
- a hostile feed redirects to localhost or a private CIDR — blocked by the SSRF guard with redirect re-check;
- a hostile feed returns an entity-expansion bomb — bounded by the parser's depth limit and the inbound size cap;
- a peer with LAN access scrapes credentials via the API — blocked by the credential-redaction invariant;
- a user pastes a feed URL with a private hostname — blocked by default, opt-in via the allowlist;
- a peer with LAN access guesses weak passwords against the API — mitigated by per-source and per-account rate limiting plus account lockout;
- a stolen session cookie is replayed from another browser — mitigated by idle and absolute session expiry and the user's session-revocation panel;
- a hostile site tries to ride a logged-in user's cookie via a cross-site request — mitigated by the cookie's same-site posture, the origin/referer check, and the CSRF token.

What Tap does not defend against:

- a hostile network operator MITMing outbound HTTPS;
- a co-tenant on the host reading the database file (and therefore recovering password hashes, TOTP secrets, and hashed session tokens);
- a user with shell access to the host.

The only personal data Tap holds is the subscription list, the read/saved state, and per-feed credentials. There is no analytics, no telemetry, no third-party request beyond what the user's feeds direct Tap to fetch.

The database is not encrypted at rest; operators who need that rely on filesystem-level encryption.

------------------------------------------------------------------------

## 9. Configuration

Configuration is read from three sources, merged with this precedence: command-line flags, then environment variables, then a YAML config file. The config file path itself is set by flag or environment variable; a missing file is not an error.

The set of knobs covers (without being exhaustive): database file path, listen address, log level and format, polling cadence and parallelism, the adaptive-polling multiplier, the per-host concurrency cap, outbound HTTP timeout and body cap, SSRF allowlist controls, the iframe host allowlist override, archival horizon, media cache directory / size cap / age cap / timeout / body cap, session idle and absolute timeouts, login rate-limit and lockout thresholds, and the first-launch admin-bootstrap environment variables.

Per-feed overrides live on the subscription row (user agent, cookie, basic-auth credentials, outbound proxy URL, HTTP/2 toggle, self- signed-cert toggle); defaults mean "use the global value."

------------------------------------------------------------------------

## 10. Operational shape

The release is a single OCI image; the image declares one volume for the database and the media cache, exposes one port, and runs as a non-root user. The image has no shell and no `curl`/`wget`, so the binary itself ships a `healthcheck` subcommand that the container's `HEALTHCHECK` invokes.

The binary also ships an `admin` subcommand family for offline account management — creating an account, resetting a password, disabling 2FA. The same code paths are used for the initial admin bootstrap and for admin self-recovery from the host.

Logs are structured to stdout. Every poll start, success, failure, and panic is logged with the feed identifier. There is no separate audit log table — operators forward stdout to whatever aggregator they use.

A small ring buffer of recent poll errors lives in process memory and is exposed via the system status endpoint for the SPA's settings panel. A panic in a worker is caught, recorded with a sentinel feed-id, the in-flight marker is cleared, and the worker continues.

Graceful shutdown drains in-flight HTTP requests with a deadline, lets workers finish their current polls, and closes the database.

------------------------------------------------------------------------

## 11. Observability

Tap is fully instrumented across the three standard observability pillars — logs, metrics, and traces — so an operator can answer "is it healthy, what is it doing, and why is this one thing slow" without attaching a debugger.

- **Logs.** Structured JSON to stdout, one event per line. Every poll start/success/failure, every authentication event, every archival sweep, every media-cache eviction pass, and every panic carries enough context (feed identifier, user identifier, request identifier, outcome, duration) to be filtered and aggregated downstream. There is no separate log file and no audit log table — operators forward stdout to whatever aggregator they use (Loki, ELK, a hosted service).
- **Metrics.** A small set of counters, gauges, and histograms exposed on a metrics endpoint in a widely-supported scrape format (Prometheus exposition is the obvious default). The set covers polling (polls dispatched, polls succeeded/failed by reason, poll duration, entries inserted, conditional-GET hit rate), HTTP (request rate, latency, status class), media proxy (cache hits/misses, evictions, bytes on disk), authentication (login attempts, lockouts), database (transaction duration, queue depth), and process health (goroutine/thread count, memory, uptime).
- **Traces.** Outbound HTTP requests, poll workflows, media proxy fetches, and inbound API requests are traced end to end so the latency of a slow poll can be attributed to DNS, TLS, the origin's response, sanitisation, extraction, or commit. Trace context propagates through the per-host concurrency cap and the shared HTTP client. Sampling is configurable, with errors always sampled.

The intended emission path is OpenTelemetry: OTLP for traces and metrics, with a bridge from the structured logger so logs, metrics, and traces share a common resource and request identifier. An operator who already runs Prometheus and Grafana can scrape the metrics endpoint directly and ignore the OTLP path; an operator who runs an OTel collector can fan all three signals out to whatever backend they prefer (Tempo, Jaeger, Honeycomb, a hosted SaaS). Tap takes no opinion on the backend.

Observability is opt-in at the edges: the metrics endpoint binds to loopback by default, OTLP exporters are off until configured, and trace sampling defaults to a low rate so a stock deployment incurs no measurable overhead. The system-status panel in the SPA is a thin built-in surface (recent errors, last poll times, version, uptime) for operators who do not run a separate observability stack.

Rationale: Tap runs unattended on heterogeneous self-hosted hardware. When something misbehaves the operator is often hours away from the host, so the diagnostic signal must already be in flight rather than reconstructable after the fact. Standardising on OpenTelemetry keeps Tap compatible with whatever the operator already runs without forcing a choice of backend.

------------------------------------------------------------------------

## 12. Brand and visual identity

**Name and metaphor.** *Tap* is the electrical-engineering term — a junction on a wire that draws a clean copy off a main line without interrupting it. The product aggregates many feeds into one chronological stream, exactly what a signal tap does. Avoid imagery suggesting faucets or plumbing.

**Junction dot.** A small filled dot at the intersection of two schematic lines is the load-bearing detail of the brand. The primary mark uses a T-junction (the letter T doubles as a schematic tap). The wordmark is lowercase `tap` in geometric sans-serif with the period rendered slightly heavier so it reads as a junction dot.

**Klein Blue.** Approximately `#002FA7`. Used sparingly: links, focus rings, the junction dot, the selection highlight. On dark themes it desaturates for AA contrast.

**Themes and typography.** Light, dark, sepia, and a system-tracking mode. A serif/sans toggle. Theme is a per-device preference (it lives in the browser, not the database) and switches without reload.

**Layout.** Content is the interface; chrome recedes. Single-column reading view, comfortable measure and line height, generous whitespace, sentence case throughout. No engagement metrics, no view counts, no share badges. Animation is minimal and functional. Iconography is line-based, thin strokes, rounded caps.

------------------------------------------------------------------------

## 13. Frontend behaviour surface

The SPA exposes a small set of routes:

- a login screen (password, with a passkey path for users who have one registered);
- an unread view (the home view, with multi-select, bulk mark- read, manual refresh, and keyboard navigation);
- a history view (newest-read-first, passive);
- a saved view (starred entries regardless of read state);
- a search view (URL-synced query, debounced, gated on minimum length);
- a settings view with sections for appearance (theme, font), account security (change password, enrol and remove passkeys, enrol and disable TOTP, regenerate recovery codes, list and revoke active sessions), and system status;
- (admin only) a user-management view (list users, create, disable, delete, reset password, disable 2FA);
- an add-feed flow (discover candidates from a URL, then subscribe);
- a per-feed detail view (recent entries, edit / refresh / delete with confirmation);
- a reader for a single entry (auto-mark-read on open, mobile swipe gestures for prev/next, keyboard bindings for read/save/open- original);
- a branded error page.

Keyboard bindings cover next / previous / open / toggle-read / toggle-saved / view-original / focus-search / hotkeys-modal / escape-modal, all suppressed inside form controls.

Mobile uses swipe gestures for prev/next in the reader and to mark as read in the unread view (with threshold and angle gates so vertical scrolling does not trigger them). Pull-to-refresh and infinite scroll are supported.

The SPA registers a service worker that caches the app shell, runs stale-while-revalidate against entry-list endpoints, and cache-firsts proxy URLs (which are immutable). Mutations and non-GET requests pass through the service worker. Navigation requests fall back to the SPA shell so the app boots offline.

A warm-cache driver runs at boot (deferred briefly so it does not fight the initial render) and on every reconnect: fetch a window of recent entries, extract the proxy URLs from each body, and warm the service worker's cache. Per-entry URL cap and modest concurrency keep this from saturating the network.

The SPA is also a Progressive Web App: standalone display mode, themed status bar, maskable icons.

------------------------------------------------------------------------

## 14. Inspirations

- **Miniflux** — single-binary architecture, adaptive polling, sanitisation, tombstones, privacy posture.
- **Readability-style extractors** — for the article-extraction pipeline that powers link-only feed support.
