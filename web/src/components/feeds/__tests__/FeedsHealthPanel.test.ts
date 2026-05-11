import { render, screen, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import FeedsHealthPanel from '../FeedsHealthPanel.svelte';
import type { FeedRow } from '../../../lib/feedsFilter';

const feedWithError: FeedRow = {
  id: 1, title: 'Bad Feed', feed_url: 'https://bad.example/feed',
  site_url: '', next_poll_at: 0, last_poll_at: Math.floor(Date.now() / 1000) - 7200,
  error_count: 5, last_error: 'HTTP 502 Bad Gateway',
  created_at: 0, extract: false, extract_selector: '', has_cookie: false, has_basic_auth: false,
  category_id: null, unread: 0, lastPollAgo: 7200,
};

describe('FeedsHealthPanel', () => {
  it('renders the last error string and the 3-cell grid', () => {
    const { container } = render(FeedsHealthPanel, {
      props: { feed: feedWithError, onRetry: vi.fn(), onEdit: vi.fn() },
    });
    expect(container.querySelector('.ts-feed-health-msg')!.textContent).toContain('HTTP 502 Bad Gateway');
    expect(container.querySelectorAll('.ts-feed-health-cell')).toHaveLength(3);
  });

  it('Retry now button calls onRetry; Edit credentials calls onEdit', async () => {
    const onRetry = vi.fn(), onEdit = vi.fn();
    render(FeedsHealthPanel, { props: { feed: feedWithError, onRetry, onEdit } });
    await fireEvent.click(screen.getByRole('button', { name: /retry now/i }));
    await fireEvent.click(screen.getByRole('button', { name: /edit credentials/i }));
    expect(onRetry).toHaveBeenCalledOnce();
    expect(onEdit).toHaveBeenCalledOnce();
  });
});
