<script lang="ts">
  import type { Category, Subscription } from '../lib/types';
  import CategoryReassignPopover from './CategoryReassignPopover.svelte';
  import CategoryReassignSheet from './CategoryReassignSheet.svelte';

  type Props = {
    category: Category;
    feeds: Subscription[];
    unread: number;
    allCategories?: Category[];
    isFirst: boolean;
    isLast: boolean;
    isUncategorised?: boolean;
    isMobile?: boolean;
    onRename: (newName: string) => void | Promise<void>;
    onDelete: () => void;
    onMarkRead: () => void;
    onReorderUp: () => void;
    onReorderDown: () => void;
    onReassignFeed: (subscriptionId: number, newCategoryId: number | null) => void;
  };
  const {
    category, feeds, unread, allCategories = [],
    isFirst, isLast, isUncategorised = false, isMobile = false,
    onRename, onDelete, onMarkRead, onReorderUp, onReorderDown, onReassignFeed,
  }: Props = $props();

  let renaming = $state(false);
  let renameValue = $state('');
  let renameInput = $state<HTMLInputElement | null>(null);
  let openPickerFeedId = $state<number | null>(null);
  const triggerByFeedId: Record<number, HTMLButtonElement | null> = $state({});

  function beginRename() {
    if (isUncategorised) return;
    renameValue = category.name;
    renaming = true;
  }
  function cancelRename() { renaming = false; }
  async function commitRename() {
    const v = renameValue.trim();
    if (!v) { renaming = false; return; }
    try {
      await onRename(v);
      renaming = false;
    } catch (e) {
      console.error('rename failed:', e);
    }
  }

  $effect(() => { if (renaming && renameInput) renameInput.focus(); });

  function onCardKey(e: KeyboardEvent) {
    if (renaming) return;
    if ((e.key === 'r' || e.key === 'R') && !isUncategorised) {
      e.preventDefault();
      beginRename();
    }
  }
</script>

<section
  class="ts-cat"
  class:is-renaming={renaming}
  class:is-uncat={isUncategorised}
  class:m-cat-card={isMobile}
  tabindex="0"
  onkeydown={onCardKey}
