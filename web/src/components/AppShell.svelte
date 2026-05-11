<script lang="ts">
  import TopTabs from './TopTabs.svelte';
  import AccountAvatar from './AccountAvatar.svelte';
  import StatusFoot from './StatusFoot.svelte';
  import MobileTopBar from './MobileTopBar.svelte';
  import MobileTabBar from './MobileTabBar.svelte';
  import { auth } from '../lib/auth';
  import { route } from '../lib/router';
  import { entries } from '../lib/store';
  import { font } from '../lib/preferences.svelte';
  import { isMobile } from '../lib/breakpoints.svelte';

  type Props = { children: import('svelte').Snippet };
  let { children }: Props = $props();

  const unread = $derived($entries.items.filter(e => !e.read).length);
  const isAdmin = $derived($auth.user?.role === 'admin');

  const pageTitle = $derived(({
    unread: 'Unread', saved: 'Saved', history: 'History',
    categories: 'Categories', feeds: 'Feeds', settings: 'Settings',
    admin: 'Admin', reader: 'Reader',
  } as Record<string, string>)[$route.name] ?? '');
</script>

<div
  class="tap ts-root"
  class:is-mobile={$isMobile}
  class:font-sans={font.value === 'sans'}
>
  {#if $isMobile}
    <MobileTopBar title={pageTitle} count={$route.name === 'unread' ? unread : undefined} countLabel="unread" />
    <main class="mbody">{@render children()}</main>
    <MobileTabBar unreadCount={unread} />
  {:else}
    <div class="shell" class:shell-reader={$route.name === 'reader'}>
      <TopTabs unreadCount={unread} isAdmin={isAdmin} />
      <AccountAvatar />
      <main class="main">{@render children()}</main>
      {#if $route.name !== 'reader'}<StatusFoot />{/if}
    </div>
  {/if}
</div>

<style>
  .tap {
    background: var(--bg);
    color: var(--ink);
    min-height: 100vh;
    position: relative;
    font-family: var(--sans);
  }
  .shell {
    max-width: 720px;
    margin: 0 auto;
    padding: 36px 24px 80px;
    min-height: 100vh;
    display: flex;
    flex-direction: column;
  }
  .shell-reader { max-width: 720px; padding-bottom: 120px; }
  .main { flex: 1; padding-top: 4px; }
  .mbody { padding: 0; }
  .is-mobile { padding-bottom: calc(56px + env(safe-area-inset-bottom, 0px)); }
</style>
