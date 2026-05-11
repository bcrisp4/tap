<script lang="ts">
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import Button from '../../components/Button.svelte';
  import { api } from '../../lib/api';
  import ImportOPMLDialog from './dialogs/ImportOPMLDialog.svelte';
  import DeleteAccountDialog from './dialogs/DeleteAccountDialog.svelte';

  let importOpen = $state(false);
  let deleteOpen = $state(false);
  let importResult = $state<{ imported: number; skipped: number } | null>(null);
  let error = $state('');
  let busy = $state(false);

  async function exportOPML() {
    busy = true; error = '';
    try {
      const blob = await api.exportOPML();
      downloadBlob(blob, 'tap-subscriptions.opml');
    } catch (e) {
      error = e instanceof Error ? e.message : 'Export failed.';
    } finally { busy = false; }
  }

  async function exportSavedJSON() {
    busy = true; error = '';
    try {
      const res = await api.listEntries({ saved: true, limit: 1000 });
      const blob = new Blob([JSON.stringify(res.data, null, 2)], { type: 'application/json' });
      downloadBlob(blob, 'tap-saved.json');
    } catch (e) {
      error = e instanceof Error ? e.message : 'Export failed.';
    } finally { busy = false; }
  }

  function downloadBlob(blob: Blob, filename: string) {
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url; a.download = filename; a.click();
    URL.revokeObjectURL(url);
  }
</script>

<SetSection num="07" title="Data">
  <SetRow label="Export subscriptions (OPML)" desc="A standard OPML file compatible with most readers.">
    {#snippet control()}
      <Button onclick={exportOPML} disabled={busy}>Export OPML</Button>
    {/snippet}
  </SetRow>
  <SetRow label="Export saved entries (JSON)" desc="One-time snapshot of your most recent 1,000 saved entries.">
    {#snippet control()}
      <Button onclick={exportSavedJSON} disabled={busy}>Export saved JSON</Button>
    {/snippet}
  </SetRow>
  <SetRow label="Import OPML" desc="Merge feeds from another reader's export. Duplicates are skipped.">
    {#snippet control()}
      <Button onclick={() => importOpen = true}>Import OPML</Button>
    {/snippet}
  </SetRow>
  <SetRow label="Delete account" desc="Permanently remove your account and every feed, entry, and session attached to it.">
    {#snippet control()}
      <Button variant="danger" onclick={() => deleteOpen = true}>Delete account</Button>
    {/snippet}
  </SetRow>
  {#if importResult}
    <div class="ok">Imported {importResult.imported} feeds, skipped {importResult.skipped}.</div>
  {/if}
  {#if error}<div role="alert" class="error">{error}</div>{/if}
</SetSection>

{#if importOpen}
  <ImportOPMLDialog onClose={() => importOpen = false} onImported={(r) => { importResult = r; importOpen = false; }} />
{/if}
{#if deleteOpen}
  <DeleteAccountDialog onClose={() => deleteOpen = false} />
{/if}

<style>
  .ok, .error { font-family: var(--mono); font-size: 11px; padding: 8px 0; color: var(--ink-2); }
</style>
