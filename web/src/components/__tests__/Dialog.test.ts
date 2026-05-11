import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import Dialog from '../Dialog.svelte';
import { createRawSnippet } from 'svelte';

const body = createRawSnippet(() => ({ render: () => '<p>body</p>' }));

describe('Dialog', () => {
  it('renders title and body when open', () => {
    const { getByText } = render(Dialog, {
      open: true, onClose: () => {}, title: 'Confirm', children: body,
    });
    expect(getByText('Confirm')).toBeTruthy();
    expect(getByText('body')).toBeTruthy();
  });

  it('closes on Escape', async () => {
    const onClose = vi.fn();
    render(Dialog, { open: true, onClose, title: 'X', children: body });
    await fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('closes on scrim click', async () => {
    const onClose = vi.fn();
    const { container } = render(Dialog, { open: true, onClose, title: 'X', children: body });
    await fireEvent.click(container.querySelector('.scrim') as HTMLElement);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('closes on close button', async () => {
    const onClose = vi.fn();
    const { getByLabelText } = render(Dialog, { open: true, onClose, title: 'X', children: body });
    await fireEvent.click(getByLabelText('Close'));
    expect(onClose).toHaveBeenCalledOnce();
  });
});
