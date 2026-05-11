<script lang="ts">
  import { api } from '../lib/api';
  import type { Subscription, Category } from '../lib/types';

  let {
    subscription,
    categories = [],
    onClose,
    onSaved = onClose,
  }: { subscription: Subscription; categories?: Category[]; onClose: () => void; onSaved?: () => void } = $props();

  // Snapshot initial prop values into local edit state (modal edits a local copy).
  let categoryId = $state<number | null>(subscription.category_id ?? null);
  let extract = $state(subscription.extract);
  let extractSelector = $state(subscription.extract_selector);
  let cookie = $state('');
  let basicUser = $state('');
  let basicPass = $state('');
  let busy = $state(false);
  let error = $state<string | null>(null);

  let dialogEl = $state<HTMLDialogElement | null>(null);

  $effect(() => {
    if (dialogEl && typeof dialogEl.showModal === 'function') dialogEl.showModal();
  });

  async function save() {
    busy = true;
    error = null;
    try {
      const patch: Parameters<typeof api.updateSubscription>[1] = {
        extract,
        extract_selector: extractSelector,
      };
      if (cookie) patch.cookie = cookie;
      if (basicUser) patch.basic_auth_user = basicUser;
      if (basicPass) patch.basic_auth_pass = basicPass;
      if (categoryId !== subscription.category_id) patch.category_id = categoryId;
      await api.updateSubscription(subscription.id, patch);
      onSaved();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }

  function handleClose() {
    if (dialogEl && typeof dialogEl.close === 'function') dialogEl.close();
    onClose();
  }
</script>

<dialog bind:this={dialogEl} class="feed-modal" aria-label="Feed settings" onclose={handleClose}>
  <header>
    <h2>Feed settings</h2>
    <button onclick={handleClose} aria-label="Close">✕</button>
  </header>
  <form onsubmit={(e) => { e.preventDefault(); void save(); }}>
    <label>
      Category
      <select bind:value={categoryId}>
        <option value={null}>— uncategorised —</option>
        {#each categories as c (c.id)}
          <option value={c.id}>{c.name}</option>
        {/each}
      </select>
    </label>
    <label>
      <input type="checkbox" bind:checked={extract} />Enable article extraction
    </label>
    <label>
      Extract CSS selector
      <input type="text" bind:value={extractSelector} placeholder="e.g. .article-body" />
    </label>
    <details>
      <summary>HTTP credentials (optional)</summary>
      <label>Cookie header<input type="text" bind:value={cookie} placeholder="session=…" /></label>
      <label>Basic auth user<input type="text" bind:value={basicUser} /></label>
      <label>Basic auth password<input type="password" bind:value={basicPass} /></label>
    </details>
    {#if error}
      <p class="err">{error}</p>
    {/if}
    <footer>
      <button type="button" onclick={handleClose}>Cancel</button>
      <button type="submit" disabled={busy}>Save</button>
    </footer>
  </form>
</dialog>

<style>
  .feed-modal {
    background: var(--surface, var(--bg));
    border: 1px solid var(--rule);
    padding: 18px 22px;
    min-width: 360px;
    max-width: 480px;
  }
  .feed-modal::backdrop { background: rgba(0, 0, 0, 0.35); }
  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12px;
  }
  h2 {
    font-family: var(--sans);
    font-size: 15px;
    margin: 0;
  }
  form {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  label {
    display: flex;
    flex-direction: column;
    font-family: var(--sans);
    font-size: 13px;
    color: var(--ink-2);
    gap: 4px;
  }
  input[type='text'],
  input[type='password'],
  select {
    padding: 4px 8px;
    font-family: var(--sans);
    font-size: 12px;
    background: var(--bg);
    color: var(--ink);
    border: 1px solid var(--rule);
    border-radius: 3px;
  }
  label:has(input[type='checkbox']) {
    flex-direction: row;
    align-items: center;
    gap: 6px;
  }
  details summary {
    font-family: var(--sans);
    font-size: 12px;
    color: var(--ink-3);
    cursor: pointer;
  }
  details > label {
    margin-top: 6px;
  }
  footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 4px;
  }
  footer button {
    font-family: var(--sans);
    font-size: 13px;
    padding: 5px 14px;
    border-radius: 3px;
    cursor: pointer;
    border: 1px solid var(--rule);
    background: var(--bg);
    color: var(--ink);
  }
  footer button[type='submit'] {
    background: var(--accent);
    color: #fff;
    border-color: var(--accent);
  }
  .err {
    font-family: var(--sans);
    font-size: 12px;
    color: #b14;
    margin: 0;
  }
</style>
