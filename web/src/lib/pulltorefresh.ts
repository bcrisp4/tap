const THRESHOLD_PX = 60;

export function recognisePull(dy: number, scrollTop: number, inFlight: boolean): boolean {
  return !inFlight && scrollTop === 0 && dy >= THRESHOLD_PX;
}

export interface PullToRefreshOptions {
  onRefresh: () => Promise<void>;
  getScrollTop: () => number;
}

// {@attach}-compatible: returns (element) => cleanup.
export function pullToRefresh(opts: PullToRefreshOptions) {
  return (el: Element) => {
    let startY = 0, inFlight = false;
    const onStart = (e: Event) => {
      startY = (e as TouchEvent).touches[0].clientY;
    };
    const onEnd = (e: Event) => {
      const dy = (e as TouchEvent).changedTouches[0].clientY - startY;
      if (!recognisePull(dy, opts.getScrollTop(), inFlight)) return;
      inFlight = true;
      opts.onRefresh().finally(() => { inFlight = false; });
    };
    el.addEventListener('touchstart', onStart, { passive: true });
    el.addEventListener('touchend', onEnd, { passive: true });
    return () => {
      el.removeEventListener('touchstart', onStart);
      el.removeEventListener('touchend', onEnd);
    };
  };
}
