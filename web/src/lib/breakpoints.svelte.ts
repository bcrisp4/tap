import { writable, type Readable } from 'svelte/store';

const MOBILE_QUERY = '(max-width: 768px)';

function makeIsMobile(): Readable<boolean> {
  // SSR guard: if matchMedia is undefined (Node/jsdom without stub), default to false.
  const supportsMQ = typeof window !== 'undefined' && typeof window.matchMedia === 'function';
  const initial = supportsMQ ? window.matchMedia(MOBILE_QUERY).matches : false;
  const { subscribe, set } = writable<boolean>(initial);

  if (supportsMQ) {
    const mql = window.matchMedia(MOBILE_QUERY);
    mql.addEventListener('change', (e: MediaQueryListEvent) => set(e.matches));
  }

  return { subscribe };
}

export const isMobile = makeIsMobile();
