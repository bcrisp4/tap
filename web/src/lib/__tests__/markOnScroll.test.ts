import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createMarkOnScroll } from '../markOnScroll';

let observers: Array<{ cb: IntersectionObserverCallback; el: Element }> = [];

class FakeIO implements IntersectionObserver {
  root = null; rootMargin = ''; thresholds = [];
  cb: IntersectionObserverCallback;
  constructor(cb: IntersectionObserverCallback) { this.cb = cb; }
  observe(el: Element) { observers.push({ cb: this.cb, el }); }
  disconnect() { observers = observers.filter(o => o.cb !== this.cb); }
  unobserve() { /* not used */ }
  takeRecords() { return []; }
}

beforeEach(() => {
  vi.useFakeTimers();
  observers = [];
  (globalThis as unknown as { IntersectionObserver: typeof IntersectionObserver }).IntersectionObserver = FakeIO as unknown as typeof IntersectionObserver;
});
afterEach(() => vi.useRealTimers());

function fireIntersection(isIntersecting: boolean, top = isIntersecting ? 100 : -50) {
  const o = observers[0];
  const entry = { isIntersecting, boundingClientRect: { top } as DOMRectReadOnly } as unknown as IntersectionObserverEntry;
  o.cb([entry], {} as IntersectionObserver);
}

describe('createMarkOnScroll', () => {
  it('fires onMark 1.5s after lede leaves the viewport above', () => {
    const onMark = vi.fn();
    const el = document.createElement('div');
    const attach = createMarkOnScroll({ onMark, delayMs: 1500 });
    attach(el);
    fireIntersection(true);
    fireIntersection(false);
    vi.advanceTimersByTime(1499);
    expect(onMark).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(onMark).toHaveBeenCalledOnce();
  });

  it('does not fire if user scrolls back before timer fires', () => {
    const onMark = vi.fn();
    const el = document.createElement('div');
    const attach = createMarkOnScroll({ onMark });
    attach(el);
    fireIntersection(true);
    fireIntersection(false);
    vi.advanceTimersByTime(500);
    fireIntersection(true);
    vi.advanceTimersByTime(2000);
    expect(onMark).not.toHaveBeenCalled();
  });

  it('fires at most once even if lede leaves and re-leaves', () => {
    const onMark = vi.fn();
    const el = document.createElement('div');
    const attach = createMarkOnScroll({ onMark });
    attach(el);
    fireIntersection(true); fireIntersection(false);
    vi.advanceTimersByTime(1500);
    expect(onMark).toHaveBeenCalledOnce();
    fireIntersection(true); fireIntersection(false);
    vi.advanceTimersByTime(2000);
    expect(onMark).toHaveBeenCalledOnce();
  });

  it('cleanup disconnects the observer', () => {
    const onMark = vi.fn();
    const el = document.createElement('div');
    const attach = createMarkOnScroll({ onMark });
    const cleanup = attach(el);
    expect(observers).toHaveLength(1);
    cleanup();
    expect(observers).toHaveLength(0);
  });

  it('treats the lede as "above viewport" only when boundingClientRect.top < 0', () => {
    const onMark = vi.fn();
    const el = document.createElement('div');
    const attach = createMarkOnScroll({ onMark });
    attach(el);
    const o = observers[0];
    o.cb([{ isIntersecting: false, boundingClientRect: { top: 5000 } as DOMRectReadOnly } as unknown as IntersectionObserverEntry], {} as IntersectionObserver);
    vi.advanceTimersByTime(2000);
    expect(onMark).not.toHaveBeenCalled();
  });
});
