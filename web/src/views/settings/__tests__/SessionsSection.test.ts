import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../../lib/api', () => ({
  api: {
    listSessions: vi.fn(),
    revokeSession: vi.fn(),
  },
}));

const { default: SessionsSection } = await import('../SessionsSection.svelte');
const { api } = await import('../../../lib/api');

describe('SessionsSection', () => {
  beforeEach(() => vi.clearAllMocks());

  it('renders one row per session', async () => {
    vi.mocked(api.listSessions).mockResolvedValue([
      { id: 1, created_at: 1, last_seen_at: 1, idle_expires_at: 1, user_agent: 'Firefox', address: '1.2.3.4', current: true },
      { id: 2, created_at: 1, last_seen_at: 1, idle_expires_at: 1, user_agent: 'Safari/iOS', address: '5.6.7.8', current: false },
    ] as never);
    const { findByText } = render(SessionsSection);
    expect(await findByText('Firefox')).toBeInTheDocument();
    expect(await findByText('Safari/iOS')).toBeInTheDocument();
  });

  it('current session row has revoke disabled', async () => {
    vi.mocked(api.listSessions).mockResolvedValue([
      { id: 1, created_at: 1, last_seen_at: 1, idle_expires_at: 1, user_agent: 'Firefox', address: '1.2.3.4', current: true },
    ] as never);
    const { findByLabelText } = render(SessionsSection);
    const btn = await findByLabelText(/cannot revoke current session/i);
    expect(btn).toBeDisabled();
  });

  it('clicking revoke removes the row optimistically and calls api.revokeSession', async () => {
    vi.mocked(api.listSessions).mockResolvedValue([
      { id: 1, created_at: 1, last_seen_at: 1, idle_expires_at: 1, user_agent: 'Firefox', address: '1.2.3.4', current: true },
      { id: 2, created_at: 1, last_seen_at: 1, idle_expires_at: 1, user_agent: 'Safari/iOS', address: '5.6.7.8', current: false },
    ] as never);
    vi.mocked(api.revokeSession).mockResolvedValue(undefined as never);
    const { findByLabelText, queryByText } = render(SessionsSection);
    const revokeBtn = await findByLabelText(/revoke session Safari/i);
    await fireEvent.click(revokeBtn);
    await waitFor(() => {
      expect(api.revokeSession).toHaveBeenCalledWith(2);
      expect(queryByText('Safari/iOS')).not.toBeInTheDocument();
    });
  });
});
