<script lang="ts">
	// Mobile reader top strip: back affordance, a 2-px scroll-progress
	// bar (the one place in the UI that uses motion to communicate
	// state), and a quick-save action. The progress bar is the single
	// signal that you're inside the reader rather than a list view; it
	// also tells you how much article is left at a glance.
	let {
		progress = 0,
		saved = false,
		onBack,
		onSave
	}: {
		progress?: number;
		saved?: boolean;
		onBack: () => void;
		onSave: () => void;
	} = $props();

	const pct = $derived(Math.max(0, Math.min(1, progress)) * 100);
</script>

<div class="m-reader-topbar">
	<button class="m-back" onclick={onBack} aria-label="Back to unread">
		<svg
			width="18"
			height="18"
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
	<div
		class="m-progress"
		role="progressbar"
		aria-valuenow={Math.round(pct)}
		aria-valuemin="0"
		aria-valuemax="100"
		aria-label="Reading progress"
	>
		<span class="m-progress-bar" style:width={pct + '%'}></span>
	</div>
	<button
		class={['m-action', { 'is-saved': saved }]}
		onclick={onSave}
		aria-label={saved ? 'Unsave entry' : 'Save entry'}
		aria-pressed={saved}
	>
		<svg
			width="18"
			height="18"
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
	</button>
</div>

<style>
	.m-reader-topbar {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 10px 14px;
		border-bottom: 1px solid var(--rule);
		background: var(--bg);
	}
	.m-back,
	.m-action {
		width: 40px;
		height: 40px;
		display: grid;
		place-items: center;
		color: var(--ink-2);
		border-radius: 8px;
		flex-shrink: 0;
	}
	.m-back:active,
	.m-action:active {
		background: var(--bg-soft);
	}
	.m-action.is-saved {
		color: var(--accent);
	}
	.m-progress {
		flex: 1;
		height: 2px;
		background: var(--rule);
		border-radius: 1px;
		overflow: hidden;
	}
	.m-progress-bar {
		display: block;
		height: 100%;
		background: var(--accent);
		transition: width 80ms ease-out;
	}
</style>
