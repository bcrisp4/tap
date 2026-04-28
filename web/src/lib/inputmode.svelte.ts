// Global input-mode rune.
//
// Tracks how the user is currently driving the UI: with the mouse, the
// keyboard, or a touch screen. The keyboard-selection highlight on
// EntryRow only renders when `mode === 'keyboard'` — without this gate
// the row that happens to be `selectedId` always paints highlighted,
// even when the user is clearly using a mouse or touch and never
// touched j/k.
//
// We default to 'mouse' (the safest default — it keeps the highlight
// hidden until the user actually presses a navigation key). The mode
// flips on the first event of each kind seen and stays there until the
// next input style is detected.
//
// `bindInputMode()` wires the listeners onto `window`; call it once at
// the root layout. The handlers are exported separately so unit tests
// can drive the store without simulating real DOM events.

export type InputMode = 'mouse' | 'keyboard' | 'touch';

// Modifier-only keypresses (e.g. holding Shift to do a chord click)
// should NOT flip the mode into 'keyboard' — otherwise reaching for
// the keyboard to shift-click would un-hide the keyboard highlight.
const MODIFIER_KEYS = new Set(['Shift', 'Control', 'Alt', 'Meta', 'CapsLock']);

class InputModeStore {
	mode = $state<InputMode>('mouse');

	// Same-value writes are guarded so high-frequency events
	// (mousemove fires every pixel of motion) don't re-notify
	// downstream consumers like the EntryRow highlight derivation.
	set(m: InputMode) {
		if (this.mode !== m) this.mode = m;
	}

	handleKeydown(ev: KeyboardEvent) {
		if (MODIFIER_KEYS.has(ev.key)) return;
		this.set('keyboard');
	}

	handleMouseMove(_ev: MouseEvent) {
		this.set('mouse');
	}

	handleTouchStart(_ev: Event) {
		this.set('touch');
	}
}

export const inputMode = new InputModeStore();

// One-call wiring for the root layout. Returns a cleanup function so
// the layout's onMount can dispose listeners on teardown.
export function bindInputMode(): () => void {
	if (typeof window === 'undefined') return () => undefined;
	const onKey = (ev: KeyboardEvent) => inputMode.handleKeydown(ev);
	const onMove = (ev: MouseEvent) => inputMode.handleMouseMove(ev);
	const onTouch = (ev: Event) => inputMode.handleTouchStart(ev);
	window.addEventListener('keydown', onKey, { capture: true });
	window.addEventListener('mousemove', onMove, { capture: true });
	window.addEventListener('touchstart', onTouch, { capture: true, passive: true });
	return () => {
		window.removeEventListener('keydown', onKey, { capture: true });
		window.removeEventListener('mousemove', onMove, { capture: true });
		window.removeEventListener('touchstart', onTouch, { capture: true });
	};
}
