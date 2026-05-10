<script lang="ts">
  import { onMount, setContext } from 'svelte';
  import { get } from 'svelte/store';
  import { route, navigate } from './lib/router';
  import { auth } from './lib/auth';
  import { offlineQueue } from './lib/offlineQueue';
  import { warmCache } from './lib/warmCache';
  import { theme, font, density } from './lib/preferences.svelte';
  import { buildHandler } from './lib/keyboard';
  import { useRegisterSW } from 'virtual:pwa-register/svelte';
  import Login from './views/Login.svelte';
  import Unread from './views/Unread.svelte';
  import Reader from './views/Reader.svelte';
  import Saved from './views/Saved.svelte';
  import Search from './views/Search.svelte';
  import Category from './views/Category.svelte';
  import Settings from './views/Settings.svelte';
  import Admin from './views/Admin.svelte';
  import HotkeysModal from './components/HotkeysModal.svelte';
  import TabBar from './components/TabBar.svelte';

  const { needRefresh, updateServiceWorker } = useRegisterSW();

  let hotkeysOpen = $state(false);
  let isMobile = $state(
    typeof window !== 'undefined' && window.matchMedia('(max-width: 768px)').matches
  );

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
      if (hotkeysOpen) { hotkeysOpen = false; return; }
      if ($route.name === 'reader') navigate('/');
    },
    setModalOpen: (open: boolean) => { hotkeysOpen = open; },
  });

  onMount(() => {
    void auth.bootstrap().then(() => {
      const user = get(auth).user;
      if (user) {
        void offlineQueue.drain(user.id);
        setTimeout(() => { void warmCache(user.id); }, 2000);
      }
    });

    const mq768 = window.matchMedia('(max-width: 768px)');
    const onResize = (e: MediaQueryListEvent) => { isMobile = e.matches; };
    mq768.addEventListener('change', onResize);

    const handleOnline = async () => {
      const user = get(auth).user;
      if (user) {
        await offlineQueue.drain(user.id);
        void warmCache(user.id);
      }
    };
    window.addEventListener('online', handleOnline);

    return () => {
      mq768.removeEventListener('change', onResize);
      window.removeEventListener('online', handleOnline);
    };
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
    html.classList.remove('density-compact', 'density-comfortable');
    if (density.value !== 'default') html.classList.add(`density-${density.value}`);
  });
</script>

<svelte:window onkeydown={(e) => {
  // '/' focuses search; only fires outside form controls.
  if (e.key === '/' && !hotkeysOpen) {
    const tag = (e.target as HTMLElement)?.tagName?.toLowerCase();
    if (tag !== 'input' && tag !== 'textarea' && tag !== 'select') {
      e.preventDefault();
      if ($route.name !== 'search') navigate('/search');
      // Focus happens in Search.svelte onMount; also dispatch a custom event.
      window.dispatchEvent(new CustomEvent('tap:focus-search'));
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

<HotkeysModal open={hotkeysOpen} onClose={() => { hotkeysOpen = false; }} />

<div class="app-shell" class:is-mobile={isMobile}>
  {#if !$auth.bootstrapped}
    <!-- empty during bootstrap window -->
  {:else if $auth.user == null}
    <Login />
  {:else if $route.name === 'reader'}
    <Reader id={$route.params.id} />
  {:else if $route.name === 'saved'}
    <Saved />
  {:else if $route.name === 'search'}
    <Search />
  {:else if $route.name === 'category'}
    <Category id={$route.params.id} />
  {:else if $route.name === 'settings'}
    <Settings />
  {:else if $route.name === 'admin'}
    {#if $auth.user.role === 'admin'}
      <Admin />
    {:else}
      <p>Access denied.</p>
    {/if}
  {:else}
    <Unread />
  {/if}
  {#if isMobile && $auth.user != null && $auth.bootstrapped}
    <TabBar />
  {/if}
</div>

<style>
  :global(.app-shell) { display: flex; flex-direction: column; height: 100vh; }

  .sw-update-banner {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    z-index: 9999;
    background: var(--accent, #002FA7);
    color: #fff;
    padding: 0.5rem 1rem;
    font-size: 0.875rem;
    display: flex;
    align-items: center;
    gap: 1rem;
  }
  .sw-update-banner button {
    background: rgba(255,255,255,0.2);
    border: 1px solid rgba(255,255,255,0.5);
    color: #fff;
    padding: 0.25rem 0.75rem;
    border-radius: 4px;
    cursor: pointer;
  }
</style>
