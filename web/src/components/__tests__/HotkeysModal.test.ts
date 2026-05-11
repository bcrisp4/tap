import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import { userEvent } from '@testing-library/user-event';
import HotkeysModal from '../HotkeysModal.svelte';

// jsdom doesn't implement showModal/close on <dialog>, so we stub them.
beforeEach(() => {
  HTMLDialogElement.prototype.showModal = vi.fn();
  HTMLDialogElement.prototype.close = vi.fn();
});

describe('HotkeysModal', () => {
  it('renders a native <dialog> element', () => {
    const { container } = render(HotkeysModal, { props: { open: true, onClose: () => {} } });
    const dlg = container.querySelector('dialog');
    expect(dlg).toBeTruthy();
    expect(dlg!.tagName).toBe('DIALOG');
    expect(screen.getByText('Keyboard shortcuts')).toBeTruthy();
  });

  it('calls onClose when the close button is clicked', async () => {
    const user = userEvent.setup();
    let closed = false;
    const { container } = render(HotkeysModal, { props: { open: true, onClose: () => { closed = true; } } });
    const closeBtn = container.querySelector('button[aria-label="Close"]') as HTMLButtonElement;
    await user.click(closeBtn);
    expect(closed).toBe(true);
  });

  it('shows all expected shortcut rows', () => {
    render(HotkeysModal, { props: { open: true, onClose: () => {} } });
    expect(screen.getByText('Next entry')).toBeTruthy();
    expect(screen.getByText('Toggle read')).toBeTruthy();
    expect(screen.getByText('This help')).toBeTruthy();
  });

  it('renders three groups (Navigation, Actions, App)', () => {
    render(HotkeysModal, { props: { open: true, onClose: () => {} } });
    expect(screen.getByText('Navigation')).toBeTruthy();
    expect(screen.getByText('Actions')).toBeTruthy();
    expect(screen.getByText('App')).toBeTruthy();
  });
});
