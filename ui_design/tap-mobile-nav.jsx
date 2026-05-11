// Tap — shared mobile chrome.
// One top app bar + one bottom tab bar, used identically across all
// top-level mobile views (Unread/Saved/History/Categories/Feeds/Settings).
//
// Touch targets are ≥44pt; nothing in here is < 11px text. The wordmark
// is the only adornment up top; the title is the page name, which doubles
// as orientation when the user is several taps deep.

const { useState: useStateMNav } = React;

// ─── Icons ──────────────────────────────────────────────────────────────
// Built from straight lines + circles. The schematic dot is the bridge to
// the brand: every active icon picks up the accent dot.
function MNavIcon({ kind, active }) {
  const sw = 1.6;
  switch (kind) {
    case "unread":
      return (
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={sw} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M4 7h16" />
          <path d="M4 12h16" />
          <path d="M4 17h10" />
          {active && <circle cx="19" cy="17" r="2.2" fill="var(--accent)" stroke="none" />}
        </svg>
      );
    case "saved":
      return (
        <svg width="22" height="22" viewBox="0 0 24 24" fill={active ? "var(--accent)" : "none"} stroke="currentColor" strokeWidth={sw} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M6 4h12v17l-6-4.5L6 21z" />
        </svg>
      );
    case "feeds":
      return (
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={sw} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M5 13a8 8 0 0 1 8 8" />
          <path d="M5 8a13 13 0 0 1 13 13" />
          <circle cx="6" cy="20" r="1.6" fill={active ? "var(--accent)" : "currentColor"} stroke="none" />
        </svg>
      );
    case "categories":
      return (
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={sw} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <rect x="3.5" y="5" width="17" height="5.5" rx="1" />
          <rect x="3.5" y="13" width="17" height="5.5" rx="1" />
          {active && <circle cx="7" cy="7.75" r="1.2" fill="var(--accent)" stroke="none" />}
        </svg>
      );
    case "more":
      return (
        <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor" stroke="none" aria-hidden="true">
          <circle cx="5.5" cy="12" r="1.6" />
          <circle cx="12" cy="12" r="1.6" />
          <circle cx="18.5" cy="12" r="1.6" />
          {active && <circle cx="12" cy="12" r="1.6" fill="var(--accent)" />}
        </svg>
      );
    case "history":
      return (
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={sw} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M3 12a9 9 0 1 0 3-6.7" />
          <path d="M3 4v4h4" />
          <path d="M12 8v4l3 2" />
        </svg>
      );
    case "settings":
      return (
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={sw} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <circle cx="12" cy="12" r="2.6" />
          <path d="M12 3v2.5M12 18.5V21M3 12h2.5M18.5 12H21M5.6 5.6l1.8 1.8M16.6 16.6l1.8 1.8M5.6 18.4l1.8-1.8M16.6 7.4l1.8-1.8" />
        </svg>
      );
    default:
      return null;
  }
}

// ─── Top app bar ────────────────────────────────────────────────────────
function TapMobileHeader({ title, eyebrow, count, countLabel, right }) {
  return (
    <header className="tmh">
      <a className="ts-wordmark tmh-mark" href="#" aria-label="Tap home">
        tap<span className="ts-wordmark-dot" aria-hidden="true"></span>
      </a>
      <div className="tmh-titles">
        {eyebrow && <div className="tmh-eyebrow">{eyebrow}</div>}
        <h1 className="tmh-title">{title}</h1>
      </div>
      <div className="tmh-right">
        {count != null && (
          <span className="tmh-count">
            <b>{count}</b>
            {countLabel && <span>{countLabel}</span>}
          </span>
        )}
        {right}
      </div>
    </header>
  );
}

// ─── Bottom tab bar ─────────────────────────────────────────────────────
// 5 tabs: Unread / Saved / Feeds / Categories / More
// 'history' and 'settings' fall back to the More sheet — when they are
// active, the More tab lights up to keep the user oriented.

const MNAV_PRIMARY = [
  { id: "unread",     label: "Unread",     icon: "unread" },
  { id: "saved",      label: "Saved",      icon: "saved" },
  { id: "feeds",      label: "Feeds",      icon: "feeds" },
  { id: "categories", label: "Categories", icon: "categories" },
  { id: "more",       label: "More",       icon: "more" },
];

const MNAV_MORE = [
  { id: "history",  label: "History",  icon: "history",  desc: "Everything you've read." },
  { id: "settings", label: "Settings", icon: "settings", desc: "Theme, account, security." },
];

// Placeholder identity for the More sheet. Hydrate from session in a
// real integration. Single source of truth so desktop + mobile match.
const MNAV_DEFAULT_USER = {
  name: "Jodie Park",
  email: "jodie@thecrisp.io",
  initial: "J",
};

