// Tap — sign-in surfaces.
// One component covers every variation: layout (centered | split), mode
// (password | magic | passkey | otp), theme, and an optional error.

const { useState: useLoginState } = React;

// ───── Marks ─────
function TapWordmark({ size = 22 }) {
  const dot = Math.max(4, Math.round(size * 0.26));
  return (
    <span
      className="tl-wordmark"
      style={{
        fontFamily: 'var(--sans)',
        fontWeight: 600,
        fontSize: size,
        letterSpacing: '-0.02em',
        color: 'var(--ink)',
        display: 'inline-flex',
        alignItems: 'baseline',
        gap: 1,
      }}
    >
      tap
      <span
        aria-hidden="true"
        style={{
          display: 'inline-block',
          width: dot,
          height: dot,
          borderRadius: '50%',
          background: 'var(--accent)',
          transform: 'translateY(-1px)',
          marginLeft: 2,
        }}
      ></span>
    </span>
  );
}

function TapJunctionMark({ size = 64, color = 'currentColor', accent = 'var(--accent)' }) {
  // Horizontal bar across, vertical descender, Klein-Blue dot at the junction.
  const w = size;
  const h = size;
  return (
    <svg width={w} height={h} viewBox="0 0 64 64" aria-hidden="true">
      <line x1="6" y1="20" x2="58" y2="20" stroke={color} strokeWidth="2" strokeLinecap="round" />
      <line x1="32" y1="20" x2="32" y2="58" stroke={color} strokeWidth="2" strokeLinecap="round" />
      <circle cx="32" cy="20" r="5" fill={accent} />
    </svg>
  );
}

