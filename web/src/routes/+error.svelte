<script lang="ts">
	// Branded error page. Renders for any unhandled status — most
	// commonly 404 (route does not exist) but also 5xx that bubble
	// out of a load function. SvelteKit hands us `page.status` and
	// `page.error.message`.
	import { page } from '$app/state';
	import Wordmark from '$brand/Wordmark.svelte';

	const status = $derived(page.status);
	const headline = $derived(
		status === 404 ? 'Lost in the feed.' : status >= 500 ? 'Tap stumbled.' : 'Something went sideways.'
	);
	const subhead = $derived(
		status === 404
			? "That route does not exist. Try one of the streams that does."
			: status >= 500
				? "We've logged it. Try again, or head back home."
				: 'An unexpected response slipped past the parser.'
	);
</script>

<svelte:head>
	<title>{status} — tap</title>
</svelte:head>

<main class="err">
	<header class="brand">
		<Wordmark />
	</header>

	<div class="card">
		<p class="status mono" data-testid="error-status">{status}</p>
		<h1 class="headline">{headline}</h1>
		<p class="subhead">{subhead}</p>
		{#if page.error?.message}
			<p class="detail mono">{page.error.message}</p>
		{/if}
		<div class="actions">
			<a class="btn primary" href="/">Home</a>
			<a class="btn ghost" href="/search">Search</a>
		</div>
	</div>
</main>

<style>
	.err {
		min-height: 100vh;
		background: var(--bg);
		color: var(--ink);
		font-family: var(--serif);
		display: grid;
		grid-template-rows: auto 1fr;
		padding: 24px 32px;
	}
	.brand {
		display: flex;
		align-items: center;
	}
	.brand :global(.wordmark) {
		font-size: 22px;
	}
	.card {
		align-self: center;
		justify-self: start;
		max-width: 540px;
		display: flex;
		flex-direction: column;
		gap: 14px;
		padding: 36px 0;
	}
	.status {
		font-size: 84px;
		line-height: 1;
		font-weight: 500;
		color: var(--accent);
		letter-spacing: -0.04em;
		margin: 0;
	}
	.headline {
		font-family: var(--serif);
		font-size: 38px;
		font-weight: 500;
		line-height: 1.1;
		letter-spacing: -0.015em;
		color: var(--ink);
		margin: 4px 0 0;
		text-wrap: balance;
	}
	.subhead {
		font-size: 17px;
		line-height: 1.5;
		color: var(--ink-2);
		margin: 0;
		max-width: 32em;
	}
	.detail {
		font-size: 11.5px;
		color: var(--ink-3);
		padding: 8px 12px;
		background: var(--bg-soft);
		border-left: 2px solid var(--ink-4);
		border-radius: 0 4px 4px 0;
		margin: 0;
		word-break: break-word;
	}
	.actions {
		display: flex;
		gap: 10px;
		padding-top: 12px;
	}
	.btn {
		font-family: var(--sans);
		font-size: 13px;
		font-weight: 500;
		padding: 10px 20px;
		border-radius: 5px;
		text-decoration: none;
		transition: filter 120ms ease, background 120ms ease, color 120ms ease;
	}
	.btn.primary {
		background: var(--accent);
		color: #fff;
		border: 1px solid var(--accent);
	}
	.btn.primary:hover {
		filter: brightness(1.05);
	}
	.btn.ghost {
		color: var(--ink-2);
		border: 1px solid var(--rule);
		background: transparent;
	}
	.btn.ghost:hover {
		color: var(--ink);
		border-color: var(--ink-4);
	}
</style>
