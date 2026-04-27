<!--
	Footer chip strip listing the keyboard shortcuts wired in
	`$lib/keyboard.svelte.ts`. Rendered at the bottom of the desktop
	frame; hidden on mobile because there's no keyboard.
-->
<footer class="hints sans">
	<span><kbd>j</kbd> <kbd>k</kbd> navigate</span>
	<span><kbd>m</kbd> mark</span>
	<span><kbd>s</kbd> save</span>
	<span><kbd>o</kbd> open</span>
	<span><kbd>v</kbd> view original</span>
	<span class="spacer"></span>
	<span class="mono">tap · self-hosted</span>
</footer>

<style>
	.hints {
		border-top: 1px solid var(--rule);
		padding: 8px 24px;
		font-size: 11px;
		color: var(--ink-3);
		display: flex;
		gap: 14px;
		align-items: center;
		background: var(--bg);
	}
	.spacer {
		flex: 1;
	}
	kbd {
		font-family: var(--mono);
		font-size: 10px;
		padding: 1px 5px;
		border: 1px solid var(--rule);
		border-bottom-width: 2px;
		border-radius: 3px;
		color: var(--ink-2);
		background: var(--surface);
	}
</style>
