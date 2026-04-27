<script lang="ts">
	import Wordmark from '$brand/Wordmark.svelte';
	import { useStatus } from '$api/queries';

	const status = useStatus();
</script>

<main class="scaffold">
	<header>
		<Wordmark />
	</header>
	<section>
		{#if status.isLoading}
			<p class="mono" data-testid="status">loading status…</p>
		{:else if status.isError}
			<p class="mono" data-testid="status">api unreachable</p>
		{:else}
			<p class="mono" data-testid="status">
				tap {status.data?.version} · uptime {status.data?.uptime_seconds}s
			</p>
		{/if}
	</section>
</main>

<style>
	.scaffold {
		max-width: 680px;
		margin: 0 auto;
		padding: 56px 24px;
		display: flex;
		flex-direction: column;
		gap: 32px;
	}
	header {
		display: flex;
		align-items: baseline;
		gap: 12px;
	}
	.mono {
		font-family: var(--mono);
		font-size: 11px;
		color: var(--ink-3);
		letter-spacing: 0.04em;
	}
</style>
