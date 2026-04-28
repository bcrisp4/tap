<script module lang="ts">
	// Render an absolute date in the user's locale ("April 26, 2026"). The
	// reader prefers `published_at` (the feed's claimed date) and falls
	// back to `created_at` (when Tap first ingested the entry).
	export function formatDate(unix?: number | null): string {
		if (!unix) return '';
		const d = new Date(unix * 1000);
		return d.toLocaleDateString(undefined, {
			year: 'numeric',
			month: 'long',
			day: 'numeric'
		});
	}
</script>

<script lang="ts">
	import type { Entry, Feed } from '$api/types';
	import { swatchFor } from '$lib/components/swatch';
	import ImageLightbox from '$lib/components/ImageLightbox.svelte';

	let { entry, feed }: { entry: Entry; feed?: Feed } = $props();

	const date = $derived(formatDate(entry.published_at ?? entry.created_at));
	const sourceName = $derived(feed?.title ?? '—');
	const sourceURL = $derived(feed?.site_url ?? feed?.feed_url ?? '');

	// Lightbox state. Opening is driven by a SINGLE delegated click on
	// the article container; we never bind one listener per <img>,
	// because the body HTML is injected via {@html} and per-element
	// hooks would have to walk the DOM after every render.
	let lightboxSrc = $state<string | null>(null);
	let lightboxAlt = $state('');

	function onArticleClick(ev: MouseEvent) {
		const target = ev.target as HTMLElement | null;
		if (!target || target.tagName !== 'IMG') return;
		// If the image is wrapped in an <a>, let the link win — the
		// author probably linked the image deliberately to a higher-res
		// version or external destination.
		if (target.closest('a')) return;
		ev.preventDefault();
		const img = target as HTMLImageElement;
		lightboxSrc = img.currentSrc || img.src;
		lightboxAlt = img.alt ?? '';
	}

	function closeLightbox() {
		lightboxSrc = null;
		lightboxAlt = '';
	}
</script>

