<script lang="ts">
  import { route, navigate } from '../lib/router';

  const tabs = [
    { name: 'unread',   label: 'Unread',   path: '/'         },
    { name: 'saved',    label: 'Saved',    path: '/saved'    },
    { name: 'search',   label: 'Search',   path: '/search'   },
    { name: 'settings', label: 'Settings', path: '/settings' },
  ] as const;

  const activeTab = $derived($route.name === 'reader' ? 'unread' : $route.name);
</script>

<nav class="m-tabbar" aria-label="Main navigation">
  {#each tabs as tab (tab.name)}
    <button
      class="tab"
      class:active={activeTab === tab.name}
      onclick={() => navigate(tab.path)}
      aria-current={activeTab === tab.name ? 'page' : undefined}
      aria-label={tab.label}
    >
      <span class="ico" aria-hidden="true">
        {#if tab.name === 'unread'}
          <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M2 4h12M2 8h12M2 12h8"/></svg>
        {:else if tab.name === 'saved'}
          <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M3 2h10v13l-5-3-5 3V2z"/></svg>
        {:else if tab.name === 'search'}
          <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="6.5" cy="6.5" r="4"/><path d="M10.5 10.5l3 3"/></svg>
        {:else}
          <svg width="20" height="20" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="8" cy="8" r="3"/><path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.1 3.1l1.4 1.4M11.5 11.5l1.4 1.4M3.1 12.9l1.4-1.4M11.5 4.5l1.4-1.4"/></svg>
        {/if}
      </span>
      <span>{tab.label}</span>
    </button>
  {/each}
</nav>
