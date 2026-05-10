<script lang="ts">
  import { onMount } from 'svelte';
  import { route } from './lib/router';
  import { auth } from './lib/auth';
  import Login from './views/Login.svelte';
  import Unread from './views/Unread.svelte';
  import Reader from './views/Reader.svelte';

  onMount(() => {
    void auth.bootstrap();
  });
</script>

{#if !$auth.bootstrapped}
  <!-- empty during the brief bootstrap window — keeps the SPA from
       flashing the login form for an authenticated user -->
{:else if $auth.user == null}
  <Login />
{:else if $route.name === 'reader'}
  <Reader id={$route.params.id} />
{:else}
  <Unread />
{/if}
