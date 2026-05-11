import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, vi } from 'vitest';

const pollPref = { interval: '15m' as string };

vi.mock('../../../lib/preferences.svelte', () => ({
  poll: {
    get interval() { return pollPref.interval; },
    set interval(v: string) { pollPref.interval = v; },
  },
}));

vi.mock('../../../lib/api', () => ({
  api: {
    listSubscriptions: vi.fn(),
    refreshSubscription: vi.fn(),
    health: vi.fn(),
  },
}));

const { default: SyncingSection } = await import('../SyncingSection.svelte');
const { api } = await import('../../../lib/api');

describe('SyncingSection', () => {
  beforeEach(() => {
    pollPref.interval = '15m';
    vi.clearAllMocks();
  });

  it('shows the current poll-interval pref as the active segmented option', () => {
    pollPref.interval = '5m';
    const { getByRole } = render(SyncingSection);
    expect(getByRole('radio', { name: /^5m$/i })).toHaveAttribute('aria-checked', 'true');
  });

  it('clicking Refresh all now calls api.refreshSubscription once per feed (M5 mechanism)', async () => {
    vi.mocked(api.listSubscriptions).mockResolvedValue([
      { id: 1 } as never, { id: 2 } as never, { id: 3 } as never,
    ]);
    vi.mocked(api.refreshSubscription).mockResolvedValue(undefined as never);
    vi.mocked(api.health).mockResolvedValue({ polls_active: 0 } as never);
    const { getByRole } = render(SyncingSection);
    await fireEvent.click(getByRole('button', { name: /refresh all/i }));
    await waitFor(() => {
      expect(api.refreshSubscription).toHaveBeenCalledTimes(3);
      expect(api.refreshSubscription).toHaveBeenCalledWith(1);
      expect(api.refreshSubscription).toHaveBeenCalledWith(2);
      expect(api.refreshSubscription).toHaveBeenCalledWith(3);
    });
  });

  it('surfaces a partial-failure status when at least one feed PATCH rejects', async () => {
    vi.mocked(api.listSubscriptions).mockResolvedValue([
      { id: 1 } as never, { id: 2 } as never,
    ]);
    vi.mocked(api.refreshSubscription).mockImplementation((id: number) =>
      id === 1 ? Promise.resolve(undefined as never) : Promise.reject(new Error('500')),
    );
    vi.mocked(api.health).mockResolvedValue({ polls_active: 0 } as never);
    const { getByRole, findByRole } = render(SyncingSection);
    await fireEvent.click(getByRole('button', { name: /refresh all/i }));
    expect(await findByRole('alert')).toHaveTextContent(/1 of 2|1 failed/i);
    expect(api.refreshSubscription).toHaveBeenCalledTimes(2);
  });
});
