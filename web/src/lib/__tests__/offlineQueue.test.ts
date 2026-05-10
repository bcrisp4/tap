import { describe, it, expect, beforeEach, vi } from 'vitest';

// We test offlineQueue in isolation with a mocked localStorage and fetch.

// Inline the types so tests don't depend on the not-yet-created module.
type QueuedMutation = {
  id: string;
  method: string;
  path: string;
  body: unknown;
  csrfToken: string;
  enqueuedAt: number;
};

// Storage helpers used by the module under test.
const storageKey = (uid: number) => `tap:queue:${uid}`;
const pendingKey = (uid: number) => `tap:queue-pending:${uid}`;

// Minimal localStorage mock (jsdom provides one; we just reset it between tests).
beforeEach(() => localStorage.clear());

// Defer the real import until after we define the mocks.
// vi.resetModules() on every beforeEach gives each test a fresh module instance,
// which also resets the module-level `draining` flag — no separate reset helper needed.
let offlineQueue: typeof import('../offlineQueue').offlineQueue;
beforeEach(async () => {
  vi.resetModules();
  offlineQueue = (await import('../offlineQueue')).offlineQueue;
});

describe('offlineQueue.enqueue', () => {
  it('writes mutation to localStorage keyed by user ID', () => {
    offlineQueue.enqueue(42, { method: 'PATCH', path: '/entries/1', body: { read: true }, csrfToken: 'tok' });
    const raw = localStorage.getItem(storageKey(42));
    expect(raw).not.toBeNull();
    const items: QueuedMutation[] = JSON.parse(raw!);
    expect(items).toHaveLength(1);
    expect(items[0].path).toBe('/entries/1');
    expect(items[0].csrfToken).toBe('tok');
  });

  it('preserves insertion order on multiple enqueues', () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/1', body: { read: true }, csrfToken: 't' });
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/2', body: { saved: true }, csrfToken: 't' });
    const items: QueuedMutation[] = JSON.parse(localStorage.getItem(storageKey(1))!);
    expect(items[0].path).toBe('/entries/1');
    expect(items[1].path).toBe('/entries/2');
  });

  it('does not cross-contaminate users', () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/a', body: {}, csrfToken: 't' });
    offlineQueue.enqueue(2, { method: 'PATCH', path: '/b', body: {}, csrfToken: 't' });
    expect(JSON.parse(localStorage.getItem(storageKey(1))!)).toHaveLength(1);
    expect(JSON.parse(localStorage.getItem(storageKey(2))!)).toHaveLength(1);
  });
});

describe('offlineQueue.drain — happy path', () => {
  it('dequeues mutation on 2xx and continues', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/1', body: { read: true }, csrfToken: 'tok' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200 }));
    await offlineQueue.drain(1);
    expect(JSON.parse(localStorage.getItem(storageKey(1)) ?? '[]')).toHaveLength(0);
  });

  it('processes mutations in order', async () => {
    const calls: string[] = [];
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/1', body: {}, csrfToken: 't' });
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/2', body: {}, csrfToken: 't' });
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      calls.push(url);
      return Promise.resolve({ ok: true, status: 200 });
    }));
    await offlineQueue.drain(1);
    expect(calls[0]).toContain('/entries/1');
    expect(calls[1]).toContain('/entries/2');
  });
});

describe('offlineQueue.drain — 401 handling', () => {
  it('pauses drain and sets pendingDrain flag on 401', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/1', body: {}, csrfToken: 't' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false, status: 401,
      json: () => Promise.resolve({ error: { code: 'invalid_session' } }),
    }));
    await offlineQueue.drain(1);
    // Queue NOT cleared — preserved for after re-login.
    expect(JSON.parse(localStorage.getItem(storageKey(1))!)).toHaveLength(1);
    // pendingDrain flag set.
    expect(localStorage.getItem(pendingKey(1))).toBe('1');
  });
});

