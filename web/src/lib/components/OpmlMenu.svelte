<script lang="ts">
	// OPML import/export popover. Sits in the sidebar footer next to
	// the Settings icon. The trigger is a Lucide FolderInput; clicking
	// it pops a small menu with Import + Export actions.
	//
	// Important wire detail: the Go handler in `internal/api/opml.go`
	// reads `r.Body` directly and parses it as XML (NOT multipart),
	// so we POST the file as-is. Sending FormData here would have the
	// server try to xml.Unmarshal a `--boundary…` MIME prelude.
	//
	// Successful imports invalidate the feeds query so newly-imported
	// rows appear in the sidebar without a page reload.
	import { FolderInput } from 'lucide-svelte';
	import { useQueryClient } from '@tanstack/svelte-query';
	import { keys } from '$api/queries';
	import { toast } from '$lib/toast.svelte';

	let open = $state(false);
	const qc = useQueryClient();

	async function importOpml(file: File) {
		try {
			const res = await fetch('/api/v1/opml/import', {
				method: 'POST',
				headers: { 'Content-Type': 'application/xml' },
				body: file
			});
			if (!res.ok) {
				const body = await res.json().catch(() => null);
				throw new Error(body?.error?.message ?? `import failed: ${res.status}`);
			}
			const body = (await res.json()) as { imported?: number };
			const n = body.imported ?? 0;
			toast.push(`Imported ${n} feed${n === 1 ? '' : 's'}`, 'info');
			void qc.invalidateQueries({ queryKey: keys.feeds() });
		} catch (e) {
			toast.push(e instanceof Error ? e.message : 'OPML import failed', 'error');
		} finally {
			open = false;
		}
	}

	function exportOpml() {
		const a = document.createElement('a');
		a.href = '/api/v1/opml/export';
		a.download = 'tap-subscriptions.opml';
		document.body.appendChild(a);
		a.click();
		a.remove();
		open = false;
	}

	function onPick(ev: Event) {
		const input = ev.target as HTMLInputElement;
		const file = input.files?.[0];
		// Reset the input so picking the same file twice in a row
		// re-fires the change event (browser otherwise dedupes).
		input.value = '';
		if (file) void importOpml(file);
	}
</script>

<div class="opml-menu">
	<button
		type="button"
		class="icon-btn"
		title="OPML import / export"
		aria-label="OPML import / export"
		aria-haspopup="menu"
		aria-expanded={open}
		onclick={() => (open = !open)}
	>
		<FolderInput size="16" aria-hidden="true" />
	</button>

	{#if open}
		<div class="opml-popover" role="menu">
			<label class="opml-action">
				<span>Import OPML…</span>
				<input
					type="file"
					accept=".opml,.xml,application/xml,text/xml"
					aria-label="Import OPML"
					onchange={onPick}
					hidden
				/>
			</label>
			<button type="button" class="opml-action" onclick={exportOpml}>
				Export OPML
			</button>
		</div>
	{/if}
</div>

<style>
	.opml-menu {
		position: relative;
		display: inline-flex;
	}
	.icon-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border: none;
		border-radius: 4px;
		background: transparent;
		color: var(--ink-3);
		cursor: pointer;
		padding: 0;
	}
	@media (hover: hover) {
		.icon-btn:hover {
			color: var(--ink);
			background: var(--surface);
		}
	}
	.opml-popover {
		position: absolute;
		bottom: calc(100% + 6px);
		left: 0;
		background: var(--surface);
		border: 1px solid var(--rule);
		border-radius: 6px;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
		padding: 4px;
		min-width: 160px;
		z-index: 10;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.opml-action {
		display: block;
		text-align: left;
		padding: 6px 10px;
		font-family: var(--sans);
		font-size: 13px;
		color: var(--ink);
		background: none;
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}
	@media (hover: hover) {
		.opml-action:hover {
			background: var(--bg-soft);
		}
	}
</style>
