// Generic horizontal swipe state machine.
//
// Tracks a single-finger touch from start through move to end and
// fires `onSwipe` if the gesture crossed a configurable threshold AND
// the horizontal magnitude clearly dominates the vertical (so the
// natural vertical-scroll doesn't trigger a swipe).
//
// Multi-touch gestures (pinch-zoom, two-finger swipes) are explicitly
// rejected — once a second finger arrives the in-flight gesture is
// canceled. This keeps the state machine focused on the simplest
// useful affordance: tap, swipe-left, swipe-right.
//
// The exposed `swipeDx` rune lets the consumer paint the row sliding
// under the finger as visual feedback.

export type SwipeDirection = 'left' | 'right';

export interface SwipeEvent {
	direction: SwipeDirection;
	dx: number;
	dy: number;
}

export interface SwipeOptions {
	// Horizontal pixels the finger must travel to fire onSwipe.
	threshold?: number;
	// Vertical:horizontal magnitude above which the gesture is treated
	// as a vertical scroll and ignored.
	verticalRatio?: number;
	onSwipe: (ev: SwipeEvent) => void;
}

export interface SwipeHandlers {
	swipeDx: number;
	onTouchStart: (ev: TouchEvent) => void;
	onTouchMove: (ev: TouchEvent) => void;
	onTouchEnd: (ev: TouchEvent) => void;
	onTouchCancel: (ev: TouchEvent) => void;
}

class SwipeState {
	startX = 0;
	startY = 0;
	currentX = 0;
	currentY = 0;
	active = false;
	canceled = false;

	swipeDx = $state(0);

	reset() {
		this.active = false;
		this.canceled = false;
		if (this.swipeDx !== 0) this.swipeDx = 0;
	}
}

export function createSwipe(opts: SwipeOptions): SwipeHandlers {
	const threshold = opts.threshold ?? 60;
	const verticalRatio = opts.verticalRatio ?? 1.5;
	const state = new SwipeState();

	const onTouchStart = (ev: TouchEvent) => {
		// Multi-touch landing on top of an in-flight gesture: kill it
		// outright. Without the reset, a later touchend could still
		// fire onSwipe based on the original single-finger dx.
		if (ev.touches.length !== 1) {
			if (state.active) state.reset();
			return;
		}
		const t = ev.touches[0];
		state.active = true;
		state.canceled = false;
		state.startX = t.clientX;
		state.startY = t.clientY;
		state.currentX = t.clientX;
		state.currentY = t.clientY;
		if (state.swipeDx !== 0) state.swipeDx = 0;
	};

	const onTouchMove = (ev: TouchEvent) => {
		if (!state.active) return;
		// Multi-touch arriving mid-swipe → fully reset. A bare
		// `canceled = true` would still let later single-finger
		// touchmoves repaint swipeDx once the second finger lifts.
		if (ev.touches.length !== 1) {
			state.reset();
			return;
		}
		const t = ev.touches[0];
		state.currentX = t.clientX;
		state.currentY = t.clientY;
		const dx = state.currentX - state.startX;
		const dy = state.currentY - state.startY;
		// Suppress the visual under-finger drag if the gesture looks
		// like a vertical scroll. The natural vertical scroll wins.
		const next = Math.abs(dy) > Math.abs(dx) * verticalRatio ? 0 : dx;
		if (state.swipeDx !== next) state.swipeDx = next;
	};

	const onTouchEnd = (ev: TouchEvent) => {
		if (!state.active) return;
		// Pull the final position from changedTouches so a quick
		// flick with no intermediate touchmove (notably iOS Safari)
		// still resolves to the real dx, not the stale touchstart x.
		if (ev.changedTouches.length > 0) {
			const t = ev.changedTouches[0];
			state.currentX = t.clientX;
			state.currentY = t.clientY;
		}
		const dx = state.currentX - state.startX;
		const dy = state.currentY - state.startY;
		const wasCanceled = state.canceled;
		state.reset();
		if (wasCanceled) return;
		// Vertical-scroll dominance: not a swipe.
		if (Math.abs(dy) > Math.abs(dx) * verticalRatio) return;
		if (Math.abs(dx) < threshold) return;
		opts.onSwipe({ direction: dx > 0 ? 'right' : 'left', dx, dy });
	};

	const onTouchCancel = (_ev: TouchEvent) => {
		state.reset();
	};

	return {
		get swipeDx() {
			return state.swipeDx;
		},
		onTouchStart,
		onTouchMove,
		onTouchEnd,
		onTouchCancel
	};
}
