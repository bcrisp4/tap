import { describe, it, expect, vi, afterEach } from 'vitest';
import { render } from '@testing-library/svelte';
import PollerStatus from '../PollerStatus.svelte';

describe('PollerStatus', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders polls_active from /healthz', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ status: 'ok', polls_active: 3, uptime_seconds: 100 })),
    );
    const { findByText, unmount } = render(PollerStatus);
    expect(await findByText(/3 active/)).toBeTruthy();
    expect(fetchSpy).toHaveBeenCalledWith('/healthz');
    unmount();
  });

  it('shows idle when polls_active is 0', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ status: 'ok', polls_active: 0, uptime_seconds: 50 })),
    );
    const { findByText, unmount } = render(PollerStatus);
    expect(await findByText(/idle/i)).toBeTruthy();
    unmount();
  });

  it('shows waking up before healthz resolves', async () => {
    vi.spyOn(globalThis, 'fetch').mockReturnValue(new Promise(() => {}));
    const { getByText, unmount } = render(PollerStatus);
    expect(getByText(/waking up/i)).toBeTruthy();
    unmount();
  });
});
