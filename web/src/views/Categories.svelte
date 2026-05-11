<script lang="ts">
  import { onMount } from 'svelte';
  import { categories, subscriptions, entries } from '../lib/store';
  import { isMobile } from '../lib/breakpoints.svelte';
  import type { Category, Subscription } from '../lib/types';
  import CategoryCard from '../components/CategoryCard.svelte';
  import CategoryDeleteDialog from '../components/CategoryDeleteDialog.svelte';
  import CategoryMarkReadDialog from '../components/CategoryMarkReadDialog.svelte';

  let creating = $state(false);
  let newName = $state('');
  let newInput = $state<HTMLInputElement | null>(null);
  $effect(() => { if (creating && newInput) newInput.focus(); });

  let confirmDelete = $state<{ category: Category } | null>(null);
  let confirmMarkRead = $state<{ category: Category | { id: null; name: string } } | null>(null);

  onMount(() => {
    void categories.load();
    void subscriptions.load();
    void entries.load();
  });

  const catList = $derived($categories.slice().sort((a, b) => a.position - b.position));
  const uncatFeeds = $derived($subscriptions.filter((s: Subscription) => s.category_id == null));
  const uncatUnread = $derived.by(() => {
    const uncatIds = new Set(uncatFeeds.map((s: Subscription) => s.id));
    return $entries.items.filter(e => !e.read && uncatIds.has(e.subscription_id)).length;
  });

  const feedsByCategory = $derived.by(() => {
    const m = new Map<number, Subscription[]>();
    for (const s of $subscriptions) {
      if (s.category_id != null) {
        const arr = m.get(s.category_id) ?? [];
        arr.push(s);
        m.set(s.category_id, arr);
      }
    }
    return m;
  });

  function feedsFor(catId: number): Subscription[] { return feedsByCategory.get(catId) ?? []; }

  async function commitCreate() {
    const v = newName.trim();
    if (!v) { creating = false; newName = ''; return; }
    await categories.create(v);
    newName = '';
    creating = false;
  }
  function cancelCreate() { creating = false; newName = ''; }

  async function reorderUp(catId: number) {
    const ids = catList.map((c: Category) => c.id);
    const idx = ids.indexOf(catId);
    if (idx <= 0) return;
    [ids[idx - 1], ids[idx]] = [ids[idx], ids[idx - 1]];
    await categories.reorder(ids);
  }
  async function reorderDown(catId: number) {
    const ids = catList.map((c: Category) => c.id);
    const idx = ids.indexOf(catId);
    if (idx === -1 || idx >= ids.length - 1) return;
    [ids[idx], ids[idx + 1]] = [ids[idx + 1], ids[idx]];
    await categories.reorder(ids);
  }

  async function performMarkRead() {
    if (!confirmMarkRead) return;
    const target = confirmMarkRead.category;
    confirmMarkRead = null;
    await categories.markRead(target.id);
  }

  async function performDelete() {
    if (!confirmDelete) return;
    const id = confirmDelete.category.id;
    confirmDelete = null;
    await categories.remove(id);
  }
</script>

