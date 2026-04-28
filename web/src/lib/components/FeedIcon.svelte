<script lang="ts">
	// Renders a feed's favicon when the backend has cached one (the
	// poller fetches + dedupes via the icons table; the API joins the
	// hash through). When no hash is available we fall back to a
	// deterministic colored square via swatchFor() so the row never
	// renders empty during the brief window between subscribing and
	// the first successful poll.
	import type { Feed } from '$api/types';
	import { swatchFor } from './EntryRow.svelte';

	let { feed }: { feed: Feed } = $props();
</script>

{#if feed.icon_hash}
	<img
		class="ico"
		src={'/api/v1/icons/' + feed.icon_hash}
		alt=""
		width="12"
		height="12"
		loading="lazy"
		decoding="async"
	/>
{:else}
	<span class="ico swatch" style="background: {swatchFor(feed.title)}" aria-hidden="true"></span>
{/if}

<style>
	.ico {
		width: 12px;
		height: 12px;
		border-radius: 2px;
		flex-shrink: 0;
		display: inline-block;
		object-fit: cover;
	}
</style>
