import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';

const store: Record<string, string> = {};
let mqListeners: ((e: { matches: boolean }) => void)[] = [];
let mqMatchesDark = false;

beforeEach(() => {
  mqListeners = [];
  mqMatchesDark = false;
  vi.stubGlobal('localStorage', {
    getItem: (k: string) => store[k] ?? null,
    setItem: (k: string, v: string) => { store[k] = v; },
    removeItem: (k: string) => { delete store[k]; },
  });
  Object.keys(store).forEach(k => delete store[k]);
  vi.stubGlobal('matchMedia', (query: string) => ({
    get matches() { return query === '(prefers-color-scheme: dark)' ? mqMatchesDark : false; },
    media: query,
    addEventListener: (_: string, fn: (e: { matches: boolean }) => void) => {
      if (query === '(prefers-color-scheme: dark)') mqListeners.push(fn);
    },
    removeEventListener: vi.fn(),
  }));
});
afterEach(() => { vi.unstubAllGlobals(); vi.resetModules(); });

describe('theme', () => {
  it('defaults to "system" when localStorage is empty', async () => {
    const { theme } = await import('../preferences.svelte');
    expect(theme.stored).toBe('system');
  });
  it('reads stored value from localStorage', async () => {
    store['tap.theme'] = 'dark';
    const { theme } = await import('../preferences.svelte');
    expect(theme.stored).toBe('dark');
  });
  it('falls back to "system" when localStorage contains an invalid theme value', async () => {
    store['tap.theme'] = 'invalid-value';
    const { theme } = await import('../preferences.svelte');
    expect(theme.stored).toBe('system');
  });
  it('resolves "system" to "light" when matchMedia does not match dark', async () => {
    const { theme } = await import('../preferences.svelte');
    expect(theme.resolved).toBe('light');
  });
  it('resolves "system" to "dark" when matchMedia matches dark at import time', async () => {
    mqMatchesDark = true;
    const { theme } = await import('../preferences.svelte');
    expect(theme.resolved).toBe('dark');
  });
  it('re-resolves to "dark" when OS theme changes to dark while stored==="system"', async () => {
    const { theme } = await import('../preferences.svelte');
    expect(theme.resolved).toBe('light');
    mqMatchesDark = true;
    mqListeners.forEach(fn => fn({ matches: true }));
    expect(theme.resolved).toBe('dark');
  });
  it('does NOT re-resolve when preference is an explicit value', async () => {
    store['tap.theme'] = 'sepia';
    const { theme } = await import('../preferences.svelte');
    mqMatchesDark = true;
    mqListeners.forEach(fn => fn({ matches: true }));
    expect(theme.resolved).toBe('sepia');
  });
  it('resolved returns stored value directly when not "system"', async () => {
    store['tap.theme'] = 'sepia';
    const { theme } = await import('../preferences.svelte');
    expect(theme.resolved).toBe('sepia');
  });
  it('persists to localStorage when stored is set', async () => {
    const { theme } = await import('../preferences.svelte');
    theme.stored = 'sepia';
    expect(store['tap.theme']).toBe('sepia');
  });
});

describe('font', () => {
  it('falls back to "serif" when localStorage contains an invalid font value', async () => {
    store['tap.font'] = 'Comic Sans';
    const { font } = await import('../preferences.svelte');
    expect(font.value).toBe('serif');
  });
  it('defaults to "serif"', async () => {
    const { font } = await import('../preferences.svelte');
    expect(font.value).toBe('serif');
  });
  it('reads stored value', async () => {
    store['tap.font'] = 'sans';
    const { font } = await import('../preferences.svelte');
    expect(font.value).toBe('sans');
  });
  it('persists on set', async () => {
    const { font } = await import('../preferences.svelte');
    font.value = 'sans';
    expect(store['tap.font']).toBe('sans');
  });
});

