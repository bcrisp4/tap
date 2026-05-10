import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { get } from 'svelte/store';

// Mock offlineQueue so auth.ts can import it without side effects.
vi.mock('../offlineQueue', () => ({
  offlineQueue: { clearForUser: vi.fn(), enqueue: vi.fn(), drain: vi.fn() },
}));

// Mock fetch globally.
const fetchMock = vi.fn();
beforeEach(() => {
  vi.stubGlobal('fetch', fetchMock);
  fetchMock.mockReset();
});
afterEach(() => {
  vi.unstubAllGlobals();
});

// Re-import per test to reset module-level state.
async function loadAuth() {
  vi.resetModules();
  const mod = await import('../auth');
  return mod.auth;
}

describe('auth store', () => {
  it('bootstraps from a successful GET /sessions/current', async () => {
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'tok' }),
      { status: 200 },
    ));
    const auth = await loadAuth();
    await auth.bootstrap();
    const s = get(auth);
    expect(s.user?.username).toBe('ben');
    expect(s.csrfToken).toBe('tok');
    expect(s.bootstrapped).toBe(true);
  });

  it('clears state on a 401 from /sessions/current', async () => {
    fetchMock.mockResolvedValueOnce(new Response('', { status: 401 }));
    const auth = await loadAuth();
    await auth.bootstrap();
    const s = get(auth);
    expect(s.user).toBeNull();
    expect(s.csrfToken).toBeNull();
    expect(s.bootstrapped).toBe(true);
  });

  it('login populates user + csrfToken on success', async () => {
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'tok2' }),
      { status: 200 },
    ));
    const auth = await loadAuth();
    await auth.login('ben', 'pw');
    const s = get(auth);
    expect(s.user?.username).toBe('ben');
    expect(s.csrfToken).toBe('tok2');
  });

  it('login throws on 401 and leaves state untouched', async () => {
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ error: { code: 'invalid_credentials', message: 'bad' } }),
      { status: 401 },
    ));
    const auth = await loadAuth();
    await expect(auth.login('ben', 'wrong')).rejects.toThrow();
    const s = get(auth);
    expect(s.user).toBeNull();
  });

  it('logout clears state', async () => {
    // First bootstrap to populate.
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'tok' }),
      { status: 200 },
    ));
    const auth = await loadAuth();
    await auth.bootstrap();

    // Then logout. (null body: spec disallows a body for 204 responses.)
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await auth.logout();
    const s = get(auth);
    expect(s.user).toBeNull();
    expect(s.csrfToken).toBeNull();
  });
});

describe('auth — SW postMessage on bootstrap', () => {
  it('posts set-user message to SW controller after successful bootstrap', async () => {
    const postMessage = vi.fn();
    Object.defineProperty(navigator, 'serviceWorker', {
      value: { controller: { postMessage } },
      configurable: true,
    });
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 7, username: 'ben', role: 'admin' }, csrf_token: 'tok' }),
      { status: 200 },
    ));
    const auth = await loadAuth();
    await auth.bootstrap();
    expect(postMessage).toHaveBeenCalledWith({ type: 'set-user', userId: 7 });
  });
});

describe('auth — logout clears queue and notifies SW', () => {
  it('calls offlineQueue.clearForUser and posts logout to SW', async () => {
    const postMessage = vi.fn();
    Object.defineProperty(navigator, 'serviceWorker', {
      value: { controller: { postMessage } },
      configurable: true,
    });
    // Bootstrap first.
    fetchMock.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 7, username: 'ben', role: 'admin' }, csrf_token: 'tok' }),
      { status: 200 },
    ));
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const auth = await loadAuth();
    const { offlineQueue } = await import('../offlineQueue');
    await auth.bootstrap();
    await auth.logout();

    expect(offlineQueue.clearForUser).toHaveBeenCalledWith(7);
    expect(postMessage).toHaveBeenCalledWith({ type: 'logout', userId: 7 });
  });
});
