// Sample feed data — lobste.rs / Dan Luu / Julia Evans tech blog vibe.
// Realistic title cadence, plausible reading times, plausible domains.

// `icon` is a small inline SVG markup string the feed provided in its
// <link rel="icon"> / atom <icon>. When present, it's rendered as the
// feed's avatar; otherwise we fall back to a flat `color` square.
// Icons are designed at a 16×16 viewBox.
const ICON_JVNS =
  '<svg viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg">' +
  '<rect width="16" height="16" rx="3" fill="#fff3b0"/>' +
  '<text x="8" y="11.6" text-anchor="middle" font-family="Georgia, serif" font-weight="700" font-size="10" fill="#d63384">je</text>' +
  '</svg>';
const ICON_LOBSTERS =
  '<svg viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg">' +
  '<rect width="16" height="16" rx="3" fill="#a6794d"/>' +
  '<path d="M5 5h6M5 8h6M5 11h4" stroke="#fff" stroke-width="1.4" stroke-linecap="round"/>' +
  '</svg>';
const ICON_PHORONIX =
  '<svg viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg">' +
  '<rect width="16" height="16" rx="3" fill="#0d0d0e"/>' +
  '<text x="8" y="12" text-anchor="middle" font-family="Helvetica, Arial, sans-serif" font-weight="700" font-size="11" fill="#e67700">P</text>' +
  '</svg>';
const ICON_LWN =
  '<svg viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg">' +
  '<rect width="16" height="16" rx="2" fill="#fff" stroke="#495057" stroke-width="1"/>' +
  '<text x="8" y="11.5" text-anchor="middle" font-family="Times, serif" font-weight="700" font-size="7" fill="#495057">LWN</text>' +
  '</svg>';
const ICON_SIMONW =
  '<svg viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg">' +
  '<circle cx="8" cy="8" r="7.5" fill="#0b7285"/>' +
  '<text x="8" y="11.6" text-anchor="middle" font-family="Helvetica, Arial, sans-serif" font-weight="700" font-size="10" fill="#fff">s</text>' +
  '</svg>';
const ICON_DREW =
  '<svg viewBox="0 0 16 16" xmlns="http://www.w3.org/2000/svg">' +
  '<rect width="16" height="16" fill="#7a4ddb"/>' +
  '<path d="M3 8 L8 3 L13 8 L8 13 Z" fill="#fff"/>' +
  '</svg>';

const FEEDS = [
  { id: 1, name: "Julia Evans",       url: "jvns.ca",            color: "#d63384", icon: ICON_JVNS },
  { id: 2, name: "Dan Luu",           url: "danluu.com",         color: "#1a1a1a" },
  { id: 3, name: "Drew DeVault",      url: "drewdevault.com",    color: "#7a4ddb", icon: ICON_DREW },
  { id: 4, name: "Fabien Sanglard",   url: "fabiensanglard.net", color: "#c9520f" },
  { id: 5, name: "lobste.rs",         url: "lobste.rs",          color: "#a6794d", icon: ICON_LOBSTERS },
  { id: 6, name: "Hillel Wayne",      url: "buttondown.email/hillelwayne", color: "#2b8a3e" },
  { id: 7, name: "Computer Things",   url: "buttondown.email/hillelwayne", color: "#1971c2", error: "Last fetch: 502 Bad Gateway" },
  { id: 8, name: "Eli Bendersky",     url: "eli.thegreenplace.net", color: "#5c940d" },
  { id: 9, name: "Phoronix",          url: "phoronix.com",       color: "#e67700", icon: ICON_PHORONIX },
  { id: 10, name: "LWN.net",          url: "lwn.net",            color: "#495057", icon: ICON_LWN },
  { id: 11, name: "Simon Willison",   url: "simonwillison.net",  color: "#0b7285", icon: ICON_SIMONW },
  { id: 12, name: "rachelbythebay",   url: "rachelbythebay.com", color: "#862e9c", error: "TLS certificate expired 3 days ago" },
];

