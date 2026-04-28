// Global open-state rune for the hotkeys help modal.
//
// The modal is mounted once at the root layout so any chrome control
// (the SidebarFooter icon, the `?` keystroke handler) can flip the
// flag without coordinating refs through the component tree. Keeping
// the rune in its own module avoids a circular import between
// SidebarFooter, HotkeysModal, and keyboard.svelte.ts.

class HotkeysModalStore {
	open = $state(false);

	show() {
		this.open = true;
	}

	hide() {
		this.open = false;
	}

	toggle() {
		this.open = !this.open;
	}
}

export const hotkeysModal = new HotkeysModalStore();
