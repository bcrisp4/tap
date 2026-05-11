import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import ResetPasswordResultDialog from './ResetPasswordResultDialog.svelte';

describe('ResetPasswordResultDialog', () => {
  it('displays the temporary password once', () => {
    render(ResetPasswordResultDialog, { props: { open: true, username: 'alice', password: 'amber-loop-shadow-2840' } });
    expect(screen.getByText('amber-loop-shadow-2840')).toBeInTheDocument();
    expect(screen.getAllByText(/alice/).length).toBeGreaterThan(0);
  });

  it('copy button calls navigator.clipboard.writeText with the password', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    render(ResetPasswordResultDialog, { props: { open: true, username: 'alice', password: 'pw-123' } });
    await fireEvent.click(screen.getByRole('button', { name: /copy/i }));
    expect(writeText).toHaveBeenCalledWith('pw-123');
  });

  it('Done button fires onClose', async () => {
    const onClose = vi.fn();
    render(ResetPasswordResultDialog, { props: { open: true, username: 'alice', password: 'pw-123', onClose } });
    await fireEvent.click(screen.getByRole('button', { name: /done/i }));
    expect(onClose).toHaveBeenCalled();
  });

  it('does not render a download button', () => {
    render(ResetPasswordResultDialog, { props: { open: true, username: 'alice', password: 'pw-123' } });
    expect(screen.queryByRole('button', { name: /download/i })).toBeNull();
  });
});
