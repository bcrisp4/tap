<script lang="ts">
  import Dialog from '../Dialog.svelte';
  import { api } from '../../lib/api';
  import type { OPMLImportResult } from '../../lib/types';

  type Props = {
    onClose: () => void;
    onImported: () => void;
  };
  let { onClose, onImported }: Props = $props();

  type Stage = 'drop' | 'uploading' | 'done' | 'error';
  let stage = $state<Stage>('drop');
  let result = $state<OPMLImportResult | null>(null);
  let errorMsg = $state<string | null>(null);
  let dragOver = $state(false);

  let fileInputEl = $state<HTMLInputElement | null>(null);

  function uploadFile(file: File) {
    stage = 'uploading';
    errorMsg = null;
    const reader = new FileReader();
    reader.onload = async (e) => {
      try {
        const buf = e.target!.result as ArrayBuffer;
        result = await api.importOPML(buf);
        stage = 'done';
      } catch (err) {
        errorMsg = (err as Error).message;
        stage = 'error';
      }
    };
    reader.onerror = () => {
      errorMsg = 'Could not read file';
      stage = 'error';
    };
    reader.readAsArrayBuffer(file);
  }

  function onFileChange(e: Event) {
    const input = (e.target ?? e.currentTarget) as HTMLInputElement;
    const file = input.files?.[0];
    if (file) uploadFile(file);
  }

  function onDrop(e: DragEvent) {
    e.preventDefault();
    dragOver = false;
    const file = e.dataTransfer?.files[0];
    if (file) uploadFile(file);
  }

  function dismiss() {
    onImported();
    onClose();
  }
</script>

<Dialog open={true} title="Import OPML" wide={true} {onClose}>
  {#snippet foot()}
    <div class="ts-import-foot">
      {#if stage === 'done'}
        <button type="button" class="btn-primary" onclick={dismiss}>Done</button>
      {:else}
        <button type="button" class="btn-cancel" onclick={onClose}>Cancel</button>
      {/if}
    </div>
  {/snippet}

  <div class="ts-import-body">
    {#if stage === 'drop'}
      <div
        class="ts-feeds-opml-drop"
        class:is-over={dragOver}
        role="region"
        aria-label="OPML drop zone"
        ondragover={(e) => { e.preventDefault(); dragOver = true; }}
        ondragleave={() => { dragOver = false; }}
        ondrop={onDrop}
      >
        <svg width="32" height="32" viewBox="0 0 32 32" fill="none" aria-hidden="true" class="drop-icon">
          <circle cx="16" cy="16" r="14" stroke="currentColor" stroke-width="1.5"/>
          <path d="M16 10v8M13 15l3 3 3-3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          <path d="M11 22h10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
        </svg>
        <p class="drop-primary">Drop an .opml file here</p>
        <p class="drop-secondary">
          or <button type="button" class="link" onclick={() => fileInputEl?.click()}>browse</button> to choose a file
        </p>
        <input
          type="file"
          accept=".opml,application/xml,text/x-opml"
          style="display: none"
          bind:this={fileInputEl}
          onchange={onFileChange}
        />
      </div>
    {:else if stage === 'uploading'}
      <div class="ts-import-loading">
        <svg class="spin" width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="2" stroke-dasharray="28 56" stroke-linecap="round"/>
        </svg>
        <p>Importing…</p>
      </div>
    {:else if stage === 'done' && result}
      <div class="ts-import-result">
        <p class="ts-import-result-eyebrow">IMPORT COMPLETE</p>
        <div class="ts-import-result-stats">
          <span class="stat">{result.imported} imported</span>
          {#if result.skipped > 0}<span class="stat muted">{result.skipped} skipped</span>{/if}
        </div>
        {#if result.errors.length > 0}
          <ul class="ts-import-errors">
            {#each result.errors as err}
              <li class="ts-import-error-item">{err}</li>
            {/each}
          </ul>
        {/if}
      </div>
    {:else if stage === 'error'}
      <div class="ts-import-error">
        <p class="ts-import-error-msg">{errorMsg}</p>
        <button type="button" class="link" onclick={() => { stage = 'drop'; }}>Try again</button>
      </div>
    {/if}
  </div>
</Dialog>

<style>
  .ts-import-body { min-height: 160px; display: flex; flex-direction: column; }
  .ts-feeds-opml-drop {
    flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center;
    gap: 10px; padding: 32px 16px;
    border: 2px dashed var(--rule); border-radius: var(--radius-input, 4px);
    background: var(--bg-soft); transition: border-color 120ms, background 120ms;
  }
  .ts-feeds-opml-drop.is-over { border-color: var(--ink-4); background: var(--bg); }
  .drop-icon { color: var(--ink-3); }
  .drop-primary { font-family: var(--sans); font-size: 14px; font-weight: 500; color: var(--ink-2); margin: 0; }
  .drop-secondary { font-family: var(--sans); font-size: 12.5px; color: var(--ink-3); margin: 0; }
  .link { border: none; background: none; color: var(--ink-2); text-decoration: underline; cursor: pointer; font-family: inherit; font-size: inherit; padding: 0; }
  .link:hover { color: var(--ink); }

  .ts-import-loading { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; color: var(--ink-3); }
  @keyframes spin { to { transform: rotate(360deg); } }
  .spin { animation: spin 0.9s linear infinite; }

  .ts-import-result { display: flex; flex-direction: column; gap: 12px; }
  .ts-import-result-eyebrow { font-family: var(--mono); font-size: 10px; text-transform: uppercase; letter-spacing: 0.1em; color: var(--ink-3); margin: 0; }
  .ts-import-result-stats { display: flex; gap: 16px; }
  .stat { font-family: var(--sans); font-size: 14px; color: var(--ink-2); }
  .stat.muted { color: var(--ink-3); }
  .ts-import-errors { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
  .ts-import-error-item { font-family: var(--mono); font-size: 11.5px; color: var(--color-error-light, #c43a3a); }
  :global(html.theme-dark) .ts-import-error-item { color: var(--color-error-dark, #ec7a7a); }
  .ts-import-error { display: flex; flex-direction: column; gap: 8px; align-items: flex-start; padding: 16px; }
  .ts-import-error-msg { font-family: var(--sans); font-size: 13px; color: var(--color-error-light, #c43a3a); margin: 0; }
  :global(html.theme-dark) .ts-import-error-msg { color: var(--color-error-dark, #ec7a7a); }

  .ts-import-foot { display: flex; justify-content: flex-end; }
  .btn-cancel {
    padding: 7px 14px; border: 1px solid var(--rule); border-radius: var(--radius-input, 4px);
    background: var(--bg); color: var(--ink-2); font-family: var(--sans); font-size: 12.5px; cursor: pointer;
  }
  .btn-primary {
    padding: 7px 16px; border: 1px solid var(--ink); border-radius: var(--radius-input, 4px);
    background: var(--ink); color: var(--bg); font-family: var(--sans); font-size: 12.5px; font-weight: 500; cursor: pointer;
  }
</style>
