<script lang="ts">
	import type { Entry } from '$api/types';

	// Desktop list rail — the slim 280-px column between the sidebar and
	// the reader pane that lets you skim adjacent unread entries without
	// leaving the article. Selected row gets a Klein accent strip on the
	// left edge; read rows fade to ink-3 with an empty dot.
	//
	// `collapsed` swaps the full list for a thin "back to unread" rail
	// while keeping `entries` in scope so swipe-to-navigate can read
	// the live sibling IDs from the prop.
	let {
		entries,
		selectedId,
		collapsed = false,
		onBack
	}: {
		entries: Entry[];
		selectedId: number;
		collapsed?: boolean;
		onBack?: () => void;
	} = $props();
</script>

{#if collapsed}
	<aside
		class="reader-rail-collapsed"
		aria-label="Back to unread"
		data-sibling-count={entries.length}
	>
		<button
			type="button"
			class="rail-back mono"
			onclick={onBack}
			title="Back to unread (Esc)"
			aria-label="Back to unread list"
		>
			<svg
				width="14"
				height="14"
				viewBox="0 0 16 16"
				fill="none"
				stroke="currentColor"
				stroke-width="1.5"
				stroke-linecap="round"
				stroke-linejoin="round"
				aria-hidden="true"
			>
				<path d="M10 3 5 8l5 5" />
			</svg>
		</button>
	</aside>
{:else}
	<aside class="reader-rail" aria-label="Adjacent unread entries">
		<div class="rail-head mono">UNREAD · {entries.length}</div>
		{#each entries as e (e.id)}
			<a
				class={['rail-row', { 'is-selected': e.id === selectedId, 'is-read': e.read }]}
				href={'/entry/' + e.id}
				aria-current={e.id === selectedId ? 'page' : undefined}
			>
				<span class="rail-dot" aria-hidden="true"></span>
				<div class="rail-text">
					<div class="rail-title">{e.title}</div>
					<div class="rail-meta sans">{e.reading_time} min</div>
				</div>
			</a>
		{/each}
	</aside>
{/if}

<style>
	.reader-rail {
		width: 280px;
		border-right: 1px solid var(--rule);
		background: var(--bg-soft);
		overflow-y: auto;
		flex-shrink: 0;
	}
	.rail-head {
		padding: 18px 20px 12px;
		font-size: 10px;
		letter-spacing: 0.12em;
		color: var(--ink-3);
	}
	.rail-row {
		position: relative;
		display: block;
		padding: 12px 20px 12px 32px;
		border-bottom: 1px solid var(--rule);
		text-decoration: none;
		color: inherit;
	}
	.rail-row:hover {
		background: var(--bg);
	}
	.rail-row.is-selected {
		background: var(--bg);
	}
	.rail-row.is-selected::before {
		content: '';
		position: absolute;
		left: 0;
		top: 0;
		bottom: 0;
		width: 2px;
		background: var(--accent);
	}
	.rail-dot {
		position: absolute;
		left: 18px;
		top: 18px;
		width: 5px;
		height: 5px;
		border-radius: 50%;
		background: var(--accent);
	}
	.rail-row.is-read .rail-dot {
		background: transparent;
		border: 1px solid var(--ink-4);
	}
	.rail-title {
		font-family: var(--serif);
		font-size: 14px;
		line-height: 1.35;
		color: var(--ink);
		font-weight: 500;
		margin-bottom: 4px;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.rail-row.is-read .rail-title {
		color: var(--ink-3);
		font-weight: 400;
	}
	.rail-meta {
		font-size: 11px;
		color: var(--ink-3);
	}

	/* Collapsed rail: 56-px back-to-unread strip. Mirrors the full rail's
	   bg-soft surface + ruled right edge so the focus-mode reader still
	   feels framed, just tighter. */
	.reader-rail-collapsed {
		width: 56px;
		border-right: 1px solid var(--rule);
		background: var(--bg-soft);
		flex-shrink: 0;
		display: flex;
		justify-content: center;
		padding-top: 14px;
	}
	.rail-back {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: 4px;
		color: var(--ink-2);
	}
	.rail-back:hover {
		color: var(--ink);
		background: var(--bg);
	}
</style>
