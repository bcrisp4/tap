import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import CreateUserDialog from './CreateUserDialog.svelte';

describe('CreateUserDialog', () => {
  it('opens with default role = user', () => {
    render(CreateUserDialog, { props: { open: true } });
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/initial password/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'User' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('regenerate-password button produces a word-number passphrase', async () => {
    render(CreateUserDialog, { props: { open: true } });
    const pwInput = screen.getByLabelText(/initial password/i) as HTMLInputElement;
    await fireEvent.click(screen.getByRole('button', { name: /regenerate password/i }));
    // word-number format: three lowercase words separated by hyphens + a 4-digit number
    expect(pwInput.value).toMatch(/^[a-z]+-[a-z]+-[a-z]+-\d{4}$/);
  });

  it('submit fires onSubmit with username, password, role', async () => {
    const onSubmit = vi.fn();
    render(CreateUserDialog, { props: { open: true, onSubmit } });
    await fireEvent.input(screen.getByLabelText(/username/i), { target: { value: 'newbie' } });
    await fireEvent.click(screen.getByRole('button', { name: 'Admin' }));
    await fireEvent.click(screen.getByRole('button', { name: /create user/i }));
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ username: 'newbie', role: 'admin' }));
    expect(onSubmit.mock.calls[0][0].password.length).toBeGreaterThanOrEqual(10);
  });

  it('submit blocked when username empty', async () => {
    const onSubmit = vi.fn();
    render(CreateUserDialog, { props: { open: true, onSubmit } });
    await fireEvent.click(screen.getByRole('button', { name: /create user/i }));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('cancel fires onClose', async () => {
    const onClose = vi.fn();
    render(CreateUserDialog, { props: { open: true, onClose } });
    await fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onClose).toHaveBeenCalled();
  });
});
