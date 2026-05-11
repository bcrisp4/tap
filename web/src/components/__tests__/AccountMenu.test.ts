import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import AccountMenu from '../AccountMenu.svelte';

describe('AccountMenu', () => {
  const user = { id: 1, username: 'ada', role: 'user' as const, has_totp: false, passkey_count: 0 };

  it('shows when open', () => {
    const { getByText } = render(AccountMenu, { open: true, user, onClose: () => {}, onLogout: () => {} });
    expect(getByText('ada')).toBeTruthy();
    expect(getByText('Log out')).toBeTruthy();
  });

  it('hidden when closed', () => {
    const { queryByText } = render(AccountMenu, { open: false, user, onClose: () => {}, onLogout: () => {} });
    expect(queryByText('Log out')).toBeNull();
  });

  it('fires onLogout', async () => {
    const onLogout = vi.fn();
    const { getByText } = render(AccountMenu, { open: true, user, onClose: () => {}, onLogout });
    await fireEvent.click(getByText('Log out'));
    expect(onLogout).toHaveBeenCalledOnce();
  });
});
