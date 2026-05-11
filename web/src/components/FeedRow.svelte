<script lang="ts">
  import FeedAvatar from './FeedAvatar.svelte';
  import FeedSettingsModal from './FeedSettingsModal.svelte';
  import { api } from '../lib/api';
  import { subscriptions } from '../lib/store';
  import type { Subscription, Category } from '../lib/types';

  let {
    subscription,
    categories: cats,
  }: { subscription: Subscription; categories: Category[] } = $props();

  let menuOpen = $state(false);
  let confirmingDelete = $state(false);
  let settingsOpen = $state(false);

  async function doDelete() {
    await api.deleteSubscription(subscription.id);
    await subscriptions.load();
  }

  async function assignCategory(catId: number | null) {
    await api.updateSubscription(subscription.id, { category_id: catId });
    await subscriptions.load();
    menuOpen = false;
  }
</script>

<div class="feedrow">
  <FeedAvatar feedURL={subscription.feed_url} />
  <span class="feedname" title={subscription.title}>{subscription.title}</span>
  <button
    class="row-actions"
    aria-label="Feed actions"
    type="button"
    onclick={() => (menuOpen = !menuOpen)}
  >⋯</button>

  {#if menuOpen}
    <div class="menu" role="menu">
      <button
        type="button"
        onclick={() => { settingsOpen = true; menuOpen = false; }}>Edit settings…</button>
      <details>
        <summary>Move to category</summary>
        <button type="button" onclick={() => assignCategory(null)}>— Uncategorised —</button>
        {#each cats as c (c.id)}
          <button type="button" onclick={() => assignCategory(c.id)}>{c.name}</button>
        {/each}
      </details>
      <button type="button" onclick={() => (confirmingDelete = true)}>Delete feed</button>
    </div>
  {/if}

  {#if confirmingDelete}
    <div class="confirm">
      Delete "{subscription.title}"?
      <button type="button" onclick={doDelete}>Confirm</button>
      <button type="button" onclick={() => (confirmingDelete = false)}>Cancel</button>
    </div>
  {/if}
</div>

{#if settingsOpen}
  <FeedSettingsModal
    {subscription}
    categories={cats}
    onClose={() => { settingsOpen = false; }}
    onSaved={() => { settingsOpen = false; void subscriptions.load(); }}
  />
{/if}

<style>
  .feedrow {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 20px;
    position: relative;
  }
  .feedname {
    flex: 1;
    font-family: var(--sans);
    font-size: 13px;
    color: var(--ink-2);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .row-actions {
    background: none;
    border: 0;
    cursor: pointer;
    color: var(--ink-3);
    font-size: 14px;
    padding: 0 6px;
    opacity: 0;
    transition: opacity 0.1s;
  }
  .feedrow:hover .row-actions,
  .feedrow:focus-within .row-actions {
    opacity: 1;
  }
  .menu {
    position: absolute;
    right: 8px;
    top: 28px;
    z-index: 5;
    background: var(--surface, var(--bg));
    border: 1px solid var(--rule);
    padding: 4px 0;
    min-width: 180px;
  }
  .menu button,
  .menu summary {
    display: block;
    width: 100%;
    text-align: left;
    padding: 6px 12px;
    background: none;
    border: 0;
    cursor: pointer;
    font-family: var(--sans);
    font-size: 12px;
    color: var(--ink);
  }
  .menu button:hover,
  .menu summary:hover {
    background: var(--bg-soft);
  }
  .confirm {
    position: absolute;
    right: 8px;
    top: 28px;
    z-index: 5;
    background: var(--surface, var(--bg));
    border: 1px solid var(--rule);
    padding: 8px 12px;
    font-family: var(--sans);
    font-size: 12px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 180px;
  }
  .confirm button {
    font-family: var(--sans);
    font-size: 12px;
    padding: 3px 8px;
    border: 1px solid var(--rule);
    background: var(--bg);
    cursor: pointer;
    border-radius: 3px;
  }
</style>
