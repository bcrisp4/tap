// Article reader view — full-text reader for one entry.

const READER_BODY = [
  { type: 'p', text: "A bug feels impossible when one of your assumptions is wrong. That's almost the definition. If your model of the system matched reality, the bug would be obvious — you'd look at the code and see it. So when a bug is hard, it isn't really the bug that's hard; it's the assumption you can't see past." },
  { type: 'p', text: "I keep a notebook of these. Not the bugs themselves, but the categories of wrong assumption that I keep running back into. Every time I lose a day to one, I add a tally mark. After a few years, the list is depressingly short." },
  { type: 'h2', text: "1. The thing you didn't think was running, is running" },
  { type: 'p', text: "A scheduled job. A retry queue. A daemon that's been there so long nobody remembers installing it. A second instance of your service in a stack you forgot about. A debugger attached. A cron in your dotfiles. The version of the binary on the path is not the version you just built." },
  { type: 'p', text: "The shape of this bug is: \u201Cit only happens sometimes.\u201D The fix is almost always to find the second thing. ps, lsof, systemctl, launchctl, your editor's process list. Look once for what you expect; look again for what you don't." },
  { type: 'h2', text: "2. The state lives somewhere you didn't look" },
  { type: 'p', text: "Most of us have a default set of places we check: the database, the in-memory caches we know about, environment variables, command-line flags. State has a way of accreting in places off this list. A file on disk left over from a previous run. A row in a config table nobody documented. A cookie. The browser's localStorage. A query parameter your tests don't exercise." },
  { type: 'pre', text: "$ find . -newer /tmp/marker -type f 2>/dev/null | head\n./var/lib/foo/state.db\n./tmp/.foo.lock\n./.config/foo/overrides.toml" },
  { type: 'p', text: "I have rescued a lot of debugging sessions with this exact incantation. Touch a marker file, do the thing, then list everything modified since. Whatever shows up that you didn't expect — that is your bug, in the form of a file path." },
  { type: 'h2', text: "3. Two things named the same thing" },
  { type: 'p', text: "Two functions, two columns, two log lines, two binaries. They look identical at the call site. They are not the same. Type systems help here, sometimes; namespaces help, sometimes; at scale, neither saves you. The fix is to rename one of them, painfully, and then watch which call sites break." },
  { type: 'p', text: "If renaming is expensive, even just adding a one-character suffix to one of them, locally, in your editor, for ten minutes, is enough to surface the bug. The compiler will tell you the answer." },
];

function ReaderHeader({ feed, entry, onBack, compact }) {
  return (
    <div className="reader-header">
      {onBack && (
        <button className="reader-back" onClick={onBack} title="Back to unread (Esc)">
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M10 3 5 8l5 5"/></svg>
          <span>Unread</span>
        </button>
      )}
      <div className="reader-actions">
        <button className="reader-action" title="Mark unread (m)">
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><circle cx="8" cy="8" r="3.5"/></svg>
          <span>Mark unread</span>
        </button>
        <button className="reader-action is-saved" title="Saved (s)">
          <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round"><path d="M4 2.5h8v11l-4-3-4 3z"/></svg>
        </button>
        <button className="reader-action" title="View original (v)">
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M9 3h4v4M13 3 7 9M11 9.5V13H3V5h3.5"/></svg>
          <span>View original</span>
        </button>
      </div>
    </div>
  );
}

