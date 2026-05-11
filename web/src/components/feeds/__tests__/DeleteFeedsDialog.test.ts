import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import DeleteFeedsDialog from '../DeleteFeedsDialog.svelte';
import type { Subscription } from '../../../lib/types';

const noop = () => {};

function makeFeed(id: number, title: string): Subscription {
  return {
    id, title, feed_url: `https://feed${id}.example/feed`, site_url: '',
    next_poll_at: 0, error_count: 0, created_at: 0,
    extract: false, extract_selector: '', has_cookie: false, has_basic_auth: false, category_id: null,
  };
}

const feed = makeFeed(1, 'Julia Evans');
const feed2 = makeFeed(2, 'Dan Luu');
const feed3 = makeFeed(3, 'Third');
const bigArray = Array.from({ length: 7 }, (_, i) => makeFeed(i + 1, `Feed ${i + 1}`));

describe('DeleteFeedsDialog', () => {
  it('single feed: title reads `Delete "<feed.title>"?`', () => {
    render(DeleteFeedsDialog, { props: { feeds: [feed], onClose: noop, onConfirm: async () => {} } });
    expect(screen.getByText(`Delete "${feed.title}"?`)).toBeInTheDocument();
  });

  it('bulk: title reads `Delete N feeds?`', () => {
    render(DeleteFeedsDialog, { props: { feeds: [feed, feed2, feed3], onClose: noop, onConfirm: async () => {} } });
    expect(screen.getByText(/delete 3 feeds\?/i)).toBeInTheDocument();
  });

  it('shows up to 5 feed rows; "…and N more" when more', () => {
    render(DeleteFeedsDialog, { props: { feeds: bigArray.slice(0, 7), onClose: noop, onConfirm: async () => {} } });
    expect(screen.getByText(/and 2 more/i)).toBeInTheDocument();
  });

  it('Confirm calls onConfirm; Cancel calls onClose', async () => {
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    render(DeleteFeedsDialog, { props: { feeds: [feed], onClose, onConfirm } });
    await fireEvent.click(screen.getByRole('button', { name: /^delete feed$/i }));
    await fireEvent.click(screen.getByRole('button', { name: /^cancel$/i }));
    expect(onConfirm).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
  });

  it('Confirm button is disabled during in-flight and re-enables after rejection', async () => {
    let rej!: (e: Error) => void;
    const onConfirm = vi.fn(() => new Promise<void>((_, r) => { rej = r; }));
    render(DeleteFeedsDialog, { props: { feeds: [feed], onClose: noop, onConfirm } });
    const confirm = screen.getByRole('button', { name: /^delete feed$/i });
    await fireEvent.click(confirm);
    expect(confirm).toBeDisabled();
    rej(new Error('boom'));
    await waitFor(() => expect(confirm).not.toBeDisabled());
  });
});
