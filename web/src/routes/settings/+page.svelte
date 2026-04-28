<script lang="ts">
	// /settings — appearance + system status + about. Three cards
	// stacked in the main column so each section breathes; the
	// theme/font runes already persist to localStorage (Plan 09), so
	// this page is essentially a presentation of those state machines.
	import { theme, type Theme, type Font } from '$lib/theme.svelte';
	import { isMobile } from '$lib/breakpoints.svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import TopBar from '$lib/components/TopBar.svelte';
	import StatsPanel from '$lib/components/StatsPanel.svelte';
	import MobileTopBar from '$lib/components/MobileTopBar.svelte';
	import MobileTabBar from '$lib/components/MobileTabBar.svelte';

	const themeOptions: ReadonlyArray<{ id: Theme; label: string; hint: string }> = [
		{ id: 'system', label: 'System', hint: 'follow OS preference' },
		{ id: 'light', label: 'Light', hint: 'paper white' },
		{ id: 'sepia', label: 'Sepia', hint: 'warm parchment' },
		{ id: 'dark', label: 'Dark', hint: 'midnight ink' }
	];

	const fontOptions: ReadonlyArray<{ id: Font; label: string; hint: string }> = [
		{ id: 'serif', label: 'Serif', hint: 'Source Serif 4' },
		{ id: 'sans', label: 'Sans', hint: 'Inter Tight' }
	];

	const mobile = $derived(isMobile());
</script>

<svelte:head>
	<title>Settings — tap</title>
</svelte:head>

{#snippet content()}
	<div class="settings-wrap">
		<section class="card">
			<header>
				<h2>Appearance</h2>
				<p class="hint">Theme and reading font. Persists locally.</p>
			</header>
			<div class="control-group" role="radiogroup" aria-label="Theme">
				<span class="label mono">Theme</span>
				<div class="chips">
					{#each themeOptions as opt (opt.id)}
						<button
							type="button"
							class="chip"
							class:active={theme.theme === opt.id}
							role="radio"
							aria-checked={theme.theme === opt.id}
							data-testid="theme-{opt.id}"
							onclick={() => theme.setTheme(opt.id)}
						>
							<span class="chip-label">{opt.label}</span>
							<span class="chip-hint mono">{opt.hint}</span>
						</button>
					{/each}
				</div>
			</div>
			<div class="control-group" role="radiogroup" aria-label="Reading font">
				<span class="label mono">Font</span>
				<div class="chips">
					{#each fontOptions as opt (opt.id)}
						<button
							type="button"
							class="chip"
							class:active={theme.font === opt.id}
							role="radio"
							aria-checked={theme.font === opt.id}
							data-testid="font-{opt.id}"
							onclick={() => theme.setFont(opt.id)}
						>
							<span class="chip-label">{opt.label}</span>
							<span class="chip-hint mono">{opt.hint}</span>
						</button>
					{/each}
				</div>
			</div>
		</section>

		<section class="card">
			<header>
				<h2>System</h2>
				<p class="hint">Live state from the running binary.</p>
			</header>
			<StatsPanel />
		</section>

		<section class="card">
			<header>
				<h2>About</h2>
				<p class="hint">A self-hosted river of unread.</p>
			</header>
			<p class="about">
				Tap is open source —
				<a href="https://github.com/bcrisp4/tap" target="_blank" rel="noopener noreferrer"
					>github.com/bcrisp4/tap</a
				>
				· Apache 2.0 · single binary · single SQLite file.
			</p>
		</section>
	</div>
{/snippet}

{#if mobile}
	<div class="tap is-mobile">
		<MobileTopBar label="settings" />
		<div class="m-wrap">{@render content()}</div>
		<MobileTabBar active="settings" />
	</div>
{:else}
	<div class="tap">
		<Sidebar />
		<div class="col">
			<TopBar title="Settings" />
			<div class="scroll">{@render content()}</div>
		</div>
	</div>
{/if}

<style>
	.tap {
		display: flex;
		height: 100vh;
		background: var(--bg);
		color: var(--ink);
		font-family: var(--serif);
	}
	.tap.is-mobile {
		flex-direction: column;
	}
	.col {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.scroll,
	.m-wrap {
		flex: 1;
		overflow-y: auto;
		min-height: 0;
	}

	.settings-wrap {
		max-width: 720px;
		margin: 0 auto;
		padding: 32px 28px 80px;
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.card {
		background: var(--surface);
		border: 1px solid var(--rule);
		border-radius: 8px;
		padding: 22px 24px 20px;
	}
	.card header {
		margin-bottom: 16px;
	}
	.card h2 {
		font-family: var(--serif);
		font-weight: 500;
		font-size: 22px;
		letter-spacing: -0.01em;
		margin: 0 0 4px;
		color: var(--ink);
	}
	.card .hint {
		font-family: var(--sans);
		font-size: 12.5px;
		color: var(--ink-3);
		margin: 0;
	}

	.control-group {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 12px 0 0;
	}
	.label {
		font-size: 10.5px;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		color: var(--ink-3);
	}
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}
	.chip {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 2px;
		padding: 8px 14px;
		font-family: var(--sans);
		background: var(--bg);
		border: 1px solid var(--rule);
		border-radius: 6px;
		color: var(--ink);
		cursor: pointer;
		transition:
			border-color 120ms ease,
			background 120ms ease;
	}
	.chip:hover {
		border-color: var(--ink-4);
		background: var(--bg-soft);
	}
	.chip.active {
		border-color: var(--accent);
		background: var(--accent-soft);
		color: var(--accent);
	}
	.chip-label {
		font-size: 13px;
		font-weight: 500;
	}
	.chip-hint {
		font-size: 10px;
		color: var(--ink-3);
		letter-spacing: 0.02em;
	}
	.chip.active .chip-hint {
		color: var(--accent);
		opacity: 0.85;
	}

	.about {
		font-family: var(--serif);
		font-size: 14px;
		line-height: 1.55;
		color: var(--ink-2);
		margin: 0;
	}
	.about a {
		color: var(--accent);
		text-decoration: none;
		border-bottom: 1px solid var(--accent-soft);
	}
	.about a:hover {
		border-bottom-color: var(--accent);
	}
</style>
