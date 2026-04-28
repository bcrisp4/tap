<script lang="ts">
	// Plan 18 / T8 — mobile "More" slide-in panel.
	//
	// Mirrors the desktop sidebar's tail: the FEEDS list and the small
	// icon strip at the bottom (theme cycle, hotkeys help, settings).
	// Opened from the MobileTabBar's "More" entry; closed by tapping
	// the scrim or pressing Escape.
	import { useFeeds } from '$api/queries';
	import FeedIcon from './FeedIcon.svelte';
	import { cycleTheme } from '$lib/theme.svelte';
	import { hotkeysModal } from '$lib/hotkeys-modal.svelte';

	let { open, onClose }: { open: boolean; onClose: () => void } = $props();

	const feeds = useFeeds();

	function onKey(ev: KeyboardEvent) {
		if (ev.key === 'Escape' && open) {
			ev.preventDefault();
			onClose();
		}
	}
</script>

<svelte:window onkeydown={onKey} />

{#if open}
	<button
		type="button"
		class="more-scrim"
		aria-label="Close more menu"
		onclick={onClose}
	></button>
	<div class="more-panel" aria-label="More" role="dialog" aria-modal="true">
		<header class="more-head">
			<span class="more-title">More</span>
			<button type="button" class="more-close" aria-label="Close" onclick={onClose}>
				<svg width="18" height="18" viewBox="0 0 16 16" aria-hidden="true">
					<path
						d="M3 3l10 10M13 3 3 13"
						stroke="currentColor"
						stroke-width="1.5"
						stroke-linecap="round"
					/>
				</svg>
			</button>
		</header>

		<div class="group-title">Feeds</div>
		<a class="more-row add-feed" href="/feeds/add" onclick={onClose}>
			<span class="add-glyph" aria-hidden="true">+</span>
			<span>Add feed</span>
		</a>
		{#each feeds.data?.data ?? [] as f (f.id)}
			<a href={'/feeds/' + f.id} class="more-row feed-row" onclick={onClose}>
				<FeedIcon feed={f} />
				<span class="name">{f.title}</span>
			</a>
		{/each}

		<div class="footer-spacer"></div>

		<div class="more-icons">
			<button type="button" class="icon-btn" aria-label="Cycle theme" onclick={cycleTheme}>
				<svg width="18" height="18" viewBox="0 0 16 16" aria-hidden="true">
					<circle cx="8" cy="8" r="6.25" fill="none" stroke="currentColor" stroke-width="1.25" />
					<path d="M8 1.75 A6.25 6.25 0 0 1 8 14.25 Z" fill="currentColor" />
				</svg>
			</button>
			<button
				type="button"
				class="icon-btn"
				aria-label="Keyboard shortcuts"
				onclick={() => {
					hotkeysModal.toggle();
					onClose();
				}}
			>
				<svg width="18" height="18" viewBox="0 0 16 16" aria-hidden="true">
					<rect
						x="1.5"
						y="3.75"
						width="13"
						height="8.5"
						rx="1.5"
						fill="none"
						stroke="currentColor"
						stroke-width="1.1"
					/>
					<circle cx="4.25" cy="6.5" r="0.75" fill="currentColor" />
					<circle cx="8" cy="6.5" r="0.75" fill="currentColor" />
					<circle cx="11.75" cy="6.5" r="0.75" fill="currentColor" />
					<rect x="4" y="9" width="8" height="1.25" rx="0.5" fill="currentColor" />
				</svg>
			</button>
			<a class="icon-btn" href="/settings" aria-label="Settings" onclick={onClose}>
				<svg width="18" height="18" viewBox="0 0 16 16" aria-hidden="true">
					<circle cx="8" cy="8" r="2.25" fill="none" stroke="currentColor" stroke-width="1.25" />
					<path
						d="M8 1.5v2.25 M8 12.25v2.25 M1.5 8h2.25 M12.25 8h2.25 M3.4 3.4l1.6 1.6 M11 11l1.6 1.6 M3.4 12.6l1.6-1.6 M11 5l1.6-1.6"
						stroke="currentColor"
						stroke-width="1.1"
						stroke-linecap="round"
					/>
				</svg>
			</a>
		</div>
	</div>
{/if}

<style>
	.more-scrim {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.35);
		border: 0;
		padding: 0;
		cursor: pointer;
		z-index: 100;
	}
	.more-panel {
		position: fixed;
		right: 0;
		top: 0;
		bottom: 0;
		width: min(82vw, 320px);
		background: var(--bg);
		border-left: 1px solid var(--rule);
		display: flex;
		flex-direction: column;
		z-index: 101;
		overflow-y: auto;
		padding: 14px 0 calc(env(safe-area-inset-bottom, 0px) + 12px);
	}
	.more-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 4px 18px 12px;
		border-bottom: 1px solid var(--rule);
	}
	.more-title {
		font-family: var(--sans);
		font-weight: 600;
		font-size: 16px;
		color: var(--ink);
	}
	.more-close {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 44px;
		min-height: 44px;
		background: transparent;
		border: 0;
		color: var(--ink-2);
		cursor: pointer;
		touch-action: manipulation;
	}
	.group-title {
		font-family: var(--mono);
		font-size: 10px;
		letter-spacing: 0.12em;
		text-transform: uppercase;
		color: var(--ink-3);
		padding: 16px 18px 6px;
	}
	.more-row {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 12px 18px;
		min-height: 44px;
		font-family: var(--sans);
		font-size: 14px;
		color: var(--ink);
		text-decoration: none;
		touch-action: manipulation;
	}
	.more-row .name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex: 1;
	}
	.add-feed .add-glyph {
		display: inline-grid;
		place-items: center;
		width: 18px;
		height: 18px;
		font-family: var(--mono);
		color: var(--ink-3);
	}
	.footer-spacer {
		flex: 1;
		min-height: 12px;
	}
	.more-icons {
		display: flex;
		gap: 8px;
		padding: 12px 18px;
		border-top: 1px solid var(--rule);
	}
	.icon-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 44px;
		min-height: 44px;
		border: none;
		border-radius: 4px;
		background: transparent;
		color: var(--ink-3);
		cursor: pointer;
		text-decoration: none;
		padding: 0;
		touch-action: manipulation;
	}
	@media (hover: hover) {
		.more-row:hover {
			background: var(--bg-soft);
		}
		.icon-btn:hover {
			color: var(--ink);
		}
	}
</style>
