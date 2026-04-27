// Single-source keyboard binding for the unread / reader views.
//
// Callers provide a small handler bag describing what each key should
// do; the binding is registered on `window` for the lifetime of the
// component that called `bindKeyboard()`. The factory shape lets each
// route compose its own selection state and mutations without leaking
// global state through a shared store.
//
// Suppression rule: shortcuts are ignored while the user is typing
// into a form control (input, textarea, contenteditable).

import { onMount } from 'svelte';
import { goto } from '$app/navigation';

export type KeyHandlers = {
	onNext: () => void;
	onPrev: () => void;
	onToggleRead: () => void;
	onToggleSaved: () => void;
	onOpen: () => void;
	onViewOriginal: () => void;
};

export function bindKeyboard(h: KeyHandlers): void {
	onMount(() => {
		const onKey = (ev: KeyboardEvent) => {
			const target = ev.target as HTMLElement | null;
			if (
				target &&
				(target.tagName === 'INPUT' ||
					target.tagName === 'TEXTAREA' ||
					target.isContentEditable)
			) {
				return;
			}
			switch (ev.key) {
				case 'j':
				case 'ArrowDown':
					ev.preventDefault();
					h.onNext();
					break;
				case 'k':
				case 'ArrowUp':
					ev.preventDefault();
					h.onPrev();
					break;
				case 'm':
					ev.preventDefault();
					h.onToggleRead();
					break;
				case 's':
					ev.preventDefault();
					h.onToggleSaved();
					break;
				case 'o':
				case 'Enter':
					ev.preventDefault();
					h.onOpen();
					break;
				case 'v':
					ev.preventDefault();
					h.onViewOriginal();
					break;
				case '/':
					ev.preventDefault();
					void goto('/search');
					break;
			}
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});
}
