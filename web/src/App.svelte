<script lang="ts">
  import { onMount, setContext } from 'svelte';
  import { get } from 'svelte/store';
  import { route, navigate } from './lib/router';
  import { auth } from './lib/auth';
  import { offlineQueue } from './lib/offlineQueue';
  import { warmCache } from './lib/warmCache';
  import { theme, font, density, measure } from './lib/preferences.svelte';
  import { buildHandler } from './lib/keyboard';
  import { subscriptions, entries } from './lib/store';
  import { searchOverlay } from './lib/searchOverlay.svelte';
  import { useRegisterSW } from 'virtual:pwa-register/svelte';

  import AppShell from './components/AppShell.svelte';
  import HotkeysModal from './components/HotkeysModal.svelte';
  import SearchOverlay from './components/SearchOverlay.svelte';

  import Login from './views/Login.svelte';
  import Unread from './views/Unread.svelte';
  import Reader from './views/Reader.svelte';
  import Saved from './views/Saved.svelte';
  import Categories from './views/Categories.svelte';
  import Feeds from './views/Feeds.svelte';
  import History from './views/History.svelte';
  import Settings from './views/Settings.svelte';
  import Admin from './views/Admin.svelte';

  const { needRefresh, updateServiceWorker } = useRegisterSW();
  let hotkeysOpen = $state(false);

  const dispatch = $state({
    onNext: () => {}, onPrev: () => {}, onOpen: () => {},
    onToggleRead: () => {}, onToggleSaved: () => {}, onViewOriginal: () => {},
  });
  setContext('keyDispatch', dispatch);

  const keyHandler = buildHandler({
    get onNext() { return dispatch.onNext; },
    get onPrev() { return dispatch.onPrev; },
    get onOpen() { return dispatch.onOpen; },
    get onToggleRead() { return dispatch.onToggleRead; },
    get onToggleSaved() { return dispatch.onToggleSaved; },
    get onViewOriginal() { return dispatch.onViewOriginal; },
    onEscape: () => {
      if (searchOverlay.open) { searchOverlay.close(); return; }
      if (hotkeysOpen) { hotkeysOpen = false; return; }
      if ($route.name === 'reader') navigate('/');
    },
    setModalOpen: (open: boolean) => { hotkeysOpen = open; },
    onMeasureNarrow: () => { if ($route.name === 'reader') measure.value = 'narrow'; },
    onMeasureComfortable: () => { if ($route.name === 'reader') measure.value = 'comfortable'; },
    onMeasureWide: () => { if ($route.name === 'reader') measure.value = 'wide'; },
    onBack: () => { if ($route.name === 'reader') navigate('/'); },
    onNavigate: (path: string) => { navigate(path); },
    onRefreshAll: () => {
      const ids = get(subscriptions).map((s) => s.id);
      if (ids.length > 0) void subscriptions.refreshMany(ids);
    },
    onMarkScopeRead: () => {
      const routeName = $route.name;
      let ids: number[] = [];
      if (routeName === 'unread' || routeName === 'history') {
        ids = get(entries).items.filter((e) => !e.read).map((e) => e.id);
      } else if (routeName === 'saved') {
        ids = get(entries).items.filter((e) => e.saved).map((e) => e.id);
      }
      if (ids.length === 0) return;
      const POOL = 4;
      void (async () => {
        for (let i = 0; i < ids.length; i += POOL) {
          await Promise.allSettled(ids.slice(i, i + POOL).map((id) => entries.toggleRead(id, true)));
        }
      })();
    },
    onCycleTheme: () => {
      const order = ['light', 'sepia', 'dark'] as const;
      const idx = order.indexOf(theme.resolved);
      theme.stored = order[(idx + 1) % order.length];
    },
  });

  onMount(() => {
    void auth.bootstrap().then(() => {
      const user = get(auth).user;
      if (user) {
        void offlineQueue.drain(user.id);
        setTimeout(() => { void warmCache(user.id); }, 2000);
      }
    });
    const handleOnline = async () => {
      const user = get(auth).user;
      if (user) {
        await offlineQueue.drain(user.id);
        void warmCache(user.id);
      }
    };
    window.addEventListener('online', handleOnline);
    return () => window.removeEventListener('online', handleOnline);
  });

  $effect(() => {
    const html = document.documentElement;
    html.classList.remove('theme-light', 'theme-dark', 'theme-sepia');
    html.classList.add(`theme-${theme.resolved}`);
  });

  $effect(() => {
    document.documentElement.classList.toggle('font-sans', font.value === 'sans');
  });

  $effect(() => {
    const html = document.documentElement;
    html.classList.remove('density-compact', 'density-comfortable', 'density-cosy');
    html.classList.add(`density-${density.value}`);
  });

  // Auth-route redirect contract:
  //   - Unauthenticated and NOT already on /sign-in → navigate('/sign-in').
  //   - Authenticated and ON /sign-in → navigate('/').
  $effect(() => {
    if (!$auth.bootstrapped) return;
    if ($auth.user == null && $route.name !== 'signin') {
      navigate('/sign-in');
    } else if ($auth.user != null && $route.name === 'signin') {
      navigate('/');
    }
  });
</script>

<svelte:window onkeydown={(e) => {
  if (e.key === '/' && !hotkeysOpen) {
    const tag = (e.target as HTMLElement)?.tagName?.toLowerCase();
    if (tag !== 'input' && tag !== 'textarea' && tag !== 'select') {
      e.preventDefault();
      searchOverlay.openOverlay();
      return;
    }
  }
  keyHandler(e);
}} />

{#if $needRefresh}
  <div class="sw-update-banner">
    Update available —
    <button onclick={() => updateServiceWorker(true)}>Reload</button>
  </div>
{/if}

{#if !$auth.bootstrapped}
  <!-- empty during bootstrap window -->
{:else if $route.name === 'signin' || $auth.user == null}
  <Login />
{:else}
  <HotkeysModal open={hotkeysOpen} onClose={() => hotkeysOpen = false} />
  <SearchOverlay />
  <AppShell>
    {#if $route.name === 'reader'}
      <Reader id={$route.params.id} />
    {:else if $route.name === 'saved'}
      <Saved />
    {:else if $route.name === 'categories'}
      <Categories />
    {:else if $route.name === 'feeds'}
      <Feeds />
    {:else if $route.name === 'history'}
      <History />
    {:else if $route.name === 'settings'}
      <Settings />
    {:else if $route.name === 'admin'}
      {#if $auth.user.role === 'admin'}<Admin />{:else}<p>Access denied.</p>{/if}
    {:else}
      <Unread />
    {/if}
  </AppShell>
{/if}

<style>
  .sw-update-banner {
    position: fixed;
    top: 0; left: 0; right: 0;
    z-index: 9999;
    background: var(--accent);
    color: #fff;
    padding: 8px 16px;
    font-size: 13px;
    display: flex; align-items: center; gap: 16px;
  }
  .sw-update-banner button {
    background: rgba(255,255,255,0.2);
    border: 1px solid rgba(255,255,255,0.5);
    color: #fff;
    padding: 4px 12px;
    border-radius: 4px;
    cursor: pointer;
  }
</style>
