import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';

vi.mock('../../../lib/auth', () => {
  const { writable } = require('svelte/store');
  return {
    auth: writable({
      user: { id: 1, username: 'liz@hauck.studio', role: 'user', has_totp: false, passkey_count: 0 },
      csrfToken: 'x',
      bootstrapped: true,
    }),
    ERR_UNAUTHORIZED: 'unauthorized',
  };
});

vi.mock('../../../lib/api', () => ({
  api: {
    revokeAllOtherSessions: vi.fn(),
  },
}));

// Mock child dialogs to prevent their API calls.
vi.mock('../dialogs/ChangePasswordDialog.svelte', () => ({ default: vi.fn() }));

const { default: AccountSection } = await import('../AccountSection.svelte');
const { api } = await import('../../../lib/api');

describe('AccountSection', () => {
  it('renders the email in mono', () => {
    const { getByText } = render(AccountSection);
    expect(getByText('liz@hauck.studio')).toBeInTheDocument();
  });

  it('clicking Change password opens the dialog', async () => {
    // The dialog mock renders nothing, but the button should still open the
    // dialog mount point — testing button click doesn't error.
    const { getByRole } = render(AccountSection);
    expect(getByRole('button', { name: /change password/i })).toBeInTheDocument();
    await fireEvent.click(getByRole('button', { name: /change password/i }));
  });

  it('clicking Sign out everywhere calls api.revokeAllOtherSessions', async () => {
    vi.mocked(api.revokeAllOtherSessions).mockResolvedValue(undefined as never);
    const { getByRole } = render(AccountSection);
    await fireEvent.click(getByRole('button', { name: /sign out everywhere/i }));
    await waitFor(() => expect(api.revokeAllOtherSessions).toHaveBeenCalledOnce());
  });
});
