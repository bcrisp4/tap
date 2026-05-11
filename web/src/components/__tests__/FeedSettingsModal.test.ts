import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, waitFor } from '@testing-library/svelte';
import { api } from '../../lib/api';

import FeedSettingsModal from '../FeedSettingsModal.svelte';

vi.mock('../../lib/router', () => ({
  navigate: vi.fn(),
  route: { subscribe: (fn: (v: { name: string }) => void) => { fn({ name: 'unread' }); return () => {}; } },
}));

describe('FeedSettingsModal', () => {
  const sub = {
    id: 7,
    title: 'X',
    feed_url: 'https://x/feed',
    extract: false,
    extract_selector: '',
    has_cookie: false,
    has_basic_auth: false,
    category_id: null,
  } as any;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('PATCHes the subscription with updated values on save', async () => {
    const spy = vi.spyOn(api, 'updateSubscription').mockResolvedValue(sub);
    const { getByRole, getByLabelText } = render(FeedSettingsModal, {
      props: { subscription: sub, onClose: vi.fn() },
    });
    await fireEvent.click(getByLabelText('Enable article extraction'));
    await fireEvent.input(getByLabelText('Extract CSS selector'), { target: { value: '.article-body' } });
    await fireEvent.click(getByRole('button', { name: /save/i }));
    await waitFor(() => {
      expect(spy).toHaveBeenCalledWith(
        7,
        expect.objectContaining({ extract: true, extract_selector: '.article-body' }),
      );
    });
  });

  it('calls onClose when Cancel is clicked', async () => {
    const onClose = vi.fn();
    const { getByRole } = render(FeedSettingsModal, {
      props: { subscription: sub, onClose },
    });
    await fireEvent.click(getByRole('button', { name: /cancel/i }));
    expect(onClose).toHaveBeenCalled();
  });
});
