<script lang="ts">
  import { onMount } from 'svelte';
  import { route, navigate } from './lib/router';
  import { auth } from './lib/auth';
  import Login from './views/Login.svelte';
  import Unread from './views/Unread.svelte';
  import Reader from './views/Reader.svelte';
  import Security from './views/settings/Security.svelte';
  import Admin from './views/Admin.svelte';

  onMount(() => {
    void auth.bootstrap();
  });

  function handleLogout() {
    auth.logout();
  }
</script>

{#if !$auth.bootstrapped}
  <!-- empty during the brief bootstrap window -->
{:else if $auth.user == null}
  <Login />
{:else}
  <nav>
    <button onclick={() => navigate('/')}>Unread</button>
    <button onclick={() => navigate('/settings')}>Settings</button>
    {#if $auth.user.role === 'admin'}
      <button onclick={() => navigate('/admin')}>Users</button>
    {/if}
    <button onclick={handleLogout}>Sign out</button>
  </nav>
  {#if $route.name === 'reader'}
    <Reader id={$route.params.id} />
  {:else if $route.name === 'settings'}
    <Security />
  {:else if $route.name === 'admin'}
    {#if $auth.user.role === 'admin'}
      <Admin />
    {:else}
      <p>Access denied.</p>
    {/if}
  {:else}
    <Unread />
  {/if}
{/if}

<style>
  nav {
    display: flex;
    gap: 0.5rem;
    padding: 0.5rem 1rem;
    border-bottom: 1px solid #ddd;
  }
</style>