function TapMobileNav({ active = "unread", unreadCount = 0, onNav, onLogout, user = MNAV_DEFAULT_USER, forceMoreOpen = false }) {
  const [moreOpen, setMoreOpen] = useStateMNav(forceMoreOpen);
  const reduced =
    active === "history" || active === "settings" ? "more" : active;

  const handle = (id) => {
    if (id === "more") {
      setMoreOpen(true);
      return;
    }
    setMoreOpen(false);
    onNav?.(id);
  };

  const handleMoreItem = (id) => {
    setMoreOpen(false);
    onNav?.(id);
  };

  const handleLogout = () => {
    setMoreOpen(false);
    onLogout?.();
  };

  return (
    <React.Fragment>
      <nav className="tmnav" aria-label="Primary">
        {MNAV_PRIMARY.map((t) => {
          const isActive = reduced === t.id;
          const showBadge = t.id === "unread" && unreadCount > 0;
          return (
            <button
              key={t.id}
              type="button"
              className={`tmnav-tab ${isActive ? "is-active" : ""}`}
              aria-current={isActive ? "page" : undefined}
              onClick={() => handle(t.id)}
            >
              <span className="tmnav-ico">
                <MNavIcon kind={t.icon} active={isActive} />
                {showBadge && (
                  <span className="tmnav-badge" aria-hidden="true">
                    {unreadCount > 99 ? "99+" : unreadCount}
                  </span>
                )}
              </span>
              <span className="tmnav-lbl">{t.label}</span>
              <span className="tmnav-rule" aria-hidden="true" />
            </button>
          );
        })}
      </nav>

      {moreOpen && (
        <React.Fragment>
          <div className="tmnav-sheet-backdrop" onClick={() => setMoreOpen(false)} />
          <div className="tmnav-sheet" role="dialog" aria-label="More">
            <div className="ts-mobile-sheet-handle" aria-hidden="true" />
            <div className="tmnav-sheet-identity">
              <span className="avatar" aria-hidden="true">{user.initial || (user.name || user.email || "?")[0].toUpperCase()}</span>
              <span className="who">
                <span className="name">{user.name || "Account"}</span>
                <span className="email">{user.email}</span>
              </span>
            </div>
            <div className="tmnav-sheet-list">
              {MNAV_MORE.map((m) => (
                <button
                  key={m.id}
                  type="button"
                  className={`tmnav-sheet-item ${active === m.id ? "is-current" : ""}`}
                  onClick={() => handleMoreItem(m.id)}
                >
                  <span className="tmnav-sheet-ico">
                    <MNavIcon kind={m.icon} active={active === m.id} />
                  </span>
                  <span className="tmnav-sheet-body">
                    <span className="tmnav-sheet-name">{m.label}</span>
                    <span className="tmnav-sheet-desc">{m.desc}</span>
                  </span>
                  <span className="tmnav-sheet-chev" aria-hidden="true">›</span>
                </button>
              ))}
              <div className="tmnav-sheet-sep" aria-hidden="true" />
              <button
                type="button"
                className="tmnav-sheet-item is-logout"
                onClick={handleLogout}
              >
                <span className="tmnav-sheet-ico" aria-hidden="true">
                  <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M9.5 3.5H4.5a1 1 0 0 0-1 1v7a1 1 0 0 0 1 1h5"/><path d="m11 5.5 2.5 2.5L11 10.5"/><path d="M7 8h6.5"/></svg>
                </span>
                <span className="tmnav-sheet-body">
                  <span className="tmnav-sheet-name">Log out</span>
                  <span className="tmnav-sheet-desc">Sign out of {user.email}</span>
                </span>
              </button>
            </div>
            <div className="tmnav-sheet-foot">
              <button type="button" className="tmnav-sheet-cancel" onClick={() => setMoreOpen(false)}>
                Close
              </button>
          </div>
          </div>
        </React.Fragment>
      )}
    </React.Fragment>
  );
}

// ─── Convenience: full shell wrapper ────────────────────────────────────
// Standard layout: header → body (scrolls) → bottom nav.
function TapMobileShell({ title, eyebrow, count, countLabel, headerRight, active, unreadCount, onNav, forceMoreOpen, children, bodyClass }) {
  return (
    <div className="tmshell">
      <TapMobileHeader
        title={title}
        eyebrow={eyebrow}
        count={count}
        countLabel={countLabel}
        right={headerRight}
      />
      <div className={`tmshell-body ${bodyClass || ""}`}>{children}</div>
      <TapMobileNav
        active={active}
        unreadCount={unreadCount}
        onNav={onNav}
        forceMoreOpen={forceMoreOpen}
      />
    </div>
  );
}

Object.assign(window, {
  TapMobileHeader,
  TapMobileNav,
  TapMobileShell,
  MNavIcon,
});
