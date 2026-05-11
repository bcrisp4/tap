// Tap — Settings page.
// Centered single-column, matches .ts-shell width.
// Sections: Appearance · Security (sessions, two-factor, passkeys) · System (admin).

const { useState: useStateSet } = React;

// ─────────────────────────────────────────────
// Sample state (purely presentational — these are static designs)
// ─────────────────────────────────────────────

const SAMPLE_SESSIONS = [
  {
    id: "s1", device: "Firefox · macOS 14", ip: "192.0.2.41",
    location: "Berlin, DE", when: "now", current: true, kind: "desktop",
  },
  {
    id: "s2", device: "Safari · iOS 17", ip: "192.0.2.41",
    location: "Berlin, DE", when: "2 h ago", kind: "mobile",
  },
  {
    id: "s3", device: "Tap CLI · ip-10-0-1-4", ip: "10.0.1.4",
    location: "eu-central-1", when: "3 d ago", kind: "cli",
  },
];

const SAMPLE_PASSKEYS = [
  { id: "p1", label: "MacBook Air · Touch ID",  added: "Mar 12, 2026", lastUsed: "today" },
  { id: "p2", label: "iPhone 15 · Face ID",     added: "Apr 02, 2026", lastUsed: "yesterday" },
  { id: "p3", label: "YubiKey 5C — Office",     added: "Feb 28, 2026", lastUsed: "12 d ago" },
];

const SAMPLE_CODES = [
  "WJ4R-K9PA-2NX7", "TQ6L-3HMC-V8YD", "B27Z-AERX-9KLP", "5VHN-DT4U-XQ8M", "PG9C-7JWS-NA3R",
  "Y6KX-RB2D-VEM9", "ZA8L-4WPN-7HJF", "MQ3U-X9CB-K5LD", "HN7P-2BVE-RT8X", "LC4J-WK6M-FQ9A",
];

const TOTP_SECRET = "JBSW Y3DP EHPK 3PXP JBSW Y3DP";

// ─────────────────────────────────────────────
// Building blocks
// ─────────────────────────────────────────────

function SetSection({ title, tag, children }) {
  return (
    <section className="ts-set-section">
      <div className="ts-set-section-eyebrow">
        <span>{title}</span>
        <span className="rule" aria-hidden="true"></span>
        {tag && <span className="tag">{tag}</span>}
      </div>
      {children}
    </section>
  );
}

function SetRow({ label, desc, control, stacked = false, block = false, children }) {
  const cls = ["ts-set-row", stacked && "is-stacked", block && "is-block"].filter(Boolean).join(" ");
  return (
    <div className={cls}>
      <div>
        <div className="ts-set-label">{label}</div>
        {desc && <div className="ts-set-desc">{desc}</div>}
        {block && children}
      </div>
      {!block && (control ?? null)}
    </div>
  );
}

function Segmented({ value, options, onChange }) {
  return (
    <div className="ts-segmented" role="radiogroup">
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={value === o.value}
          className={`ts-segmented-btn ${value === o.value ? "is-active" : ""}`}
          onClick={() => onChange?.(o.value)}
        >
          {o.swatch && <span className={`sw sw-${o.swatch}`} aria-hidden="true"></span>}
          {o.fontPrev && (
            <span className={`font-prev ${o.fontPrev}`} aria-hidden="true">Aa</span>
          )}
          {o.density && (
            <span className="ts-density-prev" aria-hidden="true">
              {Array.from({ length: o.density }).map((_, i) => <span key={i}></span>)}
            </span>
          )}
          <span>{o.label}</span>
        </button>
      ))}
    </div>
  );
}

function Btn({ children, kind, ...rest }) {
  const k = kind ? `is-${kind}` : "";
  return <button type="button" className={`ts-btn ${k}`} {...rest}>{children}</button>;
}

