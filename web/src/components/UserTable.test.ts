import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import UserTable from './UserTable.svelte';
import type { AdminUser } from '../lib/types';

const me: AdminUser = { id: 1, username: 'liz',   role: 'admin', created_at: 1_700_000_000, disabled_at: null,         has_totp: true,  passkey_count: 3 };
const alice: AdminUser = { id: 2, username: 'alice', role: 'user', created_at: 1_700_000_000, disabled_at: null,         has_totp: false, passkey_count: 0 };
const bob: AdminUser = { id: 3, username: 'bob',   role: 'user', created_at: 1_700_000_000, disabled_at: 1_700_500_000, has_totp: true,  passkey_count: 1 };

describe('UserTable', () => {
  it('renders one row per user with username + role + status', () => {
    render(UserTable, { props: { users: [me, alice, bob], currentUserId: 1 } });
    expect(screen.getByText('liz')).toBeInTheDocument();
    expect(screen.getByText('alice')).toBeInTheDocument();
    expect(screen.getByText('bob')).toBeInTheDocument();
  });

  it('shows "you" badge on current user row', () => {
    render(UserTable, { props: { users: [me, alice], currentUserId: 1 } });
    expect(screen.getByText('you')).toBeInTheDocument();
  });

  it('shows passkey count', () => {
    render(UserTable, { props: { users: [me], currentUserId: 1 } });
    expect(screen.getByTestId(`urow-pk-${me.id}`)).toHaveTextContent('3');
  });

  it('shows 2FA flag on/off', () => {
    render(UserTable, { props: { users: [me, alice], currentUserId: 1 } });
    expect(screen.getByTestId(`urow-totp-${me.id}`)).toHaveTextContent('enabled');
    expect(screen.getByTestId(`urow-totp-${alice.id}`)).toHaveTextContent('not set');
  });

  it('shows disabled pill on disabled row', () => {
    render(UserTable, { props: { users: [bob], currentUserId: 1 } });
    expect(screen.getByTestId(`urow-status-${bob.id}`)).toHaveTextContent('disabled');
  });

  it('disables "Delete" and "Disable" buttons for current user', () => {
    render(UserTable, { props: { users: [me], currentUserId: 1 } });
    expect(screen.getByTestId(`urow-action-delete-${me.id}`)).toBeDisabled();
    expect(screen.getByTestId(`urow-action-disableUser-${me.id}`)).toBeDisabled();
  });

  it('disables "Disable 2FA" when user has no TOTP', () => {
    render(UserTable, { props: { users: [alice], currentUserId: 1 } });
    expect(screen.getByTestId(`urow-action-disable2fa-${alice.id}`)).toBeDisabled();
  });

  it('shows "Re-enable" instead of "Disable" on disabled user', () => {
    render(UserTable, { props: { users: [bob], currentUserId: 1 } });
    expect(screen.getByTestId(`urow-action-enable-${bob.id}`)).toBeInTheDocument();
    expect(screen.queryByTestId(`urow-action-disableUser-${bob.id}`)).toBeNull();
  });

  it('fires onAction callback with action name and user', async () => {
    const onAction = vi.fn();
    render(UserTable, { props: { users: [alice], currentUserId: 1, onAction } });
    await fireEvent.click(screen.getByTestId(`urow-action-reset-${alice.id}`));
    expect(onAction).toHaveBeenCalledWith('reset', alice);
  });
});
