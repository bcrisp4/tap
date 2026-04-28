// Single-message toast store. Mounted once via Toast.svelte in
// +layout.svelte. Used by mutation rollback in queries.ts and (later)
// by OPML import/export feedback in Plan 21.
//
// Replaces — rather than queues — an active toast: errors during
// rapid bulk actions stay coherent without piling up overlapping
// banners on top of each other.

type Kind = 'info' | 'error';
type Message = { message: string; kind: Kind };

let current = $state<Message | null>(null);
let timer: ReturnType<typeof setTimeout> | null = null;

function dismiss() {
	if (timer) {
		clearTimeout(timer);
		timer = null;
	}
	current = null;
}

function push(message: string, kind: Kind = 'info') {
	if (timer) clearTimeout(timer);
	current = { message, kind };
	timer = setTimeout(dismiss, 4000);
}

export const toast = {
	get current() {
		return current;
	},
	push,
	dismiss
};