function IconDesktop() {
  return <svg width="18" height="18" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"><rect x="2" y="3" width="16" height="11" rx="1.2"/><path d="M7 17h6M10 14v3"/></svg>;
}
function IconMobile() {
  return <svg width="18" height="18" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"><rect x="6" y="2" width="8" height="16" rx="1.5"/><path d="M9.5 15.5h1"/></svg>;
}
function IconTerm() {
  return <svg width="18" height="18" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"><rect x="2" y="4" width="16" height="12" rx="1.2"/><path d="M5 8l2.5 2L5 12M10 13h4"/></svg>;
}
function IconKey() {
  return <svg width="16" height="16" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round"><circle cx="7" cy="10" r="3.2"/><path d="M10 10h7M14 10v2.5M16.5 10v3"/></svg>;
}
function IconX() {
  return <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round"><path d="M4 4l8 8M12 4l-8 8"/></svg>;
}
function IconClose() {
  return <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round"><path d="M4 4l8 8M12 4l-8 8"/></svg>;
}

function SessIcon({ kind }) {
  if (kind === "mobile") return <IconMobile />;
  if (kind === "cli") return <IconTerm />;
  return <IconDesktop />;
}

// ─────────────────────────────────────────────
// Dialogs (static — rendered as overlays inside the ts-shell)
// ─────────────────────────────────────────────

function OtpDisplay({ value = "", focusAt = -1 }) {
  const cells = Array.from({ length: 6 }).map((_, i) => {
    const ch = value[i] || "";
    const cls = [
      "ts-otp-cell",
      !ch && "is-empty",
      i === focusAt && "is-focus",
    ].filter(Boolean).join(" ");
    return <div key={i} className={cls}>{ch || "·"}</div>;
  });
  return <div className="ts-otp">{cells}</div>;
}

function EnrollDialog() {
  return (
    <div className="ts-overlay">
      <div className="ts-dialog">
        <div className="ts-dialog-head">
          <div className="ts-dialog-title">Set up authenticator</div>
          <button type="button" className="ts-dialog-close" aria-label="Close"><IconClose /></button>
        </div>
        <div className="ts-dialog-body">
          <div className="ts-totp-setup">
            <div className="ts-qr" aria-hidden="true">
              <div className="ts-qr-pattern"></div>
              <div className="ts-qr-corner tl"><div className="core"></div></div>
              <div className="ts-qr-corner tr"><div className="core"></div></div>
              <div className="ts-qr-corner bl"><div className="core"></div></div>
            </div>
            <div className="right">
              <div className="step">Step 1 · Scan or enter secret</div>
              <p className="ts-dialog-p" style={{ marginBottom: 10, fontSize: 14 }}>
                Add Tap to your authenticator app — 1Password, Authy, or any TOTP-compatible client.
              </p>
              <div className="ts-secret">
                <span className="ts-secret-key">{TOTP_SECRET}</span>
                <button type="button" className="ts-secret-copy">Copy</button>
              </div>
            </div>
          </div>

          <div style={{ marginTop: 22 }}>
            <div className="ts-field-label">Step 2 · Confirm code from your app</div>
            <OtpDisplay value="426" focusAt={3} />
          </div>
        </div>
        <div className="ts-dialog-foot">
          <div className="ts-dialog-foot-l">Algorithm SHA-1 · 30s · 6 digits</div>
          <Btn kind="quiet">Cancel</Btn>
          <Btn kind="primary">Enable</Btn>
        </div>
      </div>
    </div>
  );
}

