import { render } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import TopTabs from '../TopTabs.svelte';
import { navigate } from '../../lib/router';

describe('TopTabs', () => {
  beforeEach(() => {
    vi.stubGlobal('history', { pushState: vi.fn(), state: {} });
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('renders all visible tabs', () => {
    navigate('/');
    const { getByText } = render(TopTabs, { unreadCount: 0, isAdmin: false });
    ['Unread', 'Saved', 'History', 'Categories', 'Feeds', 'Settings'].forEach(label => {
      expect(getByText(label)).toBeTruthy();
    });
  });

  it('shows Admin tab when isAdmin', () => {
    navigate('/');
    const { getByText } = render(TopTabs, { unreadCount: 0, isAdmin: true });
    expect(getByText('Admin')).toBeTruthy();
  });

  it('renders unread count beside Unread tab', () => {
    navigate('/');
    const { getByText } = render(TopTabs, { unreadCount: 12, isAdmin: false });
    expect(getByText('12')).toBeTruthy();
  });

  it('marks the active tab', () => {
    navigate('/saved');
    const { getByText } = render(TopTabs, { unreadCount: 0, isAdmin: false });
    const tab = getByText('Saved').closest('button');
    expect(tab?.className).toMatch(/is-active/);
  });
});
