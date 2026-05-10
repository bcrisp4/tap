import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { writable } from 'svelte/store';

// jsdom does not implement matchMedia — stub it out before App.svelte loads.
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

// Mutable auth state for testing.
const authStore = writable({ user: null as null | { id: number; username: string; role: string }, csrfToken: null as string | null, bootstrapped: true });
const mockBootstrap = vi.fn().mockResolvedValue(undefined);

vi.mock('../../lib/auth', () => ({
  auth: {
    subscribe: (cb: (s: unknown) => void) => authStore.subscribe(cb),
    bootstrap: () => mockBootstrap(),
  },
}));

const mockDrain = vi.fn().mockResolvedValue(undefined);
const mockWarmCache = vi.fn().mockResolvedValue(undefined);

vi.mock('../../lib/offlineQueue', () => ({
  offlineQueue: { drain: (...args: unknown[]) => mockDrain(...args), clearForUser: vi.fn(), enqueue: vi.fn() },
}));
vi.mock('../../lib/warmCache', () => ({
  warmCache: (...args: unknown[]) => mockWarmCache(...args),
}));

// Mock all views / components so they render as minimal stubs.
vi.mock('../../views/Login.svelte', () => ({ default: vi.fn() }));
vi.mock('../../views/Unread.svelte', () => ({ default: vi.fn() }));
vi.mock('../../views/Reader.svelte', () => ({ default: vi.fn() }));
vi.mock('../../views/Saved.svelte', () => ({ default: vi.fn() }));
vi.mock('../../views/Search.svelte', () => ({ default: vi.fn() }));
vi.mock('../../views/Settings.svelte', () => ({ default: vi.fn() }));
vi.mock('../../views/Admin.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/HotkeysModal.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/TabBar.svelte', () => ({ default: vi.fn() }));

vi.mock('../../lib/store', () => ({
  entries: { subscribe: (fn: (v: unknown) => void) => { fn({ items: [] }); return () => {}; }, toggleRead: vi.fn() },
  subscriptions: { subscribe: (fn: (v: unknown) => void) => { fn([]); return () => {}; }, load: vi.fn() },
}));
vi.mock('../../lib/router', () => ({
  navigate: vi.fn(),
  route: { subscribe: (fn: (v: unknown) => void) => { fn({ name: 'unread', params: {} }); return () => {}; } },
}));
// Mock preferences.svelte to avoid window.matchMedia in jsdom.
vi.mock('../../lib/preferences.svelte', () => ({
  theme: { resolved: 'light' },
  font: { value: 'serif' },
  density: { value: 'default' },
}));
// Mock keyboard handler.
vi.mock('../../lib/keyboard', () => ({
  buildHandler: () => () => {},
}));

import App from '../../App.svelte';

describe('App — SW update banner', () => {
  it('does not render update banner when needRefresh is false (default)', () => {
    const { queryByText } = render(App);
    expect(queryByText(/Update available/i)).toBeNull();
  });
});

describe('App — online event triggers drain and warmCache', () => {
  beforeEach(() => {
    mockDrain.mockClear();
    mockWarmCache.mockClear();
    mockBootstrap.mockClear();
  });

  it('calls drain and warmCache when online event fires with authenticated user', async () => {
    authStore.set({ user: { id: 5, username: 'ben', role: 'admin' }, csrfToken: 'tok', bootstrapped: true });

    render(App);

    fireEvent(window, new Event('online'));

    await waitFor(() => {
      expect(mockDrain).toHaveBeenCalledWith(5);
    });
    expect(mockWarmCache).toHaveBeenCalledWith(5);
  });
});
