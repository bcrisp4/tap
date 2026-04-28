<script lang="ts">
	// Plan 18 / T8 — mobile tab bar mirrors the desktop sidebar shape:
	// Unread, History, Saved, More. "More" is a slide-in panel
	// (MobileMorePanel) that exposes the FEEDS list and the small
	// theme/hotkeys/settings icon strip — the secondary chrome that
	// lives at the bottom of the desktop sidebar.
	//
	// Routes outside the listed tabs (the reader, /search, /feeds/*,
	// /settings) pass no `active` prop so no tab is highlighted —
	// better than faking a sibling highlight. Search is reachable via
	// the MobileTopBar magnifier icon.
	import MobileMorePanel from './MobileMorePanel.svelte';

	type TabId = 'unread' | 'history' | 'saved' | 'more';

	let { active }: { active?: TabId } = $props();

	let moreOpen = $state(false);

	type Tab =
		| { id: 'unread' | 'history' | 'saved'; label: string; href: string; icon: 'unread' | 'history' | 'saved' }
		| { id: 'more'; label: string; icon: 'more' };

	const tabs: ReadonlyArray<Tab> = [
		{ id: 'unread', label: 'Unread', href: '/', icon: 'unread' },
		{ id: 'history', label: 'History', href: '/history', icon: 'history' },
		{ id: 'saved', label: 'Saved', href: '/saved', icon: 'saved' },
		{ id: 'more', label: 'More', icon: 'more' }
	];
</script>

<nav class="m-tabbar" aria-label="Primary">
	{#each tabs as t (t.id)}
		{#if t.id === 'more'}
			<button
				type="button"
				class="tab"
				class:active={active === 'more' || moreOpen}
				aria-haspopup="dialog"
				aria-expanded={moreOpen}
				aria-label={t.label}
				onclick={() => (moreOpen = true)}
			>
				<svg width="20" height="20" viewBox="0 0 20 20" aria-hidden="true">
					<circle cx="4" cy="10" r="1.5" fill="currentColor" />
					<circle cx="10" cy="10" r="1.5" fill="currentColor" />
					<circle cx="16" cy="10" r="1.5" fill="currentColor" />
				</svg>
				<span>{t.label}</span>
			</button>
		{:else}
			<a class="tab" class:active={active === t.id} href={t.href} aria-label={t.label}>
				{#if t.icon === 'unread'}
					<!-- Filled circle = unread (matches the per-row read-dot
					     iconography on desktop). -->
					<svg width="20" height="20" viewBox="0 0 20 20" aria-hidden="true">
						<circle cx="10" cy="10" r="3" fill="currentColor" />
					</svg>
				{:else if t.icon === 'history'}
					<!-- Clock-face glyph: circle + hour/minute hands. -->
					<svg
						width="20"
						height="20"
						viewBox="0 0 20 20"
						fill="none"
						stroke="currentColor"
						stroke-width="1.5"
						stroke-linecap="round"
						aria-hidden="true"
					>
						<circle cx="10" cy="10" r="6.5" />
						<path d="M10 6v4l2.5 1.5" />
					</svg>
				{:else}
					<!-- Bookmark-pennant glyph for Saved. -->
					<svg
						width="20"
						height="20"
						viewBox="0 0 20 20"
						fill="none"
						stroke="currentColor"
						stroke-width="1.5"
						stroke-linejoin="round"
						stroke-linecap="round"
						aria-hidden="true"
					>
						<path d="M5 3h10v14l-5-3.5L5 17z" />
					</svg>
				{/if}
				<span>{t.label}</span>
			</a>
		{/if}
	{/each}
</nav>

<MobileMorePanel open={moreOpen} onClose={() => (moreOpen = false)} />

<style>
	.m-tabbar {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		border-top: 1px solid var(--rule);
		background: var(--bg);
		padding: 10px 0 calc(env(safe-area-inset-bottom, 0px) + 14px);
	}
	.tab {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 5px;
		font-family: var(--sans);
		font-size: 11px;
		font-weight: 500;
		color: var(--ink-3);
		padding: 6px 0 4px;
		min-width: 44px;
		min-height: 44px;
		position: relative;
		text-decoration: none;
		background: transparent;
		border: 0;
		cursor: pointer;
		touch-action: manipulation;
	}
	.tab.active {
		color: var(--ink);
	}
	.tab.active::before {
		content: '';
		position: absolute;
		top: 0;
		width: 4px;
		height: 4px;
		border-radius: 50%;
		background: var(--accent);
	}
</style>
