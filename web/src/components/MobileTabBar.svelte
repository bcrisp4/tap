<script lang="ts">
  import MobileMoreSheet from './MobileMoreSheet.svelte';
  import { route, navigate } from '../lib/router';
  type Props = { unreadCount: number };
  let { unreadCount }: Props = $props();

  let moreOpen = $state(false);
  const activeName = $derived(
    $route.name === 'history' || $route.name === 'settings' || $route.name === 'admin' ? 'more' :
    $route.name === 'reader' ? 'unread' : $route.name,
  );

  const tabs = [
    { name: 'unread',     label: 'Unread',     path: '/' },
    { name: 'saved',      label: 'Saved',      path: '/saved' },
    { name: 'feeds',      label: 'Feeds',      path: '/feeds' },
    { name: 'categories', label: 'Categories', path: '/categories' },
    { name: 'more',       label: 'More',       path: null },
  ];

  function go(t: typeof tabs[number]) {
    if (t.name === 'more') { moreOpen = true; return; }
    moreOpen = false;
    if (t.path) navigate(t.path);
  }
</script>

<nav class="tmnav" aria-label="Primary">
  {#each tabs as t (t.name)}
    <button
      type="button"
      aria-label={t.label}
      aria-current={activeName === t.name ? 'page' : undefined}
      class="tmnav-tab"
      class:is-active={activeName === t.name}
      onclick={() => go(t)}
    >
      <span class="lbl">{t.label}</span>
      {#if t.name === 'unread' && unreadCount > 0}
        <span class="badge" aria-hidden="true">{unreadCount > 99 ? '99+' : unreadCount}</span>
      {/if}
    </button>
  {/each}
</nav>

<MobileMoreSheet open={moreOpen} onClose={() => moreOpen = false} />

<style>
  .tmnav {
    display: grid; grid-template-columns: repeat(5, 1fr);
    border-top: 1px solid var(--rule);
    background: var(--bg);
    padding: 8px 0 calc(env(safe-area-inset-bottom, 0px) + 12px);
    position: fixed; bottom: 0; left: 0; right: 0;
    z-index: 10;
  }
  .tmnav-tab {
    display: flex; flex-direction: column; align-items: center; gap: 4px;
    padding: 6px 0 4px;
    font-family: var(--sans); font-size: 11px; font-weight: 500;
    color: var(--ink-3);
    min-height: 44px;
    position: relative;
    background: transparent; border: 0; cursor: pointer;
  }
  .tmnav-tab.is-active { color: var(--ink); }
  .tmnav-tab.is-active::before {
    content: "";
    position: absolute;
    top: 0; left: 50%; transform: translateX(-50%);
    width: 4px; height: 4px; border-radius: 50%;
    background: var(--accent);
  }
  .badge {
    font-family: var(--mono);
    font-size: 9.5px;
    color: var(--ink-3);
  }
</style>
