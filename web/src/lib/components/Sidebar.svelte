<script lang="ts">
	import Wordmark from '$brand/Wordmark.svelte';

	// Plan 11 placeholder. The full sidebar — feed list, category
	// folders, unread counts, settings cog — lands in Plan 10. This
	// component exists so the desktop reader layout has a stable left
	// rail to sit against; Plan 10's Sidebar will replace it wholesale.
	let { active = 'unread' }: { active?: string } = $props();
</script>

<aside class="tap-sidebar" data-active={active}>
	<header class="tap-sidebar-head">
		<Wordmark />
	</header>
	<nav class="tap-sidebar-nav mono">
		<a class={['tap-nav-item', { 'is-active': active === 'unread' }]} href="/">UNREAD</a>
		<a class={['tap-nav-item', { 'is-active': active === 'saved' }]} href="/saved">SAVED</a>
	</nav>
</aside>

<style>
	.tap-sidebar {
		width: 220px;
		flex-shrink: 0;
		border-right: 1px solid var(--rule);
		background: var(--bg);
		display: flex;
		flex-direction: column;
	}
	.tap-sidebar-head {
		padding: 22px 22px 18px;
	}
	.tap-sidebar-nav {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 6px 12px;
	}
	.tap-nav-item {
		padding: 8px 10px;
		font-size: 11px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--ink-3);
		border-radius: 4px;
		text-decoration: none;
	}
	.tap-nav-item:hover {
		color: var(--ink-2);
		background: var(--bg-soft);
	}
	.tap-nav-item.is-active {
		color: var(--ink);
	}
</style>
