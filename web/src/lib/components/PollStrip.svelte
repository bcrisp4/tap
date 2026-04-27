<script lang="ts">
	// Live status strip: pulsing accent dot + poller activity counters.
	// `useStatus` polls /system/status every 5s (configured in queries.ts),
	// so this stays in sync without any extra wiring.
	import { useStatus, useFeeds } from '$api/queries';

	const status = useStatus();
	const feeds = useFeeds();
</script>

<div class="poll-strip mono">
	<span class="pulse" aria-hidden="true"></span>
	<span>
		POLLER · {status.data?.run_state.active_polls ?? 0} active · {feeds.data?.pagination
			.total ?? 0} feeds tracked
	</span>
	<span class="spacer"></span>
	<span data-testid="poll-status">UPTIME · {status.data?.uptime_seconds ?? 0}s</span>
</div>

<style>
	.poll-strip {
		font-size: 10px;
		color: var(--ink-3);
		letter-spacing: 0.04em;
		padding: 6px 24px;
		border-bottom: 1px solid var(--rule);
		background: var(--bg-soft);
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.spacer {
		flex: 1;
	}
	.pulse {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--accent);
		animation: tap-pulse 2.4s ease-in-out infinite;
		flex-shrink: 0;
	}
	/* tap-pulse keyframes ship globally from tokens/tap-tokens.css; we
	   re-declare here so the component is self-contained for shadow-DOM
	   embedders / future SSR migrations. */
	@keyframes tap-pulse {
		0%,
		100% {
			opacity: 1;
			transform: scale(1);
		}
		50% {
			opacity: 0.35;
			transform: scale(0.8);
		}
	}
</style>
