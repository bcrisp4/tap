import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { get } from 'svelte/store';

function setPathname(path: string) {
  Object.defineProperty(window, 'location', {
    value: { pathname: path, href: `http://localhost${path}` },
    writable: true,
    configurable: true,
  });
}

describe('router', () => {
  beforeEach(() => {
    vi.resetModules();
    vi.stubGlobal('history', { pushState: vi.fn() });
    setPathname('/');
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('parses /', async () => {
    setPathname('/');
    const { route } = await import('../router');
    expect(get(route)).toEqual({ name: 'unread' });
  });

  it('parses /entry/123', async () => {
    setPathname('/entry/123');
    const { route } = await import('../router');
    expect(get(route)).toEqual({ name: 'reader', params: { id: 123 } });
  });

  it('parses /saved', async () => {
    setPathname('/saved');
    const { route } = await import('../router');
    expect(get(route)).toEqual({ name: 'saved' });
  });

  it('parses /categories', async () => {
    setPathname('/categories');
    const { route } = await import('../router');
    expect(get(route)).toEqual({ name: 'categories' });
  });

  it('parses /feeds', async () => {
    setPathname('/feeds');
    const { route } = await import('../router');
    expect(get(route)).toEqual({ name: 'feeds' });
  });

  it('parses /history', async () => {
    setPathname('/history');
    const { route } = await import('../router');
    expect(get(route)).toEqual({ name: 'history' });
  });

  it('parses /settings', async () => {
    setPathname('/settings');
    const { route } = await import('../router');
    expect(get(route)).toEqual({ name: 'settings' });
  });

  it('parses /admin', async () => {
    setPathname('/admin');
    const { route } = await import('../router');
    expect(get(route)).toEqual({ name: 'admin' });
  });

  it('parses /sign-in', async () => {
    setPathname('/sign-in');
    const { route } = await import('../router');
    expect(get(route)).toEqual({ name: 'signin' });
  });

  it('falls back to unread for /search (deleted)', async () => {
    setPathname('/search');
    const { route } = await import('../router');
    expect(get(route)).toEqual({ name: 'unread' });
  });

  it('falls back to unread for /categories/1 (deleted per-category route)', async () => {
    setPathname('/categories/1');
    const { route } = await import('../router');
    expect(get(route)).toEqual({ name: 'unread' });
  });

  it('navigate() updates route and calls pushState', async () => {
    setPathname('/');
    const { route, navigate } = await import('../router');
    navigate('/saved');
    expect(window.history.pushState).toHaveBeenCalled();
    expect(get(route)).toEqual({ name: 'saved' });
  });

  it('navigate() does not push when already on same path', async () => {
    setPathname('/saved');
    const { navigate } = await import('../router');
    navigate('/saved');
    expect(window.history.pushState).not.toHaveBeenCalled();
  });
});
