import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';

// Helper to build fake entry content with N proxy URLs.
function makeContent(count: number, prefix = 'entry'): string {
  return Array.from({ length: count }, (_, i) =>
    `<img src="/api/v1/proxy/${prefix}-${i}.abc123">`
  ).join('');
}

beforeEach(() => vi.clearAllMocks());

let warmCache: typeof import('../warmCache').warmCache;
beforeEach(async () => {
  vi.resetModules();
  warmCache = (await import('../warmCache')).warmCache;
});

afterEach(() => vi.restoreAllMocks());

describe('warmCache — URL extraction', () => {
  it('extracts proxy URLs from entry content', async () => {
    const fetched: string[] = [];
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      fetched.push(url);
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({
        data: [{ id: 1, content: makeContent(2) }],
      }) });
    }));
    await warmCache(1);
    // First call is the entries fetch; subsequent calls are proxy URLs.
    const proxyFetches = fetched.filter(u => u.includes('/proxy/'));
    expect(proxyFetches).toHaveLength(2);
  });

  it('deduplicates URLs across entries', async () => {
    const sharedUrl = '/api/v1/proxy/shared.abc123';
    const fetched: string[] = [];
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      fetched.push(url);
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({
        data: [
          { id: 1, content: `<img src="${sharedUrl}">` },
          { id: 2, content: `<img src="${sharedUrl}">` },
        ],
      }) });
    }));
    await warmCache(1);
    const proxyFetches = fetched.filter(u => u.includes('/proxy/'));
    expect(proxyFetches).toHaveLength(1);
  });
});

describe('warmCache — per-entry URL cap', () => {
  it('caps at 20 proxy URLs per entry', async () => {
    const fetched: string[] = [];
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      fetched.push(url);
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({
        data: [{ id: 1, content: makeContent(30) }], // 30 URLs in one entry
      }) });
    }));
    await warmCache(1);
    const proxyFetches = fetched.filter(u => u.includes('/proxy/'));
    expect(proxyFetches).toHaveLength(20);
  });
});

describe('warmCache — total URL cap', () => {
  it('caps total warm list at 200 URLs', async () => {
    // 20 entries × 20 URLs each = 400 possible; should be capped at 200.
    const fetched: string[] = [];
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      fetched.push(url);
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({
        data: Array.from({ length: 20 }, (_, i) => ({ id: i, content: makeContent(20, `e${i}`) })),
      }) });
    }));
    await warmCache(1);
    const proxyFetches = fetched.filter(u => u.includes('/proxy/'));
    expect(proxyFetches).toHaveLength(200);
  });
});

describe('warmCache — concurrency', () => {
  it('runs at most 4 fetches concurrently', async () => {
    let inFlight = 0;
    let maxInFlight = 0;
    // Give entries that produce exactly 8 proxy URLs.
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      if (!url.includes('/proxy/')) {
        return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({
          data: [{ id: 1, content: makeContent(8) }],
        }) });
      }
      inFlight++;
      maxInFlight = Math.max(maxInFlight, inFlight);
      return new Promise(resolve =>
        setTimeout(() => {
          inFlight--;
          resolve({ ok: true, status: 200 });
        }, 5)
      );
    }));
    await warmCache(1);
    expect(maxInFlight).toBeLessThanOrEqual(4);
  });
});

describe('warmCache — error handling', () => {
  it('swallows fetch errors silently', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network error')));
    await expect(warmCache(1)).resolves.toBeUndefined();
  });

  it('swallows 401 silently without triggering re-auth', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 401 }));
    await expect(warmCache(1)).resolves.toBeUndefined();
  });

  it('no-ops when entries response is empty', async () => {
    const fetched: string[] = [];
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      fetched.push(url);
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ data: [] }) });
    }));
    await warmCache(1);
    expect(fetched).toHaveLength(1); // Only the entries fetch.
  });

  it('uses plain fetch (no cache option) so SW CacheFirst strategy is populated', async () => {
    const fetchMock = vi.fn().mockImplementation((_url: string) => {
      return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({
        data: [{ id: 1, content: makeContent(1) }],
      }) });
    });
    vi.stubGlobal('fetch', fetchMock);
    await warmCache(1);
    const proxyCalls = fetchMock.mock.calls.filter((call: unknown[]) => (call[0] as string).includes('/proxy/'));
    // Each proxy fetch must have NO second argument, or second arg with no 'cache' key.
    for (const call of proxyCalls) {
      expect((call[1] as RequestInit | undefined)?.cache).toBeUndefined();
    }
  });
});
