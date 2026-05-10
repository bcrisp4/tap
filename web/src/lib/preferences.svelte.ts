// .svelte.ts enables Svelte runes ($state, $derived) outside components.
type Theme = 'light' | 'dark' | 'sepia' | 'system';
type Font = 'serif' | 'sans';
type Density = 'compact' | 'default' | 'comfortable';

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
  let stored = $state<Theme>(
    (localStorage.getItem('tap.theme') as Theme) ?? 'system'
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

function makePref<T extends string>(key: string, def: T) {
  let value = $state<T>((localStorage.getItem(key) as T) ?? def);
  return {
    get value() { return value; },
    set value(v: T) { value = v; localStorage.setItem(key, v); },
  };
}

export const theme = makeTheme();
export const font = makePref<Font>('tap.font', 'serif');
export const density = makePref<Density>('tap.density', 'default');
