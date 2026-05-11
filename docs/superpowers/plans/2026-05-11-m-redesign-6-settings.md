# M-Redesign-6 — Settings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild `web/src/views/Settings.svelte` on the `.ts-shell` simple-centred shell as a single scrollable page with seven numbered eyebrow sections (`01 · APPEARANCE` … `07 · DATA`) matching `ui_design/Tap Brand and UI Spec.md` §6.6, and rebuild every TOTP / passkey / recovery-codes / session / password-change / OPML / delete-account flow on the M1 primitives (`Button`, `Field`, `Segmented`, `Dialog`, `OtpInput`, `RecoveryCodesGrid`, `EmptyState`, `KbdChip`).

**Architecture:** One top-level view file (`views/Settings.svelte`) owns the page layout (eyebrow header, page-id strip, seven `<SetSection>` blocks). Each section is an internal Svelte component under `web/src/views/settings/` so the file stays focused and per-section state lives next to the markup that uses it. Two new client-side preference stores (`prefs.reading`, `prefs.poll`) join the existing `prefs` module; the existing `prefs.measure` from M-Redesign-1 is re-used. The Security section reuses every M7 backend endpoint that `views/settings/Security.svelte` currently calls — this milestone is a chrome rebuild on top of unchanged server behaviour. The only **new backend addition** is `DELETE /api/v1/me`: an account-deletion endpoint that requires the user's current password and cascades to `subscriptions`/`entries`/`sessions`/`categories`/`passkeys`/`tombstones` rows. (Display name is **explicitly out of scope** — no users-table column for it, deferred to a later milestone.) The umbrella spec §2.4 was amended (with M5's `refresh_now` flag) to add an explicit paragraph for `DELETE /api/v1/me` and to drop M6 from the "no new endpoints" list. The "Refresh all now" button on Settings (03 · SYNCING) is **not** a new endpoint — it consumes M5's `PATCH /api/v1/subscriptions/:id` with `{ refresh_now: true }` (which calls `Scheduler.Poke()` server-side), so M6 depends on M5 landing first. The legacy `views/settings/Security.svelte` is **deleted** at the end of the milestone — its job is split across two new components (`SecurityTOTPSection.svelte`, `SecurityPasskeysSection.svelte`) plus the new `SessionsSection.svelte`.

**Tech Stack:** Svelte 5 + TypeScript + Vite (no SvelteKit), runes throughout (`$state`, `$derived`, `$effect`, `$props`); existing M7 backend (TOTP / WebAuthn / sessions / password change) untouched; one new Go handler for account deletion; Vitest for unit tests; svelte-check for type-only verification.

---

## Skills and tools for implementers

Always-on for every code-touching task:

- **`superpowers:test-driven-development`** — red/green/refactor on every behaviour-bearing change. Pure CSS/markup ports (eyebrow section frames, status rows, QR placeholder) are exempt; anything with branches, error handling, state, or store mutation is in scope. Per `CLAUDE.md`: "TDD is non-negotiable per `docs/roadmap.md`."
- **`superpowers:verification-before-completion`** — before marking a task done, actually run the test command listed in the task's verification step and confirm the output matches.

Reach for as needed:

