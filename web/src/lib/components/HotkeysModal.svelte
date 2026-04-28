<script lang="ts">
	// Hotkeys help modal. Mounted once at the root layout so the
	// global rune toggles a single instance regardless of route.
	// Open via: the `?` keystroke (wired in $lib/keyboard.svelte.ts),
	// or the keyboard icon in SidebarFooter. Close via Esc, the
	// backdrop, or the explicit close button.
	import { hotkeysModal } from '$lib/hotkeys-modal.svelte';

	type Shortcut = { keys: string[]; description: string };

	const SHORTCUTS: ReadonlyArray<Shortcut> = [
		{ keys: ['j'], description: 'Next entry' },
		{ keys: ['k'], description: 'Previous entry' },
		{ keys: ['m'], description: 'Mark read / unread' },
		{ keys: ['s'], description: 'Save / unsave' },
		{ keys: ['o'], description: 'Open entry' },
		{ keys: ['v'], description: 'View original' },
		{ keys: ['/'], description: 'Search' },
		{ keys: ['?'], description: 'Toggle this help' },
		{ keys: ['Esc'], description: 'Close modals / clear selection' }
	] as const;

	function onBackdropClick(ev: MouseEvent) {
		// Only close when the click started on the backdrop itself,
		// not when it bubbles from an inner element.
		if (ev.target === ev.currentTarget) hotkeysModal.hide();
	}

	function onKeydown(ev: KeyboardEvent) {
		if (ev.key === 'Escape') {
			ev.preventDefault();
			hotkeysModal.hide();
		}
	}
</script>

<svelte:window onkeydown={hotkeysModal.open ? onKeydown : null} />

{#if hotkeysModal.open}
	<div
		class="backdrop"
		role="dialog"
		aria-modal="true"
		aria-labelledby="hotkeys-title"
		tabindex="-1"
		onclick={onBackdropClick}
		onkeydown={onKeydown}
	>
		<div class="panel" role="document">
			<header>
				<h2 id="hotkeys-title">Keyboard shortcuts</h2>
				<button
					type="button"
					class="close"
					aria-label="Close"
					onclick={() => hotkeysModal.hide()}>×</button
				>
			</header>
			<dl>
				{#each SHORTCUTS as s (s.keys.join('+'))}
					<div class="row">
						<dt>
							{#each s.keys as k (k)}
								<kbd>{k}</kbd>
							{/each}
						</dt>
						<dd>{s.description}</dd>
					</div>
				{/each}
			</dl>
		</div>
	</div>
{/if}

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		background: color-mix(in srgb, var(--ink) 32%, transparent);
		z-index: 100;
		padding: 24px;
	}
	.panel {
		background: var(--bg);
		color: var(--ink);
		border: 1px solid var(--rule);
		border-radius: 6px;
		padding: 20px 24px 24px;
		min-width: 320px;
		max-width: 440px;
		width: 100%;
		box-shadow: 0 12px 40px color-mix(in srgb, var(--ink) 18%, transparent);
		font-family: var(--sans);
	}
	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 14px;
	}
	h2 {
		font-family: var(--sans);
		font-size: 14px;
		font-weight: 600;
		margin: 0;
		color: var(--ink);
	}
	.close {
		font-size: 20px;
		line-height: 1;
		border: none;
		background: transparent;
		color: var(--ink-3);
		cursor: pointer;
		padding: 4px 8px;
		border-radius: 4px;
	}
	.close:hover {
		color: var(--ink);
		background: var(--surface);
	}
	dl {
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 12px;
		font-size: 13px;
	}
	dt {
		display: flex;
		gap: 4px;
		min-width: 88px;
	}
	dd {
		margin: 0;
		color: var(--ink-2);
	}
	kbd {
		font-family: var(--mono);
		font-size: 11px;
		padding: 2px 6px;
		border: 1px solid var(--rule);
		border-bottom-width: 2px;
		border-radius: 3px;
		color: var(--ink-2);
		background: var(--surface);
	}
</style>
