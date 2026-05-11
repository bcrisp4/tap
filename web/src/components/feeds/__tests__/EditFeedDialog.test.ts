import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../../../lib/api', () => ({
  api: { updateSubscription: vi.fn() },
}));

vi.mock('../../../lib/auth', () => ({
  auth: { subscribe: vi.fn(() => () => {}), clearOn401: vi.fn(), setCSRFToken: vi.fn() },
  notifySW: vi.fn(),
  ERR_UNAUTHORIZED: 'unauthorized',
}));

vi.mock('../../../lib/offlineQueue', () => ({
  offlineQueue: { enqueue: vi.fn(), drain: vi.fn(), clearForUser: vi.fn() },
}));

import EditFeedDialog from '../EditFeedDialog.svelte';
import { api } from '../../../lib/api';
import type { Subscription, Category } from '../../../lib/types';

const noop = () => {};
const feed: Subscription = {
  id: 1, title: 'Julia Evans', feed_url: 'https://jvns.ca/atom.xml', site_url: 'https://jvns.ca',
  next_poll_at: 0, last_poll_at: 1000, error_count: 0, created_at: 0,
  extract: false, extract_selector: '', has_cookie: false, has_basic_auth: false, category_id: 3,
};
const categories: Category[] = [{ id: 3, name: 'People', unread: 0, created_at: 0, position: 0 }];

beforeEach(() => { vi.clearAllMocks(); });

describe('EditFeedDialog — basics', () => {
  it('renders Title + Feed URL (readonly) + Category list', () => {
    render(EditFeedDialog, { props: { feed, categories, onClose: noop, onSaved: noop, onDelete: noop } });
    expect(screen.getByDisplayValue(feed.title)).toBeInTheDocument();
    expect((screen.getByDisplayValue(feed.feed_url) as HTMLInputElement).readOnly).toBe(true);
    expect(screen.getByRole('button', { name: /people/i })).toBeInTheDocument();
  });
});

describe('EditFeedDialog — extract toggle', () => {
  it('extract toggle starts in feed.extract state and flips on click', async () => {
    const { container } = render(EditFeedDialog, { props: { feed: { ...feed, extract: false }, categories, onClose: noop, onSaved: noop, onDelete: noop } });
    const toggle = container.querySelector('.ts-feeds-edit-toggle')!;
    expect(toggle.classList.contains('is-on')).toBe(false);
    await fireEvent.click(toggle);
    expect(toggle.classList.contains('is-on')).toBe(true);
  });

  it('selector input is disabled when extract is off', async () => {
    const { container } = render(EditFeedDialog, { props: { feed: { ...feed, extract: false }, categories, onClose: noop, onSaved: noop, onDelete: noop } });
    const selectorInput = container.querySelector('input[placeholder*="article"]') as HTMLInputElement;
    expect(selectorInput.disabled).toBe(true);
    await fireEvent.click(container.querySelector('.ts-feeds-edit-toggle')!);
    expect(selectorInput.disabled).toBe(false);
  });
});

describe('EditFeedDialog — credentials', () => {
  it('cookie field is empty on open even if has_cookie is true', () => {
    render(EditFeedDialog, { props: { feed: { ...feed, has_cookie: true }, categories, onClose: noop, onSaved: noop, onDelete: noop } });
    const ta = screen.getByPlaceholderText(/cookie/i) as HTMLTextAreaElement;
    expect(ta.value).toBe('');
  });

  it('on Save, sends cookie if typed, omits basic_auth if not typed', async () => {
    vi.mocked(api.updateSubscription).mockResolvedValue(undefined as any);
    render(EditFeedDialog, { props: { feed, categories, onClose: noop, onSaved: noop, onDelete: noop } });
    await fireEvent.input(screen.getByPlaceholderText(/cookie/i), { target: { value: 'session=abc' } });
    await fireEvent.click(screen.getByRole('button', { name: /save changes/i }));
    expect(api.updateSubscription).toHaveBeenCalledWith(feed.id, expect.objectContaining({ cookie: 'session=abc' }));
  });
});

describe('EditFeedDialog — save + delete + cancel', () => {
  it('Save calls api.updateSubscription and onSaved', async () => {
    vi.mocked(api.updateSubscription).mockResolvedValue(undefined as any);
    const onSaved = vi.fn();
    render(EditFeedDialog, { props: { feed, categories, onClose: noop, onSaved, onDelete: noop } });
    await fireEvent.click(screen.getByRole('button', { name: /save changes/i }));
    expect(api.updateSubscription).toHaveBeenCalled();
    expect(onSaved).toHaveBeenCalledOnce();
  });

  it('Save surfaces errors inline; onSaved is not called', async () => {
    vi.mocked(api.updateSubscription).mockRejectedValue(new Error('csrf invalid'));
    const onSaved = vi.fn();
    render(EditFeedDialog, { props: { feed, categories, onClose: noop, onSaved, onDelete: noop } });
    await fireEvent.click(screen.getByRole('button', { name: /save changes/i }));
    expect(await screen.findByText(/csrf invalid/i)).toBeInTheDocument();
    expect(onSaved).not.toHaveBeenCalled();
  });

  it('Delete this feed button calls onDelete', async () => {
    const onDelete = vi.fn();
    render(EditFeedDialog, { props: { feed, categories, onClose: noop, onSaved: noop, onDelete } });
    await fireEvent.click(screen.getByRole('button', { name: /delete this feed/i }));
    expect(onDelete).toHaveBeenCalledOnce();
  });

  it('Cancel calls onClose', async () => {
    const onClose = vi.fn();
    render(EditFeedDialog, { props: { feed, categories, onClose, onSaved: noop, onDelete: noop } });
    await fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }));
    expect(onClose).toHaveBeenCalledOnce();
  });
});