// ───── Inline icons ─────
function IconKey() {
  return (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <circle cx="5.5" cy="10.5" r="2.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="M7.5 8.5 14 2M11.5 4.5 13 6M9.5 6.5 11 8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
function IconArrow() {
  return (
    <svg width="13" height="13" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path d="M3 8h10M9 4l4 4-4 4" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
function IconWarn() {
  return (
    <svg width="13" height="13" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path d="M8 2 1.5 13.5h13L8 2Z" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" />
      <path d="M8 6.5v3.5" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
      <circle cx="8" cy="11.7" r="0.7" fill="currentColor" />
    </svg>
  );
}
function IconEye({ open = true }) {
  return open ? (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path d="M1.5 8s2.4-4.5 6.5-4.5S14.5 8 14.5 8 12.1 12.5 8 12.5 1.5 8 1.5 8Z" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" />
      <circle cx="8" cy="8" r="2" stroke="currentColor" strokeWidth="1.4" />
    </svg>
  ) : (
    <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path d="M2 13 13.5 2.5" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
      <path d="M3 9.5C2 8.6 1.5 8 1.5 8S3.9 3.5 8 3.5c.9 0 1.7.2 2.4.5M6 12.2c.6.2 1.3.3 2 .3 4.1 0 6.5-4.5 6.5-4.5s-.6-1-1.8-2.2" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

// ───── Static OTP cells (display only) ─────
function OTPCells({ value = '', focusIndex = null }) {
  const cells = Array.from({ length: 6 }, (_, i) => value[i] ?? '');
  return (
    <div className="ts-otp" role="group" aria-label="6-digit code">
      {cells.map((d, i) => {
        const cls = [
          'ts-otp-cell',
          d ? '' : 'is-empty',
          focusIndex === i ? 'is-focus' : '',
        ].filter(Boolean).join(' ');
        return <span key={i} className={cls}>{d || ''}</span>;
      })}
    </div>
  );
}

// ───── A field with mono label and inline affordance slot ─────
function LoginField({ label, value, type = 'text', placeholder = '', monospaced = false, trailing = null, autoFocus = false, accent = false }) {
  return (
    <div className="tl-field">
      <label className="ts-field-label" style={{ display: 'flex', justifyContent: 'space-between' }}>
        <span>{label}</span>
        {trailing && <span className="tl-field-trailing">{trailing}</span>}
      </label>
      <div className={`tl-field-wrap ${accent ? 'is-accent' : ''}`}>
        <input
          type={type}
          className={`ts-field ${monospaced ? 'is-mono' : ''}`}
          defaultValue={value}
          placeholder={placeholder}
          autoFocus={autoFocus}
          style={monospaced ? { fontFamily: 'var(--mono)', fontSize: 13 } : undefined}
        />
      </div>
    </div>
  );
}

// ───── Error chip ─────
function LoginError({ children }) {
  return (
    <div className="tl-error">
      <span className="tl-error-ico" aria-hidden="true"><IconWarn /></span>
      <span className="tl-error-body">{children}</span>
    </div>
  );
}

// ───── The form itself, mode-driven ─────
function LoginForm({ mode = 'password', error = null, email = '', mobile = false }) {
  const isOTP = mode === 'otp';
  const isPasskey = mode === 'passkey';
  const isMagic = mode === 'magic';
  const isPassword = mode === 'password';

  const title =
    isOTP ? 'Verification code'
      : isPasskey ? 'Welcome back'
      : isMagic ? 'Sign in by email'
      : 'Sign in';
  const sub =
    isOTP ? <>Open your authenticator and enter the six digits for <span className="mono" style={{ color: 'var(--ink-2)' }}>{email || 'ada@example.com'}</span>.</>
      : isPasskey ? <>Your device has a passkey for <span className="mono" style={{ color: 'var(--ink-2)' }}>{email || 'ada@example.com'}</span>. No password needed.</>
      : null;

  return (
    <div className={`tl-form ${mobile ? 'is-mobile' : ''}`}>
      <h1 className="tl-title">{title}</h1>
      {sub && <p className="tl-sub">{sub}</p>}

      {error && <LoginError>{error}</LoginError>}

      {(isPassword || isMagic) && (
        <LoginField
          label="EMAIL"
          value={email}
          type="email"
          placeholder="you@domain.com"
          monospaced
          autoFocus={!email}
        />
      )}

      {isPassword && (
        <LoginField
          label="PASSWORD"
          value=""
          type="password"
          placeholder="\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022"
          trailing={
            <a href="#" className="tl-field-trailing-link mono">
              FORGOT?
            </a>
          }
        />
      )}

      {isOTP && (
        <div className="tl-otp-row">
          <OTPCells value="247" focusIndex={3} />
          {!mobile && (
            <span className="tl-otp-hint mono">PASTE WITH <span className="ts-kbd">⌘</span><span className="ts-kbd">V</span></span>
          )}
        </div>
      )}

      {isPasskey && (
        <div className="tl-passkey-card">
          <span className="tl-passkey-ico" aria-hidden="true"><IconKey /></span>
          <div>
            <div className="tl-passkey-title">MacBook · Touch ID</div>
            <div className="tl-passkey-sub mono">last used 2 days ago</div>
          </div>
        </div>
      )}

      <div className="tl-actions">
        <button type="button" className="ts-btn is-primary tl-primary">
          {isPasskey ? <><IconKey />Continue with passkey</>
            : isMagic ? 'Send sign-in link'
            : isOTP ? <>Verify and continue<IconArrow /></>
            : <>Continue<IconArrow /></>}
          {!mobile && !isPasskey && !isOTP && <span className="ts-kbd">Enter</span>}
        </button>

        {/* Secondary alt path */}
        {isPassword && (
          <button type="button" className="ts-btn is-quiet tl-alt">
            <IconKey />Use a passkey
          </button>
        )}
        {isPasskey && (
          <button type="button" className="ts-btn is-quiet tl-alt">
            Use a password instead
          </button>
        )}
        {isMagic && (
          <button type="button" className="ts-btn is-quiet tl-alt">
            Sign in with a password
          </button>
        )}
        {isOTP && (
          <button type="button" className="ts-btn is-quiet tl-alt">
            Use a recovery code
          </button>
        )}
      </div>

      {isOTP && (
        <div className="tl-switch mono">
          <span>NOT YOU?</span>
          <a href="#" className="tl-switch-link">Use a different account</a>
        </div>
      )}
    </div>
  );
}

// ───── Layout shells ─────

function CenteredShell({ children, theme = 'light' }) {
  return (
    <div className={`tap theme-${theme} tl-root`}>
      <header className="tl-header">
        <TapWordmark size={20} />
      </header>
      <main className="tl-main">
        <div className="tl-col">{children}</div>
      </main>
    </div>
  );
}

function SplitShell({ children, theme = 'light', quote }) {
  return (
    <div className={`tap theme-${theme} tl-root tl-split`}>
      <aside className="tl-aside">
        <div className="tl-aside-top">
          <TapWordmark size={22} />
          <span className="tl-aside-meta mono">A QUIET FEED READER</span>
        </div>
        <div className="tl-aside-mid">
          <span className="tl-aside-mark"><TapJunctionMark size={80} color="currentColor" accent="var(--accent)" /></span>
          <h2 className="tl-aside-h">A quiet reading inbox for the open web.</h2>
          <p className="tl-aside-p">
            Subscribe to blogs, journals, and people who still publish in feeds.
            Keyboard-first. Three themes. No algorithm.
          </p>
        </div>
        <div className="tl-aside-foot">
          <div className="tl-aside-quote">
            <span className="mono tl-aside-q-eyebrow">{quote?.eyebrow || 'WHY TAP'}</span>
            <p className="tl-aside-q-body">{quote?.body || '“It is the only thing on my desktop without a notification badge.”'}</p>
            <span className="mono tl-aside-q-cite">{quote?.cite || '— RACHEL, BETA · 2026'}</span>
          </div>
        </div>
      </aside>
      <main className="tl-main tl-split-main">
        <div className="tl-col">{children}</div>
        <footer className="tl-footer is-split mono">
          <span><span className="tl-foot-dot" aria-hidden="true"></span>Encrypted · self-hosted</span>
          <span className="tl-foot-sep">·</span>
          <a href="#">Status</a>
          <span className="tl-foot-sep">·</span>
          <a href="#">Privacy</a>
        </footer>
      </main>
    </div>
  );
}

// ───── Public top-level page components ─────

function TapLoginDesktop({ theme = 'light', mode = 'password', error = null, email = '' }) {
  return <CenteredShell theme={theme}><LoginForm mode={mode} error={error} email={email} /></CenteredShell>;
}

function TapLoginMobile({ theme = 'light', mode = 'password', error = null, email = '' }) {
  return (
    <div className={`tap theme-${theme} tl-root is-mobile`}>
      <header className="tl-m-header">
        <TapWordmark size={18} />
      </header>
      <main className="tl-m-main">
        <LoginForm mode={mode} error={error} email={email} mobile />
      </main>
    </div>
  );
}

Object.assign(window, { TapLoginDesktop, TapLoginMobile, TapWordmark, TapJunctionMark });
