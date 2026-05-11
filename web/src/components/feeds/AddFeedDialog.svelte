<script lang="ts">
  import Dialog from '../Dialog.svelte';
  import type { Category, DiscoverCandidate } from '../../lib/types';
  import { api } from '../../lib/api';
  import { notifySW } from '../../lib/auth';

  type Props = {
    categories: Category[];
    onClose: () => void;
    onAdded: () => void;
  };
  let { categories, onClose, onAdded }: Props = $props();

  let url = $state('');
  let busy = $state(false);
  let error = $state<string | null>(null);
  let candidates = $state<DiscoverCandidate[] | null>(null);
  let picked = $state<DiscoverCandidate | null>(null);
  let selectedCategory = $state<number | null>(null);

  async function lookUp() {
    const trimmed = url.trim();
    if (!trimmed) return;
    busy = true;
    error = null;
    candidates = null;
    picked = null;
    try {
      const result = await api.discoverFeeds(trimmed);
      candidates = result.candidates;
      if (candidates.length === 1) picked = candidates[0];
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }

  async function subscribe() {
    if (!picked) return;
    busy = true;
    error = null;
    try {
      const body: Parameters<typeof api.addSubscription>[0] = { feed_url: picked.feed_url };
      if (selectedCategory !== null) body.category_id = selectedCategory;
      await api.addSubscription(body);
      notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions', '/api/v1/entries'] });
      onAdded();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<Dialog open={true} title="Add feed" wide={true} {onClose}>
  {#snippet foot()}
    <div class="ts-feeds-form-foot">
      <button type="button" class="btn-cancel" onclick={onClose}>Cancel</button>
      {#if picked}
        <button
          type="button"
          class="btn-subscribe"
          disabled={busy}
          onclick={subscribe}
        >Subscribe</button>
      {/if}
    </div>
  {/snippet}

  <div class="ts-feeds-form">
    <div class="ts-feeds-form-field-row">
      <label class="ts-feeds-form-label" for="add-feed-url">Feed URL or site URL</label>
      <div class="ts-feeds-form-row">
        <input
          id="add-feed-url"
          class="ts-feeds-form-input"
          type="text"
          placeholder="example.com"
          value={url}
          oninput={(e) => { url = (e.currentTarget as HTMLInputElement).value; }}
          onkeydown={(e) => { if (e.key === 'Enter' && url.trim()) lookUp(); }}
        />
        <button
          type="button"
          class="ts-feeds-form-go"
          disabled={!url.trim() || busy}
          onclick={lookUp}
        >Look up</button>
      </div>
    </div>

    {#if error}
      <p class="ts-feeds-form-error">{error}</p>
    {/if}

    {#if candidates !== null}
      {#if candidates.length === 0}
        <p class="ts-feeds-disc-empty">No feeds found at that URL.</p>
      {:else}
        <div class="ts-feeds-disc">
          <p class="ts-feeds-disc-head">
            {candidates.length === 1 ? '1 feed found' : `${candidates.length} feeds found — pick one`}
          </p>
          {#each candidates as candidate}
            <button
              type="button"
              class="ts-feeds-disc-row"
              class:is-picked={picked === candidate}
              onclick={() => { picked = candidate; }}
            >
              <span class="ts-feeds-disc-tag">{candidate.type.toUpperCase()}</span>
              <span class="ts-feeds-disc-title">{candidate.title}</span>
              <span class="ts-feeds-disc-url">{candidate.feed_url}</span>
            </button>
          {/each}
        </div>

        {#if picked}
          <div class="ts-feeds-edit-cat">
            <p class="ts-feeds-edit-cat-label">Category</p>
            <div class="ts-feeds-edit-cat-list">
              <button
                type="button"
                class="ts-feeds-edit-cat-btn"
                class:is-active={selectedCategory === null}
                onclick={() => { selectedCategory = null; }}
              >Uncategorised</button>
              {#each categories as cat}
                <button
                  type="button"
                  class="ts-feeds-edit-cat-btn"
                  class:is-active={selectedCategory === cat.id}
                  onclick={() => { selectedCategory = cat.id; }}
                >{cat.name}</button>
              {/each}
            </div>
          </div>
        {/if}
      {/if}
    {/if}
  </div>
</Dialog>

<style>
  .ts-feeds-form { display: flex; flex-direction: column; gap: 16px; padding: 4px 0; }
  .ts-feeds-form-field-row { display: flex; flex-direction: column; gap: 6px; }
  .ts-feeds-form-label { font-family: var(--sans); font-size: 12px; color: var(--ink-3); }
  .ts-feeds-form-row { display: flex; gap: 8px; }
  .ts-feeds-form-input {
    flex: 1; padding: 8px 12px;
    border: 1px solid var(--rule); border-radius: var(--radius-input, 4px);
    background: var(--bg); color: var(--ink);
    font-family: var(--mono); font-size: 12.5px;
    outline: none;
  }
  .ts-feeds-form-input:focus { border-color: var(--ink-4); }
  .ts-feeds-form-go {
    padding: 8px 16px;
    border: 1px solid var(--ink); border-radius: var(--radius-input, 4px);
    background: var(--ink); color: var(--bg);
    font-family: var(--sans); font-size: 12.5px; font-weight: 500;
    cursor: pointer;
  }
  .ts-feeds-form-go:disabled { opacity: 0.5; cursor: not-allowed; }
  .ts-feeds-form-error { font-family: var(--sans); font-size: 12.5px; color: var(--color-error-light, #c43a3a); margin: 0; }
  :global(html.theme-dark) .ts-feeds-form-error { color: var(--color-error-dark, #ec7a7a); }

  .ts-feeds-disc { display: flex; flex-direction: column; gap: 6px; }
  .ts-feeds-disc-head { font-family: var(--sans); font-size: 12px; color: var(--ink-3); margin: 0 0 4px; }
  .ts-feeds-disc-empty { font-family: var(--sans); font-size: 13px; color: var(--ink-3); margin: 0; }
  .ts-feeds-disc-row {
    display: flex; align-items: center; gap: 10px;
    padding: 8px 12px; text-align: left;
    border: 1px solid var(--rule); border-radius: var(--radius-input, 4px);
    background: var(--bg); cursor: pointer;
  }
  .ts-feeds-disc-row:hover { background: var(--bg-soft); }
  .ts-feeds-disc-row.is-picked { border-color: var(--ink); background: var(--bg-soft); }
  .ts-feeds-disc-tag { font-family: var(--mono); font-size: 9.5px; text-transform: uppercase; letter-spacing: 0.08em; color: var(--ink-3); min-width: 36px; }
  .ts-feeds-disc-title { font-family: var(--sans); font-size: 13px; font-weight: 500; color: var(--ink); flex: 1; }
  .ts-feeds-disc-url { font-family: var(--mono); font-size: 11px; color: var(--ink-3); }

  .ts-feeds-edit-cat { display: flex; flex-direction: column; gap: 8px; }
  .ts-feeds-edit-cat-label { font-family: var(--sans); font-size: 12px; color: var(--ink-3); margin: 0; }
  .ts-feeds-edit-cat-list { display: flex; gap: 6px; flex-wrap: wrap; }
  .ts-feeds-edit-cat-btn {
    padding: 4px 10px;
    border: 1px solid var(--rule); border-radius: var(--radius-pill, 100px);
    background: var(--bg); color: var(--ink-2);
    font-family: var(--sans); font-size: 12px; cursor: pointer;
  }
  .ts-feeds-edit-cat-btn:hover { color: var(--ink); }
  .ts-feeds-edit-cat-btn.is-active { background: var(--ink); color: var(--bg); border-color: var(--ink); }

  .ts-feeds-form-foot { display: flex; justify-content: flex-end; gap: 8px; }
  .btn-cancel {
    padding: 7px 14px; border: 1px solid var(--rule); border-radius: var(--radius-input, 4px);
    background: var(--bg); color: var(--ink-2); font-family: var(--sans); font-size: 12.5px; cursor: pointer;
  }
  .btn-subscribe {
    padding: 7px 16px; border: 1px solid var(--ink); border-radius: var(--radius-input, 4px);
    background: var(--ink); color: var(--bg); font-family: var(--sans); font-size: 12.5px; font-weight: 500; cursor: pointer;
  }
  .btn-subscribe:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
