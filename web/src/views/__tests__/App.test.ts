import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { tick } from 'svelte';
import { writable } from 'svelte/store';

// Mutable auth state for testing.
const authStore = writable({ user: null as null | { id: number; username: string; role: string }, csrfToken: null as string | null, bootstrapped: true });
const mockBootstrap = vi.fn().mockResolvedValue(undefined);

vi.mock('../../lib/auth', () => ({
  auth: {
    subscribe: (cb: (s: unknown) => void) => authStore.subscribe(cb),
    bootstrap: () => mockBootstrap(),
  },
  ERR_UNAUTHORIZED: 'unauthorized',
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
vi.mock('../../views/Categories.svelte', () => ({ default: vi.fn() }));
vi.mock('../../views/Feeds.svelte', () => ({ default: vi.fn() }));
vi.mock('../../views/History.svelte', () => ({ default: vi.fn() }));
vi.mock('../../views/Settings.svelte', () => ({ default: vi.fn() }));
vi.mock('../../views/Admin.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/HotkeysModal.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/AppShell.svelte', () => ({ default: vi.fn() }));
vi.mock('../../components/SearchOverlay.svelte', () => ({ default: vi.fn() }));

vi.mock('../../lib/store', () => ({
  entries: { subscribe: (fn: (v: unknown) => void) => { fn({ items: [] }); return () => {}; }, toggleRead: vi.fn() },
  subscriptions: { subscribe: (fn: (v: unknown) => void) => { fn([]); return () => {}; }, load: vi.fn() },
}));

const mockNavigate = vi.fn();
const routeStore = writable({ name: 'unread' });
vi.mock('../../lib/router', () => ({
  navigate: (...args: unknown[]) => mockNavigate(...args),
  route: { subscribe: (fn: (v: unknown) => void) => routeStore.subscribe(fn) },
}));

vi.mock('../../lib/preferences.svelte', () => ({
  theme: { resolved: 'light' },
  font: { value: 'serif' },
  density: { value: 'comfortable' },
}));
vi.mock('../../lib/keyboard', () => ({
  buildHandler: () => () => {},
}));
vi.mock('../../lib/searchOverlay.svelte', () => ({
  searchOverlay: { open: false, openOverlay: vi.fn(), close: vi.fn(), query: '', scope: 'unread' },
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

describe('App — auth-redirect contract', () => {
  beforeEach(() => {
    mockNavigate.mockClear();
    routeStore.set({ name: 'unread' });
  });

  it('unauthenticated user on / gets redirected to /sign-in', async () => {
    authStore.set({ user: null, csrfToken: null, bootstrapped: true });
    routeStore.set({ name: 'unread' });

    render(App);
    await tick();

    expect(mockNavigate).toHaveBeenCalledWith('/sign-in');
  });

  it('authenticated user on /sign-in gets redirected to /', async () => {
    authStore.set({ user: { id: 1, username: 'ada', role: 'user' }, csrfToken: 'tok', bootstrapped: true });
    routeStore.set({ name: 'signin' });

    render(App);
    await tick();

    expect(mockNavigate).toHaveBeenCalledWith('/');
  });
});
