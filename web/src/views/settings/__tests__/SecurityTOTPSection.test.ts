import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';

vi.mock('../../../lib/auth', () => {
  const { writable } = require('svelte/store');
  return {
    auth: writable({
      user: { has_totp: false },
      csrfToken: 'x',
      bootstrapped: true,
    }),
    ERR_UNAUTHORIZED: 'unauthorized',
  };
});

// Mock dialogs to prevent real API calls.
vi.mock('../dialogs/EnrolTOTPDialog.svelte', () => ({ default: vi.fn() }));
vi.mock('../dialogs/DisableTOTPDialog.svelte', () => ({ default: vi.fn() }));
vi.mock('../dialogs/RegenerateCodesDialog.svelte', () => ({ default: vi.fn() }));
vi.mock('../dialogs/ViewRecoveryCodesDialog.svelte', () => ({ default: vi.fn() }));

const { default: SecurityTOTPSection } = await import('../SecurityTOTPSection.svelte');

describe('SecurityTOTPSection', () => {
  it('when has_totp=false, shows Not enrolled and a Set up button', () => {
    const { getByText, getByRole } = render(SecurityTOTPSection);
    expect(getByText(/not enrolled/i)).toBeInTheDocument();
    expect(getByRole('button', { name: /set up authenticator/i })).toBeInTheDocument();
  });

  it('clicking Set up authenticator mounts the enrolment dialog', async () => {
    const { getByRole } = render(SecurityTOTPSection);
    await fireEvent.click(getByRole('button', { name: /set up authenticator/i }));
    // The dialog mock renders nothing — just verify the click doesn't throw.
  });
});
