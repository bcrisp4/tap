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

<header class="stats-header">
	<h2 class="stats-title mono">tap v{status.data?.version ?? '—'}</h2>
	<a
		class="stats-github"
		href="https://github.com/bcrisp4/tap"
		target="_blank"
		rel="noreferrer noopener"
		title="View source on GitHub"
		aria-label="View source on GitHub"
	>
		<!-- Lucide-svelte v1.0.1 dropped the GitHub brand glyph, so we
		     inline the canonical mark. Tinted with currentColor so the
		     theme switcher still controls it. -->
		<svg
			width="14"
			height="14"
			viewBox="0 0 24 24"
			fill="currentColor"
			aria-hidden="true"
		>
			<path
				d="M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.39 7.86 10.91.58.1.79-.25.79-.56 0-.27-.01-1.01-.01-1.99-3.2.7-3.87-1.54-3.87-1.54-.52-1.34-1.27-1.69-1.27-1.69-1.04-.71.08-.7.08-.7 1.15.08 1.76 1.18 1.76 1.18 1.02 1.75 2.69 1.24 3.34.95.1-.74.4-1.24.72-1.53-2.55-.29-5.24-1.28-5.24-5.69 0-1.26.45-2.29 1.18-3.1-.12-.29-.51-1.46.11-3.05 0 0 .96-.31 3.15 1.18.91-.25 1.89-.38 2.86-.39.97 0 1.95.13 2.86.39 2.18-1.49 3.14-1.18 3.14-1.18.62 1.59.23 2.76.11 3.05.74.81 1.18 1.84 1.18 3.1 0 4.42-2.69 5.39-5.25 5.68.41.36.78 1.06.78 2.13 0 1.54-.01 2.78-.01 3.16 0 .31.21.67.8.55C20.21 21.39 23.5 17.07 23.5 12c0-6.35-5.15-11.5-11.5-11.5z"
			/>
		</svg>
	</a>
</header>

<dl class="stats">
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
	.stats-header {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 12px;
		margin: 0 0 6px;
	}
	.stats-title {
		font-size: 13px;
		font-weight: 500;
		color: var(--ink);
		margin: 0;
		letter-spacing: 0;
	}
	.stats-github {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		color: var(--ink-3);
		padding: 4px;
		border-radius: 4px;
		text-decoration: none;
	}
	@media (hover: hover) {
		.stats-github:hover {
			color: var(--ink);
		}
	}
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
