// Unit tests for the generic swipe state machine. We invoke the
// handlers directly with synthetic touch-event-shaped inputs rather
// than dispatching real DOM events — vitest runs in node by default
// and TouchEvent isn't defined there, plus testing the state machine
// directly is more direct than driving it through the DOM.

import { describe, it, expect, vi } from 'vitest';
import { createSwipe } from './swipe.svelte';

interface TouchPoint {
	clientX: number;
	clientY: number;
}

function makeTouchEvent(points: TouchPoint[]): TouchEvent {
	return {
		touches: points,
		changedTouches: points,
		preventDefault: () => undefined,
		stopPropagation: () => undefined
	} as unknown as TouchEvent;
}

describe('createSwipe', () => {
	it('fires onSwipe with right direction when finger moves right past threshold', () => {
		const onSwipe = vi.fn();
		const sw = createSwipe({ threshold: 60, onSwipe });
		sw.onTouchStart(makeTouchEvent([{ clientX: 50, clientY: 100 }]));
		sw.onTouchMove(makeTouchEvent([{ clientX: 130, clientY: 100 }]));
		sw.onTouchEnd(makeTouchEvent([{ clientX: 130, clientY: 100 }]));
		expect(onSwipe).toHaveBeenCalledTimes(1);
		expect(onSwipe).toHaveBeenCalledWith({ direction: 'right', dx: 80, dy: 0 });
	});

	it('fires onSwipe with left direction when finger moves left past threshold', () => {
		const onSwipe = vi.fn();
		const sw = createSwipe({ threshold: 60, onSwipe });
		sw.onTouchStart(makeTouchEvent([{ clientX: 200, clientY: 100 }]));
		sw.onTouchMove(makeTouchEvent([{ clientX: 80, clientY: 100 }]));
		sw.onTouchEnd(makeTouchEvent([{ clientX: 80, clientY: 100 }]));
		expect(onSwipe).toHaveBeenCalledTimes(1);
		expect(onSwipe.mock.calls[0][0].direction).toBe('left');
	});

	it('does not fire when horizontal delta is below threshold', () => {
		const onSwipe = vi.fn();
		const sw = createSwipe({ threshold: 60, onSwipe });
		sw.onTouchStart(makeTouchEvent([{ clientX: 50, clientY: 100 }]));
		sw.onTouchMove(makeTouchEvent([{ clientX: 80, clientY: 100 }]));
		sw.onTouchEnd(makeTouchEvent([{ clientX: 80, clientY: 100 }]));
		expect(onSwipe).not.toHaveBeenCalled();
	});

	it('does not fire when vertical scroll dominates (vertical-magnitude > horizontal × 1.5)', () => {
		const onSwipe = vi.fn();
		const sw = createSwipe({ threshold: 60, onSwipe });
		sw.onTouchStart(makeTouchEvent([{ clientX: 50, clientY: 100 }]));
		// dx = 80, dy = 200 — clearly a vertical scroll. Must NOT fire,
		// otherwise users couldn't scroll a long entry list.
		sw.onTouchMove(makeTouchEvent([{ clientX: 130, clientY: 300 }]));
		sw.onTouchEnd(makeTouchEvent([{ clientX: 130, clientY: 300 }]));
		expect(onSwipe).not.toHaveBeenCalled();
	});

	it('exposes the in-flight horizontal delta on swipeDx for visual feedback', () => {
		const onSwipe = vi.fn();
		const sw = createSwipe({ threshold: 60, onSwipe });
		sw.onTouchStart(makeTouchEvent([{ clientX: 100, clientY: 100 }]));
		sw.onTouchMove(makeTouchEvent([{ clientX: 140, clientY: 100 }]));
		expect(sw.swipeDx).toBe(40);
		sw.onTouchEnd(makeTouchEvent([{ clientX: 140, clientY: 100 }]));
		// Reset to 0 on end so the row springs back visually.
		expect(sw.swipeDx).toBe(0);
	});

	it('cancels (no fire) if a second touch starts mid-swipe (multi-touch gesture)', () => {
		const onSwipe = vi.fn();
		const sw = createSwipe({ threshold: 60, onSwipe });
		sw.onTouchStart(makeTouchEvent([{ clientX: 50, clientY: 100 }]));
		sw.onTouchMove(makeTouchEvent([{ clientX: 130, clientY: 100 }]));
		// Second finger arrives — likely a pinch-zoom; must NOT register
		// as a swipe.
		sw.onTouchMove(
			makeTouchEvent([
				{ clientX: 130, clientY: 100 },
				{ clientX: 200, clientY: 100 }
			])
		);
		sw.onTouchEnd(
			makeTouchEvent([
				{ clientX: 130, clientY: 100 },
				{ clientX: 200, clientY: 100 }
			])
		);
		expect(onSwipe).not.toHaveBeenCalled();
	});

	// Defensive edge cases (Copilot review on PR #21).

	it('cancels in-flight gesture when a second touchstart arrives without an intervening touchmove', () => {
		// Real-device flow: finger 1 down → finger 2 down (second
		// touchstart fires with touches.length === 2). Without an
		// intermediate touchmove the original implementation never
		// flagged the gesture canceled, so the eventual touchend
		// could still fire onSwipe with the original dx.
		const onSwipe = vi.fn();
		const sw = createSwipe({ threshold: 60, onSwipe });
		sw.onTouchStart(makeTouchEvent([{ clientX: 50, clientY: 100 }]));
		sw.onTouchMove(makeTouchEvent([{ clientX: 130, clientY: 100 }]));
		// Second finger lands. No touchmove between this and touchend.
		sw.onTouchStart(
			makeTouchEvent([
				{ clientX: 130, clientY: 100 },
				{ clientX: 200, clientY: 100 }
			])
		);
		sw.onTouchEnd(makeTouchEvent([{ clientX: 130, clientY: 100 }]));
		expect(onSwipe).not.toHaveBeenCalled();
	});

	it('stays canceled after multi-touch even if the second finger lifts and single-finger touchmoves resume', () => {
		// After multi-touch the gesture must stay dead until the next
		// clean touchstart. Visual swipeDx must not resume drifting
		// when one finger lifts and the other keeps moving.
		const onSwipe = vi.fn();
		const sw = createSwipe({ threshold: 60, onSwipe });
		sw.onTouchStart(makeTouchEvent([{ clientX: 50, clientY: 100 }]));
		sw.onTouchMove(makeTouchEvent([{ clientX: 130, clientY: 100 }]));
		// Second finger arrives → cancel.
		sw.onTouchMove(
			makeTouchEvent([
				{ clientX: 130, clientY: 100 },
				{ clientX: 200, clientY: 100 }
			])
		);
		// Second finger lifted; first finger keeps moving. Must NOT
		// resume painting swipeDx.
		sw.onTouchMove(makeTouchEvent([{ clientX: 180, clientY: 100 }]));
		expect(sw.swipeDx).toBe(0);
		sw.onTouchEnd(makeTouchEvent([{ clientX: 180, clientY: 100 }]));
		expect(onSwipe).not.toHaveBeenCalled();
	});

	it('uses changedTouches in touchend so a quick flick with no intermediate touchmove still registers', () => {
		// Some browsers (notably iOS Safari for very fast flicks)
		// deliver touchstart → touchend with no intermediate
		// touchmove. dx must be derived from changedTouches in
		// touchend, not the stale currentX from touchstart.
		const onSwipe = vi.fn();
		const sw = createSwipe({ threshold: 60, onSwipe });
		sw.onTouchStart(makeTouchEvent([{ clientX: 50, clientY: 100 }]));
		// No touchmove. touchend's changedTouches carries the final
		// position.
		sw.onTouchEnd(makeTouchEvent([{ clientX: 200, clientY: 100 }]));
		expect(onSwipe).toHaveBeenCalledTimes(1);
		expect(onSwipe).toHaveBeenCalledWith({ direction: 'right', dx: 150, dy: 0 });
	});
});
