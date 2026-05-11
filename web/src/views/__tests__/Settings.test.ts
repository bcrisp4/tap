import { render } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';

vi.mock('../../lib/auth', () => {
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

vi.mock('../../lib/api', () => ({
  api: {
    listSessions: vi.fn().mockResolvedValue([]),
    listPasskeys: vi.fn().mockResolvedValue([]),
    listSubscriptions: vi.fn().mockResolvedValue([]),
    refreshSubscription: vi.fn().mockResolvedValue(undefined),
    health: vi.fn().mockResolvedValue({ polls_active: 0 }),
    revokeAllOtherSessions: vi.fn().mockResolvedValue(undefined),
  },
}));

vi.mock('../../lib/preferences.svelte', () => ({
  theme: { get stored() { return 'system'; }, set stored(_v: string) {} },
  font: { get value() { return 'serif'; }, set value(_v: string) {} },
  density: { get value() { return 'comfortable'; }, set value(_v: string) {} },
  measure: { get value() { return 'comfortable'; }, set value(_v: string) {} },
  reading: {
    get markOnScroll() { return true; }, set markOnScroll(_v: boolean) {},
    get autoOpenNext() { return false; }, set autoOpenNext(_v: boolean) {},
    get showSummaries() { return true; }, set showSummaries(_v: boolean) {},
    get openLinksNewTab() { return true; }, set openLinksNewTab(_v: boolean) {},
  },
  poll: { get interval() { return '15m'; }, set interval(_v: string) {} },
}));

vi.mock('../../lib/router', () => ({ navigate: vi.fn() }));

const { default: Settings } = await import('../Settings.svelte');

describe('Settings page', () => {
  it('renders the seven numbered eyebrows', () => {
    const { getByText } = render(Settings);
    expect(getByText('Appearance')).toBeInTheDocument();
    expect(getByText('Reading')).toBeInTheDocument();
    expect(getByText('Syncing')).toBeInTheDocument();
    expect(getByText('Account')).toBeInTheDocument();
    // Security · Two-factor and Security · Passkeys both contain "Security"
    expect(getByText(/Security.*Two-factor/i)).toBeInTheDocument();
    expect(getByText('Sessions')).toBeInTheDocument();
    expect(getByText('Data')).toBeInTheDocument();
  });

  it('renders the page-id strip with the user email', () => {
    const { getAllByText } = render(Settings);
    expect(getAllByText('liz@hauck.studio').length).toBeGreaterThan(0);
  });
});
