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
	import { getJSON } from '$api/client';
	import { toast } from '$lib/toast.svelte';

	let open = $state(false);
	let fileInput: HTMLInputElement | null = $state(null);
	const qc = useQueryClient();

	async function importOpml(file: File) {
		try {
			const body = await getJSON<{ imported?: number }>('/opml/import', {
				method: 'POST',
				headers: { 'Content-Type': 'application/xml' },
				body: file
			});
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
			<button
				type="button"
				class="opml-action"
				role="menuitem"
				onclick={() => fileInput?.click()}
			>
				Import OPML…
			</button>
			<button
				type="button"
				class="opml-action"
				role="menuitem"
				onclick={exportOpml}
			>
				Export OPML
			</button>
			<!-- The file input lives outside the menu list so its presence
			     in the DOM doesn't add an empty menuitem slot. It's hidden
			     for both sighted and assistive users; the Import button
			     above triggers it programmatically. -->
			<input
				bind:this={fileInput}
				type="file"
				accept=".opml,.xml,application/xml,text/xml"
				aria-hidden="true"
				tabindex="-1"
				onchange={onPick}
				hidden
			/>
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
