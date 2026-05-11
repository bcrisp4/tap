import { render } from '@testing-library/svelte';
import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import MobileTabBar from '../MobileTabBar.svelte';
import { navigate } from '../../lib/router';

describe('MobileTabBar', () => {
  beforeEach(() => {
    vi.stubGlobal('history', { pushState: vi.fn(), state: {} });
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('renders five primary tabs', () => {
    navigate('/');
    const { getByLabelText } = render(MobileTabBar, { unreadCount: 0 });
    ['Unread', 'Saved', 'Feeds', 'Categories', 'More'].forEach(t =>
      expect(getByLabelText(t)).toBeTruthy(),
    );
  });

  it('marks More active when route is history', () => {
    navigate('/history');
    const { getByLabelText } = render(MobileTabBar, { unreadCount: 0 });
    expect(getByLabelText('More').className).toMatch(/is-active/);
  });

  it('marks More active when route is settings', () => {
    navigate('/settings');
    const { getByLabelText } = render(MobileTabBar, { unreadCount: 0 });
    expect(getByLabelText('More').className).toMatch(/is-active/);
  });

  it('renders unread badge when count > 0', () => {
    navigate('/');
    const { getByText } = render(MobileTabBar, { unreadCount: 7 });
    expect(getByText('7')).toBeTruthy();
  });
});
