<script lang="ts">
  import { route, navigate } from '../lib/router';
  type Props = { unreadCount: number; isAdmin: boolean };
  let { unreadCount, isAdmin }: Props = $props();

  const tabs = $derived([
    { name: 'unread',     label: 'Unread',     path: '/' },
    { name: 'saved',      label: 'Saved',      path: '/saved' },
    { name: 'history',    label: 'History',    path: '/history' },
    { name: 'categories', label: 'Categories', path: '/categories' },
    { name: 'feeds',      label: 'Feeds',      path: '/feeds' },
    { name: 'settings',   label: 'Settings',   path: '/settings' },
    ...(isAdmin ? [{ name: 'admin', label: 'Admin', path: '/admin' }] : []),
  ]);

  const activeName = $derived($route.name === 'reader' ? 'unread' : $route.name);
</script>

<nav class="nav" aria-label="Primary">
  <a class="wordmark" href="/" onclick={(e) => { e.preventDefault(); navigate('/'); }}>
    tap<span class="dot" aria-hidden="true"></span>
  </a>
  <ul class="tabs">
    {#each tabs as t (t.name)}
      <li>
        <button
          type="button"
          class="tab"
          class:is-active={activeName === t.name}
          onclick={() => navigate(t.path)}
        >
          <span>{t.label}</span>
          {#if t.name === 'unread' && unreadCount > 0}
            <span class="count">{unreadCount}</span>
          {/if}
        </button>
      </li>
    {/each}
  </ul>
</nav>

<style>
  .nav {
    display: flex; align-items: baseline; gap: 32px;
    padding: 0 2px 16px;
    border-bottom: 1px solid var(--rule);
    position: sticky; top: 0;
    background: var(--bg);
    z-index: 4;
  }
  .wordmark {
    font-family: var(--sans);
    font-weight: 600; font-size: 18px;
    letter-spacing: var(--tr-display, -0.02em);
    color: var(--ink);
    text-decoration: none;
    display: inline-flex; align-items: baseline; gap: 1px;
    padding: 6px 0;
    flex-shrink: 0;
  }
  .dot {
    display: inline-block;
    width: 5px; height: 5px; border-radius: 50%;
    background: var(--accent);
    transform: translateY(-1px);
    margin-left: 1px;
  }
  .tabs { list-style: none; margin: 0; padding: 0; display: flex; align-items: baseline; flex: 1; flex-wrap: wrap; }
  .tabs > li { display: inline-flex; align-items: baseline; }
  .tab {
    position: relative;
    display: inline-flex; align-items: baseline; gap: 8px;
    padding: 8px 12px;
    font-family: var(--mono);
    font-size: var(--fs-tab-nav, 11px);
    font-weight: 400;
    letter-spacing: var(--tr-tab-nav, 0.10em);
    text-transform: uppercase;
    color: var(--ink-3);
    background: transparent;
    border: 0; cursor: pointer;
    transition: color var(--dur-fast, 100ms) ease;
  }
  .tab:hover { color: var(--ink-2); }
  .tab.is-active { color: var(--ink); font-weight: 500; }
  .tab.is-active::after {
    content: "";
    position: absolute;
    left: 50%; transform: translateX(-50%);
    bottom: -18px;
    width: 20px; height: 2px;
    background: var(--ink);
  }
  .count {
    font-family: var(--mono);
    font-size: 10.5px;
    color: var(--ink-4);
    letter-spacing: 0;
    font-weight: 400;
  }
  .tab.is-active .count { color: var(--accent); }
</style>
