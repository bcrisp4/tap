import { describe, it, expect, vi, beforeEach } from 'vitest';
import { getStatus } from '../status';

const mockFetch = vi.fn();
(globalThis as typeof globalThis & { fetch: typeof fetch }).fetch = mockFetch;

describe('getStatus', () => {
  beforeEach(() => { mockFetch.mockReset(); });

  it('returns status on 200', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        version: '0.12.0', uptime_seconds: 100, db: 'ok',
        polls_active: 0, polls_total: 0, last_poll_at: null, recent_errors: []
      })
    });
    const s = await getStatus();
    expect(s.version).toBe('0.12.0');
    expect(s.uptime_seconds).toBe(100);
    expect(s.recent_errors).toHaveLength(0);
  });

  it('throws on 403', async () => {
    mockFetch.mockResolvedValue({ ok: false, status: 403 });
    await expect(getStatus()).rejects.toThrow('status 403');
  });
});
