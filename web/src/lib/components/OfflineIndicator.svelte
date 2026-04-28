<script lang="ts">
	// Offline indicator. Visible only while the browser reports
	// `navigator.onLine === false` OR TanStack Query's `onlineManager`
	// reports paused. Plan 15 replaces the always-visible pulsing-blue
	// "poller is alive" dot with this component so the river stays
	// chrome-free in the common (online) case and the user gets a
	// clear signal — and the reassurance that mutations will sync —
	// when they're disconnected.
	//
	// We attach `online`/`offline` listeners and mirror onlineManager's
	// state. The two should agree; either flipping false drives this
	// component into its visible state.

	import { onMount } from 'svelte';
	import { onlineManager } from '@tanstack/svelte-query';

	let online = $state(true);

	onMount(() => {
		const apply = () => {
			// Both signals must be true to count as "online". If either
			// one says we're offline (browser event or onlineManager
			// paused), show the indicator.
			online = navigator.onLine && onlineManager.isOnline();
		};
		apply();
		window.addEventListener('online', apply);
		window.addEventListener('offline', apply);
		const unsub = onlineManager.subscribe(() => apply());
		return () => {
			window.removeEventListener('online', apply);
			window.removeEventListener('offline', apply);
			unsub();
		};
	});
</script>

{#if !online}
	<div class="offline mono" role="status" aria-live="polite">
		<span class="dot" aria-hidden="true"></span>
		<span class="copy">You're offline. Changes will sync when you reconnect.</span>
	</div>
{/if}

<style>
	.offline {
		position: fixed;
		top: 12px;
		right: 16px;
		z-index: 30;
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 6px 12px;
		font-size: 11px;
		letter-spacing: 0.04em;
		color: var(--ink-2);
		background: var(--bg-soft);
		border: 1px solid var(--rule);
		border-radius: 4px;
		box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04);
	}
	.dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--warn, #c47b1f);
		flex-shrink: 0;
	}
	.copy {
		white-space: nowrap;
	}
</style>
