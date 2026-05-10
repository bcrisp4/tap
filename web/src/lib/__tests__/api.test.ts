import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// We mock globalThis.fetch to avoid real network calls.
const mockFetch = vi.fn();
beforeEach(() => {
  vi.stubGlobal('fetch', mockFetch);
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.resetAllMocks();
});

function makeResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('api.listSubscriptions', () => {
  it('calls GET /api/v1/subscriptions and returns the data array', async () => {
    const { api } = await import('../api');
    const subs = [{ id: 1, title: 'Test Feed' }];
    mockFetch.mockResolvedValueOnce(makeResponse({ data: subs }));

    const result = await api.listSubscriptions();

    expect(mockFetch).toHaveBeenCalledWith(
      '/api/v1/subscriptions',
      expect.objectContaining({ headers: expect.objectContaining({ 'Content-Type': 'application/json' }) }),
    );
    expect(result).toEqual(subs);
  });
});

describe('api.addSubscription', () => {
  it('POSTs to /api/v1/subscriptions with the body fields', async () => {
    const { api } = await import('../api');
    const sub = { id: 2, title: 'New Feed', feed_url: 'https://example.com/feed' };
    mockFetch.mockResolvedValueOnce(makeResponse(sub, 201));

    const result = await api.addSubscription({ feed_url: 'https://example.com/feed' });

    expect(mockFetch).toHaveBeenCalledWith(
      '/api/v1/subscriptions',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ feed_url: 'https://example.com/feed' }),
      }),
    );
    expect(result).toEqual(sub);
  });
});

describe('api.deleteSubscription', () => {
  it('sends DELETE /api/v1/subscriptions/:id and returns undefined on 204', async () => {
    const { api } = await import('../api');
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const result = await api.deleteSubscription(3);

    expect(mockFetch).toHaveBeenCalledWith(
      '/api/v1/subscriptions/3',
      expect.objectContaining({ method: 'DELETE' }),
    );
    expect(result).toBeUndefined();
  });
});

describe('api.listEntries', () => {
  it('calls GET /api/v1/entries with no query string when no params', async () => {
    const { api } = await import('../api');
    mockFetch.mockResolvedValueOnce(makeResponse({ data: [] }));

    await api.listEntries();

    expect(mockFetch).toHaveBeenCalledWith(
      '/api/v1/entries',
      expect.any(Object),
    );
  });

  it('appends unread=1 when unread param is true', async () => {
    const { api } = await import('../api');
    mockFetch.mockResolvedValueOnce(makeResponse({ data: [] }));

    await api.listEntries({ unread: true });

    const url = mockFetch.mock.calls[0][0] as string;
    expect(url).toContain('unread=1');
  });

  it('appends limit and feed query params', async () => {
    const { api } = await import('../api');
    mockFetch.mockResolvedValueOnce(makeResponse({ data: [] }));

    await api.listEntries({ limit: 50, feed: 7 });

    const url = mockFetch.mock.calls[0][0] as string;
    expect(url).toContain('limit=50');
    expect(url).toContain('feed=7');
  });

  it('appends cursor query param when cursor is provided', async () => {
    const { api } = await import('../api');
    mockFetch.mockResolvedValueOnce(makeResponse({ data: [] }));

    await api.listEntries({ cursor: '1700000000_42' });

    const url = mockFetch.mock.calls[0][0] as string;
    expect(url).toContain('cursor=1700000000_42');
  });

  it('returns the full ListResponse including next_cursor', async () => {
    const { api } = await import('../api');
    const payload = { data: [{ id: 1 }], next_cursor: '1700000000_1' };
    mockFetch.mockResolvedValueOnce(makeResponse(payload));

    const result = await api.listEntries();

    expect(result.data).toEqual(payload.data);
    expect(result.next_cursor).toBe('1700000000_1');
  });
});

describe('api.getEntry', () => {
  it('calls GET /api/v1/entries/:id', async () => {
    const { api } = await import('../api');
    const entry = { id: 5, title: 'Entry', content: '<p>hello</p>' };
    mockFetch.mockResolvedValueOnce(makeResponse(entry));

    const result = await api.getEntry(5);

    expect(mockFetch).toHaveBeenCalledWith('/api/v1/entries/5', expect.any(Object));
    expect(result).toEqual(entry);
  });
});

