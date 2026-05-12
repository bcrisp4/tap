import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../../lib/auth', () => {
  const { writable } = require('svelte/store');
  return {
    auth: writable({
      user: { has_totp: false, passkey_count: 0 },
      csrfToken: 'x',
      bootstrapped: true,
    }),
    ERR_UNAUTHORIZED: 'unauthorized',
  };
});

vi.mock('../../../lib/api', () => ({
  api: { listPasskeys: vi.fn() },
}));

vi.mock('../dialogs/EnrolTOTPDialog.svelte', () => ({ default: vi.fn() }));
vi.mock('../dialogs/DisableTOTPDialog.svelte', () => ({ default: vi.fn() }));
vi.mock('../dialogs/RegenerateCodesDialog.svelte', () => ({ default: vi.fn() }));
vi.mock('../dialogs/ViewRecoveryCodesDialog.svelte', () => ({ default: vi.fn() }));
vi.mock('../dialogs/AddPasskeyDialog.svelte', () => ({ default: vi.fn() }));
vi.mock('../dialogs/RemovePasskeyDialog.svelte', () => ({ default: vi.fn() }));

const { default: SecuritySection } = await import('../SecuritySection.svelte');
const { api } = await import('../../../lib/api');

describe('SecuritySection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.listPasskeys).mockResolvedValue([]);
    // Reset PublicKeyCredential to available by default
    Object.defineProperty(window, 'PublicKeyCredential', {
      configurable: true,
      value: function PublicKeyCredential() {},
    });
    Object.defineProperty(window.navigator, 'credentials', {
      configurable: true,
      value: { get: vi.fn(), create: vi.fn() },
    });
  });

  it('renders all three subgroup headings', async () => {
    const { getByText, getAllByText } = render(SecuritySection);
    expect(getByText('Security')).toBeInTheDocument();
    expect(getByText(/Two-factor authentication/i)).toBeInTheDocument();
    expect(getAllByText(/Passkeys/i).length).toBeGreaterThan(0);
  });

  it('shows Not enrolled + Set up button when has_totp=false', () => {
    const { getByText, getByRole } = render(SecuritySection);
    expect(getByText(/not enrolled/i)).toBeInTheDocument();
    expect(getByRole('button', { name: /set up authenticator/i })).toBeInTheDocument();
  });

  it('clicking Set up authenticator mounts the enrolment dialog without throwing', async () => {
    const { getByRole } = render(SecuritySection);
    await fireEvent.click(getByRole('button', { name: /set up authenticator/i }));
  });

  it('renders each passkey from the API', async () => {
    vi.mocked(api.listPasskeys).mockResolvedValue([
      { id: 1, label: 'MacBook · Touch ID', created_at: 1715000000 },
      { id: 2, label: 'YubiKey · Office',   created_at: 1715000100 },
    ]);
    const { findByText } = render(SecuritySection);
    expect(await findByText('MacBook · Touch ID')).toBeInTheDocument();
    expect(await findByText('YubiKey · Office')).toBeInTheDocument();
  });

  it('shows "Add a passkey" button when WebAuthn is supported', async () => {
    const { findByRole } = render(SecuritySection);
    expect(await findByRole('button', { name: /add a passkey/i })).toBeInTheDocument();
  });

  it('hides "Add a passkey" button and shows unsupported message when WebAuthn is absent', async () => {
    Object.defineProperty(window, 'PublicKeyCredential', { configurable: true, value: undefined });
    const { findByText, queryByRole } = render(SecuritySection);
    expect(await findByText(/your browser doesn't support passkeys/i)).toBeInTheDocument();
    expect(queryByRole('button', { name: /add a passkey/i })).toBeNull();
  });

  it('Remove button opens the remove dialog without throwing', async () => {
    vi.mocked(api.listPasskeys).mockResolvedValue([
      { id: 1, label: 'MacBook · Touch ID', created_at: 1715000000 },
    ]);
    const { findAllByLabelText } = render(SecuritySection);
    const revokes = await findAllByLabelText(/remove passkey/i);
    await fireEvent.click(revokes[0]);
  });

  it('Regenerate recovery codes button available when TOTP is enabled', async () => {
    const authMod = await import('../../../lib/auth');
    // The mock returns a writable store — cast to access .set for test state mutation.
    (authMod.auth as unknown as { set: (v: unknown) => void }).set({
      user: { has_totp: true, passkey_count: 0 },
      csrfToken: 'x',
      bootstrapped: true,
    });
    const { getByRole } = render(SecuritySection);
    expect(getByRole('button', { name: /regenerate recovery codes/i })).toBeInTheDocument();
  });
});
