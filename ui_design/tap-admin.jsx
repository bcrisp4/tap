// Tap — Admin page.
// Admin-only surface. User management + System status.
// Same .ts-shell as Settings/Feeds. Centered single column.

const { useState: useStateAdmin } = React;

// ─────────────────────────────────────────────
// Sample data (purely presentational)
// ─────────────────────────────────────────────

const SAMPLE_USERS = [
  {
    id: "u1", username: "liz",            role: "admin",
    created: "Feb 14, 2026", lastSeen: "now",
    disabled: false, totp: true,  passkeys: 3, isYou: true,
  },
  {
    id: "u2", username: "marcus",         role: "admin",
    created: "Feb 14, 2026", lastSeen: "today",
    disabled: false, totp: true,  passkeys: 2,
  },
  {
    id: "u3", username: "anna",           role: "user",
    created: "Mar 02, 2026", lastSeen: "3 h ago",
    disabled: false, totp: true,  passkeys: 1,
  },
  {
    id: "u4", username: "jpark",          role: "user",
    created: "Mar 18, 2026", lastSeen: "yesterday",
    disabled: false, totp: false, passkeys: 0,
  },
  {
    id: "u5", username: "ravi.s",         role: "user",
    created: "Apr 02, 2026", lastSeen: "2 d ago",
    disabled: false, totp: true,  passkeys: 1,
  },
  {
    id: "u6", username: "kbarrow",        role: "user",
    created: "Apr 11, 2026", lastSeen: "5 d ago",
    disabled: true,  totp: false, passkeys: 0,
  },
  {
    id: "u7", username: "guest-readonly", role: "user",
    created: "Apr 27, 2026", lastSeen: "never",
    disabled: false, totp: false, passkeys: 0,
  },
];

const SAMPLE_ERRORS = [
  { t: "10:42:14", level: "error", event: "phoronix.com — 502 from origin, retrying in 5m" },
  { t: "10:38:41", level: "warn",  event: "poll worker 3 slow: 28 feeds queued, p95 1.8s" },
  { t: "09:58:03", level: "error", event: "lwn.net — feed parse: unexpected <rdf:RDF> root" },
  { t: "09:12:50", level: "info",  event: "user liz signed in (passkey · MacBook Air)" },
  { t: "08:14:39", level: "info",  event: "jvns.ca — 304 not modified (no new entries)" },
  { t: "06:01:22", level: "warn",  event: "rate-limit cooldown on api.github.com (60s)" },
  { t: "03:02:11", level: "info",  event: "scheduled vacuum complete · 38 rows reclaimed" },
  { t: "02:45:08", level: "info",  event: "backup snapshot written · 11.4 mb · ok" },
];

// ─────────────────────────────────────────────
// Local building blocks (admin-specific)
// ─────────────────────────────────────────────

function AdminBtn({ children, kind, ...rest }) {
  const k = kind ? `is-${kind}` : "";
  return <button type="button" className={`ts-btn ${k}`} {...rest}>{children}</button>;
}

function AdminPill({ tone = "neutral", children }) {
  return <span className={`ts-pill is-${tone}`}>{children}</span>;
}

function RoleTag({ role }) {
  return (
    <span className={`ts-role is-${role}`}>
      <span className="ts-role-dot" aria-hidden="true"></span>
      {role}
    </span>
  );
}

