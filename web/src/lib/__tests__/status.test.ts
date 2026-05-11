import { describe, it, expect, beforeEach, vi } from 'vitest';
import { getStatus } from '../status';

describe('getStatus', () => {
  beforeEach(() => { vi.restoreAllMocks(); });

  it('returns parsed status on 200 including admin metric fields', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      version: 'v1.4.2',
      uptime_seconds: 86_400,
      db: 'ok',
      polls_active: 3,
      polls_total: 24,
      last_poll_at: 1_700_000_000,
      recent_errors: [
        { time: '2026-05-11T10:42:14Z', level: 'error', event: 'phoronix.com 502', attrs: {} },
      ],
      metrics_ok: true,
      feeds_total: 24,
      feeds_ok: 22,
      feeds_with_errors: 2,
      offending_feeds: ['Phoronix', 'LWN'],
      entries_total: 14_820,
      entries_24h: 1_402,
    }), { status: 200 }));

    const s = await getStatus();
    expect(s.version).toBe('v1.4.2');
    expect(s.db).toBe('ok');
    expect(s.metrics_ok).toBe(true);
    expect(s.feeds_total).toBe(24);
    expect(s.feeds_ok).toBe(22);
    expect(s.feeds_with_errors).toBe(2);
    expect(s.offending_feeds).toEqual(['Phoronix', 'LWN']);
    expect(s.entries_total).toBe(14_820);
    expect(s.entries_24h).toBe(1_402);
    expect(s.recent_errors).toHaveLength(1);
  });

  it('throws on non-2xx', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue(new Response('forbidden', { status: 403 }));
    await expect(getStatus()).rejects.toThrow(/403/);
  });
});