const ENTRIES = [
  {
    id: 101, feed: 1,
    title: "Reasons why bugs might feel \u201Cimpossible\u201D",
    summary: "A bug feels impossible when one of your assumptions is wrong. Here are some categories of wrong assumptions I keep running into, with concrete examples from my own debugging notes.",
    rt: 12, ago: "32m", read: false, saved: false,
  },
  {
    id: 102, feed: 5,
    title: "Show: I wrote a tiny SQLite-backed task queue in 200 lines of Go",
    summary: "Single binary, single file, FIFO with at-least-once delivery. The whole thing is one INSERT, one SELECT \u2026 RETURNING, and a poll loop. No Redis. No Postgres. Surprisingly fine.",
    rt: 6, ago: "1h", read: false, saved: true,
  },
  {
    id: 103, feed: 2,
    title: "Files are fraught with peril",
    summary: "fsync is a lie, rename is a lie, the page cache is a liar, your SSD is a liar, and your filesystem is a liar. Here is a long, deeply-cited tour of every lie, in roughly the order you will encounter them.",
    rt: 41, ago: "3h", read: false, saved: false,
  },
  {
    id: 104, feed: 11,
    title: "Notes from running a small LLM locally for a month",
    summary: "I put a 7B model on the laptop and tried to use it for everything for thirty days. Some things that surprised me, some things that didn't, and a list of small UX wins that the cloud APIs have spoiled me on.",
    rt: 18, ago: "5h", read: true, saved: false,
  },
  {
    id: 105, feed: 12,
    title: "The on-call rotation that ate itself",
    summary: "Every alert had a runbook. Every runbook said \u201Cescalate to the team that owns the service.\u201D Every team's escalation went to the same five people. You can guess what happened next.",
    rt: 8, ago: "6h", read: false, saved: false,
  },
  {
    id: 106, feed: 3,
    title: "Status update, April 2026",
    summary: "Hare 0.25 is out. The mailing list is getting a workflow refresh. I'm spending more time on the build server. As always, thank you to everyone who funded this work.",
    rt: 5, ago: "8h", read: true, saved: false,
  },
  {
    id: 107, feed: 4,
    title: "Reading Quake's renderer, twenty-eight years later",
    summary: "I went back to the original 1996 Quake source release and read the software rasterizer end-to-end. The surface caching trick still feels like cheating. Annotated walkthrough with diagrams.",
    rt: 27, ago: "11h", read: false, saved: true,
  },
  {
    id: 108, feed: 6,
    title: "TLA+ for people who already know property-based testing",
    summary: "If you write Hypothesis or QuickCheck, you already think in invariants and shrinking. TLA+ is just \u2026 the same instinct, with a model checker instead of a fuzzer. Here's a 20-minute on-ramp.",
    rt: 14, ago: "14h", read: false, saved: false,
  },
  {
    id: 109, feed: 9,
    title: "Linux 6.14 lands with major io_uring scheduler rework",
    summary: "Jens Axboe's changes to the io_uring task scheduler land in mainline. Early benchmarks on NVMe-heavy workloads show 6\u201311% improvements in tail latency.",
    rt: 4, ago: "16h", read: true, saved: false,
  },
  {
    id: 110, feed: 8,
    title: "Go's range-over-func, in practice",
    summary: "I rewrote three small libraries to use range-over-func iterators after upgrading to 1.23. Two were genuinely cleaner. One got worse. Here's what the difference was.",
    rt: 9, ago: "1d", read: false, saved: false,
  },
  {
    id: 111, feed: 10,
    title: "Btrfs send/receive after the great rewrite",
    summary: "The 6.13 cycle quietly contained the largest rewrite of btrfs send/receive since the feature was introduced. The numbers are good and the corner cases are smaller, but a few quirks remain.",
    rt: 22, ago: "1d", read: false, saved: false,
  },
  {
    id: 112, feed: 7,
    title: "Why \u201Cjust use Postgres\u201D is correct exactly 80% of the time",
    summary: "It's the right answer often enough that it's a reasonable default. It's wrong often enough that defaults need an exception list. Here's mine, after about a decade of building things on top of databases.",
    rt: 11, ago: "1d", read: false, saved: false,
  },
  {
    id: 113, feed: 5,
    title: "Ask: what's your favorite tiny piece of UI you've ever built?",
    summary: "Not a whole product. A single component, a single interaction, a tooltip, a keyboard shortcut. The smaller the better. I'll start: a textarea that auto-saves a draft to localStorage every keystroke.",
    rt: 3, ago: "1d", read: true, saved: false,
  },
  {
    id: 114, feed: 1,
    title: "How does git actually store your files?",
    summary: "A short illustrated tour of objects, trees, blobs, and packfiles. With diagrams drawn in pen, scanned, and dropped into the post unaltered, because that's how I roll.",
    rt: 16, ago: "2d", read: false, saved: true,
  },
  {
    id: 115, feed: 11,
    title: "A small note on default arguments and Python's `dataclass`",
    summary: "TIL that `field(default_factory=list)` exists for a reason and `default=[]` will absolutely betray you. Demo and a one-line lint rule.",
    rt: 4, ago: "2d", read: false, saved: false,
  },
];

window.TAP_FEEDS = FEEDS;
window.TAP_ENTRIES = ENTRIES;
