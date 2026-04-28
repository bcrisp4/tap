<script lang="ts">
	// Desktop reader header: a single-row strip with a left "back" affordance
	// and a right cluster of mono-style action buttons. The mono typeface
	// here is a non-negotiable visual rule per the design handoff — these
	// buttons must render in JetBrains Mono so they read as utility chrome,
	// not as body content. Don't "fix" the casing or the family.
	//
	// `read` defaults true because the reader auto-marks on open; the
	// route always passes a real value but the safer default is the one
	// that matches first-paint reality.
	let {
		read = true,
		saved = false,
		entryURL = null,
		onBack,
		onToggleRead,
		onToggleSaved
	}: {
		read?: boolean;
		saved?: boolean;
		entryURL?: string | null;
		onBack: () => void;
		onToggleRead: () => void;
		onToggleSaved: () => void;
	} = $props();
</script>

<div class="reader-header">
	<button class="reader-back mono" onclick={onBack} title="Back to unread (Esc)">
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
		<span>UNREAD</span>
	</button>
	<div class="reader-actions">
		<button
			class="reader-action mono"
			onclick={onToggleRead}
			title="Toggle read (m)"
			aria-pressed={read}
		>
			<svg
				width="14"
				height="14"
				viewBox="0 0 16 16"
				fill={read ? 'currentColor' : 'none'}
				stroke="currentColor"
				stroke-width="1.5"
				aria-hidden="true"
			>
				<circle cx="8" cy="8" r="3.5" />
			</svg>
			<span>{read ? 'MARK UNREAD' : 'MARK READ'}</span>
		</button>
		<button
			class={['reader-action', 'mono', { 'is-saved': saved }]}
			onclick={onToggleSaved}
			title="Toggle saved (s)"
			aria-pressed={saved}
		>
			<svg
				width="14"
				height="14"
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
			<span>{saved ? 'SAVED' : 'SAVE'}</span>
		</button>
		{#if entryURL}
			<a
				class="reader-action mono"
				href={entryURL}
				target="_blank"
				rel="noreferrer noopener"
				title="View original (v)"
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
					<path d="M9 3h4v4M13 3 7 9M11 9.5V13H3V5h3.5" />
				</svg>
				<span>VIEW ORIGINAL</span>
			</a>
		{/if}
	</div>
</div>

<style>
	.reader-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 14px 28px;
		border-bottom: 1px solid var(--rule);
		background: var(--bg);
		position: sticky;
		top: 0;
		z-index: 4;
	}
	.reader-back,
	.reader-action {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		font-size: 11px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--ink-2);
		border-radius: 4px;
		text-decoration: none;
	}
	.reader-back {
		padding: 6px 10px 6px 4px;
	}
	.reader-back:hover,
	.reader-action:hover {
		color: var(--ink);
		background: var(--bg-soft);
	}
	.reader-action.is-saved {
		color: var(--accent);
	}
	.reader-actions {
		display: flex;
		gap: 4px;
	}
</style>