describe('density pref (canonical vocabulary)', () => {
  it('falls back to "comfortable" when localStorage contains an invalid density value', async () => {
    store['tap.density'] = 'ultra-compact';
    const { density } = await import('../preferences.svelte');
    expect(density.value).toBe('comfortable');
  });
  it('defaults to comfortable when localStorage empty', async () => {
    const { density } = await import('../preferences.svelte');
    expect(density.value).toBe('comfortable');
  });
  it('migrates legacy "default" value to "comfortable"', async () => {
    store['tap.density'] = 'default';
    const { density } = await import('../preferences.svelte');
    expect(density.value).toBe('comfortable');
    // Migration persists the new value so it doesn't fire again.
    expect(store['tap.density']).toBe('comfortable');
  });
  it('reads stored value "compact"', async () => {
    store['tap.density'] = 'compact';
    const { density } = await import('../preferences.svelte');
    expect(density.value).toBe('compact');
  });
  it('accepts cosy', async () => {
    const { density } = await import('../preferences.svelte');
    density.value = 'cosy';
    expect(density.value).toBe('cosy');
    expect(store['tap.density']).toBe('cosy');
  });
  it('persists on set', async () => {
    const { density } = await import('../preferences.svelte');
    density.value = 'comfortable';
    expect(store['tap.density']).toBe('comfortable');
  });
});

describe('measure pref', () => {
  it('defaults to comfortable', async () => {
    const { measure } = await import('../preferences.svelte');
    expect(measure.value).toBe('comfortable');
  });

  it('persists to localStorage', async () => {
    const { measure } = await import('../preferences.svelte');
    measure.value = 'wide';
    expect(store['tap.measure']).toBe('wide');
  });
});

describe('markOnScroll preference', () => {
  it('defaults to true when no value is stored', async () => {
    const { markOnScroll } = await import('../preferences.svelte');
    expect(markOnScroll.value).toBe(true);
  });

  it('round-trips false via setter', async () => {
    const { markOnScroll } = await import('../preferences.svelte');
    markOnScroll.value = false;
    expect(markOnScroll.value).toBe(false);
  });

  it('persists false to localStorage as "0"', async () => {
    const { markOnScroll } = await import('../preferences.svelte');
    markOnScroll.value = false;
    expect(store['tap.markOnScroll']).toBe('0');
  });

  it('persists true to localStorage as "1"', async () => {
    const { markOnScroll } = await import('../preferences.svelte');
    markOnScroll.value = true;
    expect(store['tap.markOnScroll']).toBe('1');
  });
});

describe('reading prefs', () => {
  it('defaults markOnScroll to true, autoOpenNext to false, showSummaries to true, openLinksNewTab to true', async () => {
    const { reading } = await import('../preferences.svelte');
    expect(reading.markOnScroll).toBe(true);
    expect(reading.autoOpenNext).toBe(false);
    expect(reading.showSummaries).toBe(true);
    expect(reading.openLinksNewTab).toBe(true);
  });

  it('persists changes to localStorage', async () => {
    const { reading } = await import('../preferences.svelte');
    reading.markOnScroll = false;
    expect(store['tap.reading.markOnScroll']).toBe('false');
  });

  it('reads back a persisted value', async () => {
    store['tap.reading.autoOpenNext'] = 'true';
    const { reading } = await import('../preferences.svelte');
    expect(reading.autoOpenNext).toBe(true);
  });

  it('coerces invalid localStorage values back to default', async () => {
    store['tap.reading.markOnScroll'] = 'banana';
    const { reading } = await import('../preferences.svelte');
    expect(reading.markOnScroll).toBe(true);
  });
});

describe('poll pref (advisory display)', () => {
  it('defaults to "15m"', async () => {
    const { poll } = await import('../preferences.svelte');
    expect(poll.interval).toBe('15m');
  });

  it('accepts only 5m | 15m | 1h | manual', async () => {
    const { poll } = await import('../preferences.svelte');
    poll.interval = '5m';
    expect(poll.interval).toBe('5m');
    expect(store['tap.poll.interval']).toBe('5m');
  });

  it('falls back to 15m for invalid stored value', async () => {
    store['tap.poll.interval'] = 'forever';
    const { poll } = await import('../preferences.svelte');
    expect(poll.interval).toBe('15m');
  });
});
