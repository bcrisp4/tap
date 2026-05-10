import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';

// Helpers to manipulate the simulated location and history.
function setPathname(path: string) {
  // jsdom sets window.location via the object below:
  Object.defineProperty(window, 'location', {
    value: { pathname: path, href: `http://localhost${path}` },
    writable: true,
    configurable: true,
  });
}

describe('router URL parsing', () => {
  // The router module has module-level side effects (it reads window.location
  // and attaches a popstate listener). To test different initial paths we
  // dynamically import a fresh module per test.  Vitest's module cache is
  // reset between vi.resetModules() calls.

  beforeEach(() => {
    vi.resetModules();
    // Provide history.pushState stub so the module doesn't crash.
    vi.stubGlobal('history', { pushState: vi.fn() });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('parses / as the unread route', async () => {
    setPathname('/');
    const { route } = await import('../router');
    let current: unknown;
    const unsub = route.subscribe((v) => { current = v; });
    unsub();
    expect(current).toEqual({ name: 'unread' });
  });

  it('parses /entry/42 as the reader route with id=42', async () => {
    setPathname('/entry/42');
    const { route } = await import('../router');
    let current: unknown;
    const unsub = route.subscribe((v) => { current = v; });
    unsub();
    expect(current).toEqual({ name: 'reader', params: { id: 42 } });
  });

  it('parses /entry/0 as reader route with id=0', async () => {
    setPathname('/entry/0');
    const { route } = await import('../router');
    let current: unknown;
    const unsub = route.subscribe((v) => { current = v; });
    unsub();
    expect(current).toEqual({ name: 'reader', params: { id: 0 } });
  });

  it('falls back to unread for unknown paths', async () => {
    setPathname('/unknown/path');
    const { route } = await import('../router');
    let current: unknown;
    const unsub = route.subscribe((v) => { current = v; });
    unsub();
    expect(current).toEqual({ name: 'unread' });
  });

  it('parses /saved as saved route', async () => {
    setPathname('/saved');
    const { route } = await import('../router');
    let current: unknown;
    const unsub = route.subscribe((v) => { current = v; });
    unsub();
    expect(current).toEqual({ name: 'saved' });
  });

  it('parses /search as search route', async () => {
    setPathname('/search');
    const { route } = await import('../router');
    let current: unknown;
    const unsub = route.subscribe((v) => { current = v; });
    unsub();
    expect(current).toEqual({ name: 'search' });
  });

  it('parses /settings as settings route', async () => {
    setPathname('/settings');
    const { route } = await import('../router');
    let current: unknown;
    const unsub = route.subscribe((v) => { current = v; });
    unsub();
    expect(current).toEqual({ name: 'settings' });
  });
});

describe('navigate()', () => {
  beforeEach(() => {
    vi.resetModules();
    vi.stubGlobal('history', { pushState: vi.fn() });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('calls history.pushState and updates the route when navigating to a new path', async () => {
    setPathname('/');
    const { route, navigate } = await import('../router');

    navigate('/entry/7');

    const states: unknown[] = [];
    const unsub = route.subscribe((v) => { states.push(v); });
    unsub();

    expect(window.history.pushState).toHaveBeenCalled();
    expect(states[states.length - 1]).toEqual({ name: 'reader', params: { id: 7 } });
  });

  it('does not push a new state when navigating to the current path', async () => {
    setPathname('/entry/7');
    const { navigate } = await import('../router');

    navigate('/entry/7');

    expect(window.history.pushState).not.toHaveBeenCalled();
  });
});
