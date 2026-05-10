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

describe('density', () => {
  it('defaults to "default"', async () => {
    const { density } = await import('../preferences.svelte');
    expect(density.value).toBe('default');
  });
  it('reads stored value', async () => {
    store['tap.density'] = 'compact';
    const { density } = await import('../preferences.svelte');
    expect(density.value).toBe('compact');
  });
  it('persists on set', async () => {
    const { density } = await import('../preferences.svelte');
    density.value = 'comfortable';
    expect(store['tap.density']).toBe('comfortable');
  });
});
