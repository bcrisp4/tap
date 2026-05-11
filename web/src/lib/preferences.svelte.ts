// .svelte.ts enables Svelte runes ($state, $derived) outside components.
type Theme = 'light' | 'dark' | 'sepia' | 'system';
type Font = 'serif' | 'sans';
type Density = 'compact' | 'comfortable' | 'cosy';
type Measure = 'narrow' | 'comfortable' | 'wide';

const THEMES: Theme[] = ['light', 'dark', 'sepia', 'system'];
const FONTS: Font[] = ['serif', 'sans'];
const DENSITIES: Density[] = ['compact', 'comfortable', 'cosy'];
const MEASURES: Measure[] = ['narrow', 'comfortable', 'wide'];

const mq = typeof window !== 'undefined'
  ? window.matchMedia('(prefers-color-scheme: dark)')
  : null;

let prefersDark = $state(mq?.matches ?? false);
if (mq) {
  mq.addEventListener('change', (e) => { prefersDark = e.matches; });
}

function makeTheme() {
  const raw = localStorage.getItem('tap.theme');
  let stored = $state<Theme>(THEMES.includes(raw as Theme) ? (raw as Theme) : 'system');
  const resolved = $derived<'light' | 'dark' | 'sepia'>(
    stored === 'system' ? (prefersDark ? 'dark' : 'light') : stored,
  );
  return {
    get stored() { return stored; },
    set stored(v: Theme) { stored = v; localStorage.setItem('tap.theme', v); },
    get resolved() { return resolved; },
  };
}

function makePref<T extends string>(key: string, def: T, allowed: T[]) {
  const raw = localStorage.getItem(key);
  let value = $state<T>(allowed.includes(raw as T) ? (raw as T) : def);
  return {
    get value() { return value; },
    set value(v: T) { value = v; localStorage.setItem(key, v); },
  };
}

// One-off migration: legacy density value "default" → "comfortable".
// Run before makePref reads, so the legacy value is replaced *before* the
// allowed-list filter would drop it back to the default.
(function migrateDensity() {
  if (typeof localStorage === 'undefined') return;
  const raw = localStorage.getItem('tap.density');
  if (raw === 'default') {
    localStorage.setItem('tap.density', 'comfortable');
  }
})();

function makeBoolPref(key: string, def: boolean) {
  const raw = typeof localStorage !== 'undefined' ? localStorage.getItem(key) : null;
  let value = $state<boolean>(raw == null ? def : raw === '1');
  return {
    get value() { return value; },
    set value(v: boolean) {
      value = v;
      try { localStorage.setItem(key, v ? '1' : '0'); } catch { /* ignore */ }
    },
  };
}

export const theme = makeTheme();
export const font = makePref<Font>('tap.font', 'serif', FONTS);
export const density = makePref<Density>('tap.density', 'comfortable', DENSITIES);
export const measure = makePref<Measure>('tap.measure', 'comfortable', MEASURES);
export const markOnScroll = makeBoolPref('tap.markOnScroll', true);

// Reading-section toggles — persistent boolean prefs using 'true'/'false' strings.
function makeReadingBoolPref(key: string, def: boolean) {
  const raw = typeof localStorage !== 'undefined' ? localStorage.getItem(key) : null;
  const parsed = raw === 'true' ? true : raw === 'false' ? false : def;
  let value = $state<boolean>(parsed);
  return {
    get value() { return value; },
    set value(v: boolean) {
      value = v;
      try { localStorage.setItem(key, String(v)); } catch { /* ignore */ }
    },
  };
}

const _markOnScrollR = makeReadingBoolPref('tap.reading.markOnScroll', true);
const _autoOpenNext = makeReadingBoolPref('tap.reading.autoOpenNext', false);
const _showSummaries = makeReadingBoolPref('tap.reading.showSummaries', true);
const _openLinksNewTab = makeReadingBoolPref('tap.reading.openLinksNewTab', true);

export const reading = {
  get markOnScroll() { return _markOnScrollR.value; },
  set markOnScroll(v: boolean) { _markOnScrollR.value = v; },
  get autoOpenNext() { return _autoOpenNext.value; },
  set autoOpenNext(v: boolean) { _autoOpenNext.value = v; },
  get showSummaries() { return _showSummaries.value; },
  set showSummaries(v: boolean) { _showSummaries.value = v; },
  get openLinksNewTab() { return _openLinksNewTab.value; },
  set openLinksNewTab(v: boolean) { _openLinksNewTab.value = v; },
};

type PollInterval = '5m' | '15m' | '1h' | 'manual';
const POLL_INTERVALS: PollInterval[] = ['5m', '15m', '1h', 'manual'];

function makePollPref() {
  const raw = typeof localStorage !== 'undefined' ? localStorage.getItem('tap.poll.interval') : null;
  let value = $state<PollInterval>(
    POLL_INTERVALS.includes(raw as PollInterval) ? (raw as PollInterval) : '15m',
  );
  return {
    get interval() { return value; },
    set interval(v: PollInterval) {
      value = v;
      try { localStorage.setItem('tap.poll.interval', v); } catch { /* ignore */ }
    },
  };
}

export const poll = makePollPref();
