import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../../../lib/api', () => ({
  api: {
    changePassword: vi.fn(),
  },
}));

const { default: ChangePasswordDialog } = await import('../ChangePasswordDialog.svelte');
const { api } = await import('../../../../lib/api');

describe('ChangePasswordDialog', () => {
  beforeEach(() => vi.clearAllMocks());

  it('submits the form when current + new + confirm are filled and match', async () => {
    vi.mocked(api.changePassword).mockResolvedValue({ csrf_token: 'new' } as never);
    const onClose = vi.fn();
    const { getByLabelText, getByRole } = render(ChangePasswordDialog, { props: { onClose } });
    await fireEvent.input(getByLabelText(/current password/i), { target: { value: 'old' } });
    await fireEvent.input(getByLabelText(/^new password/i), { target: { value: 'newpass1234' } });
    await fireEvent.input(getByLabelText(/confirm/i), { target: { value: 'newpass1234' } });
    await fireEvent.click(getByRole('button', { name: /change password/i }));
    await waitFor(() => {
      expect(api.changePassword).toHaveBeenCalledWith('old', 'newpass1234');
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('shows a local mismatch error before calling the API', async () => {
    const { getByLabelText, getByRole, findByRole } = render(ChangePasswordDialog, { props: { onClose: vi.fn() } });
    await fireEvent.input(getByLabelText(/current password/i), { target: { value: 'old' } });
    await fireEvent.input(getByLabelText(/^new password/i), { target: { value: 'aaaaaaaa' } });
    await fireEvent.input(getByLabelText(/confirm/i), { target: { value: 'bbbbbbbb' } });
    await fireEvent.click(getByRole('button', { name: /change password/i }));
    expect(await findByRole('alert')).toHaveTextContent(/match/i);
    expect(api.changePassword).not.toHaveBeenCalled();
  });
});
