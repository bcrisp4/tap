<script lang="ts">
	// Generic confirm modal. Plan 15 introduces this so Mark All Read
	// (and any future destructive op — feed-delete already gets one in
	// Plan 14, this consolidates the pattern) shares a single
	// affordance.
	//
	// API surface intentionally minimal: title, body, button labels,
	// and onConfirm/onCancel callbacks. The parent controls the open
	// rune; the modal renders nothing when closed. Esc and a
	// click-outside the dialog both fire onCancel.

	import { onMount } from 'svelte';

	let {
		open,
		title,
		message,
		confirmLabel = 'Confirm',
		cancelLabel = 'Cancel',
		danger = false,
		onConfirm,
		onCancel
	}: {
		open: boolean;
		title: string;
		message: string;
		confirmLabel?: string;
		cancelLabel?: string;
		danger?: boolean;
		onConfirm: () => void;
		onCancel: () => void;
	} = $props();

	let dialogEl = $state<HTMLDivElement | null>(null);

	// Focus the confirm button when the modal opens so Enter confirms.
	$effect(() => {
		if (open && dialogEl) {
			const btn = dialogEl.querySelector<HTMLButtonElement>('[data-autofocus]');
			btn?.focus();
		}
	});

	function onKey(ev: KeyboardEvent) {
		if (!open) return;
		if (ev.key === 'Escape') {
			ev.preventDefault();
			onCancel();
		}
	}

	function onBackdropClick(ev: MouseEvent) {
		// Only close when the user clicks the scrim itself, not when a
		// click bubbles up from a child of the dialog.
		if (ev.target === ev.currentTarget) {
			onCancel();
		}
	}

	onMount(() => {
		// Bind once at mount; the {#if} below mounts/unmounts the DOM
		// nodes but the listener should always be live so Esc works the
		// instant the modal opens.
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});
</script>

{#if open}
	<div
		class="scrim"
		role="presentation"
		onclick={onBackdropClick}
		data-testid="confirm-modal"
	>
		<div
			class="dialog"
			role="dialog"
			aria-modal="true"
			aria-labelledby="confirm-modal-title"
			bind:this={dialogEl}
		>
			<h2 id="confirm-modal-title" class="title">{title}</h2>
			<p class="message">{message}</p>
			<div class="actions">
				<button type="button" class="btn ghost" onclick={onCancel}>{cancelLabel}</button>
				<button
					type="button"
					class="btn primary"
					class:danger
					data-autofocus
					onclick={onConfirm}
				>
					{confirmLabel}
				</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.scrim {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.4);
		display: grid;
		place-items: center;
		z-index: 50;
	}
	.dialog {
		background: var(--bg);
		border: 1px solid var(--rule);
		border-radius: 6px;
		padding: 22px 24px 18px;
		min-width: 320px;
		max-width: 460px;
		box-shadow: 0 12px 40px rgba(0, 0, 0, 0.18);
	}
	.title {
		font-family: var(--serif);
		font-size: 17px;
		font-weight: 600;
		color: var(--ink);
		margin: 0 0 8px;
	}
	.message {
		font-family: var(--serif);
		font-size: 14px;
		line-height: 1.45;
		color: var(--ink-2);
		margin: 0 0 18px;
		text-wrap: pretty;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}
	.btn {
		font-family: var(--sans);
		font-size: 12.5px;
		padding: 6px 14px;
		border-radius: 4px;
		border: 1px solid var(--rule);
		background: var(--bg);
		color: var(--ink);
		cursor: pointer;
	}
	.btn:hover {
		background: var(--bg-soft);
	}
	.btn.ghost {
		color: var(--ink-2);
	}
	.btn.primary {
		background: var(--accent);
		color: var(--bg);
		border-color: var(--accent);
	}
	.btn.primary:hover {
		filter: brightness(1.05);
	}
	.btn.primary.danger {
		background: var(--warn, #c0392b);
		border-color: var(--warn, #c0392b);
	}
</style>