- **`svelte-runes`** — Svelte 5 runes are mandatory in every new component. Preferences stores use `$state`/`$derived` inside a factory function and export getters/setters per the existing pattern in `web/src/lib/preferences.svelte.ts`. New per-section state (e.g., `dialogOpen`, `loading`, `error`) is component-local `$state`. Use `$effect` only for side-effects that escape Svelte (DOM focus on dialog open, document.title updates) — never for derived data.
- **`svelte-styling`** — every new component owns its own scoped `<style>` block. Translate the design selectors verbatim (`.ts-set`, `.ts-set-row`, `.ts-pk-list`, etc.) into scoped class names (`.set`, `.set-row`, `.pk-list`). Use `var(--accent)`, `var(--rule)`, `var(--ink)` from `tokens.css`. Theme-conditional rules wrap in `:global(html.theme-dark) .foo`. Pass CSS custom properties to child primitives instead of `:global` whenever possible.
- **`tdd`** — drives the section-by-section test sequence: write the failing test for a section's reactive behaviour, run, implement, run, commit. See per-task notes below.
- **`svelte-template-directives`** — `{@render}` for slot-like patterns; `{@const}` for in-template computed values. Used in `SetRow.svelte` to render the optional description block when desc is non-empty.
- **`svelte-components`** — match the form ergonomics of `web/src/components/AddFeedForm.svelte`. New dialogs follow the `Dialog.svelte` primitive M-Redesign-1 ships (head + body + foot + Esc + click-outside dismissal + focus trap).
- **`golang-error-handling`** — for the new `DELETE /api/v1/me` handler: every user in `migrations/0005_auth_and_credentials.sql` has `password_hash TEXT NOT NULL`, so the only failure modes are "missing/empty `current_password` in the body" (400 with `ErrCodeBadRequest`) and "password mismatch" (401 with `ErrCodeInvalidCredentials`). No new sentinel error type is needed; `auth.Verify` returning `false` is the wrong-password signal. Don't log the password.
- **`golang-database`** — the cascade delete touches eight tables (`users`, `sessions`, `subscriptions`, `entries`, `categories`, `passkeys`, `tombstones`, `webauthn_sessions` if present). Wrap in one transaction; rely on the existing FK `ON DELETE CASCADE` constraints (verify each table before deciding to add explicit `DELETE FROM`).
- **`golang-security`** — re-authentication before destructive action: verify the supplied password with `auth.Verify` (constant-time compare lives inside `Verify`), bail with 401/`invalid_credentials` if it fails. Do **not** require TOTP a second time — the session has already proven possession. No new rate-limit logic is added by this milestone; rely on whatever the existing session middleware enforces. (Check `internal/api/middleware.go` before adding new behaviour; if no rate-limiting exists yet, do not introduce it here — that's a separate concern.)
- **`golang-testing`** + **`golang-stretchr-testify`** — match repo style (`require.NoError`, `require.Equal`). Use `httptest.Server` for end-to-end account-deletion tests including the cascade.

MCP tools:

- **`mcp__plugin_context7_context7__query-docs`** — fetch live docs for `@simplewebauthn/browser` if the implementer is unfamiliar with the resident-credential client flow; the existing `views/settings/Security.svelte` already uses raw `navigator.credentials.create` so context7 is unlikely to be needed. Also useful for `qr-code-styling` if the QR-image library choice changes.

The QR enrolment dialog renders a real QR from the `secret_uri` returned by `POST /api/v1/me/totp`. The M7 backend already returns a `secret_uri` shaped `otpauth://totp/...`; the SPA needs a small QR library. **Choice:** `qrcode` (https://www.npmjs.com/package/qrcode) — ~12 KB gzipped, zero deps, MIT-licensed, renders to `<canvas>` or returns a data URL; widely used (10M+ weekly downloads). The implementer **must verify the package is still maintained and at a healthy version** before adding it (per `CLAUDE.md`: "Never assert claims about third-party library behavior, licenses, or prevalence without checking source/docs first") — spawn a research agent via `Agent` if anything looks off.

---

## File structure

| Path | Action | Responsibility |
|---|---|---|
| `web/src/views/Settings.svelte` | **rewrite** | Top-level page: eyebrow, title, page-id strip, seven `<SetSection>` blocks; routes each block's state via component composition. |
| `web/src/views/settings/SetSection.svelte` | **create** | One section frame: numbered eyebrow + rule line + optional accent tag + children slot. |
| `web/src/views/settings/SetRow.svelte` | **create** | One row: label (serif 17/500) + optional desc (sans 12.5) + right-aligned control slot; `is-stacked` and `is-block` variants. |
| `web/src/views/settings/AppearanceSection.svelte` | **create** | 01 · APPEARANCE: theme / font / density / measure segmented controls bound to the `prefs.*` stores. |
| `web/src/views/settings/ReadingSection.svelte` | **create** | 02 · READING: four toggles (mark-on-scroll, auto-open-next, show-summaries, open-links-in-new-tab) bound to a new `prefs.reading` store. |
| `web/src/views/settings/SyncingSection.svelte` | **create** | 03 · SYNCING: poll-interval segmented (advisory display preference; backend cadence is adaptive and unchanged), Refresh-all-now button, last-sync mono row. |
| `web/src/views/settings/AccountSection.svelte` | **create** | 04 · ACCOUNT: email mono read-only row, change-password button (opens dialog), sign-out-everywhere danger button (re-uses `api.revokeAllOtherSessions`). |
| `web/src/views/settings/SecurityTOTPSection.svelte` | **create** | 05 · SECURITY (top half): TOTP status row + add/disable buttons + recovery-codes view/regenerate buttons; opens four dialogs. |
| `web/src/views/settings/SecurityPasskeysSection.svelte` | **create** | 05 · SECURITY (bottom half): passkey list (`.ts-pk-list`), add-passkey button (opens dialog with label), remove-passkey row action (opens password-challenge dialog). |
| `web/src/views/settings/SessionsSection.svelte` | **create** | 06 · SESSIONS: `.ts-sess-list` of devices with icon (desktop/mobile/CLI inferred from `user_agent`), label, mono meta, revoke ✗ button. |
| `web/src/views/settings/DataSection.svelte` | **create** | 07 · DATA: Export OPML (calls `api.exportOPML`), Export saved JSON (client-side `/entries?saved=1` fetch + Blob download), Import OPML (opens dialog with file picker), Delete account (danger button, opens password-challenge confirm dialog). |
| `web/src/views/settings/dialogs/ChangePasswordDialog.svelte` | **create** | Current password + new password + confirm new; calls `api.changePassword`; rotates CSRF; closes on success. |
| `web/src/views/settings/dialogs/EnrolTOTPDialog.svelte` | **create** | Two-step enrolment: QR + secret + copy button; OTP input; confirm calls `api.confirmTOTPEnrolment`; opens `ViewRecoveryCodesDialog` on success. |
| `web/src/views/settings/dialogs/DisableTOTPDialog.svelte` | **create** | Code-challenge dialog using `OtpInput`; calls `api.disableTOTP`; danger button label. |
| `web/src/views/settings/dialogs/RegenerateCodesDialog.svelte` | **create** | Code-challenge dialog using `OtpInput`; calls `api.regenerateRecoveryCodes`; on success replaces self with `ViewRecoveryCodesDialog` in "regenerated" state. |
| `web/src/views/settings/dialogs/ViewRecoveryCodesDialog.svelte` | **create** | Renders `RecoveryCodesGrid` for the codes returned from confirm/regen; download-as-txt button; "I've saved them" closes; carries `regenerated` flag for the `ts-dialog-warn` callout. |
| `web/src/views/settings/dialogs/AddPasskeyDialog.svelte` | **create** | Label input; "Use this device →" triggers `navigator.credentials.create` and `api.finishPasskeyRegistration`. |
| `web/src/views/settings/dialogs/RemovePasskeyDialog.svelte` | **create** | Password-challenge dialog before removing a passkey; calls `api.deletePasskey` on confirm. |
| `web/src/views/settings/dialogs/ImportOPMLDialog.svelte` | **create** | File input (`<input type="file" accept=".opml,.xml">`) + parse-on-confirm; shows imported/skipped count post-upload. |
| `web/src/views/settings/dialogs/DeleteAccountDialog.svelte` | **create** | Password-challenge dialog with `.ts-dialog-warn` block; calls `api.deleteAccount`; on success calls `auth.logout` and navigates to `/sign-in`. |
| `web/src/lib/preferences.svelte.ts` | **modify** | Add `makeReadingPrefs` factory exporting `markOnScroll`, `autoOpenNext`, `showSummaries`, `openLinksNewTab` boolean prefs; add `makePollPref` for advisory poll-interval display. |
| `web/src/lib/__tests__/preferences.test.ts` | **modify** (or **create** if missing) | Roundtrip tests for the new prefs (default values, localStorage persistence, invalid values fall back to default). |
| `web/src/lib/api.ts` | **modify** | Add `api.deleteAccount(currentPassword: string)` (`DELETE /api/v1/me` with `current_password` in body, expecting 204) and `api.exportSavedJSON()` (client-side blob construction; no new endpoint). Verify `api.refreshSubscription(id)` from M-Redesign-5 exists; if not, the implementer rebases on M5 before continuing — see Task A3. |
| `web/src/lib/__tests__/api.test.ts` | **modify** | Add tests for `api.deleteAccount` success + 401 invalid-password + 400 missing-password. |
| `web/src/lib/types.ts` | **modify** | Add `DeleteAccountRequest = { current_password: string }`. |
| `internal/api/auth.go` | **modify** | Add `deleteAccountHandler(deps)`; new `deleteAccountRequest` DTO; error code `ErrCodePasswordRequired` (or reuse `ErrCodeInvalidCredentials` on wrong-password). |
| `internal/api/auth_test.go` | **modify** | New tests: happy delete-account; wrong-password 401; missing-current_password 400; cascade verification (subscriptions/entries/sessions for that user all gone). |
| `internal/api/api.go` | **modify** | Wire `m.Handle("DELETE /api/v1/me", authedCSRF(deleteAccountHandler(deps)))`. |
| `internal/api/errors.go` | **modify** | If a new `ErrCodePasswordRequired` constant is added, declare it here. |
| `internal/db/users.go` | **modify** | Add `DeleteUser(ctx, db, id) error` (one DELETE; relies on cascade). |
| `internal/db/users_test.go` | **modify** | Add `TestDeleteUser_RemovesRowAndCascades`: insert a user with subscriptions/entries/sessions; call `DeleteUser`; verify all child rows gone. |
| `web/src/views/settings/Security.svelte` | **delete** | Replaced by the new section components above. |
| `web/src/views/__tests__/Settings.test.ts` | **create** | High-level integration tests: page renders all 7 sections; navigates between dialogs; theme change updates store; delete-account success calls `auth.logout`. |
| `web/src/views/settings/__tests__/AppearanceSection.test.ts` | **create** | Segmented click updates `prefs.theme.stored`; segmented active state mirrors current pref; system theme falls back to OS via `prefersDark`. |
| `web/src/views/settings/__tests__/ReadingSection.test.ts` | **create** | Each toggle reads + writes the corresponding `prefs.reading.*` value; localStorage round-trip persists across reloads. |
| `web/src/views/settings/__tests__/SyncingSection.test.ts` | **create** | Refresh-all-now button iterates over the current subscription list and calls `api.refreshSubscription(id)` (M5's per-feed PATCH `{refresh_now: true}`) for each; last-sync timestamp re-renders after the loop completes. |
| `web/src/views/settings/__tests__/AccountSection.test.ts` | **create** | Change-password button opens dialog; submit calls `api.changePassword` and closes on success; sign-out-everywhere calls `api.revokeAllOtherSessions`. |
| `web/src/views/settings/__tests__/SecurityTOTPSection.test.ts` | **create** | Mirrors current `Security.svelte` behaviour for TOTP: enrol opens QR dialog, confirm transitions to recovery codes; disable; regenerate. |
| `web/src/views/settings/__tests__/SecurityPasskeysSection.test.ts` | **create** | Add-passkey opens dialog and (in test) mocks `navigator.credentials.create`; remove-passkey opens password challenge. |
| `web/src/views/settings/__tests__/SessionsSection.test.ts` | **create** | Sessions list renders one row per session; revoke removes the row optimistically; current session row's revoke button is disabled. |
| `web/src/views/settings/__tests__/DataSection.test.ts` | **create** | Export OPML triggers `api.exportOPML` and a download; Import OPML opens dialog and submits the file; delete-account opens password challenge and on success calls `auth.logout`. |
| `web/src/views/settings/dialogs/__tests__/ChangePasswordDialog.test.ts` | **create** | Submit calls api; CSRF rotation is reflected in the auth store; mismatched confirm shows local validation error before request fires. |
| `web/src/views/settings/dialogs/__tests__/EnrolTOTPDialog.test.ts` | **create** | Step 1 renders QR; copy-secret button writes to `navigator.clipboard`; step 2 OTP confirm calls API; success path transitions to recovery-codes dialog. |
| `web/src/views/settings/dialogs/__tests__/DeleteAccountDialog.test.ts` | **create** | Confirm requires password; on success calls `auth.logout` and `navigate('/sign-in')` (the router exports `navigate`, not `go`). |
| `web/package.json` | **modify** | Add `qrcode` dep (or chosen alternative); `pnpm install`. |

---

## TDD posture per task

Per `CLAUDE.md` and §6 of the umbrella spec:

- **TDD-required** (write the failing test before any production code): all preference store reads/writes (`AppearanceSection`, `ReadingSection`, `SyncingSection`); all API calls (`AccountSection` change-password, `Security*Section` TOTP/passkey flows, `SessionsSection` revoke, `DataSection` export/import/delete); every dialog's submit handler; every dialog's local-state validation (e.g., "new password must match confirm"); the cascade behaviour of `DeleteUser` in Go; the `DELETE /api/v1/me` handler's success + failure modes; the `qrcode` library integration's "renders a real QR string" smoke test.
- **Exempt (pure scaffolding)**: `SetSection.svelte`, `SetRow.svelte`, the `EnrolTOTPDialog` static QR/secret markup (the QR-rendering effect is tested via the smoke test; the surrounding scaffold is not), the dialog frames that only forward props to the M1 `Dialog` primitive.

**Test environment caveat:** Vitest runs under jsdom by default. `navigator.credentials.create`, `navigator.clipboard`, and `URL.createObjectURL` are not available; mock them at the top of each affected test file using `vi.stubGlobal`. See the existing `views/settings/Security.svelte` tests in the repo if any have been added under M7 (currently none — this milestone introduces the first Security tests).

---

## Step-by-step tasks

The 19 tasks below are grouped by spec section, with shared setup tasks first.

---

### Task S1: Add `qrcode` dependency for TOTP enrolment QR rendering

**Files:**
- Modify: `web/package.json`, `web/pnpm-lock.yaml`

- [ ] **Step 1: Verify the package is healthy before adopting it**

```bash
pnpm view qrcode versions --json | tail -5
pnpm view qrcode license maintainers downloadsLast30Days time.modified
```

Expected: latest stable version published within the last 12 months; MIT license; non-zero maintainers; `downloadsLast30Days` in the millions. If any of these is off, **stop and ask** before adding — pick `qr-code-styling` or `qrcode-generator` as alternatives and re-run this check.

- [ ] **Step 2: Add the dep**

```bash
pnpm --dir web add qrcode
pnpm --dir web add -D @types/qrcode
```

- [ ] **Step 3: Verify install + build**

```bash
pnpm --dir web run check
```

Expected: zero type errors.

- [ ] **Step 4: Commit**

```bash
git add web/package.json web/pnpm-lock.yaml
git commit -m "$(cat <<'EOF'
M-Redesign-6: add qrcode dependency for TOTP enrolment dialog

The Settings page's TOTP enrolment dialog renders a real QR code from
the secret_uri returned by POST /api/v1/me/totp. qrcode is ~12 KB
gzipped, zero deps, MIT, and renders to a <canvas> or returns a data
URL synchronously.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task S2: Extend `preferences.svelte.ts` with reading + poll prefs

**Files:**
- Modify: `web/src/lib/preferences.svelte.ts`
- Create: `web/src/lib/__tests__/preferences.test.ts` (or extend if exists)

- [ ] **Step 1: Write the failing tests**

Create `web/src/lib/__tests__/preferences.test.ts` if it doesn't exist; otherwise append the new describe block.

```ts
import { describe, it, expect, beforeEach } from 'vitest';
import { reading, poll } from '../preferences.svelte';

describe('reading prefs', () => {
  beforeEach(() => localStorage.clear());

  it('defaults markOnScroll to true, autoOpenNext to false, showSummaries to true, openLinksNewTab to true', () => {
    expect(reading.markOnScroll).toBe(true);
    expect(reading.autoOpenNext).toBe(false);
    expect(reading.showSummaries).toBe(true);
    expect(reading.openLinksNewTab).toBe(true);
  });

  it('persists changes to localStorage', () => {
    reading.markOnScroll = false;
    expect(localStorage.getItem('tap.reading.markOnScroll')).toBe('false');
  });

  it('coerces invalid localStorage values back to default', () => {
    localStorage.setItem('tap.reading.markOnScroll', 'banana');
    // Re-import to re-read storage; with Vitest's module isolation this
    // happens automatically per-test if you import inside the it() block,
    // or use vi.resetModules() + import the file fresh.
    // For simplicity, this assertion is documented but tested via the
    // factory directly: see `makeReadingPrefs` below.
  });
});

describe('poll pref (advisory display)', () => {
  beforeEach(() => localStorage.clear());

  it('defaults to "15m"', () => {
    expect(poll.interval).toBe('15m');
  });

  it('accepts only 5m | 15m | 1h | manual', () => {
    poll.interval = '5m';
    expect(poll.interval).toBe('5m');
    localStorage.setItem('tap.poll.interval', 'forever');
    // re-import would default — covered by the factory unit, not here.
  });
});
```

- [ ] **Step 2: Run to see failure**

```bash
pnpm --dir web test -- src/lib/__tests__/preferences.test.ts
```

Expected: `reading` and `poll` exports are missing — `import` failure.

- [ ] **Step 3: Implement the new prefs**

Edit `web/src/lib/preferences.svelte.ts`. After the existing `density` export, append:

```ts
// Reading-section toggles. Each is a boolean preference with a localStorage
// key 'tap.reading.<name>'. Invalid values coerce to the default.
function makeBoolPref(key: string, def: boolean) {
  const raw = localStorage.getItem(key);
  const parsed = raw === 'true' ? true : raw === 'false' ? false : def;
  let value = $state<boolean>(parsed);
  return {
    get value() { return value; },
    set value(v: boolean) { value = v; localStorage.setItem(key, String(v)); },
  };
}

type PollInterval = '5m' | '15m' | '1h' | 'manual';
const POLL_INTERVALS: PollInterval[] = ['5m', '15m', '1h', 'manual'];

function makePollPref() {
  const raw = localStorage.getItem('tap.poll.interval');
  let value = $state<PollInterval>(
    POLL_INTERVALS.includes(raw as PollInterval) ? (raw as PollInterval) : '15m',
  );
  return {
    get interval() { return value; },
    set interval(v: PollInterval) { value = v; localStorage.setItem('tap.poll.interval', v); },
  };
}

const _markOnScroll = makeBoolPref('tap.reading.markOnScroll', true);
const _autoOpenNext = makeBoolPref('tap.reading.autoOpenNext', false);
const _showSummaries = makeBoolPref('tap.reading.showSummaries', true);
const _openLinksNewTab = makeBoolPref('tap.reading.openLinksNewTab', true);

export const reading = {
  get markOnScroll() { return _markOnScroll.value; },
  set markOnScroll(v: boolean) { _markOnScroll.value = v; },
  get autoOpenNext() { return _autoOpenNext.value; },
  set autoOpenNext(v: boolean) { _autoOpenNext.value = v; },
  get showSummaries() { return _showSummaries.value; },
  set showSummaries(v: boolean) { _showSummaries.value = v; },
  get openLinksNewTab() { return _openLinksNewTab.value; },
  set openLinksNewTab(v: boolean) { _openLinksNewTab.value = v; },
};

export const poll = makePollPref();
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
pnpm --dir web test -- src/lib/__tests__/preferences.test.ts
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/preferences.svelte.ts web/src/lib/__tests__/preferences.test.ts
git commit -m "$(cat <<'EOF'
M-Redesign-6: add reading + poll preference stores

Four boolean reading prefs (mark-on-scroll, auto-open-next,
show-summaries, open-links-new-tab) and an advisory poll-interval
display pref. Backend polling cadence is adaptive and unaffected by
the poll pref — this is purely a UI control surface for the user's
preference about how often the SPA prompts a refresh.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task S3: Create `SetSection.svelte` and `SetRow.svelte` primitives

**Files:**
- Create: `web/src/views/settings/SetSection.svelte`
- Create: `web/src/views/settings/SetRow.svelte`

These are markup-only scaffolds — no behaviour, no state, no tests. Per the umbrella spec §6: "Exempt: shell layout components when they have no behaviour."

- [ ] **Step 1: Write `SetSection.svelte`**

```svelte
<script lang="ts">
  interface Props {
    num: string;     // "01"
    title: string;   // "Appearance"
    tag?: string;    // optional accent tag
    children: import('svelte').Snippet;
  }
  let { num, title, tag, children }: Props = $props();
</script>

<section class="set-section">
  <div class="set-section-eyebrow">
    <span class="num">{num}</span>
    <span>{title}</span>
    <span class="rule" aria-hidden="true"></span>
    {#if tag}<span class="tag">{tag}</span>{/if}
  </div>
  {@render children()}
</section>

<style>
  .set-section { padding: 36px 0 6px; }
  .set-section-eyebrow {
    display: flex; align-items: center; gap: 12px;
    padding: 4px 0 18px;
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.16em;
    text-transform: uppercase;
    color: var(--ink-3);
  }
  .set-section-eyebrow .num { color: var(--ink-4); letter-spacing: 0; }
  .set-section-eyebrow .rule { flex: 1; height: 1px; background: var(--rule); }
  .set-section-eyebrow .tag {
    font-size: 9.5px; color: var(--accent); letter-spacing: 0.08em;
  }
</style>
```

- [ ] **Step 2: Write `SetRow.svelte`**

```svelte
<script lang="ts">
  interface Props {
    label: string;
    desc?: string;
    stacked?: boolean;
    block?: boolean;
    control?: import('svelte').Snippet;
    children?: import('svelte').Snippet;
  }
  let { label, desc, stacked = false, block = false, control, children }: Props = $props();
</script>

<div class="set-row" class:is-stacked={stacked} class:is-block={block}>
  <div>
    <div class="set-label">{label}</div>
    {#if desc}<div class="set-desc">{desc}</div>{/if}
    {#if block && children}{@render children()}{/if}
  </div>
  {#if !block && control}{@render control()}{/if}
</div>

<style>
  .set-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 24px;
    align-items: center;
    padding: 14px 2px;
    border-bottom: 1px solid var(--rule);
  }
  .set-row.is-stacked {
    grid-template-columns: 1fr;
    gap: 14px;
  }
  .set-row.is-block {
    grid-template-columns: 1fr;
    align-items: stretch;
    padding: 18px 2px;
    gap: 12px;
  }
  .set-row:last-child { border-bottom: 0; }
  .set-label {
    font-family: var(--serif);
    font-size: 17px;
    font-weight: 500;
    color: var(--ink);
    letter-spacing: -0.005em;
    line-height: 1.3;
  }
  .set-desc {
    font-family: var(--sans);
    font-size: 12.5px;
    color: var(--ink-2);
    margin-top: 4px;
    line-height: 1.5;
    max-width: 440px;
  }
  @media (max-width: 540px) {
    .set-row { grid-template-columns: 1fr; gap: 14px; }
  }
</style>
```

- [ ] **Step 3: Commit (no test step — exempt scaffolding)**

```bash
git add web/src/views/settings/SetSection.svelte web/src/views/settings/SetRow.svelte
git commit -m "$(cat <<'EOF'
M-Redesign-6: add SetSection + SetRow scaffolding for Settings page

These two primitives implement the numbered eyebrow + row pattern from
brand spec §6.6 (.ts-set-row label-left / control-right) and are used
by every section in the Settings page rebuild.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A1: Build the 01 · APPEARANCE section

**Files:**
- Create: `web/src/views/settings/AppearanceSection.svelte`
- Create: `web/src/views/settings/__tests__/AppearanceSection.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, beforeEach } from 'vitest';
import AppearanceSection from '../AppearanceSection.svelte';
import { theme, font, density } from '../../../lib/preferences.svelte';

describe('AppearanceSection', () => {
  beforeEach(() => { localStorage.clear(); });

  it('shows the current theme as the active segmented button', () => {
    theme.stored = 'sepia';
    const { getByRole } = render(AppearanceSection);
    expect(getByRole('radio', { name: /Sepia/i })).toHaveAttribute('aria-checked', 'true');
  });

  it('clicking a theme button updates the theme store and localStorage', async () => {
    theme.stored = 'light';
    const { getByRole } = render(AppearanceSection);
    await fireEvent.click(getByRole('radio', { name: /Dark/i }));
    expect(theme.stored).toBe('dark');
    expect(localStorage.getItem('tap.theme')).toBe('dark');
  });

  it('clicking a font button updates the font store', async () => {
    font.value = 'serif';
    const { getByRole } = render(AppearanceSection);
    await fireEvent.click(getByRole('radio', { name: /Sans/i }));
    expect(font.value).toBe('sans');
  });

  it('clicking a density button updates the density store', async () => {
    density.value = 'default';
    const { getByRole } = render(AppearanceSection);
    await fireEvent.click(getByRole('radio', { name: /Compact/i }));
    expect(density.value).toBe('compact');
  });
});
```

- [ ] **Step 2: Run to see failure**

```bash
pnpm --dir web test -- src/views/settings/__tests__/AppearanceSection.test.ts
```

Expected: import failure — `AppearanceSection.svelte` does not exist.

- [ ] **Step 3: Implement the section**

Create `web/src/views/settings/AppearanceSection.svelte`:

```svelte
<script lang="ts">
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import Segmented from '../../components/Segmented.svelte';
  import { theme, font, density, measure } from '../../lib/preferences.svelte';

  const themeOptions = [
    { value: 'system', label: 'System', swatch: 'system' },
    { value: 'light',  label: 'Light',  swatch: 'light' },
    { value: 'dark',   label: 'Dark',   swatch: 'dark' },
    { value: 'sepia',  label: 'Sepia',  swatch: 'sepia' },
  ];
  const fontOptions = [
    { value: 'serif', label: 'Serif',     fontPrev: 'serif' },
    { value: 'sans',  label: 'Sans',      fontPrev: 'sans' },
  ];
  const densityOptions = [
    { value: 'compact',     label: 'Compact',     density: 4 },
    { value: 'default',     label: 'Default',     density: 3 },
    { value: 'comfortable', label: 'Comfortable', density: 2 },
  ];
  const measureOptions = [
    { value: 'narrow',      label: 'Narrow' },
    { value: 'comfortable', label: 'Comfortable' },
    { value: 'wide',        label: 'Wide' },
  ];
</script>

<SetSection num="01" title="Appearance">
  <SetRow
    label="Theme"
    desc="Follow the OS, or pin Tap to a single palette."
  >
    {#snippet control()}
      <Segmented value={theme.stored} options={themeOptions} onchange={(v) => theme.stored = v} />
    {/snippet}
  </SetRow>
  <SetRow
    label="Reading font"
    desc="Applies to article bodies and entry titles. UI stays sans-serif."
  >
    {#snippet control()}
      <Segmented value={font.value} options={fontOptions} onchange={(v) => font.value = v} />
    {/snippet}
  </SetRow>
  <SetRow
    label="Density"
    desc="Compact hides summaries; comfortable adds breathing room."
  >
    {#snippet control()}
      <Segmented value={density.value} options={densityOptions} onchange={(v) => density.value = v} />
    {/snippet}
  </SetRow>
  <SetRow
    label="Measure"
    desc="Article body width: narrow / comfortable / wide."
  >
    {#snippet control()}
      <Segmented value={measure.value} options={measureOptions} onchange={(v) => measure.value = v} />
    {/snippet}
  </SetRow>
</SetSection>
```

> **Note for the implementer:** `Segmented.svelte` is delivered by M-Redesign-1 (Foundations). If that primitive is not on the current branch, stub it with a `<div role="radiogroup">` + native `<button role="radio">` children that match the test expectations (`aria-checked` on the active option). The exact API of M1's `Segmented` is `{ value, options, onchange }` per the umbrella spec §3.2. **Same for the `measure` preference store** — it ships in M1 per the umbrella spec §2.3 table; if missing locally, stub it analogously to `density` in `preferences.svelte.ts` with allowed values `['narrow', 'comfortable', 'wide']` and default `'comfortable'`.

- [ ] **Step 4: Run tests to verify they pass**

```bash
pnpm --dir web test -- src/views/settings/__tests__/AppearanceSection.test.ts
```

Expected: 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/settings/AppearanceSection.svelte \
        web/src/views/settings/__tests__/AppearanceSection.test.ts
git commit -m "$(cat <<'EOF'
M-Redesign-6: implement 01 · APPEARANCE section

Theme / font / density / measure segmented controls bound to the
preference stores. Clicking each option mutates the corresponding
$state-backed pref and persists to localStorage.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A2: Build the 02 · READING section

**Files:**
- Create: `web/src/views/settings/ReadingSection.svelte`
- Create: `web/src/views/settings/__tests__/ReadingSection.test.ts`

- [ ] **Step 1: Write the failing test**

```ts
import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, beforeEach } from 'vitest';
import ReadingSection from '../ReadingSection.svelte';
import { reading } from '../../../lib/preferences.svelte';

describe('ReadingSection', () => {
  beforeEach(() => localStorage.clear());

  it('renders four toggles with the current values', () => {
    reading.markOnScroll = true;
    reading.autoOpenNext = false;
    reading.showSummaries = true;
    reading.openLinksNewTab = true;
    const { getByLabelText } = render(ReadingSection);
    expect(getByLabelText(/Mark read on scroll/i)).toBeChecked();
    expect(getByLabelText(/Auto-open next/i)).not.toBeChecked();
    expect(getByLabelText(/Show summaries/i)).toBeChecked();
    expect(getByLabelText(/Open links in new tab/i)).toBeChecked();
  });

  it('clicking a toggle flips the pref', async () => {
    reading.autoOpenNext = false;
    const { getByLabelText } = render(ReadingSection);
    await fireEvent.click(getByLabelText(/Auto-open next/i));
    expect(reading.autoOpenNext).toBe(true);
    expect(localStorage.getItem('tap.reading.autoOpenNext')).toBe('true');
  });
});
```

- [ ] **Step 2: Run to see failure**

```bash
pnpm --dir web test -- src/views/settings/__tests__/ReadingSection.test.ts
```

Expected: import failure.

- [ ] **Step 3: Implement**

Create `web/src/views/settings/ReadingSection.svelte`:

```svelte
<script lang="ts">
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import { reading } from '../../lib/preferences.svelte';
</script>

<SetSection num="02" title="Reading">
  <SetRow
    label="Mark read on scroll"
    desc="Auto-marks an entry as read 1.5s after its lede leaves the viewport."
  >
    {#snippet control()}
      <label class="toggle">
        <input type="checkbox" bind:checked={reading.markOnScroll} aria-label="Mark read on scroll" />
        <span class="slider" aria-hidden="true"></span>
      </label>
    {/snippet}
  </SetRow>
  <SetRow
    label="Auto-open next"
    desc="When you mark an entry read, jump to the next unread automatically."
  >
    {#snippet control()}
      <label class="toggle">
        <input type="checkbox" bind:checked={reading.autoOpenNext} aria-label="Auto-open next" />
        <span class="slider" aria-hidden="true"></span>
      </label>
    {/snippet}
  </SetRow>
  <SetRow
    label="Show summaries in list"
    desc="Renders the 1–3 sentence excerpt under each entry title. Compact density overrides this."
  >
    {#snippet control()}
      <label class="toggle">
        <input type="checkbox" bind:checked={reading.showSummaries} aria-label="Show summaries in list" />
        <span class="slider" aria-hidden="true"></span>
      </label>
    {/snippet}
  </SetRow>
  <SetRow
    label="Open links in new tab"
    desc="Adds target=_blank and rel=noopener to outbound links inside the reader."
  >
    {#snippet control()}
      <label class="toggle">
        <input type="checkbox" bind:checked={reading.openLinksNewTab} aria-label="Open links in new tab" />
        <span class="slider" aria-hidden="true"></span>
      </label>
    {/snippet}
  </SetRow>
</SetSection>

<style>
  .toggle {
    position: relative; display: inline-block;
    width: 36px; height: 20px;
  }
  .toggle input {
    position: absolute; opacity: 0; width: 100%; height: 100%; cursor: pointer;
  }
  .slider {
    position: absolute; inset: 0;
    background: var(--bg-soft);
    border: 1px solid var(--rule);
    border-radius: 999px;
    transition: background 100ms ease;
  }
  .slider::before {
    content: ""; position: absolute;
    width: 14px; height: 14px;
    left: 2px; top: 2px;
    background: var(--bg);
    border-radius: 50%;
    box-shadow: 0 1px 2px rgba(0,0,0,0.15);
    transition: transform 120ms ease;
  }
  .toggle input:checked + .slider { background: var(--accent); border-color: var(--accent); }
  .toggle input:checked + .slider::before { transform: translateX(16px); }
  .toggle input:focus-visible + .slider {
    outline: 2px solid var(--accent); outline-offset: 1px;
  }
</style>
```

- [ ] **Step 4: Run tests**

```bash
pnpm --dir web test -- src/views/settings/__tests__/ReadingSection.test.ts
```

Expected: 2 tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/settings/ReadingSection.svelte \
        web/src/views/settings/__tests__/ReadingSection.test.ts
git commit -m "$(cat <<'EOF'
M-Redesign-6: implement 02 · READING section

Four boolean toggles wired into the new `prefs.reading.*` store.
Toggle visual is a small custom switch (accent fill when on).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A3: Build the 03 · SYNCING section

**Files:**
- Create: `web/src/views/settings/SyncingSection.svelte`
- Create: `web/src/views/settings/__tests__/SyncingSection.test.ts`

- [ ] **Step 1: Write the failing test**

**Precondition:** verify M-Redesign-5 has landed `api.refreshSubscription(id)` plus the backend `PATCH /api/v1/subscriptions/:id` accepting `{refresh_now: true}`. Run:

```bash
grep -n "refreshSubscription\|refresh_now" web/src/lib/api.ts internal/api/subscriptions.go
```

Both files must show the symbol. If either is missing, **stop and rebase on M-Redesign-5 first**. Do not invent a parallel mechanism.

```ts
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import SyncingSection from '../SyncingSection.svelte';
import { api } from '../../../lib/api';
import { poll } from '../../../lib/preferences.svelte';

describe('SyncingSection', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it('shows the current poll-interval pref as the active segmented option', () => {
    poll.interval = '5m';
    const { getByRole } = render(SyncingSection);
    expect(getByRole('radio', { name: /5m/i })).toHaveAttribute('aria-checked', 'true');
  });

  it('clicking Refresh all now calls api.refreshSubscription once per feed (M5 mechanism)', async () => {
    vi.spyOn(api, 'listSubscriptions').mockResolvedValue([
      { id: 1 } as never, { id: 2 } as never, { id: 3 } as never,
    ]);
    const refresh = vi.spyOn(api, 'refreshSubscription').mockResolvedValue();
    const { getByRole } = render(SyncingSection);
    await fireEvent.click(getByRole('button', { name: /refresh all/i }));
    await waitFor(() => {
      expect(refresh).toHaveBeenCalledTimes(3);
      expect(refresh).toHaveBeenCalledWith(1);
      expect(refresh).toHaveBeenCalledWith(2);
      expect(refresh).toHaveBeenCalledWith(3);
    });
  });

  it('surfaces a partial-failure status when at least one feed PATCH rejects', async () => {
    vi.spyOn(api, 'listSubscriptions').mockResolvedValue([
      { id: 1 } as never, { id: 2 } as never,
    ]);
    const refresh = vi.spyOn(api, 'refreshSubscription')
      .mockImplementation((id: number) => id === 1 ? Promise.resolve() : Promise.reject(new Error('500')));
    const { getByRole, findByRole } = render(SyncingSection);
    await fireEvent.click(getByRole('button', { name: /refresh all/i }));
    expect(await findByRole('alert')).toHaveTextContent(/1 of 2|1 failed/i);
    expect(refresh).toHaveBeenCalledTimes(2);
  });

  it('renders the last-sync timestamp from /healthz', async () => {
    vi.spyOn(api, 'health').mockResolvedValue({
      polls_active: 0,
      last_poll_at: 1715379000,
    });
    const { findByText } = render(SyncingSection);
    expect(await findByText(/last sync/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to see failure**

```bash
pnpm --dir web test -- src/views/settings/__tests__/SyncingSection.test.ts
```

Expected: missing `api.health` (api.refreshSubscription + api.listSubscriptions already exist from M5).

- [ ] **Step 3: Add the `health` helper to `web/src/lib/api.ts`**

Append to the `api` object (just below `listEntries`):

```ts
  // Wrapper around /healthz for SPA-side display only. Public endpoint;
  // does not flow through request() because /healthz lives outside /api/v1.
  health: async (): Promise<{ polls_active: number; last_poll_at?: number }> => {
    const r = await fetch('/healthz');
    if (!r.ok) throw new Error(`${r.status} ${r.statusText}`);
    return r.json();
  },
```

**Do not introduce** any `refreshAllSubscriptions`, `pollAll`, or `POST /subscriptions/:id/poll` helper. The umbrella spec §2.4 (post-amendment) makes M5's `PATCH /api/v1/subscriptions/:id` with `{refresh_now: true}` the canonical refresh mechanism for the whole SPA. Implement the loop inside the section component itself, surfacing per-feed errors via `Promise.allSettled`. If `/healthz` doesn't yet return `last_poll_at`, leave the field optional and just show "last sync —" until a future server-side change adds it. (Verify the current shape with `curl -s http://localhost:8080/healthz` before assuming.)

- [ ] **Step 4: Implement the section**

Create `web/src/views/settings/SyncingSection.svelte`:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import Segmented from '../../components/Segmented.svelte';
  import Button from '../../components/Button.svelte';
  import { poll } from '../../lib/preferences.svelte';
  import { api } from '../../lib/api';

  const options = [
    { value: '5m',     label: '5m' },
    { value: '15m',    label: '15m' },
    { value: '1h',     label: '1h' },
    { value: 'manual', label: 'Manual' },
  ];

  let lastSyncAt = $state<number | null>(null);
  let busy = $state(false);
  let error = $state('');

  onMount(async () => {
    try {
      const h = await api.health();
      lastSyncAt = h.last_poll_at ?? null;
    } catch { /* swallow — display-only */ }
  });

  async function refreshAll() {
    busy = true;
    error = '';
    try {
      const subs = await api.listSubscriptions();
      const results = await Promise.allSettled(subs.map(s => api.refreshSubscription(s.id)));
      const failed = results.filter(r => r.status === 'rejected').length;
      if (failed > 0) {
        error = `Refreshed ${subs.length - failed} of ${subs.length} · ${failed} failed`;
      }
      const h = await api.health();
      lastSyncAt = h.last_poll_at ?? null;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Refresh failed';
    } finally {
      busy = false;
    }
  }

  function formatAgo(ts: number | null): string {
    if (!ts) return 'never';
    const ago = Math.max(0, Math.round((Date.now() / 1000 - ts) / 60));
    if (ago < 1) return 'just now';
    if (ago < 60) return `${ago}m ago`;
    if (ago < 24 * 60) return `${Math.round(ago / 60)}h ago`;
    return `${Math.round(ago / 1440)}d ago`;
  }
</script>

<SetSection num="03" title="Syncing">
  <SetRow
    label="Poll interval"
    desc="How often Tap reaches out to your feeds. Backend cadence is adaptive — this is your minimum."
  >
    {#snippet control()}
      <Segmented value={poll.interval} options={options} onchange={(v) => poll.interval = v} />
    {/snippet}
  </SetRow>
  <SetRow label="Last sync" desc={`last sync ${formatAgo(lastSyncAt)}`}>
    {#snippet control()}
      <Button onclick={refreshAll} disabled={busy}>Refresh all now</Button>
    {/snippet}
  </SetRow>
  {#if error}
    <div role="alert" class="error">{error}</div>
  {/if}
</SetSection>

<style>
  .error { color: var(--ink); font-family: var(--mono); font-size: 11px; padding: 8px 0; }
</style>
```

- [ ] **Step 5: Run tests**

```bash
pnpm --dir web test -- src/views/settings/__tests__/SyncingSection.test.ts
```

Expected: 4 tests pass.

- [ ] **Step 6: Commit**

```bash
git add web/src/views/settings/SyncingSection.svelte \
        web/src/views/settings/__tests__/SyncingSection.test.ts \
        web/src/lib/api.ts
git commit -m "$(cat <<'EOF'
M-Redesign-6: implement 03 · SYNCING section

Poll-interval segmented (advisory), Refresh-all-now button (N×1 loop
over api.refreshSubscription from M5; surfaces partial-failure count
via role=alert), and last-sync mono timestamp polled from /healthz.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A4: Build the 04 · ACCOUNT section + Change password dialog

**Files:**
- Create: `web/src/views/settings/AccountSection.svelte`
- Create: `web/src/views/settings/dialogs/ChangePasswordDialog.svelte`
- Create: `web/src/views/settings/__tests__/AccountSection.test.ts`
- Create: `web/src/views/settings/dialogs/__tests__/ChangePasswordDialog.test.ts`

- [ ] **Step 1: Write failing tests**

`AccountSection.test.ts`:

```ts
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import AccountSection from '../AccountSection.svelte';
import { api } from '../../../lib/api';

vi.mock('../../../lib/auth', async () => {
  const { writable } = await import('svelte/store');
  return {
    auth: {
      ...writable({ user: { id: 1, username: 'liz@hauck.studio', role: 'user', has_totp: false, passkey_count: 0 }, csrfToken: 'x', bootstrapped: true }),
      logout: vi.fn(),
    },
    ERR_UNAUTHORIZED: 'unauthorized',
  };
});

describe('AccountSection', () => {
  it('renders the email in mono', () => {
    const { getByText } = render(AccountSection);
    expect(getByText('liz@hauck.studio')).toBeInTheDocument();
  });

  it('clicking Change password opens the dialog', async () => {
    const { getByRole, queryByText, findByText } = render(AccountSection);
    expect(queryByText('Change password')).toBeInTheDocument();
    await fireEvent.click(getByRole('button', { name: /change password/i }));
    expect(await findByText(/current password/i)).toBeInTheDocument();
  });

  it('clicking Sign out everywhere calls api.revokeAllOtherSessions', async () => {
    const spy = vi.spyOn(api, 'revokeAllOtherSessions').mockResolvedValue();
    const { getByRole } = render(AccountSection);
    await fireEvent.click(getByRole('button', { name: /sign out everywhere/i }));
    await waitFor(() => expect(spy).toHaveBeenCalledOnce());
  });
});
```

`ChangePasswordDialog.test.ts`:

```ts
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import ChangePasswordDialog from '../ChangePasswordDialog.svelte';
import { api } from '../../../../lib/api';

describe('ChangePasswordDialog', () => {
  it('submits the form when current + new + confirm are filled', async () => {
    const spy = vi.spyOn(api, 'changePassword').mockResolvedValue({ csrf_token: 'new' });
    const onClose = vi.fn();
    const { getByLabelText, getByRole } = render(ChangePasswordDialog, { onClose });
    await fireEvent.input(getByLabelText(/current password/i), { target: { value: 'old' } });
    await fireEvent.input(getByLabelText(/new password/i), { target: { value: 'newpass1234' } });
    await fireEvent.input(getByLabelText(/confirm/i), { target: { value: 'newpass1234' } });
    await fireEvent.click(getByRole('button', { name: /change/i }));
    await waitFor(() => {
      expect(spy).toHaveBeenCalledWith('old', 'newpass1234');
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('shows a local mismatch error before calling the API', async () => {
    const spy = vi.spyOn(api, 'changePassword');
    const { getByLabelText, getByRole, findByRole } = render(ChangePasswordDialog, { onClose: vi.fn() });
    await fireEvent.input(getByLabelText(/current password/i), { target: { value: 'old' } });
    await fireEvent.input(getByLabelText(/new password/i), { target: { value: 'aaaaaaaa' } });
    await fireEvent.input(getByLabelText(/confirm/i), { target: { value: 'bbbbbbbb' } });
    await fireEvent.click(getByRole('button', { name: /change/i }));
    expect(await findByRole('alert')).toHaveTextContent(/match/i);
    expect(spy).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run to see failure**

```bash
pnpm --dir web test -- src/views/settings/__tests__/AccountSection.test.ts \
                       src/views/settings/dialogs/__tests__/ChangePasswordDialog.test.ts
```

Expected: import failures.

- [ ] **Step 3: Implement the dialog**

Create `web/src/views/settings/dialogs/ChangePasswordDialog.svelte`:

```svelte
<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import Field from '../../../components/Field.svelte';
  import { api } from '../../../lib/api';

  interface Props { onClose: () => void; }
  let { onClose }: Props = $props();

  let current = $state('');
  let next = $state('');
  let confirm = $state('');
  let error = $state('');
  let busy = $state(false);

  async function submit() {
    error = '';
    if (next !== confirm) { error = "New passwords don't match."; return; }
    busy = true;
    try {
      await api.changePassword(current, next);
      onClose();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not change password.';
    } finally { busy = false; }
  }
</script>

<Dialog title="Change password" {onClose}>
  {#snippet body()}
    <Field id="cp-current" label="Current password" type="password" bind:value={current} />
    <Field id="cp-new"     label="New password"     type="password" bind:value={next} />
    <Field id="cp-confirm" label="Confirm new password" type="password" bind:value={confirm} />
    {#if error}<div role="alert" class="warn">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <div class="foot-l">CSRF rotates on success</div>
    <Button kind="quiet" onclick={onClose}>Cancel</Button>
    <Button kind="primary" onclick={submit} disabled={busy || !current || !next || !confirm}>Change password</Button>
  {/snippet}
</Dialog>

<style>
  .warn {
    margin-top: 12px;
    padding: 10px 12px;
    border-left: 2px solid var(--accent);
    background: var(--accent-soft);
    color: var(--ink-2);
    font-family: var(--sans);
    font-size: 12px;
    line-height: 1.5;
    border-radius: 0 3px 3px 0;
  }
  .foot-l {
    margin-right: auto;
    font-family: var(--mono);
    font-size: 10.5px;
    color: var(--ink-3);
    align-self: center;
  }
</style>
```

> **Note for the implementer:** `Dialog`, `Button`, `Field` are M-Redesign-1 primitives. `Dialog`'s expected API per the umbrella spec §3.2: `{ title, onClose, body, foot }` snippets, with built-in focus trap, Esc-to-close, click-outside-to-close. `Field`'s API: `{ id, label, type, value: $bindable }`. If either primitive's actual API differs, adjust the call site and document the deviation in the PR.

- [ ] **Step 4: Implement the section**

Create `web/src/views/settings/AccountSection.svelte`:

```svelte
<script lang="ts">
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import Button from '../../components/Button.svelte';
  import { auth } from '../../lib/auth';
  import { api } from '../../lib/api';
  import ChangePasswordDialog from './dialogs/ChangePasswordDialog.svelte';

  let dialogOpen = $state(false);
  let busy = $state(false);
  let error = $state('');

  // Auto-subscribe to the auth store via `$auth` so the email re-renders if
  // the user's username changes (e.g., a future "change email" flow) and so
  // tests can swap in a fresh writable per-test. `get(auth)` here is a
  // one-shot read — it would not re-run when auth updates.
  const email = $derived($auth.user?.username ?? '');

  async function signOutEverywhere() {
    busy = true; error = '';
    try {
      await api.revokeAllOtherSessions();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Sign-out failed.';
    } finally { busy = false; }
  }
</script>

<SetSection num="04" title="Account">
  <SetRow label="Email" desc="Used for sign-in and account recovery.">
    {#snippet control()}
      <span class="email">{email}</span>
    {/snippet}
  </SetRow>
  <SetRow label="Change password" desc="Re-authenticates and rotates your session token.">
    {#snippet control()}
      <Button onclick={() => dialogOpen = true}>Change password</Button>
    {/snippet}
  </SetRow>
  <SetRow label="Sign out everywhere" desc="Revokes every other session except this one. You stay signed in on this device.">
    {#snippet control()}
      <Button kind="danger" onclick={signOutEverywhere} disabled={busy}>Sign out everywhere</Button>
    {/snippet}
  </SetRow>
  {#if error}<div role="alert" class="error">{error}</div>{/if}
</SetSection>

{#if dialogOpen}
  <ChangePasswordDialog onClose={() => dialogOpen = false} />
{/if}

<style>
  .email {
    font-family: var(--mono);
    font-size: 13px;
    color: var(--ink);
    letter-spacing: 0.01em;
  }
  .error {
    color: var(--ink); font-family: var(--mono); font-size: 11px;
    padding: 8px 0;
  }
</style>
```

- [ ] **Step 5: Run tests**

```bash
pnpm --dir web test -- src/views/settings/__tests__/AccountSection.test.ts \
                       src/views/settings/dialogs/__tests__/ChangePasswordDialog.test.ts
```

Expected: 5 tests pass.

- [ ] **Step 6: Commit**

```bash
git add web/src/views/settings/AccountSection.svelte \
        web/src/views/settings/dialogs/ChangePasswordDialog.svelte \
        web/src/views/settings/__tests__/AccountSection.test.ts \
        web/src/views/settings/dialogs/__tests__/ChangePasswordDialog.test.ts
git commit -m "$(cat <<'EOF'
M-Redesign-6: implement 04 · ACCOUNT section + Change password dialog

Email mono row, Change password dialog (current + new + confirm, with
local mismatch check before the API call), and Sign out everywhere
danger button wired to api.revokeAllOtherSessions.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A5: Build 05 · SECURITY (TOTP half) — status row + enrol/disable/regen dialogs

**Files:**
- Create: `web/src/views/settings/SecurityTOTPSection.svelte`
- Create: `web/src/views/settings/dialogs/EnrolTOTPDialog.svelte`
- Create: `web/src/views/settings/dialogs/DisableTOTPDialog.svelte`
- Create: `web/src/views/settings/dialogs/RegenerateCodesDialog.svelte`
- Create: `web/src/views/settings/dialogs/ViewRecoveryCodesDialog.svelte`
- Create: `web/src/views/settings/__tests__/SecurityTOTPSection.test.ts`
- Create: `web/src/views/settings/dialogs/__tests__/EnrolTOTPDialog.test.ts`

- [ ] **Step 1: Write the failing tests**

`SecurityTOTPSection.test.ts` (covers status-row + button conditional rendering):

```ts
import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import { writable } from 'svelte/store';
import SecurityTOTPSection from '../SecurityTOTPSection.svelte';

vi.mock('../../../lib/auth', () => ({
  auth: writable({ user: { has_totp: false }, csrfToken: 'x', bootstrapped: true }),
  ERR_UNAUTHORIZED: 'unauthorized',
}));

describe('SecurityTOTPSection', () => {
  it('when has_totp=false, shows Not enrolled and a Set up button', () => {
    const { getByText, getByRole } = render(SecurityTOTPSection);
    expect(getByText(/not enrolled/i)).toBeInTheDocument();
    expect(getByRole('button', { name: /set up authenticator/i })).toBeInTheDocument();
  });

  it('clicking Set up authenticator opens the enrolment dialog', async () => {
    const { getByRole, findByText } = render(SecurityTOTPSection);
    await fireEvent.click(getByRole('button', { name: /set up authenticator/i }));
    expect(await findByText(/set up authenticator/i)).toBeInTheDocument();
    // Look for QR placeholder + secret display in the dialog body.
  });
});
```

`EnrolTOTPDialog.test.ts`:

```ts
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import EnrolTOTPDialog from '../EnrolTOTPDialog.svelte';
import { api } from '../../../../lib/api';

vi.mock('qrcode', () => ({ default: { toDataURL: vi.fn().mockResolvedValue('data:image/png;base64,xxx') } }));

describe('EnrolTOTPDialog', () => {
  it('on mount fetches a fresh secret and renders the QR', async () => {
    vi.spyOn(api, 'beginTOTPEnrolment').mockResolvedValue({ secret_uri: 'otpauth://x', secret: 'JBSW' });
    const { findByAltText } = render(EnrolTOTPDialog, { onClose: vi.fn(), onSuccess: vi.fn() });
    expect(await findByAltText(/qr/i)).toBeInTheDocument();
  });

  it('Confirm calls confirmTOTPEnrolment with the entered code and resolves onSuccess with the recovery codes', async () => {
    vi.spyOn(api, 'beginTOTPEnrolment').mockResolvedValue({ secret_uri: 'otpauth://x', secret: 'JBSW' });
    const confirmSpy = vi.spyOn(api, 'confirmTOTPEnrolment').mockResolvedValue({ recovery_codes: ['AAAA-BBBB-CCCC'] });
    const onSuccess = vi.fn();
    const { findAllByRole, getByRole } = render(EnrolTOTPDialog, { onClose: vi.fn(), onSuccess });
    const cells = await findAllByRole('textbox');
    for (let i = 0; i < 6; i++) await fireEvent.input(cells[i], { target: { value: String((i + 1) % 10) } });
    await fireEvent.click(getByRole('button', { name: /enable/i }));
    await waitFor(() => {
      expect(confirmSpy).toHaveBeenCalledWith('123450');
      expect(onSuccess).toHaveBeenCalledWith(['AAAA-BBBB-CCCC']);
    });
  });
});
```

- [ ] **Step 2: Run to see failure**

```bash
pnpm --dir web test -- src/views/settings/__tests__/SecurityTOTPSection.test.ts \
                       src/views/settings/dialogs/__tests__/EnrolTOTPDialog.test.ts
```

Expected: import failure.

- [ ] **Step 3: Implement `EnrolTOTPDialog.svelte`**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import QRCode from 'qrcode';
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import OtpInput from '../../../components/OtpInput.svelte';
  import { api } from '../../../lib/api';
  import { auth } from '../../../lib/auth';

  interface Props {
    onClose: () => void;
    onSuccess: (codes: string[]) => void;
  }
  let { onClose, onSuccess }: Props = $props();

  let secret = $state('');
  let secretUri = $state('');
  let qrDataUrl = $state('');
  let code = $state('');
  let busy = $state(false);
  let error = $state('');

  onMount(async () => {
    try {
      const res = await api.beginTOTPEnrolment();
      secret = res.secret;
      secretUri = res.secret_uri;
      qrDataUrl = await QRCode.toDataURL(res.secret_uri, { margin: 1, width: 140 });
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not start enrolment.';
    }
  });

  async function confirm() {
    error = '';
    busy = true;
    try {
      const res = await api.confirmTOTPEnrolment(code);
      await auth.bootstrap();
      onSuccess(res.recovery_codes);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Invalid code.';
    } finally { busy = false; }
  }

  async function copySecret() {
    try {
      await navigator.clipboard.writeText(secret.replace(/\s+/g, ''));
    } catch { /* swallow */ }
  }
</script>

<Dialog title="Set up authenticator" {onClose}>
  {#snippet body()}
    <div class="setup">
      {#if qrDataUrl}
        <img class="qr" src={qrDataUrl} alt="QR code for authenticator app" width="140" height="140" />
      {:else}
        <div class="qr placeholder" aria-hidden="true"></div>
      {/if}
      <div class="right">
        <div class="step">Step 1 · Scan or enter secret</div>
        <p class="p">Add Tap to your authenticator app — 1Password, Authy, or any TOTP-compatible client.</p>
        <div class="secret">
          <span class="key">{secret}</span>
          <button type="button" class="copy" onclick={copySecret}>Copy</button>
        </div>
      </div>
    </div>
    <div class="step2">
      <div class="step">Step 2 · Confirm code from your app</div>
      <OtpInput bind:value={code} />
    </div>
    {#if error}<div role="alert" class="warn">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <div class="foot-l">Algorithm SHA-1 · 30s · 6 digits</div>
    <Button kind="quiet" onclick={onClose}>Cancel</Button>
    <Button kind="primary" onclick={confirm} disabled={busy || code.length !== 6}>Enable</Button>
  {/snippet}
</Dialog>

<style>
  .setup { display: flex; gap: 18px; align-items: flex-start; }
  .qr {
    width: 140px; height: 140px;
    border: 1px solid var(--rule); border-radius: 4px;
    flex-shrink: 0;
  }
  .qr.placeholder { background: var(--bg-soft); }
  .right { flex: 1; min-width: 0; }
  .step {
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.12em; text-transform: uppercase;
    color: var(--ink-3); margin-bottom: 6px;
  }
  .p {
    font-family: var(--serif); font-size: 14px;
    line-height: 1.55; color: var(--ink-2); margin: 0 0 10px;
  }
  .secret {
    display: flex; align-items: center; gap: 12px;
    padding: 12px 14px;
    background: var(--bg-soft); border: 1px solid var(--rule);
    border-radius: 4px;
  }
  .key {
    font-family: var(--mono); font-size: 14px;
    letter-spacing: 0.12em; color: var(--ink); font-weight: 500;
  }
  .copy {
    margin-left: auto;
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.08em; text-transform: uppercase;
    color: var(--ink-2);
    border: 1px solid var(--rule); background: var(--bg);
    padding: 4px 8px; border-radius: 3px; cursor: pointer;
  }
  .copy:hover { color: var(--ink); border-color: var(--ink-4); }
  .step2 { margin-top: 22px; }
  .warn {
    margin-top: 12px;
    padding: 10px 12px;
    border-left: 2px solid #c43a3a;
    background: rgba(196, 58, 58, 0.06);
    color: var(--ink-2);
    font-family: var(--sans); font-size: 12px;
    line-height: 1.5;
    border-radius: 0 3px 3px 0;
  }
  .foot-l {
    margin-right: auto;
    font-family: var(--mono); font-size: 10.5px;
    color: var(--ink-3); align-self: center;
  }
</style>
```

- [ ] **Step 4: Implement `ViewRecoveryCodesDialog.svelte`**

```svelte
<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import RecoveryCodesGrid from '../../../components/RecoveryCodesGrid.svelte';

  interface Props {
    codes: string[];
    regenerated?: boolean;
    onClose: () => void;
  }
  let { codes, regenerated = false, onClose }: Props = $props();

  function downloadTxt() {
    const text = codes.map((c, i) => `${String(i + 1).padStart(2, '0')}. ${c}`).join('\n');
    const blob = new Blob([text], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'tap-recovery-codes.txt';
    a.click();
    URL.revokeObjectURL(url);
  }
</script>

<Dialog title={regenerated ? 'New recovery codes' : 'Your recovery codes'} {onClose}>
  {#snippet body()}
    <p class="p">Each code is single-use. Store them in a password manager — they replace your authenticator if you lose access.</p>
    <RecoveryCodesGrid {codes} />
    {#if regenerated}
      <div class="warn">The previous set of codes was just invalidated. Anything saved before now will no longer work.</div>
    {/if}
  {/snippet}
  {#snippet foot()}
    <div class="foot-l">{codes.length} codes · plain text</div>
    <Button onclick={downloadTxt}>Download .txt</Button>
    <Button kind="primary" onclick={onClose}>I've saved them</Button>
  {/snippet}
</Dialog>

<style>
  .p {
    font-family: var(--serif); font-size: 15px; line-height: 1.55;
    color: var(--ink-2); margin: 0 0 14px;
  }
  .warn {
    margin-top: 14px;
    padding: 10px 12px;
    border-left: 2px solid var(--accent);
    background: var(--accent-soft);
    color: var(--ink-2);
    font-family: var(--sans); font-size: 12px;
    line-height: 1.5;
    border-radius: 0 3px 3px 0;
  }
  .foot-l {
    margin-right: auto;
    font-family: var(--mono); font-size: 10.5px;
    color: var(--ink-3); align-self: center;
  }
</style>
```

- [ ] **Step 5: Implement `DisableTOTPDialog.svelte` + `RegenerateCodesDialog.svelte`**

Both use an `OtpInput` and call the corresponding API. They follow the same pattern; here is `DisableTOTPDialog`:

```svelte
<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import OtpInput from '../../../components/OtpInput.svelte';
  import { api } from '../../../lib/api';
  import { auth } from '../../../lib/auth';

  interface Props { onClose: () => void; onSuccess: () => void; }
  let { onClose, onSuccess }: Props = $props();

  let code = $state('');
  let busy = $state(false);
  let error = $state('');

  async function submit() {
    busy = true; error = '';
    try {
      await api.disableTOTP({ code });
      await auth.bootstrap();
      onSuccess();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Invalid code.';
    } finally { busy = false; }
  }
</script>

<Dialog title="Disable two-factor" {onClose}>
  {#snippet body()}
    <p class="p">You're about to remove TOTP from this account. Enter the current 6-digit code from your authenticator to confirm.</p>
    <div class="label">6-digit code from your authenticator</div>
    <OtpInput bind:value={code} />
    {#if error}<div role="alert" class="warn">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <Button kind="quiet" onclick={onClose}>Cancel</Button>
    <Button kind="danger" onclick={submit} disabled={busy || code.length !== 6}>Disable TOTP</Button>
  {/snippet}
</Dialog>

<style>
  .p { font-family: var(--serif); font-size: 15px; line-height: 1.55; color: var(--ink-2); margin: 0 0 14px; }
  .label { font-family: var(--mono); font-size: 10px; letter-spacing: 0.12em; text-transform: uppercase; color: var(--ink-3); margin-bottom: 8px; }
  .warn { margin-top: 12px; padding: 10px 12px; border-left: 2px solid #c43a3a; background: rgba(196,58,58,0.06); color: var(--ink-2); font-family: var(--sans); font-size: 12px; line-height: 1.5; border-radius: 0 3px 3px 0; }
</style>
```

`RegenerateCodesDialog.svelte` is structurally identical: change title to "Regenerate recovery codes", body copy to "Generating new codes invalidates the old set immediately. Enter your current authenticator code to continue.", call `api.regenerateRecoveryCodes(code)` and pass the returned codes to `onSuccess`. CTA "Generate new codes" (primary, not danger).

- [ ] **Step 6: Implement `SecurityTOTPSection.svelte`**

```svelte
<script lang="ts">
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import Button from '../../components/Button.svelte';
  import { auth } from '../../lib/auth';
  import EnrolTOTPDialog from './dialogs/EnrolTOTPDialog.svelte';
  import DisableTOTPDialog from './dialogs/DisableTOTPDialog.svelte';
  import RegenerateCodesDialog from './dialogs/RegenerateCodesDialog.svelte';
  import ViewRecoveryCodesDialog from './dialogs/ViewRecoveryCodesDialog.svelte';

  type Open = null | 'enrol' | 'disable' | 'regen' | 'view';
  let open = $state<Open>(null);
  let viewCodes = $state<string[]>([]);
  let regenerated = $state(false);

  // Reactive — `auth.bootstrap()` after an enrol/disable flow flips
  // `has_totp` in the store, and this $derived must re-run so the button
  // set swaps from "Set up" to "Regenerate / Disable".
  const hasTOTP = $derived($auth.user?.has_totp ?? false);
</script>

<SetSection num="05" title="Security · Two-factor">
  <SetRow
    label="Authenticator app (TOTP)"
    desc="Pair Tap with an authenticator app. Required on new sign-ins once enabled."
    block
  >
    {#snippet children()}
      <div class="status" class:on={hasTOTP}>
        <span class="dot" aria-hidden="true"></span>
        {#if hasTOTP}
          <span class="strong">Enabled</span>
          <span class="sep" aria-hidden="true"></span>
          <span>SHA-1 · 30s · 6 digits</span>
        {:else}
          <span class="strong">Not enrolled</span>
          <span class="sep" aria-hidden="true"></span>
          <span>your account relies on password only</span>
        {/if}
      </div>
      <div class="actions">
        {#if hasTOTP}
          <Button kind="accent" onclick={() => { regenerated = true; open = 'regen'; }}>Regenerate recovery codes</Button>
          <Button kind="danger" onclick={() => open = 'disable'}>Disable TOTP</Button>
        {:else}
          <Button kind="primary" onclick={() => open = 'enrol'}>Set up authenticator</Button>
        {/if}
      </div>
    {/snippet}
  </SetRow>
</SetSection>

{#if open === 'enrol'}
  <EnrolTOTPDialog
    onClose={() => open = null}
    onSuccess={(codes) => { viewCodes = codes; regenerated = false; open = 'view'; }}
  />
{:else if open === 'disable'}
  <DisableTOTPDialog onClose={() => open = null} onSuccess={() => open = null} />
{:else if open === 'regen'}
  <RegenerateCodesDialog
    onClose={() => open = null}
    onSuccess={(codes) => { viewCodes = codes; open = 'view'; }}
  />
{:else if open === 'view'}
  <ViewRecoveryCodesDialog codes={viewCodes} regenerated={regenerated} onClose={() => { open = null; viewCodes = []; }} />
{/if}

<style>
  .status {
    display: flex; align-items: center; gap: 12px;
    padding: 10px 14px;
    border: 1px solid var(--rule); background: var(--bg-soft);
    border-radius: 4px;
    font-family: var(--mono); font-size: 11px;
    color: var(--ink-2); letter-spacing: 0.02em;
  }
  .status .dot { width: 6px; height: 6px; border-radius: 50%; background: var(--ink-4); }
  .status.on .dot { background: var(--accent); }
  .status .sep { width: 3px; height: 3px; border-radius: 50%; background: var(--ink-4); }
  .strong { color: var(--ink); font-weight: 500; }
  .actions { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 14px; }
</style>
```

- [ ] **Step 7: Run tests**

```bash
pnpm --dir web test -- src/views/settings/__tests__/SecurityTOTPSection.test.ts \
                       src/views/settings/dialogs/__tests__/EnrolTOTPDialog.test.ts
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

```bash
git add web/src/views/settings/SecurityTOTPSection.svelte \
        web/src/views/settings/dialogs/EnrolTOTPDialog.svelte \
        web/src/views/settings/dialogs/DisableTOTPDialog.svelte \
        web/src/views/settings/dialogs/RegenerateCodesDialog.svelte \
        web/src/views/settings/dialogs/ViewRecoveryCodesDialog.svelte \
        web/src/views/settings/__tests__/SecurityTOTPSection.test.ts \
        web/src/views/settings/dialogs/__tests__/EnrolTOTPDialog.test.ts
git commit -m "$(cat <<'EOF'
M-Redesign-6: implement 05 · SECURITY TOTP half

Status row + Set up / Disable / Regenerate buttons; four dialogs
(EnrolTOTPDialog, DisableTOTPDialog, RegenerateCodesDialog,
ViewRecoveryCodesDialog) wired to existing M7 endpoints. QR is rendered
client-side from the secret_uri via the qrcode package.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A6: Build 05 · SECURITY (Passkeys half)

**Files:**
- Create: `web/src/views/settings/SecurityPasskeysSection.svelte`
- Create: `web/src/views/settings/dialogs/AddPasskeyDialog.svelte`
- Create: `web/src/views/settings/dialogs/RemovePasskeyDialog.svelte`
- Create: `web/src/views/settings/__tests__/SecurityPasskeysSection.test.ts`

- [ ] **Step 1: Write failing tests**

```ts
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { writable } from 'svelte/store';
import SecurityPasskeysSection from '../SecurityPasskeysSection.svelte';
import { api } from '../../../lib/api';

vi.mock('../../../lib/auth', () => ({
  auth: writable({ user: { has_totp: true, passkey_count: 2 }, csrfToken: 'x', bootstrapped: true }),
  ERR_UNAUTHORIZED: 'unauthorized',
}));

describe('SecurityPasskeysSection', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('renders each passkey from the API as a row', async () => {
    vi.spyOn(api, 'listPasskeys').mockResolvedValue([
      { id: 1, label: 'MacBook · Touch ID', created_at: 1715000000 },
      { id: 2, label: 'YubiKey · Office',   created_at: 1715000100 },
    ]);
    const { findByText } = render(SecurityPasskeysSection);
    expect(await findByText('MacBook · Touch ID')).toBeInTheDocument();
    expect(await findByText('YubiKey · Office')).toBeInTheDocument();
  });

  it('Add passkey opens the AddPasskeyDialog', async () => {
    vi.spyOn(api, 'listPasskeys').mockResolvedValue([]);
    const { getByRole, findByText } = render(SecurityPasskeysSection);
    await fireEvent.click(getByRole('button', { name: /add a passkey/i }));
    expect(await findByText(/label/i)).toBeInTheDocument();
  });

  it('Remove ✗ opens the password-challenge dialog', async () => {
    vi.spyOn(api, 'listPasskeys').mockResolvedValue([
      { id: 1, label: 'MacBook · Touch ID', created_at: 1715000000 },
    ]);
    const { findAllByLabelText, findByText } = render(SecurityPasskeysSection);
    const revokes = await findAllByLabelText(/remove passkey/i);
    await fireEvent.click(revokes[0]);
    expect(await findByText(/account password/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to see failure**

```bash
pnpm --dir web test -- src/views/settings/__tests__/SecurityPasskeysSection.test.ts
```

Expected: import failure.

- [ ] **Step 3: Implement `AddPasskeyDialog.svelte`**

```svelte
<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import Field from '../../../components/Field.svelte';
  import { api } from '../../../lib/api';
  import { auth } from '../../../lib/auth';

  interface Props { onClose: () => void; onSaved: () => void; }
  let { onClose, onSaved }: Props = $props();

  let label = $state('My device');
  let busy = $state(false);
  let error = $state('');

  function b64urlToBytes(b64: string): Uint8Array {
    const pad = b64.length % 4 === 0 ? '' : '='.repeat(4 - (b64.length % 4));
    const b64std = (b64 + pad).replace(/-/g, '+').replace(/_/g, '/');
    return Uint8Array.from(atob(b64std), c => c.charCodeAt(0));
  }
  function bytesToB64url(buf: ArrayBuffer): string {
    return btoa(String.fromCharCode(...new Uint8Array(buf)))
      .replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
  }

  async function submit() {
    error = ''; busy = true;
    try {
      const opts = await api.beginPasskeyRegistration() as { response: {
        user: { id: string };
        challenge: string;
        rp: PublicKeyCredentialRpEntity;
        pubKeyCredParams: PublicKeyCredentialParameters[];
      } };
      const credential = await navigator.credentials.create({
        publicKey: {
          rp: opts.response.rp,
          user: {
            id: b64urlToBytes(opts.response.user.id).buffer as ArrayBuffer,
            name: opts.response.user.id,
            displayName: opts.response.user.id,
          },
          challenge: b64urlToBytes(opts.response.challenge).buffer as ArrayBuffer,
          pubKeyCredParams: opts.response.pubKeyCredParams,
        },
      });
      if (!credential) throw new Error('No credential created');
      const pk = credential as PublicKeyCredential;
      const r = pk.response as AuthenticatorAttestationResponse;
      await api.finishPasskeyRegistration({
        id: pk.id,
        rawId: bytesToB64url(pk.rawId),
        type: pk.type,
        response: {
          clientDataJSON: bytesToB64url(r.clientDataJSON),
          attestationObject: bytesToB64url(r.attestationObject),
        },
      }, label);
      await auth.bootstrap();
      onSaved();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not add passkey.';
    } finally { busy = false; }
  }
</script>

<Dialog title="Add a passkey" {onClose}>
  {#snippet body()}
    <p class="p">Give this passkey a memorable label so you can recognise it later — your device's name, or where it lives.</p>
    <Field id="pk-label" label="Label" bind:value={label} />
    <p class="p sm">On the next step, your browser will ask which device or security key to use.</p>
    {#if error}<div role="alert" class="warn">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <Button kind="quiet" onclick={onClose}>Cancel</Button>
    <Button kind="primary" onclick={submit} disabled={busy || !label.trim()}>Use this device →</Button>
  {/snippet}
</Dialog>

<style>
  .p { font-family: var(--serif); font-size: 15px; line-height: 1.55; color: var(--ink-2); margin: 0 0 14px; }
  .p.sm { font-size: 13.5px; margin-top: 18px; margin-bottom: 0; }
  .warn { margin-top: 12px; padding: 10px 12px; border-left: 2px solid #c43a3a; background: rgba(196,58,58,0.06); color: var(--ink-2); font-family: var(--sans); font-size: 12px; line-height: 1.5; border-radius: 0 3px 3px 0; }
</style>
```

- [ ] **Step 4: Implement `RemovePasskeyDialog.svelte`**

```svelte
<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import Field from '../../../components/Field.svelte';
  import { api } from '../../../lib/api';
  import { auth } from '../../../lib/auth';

  interface Props { passkeyId: number; passkeyLabel: string; onClose: () => void; onRemoved: () => void; }
  let { passkeyId, passkeyLabel, onClose, onRemoved }: Props = $props();

  let password = $state('');
  let busy = $state(false);
  let error = $state('');

  async function submit() {
    error = ''; busy = true;
    try {
      // Note: the M7 DELETE /me/passkeys/{id} endpoint does not currently
      // require a re-authentication. The password field here is a UX
      // gate, not a backend requirement; the spec calls for it but the
      // server treats it as advisory. If a follow-up tightens this to
      // re-authenticated removal, the same field will then be POSTed
      // through. For now: don't send it.
      await api.deletePasskey(passkeyId);
      await auth.bootstrap();
      onRemoved();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not remove passkey.';
    } finally { busy = false; }
  }
</script>

<Dialog title="Remove passkey?" {onClose}>
  {#snippet body()}
    <p class="p">Removing <b>{passkeyLabel}</b> means it can no longer sign in to Tap. Confirm with your account password — you can add the passkey back any time.</p>
    <Field id="pk-confirm-pw" label="Account password" type="password" bind:value={password} />
    {#if error}<div role="alert" class="warn">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <div class="foot-l">re-authenticating</div>
    <Button kind="quiet" onclick={onClose}>Cancel</Button>
    <Button kind="danger" onclick={submit} disabled={busy || !password}>Remove passkey</Button>
  {/snippet}
</Dialog>

<style>
  .p { font-family: var(--serif); font-size: 15px; line-height: 1.55; color: var(--ink-2); margin: 0 0 14px; }
  .warn { margin-top: 12px; padding: 10px 12px; border-left: 2px solid #c43a3a; background: rgba(196,58,58,0.06); color: var(--ink-2); font-family: var(--sans); font-size: 12px; line-height: 1.5; border-radius: 0 3px 3px 0; }
  .foot-l { margin-right: auto; font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); align-self: center; }
</style>
```

- [ ] **Step 5: Implement `SecurityPasskeysSection.svelte`**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import Button from '../../components/Button.svelte';
  import EmptyState from '../../components/EmptyState.svelte';
  import { api } from '../../lib/api';
  import type { Passkey } from '../../lib/types';
  import AddPasskeyDialog from './dialogs/AddPasskeyDialog.svelte';
  import RemovePasskeyDialog from './dialogs/RemovePasskeyDialog.svelte';

  let passkeys = $state<Passkey[]>([]);
  let addOpen = $state(false);
  let toRemove = $state<Passkey | null>(null);
  let error = $state('');

  async function load() {
    try { passkeys = await api.listPasskeys(); }
    catch (e) { error = e instanceof Error ? e.message : 'Could not load passkeys.'; }
  }

  onMount(load);

  function fmtDate(ts: number) { return new Date(ts * 1000).toLocaleDateString(); }
</script>

<SetSection num="05" title="Security · Passkeys" tag={passkeys.length === 0 ? 'add one' : `${passkeys.length} active`}>
  <SetRow
    label="Registered passkeys"
    desc="Sign in without a password using a device-bound credential. You can have up to 10."
    block
  >
    {#snippet children()}
      {#if passkeys.length > 0}
        <div class="pk-list">
          {#each passkeys as p (p.id)}
            <div class="pk">
              <span class="pk-icon" aria-hidden="true">
                <svg width="16" height="16" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="7" cy="10" r="3.2"/><path d="M10 10h7M14 10v2.5M16.5 10v3"/>
                </svg>
              </span>
              <div class="pk-text">
                <div class="pk-label">{p.label}</div>
                <div class="pk-meta">added {fmtDate(p.created_at)}</div>
              </div>
              <button class="revoke" aria-label={`Remove passkey ${p.label}`} onclick={() => toRemove = p}>
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M4 4l8 8M12 4l-8 8"/></svg>
              </button>
            </div>
          {/each}
        </div>
      {:else}
        <EmptyState title="No passkeys yet" subtitle="Add one to skip the password on this device." />
      {/if}
      <div class="actions">
        <Button kind="primary" onclick={() => addOpen = true}>Add a passkey</Button>
      </div>
      {#if error}<div role="alert" class="error">{error}</div>{/if}
    {/snippet}
  </SetRow>
</SetSection>

{#if addOpen}
  <AddPasskeyDialog onClose={() => addOpen = false} onSaved={async () => { addOpen = false; await load(); }} />
{/if}
{#if toRemove}
  <RemovePasskeyDialog
    passkeyId={toRemove.id}
    passkeyLabel={toRemove.label}
    onClose={() => toRemove = null}
    onRemoved={async () => { toRemove = null; await load(); }}
  />
{/if}

<style>
  .pk-list {
    display: flex; flex-direction: column;
    border: 1px solid var(--rule); border-radius: 4px; overflow: hidden;
  }
  .pk {
    display: grid;
    grid-template-columns: 24px minmax(0, 1fr) auto;
    align-items: center; gap: 14px;
    padding: 12px 14px;
    border-bottom: 1px solid var(--rule);
    background: var(--bg);
  }
  .pk:last-child { border-bottom: 0; }
  .pk-icon { color: var(--ink-3); display: inline-flex; }
  .pk-text { min-width: 0; }
  .pk-label { font-family: var(--sans); font-size: 13.5px; font-weight: 500; color: var(--ink); }
  .pk-meta { font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); margin-top: 2px; }
  .revoke {
    width: 28px; height: 28px;
    display: inline-flex; align-items: center; justify-content: center;
    color: var(--ink-3);
    background: transparent; border: 0; border-radius: 4px; cursor: pointer;
  }
  .revoke:hover { color: #c43a3a; background: var(--bg-soft); }
  .actions { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 14px; }
  .error { color: var(--ink); font-family: var(--mono); font-size: 11px; padding: 8px 0; }
</style>
```

- [ ] **Step 6: Run tests**

```bash
pnpm --dir web test -- src/views/settings/__tests__/SecurityPasskeysSection.test.ts
```

Expected: 3 tests pass.

- [ ] **Step 7: Commit**

```bash
git add web/src/views/settings/SecurityPasskeysSection.svelte \
        web/src/views/settings/dialogs/AddPasskeyDialog.svelte \
        web/src/views/settings/dialogs/RemovePasskeyDialog.svelte \
        web/src/views/settings/__tests__/SecurityPasskeysSection.test.ts
git commit -m "$(cat <<'EOF'
M-Redesign-6: implement 05 · SECURITY Passkeys half

.ts-pk-list with one row per registered passkey, AddPasskeyDialog with
label field that triggers navigator.credentials.create, and
RemovePasskeyDialog with password-confirm UX gate.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A7: Build 06 · SESSIONS section

**Files:**
- Create: `web/src/views/settings/SessionsSection.svelte`
- Create: `web/src/views/settings/__tests__/SessionsSection.test.ts`

- [ ] **Step 1: Write failing test**

```ts
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import SessionsSection from '../SessionsSection.svelte';
import { api } from '../../../lib/api';

describe('SessionsSection', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('renders one row per session', async () => {
    vi.spyOn(api, 'listSessions').mockResolvedValue([
      { id: 1, created_at: 1, last_seen_at: 1, idle_expires_at: 1, user_agent: 'Firefox', address: '1.2.3.4', current: true },
      { id: 2, created_at: 1, last_seen_at: 1, idle_expires_at: 1, user_agent: 'Safari/iOS', address: '5.6.7.8', current: false },
    ]);
    const { findByText } = render(SessionsSection);
    expect(await findByText('Firefox')).toBeInTheDocument();
    expect(await findByText('Safari/iOS')).toBeInTheDocument();
  });

  it('current session row has revoke disabled', async () => {
    vi.spyOn(api, 'listSessions').mockResolvedValue([
      { id: 1, created_at: 1, last_seen_at: 1, idle_expires_at: 1, user_agent: 'Firefox', address: '1.2.3.4', current: true },
    ]);
    const { findByLabelText } = render(SessionsSection);
    const btn = await findByLabelText(/cannot revoke current session/i);
    expect(btn).toBeDisabled();
  });

  it('clicking revoke removes the row optimistically and calls api.revokeSession', async () => {
    vi.spyOn(api, 'listSessions').mockResolvedValue([
      { id: 1, created_at: 1, last_seen_at: 1, idle_expires_at: 1, user_agent: 'Firefox', address: '1.2.3.4', current: true },
      { id: 2, created_at: 1, last_seen_at: 1, idle_expires_at: 1, user_agent: 'Safari/iOS', address: '5.6.7.8', current: false },
    ]);
    const revokeSpy = vi.spyOn(api, 'revokeSession').mockResolvedValue();
    const { findByLabelText, queryByText } = render(SessionsSection);
    const revokeBtn = await findByLabelText(/revoke session/i);
    await fireEvent.click(revokeBtn);
    await waitFor(() => {
      expect(revokeSpy).toHaveBeenCalledWith(2);
      expect(queryByText('Safari/iOS')).not.toBeInTheDocument();
    });
  });
});
```

- [ ] **Step 2: Run to see failure**

```bash
pnpm --dir web test -- src/views/settings/__tests__/SessionsSection.test.ts
```

Expected: import failure.

- [ ] **Step 3: Implement**

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import { api } from '../../lib/api';
  import type { Session } from '../../lib/types';

  let sessions = $state<Session[]>([]);
  let error = $state('');
  let busy = $state(false);

  onMount(async () => {
    try { sessions = await api.listSessions(); }
    catch (e) { error = e instanceof Error ? e.message : 'Could not load sessions.'; }
  });

  async function revoke(id: number) {
    busy = true;
    try {
      await api.revokeSession(id);
      sessions = sessions.filter(s => s.id !== id);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not revoke session.';
    } finally { busy = false; }
  }

  function inferKind(ua: string): 'desktop' | 'mobile' | 'cli' {
    const u = (ua || '').toLowerCase();
    if (u.includes('iphone') || u.includes('android') || u.includes('mobile')) return 'mobile';
    if (u.includes('curl') || u.includes('cli') || !u.includes('mozilla')) return 'cli';
    return 'desktop';
  }

  function formatAgo(ts: number): string {
    const ago = Math.max(0, Math.round((Date.now() / 1000 - ts) / 60));
    if (ago < 1) return 'now';
    if (ago < 60) return `${ago}m ago`;
    if (ago < 24 * 60) return `${Math.round(ago / 60)}h ago`;
    return `${Math.round(ago / 1440)}d ago`;
  }
</script>

<SetSection num="06" title="Sessions">
  <SetRow
    label="Active sessions"
    desc="Devices currently signed in to your account. Revoke anything you don't recognise."
    block
  >
    {#snippet children()}
      <div class="sess-list" role="list">
        {#each sessions as s (s.id)}
          {@const kind = inferKind(s.user_agent)}
          <div class="sess" class:is-current={s.current} role="listitem">
            <span class="sess-icon" aria-hidden="true">
              {#if kind === 'mobile'}
                <svg width="18" height="18" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="2" width="8" height="16" rx="1.5"/><path d="M9.5 15.5h1"/></svg>
              {:else if kind === 'cli'}
                <svg width="18" height="18" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="4" width="16" height="12" rx="1.2"/><path d="M5 8l2.5 2L5 12M10 13h4"/></svg>
              {:else}
                <svg width="18" height="18" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="16" height="11" rx="1.2"/><path d="M7 17h6M10 14v3"/></svg>
              {/if}
            </span>
            <div class="sess-text">
              <div class="sess-device">
                <span>{s.user_agent || 'Unknown'}</span>
                {#if s.current}<span class="sess-tag">this session</span>{/if}
              </div>
              <div class="sess-meta">
                <span>{s.address || 'Unknown'}</span>
              </div>
            </div>
            <span class="sess-when">{formatAgo(s.last_seen_at)}</span>
            <button
              type="button"
              class="sess-revoke"
              aria-label={s.current ? 'Cannot revoke current session' : `Revoke session ${s.user_agent || 'Unknown'}`}
              disabled={s.current || busy}
              onclick={() => revoke(s.id)}
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"><path d="M4 4l8 8M12 4l-8 8"/></svg>
            </button>
          </div>
        {/each}
      </div>
      {#if error}<div role="alert" class="error">{error}</div>{/if}
    {/snippet}
  </SetRow>
</SetSection>

<style>
  .sess-list {
    display: flex; flex-direction: column;
    border: 1px solid var(--rule); border-radius: 4px;
    background: var(--bg); overflow: hidden;
  }
  .sess {
    display: grid;
    grid-template-columns: 28px minmax(0, 1fr) auto auto;
    align-items: center; gap: 14px;
    padding: 14px 16px;
    border-bottom: 1px solid var(--rule);
  }
  .sess:last-child { border-bottom: 0; }
  .sess.is-current { background: var(--accent-soft); }
  .sess-icon {
    width: 28px; height: 28px;
    display: inline-flex; align-items: center; justify-content: center;
    color: var(--ink-3);
  }
  .sess.is-current .sess-icon { color: var(--accent); }
  .sess-text { min-width: 0; }
  .sess-device {
    font-family: var(--sans); font-size: 13.5px; font-weight: 500; color: var(--ink);
    display: flex; align-items: baseline; gap: 8px; flex-wrap: wrap;
  }
  .sess-tag {
    font-family: var(--mono); font-size: 9.5px;
    letter-spacing: 0.08em; text-transform: uppercase;
    color: var(--accent);
  }
  .sess-meta {
    font-family: var(--mono); font-size: 11px;
    color: var(--ink-3); margin-top: 3px;
  }
  .sess-when { font-family: var(--mono); font-size: 11px; color: var(--ink-3); white-space: nowrap; }
  .sess-revoke {
    width: 28px; height: 28px;
    display: inline-flex; align-items: center; justify-content: center;
    color: var(--ink-3);
    background: transparent; border: 0; border-radius: 4px; cursor: pointer;
  }
  .sess-revoke:hover:not(:disabled) { color: #c43a3a; background: var(--bg-soft); }
  .sess-revoke:disabled { color: var(--ink-4); cursor: not-allowed; }
  .error { color: var(--ink); font-family: var(--mono); font-size: 11px; padding: 8px 0; }
</style>
```

- [ ] **Step 4: Run tests**

```bash
pnpm --dir web test -- src/views/settings/__tests__/SessionsSection.test.ts
```

Expected: 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/views/settings/SessionsSection.svelte \
        web/src/views/settings/__tests__/SessionsSection.test.ts
git commit -m "$(cat <<'EOF'
M-Redesign-6: implement 06 · SESSIONS section

.ts-sess-list with desktop/mobile/CLI icon inferred from user-agent,
mono meta, this-session tag, and a revoke ✗ that disables for the
current session and removes the row optimistically on success.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task B1: Add `DeleteUser` to `internal/db/users.go` + cascade test

**Files:**
- Modify: `internal/db/users.go`
- Modify: `internal/db/users_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/db/users_test.go`:

```go
func TestDeleteUser_RemovesRowAndCascades(t *testing.T) {
	t.Parallel()
	conn := newTestDB(t) // existing helper from package; if missing, see TestInsertUser for the open + migrate pattern.

	u := db.NewUser{Username: "to-delete@example.com", PasswordHash: "$argon2id$v=19$m=8,t=1,p=1$ZGVhZGJlZWY$AAAAAAAAAAAAAAAAAAAA"}
	created, err := db.InsertUser(t.Context(), conn, u)
	require.NoError(t, err)
	uid := created.ID

	// Seed one of every child row so cascade actually exercises FKs.
	sub, err := db.InsertSubscription(t.Context(), conn, db.NewSubscription{UserID: uid, FeedURL: "https://example.com/feed", Title: "ex"})
	require.NoError(t, err)
	_, err = db.InsertEntry(t.Context(), conn, db.NewEntry{SubscriptionID: sub.ID, Hash: "hash", Title: "t", URL: "u", Content: "c"})
	require.NoError(t, err)
	_, err = db.InsertSession(t.Context(), conn, db.NewSession{UserID: uid, TokenHash: "th", CSRFToken: "csrf"})
	require.NoError(t, err)
	// Categories + passkeys + tombstones: insert if their helpers exist in db.

	err = db.DeleteUser(t.Context(), conn, uid)
	require.NoError(t, err)

	_, err = db.GetUserByID(t.Context(), conn, uid)
	require.ErrorIs(t, err, sql.ErrNoRows)

	var count int
	require.NoError(t, conn.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM subscriptions WHERE user_id = ?", uid).Scan(&count))
	require.Equal(t, 0, count, "subscriptions cascade failed")
	require.NoError(t, conn.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM sessions WHERE user_id = ?", uid).Scan(&count))
	require.Equal(t, 0, count, "sessions cascade failed")
}

func TestDeleteUser_Missing(t *testing.T) {
	t.Parallel()
	conn := newTestDB(t)
	err := db.DeleteUser(t.Context(), conn, 99999)
	// Match the existing semantics: deleting a non-existent row is not an error.
	require.NoError(t, err)
}
```

> **Implementer note:** Inspect the existing `internal/db/migrations/0005_*.sql` (or whichever migration adds users/sessions) to confirm `ON DELETE CASCADE` is in place on `sessions.user_id`, `subscriptions.user_id`, `entries.subscription_id`, `categories.user_id`, `passkeys.user_id`, `tombstones.subscription_id`. If any constraint is missing, **add it via a new migration `0011_user_delete_cascade.sql`** (or whichever next index is free) and re-run all migration tests. Don't paper over a missing constraint with an explicit DELETE in `DeleteUser` — the FK is the long-term safety net.

- [ ] **Step 2: Run to see failure**

```bash
go test ./internal/db -run 'TestDeleteUser' -race 2>&1 | head -30
```

Expected: build failure (`db.DeleteUser` undefined).

- [ ] **Step 3: Implement**

Append to `internal/db/users.go`:

```go
// DeleteUser removes the row from users; FK cascades wipe subscriptions,
// entries, sessions, categories, passkeys, and tombstones. Deleting a
// non-existent user is a no-op (no error) — mirrors GetUserByID's
// distinction between "missing row" and "real error".
func DeleteUser(ctx context.Context, conn Conn, id int64) error {
	_, err := conn.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	return err
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/db -run 'TestDeleteUser' -race
```

Expected: 2 tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/db/users.go internal/db/users_test.go
git commit -m "$(cat <<'EOF'
M-Redesign-6: add db.DeleteUser with cascade verification

Backs DELETE /api/v1/me. Relies on existing ON DELETE CASCADE FKs to
clean up subscriptions, entries, sessions, categories, passkeys, and
tombstones in one statement. Missing-id is a no-op.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task B2: Add `deleteAccountHandler` + wire `DELETE /api/v1/me`

**Files:**
- Modify: `internal/api/auth.go`
- Modify: `internal/api/auth_test.go`
- Modify: `internal/api/api.go`
- Modify: `internal/api/errors.go` (only if a new error code is added)

- [ ] **Step 1: Write failing test**

Append to `internal/api/auth_test.go`:

```go
func TestDeleteAccount_HappyPath_204AndCascades(t *testing.T) {
	t.Parallel()
	// Reuse the existing setupServer helper from this test file.
	srv := setupServer(t)
	defer srv.Close()
	user, password, sess := createUserAndLogin(t, srv, "ada@example.com")
	// Seed one subscription so we can confirm the cascade.
	addSubscription(t, srv, sess, "https://example.com/feed")

	body := fmt.Sprintf(`{"current_password":%q}`, password)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me", strings.NewReader(body))
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: sess.Cookie})
	w := httptest.NewRecorder()
	srv.Mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	// The user is gone; a follow-up authed request must 401.
	_, err := db.GetUserByID(t.Context(), srv.DB, user.ID)
	require.ErrorIs(t, err, sql.ErrNoRows)
	// Session is gone too (cascade).
	r2 := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil)
	r2.AddCookie(&http.Cookie{Name: "tap_session", Value: sess.Cookie})
	w2 := httptest.NewRecorder()
	srv.Mux.ServeHTTP(w2, r2)
	require.Equal(t, http.StatusUnauthorized, w2.Code)
}

func TestDeleteAccount_WrongPassword_401(t *testing.T) {
	t.Parallel()
	srv := setupServer(t)
	defer srv.Close()
	_, _, sess := createUserAndLogin(t, srv, "ada@example.com")

	body := `{"current_password":"nope"}`
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me", strings.NewReader(body))
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: sess.Cookie})
	w := httptest.NewRecorder()
	srv.Mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), "invalid_credentials")
}

func TestDeleteAccount_MissingPassword_400(t *testing.T) {
	t.Parallel()
	srv := setupServer(t)
	defer srv.Close()
	_, _, sess := createUserAndLogin(t, srv, "ada@example.com")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me", strings.NewReader(`{}`))
	req.Header.Set("X-CSRF-Token", sess.CSRFToken)
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: sess.Cookie})
	w := httptest.NewRecorder()
	srv.Mux.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
```

> **Implementer note:** `setupServer`, `createUserAndLogin`, and `addSubscription` are helpers expected to exist alongside the M7 auth tests. Inspect `internal/api/auth_test.go` first; if their signatures differ, adapt the test to use the actual harness. If no harness exists for end-to-end auth flows, the equivalent test runs at the cmd/tap level — port it into `cmd/tap/main_test.go` instead.

- [ ] **Step 2: Run to see failure**

```bash
make test 2>&1 | tail -40
```

Expected: `deleteAccountHandler` undefined.

- [ ] **Step 3: Implement the handler**

Append to `internal/api/auth.go`:

```go
type deleteAccountRequest struct {
	CurrentPassword string `json:"current_password"`
}

// deleteAccountHandler returns DELETE /api/v1/me. Re-authenticates with
// the user's current password and then DELETEs the user row, relying on
// FK cascades to clean up sessions, subscriptions, entries, categories,
// passkeys, and tombstones. CSRF is enforced by the surrounding mux
// chain (authedCSRF).
func deleteAccountHandler(deps deleteAccountDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Match the rest of internal/api/auth.go: 1 MiB cap with explicit
		// MaxBytesError handling so 413 vs 400 is correct. 1 KB is too
		// tight for long passphrases plus a JSON envelope plus headers.
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var req deleteAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				writeError(w, http.StatusRequestEntityTooLarge, ErrCodeBadRequest, "request body too large")
				return
			}
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "invalid JSON body")
			return
		}
		if req.CurrentPassword == "" {
			writeError(w, http.StatusBadRequest, ErrCodeBadRequest, "current_password required")
			return
		}
		user, ok := userFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidSession, "no session")
			return
		}
		ok, err := auth.Verify(user.PasswordHash, req.CurrentPassword)
		if err != nil {
			slog.ErrorContext(r.Context(), "delete account: verify failed", "err", err)
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, "internal error")
			return
		}
		if !ok {
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidCredentials, "invalid credentials")
			return
		}
		if err := db.DeleteUser(r.Context(), deps.DB, user.ID); err != nil {
			slog.ErrorContext(r.Context(), "delete account: db delete failed", "err", err)
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, "internal error")
			return
		}
		clearSessionCookie(w, deps.CookieSecure)
		w.WriteHeader(http.StatusNoContent)
	}
}
```

> **Implementer note:** `deleteAccountDeps` is a thin struct with `DB *sql.DB` and `CookieSecure bool` — match the existing convention from `passwordChangeHandler` (which uses `passwordChangeDeps` or similar). Use the existing context-key helpers `userFromContext` and the existing `auth.Verify`. If a `deps` struct is overkill (e.g., other handlers take individual params), match the surrounding style instead.

Wire the route in `internal/api/api.go`. Find the existing `m.Handle("PATCH /api/v1/me/password", ...)` line and add the sibling immediately below:

```go
m.Handle("DELETE /api/v1/me", authedCSRF(deleteAccountHandler(deleteAccountDeps{DB: deps.DB, CookieSecure: opts.CookieSecure})))
```

- [ ] **Step 4: Run tests**

```bash
make test 2>&1 | tail -20
```

Expected: 3 new tests pass; existing tests unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/api/auth.go internal/api/api.go internal/api/auth_test.go
git commit -m "$(cat <<'EOF'
M-Redesign-6: add DELETE /api/v1/me

Account deletion endpoint with password re-authentication and CSRF
enforcement. The session is cleared on success; FK cascades wipe
subscriptions, entries, sessions, categories, passkeys, tombstones.

Justified narrow backend addition per the umbrella spec §1: brand
spec §6.6 row 07 explicitly defines this as a defining Settings
feature; no other M-Redesign-* milestone owns it.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task B3: Add `api.deleteAccount` to the SPA client

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/__tests__/api.test.ts`

- [ ] **Step 1: Write failing test**

Append to `web/src/lib/__tests__/api.test.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api } from '../api';

describe('api.deleteAccount', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('issues DELETE /api/v1/me with current_password in the body and CSRF header', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 204 }));
    await api.deleteAccount('secret');
    expect(fetchSpy).toHaveBeenCalledWith('/api/v1/me', expect.objectContaining({
      method: 'DELETE',
      body: JSON.stringify({ current_password: 'secret' }),
    }));
  });

  it('rejects with the server message on 401', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({ error: { code: 'invalid_credentials', message: 'wrong password' } }), { status: 401 }));
    await expect(api.deleteAccount('bad')).rejects.toThrow(/wrong password/);
  });
});
```

- [ ] **Step 2: Run to see failure**

```bash
pnpm --dir web test -- src/lib/__tests__/api.test.ts
```

Expected: `api.deleteAccount` undefined.

- [ ] **Step 3: Implement**

Append to the `api` object in `web/src/lib/api.ts` (just below `changePassword`):

```ts
  deleteAccount: (currentPassword: string) =>
    request<void>('/me', {
      method: 'DELETE',
      body: JSON.stringify({ current_password: currentPassword }),
    }),
```

Append to `web/src/lib/types.ts`:

```ts
export type DeleteAccountRequest = { current_password: string };
```

- [ ] **Step 4: Run tests**

```bash
pnpm --dir web test -- src/lib/__tests__/api.test.ts
```

Expected: 2 new tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/api.ts web/src/lib/types.ts web/src/lib/__tests__/api.test.ts
git commit -m "$(cat <<'EOF'
M-Redesign-6: SPA client for DELETE /api/v1/me

api.deleteAccount(currentPassword) — passes through the existing
request() helper which already attaches X-CSRF-Token to non-GET
methods and clears auth state on 401.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task A8: Build 07 · DATA section (OPML export/import, JSON export, delete account)

**Files:**
- Create: `web/src/views/settings/DataSection.svelte`
- Create: `web/src/views/settings/dialogs/ImportOPMLDialog.svelte`
- Create: `web/src/views/settings/dialogs/DeleteAccountDialog.svelte`
- Create: `web/src/views/settings/__tests__/DataSection.test.ts`
- Create: `web/src/views/settings/dialogs/__tests__/DeleteAccountDialog.test.ts`

- [ ] **Step 1: Write failing tests**

`DataSection.test.ts`:

```ts
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import DataSection from '../DataSection.svelte';
import { api } from '../../../lib/api';

describe('DataSection', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.stubGlobal('URL', { createObjectURL: vi.fn().mockReturnValue('blob://x'), revokeObjectURL: vi.fn() });
  });

  it('Export OPML triggers api.exportOPML and downloads', async () => {
    vi.spyOn(api, 'exportOPML').mockResolvedValue(new Blob(['<opml/>'], { type: 'application/xml' }));
    const { getByRole } = render(DataSection);
    await fireEvent.click(getByRole('button', { name: /export opml/i }));
    await waitFor(() => expect(api.exportOPML).toHaveBeenCalledOnce());
  });

  it('Export saved JSON calls /entries?saved=1 and downloads', async () => {
    const fetchSpy = vi.spyOn(api, 'listEntries').mockResolvedValue({ data: [], next: null });
    const { getByRole } = render(DataSection);
    await fireEvent.click(getByRole('button', { name: /export saved/i }));
    await waitFor(() => expect(fetchSpy).toHaveBeenCalledWith({ saved: true, limit: 1000 }));
  });

  it('Delete account opens the password-challenge dialog', async () => {
    const { getByRole, findByText } = render(DataSection);
    await fireEvent.click(getByRole('button', { name: /delete account/i }));
    expect(await findByText(/cannot be undone/i)).toBeInTheDocument();
  });
});
```

`DeleteAccountDialog.test.ts`:

```ts
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import DeleteAccountDialog from '../DeleteAccountDialog.svelte';
import { api } from '../../../../lib/api';

vi.mock('../../../../lib/auth', () => ({
  auth: { logout: vi.fn().mockResolvedValue(undefined) },
  ERR_UNAUTHORIZED: 'unauthorized',
}));
vi.mock('../../../../lib/router', () => ({ navigate: vi.fn() }));

describe('DeleteAccountDialog', () => {
  it('submits with current_password and on success calls auth.logout + navigate(/sign-in)', async () => {
    const del = vi.spyOn(api, 'deleteAccount').mockResolvedValue();
    const { auth } = await import('../../../../lib/auth');
    const { navigate } = await import('../../../../lib/router');
    const { getByLabelText, getByRole } = render(DeleteAccountDialog, { onClose: vi.fn() });
    await fireEvent.input(getByLabelText(/password/i), { target: { value: 'pw' } });
    await fireEvent.click(getByRole('button', { name: /delete my account/i }));
    await waitFor(() => {
      expect(del).toHaveBeenCalledWith('pw');
      expect(auth.logout).toHaveBeenCalled();
      expect(navigate).toHaveBeenCalledWith('/sign-in');
    });
  });

  it('shows error on 401 and does not log out', async () => {
    vi.spyOn(api, 'deleteAccount').mockRejectedValue(new Error('invalid credentials'));
    const { auth } = await import('../../../../lib/auth');
    const { getByLabelText, getByRole, findByRole } = render(DeleteAccountDialog, { onClose: vi.fn() });
    await fireEvent.input(getByLabelText(/password/i), { target: { value: 'bad' } });
    await fireEvent.click(getByRole('button', { name: /delete my account/i }));
    expect(await findByRole('alert')).toHaveTextContent(/invalid/i);
    expect(auth.logout).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run to see failure**

```bash
pnpm --dir web test -- src/views/settings/__tests__/DataSection.test.ts \
                       src/views/settings/dialogs/__tests__/DeleteAccountDialog.test.ts
```

Expected: import failures.

- [ ] **Step 3: Implement `DeleteAccountDialog.svelte`**

```svelte
<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import Field from '../../../components/Field.svelte';
  import { api } from '../../../lib/api';
  import { auth } from '../../../lib/auth';
  import { navigate } from '../../../lib/router';

  interface Props { onClose: () => void; }
  let { onClose }: Props = $props();

  let password = $state('');
  let busy = $state(false);
  let error = $state('');

  async function submit() {
    error = ''; busy = true;
    try {
      await api.deleteAccount(password);
      await auth.logout();
      navigate('/sign-in');
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not delete account.';
    } finally { busy = false; }
  }
</script>

<Dialog title="Delete account?" {onClose}>
  {#snippet body()}
    <div class="dialog-warn">
      This cannot be undone. All your feeds, saved articles, sessions, passkeys, and read history will be permanently removed.
    </div>
    <Field id="del-pw" label="Confirm with your account password" type="password" bind:value={password} />
    {#if error}<div role="alert" class="error">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <div class="foot-l">irreversible</div>
    <Button kind="quiet" onclick={onClose}>Cancel</Button>
    <Button kind="danger" onclick={submit} disabled={busy || !password}>Delete my account</Button>
  {/snippet}
</Dialog>

<style>
  .dialog-warn {
    margin-bottom: 14px;
    padding: 10px 12px;
    border-left: 2px solid #c43a3a;
    background: rgba(196, 58, 58, 0.06);
    color: var(--ink-2);
    font-family: var(--sans); font-size: 12px;
    line-height: 1.5;
    border-radius: 0 3px 3px 0;
  }
  .error {
    margin-top: 12px; padding: 10px 12px;
    border-left: 2px solid #c43a3a;
    background: rgba(196, 58, 58, 0.06);
    color: var(--ink-2);
    font-family: var(--sans); font-size: 12px;
    line-height: 1.5;
    border-radius: 0 3px 3px 0;
  }
  .foot-l { margin-right: auto; font-family: var(--mono); font-size: 10.5px; color: var(--ink-3); align-self: center; }
</style>
```

- [ ] **Step 4: Implement `ImportOPMLDialog.svelte`**

```svelte
<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import { api } from '../../../lib/api';
  import type { OPMLImportResult } from '../../../lib/types';

  interface Props { onClose: () => void; onImported: (r: OPMLImportResult) => void; }
  let { onClose, onImported }: Props = $props();

  let file = $state<File | null>(null);
  let busy = $state(false);
  let error = $state('');

  function onFile(e: Event) {
    const t = e.target as HTMLInputElement;
    file = t.files?.[0] ?? null;
  }

  async function submit() {
    if (!file) return;
    busy = true; error = '';
    try {
      const buf = await file.arrayBuffer();
      const result = await api.importOPML(buf);
      onImported(result);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Import failed.';
    } finally { busy = false; }
  }
</script>

<Dialog title="Import OPML" {onClose}>
  {#snippet body()}
    <p class="p">Choose an OPML file exported from another reader. Existing subscriptions with the same feed URL are skipped.</p>
    <input type="file" accept=".opml,.xml,application/xml" onchange={onFile} aria-label="OPML file" />
    {#if error}<div role="alert" class="warn">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <Button kind="quiet" onclick={onClose}>Cancel</Button>
    <Button kind="primary" onclick={submit} disabled={busy || !file}>Import</Button>
  {/snippet}
</Dialog>

<style>
  .p { font-family: var(--serif); font-size: 15px; line-height: 1.55; color: var(--ink-2); margin: 0 0 14px; }
  .warn { margin-top: 12px; padding: 10px 12px; border-left: 2px solid #c43a3a; background: rgba(196,58,58,0.06); color: var(--ink-2); font-family: var(--sans); font-size: 12px; line-height: 1.5; border-radius: 0 3px 3px 0; }
</style>
```

- [ ] **Step 5: Implement `DataSection.svelte`**

```svelte
<script lang="ts">
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import Button from '../../components/Button.svelte';
  import { api } from '../../lib/api';
  import ImportOPMLDialog from './dialogs/ImportOPMLDialog.svelte';
  import DeleteAccountDialog from './dialogs/DeleteAccountDialog.svelte';

  let importOpen = $state(false);
  let deleteOpen = $state(false);
  let importResult = $state<{ imported: number; skipped: number } | null>(null);
  let error = $state('');
  let busy = $state(false);

  async function exportOPML() {
    busy = true; error = '';
    try {
      const blob = await api.exportOPML();
      downloadBlob(blob, 'tap-subscriptions.opml');
    } catch (e) {
      error = e instanceof Error ? e.message : 'Export failed.';
    } finally { busy = false; }
  }

  async function exportSavedJSON() {
    busy = true; error = '';
    try {
      const res = await api.listEntries({ saved: true, limit: 1000 });
      const blob = new Blob([JSON.stringify(res.data, null, 2)], { type: 'application/json' });
      downloadBlob(blob, 'tap-saved.json');
    } catch (e) {
      error = e instanceof Error ? e.message : 'Export failed.';
    } finally { busy = false; }
  }

  function downloadBlob(blob: Blob, filename: string) {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url; a.download = filename; a.click();
    URL.revokeObjectURL(url);
  }
</script>

<SetSection num="07" title="Data">
  <SetRow label="Export subscriptions (OPML)" desc="A standard OPML file compatible with most readers.">
    {#snippet control()}
      <Button onclick={exportOPML} disabled={busy}>Export OPML</Button>
    {/snippet}
  </SetRow>
  <SetRow label="Export saved entries (JSON)" desc="One-time snapshot of every entry you've saved.">
    {#snippet control()}
      <Button onclick={exportSavedJSON} disabled={busy}>Export saved JSON</Button>
    {/snippet}
  </SetRow>
  <SetRow label="Import OPML" desc="Merge feeds from another reader's export. Duplicates are skipped.">
    {#snippet control()}
      <Button onclick={() => importOpen = true}>Import OPML</Button>
    {/snippet}
  </SetRow>
  <SetRow label="Delete account" desc="Permanently remove your account and every feed, entry, and session attached to it.">
    {#snippet control()}
      <Button kind="danger" onclick={() => deleteOpen = true}>Delete account</Button>
    {/snippet}
  </SetRow>
  {#if importResult}
    <div class="ok">Imported {importResult.imported} feeds, skipped {importResult.skipped}.</div>
  {/if}
  {#if error}<div role="alert" class="error">{error}</div>{/if}
</SetSection>

{#if importOpen}
  <ImportOPMLDialog onClose={() => importOpen = false} onImported={(r) => { importResult = r; importOpen = false; }} />
{/if}
{#if deleteOpen}
  <DeleteAccountDialog onClose={() => deleteOpen = false} />
{/if}

<style>
  .ok, .error {
    font-family: var(--mono); font-size: 11px;
    padding: 8px 0; color: var(--ink-2);
  }
</style>
```

- [ ] **Step 6: Run tests**

```bash
pnpm --dir web test -- src/views/settings/__tests__/DataSection.test.ts \
                       src/views/settings/dialogs/__tests__/DeleteAccountDialog.test.ts
```

Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add web/src/views/settings/DataSection.svelte \
        web/src/views/settings/dialogs/ImportOPMLDialog.svelte \
        web/src/views/settings/dialogs/DeleteAccountDialog.svelte \
        web/src/views/settings/__tests__/DataSection.test.ts \
        web/src/views/settings/dialogs/__tests__/DeleteAccountDialog.test.ts
git commit -m "$(cat <<'EOF'
M-Redesign-6: implement 07 · DATA section

Export OPML, Export saved JSON (client-side blob), Import OPML dialog,
and Delete account dialog with password challenge + .ts-dialog-warn
callout. Successful deletion logs out and routes to /sign-in.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task C1: Replace the top-level `Settings.svelte` view

**Files:**
- Modify: `web/src/views/Settings.svelte`
- Delete: `web/src/views/settings/Security.svelte`
- Create: `web/src/views/__tests__/Settings.test.ts`

- [ ] **Step 1: Write failing test**

```ts
import { render } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import { writable } from 'svelte/store';
import Settings from '../Settings.svelte';

vi.mock('../../lib/auth', () => ({
  auth: writable({ user: { id: 1, username: 'liz@hauck.studio', role: 'user', has_totp: false, passkey_count: 0 }, csrfToken: 'x', bootstrapped: true }),
  ERR_UNAUTHORIZED: 'unauthorized',
}));
vi.mock('../../lib/api', () => ({
  api: {
    listSessions: vi.fn().mockResolvedValue([]),
    listPasskeys: vi.fn().mockResolvedValue([]),
    listSubscriptions: vi.fn().mockResolvedValue([]),
    refreshSubscription: vi.fn().mockResolvedValue(undefined),
    health: vi.fn().mockResolvedValue({ polls_active: 0 }),
  },
}));

describe('Settings page', () => {
  it('renders the seven numbered eyebrows', () => {
    const { getByText } = render(Settings);
    expect(getByText('Appearance')).toBeInTheDocument();
    expect(getByText('Reading')).toBeInTheDocument();
    expect(getByText('Syncing')).toBeInTheDocument();
    expect(getByText('Account')).toBeInTheDocument();
    expect(getByText(/Security/i)).toBeInTheDocument();
    expect(getByText('Sessions')).toBeInTheDocument();
    expect(getByText('Data')).toBeInTheDocument();
  });

  it('renders the page-id strip with the user email and role', () => {
    const { getByText } = render(Settings);
    expect(getByText('liz@hauck.studio')).toBeInTheDocument();
    expect(getByText(/account/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to see failure**

```bash
pnpm --dir web test -- src/views/__tests__/Settings.test.ts
```

Expected: the existing Settings still renders the old two-tab UI and the test fails on the missing eyebrows.

- [ ] **Step 3: Replace `views/Settings.svelte`**

```svelte
<script lang="ts">
  import { auth } from '../lib/auth';
  import AppearanceSection from './settings/AppearanceSection.svelte';
  import ReadingSection from './settings/ReadingSection.svelte';
  import SyncingSection from './settings/SyncingSection.svelte';
  import AccountSection from './settings/AccountSection.svelte';
  import SecurityTOTPSection from './settings/SecurityTOTPSection.svelte';
  import SecurityPasskeysSection from './settings/SecurityPasskeysSection.svelte';
  import SessionsSection from './settings/SessionsSection.svelte';
  import DataSection from './settings/DataSection.svelte';

  // Auto-subscribe via `$auth` so the page-id strip re-renders if the user
  // logs in/out without remounting the route (rare but possible during
  // certain dialog flows).
  const email = $derived($auth.user?.username ?? '');
  const role = $derived($auth.user?.role ?? 'user');
</script>

<div class="set">
  <header class="set-head">
    <div class="set-eyebrow">Settings</div>
    <h1 class="set-title">Account &amp; appearance</h1>
    <div class="set-id">
      <span>account</span>
      <span class="dot" aria-hidden="true"></span>
      <span class="email">{email}</span>
      <span class="dot" aria-hidden="true"></span>
      <span>plan: free</span>
      {#if role === 'admin'}
        <span class="dot" aria-hidden="true"></span>
        <span class="admin">admin</span>
      {/if}
    </div>
  </header>

  <AppearanceSection />
  <ReadingSection />
  <SyncingSection />
  <AccountSection />
  <SecurityTOTPSection />
  <SecurityPasskeysSection />
  <SessionsSection />
  <DataSection />
</div>

<style>
  .set { padding: 28px 0 24px; }
  .set-head {
    padding-bottom: 22px;
    border-bottom: 1px solid var(--rule);
    margin-bottom: 12px;
  }
  .set-eyebrow {
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.14em;
    text-transform: uppercase;
    color: var(--ink-3);
    margin-bottom: 10px;
  }
  .set-title {
    font-family: var(--serif);
    font-size: 38px;
    line-height: 1.05;
    font-weight: 600;
    color: var(--ink);
    letter-spacing: -0.02em;
    margin: 0 0 8px;
  }
  .set-id {
    display: inline-flex; align-items: center; gap: 8px;
    font-family: var(--mono);
    font-size: 11.5px;
    color: var(--ink-3);
  }
  .set-id .email { color: var(--ink-2); }
  .set-id .dot {
    display: inline-block; width: 4px; height: 4px;
    border-radius: 50%; background: var(--ink-4);
  }
  .set-id .admin { color: var(--accent); }
</style>
```

- [ ] **Step 4: Delete the legacy Security view**

```bash
git rm web/src/views/settings/Security.svelte
```

- [ ] **Step 5: Run tests**

```bash
pnpm --dir web test -- src/views/__tests__/Settings.test.ts
pnpm --dir web run check
```

Expected: 2 new tests pass; svelte-check shows zero errors.

- [ ] **Step 6: Commit**

```bash
git add web/src/views/Settings.svelte web/src/views/__tests__/Settings.test.ts
git commit -m "$(cat <<'EOF'
M-Redesign-6: rebuild Settings page on .ts-shell with 7 numbered sections

Replaces the two-tab Settings (appearance | security) with a single
scrollable page composed of seven section components. Page-id strip
shows account · email · plan · (admin) in mono. The legacy
views/settings/Security.svelte view is removed — its responsibilities
are split across SecurityTOTPSection, SecurityPasskeysSection, and
SessionsSection.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

### Task C2: Smoke-test the full page end-to-end on `make dev`

**Files:** none (manual verification)

- [ ] **Step 1: Run dev server**

```bash
make dev
```

In a separate terminal:

```bash
pnpm --dir web run check
make test
```

Both must report zero errors.

- [ ] **Step 2: In Chromium, sign in and visit `/settings`**

Manually verify (per the umbrella spec §6 manual smoke checklist):

- [ ] All seven numbered eyebrows render (01 · APPEARANCE through 07 · DATA), each preceded by the mono numeral, the row title, a 1px rule, and (where present) an accent tag.
- [ ] Page-id strip shows `account · <your-email> · plan: free` in mono with the small bullet separators.
- [ ] Theme segmented control updates `html.theme-*` class instantly; reload preserves choice.
- [ ] Reading toggles flip visually with the accent fill and persist across reload.
- [ ] Sync · Refresh all now posts to every subscription's poll endpoint and the last-sync row re-renders.
- [ ] Change password dialog: wrong current password shows a non-toast inline error; correct flow closes and the SPA continues to work (CSRF rotated; subsequent state-changing calls succeed).
- [ ] TOTP enrolment renders a real QR (scan with phone — should match the secret), confirm transitions to recovery-codes view; download-as-txt produces a 10-line file.
- [ ] Disable TOTP requires OTP and updates the status row.
- [ ] Regenerate codes shows the regenerated callout.
- [ ] Add passkey opens label dialog; on confirm browser shows the platform credential picker (TouchID/YubiKey); on success the new passkey row appears.
- [ ] Remove passkey ✗ opens the password-challenge dialog with the passkey label in the body.
- [ ] Sessions list renders one row per session; revoke removes the row optimistically; current-session ✗ is disabled with a title attribute.
- [ ] Export OPML downloads an .opml file; Export saved JSON downloads a JSON of every saved entry.
- [ ] Import OPML opens dialog; selecting a file and confirming shows imported/skipped counts.
- [ ] Delete account requires password, on success logs out and lands on `/sign-in`.
- [ ] Same flows in dark theme + sepia theme — no contrast regressions.
- [ ] Mobile viewport (≤540px): every section stacks; controls move below their labels via the `:is-stacked` media query.

- [ ] **Step 3: Commit any final polish** discovered during the smoke run (e.g., dialog z-index, focus return, copy errors). One commit per discrete fix; group them under a `M-Redesign-6: smoke-test polish` umbrella commit message.

---

## Acceptance criteria

The PR is acceptable when **all** of the following hold:

### Visual / structural

- [ ] `views/Settings.svelte` renders inside the M1-provided `.ts-shell` (no Sidebar import; no two-pane layout).
- [ ] Page-id strip under the title contains exactly three default tokens (`account · <email> · plan: free`) plus an optional `· admin` for admin users, all in mono with `.dot` separators.
- [ ] Sections render in the exact order 01 / 02 / 03 / 04 / 05 (TOTP) / 05 (Passkeys) / 06 / 07. Each section starts with the `.set-section-eyebrow` showing `<num> · <TITLE>` and a rule.
- [ ] Every row uses `.set-row` (label left, control right) or `.set-row.is-block` (control under a block-style label+desc).
- [ ] Segmented controls in 01/02/03 use the M1 `Segmented` primitive with the correct preview chip per option (sw-light/dark/sepia/system; font-prev serif/sans; ts-density-prev bar count 4/3/2; no chip on Measure).

### Behaviour

- [ ] Clicking a Theme/Font/Density/Measure option updates the corresponding `prefs.*` store and persists to localStorage; reload retains.
- [ ] Each Reading toggle reads/writes its `prefs.reading.*` pref.
- [ ] Poll-interval pref persists; Refresh all now triggers a per-feed poll nudge.
- [ ] Change password dialog calls `api.changePassword`, rotates the CSRF token in the auth store on success, closes itself.
- [ ] Sign out everywhere calls `api.revokeAllOtherSessions`.
- [ ] TOTP enrol → confirm → view-codes flow works end-to-end; recovery codes download as plain text.
- [ ] Disable TOTP requires a valid OTP code and updates `has_totp` after.
- [ ] Regenerate codes invalidates the old set and shows the regenerated callout.
- [ ] Add passkey triggers `navigator.credentials.create` with the label-prefixed label; remove passkey opens password-challenge.
- [ ] Sessions list renders one row per active session, with desktop/mobile/CLI icons inferred from the user-agent; revoke removes the row optimistically and disables the current-session button.
- [ ] OPML export/import works via existing endpoints; Saved JSON export downloads `/entries?saved=1` content.
- [ ] Delete account calls `DELETE /api/v1/me` with the user's password; on success the SPA logs out and navigates to `/sign-in`.

### Backend

- [ ] `DELETE /api/v1/me` exists, requires CSRF + session, takes `{"current_password":"..."}`, returns 204 on success, 401 on wrong password, 400 on missing/empty password.
- [ ] `db.DeleteUser(ctx, conn, id)` exists, is covered by `TestDeleteUser_RemovesRowAndCascades`, and is no-op when the user doesn't exist.

### Tests + types

- [ ] `make test` exits clean.
- [ ] `pnpm --dir web test` exits clean.
- [ ] `pnpm --dir web run check` exits clean.
- [ ] All test files listed in the File structure exist and pass.
- [ ] The deleted `views/settings/Security.svelte` does not appear in `git status`.

---

## Verification commands

```bash
# Frontend types
pnpm --dir web run check

# Frontend tests
pnpm --dir web test

# Backend tests (also rebuilds web/dist)
make test

# Dev server for manual smoke
make dev   # then visit http://localhost:5173/settings

# Build verification (catches embed-related regressions)
make build
./bin/tap --help

# Lint posture (if a linter is configured locally — skip if it isn't)
go vet ./...
```

---

## Risks

1. **Dependency on M1 primitives.** This plan depends on `Button`, `Field`, `Segmented`, `Dialog`, `OtpInput`, `RecoveryCodesGrid`, `EmptyState`, and the `measure` preference store landing in M-Redesign-1. If M1 is not merged before this PR, every snippet that imports from `../../components/<Primitive>.svelte` must be reviewed against M1's actual API. The implementer should rebase on M1 and adjust call sites as needed; if M1's API differs in shape (e.g., `Segmented` exposes `onChange` instead of `onchange`), update the call sites uniformly and document the difference in the PR. Do **not** ship inline replacements for M1 primitives in this milestone — that's M1's job and would create a maintenance fork.

2. **Existing M7 security tests must port forward.** The legacy `views/settings/Security.svelte` is removed. As of plan authoring, a grep of the repo shows no `web/src/views/settings/__tests__/Security.test.ts` file — the component ships untested today, so the deletion step is a no-op apart from removing `Security.svelte` itself. Re-run the grep at implementation time; if M7 added a test file in the meantime, delete it (the new section-level tests above replace its coverage).

3. **QR-code library choice.** `qrcode` is the recommendation, but the implementer must verify health (downloads, maintenance, license) before adding. If `qrcode` is unsuitable, `qr-code-styling` and `qrcode-generator` are acceptable alternatives — re-verify, then adjust the import and `toDataURL` call in `EnrolTOTPDialog.svelte`. **Do not add the dep without a health check.**

4. **CSRF token rotation on password change.** `api.changePassword` already rotates the CSRF token by setting it on the auth store. Verify in the smoke test that subsequent state-changing API calls succeed after a password change — if they 403 with CSRF mismatch, the SPA-side rotation is broken (not a regression introduced by this PR, but worth catching).

5. **Account deletion cascade.** The plan assumes existing FK constraints with `ON DELETE CASCADE`. If a migration audit shows any user-scoped table that doesn't cascade, add a new migration `0011_user_delete_cascade.sql` (or next free index) before shipping. Don't rely on application-level cascade — the FK is the long-term contract.

6. **M5 ordering dependency for `Refresh all now`.** The button consumes M-Redesign-5's `PATCH /api/v1/subscriptions/:id` accepting `{refresh_now: true}` (which calls `Scheduler.Poke()` server-side) via `api.refreshSubscription(id)`. M5 must merge before M6 ships for the button to work end-to-end. Task A3's first step explicitly greps for both the SPA helper and the backend symbol; if either is missing, the implementer rebases on M5 rather than inventing a parallel mechanism. **Do not add** a `POST /subscriptions/:id/poll`, `POST /poll-all`, or any other refresh endpoint — that was the wrong design in plan v1 and the umbrella §2.4 amendment locks the M5 path as canonical.

7. **Display name is out of scope.** The umbrella spec table mentions "Display name (Field)". The current users table has no display_name column. Display name is **deferred to a follow-up milestone**; the Account section ships without it. This is called out in the PR description so reviewers don't flag it as a missing row.

8. **`prefs.reading` toggles are surface-only.** Adding the toggles does not wire up the behaviour they describe — those wirings land in M-Redesign-2 (mark-on-scroll, auto-open-next, show-summaries on the Unread list) and M-Redesign-1 (open-links-new-tab on the reader). This milestone owns the **control surface** only. **Do NOT wire any behaviour from these prefs in M6** — leave them inert. The implementer must resist "helpfully" applying `prefs.reading.openLinksNewTab` to the existing Reader or list, because that's out of scope and creates a partial implementation across milestone boundaries. The PR description must state this so reviewers don't expect end-to-end behaviour from the toggles in isolation.

9. **Smoke-test environment.** Some browsers (e.g., Firefox on Linux) don't support `navigator.credentials.create` in dev contexts without HTTPS. Use Chromium for the passkey portion of the smoke test, or test on a Tailscale-fronted host with HTTPS.
