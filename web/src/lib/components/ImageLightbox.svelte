<script lang="ts">
	// Reader image lightbox: when the user clicks an inline article
	// image, this component overlays the page with the image at its
	// natural size on a dimmed backdrop. Esc, click on the backdrop
	// (anywhere outside the image), or the close button dismisses it.
	//
	// The component is presentation-only: it doesn't own the open
	// state. The parent (ReaderBody) tracks the current `src` and
	// `alt`; passing `src=null` hides the lightbox.
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

	// Tracks the element that had focus when the lightbox opened so
	// we can restore it when it closes. Without this, closing leaves
	// focus on document.body, which is jarring for keyboard users
	// (their next Tab starts at the top of the page).
	let priorFocus: HTMLElement | null = null;
	let backdropEl: HTMLDivElement | null = $state(null);

	$effect(() => {
		if (!open) return;
		// On open: capture and move focus to the backdrop. The
		// `tabindex="-1"` on the backdrop accepts programmatic focus
		// without joining the tab order, which is what makes the
		// `onkeydown` handler reachable for Enter/Space-to-dismiss
		// AND ensures screen readers announce the dialog reliably.
		priorFocus = document.activeElement as HTMLElement | null;
		// Defer one frame so the {#if open} insertion has hit the DOM.
		requestAnimationFrame(() => backdropEl?.focus());
		return () => {
			// On close: bounce focus back where it was. We deliberately
			// don't trap focus while open — the reader body is read-only
			// chrome, and a trap would make Esc-then-back jarring.
			priorFocus?.focus?.();
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
		// A click that lands on the backdrop element itself (not on
		// the image or close button) dismisses. Bubbled clicks from
		// the inner figure / image are stopped at the figure.
		if (ev.target === ev.currentTarget) onClose();
	}

	function onBackdropKey(ev: KeyboardEvent) {
		// Pairs the click-to-dismiss surface with Enter/Space activation
		// when the dialog is keyboard-focused (a11y: non-button click
		// surfaces still need keyboard parity).
		if (ev.target !== ev.currentTarget) return;
		if (ev.key === 'Enter' || ev.key === ' ') {
			ev.preventDefault();
			onClose();
		}
	}
</script>

<svelte:window onkeydown={onKey} />

{#if open}
	<!-- The wrapper is the click-outside surface. It's role=dialog so
	     screen readers announce a modal context, but we intentionally
	     do NOT trap focus: the reader body is read-only chrome and
	     pressing Esc should bounce focus straight back to where it was.
	     tabindex=-1 satisfies the dialog-must-be-focusable a11y rule
	     without inserting the dialog into the tab order. -->
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
		<!-- A button host for the image keeps focus management intact
		     and stops backdrop-click from bubbling without violating
		     the non-interactive-element-with-handler a11y rule. The
		     button itself is a visual no-op (transparent, no border,
		     cursor: default) so users see only the image. -->
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
		/* Visually neutral host — strips browser button chrome so the
		   user sees only the image. The element is a <button> only to
		   absorb clicks without bubbling to the dismiss-on-backdrop
		   handler and to satisfy keyboard-event a11y rules. */
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
		/* A 1-px hairline keeps the image edge legible against very
		   dark images on the dim backdrop. */
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

	/* On narrow viewports the close button collapses to icon-only so
	   it doesn't compete with the image. */
	@media (max-width: 720px) {
		.lightbox-backdrop {
			padding: 16px;
		}
		.lightbox-close span {
			display: none;
		}
	}
</style>
