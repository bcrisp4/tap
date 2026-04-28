<script lang="ts">
	// System status panel — version, uptime, active polls, last poll
	// time, and any recent poller errors. Lives on /settings; Plan 15
	// removes the equivalent strip from the unread-top in favour of
	// this canonical home.
	import { useStatus, useFeeds } from '$api/queries';

	const status = useStatus();
	const feeds = useFeeds();

	function formatUptime(seconds: number): string {
		if (!Number.isFinite(seconds) || seconds <= 0) return '0s';
		const d = Math.floor(seconds / 86400);
		const h = Math.floor((seconds % 86400) / 3600);
		const m = Math.floor((seconds % 3600) / 60);
		const s = Math.floor(seconds % 60);
		if (d > 0) return `${d}d ${h}h`;
		if (h > 0) return `${h}h ${m}m`;
		if (m > 0) return `${m}m ${s}s`;
		return `${s}s`;
	}

	function formatTime(unix: number | null | undefined): string {
		if (!unix || unix <= 0) return 'never';
		try {
			return new Date(unix * 1000).toLocaleString(undefined, {
				dateStyle: 'short',
				timeStyle: 'medium'
			});
		} catch {
			return '—';
		}
	}

	const recentErrors = $derived(status.data?.run_state.recent_errors ?? []);
</script>

<dl class="stats">
	<div class="row">
		<dt>Version</dt>
		<dd class="mono">{status.data?.version ?? '—'}</dd>
	</div>
	<div class="row">
		<dt>Uptime</dt>
		<dd class="mono">{formatUptime(status.data?.uptime_seconds ?? 0)}</dd>
	</div>
	<div class="row">
		<dt>Active polls</dt>
		<dd class="mono">{status.data?.run_state.active_polls ?? 0}</dd>
	</div>
	<div class="row">
		<dt>Last poll</dt>
		<dd class="mono">{formatTime(status.data?.run_state.last_poll_at)}</dd>
	</div>
	<div class="row">
		<dt>Feeds tracked</dt>
		<dd class="mono">{feeds.data?.pagination.total ?? 0}</dd>
	</div>
</dl>

{#if recentErrors.length > 0}
	<details class="errors">
		<summary>Recent poller errors ({recentErrors.length})</summary>
		<ul>
			{#each recentErrors as err, i (i)}
				<li class="mono">{err}</li>
			{/each}
		</ul>
	</details>
{/if}

<style>
	.stats {
		display: grid;
		grid-template-columns: 1fr;
		gap: 0;
		margin: 0;
	}
	.row {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		padding: 10px 0;
		border-bottom: 1px solid var(--rule);
	}
	.row:last-child {
		border-bottom: 0;
	}
	dt {
		font-family: var(--sans);
		font-size: 13px;
		color: var(--ink-2);
	}
	dd {
		margin: 0;
		font-size: 13px;
		color: var(--ink);
	}
	.errors {
		margin-top: 16px;
		font-family: var(--sans);
		font-size: 12px;
		color: var(--ink-2);
	}
	.errors summary {
		cursor: pointer;
		padding: 4px 0;
	}
	.errors ul {
		list-style: none;
		padding: 8px 0 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.errors li {
		font-size: 11px;
		color: var(--ink-3);
		padding: 4px 8px;
		background: var(--bg-soft);
		border-radius: 3px;
		border-left: 2px solid var(--ink-4);
		word-break: break-all;
	}
</style>
