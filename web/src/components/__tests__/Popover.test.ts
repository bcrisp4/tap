import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import Popover from '../Popover.svelte';
import { createRawSnippet } from 'svelte';

const noop = createRawSnippet(() => ({ render: () => '<div>hello</div>' }));

describe('Popover', () => {
  it('renders children when open', () => {
    const { getByText } = render(Popover, { open: true, onClose: () => {}, children: noop });
    expect(getByText('hello')).toBeTruthy();
  });

  it('does not render when closed', () => {
    const { queryByText } = render(Popover, { open: false, onClose: () => {}, children: noop });
    expect(queryByText('hello')).toBeNull();
  });

  it('calls onClose on scrim click', async () => {
    const onClose = vi.fn();
    const { container } = render(Popover, { open: true, onClose, children: noop });
    const scrim = container.querySelector('.scrim') as HTMLElement;
    await fireEvent.click(scrim);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('calls onClose on Escape', async () => {
    const onClose = vi.fn();
    render(Popover, { open: true, onClose, children: noop });
    await fireEvent.keyDown(document, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledOnce();
  });
});