function AdmIcon({ name, s = 14 }) {
  const stroke = { fill: "none", stroke: "currentColor", strokeWidth: 1.4, strokeLinecap: "round", strokeLinejoin: "round" };
  switch (name) {
    case "key":     return <svg width={s} height={s} viewBox="0 0 16 16" {...stroke}><circle cx="6" cy="8" r="2.6"/><path d="M8.5 8h6M12 8v2M14 8v2.5"/></svg>;
    case "shield":  return <svg width={s} height={s} viewBox="0 0 16 16" {...stroke}><path d="M8 2 3 3.6v4.3c0 3 2.1 5.4 5 6.1 2.9-.7 5-3.1 5-6.1V3.6L8 2z"/></svg>;
    case "lock":    return <svg width={s} height={s} viewBox="0 0 16 16" {...stroke}><rect x="3.5" y="7" width="9" height="6.5" rx="1.2"/><path d="M5.5 7V5.2a2.5 2.5 0 0 1 5 0V7"/></svg>;
    case "minus":   return <svg width={s} height={s} viewBox="0 0 16 16" {...stroke}><path d="M3.5 8h9"/></svg>;
    case "plus":    return <svg width={s} height={s} viewBox="0 0 16 16" {...stroke}><path d="M8 3.5v9M3.5 8h9"/></svg>;
    case "trash":   return <svg width={s} height={s} viewBox="0 0 16 16" {...stroke}><path d="M3 4.5h10M6.5 4.5V3h3v1.5M4.5 4.5l.6 8.2a1.2 1.2 0 0 0 1.2 1.1h3.4a1.2 1.2 0 0 0 1.2-1.1l.6-8.2"/><path d="M7 7v4M9 7v4"/></svg>;
    case "refresh": return <svg width={s} height={s} viewBox="0 0 16 16" {...stroke}><path d="M14 8a6 6 0 1 1-1.76-4.24"/><path d="M14 2.5V6h-3.5"/></svg>;
    case "copy":    return <svg width={s} height={s} viewBox="0 0 16 16" {...stroke}><rect x="2.5" y="2.5" width="8" height="8" rx="1.2"/><path d="M5.5 13.5h6a1.2 1.2 0 0 0 1.2-1.2v-6"/></svg>;
    case "x":       return <svg width={s} height={s} viewBox="0 0 16 16" {...stroke}><path d="M4 4l8 8M12 4l-8 8"/></svg>;
    case "power":   return <svg width={s} height={s} viewBox="0 0 16 16" {...stroke}><path d="M8 2.5V8"/><path d="M5 4.5a4.5 4.5 0 1 0 6 0"/></svg>;
    case "dot":     return <svg width={s} height={s} viewBox="0 0 16 16"><circle cx="8" cy="8" r="3" fill="currentColor"/></svg>;
    default:        return null;
  }
}

// ─────────────────────────────────────────────
// User row
// ─────────────────────────────────────────────

