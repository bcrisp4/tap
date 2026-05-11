import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { pollStatus, startPollStatus, stopPollStatus } from '../pollStatus';

describe('pollStatus store', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal('fetch', vi.fn());
  });
  afterEach(() => {
    stopPollStatus();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('reports waking-up initially', () => {
    let snapshot: unknown;
    const off = pollStatus.subscribe(s => { snapshot = s; });
    expect(snapshot).toMatchObject({ active: null });
    off();
  });

  it('updates active count after a successful poll', async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ polls_active: 3 }),
    });
    startPollStatus();
    await vi.advanceTimersByTimeAsync(0);
    let snapshot: { active: number | null } = { active: null };
    const off = pollStatus.subscribe(s => { snapshot = s; });
    expect(snapshot.active).toBe(3);
    off();
  });

  it('leaves prior value alone on network error', async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce({ ok: true, json: async () => ({ polls_active: 2 }) })
      .mockRejectedValueOnce(new Error('offline'));
    startPollStatus();
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(30000);
    let snapshot: { active: number | null } = { active: null };
    const off = pollStatus.subscribe(s => { snapshot = s; });
    expect(snapshot.active).toBe(2);
    off();
  });
});
