<script lang="ts">
  import Dialog from '../Dialog.svelte';
  import { api } from '../../lib/api';

  type Props = {
    feedCount: number;
    categoryCount: number;
    onClose: () => void;
  };
  let { feedCount, categoryCount, onClose }: Props = $props();

  let busy = $state(false);
  let error = $state<string | null>(null);

  async function download() {
    busy = true;
    error = null;
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
      onClose();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<Dialog open={true} title="Export OPML" {onClose}>
  {#snippet foot()}
    <div class="ts-export-foot">
      <button type="button" class="btn-cancel" onclick={onClose}>Cancel</button>
      <button type="button" class="btn-download" disabled={busy} onclick={download}>
        <svg width="13" height="13" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path d="M8 2v9M5 8l3 3 3-3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M3 13h10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
        </svg>
        Download
      </button>
    </div>
  {/snippet}

  <div class="ts-feeds-opml-stats">
    <p class="ts-feeds-opml-stats-eyebrow">YOUR SUBSCRIPTIONS</p>
    <div class="ts-feeds-opml-stats-row">
      <span class="ts-stat"><span class="ts-stat-val">{feedCount}</span><span class="ts-stat-lbl">feeds</span></span>
      <span class="ts-stat"><span class="ts-stat-val">{categoryCount}</span><span class="ts-stat-lbl">categories</span></span>
    </div>
    <p class="ts-feeds-opml-stats-desc">
      Download a standard OPML file that you can import into any other feed reader.
    </p>
    {#if error}
      <p class="ts-export-error">{error}</p>
    {/if}
  </div>
</Dialog>

<style>
  .ts-feeds-opml-stats { display: flex; flex-direction: column; gap: 12px; }
  .ts-feeds-opml-stats-eyebrow { font-family: var(--mono); font-size: 10px; text-transform: uppercase; letter-spacing: 0.1em; color: var(--ink-3); margin: 0; }
  .ts-feeds-opml-stats-row { display: flex; gap: 24px; }
  .ts-stat { display: flex; flex-direction: column; gap: 2px; }
  .ts-stat-val { font-family: var(--mono); font-size: 22px; font-weight: 600; color: var(--ink); line-height: 1; }
  .ts-stat-lbl { font-family: var(--sans); font-size: 11px; color: var(--ink-3); text-transform: uppercase; letter-spacing: 0.06em; }
  .ts-feeds-opml-stats-desc { font-family: var(--sans); font-size: 13px; color: var(--ink-2); margin: 0; line-height: 1.5; }
  .ts-export-error { font-family: var(--sans); font-size: 12.5px; color: var(--color-error-light, #c43a3a); margin: 0; }
  :global(html.theme-dark) .ts-export-error { color: var(--color-error-dark, #ec7a7a); }
  .ts-export-foot { display: flex; justify-content: flex-end; gap: 8px; }
  .btn-cancel {
    padding: 7px 14px; border: 1px solid var(--rule); border-radius: var(--radius-input, 4px);
    background: var(--bg); color: var(--ink-2); font-family: var(--sans); font-size: 12.5px; cursor: pointer;
  }
  .btn-download {
    display: inline-flex; align-items: center; gap: 6px;
    padding: 7px 16px; border: 1px solid var(--ink); border-radius: var(--radius-input, 4px);
    background: var(--ink); color: var(--bg); font-family: var(--sans); font-size: 12.5px; font-weight: 500; cursor: pointer;
  }
  .btn-download:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
