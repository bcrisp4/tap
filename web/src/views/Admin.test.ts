import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { writable } from 'svelte/store';
import Admin from './Admin.svelte';
import { api } from '../lib/api';
import { getStatus } from '../lib/status';

const navigate = vi.fn();
vi.mock('../lib/router', () => ({ navigate: (...a: unknown[]) => navigate(...a) }));

const _authStore = writable<{ user: { id: number; username: string; role: 'admin' | 'user' } } | null>({
  user: { id: 1, username: 'liz', role: 'admin' },
});
vi.mock('../lib/auth', () => {
  const { writable } = require('svelte/store');
  const store = writable({ user: { id: 1, username: 'liz', role: 'admin' } });
  return { auth: store };
});

vi.mock('../lib/api', () => ({
  api: {
    listUsers: vi.fn().mockResolvedValue([]),
    createUser: vi.fn(),
    patchUser: vi.fn(),
    deleteUser: vi.fn(),
    resetUserPassword: vi.fn(),
    disableUserTOTP: vi.fn(),
  },
}));

vi.mock('../lib/status', () => ({
  getStatus: vi.fn().mockResolvedValue({
    version: 'v1', uptime_seconds: 0, db: 'ok',
    polls_active: 0, polls_total: 0, last_poll_at: null,
    recent_errors: [],
    feeds_total: 0, feeds_ok: 0, feeds_with_errors: 0, offending_feeds: [],
    entries_total: 0, entries_24h: 0,
  }),
}));

// Import the mocked auth store after mocking
import { auth as authStore } from '../lib/auth';

describe('Admin (access denied)', () => {
  beforeEach(() => { navigate.mockReset(); });
  it('non-admin sees access-denied and redirects to /', async () => {
    (authStore as ReturnType<typeof writable>).set({ user: { id: 2, username: 'alice', role: 'user' } });
    render(Admin, {});
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/'));
    expect(screen.getByText(/access denied/i)).toBeInTheDocument();
  });
});

describe('Admin (admin role)', () => {
  beforeEach(() => {
    (authStore as ReturnType<typeof writable>).set({ user: { id: 1, username: 'liz', role: 'admin' } });
    (api.listUsers as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([
      { id: 1, username: 'liz',   role: 'admin', created_at: 1_700_000_000, disabled_at: null, has_totp: true,  passkey_count: 3 },
      { id: 2, username: 'alice', role: 'user',  created_at: 1_700_000_000, disabled_at: null, has_totp: false, passkey_count: 0 },
    ]);
    (getStatus as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue({
      version: 'v1', uptime_seconds: 0, db: 'ok',
      polls_active: 0, polls_total: 0, last_poll_at: 1_700_000_000,
      recent_errors: [
        { time: '2026-05-11T10:42:14Z', level: 'error', event: 'phoronix.com 502', attrs: {} },
      ],
      feeds_total: 12, feeds_ok: 11, feeds_with_errors: 1, offending_feeds: ['Phoronix'],
      entries_total: 1500, entries_24h: 80,
    });
  });

  it('renders SysGrid + ErrorsTable + UserTable after load', async () => {
    render(Admin, {});
    await waitFor(() => expect(screen.getByText('FEEDS')).toBeInTheDocument());
    await waitFor(() => expect(screen.getByTestId('urow-pk-1')).toBeInTheDocument());
    expect(screen.getByTestId('urow-pk-2')).toBeInTheDocument();
    expect(screen.getByText('Phoronix')).toBeInTheDocument();
    expect(screen.getByText('phoronix.com 502')).toBeInTheDocument();
  });

  it('filters users by search query', async () => {
    render(Admin, {});
    await waitFor(() => expect(screen.getByTestId('urow-pk-1')).toBeInTheDocument());
    await fireEvent.input(screen.getByPlaceholderText('Filter by username'), { target: { value: 'ali' } });
    expect(screen.queryByTestId('urow-pk-1')).toBeNull();
    expect(screen.getByTestId('urow-pk-2')).toBeInTheDocument();
  });

  it('filters users by chip (Admins)', async () => {
    render(Admin, {});
    await waitFor(() => expect(screen.getByTestId('urow-pk-1')).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: 'Admins' }));
    expect(screen.getByTestId('urow-pk-1')).toBeInTheDocument();
    expect(screen.queryByTestId('urow-pk-2')).toBeNull();
  });
});

