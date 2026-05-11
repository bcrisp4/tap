import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import ConfirmDialog from './ConfirmDialog.svelte';

describe('ConfirmDialog', () => {
  it('renders title, body and CTA label', () => {
    render(ConfirmDialog, { props: { open: true, title: 'Delete alice?', body: 'This cannot be undone.', cta: 'Delete alice' } });
    expect(screen.getByText('Delete alice?')).toBeInTheDocument();
    expect(screen.getByText('This cannot be undone.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Delete alice' })).toBeInTheDocument();
  });

  it('confirm fires onConfirm', async () => {
    const onConfirm = vi.fn();
    render(ConfirmDialog, { props: { open: true, title: 't', body: 'b', cta: 'OK', onConfirm } });
    await fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    expect(onConfirm).toHaveBeenCalled();
  });

  it('cancel fires onCancel', async () => {
    const onCancel = vi.fn();
    render(ConfirmDialog, { props: { open: true, title: 't', body: 'b', cta: 'OK', onCancel } });
    await fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalled();
  });

  it('renders consequence list when supplied', () => {
    render(ConfirmDialog, { props: {
      open: true, title: 't', body: 'b', cta: 'OK',
      list: ['3 passkeys revoked', '42 feeds released', '2,148 records purged'],
    } });
    expect(screen.getByText('3 passkeys revoked')).toBeInTheDocument();
    expect(screen.getByText('42 feeds released')).toBeInTheDocument();
  });

  it('danger flag applies danger variant to CTA', () => {
    render(ConfirmDialog, { props: { open: true, title: 't', body: 'b', cta: 'Delete', danger: true } });
    const btn = screen.getByRole('button', { name: 'Delete' });
    expect(btn).toHaveAttribute('data-variant', 'danger');
  });
});
