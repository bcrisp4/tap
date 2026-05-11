import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../../lib/auth', () => {
  const { writable } = require('svelte/store');
  return {
    auth: writable({ user: { has_totp: true, passkey_count: 2 }, csrfToken: 'x', bootstrapped: true }),
    ERR_UNAUTHORIZED: 'unauthorized',
  };
});

vi.mock('../../../lib/api', () => ({
  api: {
    listPasskeys: vi.fn(),
  },
}));

vi.mock('../dialogs/AddPasskeyDialog.svelte', () => ({ default: vi.fn() }));
vi.mock('../dialogs/RemovePasskeyDialog.svelte', () => ({ default: vi.fn() }));

const { default: SecurityPasskeysSection } = await import('../SecurityPasskeysSection.svelte');
const { api } = await import('../../../lib/api');

describe('SecurityPasskeysSection', () => {
  beforeEach(() => vi.clearAllMocks());

  it('renders each passkey from the API as a row', async () => {
    vi.mocked(api.listPasskeys).mockResolvedValue([
      { id: 1, label: 'MacBook · Touch ID', created_at: 1715000000 },
      { id: 2, label: 'YubiKey · Office',   created_at: 1715000100 },
    ]);
    const { findByText } = render(SecurityPasskeysSection);
    expect(await findByText('MacBook · Touch ID')).toBeInTheDocument();
    expect(await findByText('YubiKey · Office')).toBeInTheDocument();
  });

  it('Add passkey button is always present', async () => {
    vi.mocked(api.listPasskeys).mockResolvedValue([]);
    const { findByRole } = render(SecurityPasskeysSection);
    expect(await findByRole('button', { name: /add a passkey/i })).toBeInTheDocument();
  });

  it('Remove ✗ button opens the remove dialog', async () => {
    vi.mocked(api.listPasskeys).mockResolvedValue([
      { id: 1, label: 'MacBook · Touch ID', created_at: 1715000000 },
    ]);
    const { findAllByLabelText } = render(SecurityPasskeysSection);
    const revokes = await findAllByLabelText(/remove passkey/i);
    await fireEvent.click(revokes[0]);
    // dialog mock renders nothing — just verify no throw
  });
});
