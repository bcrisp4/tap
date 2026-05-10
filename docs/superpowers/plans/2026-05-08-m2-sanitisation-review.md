# M2 Sanitisation Plan — Skeptical Review

Review of `2026-05-08-m2-sanitisation.md`. Findings ranked blocker / serious / nit.

## Blockers

**1. Tasks 15 & 18 silently assume two unbuilt dependencies exist.**
- Step 15.1b uses `db.GetEntry(ctx, d, id)` to fetch the full entry. The plan never verifies this function exists. If it doesn't, the test won't compile and the task balloons.
- Step 18.2(b) does `mux.ServeHTTP` against `GET /api/v1/entries/{id}`. The plan never confirms this endpoint is wired up. If it isn't, you're suddenly extending the API in an M2 sanitisation task.

The CLAUDE.md says "API DTOs are explicit. internal/api/*.go defines per-endpoint DTOs" — but `internal/api/entries.go` may only have the list endpoint. **Verify both exist before starting.** If either is missing, surface it as a precursor task or change the assertion strategy (e.g. read directly from `db` instead of going through HTTP).

**2. Upgrade path is a security regression, not just a footgun.**
The README note ("delete tap.db") is the only mitigation against an operator running the new binary against an M1 DB. If they forget, the DB still holds raw HTML with `<script>`, `onclick=`, javascript: URLs, etc., and the SPA renders it directly per Task 19's "no runtime sanitiser" promise. The whole milestone gets bypassed silently.

A README warning is not a security control. Concrete alternatives (any of these is < 1 hour):
- (a) Stamp a `feature_flags` row at startup; refuse to start if the row is missing AND `entries` is non-empty, with an opt-in `--accept-m1-data` flag.
- (b) One-shot re-sanitise all existing entries on startup (idempotent — running again on already-clean HTML is fine).
- (c) Add migration `0003_purge_m1_entries.sql` that truncates `entries` (subscriptions survive). Crude but bulletproof.

This belongs in the spec, not just the README. **Pick one before writing code.**

## Serious

**3. No observability anywhere.** Not a single `slog.Debug` for "stripped N iframes" or "dropped pixel tracker" or "truncated 1.3 MiB input". When a user reports "X feed looks broken in M2", the operator has no signal. CLAUDE.md doesn't ban observability — this is a plain miss. Adding three log lines to `Sanitise` (one for truncation, one summary count after walk, one for dropped iframes) costs ten minutes.

**4. Redundant defensive defaulting.** Both `NewWorker` (Task 15.3) and `NewScheduler` (Task 16.1) default `Policy` to `DefaultPolicy()` if nil. Pick one. The scheduler's default is the production path; the worker's default is dead code that hides bugs (a test that forgets to pass a Policy will look like it's working). Drop the worker-level default; let `NewWorker` panic or doc-comment "Policy must be non-nil."

**5. `bluemonday.Policy` thread-safety is assumed, not verified.** The scheduler constructs one Policy and shares it across N=3 worker goroutines (Task 16.1). bluemonday's docs say `Policy.Sanitize` is safe for concurrent use after construction — but the plan doesn't verify it, and if it's wrong you'll get rare data corruption that won't show up in the existing tests (which are sequential). Add a `-race` test that hits one Policy from N goroutines on hostile input.

**6. `bluemonday.AllowAttrs(...).OnElements("iframe")` may not actually whitelist `<iframe>`.** The plan assumes attribute-allowlisting an element implicitly allows the element. This is true in bluemonday's current API, but it's an undocumented contract for a security-critical tool. If it changes (or you're wrong), Task 10's "iframe allowed through bluemonday" comment is a lie and your post-pass walker never sees iframes — UGCPolicy strips them all upstream. The Task 10 step-4 test would catch it, but if it fails, you'll spend an hour debugging the wrong layer. Add an explicit `bm.AllowElements("iframe")` for belt + braces.

**7. URL cleaning is incomplete.** Task 12 only handles `<a href>` and `<img src>`. UGCPolicy permits at minimum: `<source srcset>`, `<source src>`, `<video poster>`, `<video src>`, `<audio src>`, `<area href>`. Tracking params in any of these survive. Either extend the walker (cheap — same `cleanAttrURL` helper) or doc the limitation in `internal/sanitise/sanitise.go` so future-you doesn't think it's covered.

**8. Pre-sanitise byte cap can produce mid-codepoint cut.** Task 13 does `rawHTML[:MaxInputBytes]` on a string. If position 1<<20 is mid-UTF-8, bluemonday gets invalid UTF-8 input. The HTML parser is lax enough to recover, but `html.Render` may then produce a `&#xFFFD;` or weird byte. Cheap fix: snap to a UTF-8 boundary using `utf8.ValidString` walk-back, or use `bytes.LastIndex` on a recognisable separator. Five lines.

**9. Pixel-tracker heuristic misses common evasions.** Task 11 only checks `width="1"` / `height="1"` attributes. CSS-styled trackers (`<img style="width:1px;height:1px">`), 2×2 pixels, and naked `<img src="https://t.example/p">` (no dims, browser sizes to 1×1 if the image is 1×1) all survive. The plan's notes say not to expand beyond miniflux's heuristic — fine, but document this as a known gap in the package doc comment so it isn't mistaken for the policy.

## Nits

**10. Task 7 — `urlcleaner.Clean("javascript:alert(1)")` returns the input unchanged.** Correct, because there's no query string. But the test name is "NonURLReturnsUnchanged" and `javascript:alert(1)` is in fact a parseable URL. Rename or doc that this URL relies on bluemonday upstream for scheme defence — `urlcleaner` is not a security boundary.

**11. Task 13 slack of `+1024` for output cap is unjustified.** Why 1024? Tag wrapping won't add that much. Pick a defensible number (256?) or drop the slack and tighten the assertion.

**12. Task 20 step 3 smoke-tests against live HN RSS.** Flaky — if HN is down or HN's feed format trips a parser bug, your manual smoke fails. Use a local httptest fixture, or mark the step explicitly optional and don't run it from CI.

**13. URL re-encoding can change bytes even when nothing is stripped.** `url.URL.String()` re-encodes paths (`%20` ↔ `+`, normalised escapes). The early-return-when-`!changed` saves you for the no-tracking case, but if a tracking param is stripped, the rest of the URL is also re-encoded. Probably fine, but worth one comment in the code.

**14. `m.youtube.com`, `youtu.be` are denied.** Real YouTube domains. Test 10 explicitly asserts `m.youtube.com` is denied. Spec choice, fine — but worth a TODO comment pointing at the per-user override deferred item, otherwise this looks like a bug to a future reader.

## Estimate reality-check

The plan claims "2–5 minute" steps. Realistic per task: 5–15 minutes if everything compiles first try, 30+ if not. Tasks 10 (post-pass walker), 15 (worker integration), and 18 (E2E) are the long poles. Total range: **3–6 hours focused work**, not the ~100 min the per-step pacing implies.

---

## Top 3 things to fix before any code is written

1. **Verify `db.GetEntry` and `GET /api/v1/entries/{id}` exist** (Task 15 & 18 dependency). If either is missing, restructure the plan — the e2e and worker tests need a way to read entry content back.
2. **Decide upgrade-path policy in the spec, not the README.** Pick from: startup feature-flag check, on-startup re-sanitise migration, or destructive `entries` purge migration. README warnings are not a security control.
3. **Add observability to `Sanitise`.** Three slog lines (truncation, dropped iframes, dropped pixel trackers). Without these, M2 is undebuggable in production.
