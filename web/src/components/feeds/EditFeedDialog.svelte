<script lang="ts">
  import Dialog from '../Dialog.svelte';
  import type { Subscription, Category } from '../../lib/types';
  import { api } from '../../lib/api';
  import { notifySW } from '../../lib/auth';

  type Props = {
    feed: Subscription;
    categories: Category[];
    onClose: () => void;
    onSaved: () => void;
    onDelete: () => void;
  };
  let { feed, categories, onClose, onSaved, onDelete }: Props = $props();

  // Snapshot feed values on open — intentional for edit-form pattern.
  const init = $state.snapshot(feed);
  let title = $state(init.title);
  let extract = $state(init.extract);
  let extractSelector = $state(init.extract_selector);
  let cookie = $state('');
  let basicAuthUser = $state('');
  let basicAuthPass = $state('');
  let selectedCategory = $state<number | null>(init.category_id);
  const originalCategoryId = init.category_id;

  let busy = $state(false);
  let error = $state<string | null>(null);

  async function save() {
    busy = true;
    error = null;
    try {
      const patch: Parameters<typeof api.updateSubscription>[1] = {
        extract,
        extract_selector: extractSelector,
      };
      if (cookie) patch.cookie = cookie;
      if (basicAuthUser) patch.basic_auth_user = basicAuthUser;
      if (basicAuthPass) patch.basic_auth_pass = basicAuthPass;
      if (selectedCategory !== originalCategoryId) patch.category_id = selectedCategory;
      await api.updateSubscription(feed.id, patch);
      notifySW({ type: 'invalidate', paths: ['/api/v1/subscriptions', '/api/v1/categories'] });
      onSaved();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<Dialog open={true} title="Edit feed" wide={true} {onClose}>
  {#snippet foot()}
    <div class="ts-feeds-edit-foot">
      <button type="button" class="btn-delete" onclick={onDelete}>Delete this feed</button>
      <div class="ts-feeds-edit-foot-right">
        <button type="button" class="btn-cancel" onclick={onClose}>Cancel</button>
        <button type="button" class="btn-save" disabled={busy} onclick={save}>Save changes</button>
      </div>
    </div>
  {/snippet}

  <div class="ts-feeds-edit-grid">
    <div class="head">01 · BASICS</div>
    <label class="ts-feeds-edit-field">
      <span class="lbl">Title</span>
      <input
        type="text"
        class="ts-feeds-edit-input"
        value={title}
        oninput={(e) => { title = (e.currentTarget as HTMLInputElement).value; }}
      />
    </label>
    <label class="ts-feeds-edit-field">
      <span class="lbl">Feed URL</span>
      <input
        type="text"
        class="ts-feeds-edit-input mono"
        value={feed.feed_url}
        readonly
      />
    </label>

    <div class="ts-feeds-edit-field">
      <span class="lbl">Category</span>
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

    <div class="head">02 · ARTICLE EXTRACTION</div>
    <div class="ts-feeds-edit-field">
      <span class="lbl">Extract full articles</span>
      <button
        type="button"
        class="ts-feeds-edit-toggle"
        class:is-on={extract}
        onclick={() => { extract = !extract; }}
        aria-pressed={extract}
      >
        <span class="ts-feeds-edit-toggle-sw"></span>
        <span>{extract ? 'On' : 'Off'}</span>
      </button>
    </div>
    <label class="ts-feeds-edit-field">
      <span class="lbl">CSS selector override</span>
      <input
        type="text"
        class="ts-feeds-edit-input mono"
        placeholder=".article-body (leave blank for readability mode)"
        value={extractSelector}
        disabled={!extract}
        oninput={(e) => { extractSelector = (e.currentTarget as HTMLInputElement).value; }}
      />
    </label>

    <div class="head">03 · CREDENTIALS</div>
    <label class="ts-feeds-edit-field">
      <span class="lbl">Cookie{feed.has_cookie ? ' (currently set — clear to remove)' : ''}</span>
      <textarea
        class="ts-feeds-edit-textarea mono"
        placeholder="cookie=value; or session=abc"
        value={cookie}
        oninput={(e) => { cookie = (e.currentTarget as HTMLTextAreaElement).value; }}
      ></textarea>
    </label>
    <label class="ts-feeds-edit-field">
      <span class="lbl">Username{feed.has_basic_auth ? ' (currently set)' : ''}</span>
      <input
        type="text"
        class="ts-feeds-edit-input mono"
        placeholder="Basic auth username"
        value={basicAuthUser}
        oninput={(e) => { basicAuthUser = (e.currentTarget as HTMLInputElement).value; }}
      />
    </label>
    <label class="ts-feeds-edit-field">
      <span class="lbl">Password</span>
      <input
        type="password"
        class="ts-feeds-edit-input mono"
        placeholder="Basic auth password"
        value={basicAuthPass}
        oninput={(e) => { basicAuthPass = (e.currentTarget as HTMLInputElement).value; }}
      />
    </label>
  </div>

  {#if error}
    <p class="ts-feeds-edit-error">{error}</p>
  {/if}
</Dialog>

<style>
  .ts-feeds-edit-grid { display: flex; flex-direction: column; gap: 14px; }
  .head { font-family: var(--mono); font-size: 10px; text-transform: uppercase; letter-spacing: 0.1em; color: var(--ink-3); padding-top: 8px; border-top: 1px solid var(--rule); margin-top: 4px; }
  .ts-feeds-edit-field { display: flex; flex-direction: column; gap: 5px; }
  .lbl { font-family: var(--sans); font-size: 11.5px; color: var(--ink-3); }
  .ts-feeds-edit-input {
    padding: 7px 10px;
    border: 1px solid var(--rule); border-radius: var(--radius-input, 4px);
    background: var(--bg); color: var(--ink);
    font-family: var(--sans); font-size: 13px;
    outline: none;
  }
  .ts-feeds-edit-input.mono { font-family: var(--mono); font-size: 12px; }
  .ts-feeds-edit-input:focus { border-color: var(--ink-4); }
  .ts-feeds-edit-input[readonly] { opacity: 0.6; cursor: default; }
  .ts-feeds-edit-input:disabled { opacity: 0.4; }
  .ts-feeds-edit-textarea {
    padding: 7px 10px; resize: vertical; min-height: 56px;
    border: 1px solid var(--rule); border-radius: var(--radius-input, 4px);
    background: var(--bg); color: var(--ink);
    font-family: var(--mono); font-size: 12px;
    outline: none;
  }
  .ts-feeds-edit-textarea:focus { border-color: var(--ink-4); }

  .ts-feeds-edit-toggle {
    display: inline-flex; align-items: center; gap: 8px;
    padding: 5px 10px; width: fit-content;
    border: 1px solid var(--rule); border-radius: var(--radius-input, 4px);
    background: var(--bg); color: var(--ink-2);
    font-family: var(--sans); font-size: 12.5px; cursor: pointer;
  }
  .ts-feeds-edit-toggle.is-on { background: var(--ink); color: var(--bg); border-color: var(--ink); }
  .ts-feeds-edit-toggle-sw {
    width: 26px; height: 14px; border-radius: 7px;
    background: var(--ink-4); position: relative;
    transition: background 120ms;
  }
  .ts-feeds-edit-toggle.is-on .ts-feeds-edit-toggle-sw { background: var(--bg); }

  .ts-feeds-edit-cat-list { display: flex; gap: 6px; flex-wrap: wrap; }
  .ts-feeds-edit-cat-btn {
    padding: 4px 10px;
    border: 1px solid var(--rule); border-radius: var(--radius-pill, 100px);
    background: var(--bg); color: var(--ink-2);
    font-family: var(--sans); font-size: 12px; cursor: pointer;
  }
  .ts-feeds-edit-cat-btn.is-active { background: var(--ink); color: var(--bg); border-color: var(--ink); }

  .ts-feeds-edit-error { font-family: var(--sans); font-size: 12.5px; color: var(--color-error-light, #c43a3a); margin: 8px 0 0; }
  :global(html.theme-dark) .ts-feeds-edit-error { color: var(--color-error-dark, #ec7a7a); }

  .ts-feeds-edit-foot { display: flex; justify-content: space-between; align-items: center; }
  .ts-feeds-edit-foot-right { display: flex; gap: 8px; }
  .btn-delete {
    padding: 7px 12px; border: 1px solid rgba(196,58,58,0.4); border-radius: var(--radius-input, 4px);
    background: transparent; color: var(--color-error-light, #c43a3a);
    font-family: var(--sans); font-size: 12.5px; cursor: pointer;
  }
  :global(html.theme-dark) .btn-delete { color: var(--color-error-dark, #ec7a7a); border-color: rgba(236,122,122,0.3); }
  .btn-cancel {
    padding: 7px 14px; border: 1px solid var(--rule); border-radius: var(--radius-input, 4px);
    background: var(--bg); color: var(--ink-2); font-family: var(--sans); font-size: 12.5px; cursor: pointer;
  }
  .btn-save {
    padding: 7px 16px; border: 1px solid var(--ink); border-radius: var(--radius-input, 4px);
    background: var(--ink); color: var(--bg); font-family: var(--sans); font-size: 12.5px; font-weight: 500; cursor: pointer;
  }
  .btn-save:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
