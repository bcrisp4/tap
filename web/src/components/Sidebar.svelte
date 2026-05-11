<script lang="ts">
  import { onMount } from 'svelte';
  import FeedRow from './FeedRow.svelte';
  import SystemActions from './SystemActions.svelte';
  import AddFeedForm from './AddFeedForm.svelte';
  import { subscriptions, categories } from '../lib/store';
  import { navigate, route } from '../lib/router';
  import { api } from '../lib/api';

  // Inline category creation state.
  let newCatName = $state('');
  let creating = $state(false);
  let createError = $state('');
  let actionError = $state(''); // rename/delete errors rendered outside the create form
  let newCatInputEl = $state<HTMLInputElement | null>(null);
  let editInputEl = $state<HTMLInputElement | null>(null);

  // Per-category edit state.
  let editingCatId = $state<number | null>(null);
  let editName = $state('');
  let confirmDeleteId = $state<number | null>(null);

  $effect(() => {
    if (creating && newCatInputEl) newCatInputEl.focus();
  });

  $effect(() => {
    if (editingCatId !== null && editInputEl) editInputEl.focus();
  });

  // Feeds by category — memoized as a derived value, not a function.
  const catFeeds = $derived.by(() => {
    const map = new Map<number | null, typeof $subscriptions>();
    for (const s of $subscriptions) {
      const k = s.category_id ?? null;
      if (!map.has(k)) map.set(k, []);
      map.get(k)!.push(s);
    }
    return map;
  });

  onMount(() => {
    void categories.load();
  });

  async function createCategory() {
    const name = newCatName.trim();
    if (!name) return;
    creating = true;
    createError = '';
    try {
      await api.createCategory(name);
      newCatName = '';
      await categories.load();
    } catch (e) {
      createError = e instanceof Error ? e.message : 'Failed to create category';
    } finally {
      creating = false;
    }
  }

  async function renameCategory(id: number, name: string) {
    actionError = '';
    try {
      await api.renameCategory(id, name);
      editingCatId = null;
      await categories.load();
    } catch (e) {
      actionError = e instanceof Error ? e.message : 'Failed to rename';
    }
  }

  async function deleteCategory(id: number) {
    actionError = '';
    try {
      await api.deleteCategory(id);
      confirmDeleteId = null;
      await Promise.all([categories.load(), subscriptions.load()]);
    } catch (e) {
      actionError = e instanceof Error ? e.message : 'Failed to delete';
    }
  }
</script>

