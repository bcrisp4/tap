// Shared breakpoint runes. Any route deciding between desktop / mobile
// chrome reads from this module so the threshold (≤ 720 px per the
// design handoff's `.is-mobile` rule) stays in one place.
//
// Built on Svelte's MediaQuery class so the value is rune-reactive
// without the consumer wiring an onMount + matchMedia listener.

import { MediaQuery } from 'svelte/reactivity';

const mobileMQ = new MediaQuery('max-width: 720px');

export function isMobile(): boolean {
	return mobileMQ.current;
}
