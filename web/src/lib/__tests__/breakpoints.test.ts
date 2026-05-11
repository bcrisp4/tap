import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';

describe('breakpoints.isMobile store', () => {
  let listeners: Array<(e: MediaQueryListEvent) => void>;
  let mql: { matches: boolean; addEventListener: ReturnType<typeof vi.fn>; removeEventListener: ReturnType<typeof vi.fn>; media: string };

  beforeEach(() => {
    listeners = [];
    mql = {
      matches: false,
      media: '(max-width: 768px)',
      addEventListener: vi.fn((_evt: string, fn: (e: MediaQueryListEvent) => void) => listeners.push(fn)),
      removeEventListener: vi.fn(),
    };
    vi.stubGlobal('matchMedia', vi.fn(() => mql));
  });

  it('initial value reflects matchMedia.matches', async () => {
    mql.matches = true;
    const { isMobile } = await import('../breakpoints.svelte?fresh-initial-true' as string);
    expect(get(isMobile)).toBe(true);
  });

  it('initial value false when not mobile', async () => {
    mql.matches = false;
    const { isMobile } = await import('../breakpoints.svelte?fresh-initial-false' as string);
    expect(get(isMobile)).toBe(false);
  });

  it('updates when the media query fires change', async () => {
    mql.matches = false;
    const { isMobile } = await import('../breakpoints.svelte?fresh-change' as string);
    expect(get(isMobile)).toBe(false);
    listeners.forEach(fn => fn({ matches: true } as MediaQueryListEvent));
    expect(get(isMobile)).toBe(true);
  });
});