>
  <header class="ts-cat-head">
    {#if renaming}
      <div class="ts-cat-rename-row">
        <input
          bind:this={renameInput}
          bind:value={renameValue}
          class="ts-cat-title-input"
          aria-label="Rename category"
          onkeydown={(e) => {
            if (e.key === 'Enter') commitRename();
            if (e.key === 'Escape') cancelRename();
          }}
          onblur={() => { if (renameValue.trim()) commitRename(); else cancelRename(); }}
        />
        <span class="ts-cat-rename-hint">↵ save · esc cancel</span>
      </div>
    {:else}
      <h2 class="ts-cat-title" onclick={beginRename}>
        <span>{category.name}</span>
        {#if !isUncategorised}<span class="ts-cat-title-edit">click to rename</span>{/if}
      </h2>
    {/if}
    <div class="ts-cat-stats" class:is-zero={unread === 0}>
      <span class="ll">{unread}</span><span>unread</span>
      <span class="dot" aria-hidden="true"></span>
      <b>{feeds.length}</b><span>{feeds.length === 1 ? 'feed' : 'feeds'}</span>
    </div>
  </header>

  {#if isUncategorised}
    <p class="ts-cat-uncat-note">
      Feeds without a category &mdash; still subscribed, still appear in Unread.
    </p>
  {/if}

  {#if feeds.length === 0}
    <div class="ts-cat-empty">
      No feeds here yet. Move one in from another category, or from <i>Uncategorised</i> below.
    </div>
  {:else}
    <ul class="ts-cat-feeds">
      {#each feeds as f (f.id)}
        <li class="ts-cat-feed">
          <span class="ts-cat-feed-name">{f.title}</span>
          <span class="ts-cat-feed-url">{f.feed_url}</span>
          <span class="ts-cat-feed-right">
            <button
              class="ts-cat-feed-pick"
              class:is-open={openPickerFeedId === f.id}
              aria-expanded={openPickerFeedId === f.id}
              aria-haspopup="menu"
              bind:this={triggerByFeedId[f.id]}
              onclick={() => openPickerFeedId = openPickerFeedId === f.id ? null : f.id}
            >
              <span>{isUncategorised ? 'Assign' : 'Move'}</span>
            </button>
            {#if !isMobile}
              <CategoryReassignPopover
                open={openPickerFeedId === f.id}
                anchor={triggerByFeedId[f.id] ?? null}
                feedName={f.title}
                currentCategoryId={f.category_id}
                categories={allCategories}
                label={isUncategorised ? 'Assign' : 'Move'}
                onPick={(catId) => {
                  openPickerFeedId = null;
                  onReassignFeed(f.id, catId);
                }}
                onClose={() => { if (openPickerFeedId === f.id) openPickerFeedId = null; }}
              />
            {/if}
          </span>
        </li>
      {/each}
    </ul>
  {/if}

  <footer class="ts-cat-actions">
    <button
      class="ts-cat-action"
      onclick={onMarkRead}
      disabled={unread === 0}
      aria-label={unread === 0 ? 'Nothing unread' : `Mark ${unread} read`}
    >
      {unread === 0 ? 'Nothing unread' : `Mark ${unread} read`}
    </button>
    {#if !isUncategorised}
      <button class="ts-cat-action" onclick={beginRename} aria-label="Rename">Rename</button>
      <button
        class="ts-cat-action"
        onclick={onReorderUp}
        disabled={isFirst}
        aria-label="Reorder up"
      >Reorder up</button>
      <button
        class="ts-cat-action"
        onclick={onReorderDown}
        disabled={isLast}
        aria-label="Reorder down"
      >Reorder down</button>
      <button class="ts-cat-action is-danger" onclick={onDelete} aria-label="Delete">Delete</button>
    {/if}
  </footer>

  {#if isMobile && openPickerFeedId !== null}
    {@const sheetFeed = feeds.find(x => x.id === openPickerFeedId)}
    {#if sheetFeed}
      <CategoryReassignSheet
        open={true}
        feedName={sheetFeed.title}
        currentCategoryId={sheetFeed.category_id}
        categories={allCategories}
        onSelect={(catId) => onReassignFeed(sheetFeed.id, catId)}
        onClose={() => openPickerFeedId = null}
      />
    {/if}
  {/if}
</section>

<style>
  .ts-cat {
    padding: 26px 2px 18px;
    border-top: 1px solid var(--rule);
    position: relative;
  }
  .ts-cat:last-child { border-bottom: 1px solid var(--rule); }
  .ts-cat:focus { outline: none; }
  .ts-cat:focus-visible { box-shadow: inset 0 0 0 2px var(--accent); }
  .ts-cat.is-renaming { background: var(--accent-soft); }
  .ts-cat-head {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: baseline;
    gap: 16px;
    margin-bottom: 16px;
  }
  .ts-cat-title {
    font-family: var(--serif);
    font-size: 28px;
    line-height: 1.1;
    font-weight: 600;
    color: var(--ink);
    letter-spacing: -0.02em;
    margin: 0;
    cursor: text;
    display: inline-flex;
    align-items: baseline;
    gap: 10px;
    min-width: 0;
  }
  .ts-cat-title:hover .ts-cat-title-edit { opacity: 1; }
  .ts-cat-title-edit {
    font-family: var(--mono);
    font-size: 9.5px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ink-3);
    opacity: 0;
    transition: opacity 100ms ease;
    font-weight: 400;
    align-self: center;
  }
  .ts-cat-title-input {
    flex: 1;
    font-family: var(--serif);
    font-size: 28px;
    font-weight: 600;
    letter-spacing: -0.02em;
    color: var(--ink);
    background: var(--bg);
    border: 0;
    border-bottom: 1px solid var(--accent);
    outline: none;
    padding: 0 0 3px;
    min-width: 0;
    width: 100%;
  }
  .ts-cat-rename-row {
    display: flex; align-items: center; gap: 10px;
    width: 100%;
  }
  .ts-cat-rename-hint {
    font-family: var(--mono);
    font-size: 10px;
    letter-spacing: 0.06em;
    color: var(--ink-3);
    white-space: nowrap;
    flex-shrink: 0;
  }
  .ts-cat-stats {
    display: inline-flex;
    align-items: baseline;
    gap: 6px;
    font-family: var(--mono);
    font-size: 10.5px;
    color: var(--ink-3);
    letter-spacing: 0.02em;
    white-space: nowrap;
    text-transform: uppercase;
  }
  .ts-cat-stats b {
    color: var(--ink); font-weight: 500; font-size: 14px;
    letter-spacing: 0; text-transform: none;
    font-feature-settings: "tnum";
  }
  .ts-cat-stats .dot {
    display: inline-block; width: 3px; height: 3px;
    border-radius: 50%; background: var(--ink-4);
    align-self: center; margin: 0 4px;
  }
  .ts-cat-stats .ll {
    color: var(--accent); font-size: 14px; font-weight: 500;
    font-feature-settings: "tnum";
  }
  .ts-cat-stats.is-zero b, .ts-cat-stats.is-zero .ll { color: var(--ink-3); }
  .ts-cat-empty {
    padding: 14px 0 18px;
    font-family: var(--mono); font-size: 11px;
    color: var(--ink-3); letter-spacing: 0.04em;
  }
  .ts-cat-feeds {
    list-style: none; margin: 0 0 14px; padding: 0;
    display: flex; flex-direction: column;
  }
  .ts-cat-feed {
    display: grid;
    grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr) auto;
    gap: 14px;
    align-items: center;
    padding: 11px 4px 11px 2px;
    border-bottom: 1px solid var(--rule);
    position: relative;
  }
  .ts-cat-feed:last-child { border-bottom: 0; }
  .ts-cat-feed:hover { background: var(--bg-soft); }
  .ts-cat-feed-name {
    font-family: var(--sans); font-size: 13.5px; font-weight: 500;
    color: var(--ink); white-space: nowrap;
    overflow: hidden; text-overflow: ellipsis;
  }
  .ts-cat-feed-url {
    font-family: var(--mono); font-size: 10.5px;
    color: var(--ink-3); white-space: nowrap;
    overflow: hidden; text-overflow: ellipsis;
  }
  .ts-cat-feed-right {
    display: inline-flex; align-items: center; gap: 14px;
    position: relative; white-space: nowrap;
  }
  .ts-cat-feed-pick {
    display: inline-flex; align-items: center; gap: 5px;
    font-family: var(--mono); font-size: 9.5px;
    letter-spacing: 0.08em; text-transform: uppercase;
    color: var(--ink-3); background: transparent;
    border: 0; padding: 4px 6px;
    cursor: pointer; border-radius: 3px;
    transition: color 100ms ease, background 100ms ease;
  }
  .ts-cat-feed-pick:hover, .ts-cat-feed-pick.is-open {
    color: var(--ink); background: var(--bg);
  }
  .ts-cat-actions {
    display: flex; align-items: center; gap: 0;
    margin-top: 12px; padding-top: 4px; flex-wrap: wrap;
  }
  .ts-cat-action {
    display: inline-flex; align-items: center; gap: 6px;
    font-family: var(--mono); font-size: 10px;
    letter-spacing: 0.08em; text-transform: uppercase;
    color: var(--ink-3); background: transparent;
    border: 0; padding: 6px 14px 6px 0;
    margin-right: 6px; cursor: pointer;
    border-radius: 0; position: relative;
    transition: color 100ms ease;
  }
  .ts-cat-action:hover { color: var(--ink); }
  .ts-cat-action.is-danger:hover { color: #c43a3a; }
  :global(html.theme-dark) .ts-cat-action.is-danger:hover { color: #ec7a7a; }
  .ts-cat-action:disabled { color: var(--ink-4); cursor: not-allowed; }
  .ts-cat-action:disabled:hover { color: var(--ink-4); }
  .ts-cat.is-uncat .ts-cat-title {
    color: var(--ink-3); font-weight: 500; font-style: italic;
  }
  .ts-cat-uncat-note {
    font-family: var(--serif); font-size: 14px; font-style: italic;
    color: var(--ink-3); margin: -6px 0 14px;
    max-width: 460px; line-height: 1.55;
  }
</style>