<main class="ts-main">
  <div class="ts-cats">
    <header class="ts-set-head">
      <div class="ts-set-eyebrow">
        Organisation · {catList.length} {catList.length === 1 ? 'category' : 'categories'}
      </div>
      <h1 class="ts-set-title">Categories</h1>
    </header>

    <div class="ts-cats-toolbar">
      <span class="ts-cats-toolbar-l">
        <b>{catList.length}</b><span>{catList.length === 1 ? 'category' : 'categories'}</span>
        <span class="dot" aria-hidden="true"></span>
        <b>{$subscriptions.length}</b><span>{$subscriptions.length === 1 ? 'feed' : 'feeds'}</span>
        {#if uncatFeeds.length > 0}
          <span class="dot" aria-hidden="true"></span>
          <b>{uncatFeeds.length}</b><span>uncategorised</span>
        {/if}
      </span>
      {#if !creating && catList.length > 0}
        <button class="ts-cats-new" onclick={() => creating = true}>New category</button>
      {/if}
    </div>

    {#if creating}
      <div class="ts-cats-newrow">
        <input
          bind:this={newInput}
          bind:value={newName}
          class="ts-cats-newinput"
          placeholder="Name the category…"
          onkeydown={(e) => {
            if (e.key === 'Enter') void commitCreate();
            if (e.key === 'Escape') cancelCreate();
          }}
          onblur={() => { if (!newName.trim()) cancelCreate(); }}
        />
        <div class="ts-cats-newrow-hint">
          <span class="k">↵</span> create · <span class="k">esc</span> cancel
        </div>
      </div>
    {/if}

    {#if catList.length === 0 && !creating}
      <div class="ts-cats-empty">
        <div class="ts-cats-empty-mark" aria-hidden="true">
          <span class="ts-cats-empty-dot"></span>
        </div>
        <div class="ts-cats-empty-title">No categories yet.</div>
        <p class="ts-cats-empty-sub">
          Categories group your {$subscriptions.length} {$subscriptions.length === 1 ? 'feed' : 'feeds'} into named sections.
          Add one to start sorting; feeds you don't assign keep working as normal.
        </p>
        <div class="ts-cats-empty-examples">
          <span class="ts-cats-empty-example">People</span>
          <span class="ts-cats-empty-example">Systems &amp; PL</span>
          <span class="ts-cats-empty-example">Letters</span>
        </div>
        <button class="ts-cats-empty-cta" onclick={() => creating = true}>
          Create your first category
        </button>
      </div>
    {:else}
      <div class="ts-cats-list">
        {#each catList as cat, i (cat.id)}
          <CategoryCard
            category={cat}
            feeds={feedsFor(cat.id)}
            unread={cat.unread}
            allCategories={catList}
            isMobile={$isMobile}
            isFirst={i === 0}
            isLast={i === catList.length - 1}
            onRename={(name) => void categories.rename(cat.id, name).catch(console.error)}
            onDelete={() => confirmDelete = { category: cat }}
            onMarkRead={() => confirmMarkRead = { category: cat }}
            onReorderUp={() => void reorderUp(cat.id).catch(console.error)}
            onReorderDown={() => void reorderDown(cat.id).catch(console.error)}
            onReassignFeed={(subId, newCatId) => void categories.reassignSubscription(subId, newCatId).catch(console.error)}
          />
        {/each}

        {#if uncatFeeds.length > 0}
          <CategoryCard
            category={{ id: -1, name: 'Uncategorised', unread: 0, created_at: 0, position: Number.MAX_SAFE_INTEGER }}
            feeds={uncatFeeds}
            unread={uncatUnread}
            allCategories={catList}
            isUncategorised={true}
            isMobile={$isMobile}
            isFirst={false}
            isLast={true}
            onRename={() => {}}
            onDelete={() => {}}
            onMarkRead={() => confirmMarkRead = { category: { id: null, name: 'Uncategorised' } }}
            onReorderUp={() => {}}
            onReorderDown={() => {}}
            onReassignFeed={(subId, newCatId) => void categories.reassignSubscription(subId, newCatId).catch(console.error)}
          />
        {/if}
      </div>
    {/if}
  </div>

  {#if confirmDelete}
    <CategoryDeleteDialog
      categoryName={confirmDelete.category.name}
      affectedFeeds={feedsFor(confirmDelete.category.id)}
      onCancel={() => confirmDelete = null}
      onConfirm={performDelete}
    />
  {/if}

  {#if confirmMarkRead}
    <CategoryMarkReadDialog
      categoryName={confirmMarkRead.category.name}
      unread={confirmMarkRead.category.id == null ? uncatUnread : (confirmMarkRead.category as Category).unread}
      feedCount={confirmMarkRead.category.id == null ? uncatFeeds.length : feedsFor((confirmMarkRead.category as Category).id).length}
      onCancel={() => confirmMarkRead = null}
      onConfirm={performMarkRead}
    />
  {/if}
</main>

<style>
  .ts-main {
    flex: 1;
    min-width: 0;
    overflow-y: auto;
    padding: 0 40px 80px;
  }
  .ts-cats { max-width: 860px; margin: 0 auto; padding-top: 40px; }
  .ts-set-head { margin-bottom: 28px; }
  .ts-set-eyebrow {
    font-family: var(--mono); font-size: 10px; letter-spacing: 0.14em;
    text-transform: uppercase; color: var(--ink-3);
    margin-bottom: 6px;
  }
  .ts-set-title {
    font-family: var(--serif); font-size: 36px; font-weight: 700;
    letter-spacing: -0.025em; color: var(--ink); margin: 0; line-height: 1.05;
  }
  .ts-cats-toolbar {
    display: flex; align-items: center; justify-content: space-between;
    margin-bottom: 8px; padding-bottom: 12px;
    border-bottom: 1px solid var(--rule);
  }
  .ts-cats-toolbar-l {
    display: inline-flex; align-items: baseline; gap: 6px;
    font-family: var(--mono); font-size: 10.5px; color: var(--ink-3);
    text-transform: uppercase; letter-spacing: 0.04em;
  }
  .ts-cats-toolbar-l b {
    color: var(--ink); font-weight: 500; font-size: 13px;
    letter-spacing: 0; text-transform: none; font-feature-settings: "tnum";
  }
  .ts-cats-toolbar-l .dot {
    display: inline-block; width: 3px; height: 3px;
    border-radius: 50%; background: var(--ink-4);
    align-self: center; margin: 0 2px;
  }
  .ts-cats-new {
    font-family: var(--mono); font-size: 10px; letter-spacing: 0.08em;
    text-transform: uppercase; color: var(--ink-3);
    background: transparent; border: 0;
    padding: 6px 10px; cursor: pointer; border-radius: 3px;
    transition: color 100ms ease, background 100ms ease;
  }
  .ts-cats-new:hover { color: var(--ink); background: var(--bg-soft); }
  .ts-cats-newrow {
    display: flex; align-items: center; gap: 12px;
    padding: 18px 0; border-bottom: 1px solid var(--rule);
  }
  .ts-cats-newinput {
    flex: 1; font-family: var(--serif); font-size: 24px; font-weight: 600;
    letter-spacing: -0.02em; color: var(--ink); background: var(--bg);
    border: 0; border-bottom: 1px solid var(--accent); outline: none;
    padding: 0 0 3px; min-width: 0;
  }
  .ts-cats-newrow-hint {
    font-family: var(--mono); font-size: 10px; letter-spacing: 0.06em;
    color: var(--ink-3); white-space: nowrap; flex-shrink: 0;
  }
  .ts-cats-newrow-hint .k {
    background: var(--bg-soft); border: 1px solid var(--rule);
    border-radius: 3px; padding: 1px 4px; font-size: 9px;
  }

  .ts-cats-empty {
    padding: 60px 0 40px;
    display: flex; flex-direction: column; align-items: center; text-align: center;
    gap: 0;
  }
  .ts-cats-empty-mark {
    width: 24px; height: 24px; display: flex; align-items: center; justify-content: center;
    margin-bottom: 18px;
  }
  .ts-cats-empty-dot {
    display: block; width: 8px; height: 8px; border-radius: 50%;
    background: var(--ink-4);
  }
  .ts-cats-empty-title {
    font-family: var(--serif); font-size: 22px; font-weight: 600;
    letter-spacing: -0.015em; color: var(--ink); margin-bottom: 10px;
  }
  .ts-cats-empty-sub {
    font-family: var(--serif); font-size: 15px; line-height: 1.55;
    color: var(--ink-2); max-width: 400px; margin: 0 0 20px;
  }
  .ts-cats-empty-examples {
    display: flex; gap: 8px; flex-wrap: wrap;
    justify-content: center; margin-bottom: 24px;
  }
  .ts-cats-empty-example {
    font-family: var(--mono); font-size: 10.5px; letter-spacing: 0.04em;
    text-transform: uppercase; color: var(--ink-3);
    border: 1px solid var(--rule); border-radius: 3px;
    padding: 4px 10px;
  }
  .ts-cats-empty-cta {
    font-family: var(--mono); font-size: 10.5px; letter-spacing: 0.08em;
    text-transform: uppercase; color: var(--bg);
    background: var(--ink); border: 0; border-radius: 4px;
    padding: 10px 20px; cursor: pointer;
    transition: opacity 100ms ease;
  }
  .ts-cats-empty-cta:hover { opacity: 0.85; }
</style>