<!-- The article's only click handler is a delegated open-lightbox
     that fires only on <img> targets. The article itself is not
     focusable, so the equivalent keyboard activation lives on the
     image elements via Tab + Enter when alt text or wrapping links
     promote them to focusable. The whole-article click hook is the
     simplest sound place to delegate from given the body comes in
     via {@html}; we'd otherwise need a post-render walk. -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<article class="reader-body" data-testid="reader-body" onclick={onArticleClick}>
	<div class="reader-source">
		<span class="src-ico" style="background: {swatchFor(feed?.title ?? '')}"></span>
		<span class="src-name">{sourceName}</span>
		<span class="src-sep" aria-hidden="true"></span>
		<span class="src-url mono">{sourceURL}</span>
	</div>

	<h1 class="reader-title">{entry.title}</h1>

	<div class="reader-byline mono">
		{#if entry.author}
			<span>By {entry.author}</span>
			<span aria-hidden="true">·</span>
		{/if}
		<span>{date}</span>
		<span aria-hidden="true">·</span>
		<span>{entry.reading_time} min read</span>
	</div>

	<div class="reader-rule" aria-hidden="true">
		<span class="reader-rule-line"></span>
		<span class="reader-rule-dot"></span>
		<span class="reader-rule-line"></span>
	</div>

	{#if entry.summary}
		<p class="reader-lede">{entry.summary}</p>
	{/if}

	<div class="reader-content">
		{#if entry.content}
			<!-- Server-side bluemonday + Tap pre-pass already sanitise this
			     HTML (see design.md §5). Inline `{@html}` is the only way
			     to render the type-ramped article body faithfully. -->
			{@html entry.content}
		{:else}
			<p>No content available.</p>
		{/if}
	</div>

	<div class="reader-end" aria-hidden="true">
		<span class="reader-end-line"></span>
		<span class="reader-end-dot"></span>
		<span class="reader-end-line"></span>
	</div>
</article>

<ImageLightbox src={lightboxSrc} alt={lightboxAlt} onClose={closeLightbox} />

<style>
	.reader-body {
		max-width: 680px;
		margin: 0 auto;
		padding: 56px 56px 80px;
		width: 100%;
	}
	.reader-source {
		display: flex;
		align-items: center;
		gap: 8px;
		font-family: var(--sans);
		font-size: 12px;
		color: var(--ink-2);
		margin-bottom: 18px;
	}
	.reader-source .src-ico {
		width: 10px;
		height: 10px;
		border-radius: 2px;
		flex-shrink: 0;
	}
	.reader-source .src-name {
		color: var(--ink);
		font-weight: 500;
	}
	.reader-source .src-sep {
		width: 3px;
		height: 3px;
		border-radius: 50%;
		background: var(--ink-4);
	}
	.reader-source .src-url {
		font-size: 11px;
		color: var(--ink-3);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
	}

	.reader-title {
		font-family: var(--serif);
		font-size: 38px;
		line-height: 1.15;
		font-weight: 600;
		color: var(--ink);
		letter-spacing: -0.015em;
		margin: 0 0 16px;
		text-wrap: balance;
	}
	:global(.is-mobile) .reader-title {
		font-size: 28px;
	}

	.reader-byline {
		font-size: 11px;
		letter-spacing: 0.04em;
		color: var(--ink-3);
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
		margin-bottom: 28px;
		text-transform: uppercase;
	}
	.reader-byline span[aria-hidden] {
		color: var(--ink-4);
	}

	.reader-rule {
		display: flex;
		align-items: center;
		gap: 10px;
		margin: 0 0 32px;
	}
	.reader-rule-line {
		flex: 1;
		height: 1px;
		background: var(--rule);
	}
	.reader-rule-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--accent);
		flex-shrink: 0;
	}

	.reader-lede {
		font-family: var(--serif);
		font-size: 19px;
		line-height: 1.55;
		color: var(--ink);
		font-style: italic;
		margin: 0 0 28px;
		text-wrap: pretty;
	}
	:global(.is-mobile) .reader-lede {
		font-size: 17px;
	}

	/* Body type ramp — these :global rules target the sanitised HTML the
	   feed pipeline injects via {@html}; component-scoped selectors
	   would be stripped by Svelte's CSS scoper from injected DOM. */
	.reader-content :global(p) {
		font-family: var(--serif);
		font-size: 17px;
		line-height: 1.7;
		color: var(--ink);
		margin: 0 0 22px;
		text-wrap: pretty;
	}
	:global(.is-mobile) .reader-content :global(p) {
		font-size: 16px;
		line-height: 1.65;
	}

	.reader-content :global(h2) {
		font-family: var(--serif);
		font-size: 22px;
		line-height: 1.25;
		font-weight: 600;
		color: var(--ink);
		letter-spacing: -0.01em;
		margin: 40px 0 14px;
		text-wrap: pretty;
	}
	:global(.is-mobile) .reader-content :global(h2) {
		font-size: 20px;
		margin: 32px 0 12px;
	}

	.reader-content :global(h3) {
		font-family: var(--serif);
		font-size: 18px;
		line-height: 1.3;
		font-weight: 600;
		color: var(--ink);
		margin: 28px 0 10px;
	}

	.reader-content :global(pre) {
		font-family: var(--mono);
		font-size: 13px;
		line-height: 1.55;
		color: var(--ink-2);
		background: var(--bg-soft);
		border-left: 2px solid var(--accent);
		padding: 14px 16px;
		margin: 0 0 22px;
		overflow-x: auto;
		white-space: pre;
	}
	.reader-content :global(code) {
		font-family: var(--mono);
		font-size: 0.92em;
	}
	.reader-content :global(a) {
		color: var(--accent);
	}
	.reader-content :global(blockquote) {
		margin: 0 0 22px;
		padding-left: 20px;
		border-left: 2px solid var(--rule);
		color: var(--ink-2);
	}
	.reader-content :global(img),
	.reader-content :global(picture),
	.reader-content :global(video) {
		max-width: 100%;
		height: auto;
		display: block;
		margin: 0 auto 22px;
	}
	/* Clickable inline images — the delegated handler on the article
	   opens the lightbox. zoom-in is the most legible cursor cue for
	   "this image will enlarge"; images wrapped in <a> keep their
	   link cursor since the closest-<a> early return in onArticleClick
	   yields to the link. */
	.reader-content :global(img) {
		cursor: zoom-in;
	}
	.reader-content :global(a img) {
		cursor: pointer;
	}
	.reader-content :global(ul),
	.reader-content :global(ol) {
		font-family: var(--serif);
		font-size: 17px;
		line-height: 1.7;
		color: var(--ink);
		margin: 0 0 22px;
		padding-left: 28px;
	}
	.reader-content :global(li) {
		margin-bottom: 6px;
	}

	.reader-end {
		display: flex;
		align-items: center;
		gap: 10px;
		margin: 48px 0 18px;
	}
	.reader-end-line {
		flex: 1;
		height: 1px;
		background: var(--rule);
	}
	.reader-end-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--ink-4);
	}

	:global(.is-mobile) .reader-body {
		padding: 22px 22px 28px;
	}
	:global(.is-mobile) .reader-source {
		margin-bottom: 14px;
	}
	:global(.is-mobile) .reader-byline {
		margin-bottom: 22px;
	}
	:global(.is-mobile) .reader-rule {
		margin-bottom: 24px;
	}
</style>
