<script lang="ts">
	// Mobile bottom action strip: three slot grid for read / save /
	// view-original. Sans family (12 px) keeps the labels readable next
	// to the icons on small screens — the desktop reader's mono labels
	// would be too tight here. Honors safe-area-inset-bottom so the
	// home-indicator on iPhones doesn't eat into the targets.
	let {
		read = false,
		saved = false,
		entryURL = null,
		onToggleRead,
		onToggleSaved
	}: {
		read?: boolean;
		saved?: boolean;
		entryURL?: string | null;
		onToggleRead: () => void;
		onToggleSaved: () => void;
	} = $props();
</script>

<div class="m-reader-footbar">
	<button class="m-foot-btn sans" onclick={onToggleRead} aria-pressed={read}>
		<svg
			width="20"
			height="20"
			viewBox="0 0 16 16"
			fill={read ? 'currentColor' : 'none'}
			stroke="currentColor"
			stroke-width="1.5"
			stroke-linecap="round"
			stroke-linejoin="round"
			aria-hidden="true"
		>
			<circle cx="8" cy="8" r="3.5" />
		</svg>
		<span>{read ? 'Mark unread' : 'Mark read'}</span>
	</button>
	<button
		class={['m-foot-btn', 'sans', { 'is-saved': saved }]}
		onclick={onToggleSaved}
		aria-pressed={saved}
	>
		<svg
			width="20"
			height="20"
			viewBox="0 0 16 16"
			fill={saved ? 'currentColor' : 'none'}
			stroke="currentColor"
			stroke-width="1.5"
			stroke-linejoin="round"
			stroke-linecap="round"
			aria-hidden="true"
		>
			<path d="M4 2.5h8v11l-4-3-4 3z" />
		</svg>
		<span>{saved ? 'Saved' : 'Save'}</span>
	</button>
	{#if entryURL}
		<a class="m-foot-btn sans" href={entryURL} target="_blank" rel="noreferrer noopener">
			<svg
				width="20"
				height="20"
				viewBox="0 0 16 16"
				fill="none"
				stroke="currentColor"
				stroke-width="1.5"
				stroke-linecap="round"
				stroke-linejoin="round"
				aria-hidden="true"
			>
				<path d="M9 3h4v4M13 3 7 9M11 9.5V13H3V5h3.5" />
			</svg>
			<span>Original</span>
		</a>
	{:else}
		<span class="m-foot-btn sans is-disabled" aria-hidden="true"></span>
	{/if}
</div>

<style>
	.m-reader-footbar {
		display: grid;
		grid-template-columns: 1fr 1fr 1fr;
		border-top: 1px solid var(--rule);
		padding: 8px 8px calc(env(safe-area-inset-bottom, 0px) + 12px);
		background: var(--bg);
		gap: 4px;
	}
	.m-foot-btn {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 4px;
		padding: 8px 0;
		font-size: 11px;
		font-weight: 500;
		color: var(--ink-2);
		min-width: 44px;
		min-height: 52px;
		border-radius: 8px;
		text-decoration: none;
		background: transparent;
		border: 0;
		cursor: pointer;
		font-family: inherit;
		touch-action: manipulation;
	}
	.m-foot-btn:active {
		background: var(--bg-soft);
	}
	.m-foot-btn.is-saved {
		color: var(--accent);
	}
	.m-foot-btn.is-disabled {
		pointer-events: none;
	}
</style>
