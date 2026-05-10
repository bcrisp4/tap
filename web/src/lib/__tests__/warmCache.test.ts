import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';

// Helper to build fake entry content with N proxy URLs.
function makeContent(count: number, prefix = 'entry'): string {
  return Array.from({ length: count }, (_, i) =>
    `<img src="/api/v1/proxy/${prefix}-${i}.abc123">`
  ).join('');
}

// The warmCache driver now does a two-phase fetch:
//   1. GET /api/v1/entries?limit=50&unread=1  → list of {id} items
//   2. GET /api/v1/entries/:id                 → detail with content
//   3. fetch each extracted proxy URL
//
// Mocks must return list shape for the list call and detail shape for detail calls.

function makeListResponse(ids: number[]) {
  return { ok: true, status: 200, json: () => Promise.resolve({ data: ids.map(id => ({ id })) }) };
}
function makeDetailResponse(content: string) {
  return { ok: true, status: 200, json: () => Promise.resolve({ content }) };
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
      if (url.includes('?limit=')) return Promise.resolve(makeListResponse([1]));
      if (url.includes('/entries/')) return Promise.resolve(makeDetailResponse(makeContent(2)));
      return Promise.resolve({ ok: true, status: 200 });
    }));
    await warmCache(1);
    const proxyFetches = fetched.filter(u => u.includes('/proxy/'));
    expect(proxyFetches).toHaveLength(2);
  });

  it('deduplicates URLs across entries', async () => {
    const sharedUrl = '/api/v1/proxy/shared.abc123';
    const fetched: string[] = [];
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      fetched.push(url);
      if (url.includes('?limit=')) return Promise.resolve(makeListResponse([1, 2]));
      if (url.includes('/entries/')) return Promise.resolve(makeDetailResponse(`<img src="${sharedUrl}">`));
      return Promise.resolve({ ok: true, status: 200 });
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
      if (url.includes('?limit=')) return Promise.resolve(makeListResponse([1]));
      if (url.includes('/entries/')) return Promise.resolve(makeDetailResponse(makeContent(30)));
      return Promise.resolve({ ok: true, status: 200 });
    }));
    await warmCache(1);
    const proxyFetches = fetched.filter(u => u.includes('/proxy/'));
    expect(proxyFetches).toHaveLength(20);
  });
});

describe('warmCache — total URL cap', () => {
  it('caps total warm list at 200 URLs', async () => {
    // 20 entries × 20 URLs each = 400 possible; should be capped at 200.
    const ids = Array.from({ length: 20 }, (_, i) => i + 1);
    const fetched: string[] = [];
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      fetched.push(url);
      if (url.includes('?limit=')) return Promise.resolve(makeListResponse(ids));
      if (url.includes('/entries/')) {
        const id = Number(url.split('/entries/')[1]);
        return Promise.resolve(makeDetailResponse(makeContent(20, `e${id}`)));
      }
      return Promise.resolve({ ok: true, status: 200 });
    }));
    await warmCache(1);
    const proxyFetches = fetched.filter(u => u.includes('/proxy/'));
    expect(proxyFetches).toHaveLength(200);
  });
});

describe('warmCache — concurrency', () => {
  it('runs at most 4 proxy fetches concurrently', async () => {
    let inFlight = 0;
    let maxInFlight = 0;
    vi.stubGlobal('fetch', vi.fn().mockImplementation((url: string) => {
      if (url.includes('?limit=')) return Promise.resolve(makeListResponse([1]));
      if (url.includes('/entries/')) return Promise.resolve(makeDetailResponse(makeContent(8)));
      // proxy fetches
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

  it('swallows 401 on list silently without triggering re-auth', async () => {
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
    expect(fetched).toHaveLength(1); // Only the list fetch.
  });

  it('uses plain fetch (no cache option) so SW CacheFirst strategy is populated', async () => {
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.includes('?limit=')) return Promise.resolve(makeListResponse([1]));
      if (url.includes('/entries/')) return Promise.resolve(makeDetailResponse(makeContent(1)));
      return Promise.resolve({ ok: true, status: 200 });
    });
    vi.stubGlobal('fetch', fetchMock);
    await warmCache(1);
    const proxyCalls = fetchMock.mock.calls.filter((call: unknown[]) => (call[0] as string).includes('/proxy/'));
    for (const call of proxyCalls) {
      expect((call[1] as RequestInit | undefined)?.cache).toBeUndefined();
    }
  });
});
