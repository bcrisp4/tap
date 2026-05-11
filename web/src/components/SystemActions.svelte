<script lang="ts">
  import { auth } from '../lib/auth';
  import { navigate } from '../lib/router';
  import { api } from '../lib/api';
  import { subscriptions, categories } from '../lib/store';

  let busyOpml = $state(false);
  let opmlError = $state<string | null>(null);
  let opmlResult = $state<{ imported: number; skipped: number } | null>(null);
  let fileInput = $state<HTMLInputElement | null>(null);

  async function doLogout() {
    await auth.logout();
    navigate('/');
  }

  async function doExport() {
    busyOpml = true;
    opmlError = null;
    try {
      const blob = await api.exportOPML();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'tap-subscriptions.opml';
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (e) {
      opmlError = (e as Error).message;
    } finally {
      busyOpml = false;
    }
  }

  async function doImport(ev: Event) {
    const f = (ev.target as HTMLInputElement).files?.[0];
    if (!f) return;
    busyOpml = true;
    opmlError = null;
    opmlResult = null;
    try {
      const buf = await f.arrayBuffer();
      const r = await api.importOPML(buf);
      opmlResult = { imported: r.imported, skipped: r.skipped };
      await Promise.all([subscriptions.load(), categories.load()]);
    } catch (e) {
      opmlError = (e as Error).message;
    } finally {
      busyOpml = false;
      if (fileInput) fileInput.value = '';
    }
  }
</script>

{#if $auth.user?.role === 'admin'}
  <a class="navitem" href="/admin" onclick={(e) => { e.preventDefault(); navigate('/admin'); }}>Admin</a>
{/if}

<button class="sys-action" type="button" onclick={doExport} disabled={busyOpml}>Export OPML</button>
<label class="sys-action" for="opml-import">Import OPML</label>
<input id="opml-import" type="file" accept=".opml,.xml,application/xml,text/xml"
       bind:this={fileInput} onchange={doImport} hidden />

{#if opmlResult}
  <p class="opml-feedback">Imported {opmlResult.imported}, skipped {opmlResult.skipped}.</p>
{/if}
{#if opmlError}
  <p class="opml-feedback err">{opmlError}</p>
{/if}

<button class="sys-action sys-logout" type="button" onclick={doLogout}>Sign out</button>

<style>
  .navitem {
    display: block;
    padding: 6px 20px;
    font-family: var(--sans);
    font-size: 13px;
    color: var(--ink);
    border-left: 2px solid transparent;
    text-decoration: none;
  }
  .sys-action {
    display: block;
    width: 100%;
    text-align: left;
    padding: 6px 20px;
    font-family: var(--sans);
    font-size: 13px;
    color: var(--ink-2);
    background: none;
    border: 0;
    cursor: pointer;
  }
  .sys-action:hover { color: var(--ink); background: var(--bg-soft); }
  .sys-logout { margin-top: 4px; }
  .opml-feedback {
    font-family: var(--mono);
    font-size: 10px;
    color: var(--ink-3);
    padding: 4px 20px;
    margin: 0;
  }
  .opml-feedback.err { color: #b14; }
</style>