describe('Admin (mutations)', () => {
  beforeEach(() => {
    (authStore as ReturnType<typeof writable>).set({ user: { id: 1, username: 'liz', role: 'admin' } });
    (api.listUsers as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([
      { id: 1, username: 'liz',   role: 'admin', created_at: 1_700_000_000, disabled_at: null, has_totp: true,  passkey_count: 0 },
      { id: 2, username: 'alice', role: 'user',  created_at: 1_700_000_000, disabled_at: null, has_totp: false, passkey_count: 0 },
    ]);
    (api.createUser as ReturnType<typeof vi.fn>).mockReset();
    (api.deleteUser as ReturnType<typeof vi.fn>).mockReset();
    (api.resetUserPassword as ReturnType<typeof vi.fn>).mockReset();
    (api.disableUserTOTP as ReturnType<typeof vi.fn>).mockReset();
    (api.patchUser as ReturnType<typeof vi.fn>).mockReset();
    (getStatus as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue({
      version: 'v1', uptime_seconds: 0, db: 'ok',
      polls_active: 0, polls_total: 0, last_poll_at: null,
      recent_errors: [],
      feeds_total: 0, feeds_ok: 0, feeds_with_errors: 0, offending_feeds: [],
      entries_total: 0, entries_24h: 0,
    });
  });

  it('Create user dialog → submit → API call → row appended', async () => {
    (api.createUser as ReturnType<typeof vi.fn>).mockResolvedValue({
      id: 3, username: 'newbie', role: 'user', created_at: 1_700_000_100, disabled_at: null, has_totp: false, passkey_count: 0,
    });
    render(Admin, {});
    await waitFor(() => expect(screen.getByTestId('urow-pk-1')).toBeInTheDocument());
    // click the toolbar "Create user" (first one on page)
    await fireEvent.click(screen.getAllByRole('button', { name: /create user/i })[0]);
    await waitFor(() => expect(screen.getByLabelText(/username/i)).toBeInTheDocument());
    await fireEvent.input(screen.getByLabelText(/username/i), { target: { value: 'newbie' } });
    // click the dialog submit "Create user" button (last one)
    const createBtns = screen.getAllByRole('button', { name: /create user/i });
    await fireEvent.click(createBtns[createBtns.length - 1]);
    await waitFor(() => expect(api.createUser).toHaveBeenCalled());
    await waitFor(() => expect(screen.getByTestId('urow-pk-3')).toBeInTheDocument());
  });

  it('Reset password → confirm → result dialog shows temp password', async () => {
    (api.resetUserPassword as ReturnType<typeof vi.fn>).mockResolvedValue({ temporary_password: 'one-shot-pw' });
    render(Admin, {});
    await waitFor(() => expect(screen.getByTestId('urow-pk-2')).toBeInTheDocument());
    await fireEvent.click(screen.getByTestId('urow-action-reset-2'));
    // find the ConfirmDialog CTA button specifically (has data-variant attribute)
    await waitFor(() => expect(document.querySelector('[data-variant="primary"]')).toBeTruthy());
    await fireEvent.click(document.querySelector('[data-variant="primary"]') as Element);
    await waitFor(() => expect(screen.getByText('one-shot-pw')).toBeInTheDocument());
  });

  it('Delete user → confirm → row removed', async () => {
    (api.deleteUser as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    render(Admin, {});
    await waitFor(() => expect(screen.getByTestId('urow-pk-2')).toBeInTheDocument());
    await fireEvent.click(screen.getByTestId('urow-action-delete-2'));
    await waitFor(() => expect(screen.getByRole('button', { name: /^Delete alice$/i })).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /^Delete alice$/i }));
    await waitFor(() => expect(api.deleteUser).toHaveBeenCalledWith(2));
    await waitFor(() => expect(screen.queryByTestId('urow-pk-2')).toBeNull());
  });

  it('Disable 2FA → confirm → API call', async () => {
    (api.listUsers as ReturnType<typeof vi.fn>).mockResolvedValue([
      { id: 1, username: 'liz',   role: 'admin', created_at: 1_700_000_000, disabled_at: null, has_totp: true, passkey_count: 0 },
      { id: 2, username: 'alice', role: 'user',  created_at: 1_700_000_000, disabled_at: null, has_totp: true, passkey_count: 0 },
    ]);
    (api.disableUserTOTP as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    render(Admin, {});
    await waitFor(() => expect(screen.getByTestId('urow-pk-2')).toBeInTheDocument());
    await fireEvent.click(screen.getByTestId('urow-action-disable2fa-2'));
    await waitFor(() => expect(screen.getByRole('button', { name: /disable totp/i })).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /disable totp/i }));
    await waitFor(() => expect(api.disableUserTOTP).toHaveBeenCalledWith(2));
  });

  it('Disable user → patchUser({disabled:true})', async () => {
    (api.patchUser as ReturnType<typeof vi.fn>).mockResolvedValue({
      id: 2, username: 'alice', role: 'user', created_at: 1_700_000_000, disabled_at: 1_700_500_000, has_totp: false, passkey_count: 0,
    });
    render(Admin, {});
    await waitFor(() => expect(screen.getByTestId('urow-pk-2')).toBeInTheDocument());
    await fireEvent.click(screen.getByTestId('urow-action-disableUser-2'));
    await waitFor(() => expect(screen.getByRole('button', { name: /disable account/i })).toBeInTheDocument());
    await fireEvent.click(screen.getByRole('button', { name: /disable account/i }));
    await waitFor(() => expect(api.patchUser).toHaveBeenCalledWith(2, { disabled: true }));
  });
});

describe('Admin (status polling)', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    (authStore as ReturnType<typeof writable>).set({ user: { id: 1, username: 'liz', role: 'admin' } });
    (api.listUsers as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([]);
    (getStatus as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue({
      version: 'v1', uptime_seconds: 0, db: 'ok',
      polls_active: 0, polls_total: 0, last_poll_at: null,
      recent_errors: [],
      feeds_total: 1, feeds_ok: 1, feeds_with_errors: 0, offending_feeds: [],
      entries_total: 0, entries_24h: 0,
    });
  });
  afterEach(() => vi.useRealTimers());

  it('polls status every 60s and stops on unmount', async () => {
    const { unmount } = render(Admin, {});
    await vi.runOnlyPendingTimersAsync();
    const callsAfterMount = (getStatus as ReturnType<typeof vi.fn>).mock.calls.length;
    expect(callsAfterMount).toBeGreaterThanOrEqual(1);
    vi.advanceTimersByTime(60_000);
    await vi.runOnlyPendingTimersAsync();
    const callsAfterInterval = (getStatus as ReturnType<typeof vi.fn>).mock.calls.length;
    expect(callsAfterInterval).toBeGreaterThan(callsAfterMount);
    unmount();
    vi.advanceTimersByTime(60_000);
    await vi.runOnlyPendingTimersAsync();
    expect((getStatus as ReturnType<typeof vi.fn>).mock.calls.length).toBe(callsAfterInterval);
  });
});
