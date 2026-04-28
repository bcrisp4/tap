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
});
