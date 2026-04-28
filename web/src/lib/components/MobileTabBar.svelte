<script lang="ts">
	// iOS-style bottom tab bar. Routes for /saved, /search, /settings
	// land in later plans (12, 14); we render the affordances now so
	// the navigation surface matches the design handoff on day one.
	type TabId = 'unread' | 'saved' | 'search' | 'settings';

	// Routes outside the four tabs (history, /feeds/*, the reader)
	// pass no `active` prop so no tab is highlighted — better than
	// faking a sibling highlight.
	let { active }: { active?: TabId } = $props();

	const tabs: ReadonlyArray<{ id: TabId; label: string; href: string }> = [
		{ id: 'unread', label: 'Unread', href: '/' },
		{ id: 'saved', label: 'Saved', href: '/saved' },
		{ id: 'search', label: 'Search', href: '/search' },
		{ id: 'settings', label: 'Settings', href: '/settings' }
	];
</script>

<nav class="m-tabbar" aria-label="Primary">
	{#each tabs as t (t.id)}
		<a class="tab" class:active={active === t.id} href={t.href}>
			<span>{t.label}</span>
		</a>
	{/each}
</nav>

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
