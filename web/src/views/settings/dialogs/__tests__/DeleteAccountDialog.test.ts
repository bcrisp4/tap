import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../../../lib/auth', () => ({
  auth: { logout: vi.fn().mockResolvedValue(undefined) },
  ERR_UNAUTHORIZED: 'unauthorized',
}));
vi.mock('../../../../lib/router', () => ({ navigate: vi.fn() }));
vi.mock('../../../../lib/api', () => ({
  api: { deleteAccount: vi.fn() },
}));

const { default: DeleteAccountDialog } = await import('../DeleteAccountDialog.svelte');
const { api } = await import('../../../../lib/api');
const { auth } = await import('../../../../lib/auth');
const { navigate } = await import('../../../../lib/router');

describe('DeleteAccountDialog', () => {
  beforeEach(() => vi.clearAllMocks());

  it('submits with current_password and on success calls auth.logout + navigate(/sign-in)', async () => {
    vi.mocked(api.deleteAccount).mockResolvedValue(undefined as never);
    const { getByLabelText, getByRole } = render(DeleteAccountDialog, { props: { onClose: vi.fn() } });
    await fireEvent.input(getByLabelText(/password/i), { target: { value: 'pw' } });
    await fireEvent.click(getByRole('button', { name: /delete my account/i }));
    await waitFor(() => {
      expect(api.deleteAccount).toHaveBeenCalledWith('pw');
      expect(auth.logout).toHaveBeenCalled();
      expect(navigate).toHaveBeenCalledWith('/sign-in');
    });
  });

  it('shows error on failure and does not log out', async () => {
    vi.mocked(api.deleteAccount).mockRejectedValue(new Error('invalid credentials'));
    const { getByLabelText, getByRole, findByRole } = render(DeleteAccountDialog, { props: { onClose: vi.fn() } });
    await fireEvent.input(getByLabelText(/password/i), { target: { value: 'bad' } });
    await fireEvent.click(getByRole('button', { name: /delete my account/i }));
    expect(await findByRole('alert')).toHaveTextContent(/invalid/i);
    expect(auth.logout).not.toHaveBeenCalled();
  });
});