function ReaderBody({ entry, feed }) {
  return (
    <article className="reader-body">
      <div className="reader-source">
        <span className="src-ico" style={{ background: feed.color }}></span>
        <span className="src-name">{feed.name}</span>
        <span className="src-sep" aria-hidden="true"></span>
        <span className="src-url mono">{feed.url}</span>
      </div>
      <h1 className="reader-title">{entry.title}</h1>
      <div className="reader-byline mono">
        <span>By Julia Evans</span>
        <span aria-hidden="true">·</span>
        <span>April 26, 2026</span>
        <span aria-hidden="true">·</span>
        <span>{entry.rt} min read</span>
      </div>
      <div className="reader-rule" aria-hidden="true">
        <span className="reader-rule-line"></span>
        <span className="reader-rule-dot"></span>
        <span className="reader-rule-line"></span>
      </div>
      <p className="reader-lede">{entry.summary}</p>
      {READER_BODY.map((b, i) => {
        if (b.type === 'h2') return <h2 key={i} className="reader-h2">{b.text}</h2>;
        if (b.type === 'pre') return <pre key={i} className="reader-pre"><code>{b.text}</code></pre>;
        return <p key={i} className="reader-p">{b.text}</p>;
      })}
      <div className="reader-end" aria-hidden="true">
        <span className="reader-end-line"></span>
        <span className="reader-end-dot"></span>
        <span className="reader-end-line"></span>
      </div>
      <div className="reader-foot mono">
        Cached locally · last refreshed 32m ago
      </div>
    </article>
  );
}

function DesktopReader({ entries, feeds, entryId = 101, theme = "light", fontMode = "serif", measure = "comfortable" }) {
  const entry = entries.find((e) => e.id === entryId);
  const feed = feeds.find((f) => f.id === entry.feed);
  const fontStyle = fontMode === 'sans' ? { fontFamily: 'var(--sans)' } : {};
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const [moreOpen, setMoreOpen] = useState(false);
  return (
    <div className={`tap theme-${theme} measure-${measure}`} style={{ display: 'flex', height: '100%', ...fontStyle }}>
      <Sidebar
        active="unread"
        feeds={feeds}
        unreadCounts={Object.fromEntries(feeds.map(f => [f.id, entries.filter(e => e.feed===f.id && !e.read).length]))}
        onOpenShortcuts={() => setShortcutsOpen(true)}
        onOpenMore={() => setMoreOpen(true)}
      />
      <div className="reader-pane">
        <ReaderHeader feed={feed} entry={entry} onBack={() => {}} />
        <ReaderBody entry={entry} feed={feed} />
      </div>
      <ShortcutsModal open={shortcutsOpen} onClose={() => setShortcutsOpen(false)} />
      <MoreMenu open={moreOpen} onClose={() => setMoreOpen(false)} />
    </div>
  );
}

function MobileReader({ entries, feeds, entryId = 103, theme = "light", fontMode = "serif" }) {
  const entry = entries.find((e) => e.id === entryId);
  const feed = feeds.find((f) => f.id === entry.feed);
  const fontStyle = fontMode === 'sans' ? { fontFamily: 'var(--sans)' } : {};
  return (
    <div className={`tap theme-${theme} is-mobile measure-mobile`} style={{ display: 'flex', flexDirection: 'column', height: '100%', ...fontStyle }}>
      <div className="m-reader-topbar">
        <button className="m-back">
          <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M10 3 5 8l5 5"/></svg>
        </button>
        <div className="m-progress" aria-hidden="true">
          <span className="m-progress-bar"></span>
        </div>
        <button className="m-action">
          <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round"><path d="M4 2.5h8v11l-4-3-4 3z"/></svg>
        </button>
      </div>
      <div style={{ flex: 1, overflowY: 'auto' }}>
        <ReaderBody entry={entry} feed={feed} />
      </div>
      <div className="m-reader-footbar">
        <button className="m-foot-btn">
          <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><circle cx="8" cy="8" r="3.5"/></svg>
          <span>Mark unread</span>
        </button>
        <button className="m-foot-btn is-saved">
          <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round"><path d="M4 2.5h8v11l-4-3-4 3z" fill="currentColor"/></svg>
          <span>Saved</span>
        </button>
        <button className="m-foot-btn">
          <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M9 3h4v4M13 3 7 9M11 9.5V13H3V5h3.5"/></svg>
          <span>Original</span>
        </button>
      </div>
    </div>
  );
}

Object.assign(window, { DesktopReader, MobileReader, READER_BODY });
