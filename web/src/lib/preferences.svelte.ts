// .svelte.ts enables Svelte runes ($state, $derived) outside components.
type Theme = 'light' | 'dark' | 'sepia' | 'system';
type Font = 'serif' | 'sans';
type Density = 'compact' | 'default' | 'comfortable';

const THEMES: Theme[] = ['light', 'dark', 'sepia', 'system'];
const FONTS: Font[] = ['serif', 'sans'];
const DENSITIES: Density[] = ['compact', 'default', 'comfortable'];

const mq = typeof window !== 'undefined'
  ? window.matchMedia('(prefers-color-scheme: dark)')
  : null;

// prefersDark is $state so changes to it trigger $derived re-evaluation.
let prefersDark = $state(mq?.matches ?? false);

if (mq) {
  mq.addEventListener('change', (e) => {
    prefersDark = e.matches;
  });
}

function makeTheme() {
  const raw = localStorage.getItem('tap.theme');
  let stored = $state<Theme>(
    THEMES.includes(raw as Theme) ? (raw as Theme) : 'system'
  );
  const resolved = $derived<'light' | 'dark' | 'sepia'>(
    stored === 'system' ? (prefersDark ? 'dark' : 'light') : stored
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

export const theme = makeTheme();
export const font = makePref<Font>('tap.font', 'serif', FONTS);
export const density = makePref<Density>('tap.density', 'default', DENSITIES);
