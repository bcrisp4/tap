import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { get } from 'svelte/store';

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