function RecoveryCodesDialog({ regenerated = false }) {
  return (
    <div className="ts-overlay">
      <div className="ts-dialog">
        <div className="ts-dialog-head">
          <div className="ts-dialog-title">{regenerated ? "New recovery codes" : "Your recovery codes"}</div>
          <button type="button" className="ts-dialog-close" aria-label="Close"><IconClose /></button>
        </div>
        <div className="ts-dialog-body">
          <p className="ts-dialog-p">
            Each code is single-use. Store them in a password manager — they replace your
            authenticator if you lose access.
          </p>
          <div className="ts-codes">
            {SAMPLE_CODES.map((c, i) => (
              <div key={c} className="ts-code">
                <span className="n">{String(i + 1).padStart(2, "0")}</span>
                <span>{c}</span>
              </div>
            ))}
          </div>
          {regenerated && (
            <div className="ts-dialog-warn">
              The previous set of codes was just invalidated. Anything saved before now will no
              longer work.
            </div>
          )}
        </div>
        <div className="ts-dialog-foot">
          <div className="ts-dialog-foot-l">10 codes · plain text</div>
          <Btn>Download .txt</Btn>
          <Btn kind="primary">I've saved them</Btn>
        </div>
      </div>
    </div>
  );
}

function CodeChallengeDialog({ title, body, danger = false, ctaLabel = "Confirm" }) {
  return (
    <div className="ts-overlay">
      <div className="ts-dialog">
        <div className="ts-dialog-head">
          <div className="ts-dialog-title">{title}</div>
          <button type="button" className="ts-dialog-close" aria-label="Close"><IconClose /></button>
        </div>
        <div className="ts-dialog-body">
          <p className="ts-dialog-p">{body}</p>
          <div className="ts-field-label">6-digit code from your authenticator</div>
          <OtpDisplay value="91" focusAt={2} />
        </div>
        <div className="ts-dialog-foot">
          <Btn kind="quiet">Cancel</Btn>
          <Btn kind={danger ? "danger" : "primary"}>{ctaLabel}</Btn>
        </div>
      </div>
    </div>
  );
}

function PasswordChallengeDialog({ title, body, danger = false, ctaLabel = "Confirm" }) {
  return (
    <div className="ts-overlay">
      <div className="ts-dialog">
        <div className="ts-dialog-head">
          <div className="ts-dialog-title">{title}</div>
          <button type="button" className="ts-dialog-close" aria-label="Close"><IconClose /></button>
        </div>
        <div className="ts-dialog-body">
          <p className="ts-dialog-p">{body}</p>
          <div className="ts-field-label">Account password</div>
          <input className="ts-field" type="password" defaultValue="••••••••••••" />
        </div>
        <div className="ts-dialog-foot">
          <div className="ts-dialog-foot-l">re-authenticating</div>
          <Btn kind="quiet">Cancel</Btn>
          <Btn kind={danger ? "danger" : "primary"}>{ctaLabel}</Btn>
        </div>
      </div>
    </div>
  );
}

