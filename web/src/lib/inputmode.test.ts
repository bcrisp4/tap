// Unit tests for the global input-mode tracker. The store starts in
// 'mouse' (the safest default — selection highlight stays hidden until
// the user actually presses a key) and flips on the first event of
// each kind seen.
//
// We construct minimal event-shaped objects rather than real DOM event
// constructors because vitest runs in node by default and KeyboardEvent
// / MouseEvent aren't defined there. The handlers only read `.key` off
// the input, so a structural type is sufficient.

import { describe, it, expect, beforeEach } from 'vitest';
import { inputMode } from './inputmode.svelte';

const keyEv = (key: string) => ({ key }) as KeyboardEvent;
const mouseEv = () => ({}) as MouseEvent;
const touchEv = () => ({}) as Event;

describe('inputMode', () => {
	beforeEach(() => {
		// Reset to the default between tests so cases stay independent.
		inputMode.set('mouse');
	});

	it('defaults to "mouse" so the keyboard highlight stays hidden until proven otherwise', () => {
		expect(inputMode.mode).toBe('mouse');
	});

	it('flips to keyboard on a keydown event', () => {
		inputMode.handleKeydown(keyEv('j'));
		expect(inputMode.mode).toBe('keyboard');
	});

	it('flips to mouse on a mousemove event', () => {
		inputMode.set('keyboard');
		inputMode.handleMouseMove(mouseEv());
		expect(inputMode.mode).toBe('mouse');
	});

	it('flips to touch on a touchstart event', () => {
		inputMode.set('keyboard');
		inputMode.handleTouchStart(touchEv());
		expect(inputMode.mode).toBe('touch');
	});

	it('cycles between modes as the user changes input style', () => {
		inputMode.handleKeydown(keyEv('j'));
		expect(inputMode.mode).toBe('keyboard');
		inputMode.handleMouseMove(mouseEv());
		expect(inputMode.mode).toBe('mouse');
		inputMode.handleTouchStart(touchEv());
		expect(inputMode.mode).toBe('touch');
		inputMode.handleKeydown(keyEv('k'));
		expect(inputMode.mode).toBe('keyboard');
	});

	it('ignores keydown for non-navigation keys like modifier-only presses', () => {
		// Pressing Shift on its own (e.g. preparing for a mouse-shift-click)
		// shouldn't flip into keyboard mode — that would un-hide the
		// keyboard-only highlight as soon as a user reaches for the keyboard
		// to do a chord click. Only "real" key presses count.
		inputMode.handleKeydown(keyEv('Shift'));
		expect(inputMode.mode).toBe('mouse');
		inputMode.handleKeydown(keyEv('Control'));
		expect(inputMode.mode).toBe('mouse');
		inputMode.handleKeydown(keyEv('Alt'));
		expect(inputMode.mode).toBe('mouse');
		inputMode.handleKeydown(keyEv('Meta'));
		expect(inputMode.mode).toBe('mouse');
	});
});
