const MIN_TRAVEL = 40;
const MAX_ANGLE_DEG = 30;
const EDGE_GUARD_PX = 20;

export function recogniseSwipe(dx: number, dy: number, startX: number): 'left' | 'right' | null {
  if (startX <= EDGE_GUARD_PX) return null;
  const absDx = Math.abs(dx);
  if (absDx < MIN_TRAVEL) return null;
  const angleDeg = Math.atan2(Math.abs(dy), absDx) * (180 / Math.PI);
  if (angleDeg >= MAX_ANGLE_DEG) return null;
  return dx > 0 ? 'right' : 'left';
}

export interface SwipeOptions {
  onSwipeLeft?: () => void;
  onSwipeRight?: () => void;
}

// {@attach}-compatible: returns (element) => cleanup.
export function swipe(opts: SwipeOptions) {
  return (el: Element) => {
    let startX = 0, startY = 0;
    const onStart = (e: TouchEvent) => {
      startX = e.touches[0].clientX; startY = e.touches[0].clientY;
    };
    const onEnd = (e: TouchEvent) => {
      const t = e.changedTouches[0];
      const dir = recogniseSwipe(t.clientX - startX, t.clientY - startY, startX);
      if (dir === 'left') opts.onSwipeLeft?.();
      if (dir === 'right') opts.onSwipeRight?.();
    };
    el.addEventListener('touchstart', onStart as EventListener, { passive: true });
    el.addEventListener('touchend', onEnd as EventListener, { passive: true });
    return () => {
      el.removeEventListener('touchstart', onStart as EventListener);
      el.removeEventListener('touchend', onEnd as EventListener);
    };
  };
}
