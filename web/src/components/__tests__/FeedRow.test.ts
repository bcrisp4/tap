import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { api } from '../../lib/api';

vi.mock('../../lib/store', () => ({
  subscriptions: {
    subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; },
    load: vi.fn(),
  },
  categories: {
    subscribe: (fn: (v: unknown[]) => void) => { fn([]); return () => {}; },
    load: vi.fn(),
  },
}));

vi.mock('../../lib/router', () => ({
  navigate: vi.fn(),
  route: { subscribe: (fn: (v: { name: string }) => void) => { fn({ name: 'unread' }); return () => {}; } },
}));

import FeedRow from '../FeedRow.svelte';

describe('FeedRow', () => {
  const sub = {
    id: 3,
    title: 'Demo',
    feed_url: 'https://d/f',
    category_id: null,
    extract: false,
    extract_selector: '',
    has_cookie: false,
    has_basic_auth: false,
  } as any;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows the feed title', () => {
    const { getByText } = render(FeedRow, { props: { subscription: sub, categories: [] } });
    expect(getByText('Demo')).toBeTruthy();
  });

  it('calls api.deleteSubscription on confirmed delete', async () => {
    const spy = vi.spyOn(api, 'deleteSubscription').mockResolvedValue(undefined);
    const { getByLabelText, getByRole } = render(FeedRow, { props: { subscription: sub, categories: [] } });
    await fireEvent.click(getByLabelText('Feed actions'));
    await fireEvent.click(getByRole('button', { name: /delete feed/i }));
    await fireEvent.click(getByRole('button', { name: /confirm/i }));
    await waitFor(() => {
      expect(spy).toHaveBeenCalledWith(3);
    });
  });
});