<nav class="sidebar tap-sidebar" aria-label="Sidebar navigation">
  <div class="brand">tap<span class="dot">.</span></div>

  <div class="group-title">READING</div>
  <a class="navitem" class:active={$route.name === 'unread'} href="/" onclick={(e) => { e.preventDefault(); navigate('/'); }}>Unread</a>
  <a class="navitem" class:active={$route.name === 'saved'} href="/saved" onclick={(e) => { e.preventDefault(); navigate('/saved'); }}>Saved</a>
  <a class="navitem" class:active={$route.name === 'search'} href="/search" onclick={(e) => { e.preventDefault(); navigate('/search'); }}>Search</a>

  {#if actionError}
    <span class="cat-error" role="alert">{actionError}</span>
  {/if}

  <div class="group-header">
    <div class="group-title">FEEDS</div>
    <button class="add-cat-btn" title="New category" aria-label="Create category"
      onclick={() => { creating = !creating; createError = ''; actionError = ''; }}>+</button>
  </div>

  {#if creating}
    <form class="new-cat-form" onsubmit={(e) => { e.preventDefault(); void createCategory(); }}>
      <input
        class="cat-input"
        bind:this={newCatInputEl}
        bind:value={newCatName}
        placeholder="Category name"
        onblur={() => { if (!newCatName.trim()) { creating = false; } }}
        onkeydown={(e) => { if (e.key === 'Escape') { creating = false; newCatName = ''; } }}
      />
      {#if createError}<span class="cat-error">{createError}</span>{/if}
    </form>
  {/if}

  {#each $categories as cat (cat.id)}
    <div class="cat-row">
      {#if editingCatId === cat.id}
        <form class="edit-form" onsubmit={(e) => { e.preventDefault(); void renameCategory(cat.id, editName); }}>
          <input class="cat-input" bind:this={editInputEl} bind:value={editName}
            onblur={() => { if (editName.trim()) void renameCategory(cat.id, editName); else editingCatId = null; }}
            onkeydown={(e) => { if (e.key === 'Escape') editingCatId = null; }}
          />
        </form>
      {:else if confirmDeleteId === cat.id}
        <div class="confirm-delete">
          <span class="confirm-msg">Delete "{cat.name}"? Feeds will be uncategorised.</span>
          <button class="btn-yes" onclick={() => void deleteCategory(cat.id)}>Delete</button>
          <button class="btn-no" onclick={() => { confirmDeleteId = null; }}>Cancel</button>
        </div>
      {:else}
        <a class="cat-name" class:active={$route.name === 'category' && $route.params.id === cat.id}
          href="/categories/{cat.id}"
          onclick={(e) => { e.preventDefault(); navigate(`/categories/${cat.id}`); }}>
          {cat.name}
          {#if cat.unread > 0}<span class="cat-unread">{cat.unread}</span>{/if}
        </a>
        <div class="cat-actions">
          <button class="cat-action-btn" title="Rename" aria-label="Rename {cat.name}"
            onclick={() => { editingCatId = cat.id; editName = cat.name; }}>✎</button>
          <button class="cat-action-btn" title="Delete" aria-label="Delete {cat.name}"
            onclick={() => { confirmDeleteId = cat.id; }}>✕</button>
        </div>
      {/if}
    </div>

    {#each catFeeds.get(cat.id) ?? [] as sub (sub.id)}
      <div class="nested-feed">
        <FeedRow subscription={sub} categories={$categories} />
      </div>
    {/each}
  {/each}

  {#if (catFeeds.get(null) ?? []).length > 0}
    {#if $categories.length > 0}
      <div class="group-title uncat-header">UNCATEGORISED</div>
    {/if}
    {#each catFeeds.get(null) ?? [] as sub (sub.id)}
      <FeedRow subscription={sub} categories={$categories} />
    {/each}
  {/if}

  <div class="group-title">SYSTEM</div>
  <a class="navitem" class:active={$route.name === 'settings'} href="/settings"
    onclick={(e) => { e.preventDefault(); navigate('/settings'); }}>Settings</a>
  <SystemActions />
  <AddFeedForm />
</nav>

<style>
  .sidebar {
    width: 240px;
    border-right: 1px solid var(--rule);
    background: var(--bg);
    height: 100vh;
    overflow-y: auto;
    flex: none;
  }
  .brand {
    font-family: var(--sans);
    font-weight: 600;
    font-size: 17px;
    padding: 18px 20px 8px;
  }
  .brand .dot { color: var(--accent); font-weight: 700; margin-left: 1px; }
  .group-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-right: 12px;
  }
  .group-title {
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--ink-3);
    padding: 16px 20px 6px;
  }
  .uncat-header { padding-top: 12px; }
  .add-cat-btn {
    font-size: 16px;
    color: var(--ink-3);
    background: none;
    border: none;
    cursor: pointer;
    padding: 2px 4px;
    line-height: 1;
  }
  .add-cat-btn:hover { color: var(--accent); }
  .new-cat-form, .edit-form { padding: 4px 12px; }
  .cat-input {
    width: 100%;
    box-sizing: border-box;
    padding: 4px 8px;
    font-family: var(--sans);
    font-size: 12px;
    background: var(--bg);
    color: var(--ink);
    border: 1px solid var(--accent);
    border-radius: 3px;
    outline: none;
  }
  .cat-error { font-size: 10px; color: var(--error, #c0392b); padding: 2px 8px; display: block; }
  .cat-row {
    display: flex;
    align-items: center;
    padding: 3px 8px 3px 20px;
    gap: 4px;
  }
  .cat-row:hover .cat-actions { opacity: 1; }
  .cat-name {
    flex: 1;
    font-family: var(--sans);
    font-size: 13px;
    color: var(--ink);
    text-decoration: none;
    display: flex;
    align-items: center;
    gap: 6px;
    border-left: 2px solid transparent;
    padding-left: 2px;
    border-radius: 0;
  }
  .cat-name.active { border-left-color: var(--accent); }
  .cat-unread {
    font-family: var(--mono);
    font-size: 10px;
    color: var(--accent);
  }
  .cat-actions {
    display: flex;
    gap: 2px;
    opacity: 0;
    transition: opacity 0.1s;
  }
  .cat-action-btn {
    background: none;
    border: none;
    cursor: pointer;
    font-size: 11px;
    color: var(--ink-3);
    padding: 2px 3px;
  }
  .cat-action-btn:hover { color: var(--accent); }
  .confirm-delete {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 4px 0;
    flex: 1;
  }
  .confirm-msg { font-size: 11px; color: var(--ink-2); font-family: var(--sans); }
  .btn-yes, .btn-no {
    font-size: 11px;
    font-family: var(--sans);
    padding: 2px 8px;
    border-radius: 3px;
    cursor: pointer;
    border: 1px solid var(--rule);
    background: var(--bg);
    color: var(--ink);
  }
  .btn-yes { color: var(--error, #c0392b); border-color: var(--error, #c0392b); }
  .navitem {
    display: block;
    padding: 6px 20px;
    font-family: var(--sans);
    font-size: 13px;
    color: var(--ink);
    border-left: 2px solid transparent;
    text-decoration: none;
  }
  .navitem.active { border-left-color: var(--accent); }
  .nested-feed :global(.feedrow) { padding-left: 32px; }
</style>