function AddPasskeyDialog() {
  return (
    <div className="ts-overlay">
      <div className="ts-dialog">
        <div className="ts-dialog-head">
          <div className="ts-dialog-title">Add a passkey</div>
          <button type="button" className="ts-dialog-close" aria-label="Close"><IconClose /></button>
        </div>
        <div className="ts-dialog-body">
          <p className="ts-dialog-p">
            Give this passkey a memorable label so you can recognise it later — your device's name,
            or where it lives.
          </p>
          <div className="ts-field-label">Label</div>
          <input className="ts-field" defaultValue="MacBook Pro · Touch ID" />
          <p className="ts-dialog-p" style={{ marginTop: 18, marginBottom: 0, fontSize: 13.5 }}>
            On the next step, your browser will ask which device or security key to use.
          </p>
        </div>
        <div className="ts-dialog-foot">
          <Btn kind="quiet">Cancel</Btn>
          <Btn kind="primary">Use this device →</Btn>
        </div>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// The page
// ─────────────────────────────────────────────

function TapSettings({
  theme = "light",
  fontMode = "serif",
  density = "default",
  totp = "off",            // "off" | "on"
  admin = false,
  overlay = null,          // null | "enroll" | "codes" | "codes-regen" | "disable" | "regen" | "addpk" | "removepk"
  onSetTheme,
  onSetFont,
  onSetDensity,
}) {
  return (
    <div className="ts-set">
      <header className="ts-set-head">
        <div className="ts-set-eyebrow">Settings</div>
        <h1 className="ts-set-title">Account &amp; appearance</h1>
        <div className="ts-set-id">
          <span>Signed in as <b>liz@hauck.studio</b></span>
          {admin && (
            <>
              <span className="dot" aria-hidden="true"></span>
              <span style={{ color: "var(--accent)" }}>admin</span>
            </>
          )}
        </div>
      </header>

      {/* ─── Appearance ─── */}
      <SetSection title="Appearance">
        <SetRow
          label="Theme"
          desc={<>Follow the OS, or pin Tap to a single palette.</>}
          control={
            <Segmented
              value={theme === "system" ? "system" : theme}
              options={[
                { value: "system", label: "System", swatch: "system" },
                { value: "light",  label: "Light",  swatch: "light" },
                { value: "dark",   label: "Dark",   swatch: "dark" },
                { value: "sepia",  label: "Sepia",  swatch: "sepia" },
              ]}
              onChange={onSetTheme}
            />
          }
        />
        <SetRow
          label="Reading font"
          desc={<>Applies to article bodies and entry titles. UI stays sans-serif.</>}
          control={
            <Segmented
              value={fontMode}
              options={[
                { value: "serif", label: "Serif",      fontPrev: "serif" },
                { value: "sans",  label: "Sans-serif", fontPrev: "sans" },
              ]}
              onChange={onSetFont}
            />
          }
        />
        <SetRow
          label="Density"
          desc={<>Compact hides summaries; comfortable adds breathing room.</>}
          control={
            <Segmented
              value={density}
              options={[
                { value: "compact",     label: "Compact",     density: 4 },
                { value: "default",     label: "Default",     density: 3 },
                { value: "comfortable", label: "Comfortable", density: 2 },
              ]}
              onChange={onSetDensity}
            />
          }
        />
      </SetSection>

      {/* ─── Security · Sessions ─── */}
      <SetSection title="Security · Sessions">
        <SetRow
          label="Active sessions"
          desc={<>Devices currently signed in to your account. Revoke anything you don't recognise.</>}
          block
        >
          <div className="ts-sess-list">
            {SAMPLE_SESSIONS.map((s) => (
              <div key={s.id} className={`ts-sess ${s.current ? "is-current" : ""}`}>
                <span className="ts-sess-icon"><SessIcon kind={s.kind} /></span>
                <div className="ts-sess-text">
                  <div className="ts-sess-device">
                    <span>{s.device}</span>
                    {s.current && <span className="ts-sess-tag">this session</span>}
                  </div>
                  <div className="ts-sess-meta">
                    <span>{s.ip}</span>
                    <span className="sep" aria-hidden="true"></span>
                    <span>{s.location}</span>
                  </div>
                </div>
                <span className="ts-sess-when">{s.when}</span>
                <button
                  type="button"
                  className="ts-sess-revoke"
                  aria-label={s.current ? "Cannot revoke current session" : "Revoke session"}
                  disabled={s.current}
                  title={s.current ? "You can't revoke the session you're signed in with" : "Revoke"}
                >
                  <IconX />
                </button>
              </div>
            ))}
          </div>
          <div className="ts-actions" style={{ marginTop: 14 }}>
            <Btn kind="danger">Log out all other sessions</Btn>
            <Btn kind="quiet">View full audit log →</Btn>
          </div>
        </SetRow>
      </SetSection>

      {/* ─── Security · Two-factor ─── */}
      <SetSection title="Security · Two-factor">
        <SetRow
          label="Authenticator app (TOTP)"
          desc={<>Pair Tap with an authenticator app. Required on new sign-ins once enabled.</>}
          block
        >
          <div className={`ts-status-row ${totp === "on" ? "is-on" : ""}`}>
            <span className="dot" aria-hidden="true"></span>
            {totp === "on" ? (
              <>
                <span style={{ color: "var(--ink)", fontWeight: 500 }}>Enabled</span>
                <span className="sep" aria-hidden="true"></span>
                <span>SHA-1 · 30s · 6 digits</span>
                <span className="sep" aria-hidden="true"></span>
                <span>enrolled Mar 12, 2026</span>
                <span className="grow"></span>
                <span style={{ color: "var(--accent)" }}>9 / 10 recovery codes remaining</span>
              </>
            ) : (
              <>
                <span style={{ color: "var(--ink)", fontWeight: 500 }}>Not enrolled</span>
                <span className="sep" aria-hidden="true"></span>
                <span>your account relies on password only</span>
              </>
            )}
          </div>
          <div className="ts-actions" style={{ marginTop: 14 }}>
            {totp === "on" ? (
              <>
                <Btn kind="accent">Regenerate recovery codes</Btn>
                <Btn>View recovery codes</Btn>
                <Btn kind="danger">Disable TOTP</Btn>
              </>
            ) : (
              <Btn kind="primary">Set up authenticator</Btn>
            )}
          </div>
        </SetRow>
      </SetSection>

      {/* ─── Security · Passkeys ─── */}
      <SetSection title="Security · Passkeys">
        <SetRow
          label="Registered passkeys"
          desc={<>Sign in without a password using a device-bound credential. You can have up to 10.</>}
          block
        >
          {totp === "on" ? (
            <div className="ts-pk-list">
              {SAMPLE_PASSKEYS.map((p) => (
                <div key={p.id} className="ts-pk">
                  <span className="ts-pk-icon"><IconKey /></span>
                  <div className="ts-pk-text">
                    <div className="ts-pk-label">{p.label}</div>
                    <div className="ts-pk-meta">added {p.added} · used {p.lastUsed}</div>
                  </div>
                  <span className="ts-sess-when">
                    {p.lastUsed === "today" ? "active" : "—"}
                  </span>
                  <button type="button" className="ts-sess-revoke" aria-label="Remove passkey">
                    <IconX />
                  </button>
                </div>
              ))}
            </div>
          ) : (
            <div className="ts-status-row">
              <span className="dot" aria-hidden="true"></span>
              <span style={{ color: "var(--ink)", fontWeight: 500 }}>No passkeys yet</span>
              <span className="sep" aria-hidden="true"></span>
              <span>add one to skip the password on this device</span>
            </div>
          )}
          <div className="ts-actions" style={{ marginTop: 14 }}>
            <Btn kind="primary">Add a passkey</Btn>
            <Btn kind="quiet">What is a passkey? →</Btn>
          </div>
        </SetRow>
      </SetSection>

      {/* Admin-only functions (user management, system status) live on the Admin tab. */}
      {admin && (
        <SetSection title="Instance">
          <SetRow
            label="Admin controls"
            desc={<>You can manage other accounts and view system status from the Admin tab.</>}
            control={<Btn kind="accent">Open Admin →</Btn>}
          />
        </SetSection>
      )}

      {/* Overlays */}
      {overlay === "enroll" && <EnrollDialog />}
      {overlay === "codes" && <RecoveryCodesDialog />}
      {overlay === "codes-regen" && <RecoveryCodesDialog regenerated />}
      {overlay === "disable" && (
        <CodeChallengeDialog
          title="Disable two-factor"
          body="You're about to remove TOTP from this account. Enter the current 6-digit code from your authenticator to confirm."
          danger
          ctaLabel="Disable TOTP"
        />
      )}
      {overlay === "regen" && (
        <CodeChallengeDialog
          title="Regenerate recovery codes"
          body="Generating new codes invalidates the old set immediately. Enter your current authenticator code to continue."
          ctaLabel="Generate new codes"
        />
      )}
      {overlay === "addpk" && <AddPasskeyDialog />}
      {overlay === "removepk" && (
        <PasswordChallengeDialog
          title="Remove passkey?"
          body={<>Removing <b>iPhone 15 · Face ID</b> means it can no longer sign in to Tap. Confirm with your account password — you can add the passkey back any time.</>}
          danger
          ctaLabel="Remove passkey"
        />
      )}
    </div>
  );
}

Object.assign(window, {
  TapSettings,
});
