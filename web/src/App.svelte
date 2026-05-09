<script lang="ts">
  import { route } from './lib/router';
  import Unread from './views/Unread.svelte';
  import Reader from './views/Reader.svelte';
</script>

{#if $route.name === 'reader'}
  <Reader id={$route.params.id} />
{:else}
  <Unread />
{/if}