function UserRow({ user, selected = false, onAction }) {
  const cls = [
    "ts-urow",
    user.disabled ? "is-disabled" : "",
    user.isYou ? "is-you" : "",
    selected ? "is-selected" : "",
  ].filter(Boolean).join(" ");

  return (
    <div className={cls}>
      <div className="ts-urow-cell ts-urow-user">
        <span className="ts-urow-avatar" aria-hidden="true">
          {user.username[0].toUpperCase()}
        </span>
        <div className="ts-urow-stack">
          <div className="ts-urow-name">
            <span>{user.username}</span>
            {user.isYou && <span className="ts-urow-you">you</span>}
          </div>
          <div className="ts-urow-sub">
            last seen {user.lastSeen}
          </div>
        </div>
      </div>

      <div className="ts-urow-cell">
        <RoleTag role={user.role} />
      </div>

      <div className="ts-urow-cell ts-urow-date">{user.created}</div>

      <div className="ts-urow-cell">
        {user.disabled
          ? <AdminPill tone="muted">disabled</AdminPill>
          : <AdminPill tone="active">active</AdminPill>}
      </div>

      <div className="ts-urow-cell">
        {user.totp
          ? <span className="ts-flag is-on"><AdmIcon name="shield" s={12} />enabled</span>
          : <span className="ts-flag is-off">not set</span>}
      </div>

      <div className="ts-urow-cell ts-urow-pk">
        <AdmIcon name="key" s={12} />
        <span>{user.passkeys}</span>
      </div>

      <div className="ts-urow-cell ts-urow-actions">
        <button
          type="button"
          className="ts-urow-act"
          title="Reset password"
          onClick={() => onAction?.("reset", user)}
        >
          <AdmIcon name="refresh" s={13} />
          <span>Reset password</span>
        </button>
        <button
          type="button"
          className="ts-urow-act"
          title="Disable two-factor"
          disabled={!user.totp}
          onClick={() => onAction?.("disable2fa", user)}
        >
          <AdmIcon name="shield" s={13} />
          <span>Disable 2FA</span>
        </button>
        {user.disabled ? (
          <button
            type="button"
            className="ts-urow-act"
            title="Re-enable account"
            onClick={() => onAction?.("enable", user)}
          >
            <AdmIcon name="power" s={13} />
            <span>Re-enable</span>
          </button>
        ) : (
          <button
            type="button"
            className="ts-urow-act"
            title="Disable account"
            disabled={user.isYou}
            onClick={() => onAction?.("disableUser", user)}
          >
            <AdmIcon name="minus" s={13} />
            <span>Disable</span>
          </button>
        )}
        <button
          type="button"
          className="ts-urow-act is-danger"
          title={user.isYou ? "You can't delete yourself" : "Delete user"}
          disabled={user.isYou}
          onClick={() => onAction?.("delete", user)}
        >
          <AdmIcon name="trash" s={13} />
          <span>Delete</span>
        </button>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// Dialogs
// ─────────────────────────────────────────────

function AdmOverlay({ children }) {
  return <div className="ts-overlay">{children}</div>;
}

function CreateUserDialog({ values = {} }) {
  const v = { username: "kira.lin", password: "summer-deck-quiet-9421", role: "user", ...values };
  return (
    <AdmOverlay>
      <div className="ts-dialog">
        <div className="ts-dialog-head">
          <div className="ts-dialog-title">Create user</div>
          <button type="button" className="ts-dialog-close" aria-label="Close">
            <AdmIcon name="x" s={14} />
          </button>
        </div>
        <div className="ts-dialog-body">
          <p className="ts-dialog-p">
            New users sign in with this password and are prompted to enrol in two-factor on first
            successful sign-in.
          </p>

          <div className="ts-field-label">Username</div>
          <input className="ts-field" defaultValue={v.username} />

          <div style={{ marginTop: 14 }}>
            <div className="ts-field-label">Initial password</div>
            <div className="ts-field-with-aff">
              <input className="ts-field" defaultValue={v.password} />
              <button type="button" className="ts-aff-btn" title="Generate new">
                <AdmIcon name="refresh" s={13} />
              </button>
            </div>
            <div className="ts-field-hint">
              <span>20 chars · 4-word passphrase</span>
              <span className="ts-field-hint-sep" aria-hidden="true">·</span>
              <span>shown once, copy before saving</span>
            </div>
          </div>

          <div style={{ marginTop: 18 }}>
            <div className="ts-field-label">Role</div>
            <div className="ts-segmented" role="radiogroup">
              <button type="button" className={`ts-segmented-btn ${v.role === "user" ? "is-active" : ""}`}>
                <span>User</span>
              </button>
              <button type="button" className={`ts-segmented-btn ${v.role === "admin" ? "is-active" : ""}`}>
                <span>Admin</span>
              </button>
            </div>
            <div className="ts-field-hint">
              {v.role === "admin"
                ? "Admins can manage users and view system status."
                : "Users can read, save and manage their own feeds."}
            </div>
          </div>
        </div>
        <div className="ts-dialog-foot">
          <div className="ts-dialog-foot-l">password is shown once</div>
          <AdminBtn kind="quiet">Cancel</AdminBtn>
          <AdminBtn kind="primary">Create user</AdminBtn>
        </div>
      </div>
    </AdmOverlay>
  );
}

function ResetPasswordDialog({ user, password = "amber-loop-shadow-2840" }) {
  return (
    <AdmOverlay>
      <div className="ts-dialog">
        <div className="ts-dialog-head">
          <div className="ts-dialog-title">Temporary password for <b>{user.username}</b></div>
          <button type="button" className="ts-dialog-close" aria-label="Close">
            <AdmIcon name="x" s={14} />
          </button>
        </div>
        <div className="ts-dialog-body">
          <p className="ts-dialog-p">
            This password works once. {user.username} will be asked to set a new one on next sign-in.
            Share it through a secure channel — it will not be shown again.
          </p>

          <div className="ts-temp-pass">
            <span className="ts-temp-pass-key">{password}</span>
            <button type="button" className="ts-temp-pass-copy">
              <AdmIcon name="copy" s={12} />
              <span>Copy</span>
            </button>
          </div>

          <div className="ts-dialog-warn">
            All existing sessions for {user.username} were just signed out. Any passkeys remain valid.
          </div>
        </div>
        <div className="ts-dialog-foot">
          <div className="ts-dialog-foot-l">shown once</div>
          <AdminBtn>Download .txt</AdminBtn>
          <AdminBtn kind="primary">Done</AdminBtn>
        </div>
      </div>
    </AdmOverlay>
  );
}

function ConfirmDialog({ title, body, ctaLabel = "Confirm", danger = false, list = null, foot = null }) {
  return (
    <AdmOverlay>
      <div className="ts-dialog">
        <div className="ts-dialog-head">
          <div className="ts-dialog-title">{title}</div>
          <button type="button" className="ts-dialog-close" aria-label="Close">
            <AdmIcon name="x" s={14} />
          </button>
        </div>
        <div className="ts-dialog-body">
          <p className="ts-dialog-p">{body}</p>
          {list}
        </div>
        <div className="ts-dialog-foot">
          {foot && <div className="ts-dialog-foot-l">{foot}</div>}
          <AdminBtn kind="quiet">Cancel</AdminBtn>
          <AdminBtn kind={danger ? "danger" : "primary"}>{ctaLabel}</AdminBtn>
        </div>
      </div>
    </AdmOverlay>
  );
}

// ─────────────────────────────────────────────
// System status
// ─────────────────────────────────────────────

function LevelTag({ level }) {
  return <span className={`ts-lvl is-${level}`}>{level}</span>;
}

function SystemStatus({ live = true, expanded = false }) {
  const errors = expanded ? SAMPLE_ERRORS : SAMPLE_ERRORS.slice(0, 5);
  return (
    <>
      <div className="ts-sys-headstrip">
        <div className="ts-sys-headstrip-l">
          <span className={`ts-live ${live ? "is-on" : ""}`} aria-hidden="true">
            <span className="ts-live-dot"></span>
          </span>
          <span>
            {live
              ? <>auto-refreshing every <b>5s</b></>
              : <>auto-refresh paused</>}
          </span>
        </div>
        <div className="ts-sys-headstrip-r">
          <span>updated 00:02s ago</span>
        </div>
      </div>

      <div className="ts-sys-grid">
        <div className="ts-sys-cell">
          <span className="ts-sys-cell-l">Version</span>
          <span className="ts-sys-cell-v">v1.4.2</span>
          <span className="ts-sys-cell-sub">commit 8b3f9a1 · go 1.22</span>
        </div>
        <div className="ts-sys-cell">
          <span className="ts-sys-cell-l">Uptime</span>
          <span className="ts-sys-cell-v">14d 06h 12m</span>
          <span className="ts-sys-cell-sub">since Apr 26 04:30</span>
        </div>
        <div className="ts-sys-cell">
          <span className="ts-sys-cell-l">Database</span>
          <span className="ts-sys-cell-v ok">ok</span>
          <span className="ts-sys-cell-sub">postgres 16.2 · 412 mb</span>
        </div>
        <div className="ts-sys-cell">
          <span className="ts-sys-cell-l">Active polls</span>
          <span className="ts-sys-cell-v">36 <span className="ts-sys-cell-frac">/ 248</span></span>
          <span className="ts-sys-cell-sub">next sweep in 04m 12s</span>
        </div>
      </div>

      <div className="ts-sys-errors-head">
        <span>Recent events</span>
        <span className="rule" aria-hidden="true"></span>
        <span className="ts-sys-errors-count">ring buffer · last {errors.length} of 256</span>
      </div>

      <div className="ts-sys-errors2">
        {errors.map((e, i) => (
          <div key={i} className={`ts-sys-err2 is-${e.level}`}>
            <span className="ts-sys-err2-t">{e.t}</span>
            <LevelTag level={e.level} />
            <span className="ts-sys-err2-m">{e.event}</span>
          </div>
        ))}
      </div>

      <div className="ts-actions" style={{ marginTop: 14 }}>
        <AdminBtn kind="quiet">Open full logs →</AdminBtn>
        <AdminBtn kind="quiet">Re-run all polls now</AdminBtn>
        <AdminBtn kind="quiet">{live ? "Pause auto-refresh" : "Resume auto-refresh"}</AdminBtn>
      </div>
    </>
  );
}

// ─────────────────────────────────────────────
// The page
// ─────────────────────────────────────────────

function TapAdmin({
  theme = "light",
  fontMode = "serif",
  overlay = null,     // null | "create" | "reset" | "disable2fa" | "delete" | "disable" | "enable"
  overlayUser = null, // user object for overlays
  highlightId = null, // row id to render selected
  live = true,
  expandedErrors = false,
}) {
  const target = overlayUser || SAMPLE_USERS.find((u) => u.id === highlightId) || SAMPLE_USERS[3];

  return (
    <div className="ts-set ts-admin">
      <header className="ts-set-head">
        <div className="ts-set-eyebrow">Admin</div>
        <h1 className="ts-set-title">Instance &amp; users</h1>
        <div className="ts-set-id">
          <span>Signed in as <b>liz</b></span>
          <span className="dot" aria-hidden="true"></span>
          <span style={{ color: "var(--accent)" }}>admin</span>
          <span className="dot" aria-hidden="true"></span>
          <span>tap.local · v1.4.2</span>
        </div>
      </header>

      {/* ─── Users ─── */}
      <section className="ts-set-section">
        <div className="ts-set-section-eyebrow">
          <span>User management</span>
          <span className="rule" aria-hidden="true"></span>
          <span className="tag">{SAMPLE_USERS.length} accounts</span>
        </div>

        <div className="ts-admin-toolbar">
          <div className="ts-admin-toolbar-l">
            <div className="ts-search">
              <span className="ts-search-ico" aria-hidden="true">
                <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round"><circle cx="7" cy="7" r="4.5"/><path d="m10.5 10.5 3 3"/></svg>
              </span>
              <input className="ts-search-input" placeholder="Filter by username" />
            </div>
            <div className="ts-admin-filters">
              <button type="button" className="ts-chip is-active">All</button>
              <button type="button" className="ts-chip">Admins</button>
              <button type="button" className="ts-chip">Users</button>
              <button type="button" className="ts-chip">Disabled</button>
            </div>
          </div>
          <AdminBtn kind="primary">
            <AdmIcon name="plus" s={12} />
            <span>Create user</span>
          </AdminBtn>
        </div>

        <div className="ts-utable">
          <div className="ts-utable-head">
            <div className="ts-urow-cell">User</div>
            <div className="ts-urow-cell">Role</div>
            <div className="ts-urow-cell">Created</div>
            <div className="ts-urow-cell">Status</div>
            <div className="ts-urow-cell">2FA</div>
            <div className="ts-urow-cell">Passkeys</div>
            <div className="ts-urow-cell ts-urow-actions-h">Actions</div>
          </div>
          {SAMPLE_USERS.map((u) => (
            <UserRow
              key={u.id}
              user={u}
              selected={highlightId === u.id}
            />
          ))}
        </div>
      </section>

      {/* ─── System status ─── */}
      <section className="ts-set-section">
        <div className="ts-set-section-eyebrow">
          <span>System status</span>
          <span className="rule" aria-hidden="true"></span>
          <span className="tag">live</span>
        </div>
        <SystemStatus live={live} expanded={expandedErrors} />
      </section>

      {/* ─── Overlays ─── */}
      {overlay === "create" && <CreateUserDialog />}
      {overlay === "reset" && <ResetPasswordDialog user={target} />}
      {overlay === "disable2fa" && (
        <ConfirmDialog
          title={<>Disable two-factor for <b>{target.username}</b>?</>}
          body={<>This removes the authenticator binding from {target.username}'s account.
            They'll sign in with password only until they re-enrol. Their recovery codes are
            invalidated immediately.</>}
          danger
          ctaLabel="Disable TOTP"
          foot="acts immediately"
        />
      )}
      {overlay === "delete" && (
        <ConfirmDialog
          title={<>Delete <b>{target.username}</b>?</>}
          body={<>This permanently removes {target.username}, all their feeds, saved entries and
            sessions. This can't be undone.</>}
          danger
          ctaLabel={`Delete ${target.username}`}
          foot="permanent · cannot be undone"
          list={(
            <ul className="ts-dialog-list">
              <li className="ts-dialog-list-item">
                <span className="ts-dialog-list-item-name">{target.passkeys} passkey{target.passkeys === 1 ? "" : "s"} revoked</span>
              </li>
              <li className="ts-dialog-list-item">
                <span className="ts-dialog-list-item-name">42 subscribed feeds released</span>
              </li>
              <li className="ts-dialog-list-item">
                <span className="ts-dialog-list-item-name">2,148 read/save records purged</span>
              </li>
            </ul>
          )}
        />
      )}
      {overlay === "disable" && (
        <ConfirmDialog
          title={<>Disable <b>{target.username}</b>?</>}
          body={<>{target.username} will be signed out everywhere and can't sign back in until you
            re-enable the account. Their data and feeds are preserved.</>}
          ctaLabel="Disable account"
          foot="reversible"
        />
      )}
      {overlay === "enable" && (
        <ConfirmDialog
          title={<>Re-enable <b>{target.username}</b>?</>}
          body={<>{target.username} will be able to sign in again with their existing password.
            Their feeds and saved entries are unchanged.</>}
          ctaLabel="Re-enable account"
        />
      )}
    </div>
  );
}

Object.assign(window, {
  TapAdmin,
  SAMPLE_USERS,
});