describe('offlineQueue.drain — 403 csrf_invalid handling', () => {
  it('re-fetches CSRF token, retries once, dequeues on success', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/1', body: {}, csrfToken: 'old' });
    let callCount = 0;
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      callCount++;
      if (url.includes('/sessions/current')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ csrf_token: 'new', user: {} }) });
      }
      if (callCount === 1) {
        return Promise.resolve({ ok: false, status: 403, json: () => Promise.resolve({ error: { code: 'csrf_invalid' } }) });
      }
      return Promise.resolve({ ok: true, status: 200 });
    }));
    await offlineQueue.drain(1);
    expect(JSON.parse(localStorage.getItem(storageKey(1)) ?? '[]')).toHaveLength(0);
  });

  it('dequeues and logs if retry also returns 403', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/1', body: {}, csrfToken: 'old' });
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      if (url.includes('/sessions/current')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ csrf_token: 'new', user: {} }) });
      }
      return Promise.resolve({ ok: false, status: 403, json: () => Promise.resolve({ error: { code: 'csrf_invalid' } }) });
    }));
    await offlineQueue.drain(1);
    // Dequeued (can't recover from double-403).
    expect(JSON.parse(localStorage.getItem(storageKey(1)) ?? '[]')).toHaveLength(0);
  });
});

describe('offlineQueue.drain — network error', () => {
  it('stops drain on network error; next drain retries from failed mutation', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/1', body: {}, csrfToken: 't' });
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/2', body: {}, csrfToken: 't' });
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network error')));
    await offlineQueue.drain(1);
    // Both mutations still queued.
    expect(JSON.parse(localStorage.getItem(storageKey(1))!)).toHaveLength(2);
  });
});

describe('offlineQueue.drain — unrecoverable 4xx', () => {
  it('dequeues and continues on other 4xx', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/1', body: {}, csrfToken: 't' });
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/2', body: {}, csrfToken: 't' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false, status: 404, json: () => Promise.resolve({ error: { code: 'not_found' } }),
    }));
    await offlineQueue.drain(1);
    expect(JSON.parse(localStorage.getItem(storageKey(1)) ?? '[]')).toHaveLength(0);
  });
});

describe('offlineQueue.drain — 5xx transient error', () => {
  it('stops drain on 5xx (keeps head) for retry on next drain', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/1', body: {}, csrfToken: 't' });
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/2', body: {}, csrfToken: 't' });
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false, status: 500, json: () => Promise.resolve({ error: { code: 'internal_error' } }),
    }));
    await offlineQueue.drain(1);
    // Both mutations still queued — 500 is transient.
    expect(JSON.parse(localStorage.getItem(storageKey(1))!)).toHaveLength(2);
  });
});

describe('offlineQueue.drain — re-entrancy', () => {
  it('ignores concurrent drain calls (draining flag)', async () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/entries/1', body: {}, csrfToken: 't' });
    let fetchCount = 0;
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => {
      fetchCount++;
      return new Promise(resolve => setTimeout(() => resolve({ ok: true, status: 200 }), 10));
    }));
    // Start two drains concurrently.
    const [d1, d2] = [offlineQueue.drain(1), offlineQueue.drain(1)];
    await Promise.all([d1, d2]);
    // Only one fetch should have fired.
    expect(fetchCount).toBe(1);
  });
});

describe('offlineQueue.drain — reload survival', () => {
  it('drains queue written by a previous instance', async () => {
    // Simulate a previous page session writing to localStorage directly.
    const mutation: QueuedMutation = {
      id: 'abc', method: 'PATCH', path: '/entries/99',
      body: { read: true }, csrfToken: 'tok', enqueuedAt: Date.now(),
    };
    localStorage.setItem(storageKey(5), JSON.stringify([mutation]));
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200 }));
    // Fresh module instance picks it up.
    vi.resetModules();
    const fresh = (await import('../offlineQueue')).offlineQueue;
    await fresh.drain(5);
    expect(JSON.parse(localStorage.getItem(storageKey(5)) ?? '[]')).toHaveLength(0);
  });
});

describe('offlineQueue.clearForUser', () => {
  it('removes only the specified user keys', () => {
    offlineQueue.enqueue(1, { method: 'PATCH', path: '/a', body: {}, csrfToken: 't' });
    offlineQueue.enqueue(2, { method: 'PATCH', path: '/b', body: {}, csrfToken: 't' });
    localStorage.setItem(pendingKey(1), '1');
    offlineQueue.clearForUser(1);
    expect(localStorage.getItem(storageKey(1))).toBeNull();
    expect(localStorage.getItem(pendingKey(1))).toBeNull();
    // User 2 unaffected.
    expect(localStorage.getItem(storageKey(2))).not.toBeNull();
  });
});
