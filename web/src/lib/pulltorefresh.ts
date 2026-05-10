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
    const onStart = (e: TouchEvent) => { startY = e.touches[0].clientY; };
    const onEnd = async (e: TouchEvent) => {
      const dy = e.changedTouches[0].clientY - startY;
      if (!recognisePull(dy, opts.getScrollTop(), inFlight)) return;
      inFlight = true;
      try { await opts.onRefresh(); } finally { inFlight = false; }
    };
    el.addEventListener('touchstart', onStart as EventListener, { passive: true });
    el.addEventListener('touchend', onEnd as EventListener, { passive: true });
    return () => {
      el.removeEventListener('touchstart', onStart as EventListener);
      el.removeEventListener('touchend', onEnd as EventListener);
    };
  };
}
