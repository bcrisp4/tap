<script lang="ts">
	// Presentation-only image lightbox: parent owns open state via `src`
	// (null hides). Esc, backdrop click, or the close button dismisses.
	let {
		src,
		alt = '',
		onClose
	}: {
		src: string | null;
		alt?: string;
		onClose: () => void;
	} = $props();

	const open = $derived(src !== null);

	let priorFocus: HTMLElement | null = null;
	let backdropEl: HTMLDivElement | null = $state(null);

	$effect(() => {
		if (!open) return;
		// Capture focus so we can restore it on close — otherwise the
		// closing dialog leaves focus on document.body, restarting the
		// next keyboard Tab at the top of the page.
		priorFocus = document.activeElement as HTMLElement | null;
		// Defer one frame so the {#if open} insertion has hit the DOM.
		requestAnimationFrame(() => backdropEl?.focus());
		return () => {
			priorFocus?.focus();
			priorFocus = null;
		};
	});

	function onKey(ev: KeyboardEvent) {
		if (!open) return;
		if (ev.key === 'Escape') {
			ev.preventDefault();
			onClose();
		}
	}

	function onBackdropClick(ev: MouseEvent) {
		if (ev.target === ev.currentTarget) onClose();
	}

	function onBackdropKey(ev: KeyboardEvent) {
		// Keyboard parity for the click-to-dismiss backdrop.
		if (ev.target !== ev.currentTarget) return;
		if (ev.key === 'Enter' || ev.key === ' ') {
			ev.preventDefault();
			onClose();
		}
	}
</script>

<svelte:window onkeydown={onKey} />

{#if open}
	<!-- role=dialog announces a modal context, but we intentionally do
	     NOT trap focus — the reader body is read-only chrome and Esc
	     should bounce focus back where it was. tabindex=-1 makes the
	     dialog programmatically focusable without joining the tab order. -->
	<div
		bind:this={backdropEl}
		class="lightbox-backdrop"
		role="dialog"
		aria-modal="true"
		aria-label="Image preview"
		tabindex="-1"
		onclick={onBackdropClick}
		onkeydown={onBackdropKey}
	>
		<button
			type="button"
			class="lightbox-close mono"
			onclick={onClose}
			aria-label="Close image preview"
			title="Close (Esc)"
		>
			<svg
				width="14"
				height="14"
				viewBox="0 0 16 16"
				fill="none"
				stroke="currentColor"
				stroke-width="1.5"
				stroke-linecap="round"
				aria-hidden="true"
			>
				<path d="M3 3 L13 13 M13 3 L3 13" />
			</svg>
			<span>CLOSE</span>
		</button>
		<!-- Visual no-op button: absorbs clicks on the image so they
		     don't bubble to the dismiss-on-backdrop handler, and lets
		     the figure host keyboard handlers without violating the
		     non-interactive-element-with-handler a11y rule. -->
		<button
			type="button"
			class="lightbox-figure"
			onclick={(e) => e.stopPropagation()}
			aria-label="Image"
		>
			<img class="lightbox-image" {src} {alt} />
		</button>
	</div>
{/if}

<style>
	.lightbox-backdrop {
		position: fixed;
		inset: 0;
		z-index: 50;
		background: rgba(8, 8, 12, 0.88);
		display: grid;
		place-items: center;
		padding: 56px;
		cursor: zoom-out;
	}
	.lightbox-figure {
		/* Strip default button chrome so the user sees only the image. */
		margin: 0;
		padding: 0;
		border: 0;
		background: transparent;
		max-width: 100%;
		max-height: 100%;
		display: flex;
		justify-content: center;
		align-items: center;
		cursor: default;
	}
	.lightbox-image {
		max-width: 100%;
		max-height: 100%;
		width: auto;
		height: auto;
		object-fit: contain;
		display: block;
		/* Hairline keeps very dark images legible on the dim backdrop. */
		box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.06);
	}
	.lightbox-close {
		position: absolute;
		top: 18px;
		right: 18px;
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		font-size: 11px;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: rgba(255, 255, 255, 0.78);
		background: transparent;
		border: 1px solid rgba(255, 255, 255, 0.18);
		border-radius: 4px;
		cursor: pointer;
	}
	.lightbox-close:hover {
		color: #fff;
		border-color: rgba(255, 255, 255, 0.4);
	}

	/* Narrow viewports: collapse the close button to icon-only. */
	@media (max-width: 720px) {
		.lightbox-backdrop {
			padding: 16px;
		}
		.lightbox-close span {
			display: none;
		}
	}
</style>
