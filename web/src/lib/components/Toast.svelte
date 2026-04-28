<script lang="ts">
	// Single-message toast surface. Reads the rune store in
	// `$lib/toast.svelte` and renders the active message (if any) at
	// the bottom-centre of the viewport. The store auto-dismisses, so
	// this component carries no timer logic of its own.

	import { toast } from '$lib/toast.svelte';
</script>

{#if toast.current}
	<div
		class={['toast', `toast--${toast.current.kind}`]}
		role="status"
		aria-live="polite"
	>
		<span>{toast.current.message}</span>
		<button
			type="button"
			class="toast-close"
			onclick={() => toast.dismiss()}
			aria-label="Dismiss"
		>
			×
		</button>
	</div>
{/if}

<style>
	.toast {
		position: fixed;
		bottom: 24px;
		left: 50%;
		transform: translateX(-50%);
		z-index: 1000;
		display: inline-flex;
		align-items: center;
		gap: 12px;
		padding: 10px 14px;
		background: var(--bg-elev);
		color: var(--ink);
		border: 1px solid var(--rule);
		border-radius: 6px;
		font-size: 13px;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
	}
	.toast--error {
		border-color: var(--accent-warn, #c33);
	}
	.toast-close {
		background: none;
		border: none;
		color: var(--ink-3);
		font-size: 16px;
		line-height: 1;
		cursor: pointer;
		padding: 2px 6px;
	}
</style>
