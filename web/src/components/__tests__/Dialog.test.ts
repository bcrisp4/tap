import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import Dialog from '../Dialog.svelte';
import { createRawSnippet, tick } from 'svelte';

const body = createRawSnippet(() => ({ render: () => '<p>body</p>' }));
const twoButtons = createRawSnippet(() => ({
  render: () => '<div><button id="first">First</button><button id="second">Second</button></div>',
}));

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

  it('focuses first focusable element (close button) on open', async () => {
    const { container } = render(Dialog, {
      open: true, onClose: () => {}, title: 'Focus', children: twoButtons,
    });
    await tick();
    // The close button in the head is the first focusable element in the dialog.
    const closeBtn = container.querySelector('button[aria-label="Close"]') as HTMLElement;
    expect(document.activeElement).toBe(closeBtn);
  });

  it('wraps Tab from last focusable back to close button', async () => {
    const { container } = render(Dialog, {
      open: true, onClose: () => {}, title: 'Focus', children: twoButtons,
    });
    await tick();
    const second = container.querySelector('#second') as HTMLElement;
    second.focus();
    await fireEvent.keyDown(container.querySelector('.dialog') as HTMLElement, { key: 'Tab', shiftKey: false });
    // Wraps to first focusable = close button
    const closeBtn = container.querySelector('button[aria-label="Close"]') as HTMLElement;
    expect(document.activeElement).toBe(closeBtn);
  });

  it('wraps Shift+Tab from close button to last focusable', async () => {
    const { container } = render(Dialog, {
      open: true, onClose: () => {}, title: 'Focus', children: twoButtons,
    });
    await tick();
    const closeBtn = container.querySelector('button[aria-label="Close"]') as HTMLElement;
    closeBtn.focus();
    await fireEvent.keyDown(container.querySelector('.dialog') as HTMLElement, { key: 'Tab', shiftKey: true });
    // Wraps to last focusable = #second
    expect(document.activeElement).toBe(container.querySelector('#second'));
  });
});