describe('api.patchEntry', () => {
  it('sends PATCH /api/v1/entries/:id with the patch body', async () => {
    const { api } = await import('../api');
    const updated = { id: 5, read: true };
    mockFetch.mockResolvedValueOnce(makeResponse(updated));

    const result = await api.patchEntry(5, { read: true });

    expect(mockFetch).toHaveBeenCalledWith(
      '/api/v1/entries/5',
      expect.objectContaining({
        method: 'PATCH',
        body: JSON.stringify({ read: true }),
      }),
    );
    expect(result).toEqual(updated);
  });
});

describe('api error handling', () => {
  it('throws an error with the API error message on non-ok response', async () => {
    const { api } = await import('../api');
    const errBody = { error: { code: 'not_found', message: 'Entry not found' } };
    mockFetch.mockResolvedValueOnce(new Response(JSON.stringify(errBody), { status: 404 }));

    await expect(api.getEntry(99)).rejects.toThrow('Entry not found');
  });

  it('throws a fallback message when error body is not parseable', async () => {
    const { api } = await import('../api');
    mockFetch.mockResolvedValueOnce(new Response('bad gateway', { status: 502 }));

    await expect(api.getEntry(99)).rejects.toThrow('502');
  });
});

// M6: CSRF + 401 handling + changePassword.
async function loadApi() {
  vi.resetModules();
  const apiMod = await import('../api');
  const authMod = await import('../auth');
  return { api: apiMod.api, auth: authMod.auth };
}

describe('api client (M6 auth integration)', () => {
  it('attaches X-CSRF-Token on POST when csrfToken is set', async () => {
    mockFetch.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'tok' }),
      { status: 200 },
    ));
    const { api, auth } = await loadApi();
    await auth.login('ben', 'pw');

    mockFetch.mockResolvedValueOnce(new Response('{}', { status: 201 }));
    await api.addSubscription({ feed_url: 'https://x' });

    const lastCall = mockFetch.mock.calls[mockFetch.mock.calls.length - 1];
    const init = lastCall[1] as RequestInit;
    const headers = init.headers as Record<string, string>;
    expect(headers['X-CSRF-Token']).toBe('tok');
  });

  it('does not attach X-CSRF-Token on GET', async () => {
    mockFetch.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'tok' }),
      { status: 200 },
    ));
    const { api, auth } = await loadApi();
    await auth.login('ben', 'pw');

    mockFetch.mockResolvedValueOnce(new Response(JSON.stringify({ data: [] }), { status: 200 }));
    await api.listSubscriptions();

    const lastCall = mockFetch.mock.calls[mockFetch.mock.calls.length - 1];
    const init = lastCall[1] as RequestInit;
    const headers = (init.headers ?? {}) as Record<string, string>;
    expect(headers['X-CSRF-Token']).toBeUndefined();
  });

  it('clears auth state when any request returns 401', async () => {
    mockFetch.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'tok' }),
      { status: 200 },
    ));
    const { api, auth } = await loadApi();
    await auth.login('ben', 'pw');

    mockFetch.mockResolvedValueOnce(new Response('', { status: 401 }));
    await expect(api.listSubscriptions()).rejects.toThrow();

    const { get } = await import('svelte/store');
    expect(get(auth).user).toBeNull();
  });

  it('changePassword updates auth.csrfToken from the response', async () => {
    mockFetch.mockResolvedValueOnce(new Response(
      JSON.stringify({ user: { id: 1, username: 'ben', role: 'admin' }, csrf_token: 'old' }),
      { status: 200 },
    ));
    const { api, auth } = await loadApi();
    await auth.login('ben', 'pw');

    mockFetch.mockResolvedValueOnce(new Response(
      JSON.stringify({ csrf_token: 'new' }),
      { status: 200 },
    ));
    await api.changePassword('pw', 'new-good-password');

    const { get } = await import('svelte/store');
    expect(get(auth).csrfToken).toBe('new');
  });
});
