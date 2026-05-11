<script lang="ts">
  import Dialog from '../../../components/Dialog.svelte';
  import Button from '../../../components/Button.svelte';
  import { api } from '../../../lib/api';
  import type { OPMLImportResult } from '../../../lib/types';

  interface Props { onClose: () => void; onImported: (r: OPMLImportResult) => void; }
  let { onClose, onImported }: Props = $props();

  let file = $state<File | null>(null);
  let busy = $state(false);
  let error = $state('');

  function onFile(e: Event) {
    const t = e.target as HTMLInputElement;
    file = t.files?.[0] ?? null;
  }

  async function submit() {
    if (!file) return;
    busy = true; error = '';
    try {
      const buf = await file.arrayBuffer();
      const result = await api.importOPML(buf);
      onImported(result);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Import failed.';
    } finally { busy = false; }
  }
</script>

<Dialog open title="Import OPML" {onClose}>
  {#snippet children()}
    <p class="p">Choose an OPML file exported from another reader. Existing subscriptions with the same feed URL are skipped.</p>
    <input type="file" accept=".opml,.xml,application/xml" onchange={onFile} aria-label="OPML file" />
    {#if error}<div role="alert" class="warn">{error}</div>{/if}
  {/snippet}
  {#snippet foot()}
    <Button onclick={onClose}>Cancel</Button>
    <Button variant="primary" onclick={submit} disabled={busy || !file}>Import</Button>
  {/snippet}
</Dialog>

<style>
  .p { font-family: var(--serif); font-size: 15px; line-height: 1.55; color: var(--ink-2); margin: 0 0 14px; }
  .warn { margin-top: 12px; padding: 10px 12px; border-left: 2px solid #c43a3a; background: rgba(196,58,58,0.06); color: var(--ink-2); font-family: var(--sans); font-size: 12px; line-height: 1.5; border-radius: 0 3px 3px 0; }
</style>
